---
page_title: "Quick Start Guide"
subcategory: ""
description: |-
  Get started quickly with common Terraform Kafka provider scenarios.
---

# Quick Start Guide

This guide provides ready-to-use configurations for common scenarios when using the Terraform Kafka provider.

## Table of Contents

- [Local Development Setup](#local-development-setup)
- [AWS MSK Quick Start](#aws-msk-quick-start)
- [Confluent Cloud Setup](#confluent-cloud-setup)
- [Common Patterns](#common-patterns)

## Local Development Setup

### Docker Compose Kafka

```hcl
# Provider configuration for local Kafka
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
}

# Create a test topic
resource "kafka_topic" "test" {
  name               = "test-events"
  replication_factor = 1
  partitions         = 3
}
```

### Local Kafka with SASL/SCRAM

```hcl
# First, create the user
resource "kafka_user_scram_credential" "admin" {
  username        = "admin"
  scram_mechanism = "SCRAM-SHA-256"
  password        = "admin-secret"
}

# Configure provider
provider "kafka" {
  bootstrap_servers = ["localhost:9092"]
  sasl_mechanism   = "scram-sha256"
  sasl_username    = "admin"
  sasl_password    = "admin-secret"
}
```

## AWS MSK Quick Start

### Minimal MSK Setup

```hcl
# Data source for existing MSK cluster
data "aws_msk_cluster" "main" {
  cluster_name = "my-kafka-cluster"
}

# Provider configuration
provider "kafka" {
  bootstrap_servers = split(",", data.aws_msk_cluster.main.bootstrap_brokers_sasl_iam)
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
}

# Create application topics
resource "kafka_topic" "events" {
  name               = "application-events"
  replication_factor = 3
  partitions         = 10
  
  config = {
    "retention.ms" = "86400000"  # 1 day
  }
}
```

### MSK with Terraform in CI/CD

```hcl
# For GitHub Actions, GitLab CI, etc.
provider "kafka" {
  bootstrap_servers = split(",", var.msk_bootstrap_brokers)
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = var.aws_region
  sasl_aws_role_arn = var.ci_role_arn  # Role assumed by CI
  timeout          = 120  # Increase for CI environments
}
```

### MSK from EKS Pod

```hcl
provider "kafka" {
  bootstrap_servers = split(",", data.aws_msk_cluster.main.bootstrap_brokers_sasl_iam)
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_role_arn = ""  # Important: empty string to use pod role
}
```

## Confluent Cloud Setup

```hcl
provider "kafka" {
  bootstrap_servers = ["pkc-12345.us-east-1.aws.confluent.cloud:9092"]
  tls_enabled      = true
  sasl_mechanism   = "plain"
  sasl_username    = var.confluent_api_key
  sasl_password    = var.confluent_api_secret
}

resource "kafka_topic" "events" {
  name               = "events"
  replication_factor = 3
  partitions         = 6
  
  config = {
    "retention.ms" = "604800000"  # 7 days
  }
}
```

## Common Patterns

### Multi-Topic Creation

```hcl
locals {
  topics = {
    events = {
      partitions = 10
      retention  = "604800000"  # 7 days
    }
    commands = {
      partitions = 5
      retention  = "86400000"   # 1 day
    }
    dlq = {
      partitions = 3
      retention  = "2592000000" # 30 days
    }
  }
}

resource "kafka_topic" "application_topics" {
  for_each = local.topics
  
  name               = each.key
  replication_factor = 3
  partitions         = each.value.partitions
  
  config = {
    "retention.ms" = each.value.retention
    "compression.type" = "lz4"
  }
}
```

### Topic with ACLs

```hcl
# Topic for user data
resource "kafka_topic" "user_data" {
  name               = "user-data"
  replication_factor = 3
  partitions         = 20
}

# Producer ACL
resource "kafka_acl" "producer" {
  resource_name       = kafka_topic.user_data.name
  resource_type       = "Topic"
  acl_principal       = "User:producer-service"
  acl_host           = "*"
  acl_operation      = "Write"
  acl_permission_type = "Allow"
}

# Consumer ACL
resource "kafka_acl" "consumer" {
  resource_name       = kafka_topic.user_data.name
  resource_type       = "Topic"
  acl_principal       = "User:consumer-service"
  acl_host           = "*"
  acl_operation      = "Read"
  acl_permission_type = "Allow"
}

# Consumer group ACL
resource "kafka_acl" "consumer_group" {
  resource_name       = "consumer-group-*"
  resource_type       = "Group"
  resource_pattern_type_filter = "Prefixed"
  acl_principal       = "User:consumer-service"
  acl_host           = "*"
  acl_operation      = "Read"
  acl_permission_type = "Allow"
}
```

### Environment-Specific Configuration

```hcl
locals {
  env_config = {
    dev = {
      replication_factor = 1
      partitions        = 3
      retention_ms      = "86400000"    # 1 day
    }
    staging = {
      replication_factor = 2
      partitions        = 10
      retention_ms      = "259200000"   # 3 days
    }
    prod = {
      replication_factor = 3
      partitions        = 50
      retention_ms      = "604800000"   # 7 days
    }
  }
  
  env = terraform.workspace
}

resource "kafka_topic" "main" {
  name               = "${local.env}-events"
  replication_factor = local.env_config[local.env].replication_factor
  partitions         = local.env_config[local.env].partitions
  
  config = {
    "retention.ms" = local.env_config[local.env].retention_ms
  }
}
```

### Quotas Configuration

```hcl
# Default quota for all users
resource "kafka_quota" "default_user" {
  entity_type = "user"
  
  config = {
    "producer_byte_rate" = "1048576"  # 1 MB/s
    "consumer_byte_rate" = "2097152"  # 2 MB/s
  }
}

# Specific quota for high-traffic service
resource "kafka_quota" "high_traffic_service" {
  entity_name = "high-traffic-service"
  entity_type = "user"
  
  config = {
    "producer_byte_rate" = "10485760" # 10 MB/s
    "consumer_byte_rate" = "20971520" # 20 MB/s
  }
}

# IP-based quota
resource "kafka_quota" "ip_quota" {
  entity_name = "10.0.1.0/24"
  entity_type = "ip"
  
  config = {
    "connection_creation_rate" = "10"
  }
}
```

### Migration Pattern

```hcl
# When migrating from one cluster to another
variable "migration_in_progress" {
  type    = bool
  default = false
}

locals {
  # Use old cluster during migration, new cluster after
  bootstrap_servers = var.migration_in_progress ? 
    var.old_cluster_brokers : 
    var.new_cluster_brokers
}

provider "kafka" {
  bootstrap_servers = local.bootstrap_servers
  # ... other config
}

# Topics exist in both clusters during migration
resource "kafka_topic" "main" {
  name               = "events"
  replication_factor = 3
  partitions         = 10
}
```

## Next Steps

1. Review the [Authentication Guide](./authentication.md) for detailed auth configuration
2. Check the [AWS MSK Integration Guide](./aws-msk-integration.md) for MSK-specific features
3. See the [Troubleshooting Guide](./troubleshooting.md) if you encounter issues
4. Explore the resource documentation for advanced configurations