# CDK Bootstrap Guide

## Overview

AWS CDK bootstrapping is the process of preparing your AWS environment for CDK deployments. This guide provides comprehensive information about CDK bootstrap requirements, especially for multi-stack applications like Brain2.

## Table of Contents

- [What is CDK Bootstrap?](#what-is-cdk-bootstrap)
- [Bootstrap Requirements](#bootstrap-requirements)
- [Multi-Stack Considerations](#multi-stack-considerations)
- [Bootstrap Procedures](#bootstrap-procedures)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)
- [Advanced Configuration](#advanced-configuration)

## What is CDK Bootstrap?

### Definition

CDK bootstrapping creates the necessary AWS resources that the CDK needs to deploy your stacks. These resources include:

- **S3 Bucket**: Stores CDK assets (Lambda code, Docker images, etc.)
- **ECR Repository**: Stores Docker images for container-based applications
- **IAM Roles**: Provides permissions for CDK operations
- **CloudFormation Stack**: Manages the bootstrap resources (`CDKToolkit`)

### Why Bootstrap is Needed

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Your CDK      │    │   Bootstrap     │    │   Target AWS    │
│   Application   │ ──▶│   Resources     │ ──▶│   Resources     │
│                 │    │   (S3, IAM,     │    │   (Your App)    │
│                 │    │    ECR)         │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

1. **Asset Upload**: CDK needs a place to upload Lambda code and other assets
2. **Permissions**: CDK needs IAM roles to create and manage resources
3. **Container Images**: If using containers, CDK needs ECR for image storage
4. **CloudFormation**: CDK uses CloudFormation for deployments

## Bootstrap Requirements

### When Bootstrap is Required

Bootstrap is required when:
- ✅ **First CDK deployment** in an AWS account/region
- ✅ **Lambda functions** are included in stacks
- ✅ **Docker containers** are used
- ✅ **Large CloudFormation templates** (>51,200 bytes)
- ✅ **CDK Pipelines** are used
- ✅ **Cross-account deployments** are needed

### When Bootstrap May Not Be Required

Bootstrap is not required for:
- ❌ **Simple stacks** with only basic resources (S3, DynamoDB)
- ❌ **No Lambda functions** or containers
- ❌ **Small CloudFormation templates**

### Brain2 Requirements

For the Brain2 application, bootstrap **IS REQUIRED** because:
- ✅ Multiple Lambda functions (Go and Node.js)
- ✅ Lambda code assets need S3 storage
- ✅ Multi-stack architecture
- ✅ Cross-stack references
- ✅ EventBridge and API Gateway integrations

## Multi-Stack Considerations

### Per-Environment Bootstrap

Each AWS environment must be bootstrapped separately:

```bash
# Different AWS accounts
Account A: 123456789012  # Development
Account B: 987654321098  # Production

# Different regions in same account
us-east-1    # Primary region
us-west-2    # DR region
```

### Bootstrap Scope

Bootstrap applies to:
- **AWS Account + Region combination**
- **All CDK applications** in that account/region
- **All stacks** within applications

### Brain2 Multi-Stack Architecture

```
Bootstrap Resources (once per account/region)
├── CDKToolkit Stack
│   ├── S3 Bucket (cdk-assets-123456789012-us-east-1)
│   ├── ECR Repository (cdk-assets-123456789012-us-east-1)
│   └── IAM Roles
│
└── Brain2 Application Stacks (use bootstrap resources)
    ├── Brain2Stack/Database
    ├── Brain2Stack/Compute
    ├── Brain2Stack/Api
    └── Brain2Stack/Frontend
```

## Bootstrap Procedures

### Basic Bootstrap

#### 1. Check Bootstrap Status

```bash
# Check if environment is already bootstrapped
cdk bootstrap --show-template

# List existing CloudFormation stacks
aws cloudformation list-stacks --query 'StackSummaries[?StackName==`CDKToolkit`]'
```

#### 2. Basic Bootstrap Command

```bash
# Basic bootstrap (development environments)
cdk bootstrap

# With explicit account and region
cdk bootstrap aws://123456789012/us-east-1
```

#### 3. Verify Bootstrap

```bash
# Check that CDKToolkit stack exists
aws cloudformation describe-stacks --stack-name CDKToolkit

# Verify S3 bucket exists
aws s3 ls | grep cdk-assets

# Verify IAM roles exist
aws iam list-roles --query 'Roles[?contains(RoleName, `cdk-`)]'
```

### Production Bootstrap

#### Enhanced Bootstrap with Security

```bash
# Production bootstrap with enhanced security
cdk bootstrap \
  --termination-protection \
  --cloudformation-execution-policies arn:aws:iam::aws:policy/AdministratorAccess \
  --trust-accounts 123456789012,987654321098 \
  --qualifier prod
```

**Parameters Explained:**
- `--termination-protection`: Prevents accidental deletion
- `--cloudformation-execution-policies`: Defines permissions for deployments
- `--trust-accounts`: Allows cross-account deployments
- `--qualifier`: Unique identifier for this bootstrap

#### Custom Bootstrap Template

```bash
# Use custom bootstrap template (advanced)
cdk bootstrap \
  --template bootstrap-template.yaml \
  --parameters S3BucketName=my-custom-cdk-assets-bucket
```

### Environment-Specific Bootstrap

#### Development Environment

```bash
# Development bootstrap (simple)
export AWS_PROFILE=development
cdk bootstrap --profile development
```

#### Staging Environment

```bash
# Staging bootstrap (enhanced monitoring)
export AWS_PROFILE=staging
cdk bootstrap \
  --termination-protection \
  --profile staging
```

#### Production Environment

```bash
# Production bootstrap (maximum security)
export AWS_PROFILE=production
cdk bootstrap \
  --termination-protection \
  --cloudformation-execution-policies arn:aws:iam::aws:policy/PowerUserAccess \
  --trust-accounts 123456789012 \
  --qualifier prod \
  --profile production
```

## Troubleshooting

### Issue: "This stack uses assets, so the toolkit stack must be deployed"

**Problem:** Stack contains Lambda functions or other assets but environment isn't bootstrapped

**Solution:**
```bash
# Bootstrap the environment
cdk bootstrap

# Then deploy your stacks
cdk deploy
```

### Issue: "Access Denied" During Bootstrap

**Problem:** Insufficient permissions to create bootstrap resources

**Solutions:**

1. **Check IAM Permissions**
   ```bash
   # Verify current identity
   aws sts get-caller-identity
   
   # Required permissions for bootstrap:
   # - CloudFormation: Full access
   # - S3: Create bucket, put objects
   # - IAM: Create roles and policies
   # - ECR: Create repository
   ```

2. **Use Admin Permissions** (temporarily)
   ```bash
   # Ensure you have AdministratorAccess policy
   aws iam list-attached-user-policies --user-name $(aws sts get-caller-identity --query 'Arn' --output text | cut -d'/' -f2)
   ```

### Issue: "CDKToolkit stack already exists but is different"

**Problem:** Existing bootstrap stack doesn't match current requirements

**Solutions:**

1. **Update Bootstrap Stack**
   ```bash
   # Update existing bootstrap
   cdk bootstrap --force
   ```

2. **Manual Stack Update** (if needed)
   ```bash
   # Check current bootstrap stack
   aws cloudformation describe-stacks --stack-name CDKToolkit
   
   # Update through CloudFormation if necessary
   ```

### Issue: "Bootstrap bucket access denied"

**Problem:** CDK can't access the bootstrap S3 bucket

**Solutions:**

1. **Check Bucket Policy**
   ```bash
   # Verify bucket exists and is accessible
   aws s3 ls s3://cdk-assets-$(aws sts get-caller-identity --query 'Account' --output text)-$(aws configure get region)
   ```

2. **Re-bootstrap** (if bucket corrupted)
   ```bash
   # Re-bootstrap to fix bucket issues
   cdk bootstrap --force
   ```

### Issue: "ECR repository not found"

**Problem:** Container deployments fail due to missing ECR repository

**Solutions:**

1. **Verify ECR Repository**
   ```bash
   # Check if ECR repository exists
   aws ecr describe-repositories --repository-names cdk-assets-$(aws sts get-caller-identity --query 'Account' --output text)-$(aws configure get region)
   ```

2. **Re-bootstrap with Container Support**
   ```bash
   # Ensure container support is enabled
   cdk bootstrap --force
   ```

## Best Practices

### Security

1. **Use Termination Protection**
   ```bash
   # Always use termination protection in production
   cdk bootstrap --termination-protection
   ```

2. **Least Privilege Policies**
   ```bash
   # Use specific policies instead of AdministratorAccess
   cdk bootstrap --cloudformation-execution-policies arn:aws:iam::aws:policy/PowerUserAccess
   ```

3. **Cross-Account Trust**
   ```bash
   # Only trust necessary accounts
   cdk bootstrap --trust-accounts 123456789012,987654321098
   ```

### Organization

1. **Use Qualifiers for Separation**
   ```bash
   # Separate bootstrap resources by environment
   cdk bootstrap --qualifier dev    # Development
   cdk bootstrap --qualifier prod   # Production
   ```

2. **Consistent Naming**
   ```bash
   # Use consistent qualifier naming across environments
   # dev, staging, prod (not development, stage, production)
   ```

### Maintenance

1. **Regular Updates**
   ```bash
   # Periodically update bootstrap stack
   cdk bootstrap --force
   ```

2. **Monitor Costs**
   ```bash
   # Monitor S3 bucket costs for asset storage
   aws s3 ls s3://cdk-assets-* --recursive --summarize
   ```

3. **Cleanup Old Assets**
   ```bash
   # CDK doesn't automatically clean old assets
   # Consider lifecycle policies on S3 bucket
   ```

## Advanced Configuration

### Custom S3 Bucket

```bash
# Use existing S3 bucket for assets
cdk bootstrap --toolkit-bucket-name my-existing-cdk-bucket
```

### Custom KMS Key

```bash
# Use custom KMS key for encryption
cdk bootstrap --bootstrap-kms-key-id arn:aws:kms:region:account:key/key-id
```

### Custom IAM Policies

Create custom bootstrap template:

```yaml
# bootstrap-template.yaml
Parameters:
  CloudFormationExecutionPolicies:
    Type: CommaDelimitedList
    Default: "arn:aws:iam::aws:policy/PowerUserAccess"

Resources:
  # Custom bootstrap resources
  FileAssetsBucketEncryptionKey:
    Type: AWS::KMS::Key
    Properties:
      KeyPolicy:
        Statement:
          - Sid: Enable IAM User Permissions
            Effect: Allow
            Principal:
              AWS: !Sub 'arn:aws:iam::${AWS::AccountId}:root'
            Action: 'kms:*'
            Resource: '*'
```

### Multi-Region Bootstrap

```bash
#!/bin/bash
# Bootstrap multiple regions
REGIONS=("us-east-1" "us-west-2" "eu-west-1")
ACCOUNT=$(aws sts get-caller-identity --query 'Account' --output text)

for region in "${REGIONS[@]}"; do
  echo "Bootstrapping $region..."
  cdk bootstrap aws://$ACCOUNT/$region --termination-protection
done
```

### Organization-Wide Bootstrap

For AWS Organizations:

```bash
# Bootstrap all accounts in organization
# (Requires OrganizationAccountAccessRole in each account)

for account in $(aws organizations list-accounts --query 'Accounts[].Id' --output text); do
  aws sts assume-role --role-arn "arn:aws:iam::$account:role/OrganizationAccountAccessRole" --role-session-name bootstrap-session
  # Use temporary credentials to bootstrap each account
  cdk bootstrap aws://$account/us-east-1
done
```

## Verification Checklist

After bootstrap, verify:

- [ ] **CDKToolkit Stack Exists**
  ```bash
  aws cloudformation describe-stacks --stack-name CDKToolkit
  ```

- [ ] **S3 Bucket Created**
  ```bash
  aws s3 ls | grep cdk-assets
  ```

- [ ] **IAM Roles Created**
  ```bash
  aws iam list-roles --query 'Roles[?contains(RoleName, `cdk-`)]'
  ```

- [ ] **ECR Repository Created** (if needed)
  ```bash
  aws ecr describe-repositories
  ```

- [ ] **Permissions Working**
  ```bash
  # Test with a simple stack deployment
  cdk deploy TestStack
  ```

- [ ] **Cross-Stack References Work** (for multi-stack)
  ```bash
  # Deploy dependent stacks
  cdk deploy Brain2Stack/Database Brain2Stack/Compute
  ```

## Environment-Specific Examples

### Brain2 Development Bootstrap

```bash
#!/bin/bash
# Development environment bootstrap

export NODE_ENV=development
export AWS_PROFILE=b2-development

# Bootstrap with basic settings
cdk bootstrap \
  --profile b2-development \
  --qualifier dev

# Verify bootstrap
aws cloudformation describe-stacks --stack-name CDKToolkit --profile b2-development
```

### Brain2 Production Bootstrap

```bash
#!/bin/bash
# Production environment bootstrap

export NODE_ENV=production
export AWS_PROFILE=b2-production

# Bootstrap with enhanced security
cdk bootstrap \
  --profile b2-production \
  --qualifier prod \
  --termination-protection \
  --cloudformation-execution-policies arn:aws:iam::aws:policy/PowerUserAccess \
  --trust-accounts 123456789012

# Verify bootstrap
aws cloudformation describe-stacks --stack-name CDKToolkit --profile b2-production
```

## Cost Considerations

### Bootstrap Costs

Bootstrap resources incur minimal costs:

- **S3 Bucket**: ~$0.023 per GB/month
- **ECR Repository**: ~$0.10 per GB/month
- **IAM Roles**: No cost
- **CloudFormation Stack**: No cost for stack itself

### Cost Optimization

1. **S3 Lifecycle Policies**
   ```json
   {
     "Rules": [{
       "Status": "Enabled",
       "Transitions": [{
         "Days": 30,
         "StorageClass": "STANDARD_IA"
       }]
     }]
   }
   ```

2. **Regular Cleanup**
   ```bash
   # Script to clean old CDK assets
   aws s3 ls s3://cdk-assets-bucket --recursive | grep -v $(date +%Y-%m) | awk '{print $4}' | head -100
   ```

## Conclusion

Proper CDK bootstrapping is essential for successful multi-stack deployments. For Brain2:

1. **Bootstrap is required** due to Lambda functions and multi-stack architecture
2. **Use environment-specific qualifiers** for proper separation
3. **Enable termination protection** in production
4. **Monitor and maintain** bootstrap resources regularly

Always test bootstrap procedures in development before applying to production environments.