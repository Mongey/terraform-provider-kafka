terraform {
  required_providers {
    kafka = {
      source  = "Mongey/kafka"
      version = "0.0.1"
    }
  }
}

variable "topic_count" {
  type    = number
  default = 593
  validation {
    condition     = var.topic_count >= 49
    error_message = "Use at least 49 topics so all 49 ACL targets exist."
  }
}

locals {
  # Terraform range permits at most 1024 values per call.
  topic_indices = flatten([for start in range(0, var.topic_count, 1000) : range(start, min(start + 1000, var.topic_count))])
  topics        = { for i in local.topic_indices : format("refresh-topic-%04d", i) => i }
  acls          = { for i in range(49) : format("refresh-topic-%04d", i) => i }
}

provider "kafka" {
  alias             = "first"
  bootstrap_servers = ["127.0.0.1:19092", "127.0.0.1:29092", "127.0.0.1:39092"]
  kafka_version     = "3.8.0"
  tls_enabled       = false
  timeout           = 120
}

provider "kafka" {
  alias             = "second"
  bootstrap_servers = ["127.0.0.1:19093", "127.0.0.1:29093", "127.0.0.1:39093"]
  kafka_version     = "3.8.0"
  tls_enabled       = false
  timeout           = 120
}

resource "kafka_topic" "first" {
  provider           = kafka.first
  for_each           = local.topics
  name               = each.key
  replication_factor = 3
  partitions         = 1
  config = {
    "retention.ms"   = tostring(60000 + each.value)
    "cleanup.policy" = each.value % 2 == 0 ? "compact" : "delete"
  }
}

resource "kafka_topic" "second" {
  provider           = kafka.second
  for_each           = local.topics
  name               = each.key
  replication_factor = 3
  partitions         = 1
  config = {
    "retention.ms"   = tostring(120000 + each.value)
    "cleanup.policy" = each.value % 2 == 0 ? "compact" : "delete"
  }
}

resource "kafka_acl" "first" {
  provider                     = kafka.first
  for_each                     = local.acls
  resource_name                = kafka_topic.first[each.key].name
  resource_type                = "Topic"
  resource_pattern_type_filter = "Literal"
  acl_principal                = "User:refresh-bench-${each.value}"
  acl_host                     = "*"
  acl_operation                = "Read"
  acl_permission_type          = "Allow"
}

resource "kafka_acl" "second" {
  provider                     = kafka.second
  for_each                     = local.acls
  resource_name                = kafka_topic.second[each.key].name
  resource_type                = "Topic"
  resource_pattern_type_filter = "Literal"
  acl_principal                = "User:refresh-bench-${each.value}"
  acl_host                     = "*"
  acl_operation                = "Read"
  acl_permission_type          = "Allow"
}
