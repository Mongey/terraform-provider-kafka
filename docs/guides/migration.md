---
page_title: "Migration Guide"
subcategory: ""
description: |-
  Guide for migrating between different Kafka setups and provider versions.
---

# Migration Guide

This guide helps you migrate between different Kafka configurations, authentication methods, and handle breaking changes between provider versions.

## Table of Contents

- [Migrating to Dynamic Bootstrap Servers](#migrating-to-dynamic-bootstrap-servers)
- [Migrating Authentication Methods](#migrating-authentication-methods)
- [Handling Breaking Changes](#handling-breaking-changes)
- [MSK Migration Scenarios](#msk-migration-scenarios)

## Migrating to Dynamic Bootstrap Servers

The provider requires `bootstrap_servers` at initialization time, which can be challenging when the broker addresses are dynamically determined.

### Problem Scenario

```hcl
# This doesn't work as expected
resource "aws_msk_cluster" "kafka" {
  # ... cluster configuration
}

provider "kafka" {
  # ERROR: Cannot reference resource attributes in provider config
  bootstrap_servers = aws_msk_cluster.kafka.bootstrap_brokers_sasl_iam
}
```

### Solution 1: Two-Stage Apply

Split your Terraform into two configurations:

**Stage 1: Infrastructure (create MSK cluster)**
```hcl
# infrastructure/main.tf
resource "aws_msk_cluster" "kafka" {
  cluster_name = "my-cluster"
  # ... configuration
}

output "bootstrap_brokers" {
  value = aws_msk_cluster.kafka.bootstrap_brokers_sasl_iam
}
```

**Stage 2: Kafka Resources**
```hcl
# kafka-config/main.tf
data "terraform_remote_state" "infra" {
  backend = "s3"
  config = {
    bucket = "my-terraform-state"
    key    = "infrastructure/terraform.tfstate"
    region = "us-east-1"
  }
}

provider "kafka" {
  bootstrap_servers = split(",", data.terraform_remote_state.infra.outputs.bootstrap_brokers)
  # ... other config
}

resource "kafka_topic" "example" {
  name = "my-topic"
  # ... configuration
}
```

### Solution 2: Data Source Pattern

If the MSK cluster already exists:

```hcl
data "aws_msk_cluster" "existing" {
  cluster_name = "my-cluster"
}

provider "kafka" {
  bootstrap_servers = split(",", data.aws_msk_cluster.existing.bootstrap_brokers_sasl_iam)
  # ... other config
}
```

### Solution 3: Variable-Based Configuration

```hcl
variable "bootstrap_servers" {
  type        = list(string)
  description = "Kafka bootstrap servers"
}

provider "kafka" {
  bootstrap_servers = var.bootstrap_servers
  # ... other config
}

# Use -var or -var-file when applying
# terraform apply -var='bootstrap_servers=["broker1:9092","broker2:9092"]'
```

## Migrating Authentication Methods

### From SASL/PLAIN to SASL/SCRAM

```hcl
# Step 1: Create SCRAM credentials while still using PLAIN
provider "kafka" {
  bootstrap_servers = var.brokers
  sasl_mechanism   = "plain"
  sasl_username    = "admin"
  sasl_password    = var.admin_password
  tls_enabled      = true
}

resource "kafka_user_scram_credential" "users" {
  for_each = toset(["admin", "app-user", "consumer-user"])
  
  username        = each.value
  scram_mechanism = "SCRAM-SHA-512"
  password        = var.user_passwords[each.value]
}

# Step 2: After apply, update the provider
provider "kafka" {
  bootstrap_servers = var.brokers
  sasl_mechanism   = "scram-sha512"  # Changed
  sasl_username    = "admin"
  sasl_password    = var.admin_password
  tls_enabled      = true
}
```

### From SASL/SCRAM to AWS IAM (MSK)

```hcl
# Step 1: Enable IAM auth on MSK cluster (AWS Console or CLI)

# Step 2: Update provider configuration
# Old configuration
provider "kafka" {
  bootstrap_servers = var.msk_brokers_scram  # Port 9096
  sasl_mechanism   = "scram-sha512"
  sasl_username    = "terraform"
  sasl_password    = var.password
  tls_enabled      = true
}

# New configuration
provider "kafka" {
  bootstrap_servers = var.msk_brokers_iam  # Port 9098 (different!)
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_role_arn = var.terraform_role_arn
  tls_enabled      = true
}

# Step 3: Update ACLs if principal format changed
# From: User:terraform
# To: User:arn:aws:iam::123456789012:role/terraform-role
```

### From mTLS to SASL

```hcl
# Step 1: Extract CN from certificate for username
# openssl x509 -in client-cert.pem -noout -subject
# Subject: CN=kafka-client

# Step 2: Create SASL user
resource "kafka_user_scram_credential" "mtls_migration" {
  username        = "kafka-client"  # Match CN from cert
  scram_mechanism = "SCRAM-SHA-512"
  password        = var.new_password
}

# Step 3: Update ACLs to match new principal format
# Old mTLS principal: User:CN=kafka-client
# New SASL principal: User:kafka-client

# Step 4: Update provider
provider "kafka" {
  bootstrap_servers = var.brokers
  tls_enabled      = true
  sasl_mechanism   = "scram-sha512"
  sasl_username    = "kafka-client"
  sasl_password    = var.new_password
  # Remove client_cert and client_key
}
```

## Handling Breaking Changes

### Provider Version 0.10.2+ Empty Bootstrap Servers

**Issue**: Provider crashes with nil pointer when `bootstrap_servers` is empty

**Before (< 0.10.2)**:
```hcl
# This worked but was not recommended
provider "kafka" {
  bootstrap_servers = local.kafka_enabled ? var.brokers : []
}
```

**After (>= 0.10.2)**:
```hcl
# Always provide valid brokers
provider "kafka" {
  bootstrap_servers = var.brokers
}

# Use count on resources instead
resource "kafka_topic" "example" {
  count = local.kafka_enabled ? 1 : 0
  name  = "my-topic"
  # ... configuration
}
```

### Kafka 4.0.0 Compatibility

**Issue**: Unsupported API version errors with Kafka 4.0.0

**Solution**: Upgrade to provider version >= 0.8.0 which includes Kafka 4.0.0 support

```hcl
terraform {
  required_providers {
    kafka = {
      source  = "Mongey/kafka"
      version = "~> 0.8"  # Supports Kafka 4.0.0
    }
  }
}
```

## MSK Migration Scenarios

### Migrating from Self-Managed to MSK

```hcl
# Step 1: Set up MirrorMaker 2.0 for data replication
# (Configure outside of Terraform)

# Step 2: Create matching topics in MSK
locals {
  topics_to_migrate = ["events", "commands", "users"]
}

# MSK provider
provider "kafka" {
  alias = "msk"
  bootstrap_servers = var.msk_brokers
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  tls_enabled      = true
}

# Self-managed provider
provider "kafka" {
  alias = "self_managed"
  bootstrap_servers = var.self_managed_brokers
  # ... existing auth
}

# Get existing topic configurations
data "kafka_topic" "existing" {
  for_each = toset(local.topics_to_migrate)
  provider = kafka.self_managed
  name     = each.value
}

# Create in MSK with same configuration
resource "kafka_topic" "msk" {
  for_each = data.kafka_topic.existing
  provider = kafka.msk
  
  name               = each.value.name
  partitions         = each.value.partitions
  replication_factor = 3  # MSK best practice
  
  config = each.value.config
}
```

### MSK Provisioned to MSK Serverless

```hcl
# Important: MSK Serverless has limitations

# Topics that can migrate
resource "kafka_topic" "serverless_compatible" {
  name               = "events"
  replication_factor = 3      # Always 3 in serverless
  partitions         = 100    # Max 2000
  
  config = {
    "retention.ms"   = "604800000"  # 7 days max
    "cleanup.policy" = "delete"     # Set at creation only!
  }
}

# Topics that need adjustment
# Original provisioned topic:
# - partitions = 3000 (too many for serverless)
# - retention.ms = 7776000000 (90 days, too long)

resource "kafka_topic" "adjusted_for_serverless" {
  name               = "high-volume-events"
  replication_factor = 3
  partitions         = 2000  # Maximum for serverless
  
  config = {
    "retention.ms" = "604800000"  # Reduced to 7 days
  }
}
```

### Rolling Cluster Migration

```hcl
# Use workspace or environment-based configuration
locals {
  clusters = {
    old = {
      brokers  = var.old_cluster_brokers
      auth     = "scram-sha512"
      username = var.old_username
      password = var.old_password
    }
    new = {
      brokers  = var.new_cluster_brokers
      auth     = "aws-iam"
      role_arn = var.new_role_arn
    }
  }
  
  active_cluster = var.migration_phase == "cutover" ? "new" : "old"
}

provider "kafka" {
  bootstrap_servers = local.clusters[local.active_cluster].brokers
  sasl_mechanism   = local.clusters[local.active_cluster].auth
  
  # Conditional authentication
  sasl_username    = local.active_cluster == "old" ? local.clusters.old.username : null
  sasl_password    = local.active_cluster == "old" ? local.clusters.old.password : null
  sasl_aws_role_arn = local.active_cluster == "new" ? local.clusters.new.role_arn : null
  sasl_aws_region  = local.active_cluster == "new" ? "us-east-1" : null
  
  tls_enabled = true
}
```

## Best Practices for Migrations

1. **Always backup topic configurations** before migration
2. **Test authentication changes** in a non-production environment
3. **Plan for dual-running** during migration periods
4. **Monitor consumer lag** during data migrations
5. **Update documentation** and runbooks after migration
6. **Keep rollback plans** ready

## Getting Help

If you encounter issues during migration:

1. Check the [Troubleshooting Guide](./troubleshooting.md)
2. Review [GitHub Issues](https://github.com/Mongey/terraform-provider-kafka/issues) for similar migrations
3. Test with minimal configuration first
4. Enable debug logging during migration