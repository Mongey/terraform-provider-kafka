A [Terraform][1] plugin for managing [Apache Kafka][2].

[![CircleCI](https://circleci.com/gh/Mongey/terraform-provider-kafka.svg?style=svg)](https://circleci.com/gh/Mongey/terraform-provider-kafka)

# Requirements
* [Kafka 0.11][3]

# Example

```terraform
provider "kafka" {
  brokers = ["localhost:9092"]
}

resource "kafka_topic" "logs" {
  name               = "systemd_logs"
  replication_factor = 2
  partitions         = 100

  config = {
    "segment.ms" = "20000"
  }
}
```

# Importing Existing Topics
You can import topics with the following

```sh
terraform import kafka_topic.logs systemd_logs
```

# Resources
* kafka_topic

# Planned Resources
* kafka_acl

[1]: https://www.terraform.io
[2]: https://kafka.apache.org
[3]: https://cwiki.apache.org/confluence/display/KAFKA/KIP-117%3A+Add+a+public+AdminClient+API+for+Kafka+admin+operations
