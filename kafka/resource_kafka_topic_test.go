package kafka

import (
	"fmt"
	"testing"

	r "github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestBasicTopic(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers: accProvider(),
		PreCheck:  func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceTopic_initialConfig,
				Check:  testResourceTopic_initialCheck,
			},
			{
				Config: testResourceTopic_updateConfig,
				Check:  testResourceTopic_updateCheck,
			},
		},
	})
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

	if name != "syslog" {
		return fmt.Errorf("unexpected topic name %s", name)
	}

	client, _ := NewClient(&Config{
		Brokers: &[]string{"localhost:9092"},
	})

	topic, err := client.ReadTopic("syslog")

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
	client, _ := NewClient(&Config{
		Brokers: &[]string{"localhost:9092"},
	})
	topic, err := client.ReadTopic("syslog")
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

const testResourceTopic_initialConfig = `
provider "kafka" {
  brokers = ["localhost:9092"]
}

resource "kafka_topic" "test" {
  name               = "syslog"
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
  brokers = ["localhost:9092"]
}

resource "kafka_topic" "test" {
  name               = "syslog"
  replication_factor = 1
  partitions         = 1

  config = {
    "segment.ms" = "33333"
    "segment.bytes" = "44444"
  }
}
`
