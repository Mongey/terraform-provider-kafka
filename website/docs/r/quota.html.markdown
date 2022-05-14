---
layout: "kafka"
page_title: "Kafka: kafka_quota"
sidebar_current: "docs-kafka-resource-quota"
description: |-
  A resource for managing Kafka quotas.
---

# Resource: kafka_quota

A resource for managing Kafka quotas.

## Example Usage

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

resource "kafka_quota" "quota" {
  entity_name               = "app_consumer"
  entity_type               = "client-id"
  config = {
    "consumer_byte_rate" = "5000000"
	  "producer_byte_rate" = "2500000"
  }
}
```

## Argument Reference

The following arguments are supported:

* `entity_name` - (Required) The name of the entity to target.
* `entity_type` - (Required) The type of entity. Valid values are `client-id`, `user`, `ip`.
* `config` - (Optional) A map of string k/v attributes.
