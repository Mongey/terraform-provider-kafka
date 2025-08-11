---
page_title: "Troubleshooting Guide"
subcategory: ""
description: |-
  Common issues and solutions when using the Kafka provider.
---

# Troubleshooting Guide

This guide covers common issues encountered when using the Terraform Kafka provider and their solutions.

## Table of Contents

- [Provider Crashes and Errors](#provider-crashes-and-errors)
- [AWS MSK Authentication Issues](#aws-msk-authentication-issues)
- [Topic Configuration Problems](#topic-configuration-problems)
- [Connection and Timeout Issues](#connection-and-timeout-issues)
- [Container Environment Issues](#container-environment-issues)
- [Resource-Specific Issues](#resource-specific-issues)

## Provider Crashes and Errors

### "Empty Summary" Error When Modifying Topics

**Symptom:**
```
Error: Empty Summary: This is always a bug in the provider and should be reported to the provider developers.
```

**Common Causes:**
1. **Insufficient IAM permissions** when using AWS MSK with IAM authentication
2. **Attempting to modify immutable properties** on MSK Serverless

**Solutions:**

For IAM permission issues, ensure your IAM policy includes all necessary permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["kafka:*"],
      "Resource": "arn:aws:kafka:region:account-id:/*"
    },
    {
      "Action": ["kafka-cluster:*"],
      "Effect": "Allow",
      "Resource": [
        "arn:aws:kafka:region:account-id:cluster/cluster-name/*",
        "arn:aws:kafka:region:account-id:topic/cluster-name/*/*/*",
        "arn:aws:kafka:region:account-id:group/cluster-name/*/*/*"
      ]
    }
  ]
}
```

For MSK Serverless, note that `cleanup.policy` can only be set at topic creation time and cannot be modified afterward.

### Provider Crashes with Nil Pointer Dereference

**Symptom:**
```
panic: runtime error: invalid memory address or nil pointer dereference
```

**Cause:** Empty `bootstrap_servers` list when conditionally deploying Kafka resources

**Solution:** Ensure bootstrap_servers is never empty. If conditionally deploying:

```hcl
# Bad - causes crash
provider "kafka" {
  bootstrap_servers = local.kafka_enabled ? local.brokers : []
}

# Good - use count or for_each on resources instead
provider "kafka" {
  bootstrap_servers = local.brokers  # Always provide valid brokers
}

resource "kafka_topic" "example" {
  count = local.kafka_enabled ? 1 : 0
  # ... configuration
}
```

## AWS MSK Authentication Issues

### "The client is not authorized to access this topic"

**Common Causes:**
1. Missing or incorrect IAM permissions
2. Wrong authentication mechanism for the port
3. Security group blocking connections

**Solutions:**

1. **Verify the correct port for your authentication method:**
   - Port 9098: IAM authentication
   - Port 9096: SASL/SCRAM
   - Port 9094: mTLS
   - Port 9092: PLAINTEXT (not recommended)

2. **Check IAM permissions match the resource ARNs:**
   ```hcl
   # Get the correct cluster ARN
   data "aws_msk_cluster" "example" {
     cluster_name = "my-cluster"
   }
   
   # Use in IAM policy
   resource "aws_iam_policy" "kafka" {
     policy = jsonencode({
       Version = "2012-10-17"
       Statement = [{
         Effect = "Allow"
         Action = ["kafka-cluster:*"]
         Resource = [
           data.aws_msk_cluster.example.arn,
           "${data.aws_msk_cluster.example.arn}/*"
         ]
       }]
     })
   }
   ```

### EKS/ECS Container Authentication Failures

**Symptom:** Authentication fails when running in EKS or ECS, but works locally

**Cause:** The provider may be picking up `AWS_ROLE_ARN` environment variable set by the container

**Solution:** Explicitly set `sasl_aws_role_arn` to empty string to use the pod/task role:

```hcl
provider "kafka" {
  bootstrap_servers = split(",", data.aws_msk_cluster.example.bootstrap_brokers_sasl_iam)
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_role_arn = ""  # Force use of container credentials
}
```

For EKS with Pod Identity:
```hcl
provider "kafka" {
  bootstrap_servers = split(",", data.aws_msk_cluster.example.bootstrap_brokers_sasl_iam)
  tls_enabled      = true
  sasl_mechanism   = "aws-iam"
  sasl_aws_region  = "us-east-1"
  sasl_aws_container_authorization_token_file = "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"
  sasl_aws_container_credentials_full_uri     = "http://169.254.170.23/v1/credentials"
}
```

### Cross-Account Access Issues

**Solution:** Use external ID for secure cross-account access:

```hcl
provider "kafka" {
  bootstrap_servers    = split(",", data.aws_msk_cluster.example.bootstrap_brokers_sasl_iam)
  tls_enabled         = true
  sasl_mechanism      = "aws-iam"
  sasl_aws_region     = "us-east-1"
  sasl_aws_role_arn   = "arn:aws:iam::987654321098:role/cross-account-kafka-role"
  sasl_aws_external_id = "unique-external-id"
}
```

## Topic Configuration Problems

### Topic Names with Dots Being Replaced

**Symptom:** Topic name `msghub.callback.mpm` becomes `msghub_callback_mpm`

**Status:** This is a known issue (#452) under investigation

**Workaround:** Use underscores or hyphens instead of dots in topic names

### Cannot Modify Topic Config on MSK Serverless

**Symptom:** Error when trying to update topic configuration like `cleanup.policy`

**Cause:** MSK Serverless has limitations on which configurations can be modified after creation

**Solution:** For MSK Serverless, certain properties like `cleanup.policy` must be set correctly at creation time and cannot be changed. Plan your topic configuration carefully before creation.

### Resource Pattern Type Filter Ignored

**Symptom:** Setting `resource_pattern_type_filter = "Prefixed"` results in `"Literal"` being used

**Solution:** Ensure you're using a recent version of the provider (>= 0.8.0) which includes fixes for pattern type handling

## Connection and Timeout Issues

### "Client has run out of available brokers"

**Common Causes:**
1. Incorrect broker addresses
2. Security group blocking connections
3. Using private IPs from outside the VPC
4. DNS resolution issues

**Solutions:**

1. **Verify connectivity:**
   ```bash
   # Test connection to broker
   nc -zv broker-address.amazonaws.com 9098
   
   # Check DNS resolution
   nslookup broker-address.amazonaws.com
   ```

2. **Increase timeout for slow connections:**
   ```hcl
   provider "kafka" {
     bootstrap_servers = var.brokers
     timeout          = 300  # 5 minutes
     # ... other config
   }
   ```

3. **For cross-VPC or VPN connections:**
   - Ensure VPC peering or VPN is properly configured
   - Check route tables include routes to MSK VPC
   - Verify security groups allow traffic from your source

### "Too many colons in address" Error

**Cause:** Incorrect format of bootstrap_servers

**Solution:** Ensure proper list format:
```hcl
# Correct
bootstrap_servers = ["broker1:9092", "broker2:9092"]

# Incorrect - will cause error
bootstrap_servers = ["broker1:9092,broker2:9092"]
```

## Container Environment Issues

### Terraform Running in Atlantis/CI/CD

**Special Considerations:**
1. Ensure the container has network access to Kafka brokers
2. Use IAM roles for service accounts (IRSA) in EKS
3. Use task roles in ECS
4. Set appropriate timeouts for CI/CD environments

Example for GitLab CI with AWS:
```yaml
.terraform:
  image: hashicorp/terraform:latest
  variables:
    AWS_REGION: us-east-1
    AWS_ROLE_ARN: arn:aws:iam::123456789012:role/ci-kafka-role
  before_script:
    - apk add --no-cache aws-cli
    - aws sts assume-role --role-arn $AWS_ROLE_ARN --role-session-name ci-job > creds.json
    - export AWS_ACCESS_KEY_ID=$(jq -r .Credentials.AccessKeyId creds.json)
    - export AWS_SECRET_ACCESS_KEY=$(jq -r .Credentials.SecretAccessKey creds.json)
    - export AWS_SESSION_TOKEN=$(jq -r .Credentials.SessionToken creds.json)
```

## Resource-Specific Issues

### ACL Creation Failures

**Wildcard ACLs:**
```hcl
# May fail on some Kafka versions
resource "kafka_acl" "wildcard" {
  resource_name = "*"
  resource_type = "Topic"
  # ... other config
}
```

**Solution:** Ensure your Kafka version supports wildcard ACLs (>= 2.0.0)

### Quota Resource Issues

**Default Quotas:**
```hcl
# Create default quota (no entity_name)
resource "kafka_quota" "default_user" {
  entity_type = "user"
  # entity_name omitted for default
  config = {
    "consumer_byte_rate" = "1048576"
    "producer_byte_rate" = "1048576"
  }
}
```

### SCRAM Credential Import Issues

**Correct Import Format:**
```bash
# Format: username|mechanism|password
terraform import kafka_user_scram_credential.example 'myuser|SCRAM-SHA-256|mypassword'
```

## Debug Logging

Enable debug logging to troubleshoot issues:

```bash
# Terraform debug logs
export TF_LOG=DEBUG
export TF_LOG_PATH=terraform-debug.log

# Provider-specific debug
export TF_LOG_PROVIDER_KAFKA=DEBUG

# AWS IAM debug (in provider config)
provider "kafka" {
  # ... other config
  sasl_aws_creds_debug = true
}
```

## Getting Help

If you continue to experience issues:

1. Check the [GitHub Issues](https://github.com/Mongey/terraform-provider-kafka/issues) for similar problems
2. Enable debug logging and collect logs
3. Open a new issue with:
   - Terraform version
   - Provider version
   - Kafka/MSK version
   - Minimal reproducible configuration
   - Full error messages and debug logs

## Known Limitations

1. **MSK Serverless:**
   - Cannot modify `cleanup.policy` after topic creation
   - Limited configuration options compared to provisioned MSK
   - Maximum 2000 partitions per topic

2. **Provider Limitations:**
   - DelegationToken resource type not yet supported
   - Cannot use dynamic bootstrap_servers from resource outputs (Terraform limitation)
   - Some advanced Kafka 4.0+ features may not be fully supported

3. **Authentication:**
   - OAuth2 support is basic and may not work with all identity providers
   - Some SASL mechanisms may require specific Kafka versions