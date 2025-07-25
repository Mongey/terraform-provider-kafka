---
page_title: "AWS MSK Integration Guide"
subcategory: ""
description: |-
  Complete guide for using the Kafka provider with Amazon Managed Streaming for Apache Kafka (MSK).
---

# AWS MSK Integration Guide

This guide provides comprehensive examples and best practices for using the Terraform Kafka provider with Amazon Managed Streaming for Apache Kafka (MSK).

## Table of Contents

- [MSK Cluster Types](#msk-cluster-types)
- [IAM Authentication](#iam-authentication)
- [SASL/SCRAM Authentication](#saslscram-authentication)
- [mTLS Authentication](#mtls-authentication)
- [Complete Examples](#complete-examples)
- [MSK Serverless](#msk-serverless)
- [Troubleshooting](#troubleshooting)

## MSK Cluster Types

### MSK Provisioned

Standard MSK clusters with dedicated broker instances.

```hcl
# Example MSK provisioned cluster endpoint
provider "kafka" {
  bootstrap_servers = [
    "b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098",
    "b-2.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098",
    "b-3.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"
  ]
  # ... authentication config
}
```

### MSK Serverless

Serverless MSK clusters with automatic scaling.

```hcl
# MSK Serverless endpoint
provider "kafka" {
  bootstrap_servers = ["boot-xxx.c2.kafka-serverless.us-east-1.amazonaws.com:9098"]
  # ... authentication config
}
```

## IAM Authentication

IAM is the recommended authentication method for MSK.

### Basic IAM Setup

```hcl
# Using default AWS credentials chain
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
}
```

### IAM with AssumeRole

```hcl
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_role_arn = "arn:aws:iam::123456789012:role/msk-terraform-role"
  sasl_aws_external_id = "unique-external-id"  # Optional
}
```

### IAM with Named Profile

```hcl
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_profile = "production"
}
```

### IAM Policy Example

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "kafka-cluster:Connect",
        "kafka-cluster:AlterCluster",
        "kafka-cluster:DescribeCluster"
      ],
      "Resource": [
        "arn:aws:kafka:us-east-1:123456789012:cluster/my-cluster/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "kafka-cluster:*Topic*",
        "kafka-cluster:ReadData",
        "kafka-cluster:WriteData"
      ],
      "Resource": [
        "arn:aws:kafka:us-east-1:123456789012:topic/my-cluster/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": [
        "kafka-cluster:AlterGroup",
        "kafka-cluster:DescribeGroup"
      ],
      "Resource": [
        "arn:aws:kafka:us-east-1:123456789012:group/my-cluster/*"
      ]
    }
  ]
}
```

### ECS/EKS Container Authentication

```hcl
# ECS Task with IAM role
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  # Credentials automatically obtained from task role
}
```

```hcl
# EKS Pod Identity
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_container_authorization_token_file = "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
  sasl_aws_container_credentials_full_uri     = "http://169.254.170.23/v1/credentials"
}
```

## SASL/SCRAM Authentication

MSK supports SCRAM-SHA-512 authentication with credentials stored in AWS Secrets Manager.

### SCRAM Setup

```hcl
# First, create the secret in AWS Secrets Manager
resource "aws_secretsmanager_secret" "kafka_credentials" {
  name = "AmazonMSK_kafka_user"
}

resource "aws_secretsmanager_secret_version" "kafka_credentials" {
  secret_id = aws_secretsmanager_secret.kafka_credentials.id
  secret_string = jsonencode({
    username = "terraform-user"
    password = random_password.kafka.result
  })
}

resource "random_password" "kafka" {
  length  = 32
  special = true
}

# Configure provider
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "scram-sha512"
  sasl_username    = "terraform-user"
  sasl_password    = random_password.kafka.result
}
```

### Associate Secret with MSK

```hcl
# Note: This is typically done through AWS Console or AWS CLI
# as Terraform AWS provider doesn't directly support this
```

## mTLS Authentication

For MSK clusters with mTLS enabled.

### mTLS Configuration

```hcl
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9094"]
  tls_enabled      = true
  ca_cert          = data.aws_msk_cluster.cluster.certificate_authority_arns[0]
  client_cert      = file("client-cert.pem")
  client_key       = file("client-key.pem")
}
```

## Complete Examples

### Example 1: Production Setup with IAM

```hcl
terraform {
  required_providers {
    kafka = {
      source  = "Mongey/kafka"
      version = "~> 0.7"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Data source for existing MSK cluster
data "aws_msk_cluster" "main" {
  cluster_name = "production-kafka-cluster"
}

# Configure Kafka provider
provider "kafka" {
  bootstrap_servers = split(",", data.aws_msk_cluster.main.bootstrap_brokers_sasl_iam)
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_role_arn = "arn:aws:iam::123456789012:role/kafka-terraform-role"
}

# Create topics
resource "kafka_topic" "events" {
  name               = "application-events"
  replication_factor = 3
  partitions         = 50
  
  config = {
    "retention.ms"        = "604800000"  # 7 days
    "compression.type"    = "lz4"
    "min.insync.replicas" = "2"
  }
}

# Set up quotas
resource "kafka_quota" "default" {
  entity_type = "user"
  
  config = {
    "producer_byte_rate" = "10485760"  # 10 MB/s
    "consumer_byte_rate" = "20971520"  # 20 MB/s
    "request_percentage" = "200"
  }
}
```

### Example 2: Multi-Environment Setup

```hcl
locals {
  environment = terraform.workspace
  
  msk_clusters = {
    dev = {
      bootstrap_servers = ["b-1.dev-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
      role_arn         = "arn:aws:iam::123456789012:role/dev-kafka-role"
    }
    staging = {
      bootstrap_servers = ["b-1.staging-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
      role_arn         = "arn:aws:iam::123456789012:role/staging-kafka-role"
    }
    prod = {
      bootstrap_servers = ["b-1.prod-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
      role_arn         = "arn:aws:iam::123456789012:role/prod-kafka-role"
    }
  }
}

provider "kafka" {
  bootstrap_servers = local.msk_clusters[local.environment].bootstrap_servers
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_role_arn = local.msk_clusters[local.environment].role_arn
}
```

### Example 3: MSK with VPC Endpoints

```hcl
# When accessing MSK from outside the VPC
provider "kafka" {
  bootstrap_servers = [
    "b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098",
    "b-2.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"
  ]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  timeout          = 120  # Increase timeout for cross-VPC connections
}
```

## MSK Serverless

MSK Serverless has some specific considerations.

### Serverless Configuration

```hcl
provider "kafka" {
  bootstrap_servers = ["boot-xxx.c2.kafka-serverless.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
}

# Note: MSK Serverless has some limitations
resource "kafka_topic" "serverless_topic" {
  name               = "events"
  replication_factor = 3  # Always 3 for serverless
  partitions         = 100 # Max 2000 for serverless
  
  config = {
    "retention.ms" = "604800000"  # 7 days max for serverless
    # Note: segment.bytes is not configurable in MSK Serverless
  }
}
```

### Serverless IAM Policy

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "kafka-cluster:Connect",
      "Resource": "arn:aws:kafka:*:123456789012:cluster/*/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "kafka-cluster:DescribeTopic",
        "kafka-cluster:CreateTopic",
        "kafka-cluster:WriteData",
        "kafka-cluster:ReadData"
      ],
      "Resource": "arn:aws:kafka:*:123456789012:topic/*/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "kafka-cluster:AlterGroup",
        "kafka-cluster:DescribeGroup"
      ],
      "Resource": "arn:aws:kafka:*:123456789012:group/*/*"
    }
  ]
}
```

## Monitoring and Observability

### CloudWatch Integration

```hcl
# Create topics with monitoring in mind
resource "kafka_topic" "metrics" {
  name               = "application-metrics"
  replication_factor = 3
  partitions         = 10
  
  config = {
    "retention.ms"     = "86400000"  # 1 day for metrics
    "compression.type" = "lz4"
  }
}

# Note: Configure CloudWatch metrics in MSK console or via AWS provider
```

## Troubleshooting

### Common Issues

#### 1. Authentication Failures

```hcl
# Enable debug logging
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_creds_debug = true  # Enable IAM debug logs
}
```

#### 2. Connection Timeouts

```hcl
# Increase timeout for cross-region or VPC peering scenarios
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  timeout          = 300  # 5 minutes
  # ... other config
}
```

#### 3. Security Group Issues

Ensure your security groups allow:
- Port 9098 for IAM authentication
- Port 9096 for SASL/SCRAM
- Port 9094 for mTLS
- Port 9092 for PLAINTEXT (not recommended)

#### 4. Cross-Account Access

```hcl
# Provider configuration for cross-account access
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_role_arn = "arn:aws:iam::987654321098:role/cross-account-kafka-role"
  sasl_aws_external_id = "unique-external-id"
}
```

### Debugging Commands

```bash
# Test connectivity
nc -zv b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com 9098

# Check AWS credentials
aws sts get-caller-identity

# List MSK clusters
aws kafka list-clusters --region us-east-1

# Get bootstrap brokers
aws kafka get-bootstrap-brokers --cluster-arn arn:aws:kafka:us-east-1:123456789012:cluster/my-cluster/xxx
```

## Best Practices

1. **Use IAM Authentication**: It's the most secure and AWS-native approach
2. **Enable Encryption in Transit**: Always use TLS
3. **Set Appropriate Quotas**: Prevent runaway clients from impacting the cluster
4. **Monitor CloudWatch Metrics**: Track broker CPU, disk usage, and network metrics
5. **Use MSK Configuration**: Store Kafka configuration parameters in MSK Configuration
6. **Implement Least Privilege**: Grant only necessary permissions in IAM policies
7. **Use Tags**: Tag your MSK resources for cost allocation and organization
8. **Plan for Scaling**: MSK supports automatic scaling based on metrics

## Migration to MSK

### From Self-Managed Kafka

```hcl
# Step 1: Set up MSK cluster with same configuration
# Step 2: Use MirrorMaker 2.0 or Kafka Connect to replicate data
# Step 3: Update Terraform configuration

# Old configuration (self-managed)
provider "kafka" {
  bootstrap_servers = ["kafka1.example.com:9092", "kafka2.example.com:9092"]
  # ... auth config
}

# New configuration (MSK)
provider "kafka" {
  bootstrap_servers = ["b-1.my-cluster.xxx.c2.kafka.us-east-1.amazonaws.com:9098"]
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
}
```

### From MSK with SASL/SCRAM to IAM

```hcl
# Step 1: Enable IAM auth on MSK cluster
# Step 2: Update provider configuration
# Step 3: Update client applications
# Step 4: Disable SASL/SCRAM after migration
```