package kafka

import (
	"testing"

	"github.com/IBM/sarama"
)

type topicRefreshClient struct {
	sarama.Client
	broker *sarama.Broker
}

func (c topicRefreshClient) RefreshMetadata(...string) error         { return nil }
func (c topicRefreshClient) Topics() ([]string, error)               { return []string{"example"}, nil }
func (c topicRefreshClient) Partitions(string) ([]int32, error)      { return []int32{0}, nil }
func (c topicRefreshClient) Replicas(string, int32) ([]int32, error) { return []int32{0}, nil }
func (c topicRefreshClient) Controller() (*sarama.Broker, error)     { return c.broker, nil }

func TestTopicRefreshWaitsForExpectedTopic(t *testing.T) {
	mock := sarama.NewMockBroker(t, 0)
	defer mock.Close()
	mock.SetHandlerByMap(map[string]sarama.MockResponse{
		"DescribeConfigsRequest": sarama.NewMockWrapper(&sarama.DescribeConfigsResponse{Version: 2, Resources: []*sarama.ResourceResponse{{}}}),
	})
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_0_0_0
	cfg.ApiVersionsRequest = false
	broker := sarama.NewBroker(mock.Addr())
	if err := broker.Open(cfg); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = broker.Close() }()
	client := &LazyClient{inner: &Client{client: topicRefreshClient{broker: broker}, kafkaConfig: cfg, config: &Config{}}}
	client.once.Do(func() {})

	for _, tc := range []struct {
		partitions int32
		state      string
	}{{2, "Updating"}, {1, "Ready"}} {
		expected := Topic{Name: "example", Partitions: tc.partitions, ReplicationFactor: 1, Config: map[string]*string{}}
		result, state, err := topicRefreshFunc(client, "example", expected)()
		if err != nil || state != tc.state || result == nil {
			t.Errorf("partitions %d: got result %#v, state %q, error %v; want non-nil result and state %q", tc.partitions, result, state, err, tc.state)
		}
	}
}
