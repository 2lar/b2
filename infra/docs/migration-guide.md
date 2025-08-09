# CDK Stack Migration Guide

## Overview

This guide provides comprehensive procedures for migrating from a monolithic CDK stack to a modular multi-stack architecture, specifically addressing resource conflict issues that arise during the transition.

## Table of Contents

- [Understanding the Migration](#understanding-the-migration)
- [Common Issues](#common-issues)
- [Migration Strategies](#migration-strategies)
- [Step-by-Step Procedures](#step-by-step-procedures)
- [Data Backup and Recovery](#data-backup-and-recovery)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Understanding the Migration

### What's Changing

**From: Monolithic Stack**
```
b2-stack.ts
├── DynamoDB Tables
├── Lambda Functions  
├── API Gateway
├── S3 + CloudFront
└── All resources in one stack
```

**To: Multi-Stack Architecture**
```
Brain2Stack/
├── Database Stack (DynamoDB)
├── Compute Stack (Lambda + EventBridge)
├── API Stack (API Gateway)
└── Frontend Stack (S3 + CloudFront)
```

### Why Migration is Needed

1. **Better Organization**: Logical separation of concerns
2. **Independent Deployments**: Deploy components separately
3. **Team Collaboration**: Different teams can own different stacks
4. **Reduced Blast Radius**: Changes affect only relevant components
5. **Enterprise Standards**: Industry best practices for large applications

## Common Issues

### 1. Resource Already Exists Errors

**Symptom:**
```bash
CREATE_FAILED: Resource already exists
```

**Root Cause:**
- CloudFormation sees the new stacks as trying to create "new" resources
- Physical resources exist from the old monolithic stack
- Resource names/ARNs conflict between old and new stacks

### 2. Cross-Stack Reference Issues

**Symptom:**
```bash
No export named 'xxx' found. Exports must be unique across a given region.
```

**Root Cause:**
- Old stack exports still exist
- New stacks trying to create exports with same names

### 3. IAM Role Conflicts

**Symptom:**
```bash
Role already exists: b2-dev-backend-role
```

**Root Cause:**
- IAM roles have global names within account
- Same role names used in old and new stacks

## Migration Strategies

### Strategy 1: Clean Slate (Recommended for Development)

**Pros:**
- ✅ Clean, predictable outcome
- ✅ No complex migration logic
- ✅ Fresh start with new architecture

**Cons:**
- ❌ Data loss if not backed up
- ❌ Service downtime during migration

**Best For:** Development and staging environments

### Strategy 2: Blue-Green Migration (Production)

**Pros:**
- ✅ Zero downtime
- ✅ Easy rollback
- ✅ Data preservation

**Cons:**
- ❌ More complex
- ❌ Temporary double costs
- ❌ Requires careful DNS switching

**Best For:** Production environments with strict uptime requirements

### Strategy 3: CloudFormation Import (Advanced)

**Pros:**
- ✅ Preserves existing resources
- ✅ No data loss
- ✅ Maintains resource history

**Cons:**
- ❌ Complex and error-prone
- ❌ Limited to compatible resources
- ❌ Requires deep CloudFormation knowledge

**Best For:** Critical production systems with complex data

## Step-by-Step Procedures

### Pre-Migration Checklist

- [ ] **Environment Identification**: Confirm target environment (dev/staging/prod)
- [ ] **Data Backup**: Complete backup of all critical data
- [ ] **Documentation**: Review current stack configuration
- [ ] **Team Notification**: Inform team of planned migration
- [ ] **Rollback Plan**: Prepare rollback procedures
- [ ] **Testing Environment**: Test migration in non-production first

### Strategy 1: Clean Slate Migration

#### Phase 1: Assessment and Backup

1. **List Current Stacks**
   ```bash
   # Check existing CDK stacks
   cdk list
   
   # Check CloudFormation stacks
   aws cloudformation list-stacks --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE
   ```

2. **Backup DynamoDB Data**
   ```bash
   # Export DynamoDB table data
   aws dynamodb scan --table-name MemoryTable --output json > memory-table-backup.json
   aws dynamodb scan --table-name ConnectionsTable --output json > connections-table-backup.json
   ```

3. **Document Current Configuration**
   ```bash
   # Save current stack outputs
   aws cloudformation describe-stacks --stack-name <old-stack-name> --query 'Stacks[0].Outputs' > stack-outputs-backup.json
   
   # Save current stack resources
   aws cloudformation list-stack-resources --stack-name <old-stack-name> > stack-resources-backup.json
   ```

4. **Backup Environment Variables**
   ```bash
   # Save current Lambda environment variables
   aws lambda get-function-configuration --function-name <function-name> > lambda-config-backup.json
   ```

#### Phase 2: Clean Destruction

1. **Destroy Old Stack**
   ```bash
   # WARNING: This will delete all resources
   cdk destroy <old-stack-name>
   
   # Verify destruction
   aws cloudformation describe-stacks --stack-name <old-stack-name>
   # Should return StackStatus: DELETE_COMPLETE or stack not found
   ```

2. **Clean Up Remaining Resources** (if any)
   ```bash
   # Check for orphaned resources
   aws dynamodb list-tables --query 'TableNames[?contains(@, `b2-`) || contains(@, `Memory`) || contains(@, `Connection`)]'
   aws lambda list-functions --query 'Functions[?contains(FunctionName, `b2-`)]'
   aws s3api list-buckets --query 'Buckets[?contains(Name, `b2-`)]'
   ```

#### Phase 3: Bootstrap Verification

1. **Check Bootstrap Status**
   ```bash
   cdk bootstrap --show-template
   ```

2. **Re-bootstrap if Needed**
   ```bash
   cdk bootstrap --termination-protection
   ```

#### Phase 4: Deploy New Architecture

1. **Deploy Database Stack First**
   ```bash
   export NODE_ENV=development  # or staging/production
   cdk deploy Brain2Stack/Database
   ```

2. **Verify Database Deployment**
   ```bash
   aws dynamodb list-tables
   aws dynamodb describe-table --table-name MemoryTable
   aws dynamodb describe-table --table-name ConnectionsTable
   ```

3. **Deploy Compute Stack**
   ```bash
   cdk deploy Brain2Stack/Compute
   ```

4. **Verify Compute Deployment**
   ```bash
   aws lambda list-functions --query 'Functions[?contains(FunctionName, `b2-dev`)]'
   aws events list-event-buses --name-prefix B2EventBus
   ```

5. **Deploy API Stack**
   ```bash
   cdk deploy Brain2Stack/Api
   ```

6. **Verify API Deployment**
   ```bash
   aws apigatewayv2 get-apis --query 'Items[?Name==`B2HttpApi`]'
   aws apigatewayv2 get-apis --query 'Items[?Name==`B2WebSocketApi`]'
   ```

7. **Deploy Frontend Stack**
   ```bash
   cdk deploy Brain2Stack/Frontend
   ```

8. **Verify Frontend Deployment**
   ```bash
   aws s3api list-buckets --query 'Buckets[?contains(Name, `b2-frontend`)]'
   aws cloudfront list-distributions --query 'DistributionList.Items[?Comment==`Brain2 Frontend Distribution - b2-dev`]'
   ```

#### Phase 5: Data Restoration

1. **Restore DynamoDB Data** (if needed)
   ```bash
   # Restore memory table data
   aws dynamodb batch-write-item --request-items file://memory-table-restore.json
   
   # Restore connections table data
   aws dynamodb batch-write-item --request-items file://connections-table-restore.json
   ```

2. **Verify Data Restoration**
   ```bash
   aws dynamodb scan --table-name MemoryTable --select COUNT
   aws dynamodb scan --table-name ConnectionsTable --select COUNT
   ```

### Strategy 2: Blue-Green Migration

#### Phase 1: Prepare Green Environment

1. **Deploy New Stacks with Different Names**
   ```bash
   # Temporarily modify stack names in environments.ts
   # Change stackName: 'b2-dev' to stackName: 'b2-dev-new'
   
   cdk deploy --all
   ```

2. **Test New Environment**
   ```bash
   # Run integration tests against new environment
   npm run test:integration
   ```

#### Phase 2: Data Synchronization

1. **Set Up DynamoDB Streams** (if using real-time sync)
2. **Export/Import Data**
   ```bash
   # Export from old tables
   aws dynamodb export-table-to-point-in-time --table-arn <old-table-arn> --s3-bucket <backup-bucket>
   
   # Import to new tables
   aws dynamodb import-table --input-format DYNAMODB_JSON --s3-bucket-source <backup-bucket>
   ```

#### Phase 3: DNS/Traffic Switching

1. **Update Frontend Configuration**
   - Point frontend to new API endpoints
   - Update CORS configurations

2. **Switch Traffic**
   - Update load balancer targets
   - Update DNS records (if using custom domain)

#### Phase 4: Cleanup

1. **Verify New Environment**
   - Monitor for 24-48 hours
   - Check all functionality

2. **Destroy Old Environment**
   ```bash
   cdk destroy <old-stack-name>
   ```

## Data Backup and Recovery

### DynamoDB Backup Strategies

#### Method 1: Point-in-Time Recovery
```bash
# Enable PITR (if not already enabled)
aws dynamodb update-continuous-backups --table-name MemoryTable --point-in-time-recovery-specification PointInTimeRecoveryEnabled=true

# Create backup
aws dynamodb create-backup --table-name MemoryTable --backup-name memory-table-migration-backup
```

#### Method 2: Export/Import
```bash
# Export table
aws dynamodb export-table-to-point-in-time \
  --table-arn arn:aws:dynamodb:region:account:table/MemoryTable \
  --s3-bucket my-backup-bucket \
  --s3-prefix memory-table-backup/

# Import table
aws dynamodb import-table \
  --input-format DYNAMODB_JSON \
  --s3-bucket-source Bucket=my-backup-bucket,KeyPrefix=memory-table-backup/
```

#### Method 3: Scan and Restore (Small Tables)
```bash
# Backup script
#!/bin/bash
aws dynamodb scan --table-name MemoryTable --output json > memory-table-backup.json

# Restore script
#!/bin/bash
cat memory-table-backup.json | jq -r '.Items[] | @json' | while read item; do
  aws dynamodb put-item --table-name MemoryTable --item "$item"
done
```

### Lambda Configuration Backup

```bash
# Backup all function configurations
for func in $(aws lambda list-functions --query 'Functions[].FunctionName' --output text); do
  aws lambda get-function --function-name $func > "backup-$func.json"
done
```

### S3 Backup

```bash
# Sync S3 bucket contents
aws s3 sync s3://old-bucket s3://backup-bucket --delete
```

## Troubleshooting

### Issue: "Resource Already Exists"

**Problem:** CloudFormation can't create resource because it already exists
```bash
CREATE_FAILED: Table already exists: MemoryTable
```

**Solutions:**

1. **Check Resource Existence**
   ```bash
   aws dynamodb describe-table --table-name MemoryTable
   ```

2. **Delete Resource Manually** (if safe)
   ```bash
   aws dynamodb delete-table --table-name MemoryTable
   ```

3. **Import Existing Resource** (advanced)
   ```bash
   # Create import template
   cdk deploy --no-execute
   # Use CloudFormation console to import existing resources
   ```

### Issue: "Export Already Exists"

**Problem:** Stack trying to create export that already exists
```bash
Export b2-dev-api-url already exists
```

**Solutions:**

1. **Remove Old Export**
   ```bash
   # Update old stack to remove export, or destroy old stack
   cdk destroy <old-stack-name>
   ```

2. **Rename New Export** (temporary)
   ```bash
   # Modify export name in new stack temporarily
   exportName: `${config.stackName}-api-url-new`
   ```

### Issue: "Role Already Exists"

**Problem:** IAM role name conflict
```bash
CREATE_FAILED: Role already exists: b2-dev-lambda-role
```

**Solutions:**

1. **Delete Old Role** (if unused)
   ```bash
   aws iam delete-role --role-name b2-dev-lambda-role
   ```

2. **Update Role Names**
   ```bash
   # Add timestamp or unique identifier to role names
   roleName: `${config.stackName}-lambda-role-${Date.now()}`
   ```

### Issue: "Cross-Stack Reference Not Found"

**Problem:** New stack can't find reference from old stack
```bash
No export named 'b2-dev-vpc-id' found
```

**Solutions:**

1. **Deploy Dependencies First**
   ```bash
   # Ensure all required stacks are deployed
   cdk deploy Brain2Stack/Database Brain2Stack/Compute
   ```

2. **Update Reference**
   ```bash
   # Use Fn::ImportValue with correct export name
   Fn.importValue(`${props.config.stackName}-vpc-id`)
   ```

## Best Practices

### Development Workflow

1. **Test in Non-Production First**
   - Always test migration in development environment
   - Document any issues and solutions
   - Create runbook for production migration

2. **Incremental Migration**
   - Migrate one stack at a time
   - Verify each stack before proceeding
   - Have rollback plan for each step

3. **Automated Testing**
   - Run integration tests after each stack deployment
   - Verify cross-stack references work
   - Test application functionality end-to-end

### Production Considerations

1. **Maintenance Windows**
   - Schedule migration during low-traffic periods
   - Notify users of potential service interruption
   - Have support team on standby

2. **Monitoring**
   - Set up enhanced monitoring during migration
   - Watch CloudWatch metrics and alarms
   - Monitor application logs for errors

3. **Communication**
   - Keep stakeholders informed of progress
   - Document any deviations from plan
   - Provide regular status updates

### Post-Migration

1. **Verification Checklist**
   - [ ] All stacks deployed successfully
   - [ ] Cross-stack references working
   - [ ] Application functionality verified
   - [ ] Data integrity confirmed
   - [ ] Performance metrics normal
   - [ ] No CloudFormation errors

2. **Documentation Updates**
   - Update deployment procedures
   - Document any configuration changes
   - Update team runbooks

3. **Cleanup**
   - Remove old backup files (after retention period)
   - Clean up temporary resources
   - Update CI/CD pipelines

## Emergency Procedures

### Rollback During Migration

1. **If Database Stack Fails**
   ```bash
   cdk destroy Brain2Stack/Database
   # Restore from backup if data was modified
   ```

2. **If Application Not Working**
   ```bash
   # Quick rollback to monolithic stack
   git checkout <previous-commit>
   cdk deploy <old-stack-name>
   ```

3. **If Data Corruption Detected**
   ```bash
   # Stop all traffic immediately
   # Restore from most recent backup
   aws dynamodb restore-table-from-backup --target-table-name MemoryTable --backup-arn <backup-arn>
   ```

### Recovery Procedures

1. **Complete Stack Recovery**
   ```bash
   # Destroy all new stacks
   cdk destroy --all
   
   # Restore original stack
   git checkout <working-commit>
   cdk deploy <old-stack-name>
   
   # Restore data from backups
   aws dynamodb restore-table-from-backup --target-table-name MemoryTable --backup-arn <backup-arn>
   ```

## Conclusion

CDK stack migration requires careful planning and execution. Always prioritize data safety and have tested rollback procedures. When in doubt, test in a non-production environment first and consult with your team before making irreversible changes.

Remember: **It's better to take extra time to do the migration safely than to rush and cause data loss or extended downtime.**