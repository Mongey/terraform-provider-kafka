package kafka

import (
	"fmt"
	"testing"

	"github.com/Shopify/sarama"
	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAcc_ACLCreateAndUpdate(t *testing.T) {
	r.Test(t, r.TestCase{
		Providers:  accProvider(),
		IsUnitTest: false,
		PreCheck:   func() { testAccPreCheck(t) },
		Steps: []r.TestStep{
			{
				Config: testResourceACL_initialConfig,
				Check:  testResourceACL_initialCheck,
			},
			{
				Config: testResourceACL_updateConfig,
				Check:  testResourceACL_updateCheck,
			},
		},
	})
}

func testResourceACL_initialCheck(s *terraform.State) error {
	resourceState := s.Modules[0].Resources["kafka_acl.test"]
	if resourceState == nil {
		return fmt.Errorf("resource not found in state")
	}

	instanceState := resourceState.Primary
	if instanceState == nil {
		return fmt.Errorf("resource has no primary instance")
	}

	client := testProvider.Meta().(*Client)

	acls, err := client.ListACLs()
	if err != nil {
		return err
	}

	if len(acls) != 1 {
		return fmt.Errorf("There should be one acls %v %s", acls, err)
	}

	if acls[0].Acls[0].PermissionType != sarama.AclPermissionAllow {
		return fmt.Errorf("Should be Allow, not %v", acls[0].Acls[0].PermissionType)
	}
	return nil
}

func testResourceACL_updateCheck(s *terraform.State) error {
	client := testProvider.Meta().(*Client)

	acls, err := client.ListACLs()
	if err != nil {
		return err
	}

	if len(acls) == 0 {
		return fmt.Errorf("There should be some acls %v %s", acls, err)
	}

	if acls[0].ResourceName != "syslog" {
		return fmt.Errorf("The expected ACL should be for syslog")
	}

	if acls[0].ResourceType != sarama.AclResourceTopic {
		return fmt.Errorf("Should be for a topic")
	}

	if acls[0].Acls[0].Principal != "User:Alice" {
		return fmt.Errorf("Should be for Alice")
	}

	if acls[0].Acls[0].Host != "*" {
		return fmt.Errorf("Should be for *")
	}
	if acls[0].Acls[0].PermissionType != sarama.AclPermissionDeny {
		return fmt.Errorf("Should be Deny, not %v", acls[0].Acls[0].PermissionType)
	}

	return nil
}

const testResourceACL_initialConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
	ca_cert_file      = "../secrets/snakeoil-ca-1.crt"
	client_cert_file = "../secrets/kafkacat-ca1-signed.pem"
	client_key_file  = "../secrets/kafkacat-raw-private-key.pem"
	skip_tls_verify   = true
}

resource "kafka_acl" "test" {
	resource_name       = "syslog"
	resource_type       = "Topic"
	acl_principal       = "User:Alice"
	acl_host            = "*"
	acl_operation       = "Write"
	acl_permission_type = "Allow"
}
`

const testResourceACL_updateConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
	ca_cert_file      = "../secrets/snakeoil-ca-1.crt"
	client_cert_file = "../secrets/kafkacat-ca1-signed.pem"
	client_key_file  = "../secrets/kafkacat-raw-private-key.pem"
	skip_tls_verify   = true
}

resource "kafka_acl" "test" {
	resource_name       = "syslog"
	resource_type       = "Topic"
	acl_principal       = "User:Alice"
	acl_host            = "*"
	acl_operation       = "Write"
	acl_permission_type = "Deny"
}
`

func TestAccTopicACLDelConfigUpdate(t *testing.T) {
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
				Config: fmt.Sprintf(testResourceTopicAndACL_initialConfig, topicName, topicName),
			},
			{
				Config: fmt.Sprintf(testResourceTopicAndACL_updateConfig, topicName),
			},
		},
	})
}

const testResourceTopicAndACL_initialConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert_file      = "../secrets/snakeoil-ca-1.crt"
  client_cert_file = "../secrets/kafkacat-ca1-signed.pem"
  client_key_file  = "../secrets/kafkacat-raw-private-key.pem"
  skip_tls_verify   = true
}
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1
}
resource "kafka_acl" "test" {
  resource_name       = "%s"
  resource_type       = "Topic"
  resource_pattern_type_filter = "Literal"
  acl_principal       = "User:Alice"
  acl_host            = "*"
  acl_operation       = "Write"
  acl_permission_type = "Allow"
}
`

const testResourceTopicAndACL_updateConfig = `
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert_file      = "../secrets/snakeoil-ca-1.crt"
  client_cert_file = "../secrets/kafkacat-ca1-signed.pem"
  client_key_file  = "../secrets/kafkacat-raw-private-key.pem"
  skip_tls_verify   = true
}
resource "kafka_topic" "test" {
  name               = "%s"
  replication_factor = 1
  partitions         = 1
}
`
