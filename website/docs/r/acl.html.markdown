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
  ca_cert      = file("../secrets/ca.crt")
  client_cert  = file("../secrets/terraform-cert.pem")
  client_key   = file("../secrets/terraform.pem")
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
* `resource_pattern_type_filter` - (Optional) The pattern filter. Valid values
  are `Prefixed`, `Any`, `Match`, `Literal`. Default `Literal`.
* `acl_principal` - (Required) Principal that is being allowed or denied.
* `acl_host` - (Required) Host from which principal listed in `acl_principal`
  will have access.
* `acl_operation` - (Required) Operation that is being allowed or denied. Valid
  values are `Unknown`, `Any`, `All`, `Read`, `Write`, `Create`, `Delete`, `Alter`,
  `Describe`, `ClusterAction`, `DescribeConfigs`, `AlterConfigs`, `IdempotentWrite`.
* `acl_permission_type` - (Required) Type of permission. Valid values are `Unknown`,
  `Any`, `Allow`, `Deny`.

## Import

ACLs can be imported using the following pattern

```
$ terraform import kafka_acl.test "acl_principal|acl_host|acl_operation|acl_permission_type|resource_type|resource_name|resource_pattern_type_filter"
```
e.g.
```
$ terraform import kafka_acl.test "User:Alice|*|Write|Deny|Topic|syslog|Prefixed"
```
