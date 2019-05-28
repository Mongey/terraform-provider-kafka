package kafka

import (
	"fmt"
	"testing"

	uuid "github.com/hashicorp/go-uuid"
	r "github.com/hashicorp/terraform/helper/resource"
)

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
