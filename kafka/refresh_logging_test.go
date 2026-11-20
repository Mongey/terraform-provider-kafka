package kafka

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/IBM/sarama"
)

func TestRefreshLoggingIsBounded(t *testing.T) {
	h := newTopicRefreshHarness(t, 1, sarama.V2_0_0_0, 0)
	defer h.close(t)
	entries := []*sarama.ConfigEntry{{Name: "custom.config", Value: "secret-sentinel", Source: sarama.SourceTopic}}
	for i := 0; i < 200; i++ {
		entries = append(entries, &sarama.ConfigEntry{Name: fmt.Sprintf("default-%d", i), Value: "default", Default: true, Source: sarama.SourceDefault})
	}
	for _, broker := range h.brokers {
		broker.SetHandlerByMap(map[string]sarama.MockResponse{
			"DescribeConfigsRequest": sarama.NewMockWrapper(&sarama.DescribeConfigsResponse{Version: 2,
				Resources: []*sarama.ResourceResponse{{Configs: entries}}}),
		})
	}
	lazy := &LazyClient{inner: h.client, Config: &Config{SASLPassword: "credential-sentinel"}}
	lazy.once.Do(func() {})
	logs := captureSecurityLogs(t)
	const reads = 20
	for i := 0; i < reads; i++ {
		topic, err := lazy.ReadTopic(h.names[0], false)
		if err != nil || len(topic.Config) != 1 || topic.Config["custom.config"] == nil || *topic.Config["custom.config"] != "secret-sentinel" {
			t.Fatalf("topic refresh changed: %#v, %v", topic, err)
		}
	}
	output := logs.String()
	if lines := strings.Count(output, "\n"); lines > 3*reads {
		t.Fatalf("refresh logging scales with config entries: %d lines", lines)
	}
	if strings.Count(output, "reading topic '") != reads || strings.Count(output, "Read 1 config entries from Kafka; partitions=1 replication_factor=3") != reads {
		t.Fatalf("missing refresh diagnostics: %s", output)
	}
	for _, unwanted := range []string{"default-", "secret-sentinel", "credential-sentinel", "lazy client init"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("refresh logs contain %q", unwanted)
		}
	}
	for _, broker := range h.brokers {
		broker.SetHandlerByMap(map[string]sarama.MockResponse{
			"DescribeConfigsRequest": sarama.NewMockWrapper(&sarama.DescribeConfigsResponse{Version: 2,
				Resources: []*sarama.ResourceResponse{{ErrorCode: int16(sarama.ErrTopicAuthorizationFailed), ErrorMsg: "denied"}}}),
		})
	}
	if _, err := lazy.ReadTopic(h.names[0], false); !errors.Is(err, sarama.ErrTopicAuthorizationFailed) {
		t.Fatalf("authorization error lost: %v", err)
	}
	if !strings.Contains(logs.String(), "[ERROR]") {
		t.Fatal("refresh errors must still be logged")
	}
}

func TestCachedACLReadsDoNotLog(t *testing.T) {
	c := &Client{aclCache: aclCache{valid: true}}
	logs := captureSecurityLogs(t)
	for i := 0; i < 20; i++ {
		if _, err := c.ListACLs(); err != nil {
			t.Fatal(err)
		}
	}
	if logs.Len() != 0 {
		t.Fatalf("cached reads produced logs: %s", logs.String())
	}
}
