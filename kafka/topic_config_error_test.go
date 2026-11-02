package kafka

import (
	"errors"
	"strings"
	"testing"

	"github.com/IBM/sarama"
)

type configControllerClient struct {
	sarama.Client
	broker *sarama.Broker
}

func (c configControllerClient) Controller() (*sarama.Broker, error) { return c.broker, nil }

func TestTopicConfigResponse(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resource *sarama.ResourceResponse
		wantErr  error
	}{
		{name: "authorization", resource: &sarama.ResourceResponse{ErrorCode: int16(sarama.ErrTopicAuthorizationFailed), ErrorMsg: "access denied"}, wantErr: sarama.ErrTopicAuthorizationFailed},
		{name: "success", resource: &sarama.ResourceResponse{Configs: []*sarama.ConfigEntry{{Name: "cleanup.policy", Value: "compact", Source: sarama.SourceTopic}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := sarama.NewMockBroker(t, 0)
			defer mock.Close()
			mock.SetHandlerByMap(map[string]sarama.MockResponse{"DescribeConfigsRequest": sarama.NewMockWrapper(&sarama.DescribeConfigsResponse{Version: 2, Resources: []*sarama.ResourceResponse{tc.resource}})})
			cfg := sarama.NewConfig()
			cfg.Version = sarama.V2_0_0_0
			cfg.ApiVersionsRequest = false
			broker := sarama.NewBroker(mock.Addr())
			if err := broker.Open(cfg); err != nil {
				t.Fatal(err)
			}
			defer broker.Close()
			client := &Client{client: configControllerClient{broker: broker}, kafkaConfig: cfg, config: &Config{}}
			got, err := client.topicConfig("example")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || !strings.Contains(err.Error(), "access denied") {
					t.Fatalf("expected authorization error with broker message, got %v", err)
				}
			} else if err != nil || got["cleanup.policy"] == nil || *got["cleanup.policy"] != "compact" {
				t.Fatalf("expected config value, got %v, error %v", got, err)
			}
		})
	}
}
