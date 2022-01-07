package kafka

import (
	"testing"

	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_Topics(t *testing.T) {
	bs := testBootstrapServers[0]
	// Should be only one topic in a brand new kafka cluster
	expectedTopic := "__confluent.support.metrics"
	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, testDataSourceKafkaTopics),
				Check: r.ComposeTestCheckFunc(
					r.TestCheckOutput("partitions", "1"),
					r.TestCheckOutput("replication_factor", "3"),
					r.TestCheckOutput("topic_name", expectedTopic),
					r.TestCheckOutput("retention_ms", "31536000000"),
				),
			},
		},
	})
}

const testDataSourceKafkaTopics = `
data "kafka_topics" "test" {
}

output "partitions" {
 value = data.kafka_topics.test.list[0].partitions
}

output "replication_factor" {
 value = data.kafka_topics.test.list[0].replication_factor
}

output "topic_name" {
 value = data.kafka_topics.test.list[0].topic_name
}

output "retention_ms" {
 value = data.kafka_topics.test.list[0].config["retention.ms"]
}
`
