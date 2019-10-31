# `terraform-provider-kafka`
[![CircleCI](https://circleci.com/gh/Mongey/terraform-provider-kafka.svg?style=svg)](https://circleci.com/gh/Mongey/terraform-provider-kafka)

A [Terraform][1] plugin for managing [Apache Kafka][2].

## Contents

* [Installation](#installation)
  * [Developing](#developing)
* [`kafka` Provider](#provider-configuration)
* [Resources](#resources)
  * [`kafka_topic`](#kafka_topic)
  * [`kafka_acl`](#kafka_acl)
* [Requirements](#requirements)

## Installation

Download and extract the [latest
release](https://github.com/Mongey/terraform-provider-kafka/releases/latest) to
your [terraform plugin directory][third-party-plugins] (typically `~/.terraform.d/plugins/`)

### Developing

0. [Install go][install-go]
0. Clone repository to: `$GOPATH/src/github.com/Mongey/terraform-provider-kafka`
    ``` bash
    mkdir -p $GOPATH/src/github.com/Mongey/terraform-provider-kafka; cd $GOPATH/src/github.com/Mongey/
    git clone https://github.com/Mongey/terraform-provider-kafka.git
    ```
0. Build the provider `make build`
0. Run the tests `make test`
0. Start a TLS enabled kafka-cluster `docker-compose up`
0. Run the acceptance tests `make testacc`

## Provider Configuration

### Example

Example provider with SSL client authentication.
```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert           = file("../secrets/snakeoil-ca-1.crt")
  client_cert       = file("../secrets/kafkacat-ca1-signed.pem")
  client_key        = file("../secrets/kafkacat-raw-private-key.pem")
  skip_tls_verify   = true
}
```

| Property            | Description                                                                                                           | Default    |
| ------------------- | --------------------------------------------------------------------------------------------------------------------- | ---------- |
| `bootstrap_servers` | A list of host:port addresses that will be used to discover the full set of alive brokers                             | `Required` |
| `ca_cert`           | The CA certificate or path to a CA certificate file to validate the server's certificate.                             | `""`       |
| `client_cert`       | The client certificate or path to a file containing the client certificate -- Use for Client authentication to Kafka. | `""`       |
| `client_key`        | The private key or path to a file containing the private key that the client certificate was issued for.              | `""`       |
| `skip_tls_verify`   | Skip TLS verification.                                                                                                | `false`    |
| `tls_enabled`       | Enable communication with the Kafka Cluster over TLS.                                                                 | `false`    |
| `sasl_username`     | Username for SASL authentication.                                                                                     | `""`       |
| `sasl_password`     | Password for SASL authentication.                                                                                     | `""`       |
| `sasl_mechanism`    | Mechanism for SASL authentication. Allowed values are plain, scram-sha512 and scram-sha256                            | `plain`    |

## Resources
### `kafka_topic`

A resource for managing Kafka topics. Increases partition count without
destroying the topic.

#### Example

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

#### Properties

| Property             | Description                                    |
| -------------------- | ---------------------------------------------- |
| `name`               | The name of the topic                          |
| `partitions`         | The number of partitions the topic should have |
| `replication_factor` | The number of replicas the topic should have   |
| `config`             | A map of string k/v attributes   - see https://kafka.apache.org/documentation/#brokerconfigs              |


#### Importing Existing Topics
You can import topics with the following

```sh
terraform import kafka_topic.logs systemd_logs
```


### `kafka_acl`
A resource for managing Kafka ACLs.

#### Example

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

#### Properties

| Property                       | Description                                                        | Valid values                                                                                                                                             |
| ------------------------------ | ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `acl_host`                     | Host from which principal listed in acl_principal will have access | `*`                                                                                                                                                      |
| `acl_operation`                | Operation that is being allowed or denied                          | `Unknown`, `Any`, `All`, `Read`, `Write`, `Create`, `Delete`, `Alter`, `Describe`, `ClusterAction`, `DescribeConfigs`, `AlterConfigs`, `IdempotentWrite` |
| `acl_permission_type`          | Type of permission                                                 | `Unknown`, `Any`, `Allow`, `Deny`                                                                                                                        |
| `acl_principal`                | Principal that is being allowed or denied                          | `*`                                                                                                                                                      |
| `resource_name`                | The name of the resource                                           | `*`                                                                                                                                                      |
| `resource_type`                | The type of resource                                               | `Unknown`, `Any`, `Topic`, `Group`, `Cluster`, `TransactionalID`                                                                                         |
| `resource_pattern_type_filter` |                                                                    | `Prefixed`, `Any`, `Match`, `Literal`                                                                                                                    |


## Requirements
* [Kafka 1.0.0][3]

[1]: https://www.terraform.io
[2]: https://kafka.apache.org
[3]: https://cwiki.apache.org/confluence/display/KAFKA/KIP-117%3A+Add+a+public+AdminClient+API+for+Kafka+admin+operations
[third-party-plugins]: https://www.terraform.io/docs/configuration/providers.html#third-party-plugins
[install-go]: https://golang.org/doc/install#install
