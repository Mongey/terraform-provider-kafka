package kafka

import (
	"fmt"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"testing"

	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_Topics(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)

	bs := testBootstrapServers[0]
	r.Test(t, r.TestCase{
		ProviderFactories: overrideProviderFactory(),
		Steps: []r.TestStep{
			{
				Config: cfg(t, bs, fmt.Sprintf(testDataSourceKafkaTopics, topicName)),
				Check: r.ComposeTestCheckFunc(
					testDatasourceTopics,
				),
			},
		},
	})
}

const testDataSourceKafkaTopics = `
resource "kafka_topic" "test" {
  name               = "%[1]s"
  replication_factor = 1
  partitions         = 1
  config = {
    "retention.ms" = "22222"
  }
}
data "kafka_topics" "test" {
 depends_on = [kafka_topic.test]
}
`

func testDatasourceTopics(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["data.kafka_topics.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}
	instanceState := resourceState.Primary
	client := testProvider.Meta().(*LazyClient)
	expectedTopics, err := client.GetKafkaTopics()
	if err != nil {
		return fmt.Errorf(err.Error())
	}
	for i := 0; i < len(expectedTopics); i++ {
		expectedTopicName := instanceState.Attributes[fmt.Sprintf("list.%d.topic_name", i)]
		expectedTopicOutput, err := client.ReadTopic(expectedTopicName, true)
		if err != nil {
			return fmt.Errorf(err.Error())
		}

		if instanceState.Attributes[fmt.Sprintf("list.%d.partitions", i)] != fmt.Sprint(expectedTopicOutput.Partitions) {
			return fmt.Errorf("expected %d for topic %s partition, got %s", expectedTopicOutput.Partitions, expectedTopicOutput.Name, instanceState.Attributes[fmt.Sprintf("list.%d.partitions", i)])
		}
		if instanceState.Attributes[fmt.Sprintf("list.%d.replication_factor", i)] != fmt.Sprint(expectedTopicOutput.ReplicationFactor) {
			return fmt.Errorf("expected %d for topic %s replication factor, got %s", expectedTopicOutput.ReplicationFactor, expectedTopicOutput.Name, instanceState.Attributes[fmt.Sprintf("list.%d.replication_factor", i)])
		}
		retentionMs := expectedTopicOutput.Config["retention.ms"]
		if instanceState.Attributes[fmt.Sprintf("list.%d.config.retention.ms", i)] != *retentionMs {
			return fmt.Errorf("expected %s for topic %s config retention.ms, got %s", *retentionMs, expectedTopicOutput.Name, instanceState.Attributes[fmt.Sprintf("list.%d.config.retention.ms", i)])
		}
	}
	return nil
}
