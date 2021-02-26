package kafka

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAcc_TopicData(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	bs := testBootstrapServers[0]
	r.Test(t, r.TestCase{
		Providers: accProvider(),
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(testDataSourceTopic_readMissingTopic, bs, topicName),
				Check:  testDataSourceTopic_missingTopicCheck,
			},
			{
				Config: fmt.Sprintf(testDataSourceTopic_readExistingTopic, bs, topicName),
				Check:  testDataSourceTopic_existingTopicCheck,
			},
		},
	})
}

func testDataSourceTopic_missingTopicCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["data.kafka_topic.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	if instanceState.ID != "" {
		return fmt.Errorf("topic resource present")
	}

	return nil
}

func testDataSourceTopic_existingTopicCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["data.kafka_topic.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	name := instanceState.ID

	if name != instanceState.Attributes["name"] {
		return fmt.Errorf("id doesn't match name")
	}

	if v, ok := instanceState.Attributes["replication_factor"]; ok && v != "1" {
		return fmt.Errorf("replication_factor did not match, got: %v", instanceState.Attributes["replication_factor"])
	}
	if v, ok := instanceState.Attributes["partitions"]; ok && v != "1" {
		return fmt.Errorf("partitions did not get match, got: %v", instanceState.Attributes["partitions"])
	}
	if v, ok := instanceState.Attributes["config.segment.ms"]; ok && v != "22222" {
		return fmt.Errorf("segment.ms did not get match, got: %v", instanceState.Attributes["config.segment.ms"])
	}
	
	return nil
}

const testDataSourceTopic_readExistingTopic = `
provider "kafka" {
  bootstrap_servers = ["%[1]s"]
}

resource "kafka_topic" "test" {
  name               = "%[2]s"
  replication_factor = 1
  partitions         = 1

  config = {
    "segment.ms" = "22222"
  }
}

data "kafka_topic" "test" {
  name               = "%[2]s"
}
`

const testDataSourceTopic_readMissingTopic = `
provider "kafka" {
  bootstrap_servers = ["%[1]s"]
}

data "kafka_topic" "test" {
  name               = "%[2]s"
}
`
