package kafka

import (
	"fmt"
	"regexp"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_TopicData(t *testing.T) {
	t.Parallel()
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]

	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		//PreCheck: func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config:      cfg(t, bs, fmt.Sprintf(testDataSourceTopic_readMissingTopic, topicName)),
				ExpectError: regexp.MustCompile(fmt.Sprintf("could not find topic '%s'", topicName)),
			},
			{
				Config: cfg(t, bs, fmt.Sprintf(testDataSourceTopic_readExistingTopic, topicName)),
				Check: r.ComposeTestCheckFunc(
					r.TestCheckResourceAttr("data.kafka_topic.test", "id", topicName),
					r.TestCheckResourceAttr("data.kafka_topic.test", "name", topicName),
					r.TestCheckResourceAttr("data.kafka_topic.test", "replication_factor", "1"),
					r.TestCheckResourceAttr("data.kafka_topic.test", "partitions", "1"),
					r.TestCheckResourceAttr("data.kafka_topic.test", "config.segment.ms", "22222"),
				),
			},
		},
	})
}

const testDataSourceTopic_readExistingTopic = `
resource "kafka_topic" "test" {
  name               = "%[1]s"
  replication_factor = 1
  partitions         = 1
  config = {
    "segment.ms" = "22222"
  }
}

data "kafka_topic" "test" {
  name               = kafka_topic.test.name
}
`

const testDataSourceTopic_readMissingTopic = `
data "kafka_topic" "test" {
  name               = "%[1]s"
}
`
