package kafka

import (
	"fmt"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestAccTopicAndACL(t *testing.T) {
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
				Config: fmt.Sprintf(testResourceTopic_aclAndTopicConfig, topicName, topicName),
				Check:  test_aclAndTopicConfigCheck,
			},
		},
	})
}

func test_aclAndTopicConfigCheck(s *terraform.State) error {
	topicState := s.Modules[0].Resources["kafka_topic.syslog"]
	if topicState == nil {
		return fmt.Errorf("topic not found in state")
	}
	return nil
}

const testResourceTopic_aclAndTopicConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

resource "kafka_topic" "syslog" {
  name               = "%s"
  replication_factor = 1
  partitions         = 4

  config = {
    "segment.ms"   = "4000"
    "retention.ms" = "86400000"
  }
}

resource "kafka_acl" "test" {
  resource_name       = "%s"
  resource_type       = "Topic"
  acl_principal       = "User:Alice"
  acl_host            = "*"
  acl_operation       = "Write"
  acl_permission_type = "Deny"
}
`
