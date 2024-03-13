---
layout: "kafka"
page_title: "Provider: Kafka"
sidebar_current: "docs-kafka-index"
description: |-
  A Terraform plugin for managing Apache Kafka.
---

# Kafka Provider

A Terraform plugin for managing Apache Kafka.

Use the navigation to the left to read about the available resources.

## Example Usage

```hcl
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  ca_cert           = file("../secrets/ca.crt")
  client_cert       = file("../secrets/terraform-cert.pem")
  client_key        = file("../secrets/terraform.pem")
  skip_tls_verify   = true
}
```

## Argument Reference

In addition to [generic `provider` arguments](https://www.terraform.io/docs/configuration/providers.html)
(e.g. `alias` and `version`), the following arguments are supported in the Kafka
 `provider` block:

* `bootstrap_servers` - (Required) A list of host:port addresses that will be used
  to discover the full set of alive brokers.

* `ca_cert` - (Optional) The CA certificate or path to a CA certificate file to
  validate the server's certificate. Can be set through the `KAFKA_CA_CERT` environment variable.

* `client_cert` - (Optional) The client certificate or path to a file containing
  the client certificate -- Use for Client authentication to Kafka. Can be set through the `KAFKA_CLIENT_CERT` environment variable.

* `client_key` - (Optional) The private key or path to a file containing the private
  key that the client certificate was issued for. Can be set through the `KAFKA_CLIENT_KEY` environment variable.

* `client_key_passphrase` - (Optional) The passphrase of the private key. Can be set through the `KAFKA_CLIENT_KEY_PASSPHRASE` environment variable.

* `skip_tls_verify` - (Optional) Skip TLS verification. Default `false`. Can be set through the `KAFKA_SKIP_VERIFY` environment variable.

* `tls_enabled` - (Optional) Enable communication with the Kafka Cluster over TLS.
  Default `true`. Can be set through the `KAFKA_ENABLE_TLS` environment variable.

* `sasl_username` - (Optional) Username for SASL authentication. Can be set through the `KAFKA_SASL_USERNAME` environment variable.

* `sasl_password` - (Optional) Password for SASL authentication. Can be set through the `KAFKA_SASL_PASSWORD` environment variable.

* `sasl_mechanism` - (Optional) Mechanism for SASL authentication. Allowed values
  are `plain`, `scram-sha512`, `scram-sha256` and `aws-iam`. Default `plain`. Can be set through the `KAFKA_SASL_MECHANISM` environment variable.

* `sasl_aws_region` - (Optional) AWS region where MSK is deployed. Required when sasl_mechanism is aws-iam.

* `sasl_aws_role_arn` - (Optional) IAM role ARN to Assume.

* `sasl_aws_profile` - (Optional) AWS profile name to use.

* `sasl_aws_creds_debug` - (Optional) Set this to true to turn AWS credentials debug.