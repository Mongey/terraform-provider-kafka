---
layout: "kafka"
page_title: "Kafka: kafka_topic"
sidebar_current: "docs-kafka-resource-topic"
description: |-
  A resource for managing Kafka topics. Increases partition count without destroying the topic.
---

# Resource: kafka_topic

A resource for managing Kafka topics. Increases partition count without destroying the topic.

## Example Usage

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

resource "kafka_topic" "logs" {
  name               = "systemd_logs"
  replication_factor = 2
  partitions         = 100

  config = {
    "segment.ms"     = "20000"
    "cleanup.policy" = "compact"
  }
}
```

## Argument Reference

The following arguments are supported:

* `name` - (Required) The name of the topic.
* `partitions` - (Required) The number of partitions the topic should have.
* `replication_factor` - (Required) The number of replicas the topic should have.
* `config` - (Optional) A map of string k/v attributes.

## Import

Topics can be imported using their ARN, e.g.

```
$ terraform import kafka_topic.logs systemd_logs
```
