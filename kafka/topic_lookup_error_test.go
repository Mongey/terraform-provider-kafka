package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/IBM/sarama"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type partitionErrorClient struct {
	sarama.Client
	err error
}

func (c partitionErrorClient) Partitions(string) ([]int32, error) {
	return nil, c.err
}

func TestTopicReadPartitionErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		missing bool
	}{
		{name: "authorization", err: sarama.ErrTopicAuthorizationFailed},
		{name: "wrapped authorization", err: fmt.Errorf("example: %w", sarama.ErrTopicAuthorizationFailed)},
		{name: "network", err: errors.New("network unavailable")},
		{name: "missing", err: sarama.ErrUnknownTopicOrPartition, missing: true},
		{name: "wrapped missing", err: fmt.Errorf("example: %w", sarama.ErrUnknownTopicOrPartition), missing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &Client{client: partitionErrorClient{err: tc.err}, topics: map[string]void{"example": member}}
			_, err := client.ReadTopic("example", false)
			if tc.missing {
				if _, ok := err.(TopicMissingError); !ok {
					t.Fatalf("expected missing topic error, got %v", err)
				}
			} else if !errors.Is(err, tc.err) {
				t.Fatalf("expected partition error %v, got %v", tc.err, err)
			}

			lazy := &LazyClient{inner: client}
			lazy.once.Do(func() {})
			d := schema.TestResourceDataRaw(t, kafkaTopicResource().Schema, nil)
			d.SetId("example")
			diags := topicRead(context.Background(), d, lazy)
			if tc.missing {
				if d.Id() != "" || diags.HasError() {
					t.Fatalf("missing topic: ID = %q, diagnostics = %v", d.Id(), diags)
				}
			} else if d.Id() != "example" || !diags.HasError() {
				t.Fatalf("partition error: ID = %q, diagnostics = %v", d.Id(), diags)
			}
		})
	}
}

func TestTopicReadMissingMetadata(t *testing.T) {
	broker := sarama.NewMockBroker(t, 0)
	defer broker.Close()
	metadata := sarama.NewMockMetadataResponse(t).SetBroker(broker.Addr(), broker.BrokerID()).SetController(broker.BrokerID())
	broker.SetHandlerByMap(map[string]sarama.MockResponse{"MetadataRequest": metadata})
	cfg := sarama.NewConfig()
	cfg.ApiVersionsRequest = false
	cfg.Metadata.Retry.Max = 0
	cfg.Metadata.RefreshFrequency = 0
	client, err := sarama.NewClient([]string{broker.Addr()}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	broker.SetHandlerByMap(map[string]sarama.MockResponse{"MetadataRequest": metadata.SetError("missing", sarama.ErrUnknownTopicOrPartition)})

	// Use Sarama's actual metadata error rather than a bare Kafka error code.
	metadataErr := client.RefreshMetadata("missing")
	if !errors.Is(metadataErr, sarama.ErrUnknownTopicOrPartition) {
		t.Fatalf("expected Kafka missing-topic error, got %v", metadataErr)
	}
	lazy := &LazyClient{inner: &Client{client: client, topics: map[string]void{"missing": member}}}
	lazy.once.Do(func() {})
	t.Run("refresh", func(t *testing.T) {
		_, err := lazy.ReadTopic("missing", true)
		if _, ok := err.(TopicMissingError); !ok {
			t.Fatalf("expected TopicMissingError, got %T: %v", err, err)
		}
	})
	t.Run("partitions", func(t *testing.T) {
		d := schema.TestResourceDataRaw(t, kafkaTopicResource().Schema, nil)
		d.SetId("missing")
		diagnostics := topicRead(context.Background(), d, lazy)
		if diagnostics.HasError() || d.Id() != "" {
			t.Fatalf("expected missing resource to clear state, got ID %q, diagnostics %v", d.Id(), diagnostics)
		}
	})
	t.Run("data source", func(t *testing.T) {
		d := schema.TestResourceDataRaw(t, kafkaTopicDataSource().Schema, map[string]interface{}{"name": "missing"})
		err := dataSourceTopicRead(d, lazy)
		if err == nil || !strings.Contains(err.Error(), "could not find topic 'missing'") {
			t.Fatalf("unexpected data source error: %v", err)
		}
	})
	t.Run("delete waiter", func(t *testing.T) {
		_, state, err := topicDeleteFunc(lazy, "missing", Topic{Name: "missing"})()
		if err != nil || state != "Deleted" {
			t.Fatalf("expected successful deletion, got state %q, error %v", state, err)
		}
	})
	t.Run("create waiter", func(t *testing.T) {
		_, state, err := topicCreateFunc(lazy, Topic{Name: "missing"})()
		if err != nil || state != "Pending" {
			t.Fatalf("expected pending creation, got state %q, error %v", state, err)
		}
	})
}
