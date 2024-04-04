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
  * [`kafka_quota`](#kafka_quota)
* [Requirements](#requirements)

## Installation

`terraform-provider-kafka` is available on the terraform registry. To install, add
the below into your `main.tf` and execute `terraform init`

```tf
terraform {
  required_providers {
    kafka = {
      source = "Mongey/kafka"
    }
  }
}

provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert           = file("../secrets/ca.crt")
  client_cert       = file("../secrets/terraform-cert.pem")
  client_key        = file("../secrets/terraform.pem")
  tls_enabled       = true
}
```

Otherwise, install by downloading and extracting the [latest
release](https://github.com/Mongey/terraform-provider-kafka/releases/latest) to
your [terraform plugin directory][third-party-plugins] (typically `~/.terraform.d/plugins/`)

### Developing

0. [Install go][install-go]
0. Clone repository to: `$GOPATH/src/github.com/Mongey/terraform-provider-kafka`
    ``` bash
    mkdir -p $GOPATH/src/github.com/Mongey/terraform-provider-kafka; cd $GOPATH/src/github.com/Mongey/
    git clone https://github.com/Mongey/terraform-provider-kafka.git
    cd terraform-provider-kafka
    ```
0. Build the provider `make build`
0. Run the tests `make test`
0. Start a TLS enabled kafka-cluster `docker-compose up`
0. Run the acceptance tests `make testacc`

## Provider Configuration

### Example

Example provider with TLS client authentication.
```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert           = file("../secrets/ca.crt")
  client_cert       = file("../secrets/terraform-cert.pem")
  client_key        = file("../secrets/terraform.pem")
  tls_enabled       = true
}
```

Example provider with aws-iam(Assume role) client authentication.
```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9098"]
  tls_enabled       = true
  sasl_mechanism    = "aws-iam"
  sasl_aws_region   = "us-east-1"
  sasl_aws_role_arn = "arn:aws:iam::account:role/role-name"
}
```

Example provider with aws-iam(Aws Profile) client authentication.
```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9098"]
  tls_enabled       = true
  sasl_mechanism    = "aws-iam"
  sasl_aws_region   = "us-east-1"
  sasl_aws_profile  = "dev"
}
```

Example provider with aws-iam(Static Creds) client authentication. You have to export `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`(Optional if you are using temp creds)
```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9098"]
  tls_enabled       = true
  sasl_mechanism    = "aws-iam"
  sasl_aws_region   = "us-east-1"
}
```
#### Compatibility with Redpanda

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  kafka_version = "2.1.0"
}
```

Due to Redpanda not implementing some Metadata APIs, we need to force the Kafka version to use when creating the provider.

| Property                | Description                                                                                                           | Default    |
| -------------------     | --------------------------------------------------------------------------------------------------------------------- | ---------- |
| `bootstrap_servers`     | A list of host:port addresses that will be used to discover the full set of alive brokers                             | `Required` |
| `ca_cert`               | The CA certificate or path to a CA certificate file in `PEM` format to validate the server's certificate.             | `""`       |
| `client_cert`           | The client certificate or path to a file containing the client certificate in `PEM` format. Use for Client authentication to Kafka.<br>If you have Intermediate CA certificate(s) append them to `client_cert`.| `""`       |
| `client_key`            | The private key or path to a file containing the private key that the client certificate was issued for.              | `""`       |
| `client_key_passphrase` | The passphrase for the private key that the certificate was issued for.                                               | `""`       |
| `kafka_version`         | The version of Kafka protocol to use in `$MAJOR.$MINOR.$PATCH` format. Some features may not be available on older versions.  | `""`       |
| `tls_enabled`           | Enable communication with the Kafka Cluster over TLS.                                                                 | `true`     |
| `skip_tls_verify`       | Skip TLS verification.                                                                                                | `false`    |
| `sasl_username`         | Username for SASL authentication.                                                                                     | `""`       |
| `sasl_password`         | Password for SASL authentication.                                                                                     | `""`       |
| `sasl_mechanism`        | Mechanism for SASL authentication. Allowed values are `plain`, `aws-iam`,  `scram-sha256`, `scram-sha512` or `oauthbearer`      | `plain`    |
| `sasl_aws_region`       | AWS region for IAM authentication.                                                                                    | `""`       |
| `sasl_aws_role_arn`     | Arn of AWS IAM role to assume for IAM authentication.                                                                 | `""`       |
| `sasl_aws_profile`      | AWS profile to use for IAM authentication.                                                                            | `""`       |
| `sasl_aws_creds_debug`  | Enable debug logging for AWS authentication.                                                                          | `false`    |
| `sasl_token_url`        | The url to retrieve oauth2 tokens from, when using sasl mechanism `oauthbearer`                                         | `""`    |


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
| `config`             | A map of string [K/V attributes][topic-config] |


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
  ca_cert           = file("../secrets/ca.crt")
  client_cert       = file("../secrets/terraform-cert.pem")
  client_key        = file("../secrets/terraform.pem")
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
| `acl_principal`                | Principal that is being allowed or denied                          | `*`                                                                                                                                                      |
| `acl_host`                     | Host from which principal listed in acl_principal will have access | `*`                                                                                                                                                      |
| `acl_operation`                | Operation that is being allowed or denied                          | `Unknown`, `Any`, `All`, `Read`, `Write`, `Create`, `Delete`, `Alter`, `Describe`, `ClusterAction`, `DescribeConfigs`, `AlterConfigs`, `IdempotentWrite` |
| `acl_permission_type`          | Type of permission                                                 | `Unknown`, `Any`, `Allow`, `Deny`                                                                                                                        |
| `resource_name`                | The name of the resource                                           | `*`                                                                                                                                                      |
| `resource_type`                | The type of resource                                               | `Unknown`, `Any`, `Topic`, `Group`, `Cluster`, `TransactionalID`                                                                                         |
| `resource_pattern_type_filter` |                                                                    | `Prefixed`, `Any`, `Match`, `Literal`                                                                                                                    |


#### Importing Existing ACLs
For import, use as a parameter the items separated by `|` character. Quote it to avoid shell expansion.

```sh
# Fields in shell notation are
# ${acl_principal}|${acl_host}|${acl_operation}|${acl_permission_type}|${resource_type}|${resource_name}|${resource_pattern_type_filter}
terraform import kafka_acl.admin 'User:12345|*|Describe|Allow|Topic|experimental-topic|Prefixed'
```

### `kafka_quota`
A resource for managing Kafka Quotas.

#### Example

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert           = file("../secrets/ca.crt")
  client_cert       = file("../secrets/terraform-cert.pem")
  client_key        = file("../secrets/terraform.pem")
}

resource "kafka_quota" "test" {
  entity_name       = "client1"
  entity_type       = "client-id"
  config = {
    "consumer_byte_rate" = "4000000"
    "producer_byte_rate" = "3500000"
  }
}
```

#### Properties

| Property             | Description                                    |
| -------------------- | ---------------------------------------------- |
| `entity_name`        | The name of the entity                         |
| `entity_type`        | The entity type (client-id, user, ip)          |
| `config`             | A map of string attributes for the entity      |

### `kafka_user_scram_credential`
A resource for managing Kafka SCRAM user credentials.

#### Example

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert           = file("../secrets/ca.crt")
  client_cert       = file("../secrets/terraform-cert.pem")
  client_key        = file("../secrets/terraform.pem")
}

resource "kafka_user_scram_credential" "test" {
  username               = "user1"
  scram_mechanism        = "SCRAM-SHA-256"
  scram_iterations       = "8192"
  password               = "password"
}
```

#### Properties

| Property             | Description                                    |
| -------------------- | ---------------------------------------------- |
| `username`        | The username                         |
| `scram_mechanism`        | The SCRAM mechanism (SCRAM-SHA-256 or SCRAM-SHA-512)          |
| `scram_iterations`             | The number of SCRAM iterations (must be >= 4096). Default: 4096       |
| `password` | The password for the user |

## Requirements
* [>= Kafka 1.0.0][3]

[1]: https://www.terraform.io
[2]: https://kafka.apache.org
[3]: https://cwiki.apache.org/confluence/display/KAFKA/KIP-117%3A+Add+a+public+AdminClient+API+for+Kafka+admin+operations
[third-party-plugins]: https://www.terraform.io/docs/configuration/providers.html#third-party-plugins
[install-go]: https://golang.org/doc/install#install
[topic-config]: https://kafka.apache.org/documentation/#topicconfigs 
