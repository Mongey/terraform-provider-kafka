package kafka

import (
	"fmt"
	"log"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestAccBasicTopic(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)
	r.Test(t, r.TestCase{
		Providers: accProvider(),
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(testResourceTopic_noConfig, topicName),
				Check:  testResourceTopic_noConfigCheck,
			},
		},
	})
}

func TestAccTopicConfigUpdate(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)

	r.Test(t, r.TestCase{
		Providers: accProvider(),
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(testResourceTopic_initialConfig, topicName),
				Check:  testResourceTopic_initialCheck,
			},
			{
				Config: fmt.Sprintf(testResourceTopic_updateConfig, topicName),
				Check:  testResourceTopic_updateCheck,
			},
		},
	})
}

func TestAccTopicUpdatePartitions(t *testing.T) {
	u, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatal(err)
	}
	topicName := fmt.Sprintf("syslog-%s", u)

	r.Test(t, r.TestCase{
		Providers: accProvider(),
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: fmt.Sprintf(testResourceTopic_initialConfig, topicName),
				Check:  testResourceTopic_initialCheck,
			},
			{
				Config: fmt.Sprintf(testResourceTopic_updatePartitions, topicName),
				Check:  testResourceTopic_updatePartitionsCheck,
			},
		},
	})
}

func testResourceTopic_noConfigCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
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

	//if name != "syslog" {
	//return fmt.Errorf("unexpected topic name %s", name)
	//}

	client := testProvider.Meta().(*Client)
	topic, err := client.ReadTopic(name)

	if err != nil {
		return err
	}

	log.Println("[INFO] Hello from tests")

	if len(topic.Config) != 0 {
		return fmt.Errorf("expected no configs for %s, got %v", name, topic.Config)
	}

	return nil
}

func testResourceTopic_initialCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
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

	//if name != "syslog" {
	//return fmt.Errorf("unexpected topic name %s", name)
	//}

	client := testProvider.Meta().(*Client)
	topic, err := client.ReadTopic(name)
	if err != nil {
		return err
	}

	if v, ok := topic.Config["retention.ms"]; ok && *v != "11111" {
		return fmt.Errorf("retention.ms did not get set got: %v", topic.Config)
	}
	if v, ok := topic.Config["segment.ms"]; ok && *v != "22222" {
		return fmt.Errorf("segment.ms !=  %v", topic)
	}

	return nil
}

func testResourceTopic_updateCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
	instanceState := resourceState.Primary
	client := testProvider.Meta().(*Client)
	name := instanceState.ID

	if name != instanceState.Attributes["name"] {
		return fmt.Errorf("id doesn't match name")
	}

	topic, err := client.ReadTopic(name)
	if err != nil {
		return err
	}

	if v, ok := topic.Config["segment.ms"]; ok && *v != "33333" {
		return fmt.Errorf("segment.ms did not get updated, got: %v", topic.Config)
	}
	if v, ok := topic.Config["segment.bytes"]; ok && *v != "44444" {
		return fmt.Errorf("segment.bytes did not get updated, got: %s, expected 44444", *v)
	}

	if v, ok := topic.Config["retention.ms"]; ok || v != nil {
		return fmt.Errorf("retention.ms did not get removed, got: %v", topic.Config)
	}

	return nil
}

func testResourceTopic_updatePartitionsCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_topic.test"]
	instanceState := resourceState.Primary
	client := testProvider.Meta().(*Client)
	name := instanceState.ID
	topic, err := client.ReadTopic(name)
	if err != nil {
		return err
	}
	if topic.Partitions != 2 {
		return fmt.Errorf("partitions did not get increated got: %d", topic.Partitions)
	}

	if v, ok := topic.Config["segment.ms"]; ok && *v != "33333" {
		return fmt.Errorf("segment.ms !=  %v", topic)
	}
	return nil
}

const testResourceTopic_noConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1
}
`

const testResourceTopic_initialConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1

  config = {
    "retention.ms" = "11111"
    "segment.ms" = "22222"
  }
}
`

const testResourceTopic_updateConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1

  config = {
    "segment.ms" = "33333"
    "segment.bytes" = "44444"
  }
}
`

const testResourceTopic_updatePartitions = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 2

  config = {
    "retention.ms" = "11111"
    "segment.ms" = "33333"
  }
}
`
