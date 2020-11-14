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
  validate the server's certificate.

* `client_cert` - (Optional) The client certificate or path to a file containing
  the client certificate -- Use for Client authentication to Kafka.

* `client_key` - (Optional) The private key or path to a file containing the private
  key that the client certificate was issued for.

* `skip_tls_verify` - (Optional) Skip TLS verification. Default `false`.

* `tls_enabled` - (Optional) Enable communication with the Kafka Cluster over TLS.
  Default `false`.

* `sasl_username` - (Optional) Username for SASL authentication.

* `sasl_password` - (Optional) Password for SASL authentication.

* `sasl_mechanism` - (Optional) Mechanism for SASL authentication. Allowed values
  are `plain`, `scram-sha512` and `scram-sha256`. Default `plain`.
