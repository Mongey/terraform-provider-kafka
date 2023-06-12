terraform {
  required_providers {
    kafka = {
      source = "terraform.local/local/kafka"
    }
  }
}

provider "kafka" {
  bootstrap_servers = ["localhost:9092"]

  ca_cert     = file("../secrets/ca.crt")
  client_cert = file("../secrets/client.pem")
  client_key  = file("../secrets/client-no-password.key")
  tls_enabled = true
}

# Make sure we don't lock down ourself on first run of terraform.
# First grant ourself admin permissions, then add ACL for topic.
resource "kafka_acl" "global" {
  resource_name       = "*"
  resource_type       = "Topic"
  acl_principal       = "User:*"
  acl_host            = "*"
  acl_operation       = "All"
  acl_permission_type = "Allow"
}

resource "kafka_topic" "syslog" {
  name               = "syslog"
  replication_factor = 1
  partitions         = 4

  config = {
    "segment.ms"   = "4000"
    "retention.ms" = "86400000"
  }

  depends_on = [kafka_acl.global]
}

resource "kafka_acl" "test" {
  resource_name       = "syslog"
  resource_type       = "Topic"
  acl_principal       = "User:Alice"
  acl_host            = "*"
  acl_operation       = "Write"
  acl_permission_type = "Deny"

  depends_on = [kafka_acl.global]
}

resource "kafka_quota" "quota1" {
  entity_name = "client1"
  entity_type = "client-id"
  config = {
    "consumer_byte_rate" = "4000000"
    "producer_byte_rate" = "3500000"
  }
}

data "kafka_topic" "a" {
  name = "syslog"
}

data "kafka_topics" "a" {}


output "topic_list" {
  description = "Topic name"
  value       = data.kafka_topics.a
}
