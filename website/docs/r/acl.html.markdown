---
layout: "kafka"
page_title: "Kafka: kafka_acl"
sidebar_current: "docs-kafka-resource-acl"
description: |-
  A resource for managing Kafka ACLs.
---

# Resource: kafka_acl

A resource for managing Kafka ACLs.

## Example Usage

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert      = file("../secrets/snakeoil-ca-1.crt")
  client_cert  = file("../secrets/kafkacat-ca1-signed.pem")
  client_key   = file("../secrets/kafkacat-raw-private-key.pem")
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
```

## Argument Reference

The following arguments are supported:

* `resource_name` - (Required) The name of the resource.
* `resource_type` - (Required) The type of resource. Valid values are `Unknown`,
  `Any`, `Topic`, `Group`, `Cluster`, `TransactionalID`.
* `resource_pattern_type_filter` - (Required) The pattern filter. Valid values
  are `Prefixed`, `Any`, `Match`, `Literal`.
* `acl_principal` - (Required) Principal that is being allowed or denied.
* `acl_host` - (Required) Host from which principal listed in `acl_principal`
  will have access.
* `acl_operation` - (Required) Operation that is being allowed or denied. Valid
  values are `Unknown`, `Any`, `All`, `Read`, `Write`, `Create`, `Delete`, `Alter`,
  `Describe`, `ClusterAction`, `DescribeConfigs`, `AlterConfigs`, `IdempotentWrite`.
* `acl_permission_type` - (Required) Type of permission. Valid values are `Unknown`,
  `Any`, `Allow`, `Deny`.
