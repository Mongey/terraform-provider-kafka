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
				Config: cfg(t, bs,testDataSourceKafkaTopics),
				Check: r.ComposeTestCheckFunc(
					r.TestCheckResourceAttr("data.kafka_topics.test", "list.0", expectedTopic),
					r.TestCheckResourceAttr("data.kafka_topics.test", "list.#", "1"),
				),
			},
		},
	})
}

const testDataSourceKafkaTopics = `
data "kafka_topics" "test" {
}
`