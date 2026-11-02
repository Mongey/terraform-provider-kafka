package kafka

import (
	"context"
	"errors"
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
		{name: "network", err: errors.New("network unavailable")},
		{name: "missing", err: sarama.ErrUnknownTopicOrPartition, missing: true},
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
