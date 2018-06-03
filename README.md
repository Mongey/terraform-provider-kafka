A [Terraform][1] plugin for managing [Apache Kafka][2].

[![CircleCI](https://circleci.com/gh/Mongey/terraform-provider-kafka.svg?style=svg)](https://circleci.com/gh/Mongey/terraform-provider-kafka)

# Requirements
* [Kafka 1.0.0][3]

# Example

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
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

# Provider Configuration
| Property            | Description                                                                                      | Default    |
| ----------------    | -----------------------                                                                          | ---------- |
| `bootstrap_servers` | A list of host:port addresses that will be used to discover the full set of alive brokers        | `Required` |
| `ca_cert_file`      | The path to a CA certificate file to validate the server's certificate.                          | `""`       |
| `client_cert_file`  | The path the a file containing the client certificate -- Use for Client authentication to Kafka. | `""`       |
| `client_key_file`   | Path to a file containing the private key that the client certificate was issued for.            | `""`       |
| `skip_tls_verify`   | Skip TLS verification.                                                                           | `false`    |
| `tls_enabled`       | Enable communication with the Kafka Cluster over TLS.                                            | `false`    |

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
