# Deployment Guide

## Overview

This guide covers deployment procedures for the Brain2 infrastructure across different environments using AWS CDK.

## Prerequisites

### Required Tools
- **Node.js**: Version 18+ 
- **AWS CDK**: Version 2.118+
- **AWS CLI**: Version 2+
- **TypeScript**: Version 5.3+

### Required Permissions
- CloudFormation: Full access
- IAM: Role and policy management
- Lambda: Function deployment
- API Gateway: API management
- DynamoDB: Table management
- S3: Bucket management
- CloudFront: Distribution management

### Environment Variables

Create a `.env` file in the project root:

```env
# AWS Configuration
AWS_REGION=us-east-1
AWS_ACCOUNT_ID=your-account-id

# Supabase Configuration
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key

# Environment
NODE_ENV=development  # or staging, production
```

## Environment Setup

### Development Environment
```bash
# Install dependencies
npm install

# Build TypeScript
npm run build

# Run tests
npm test

# Synthesize CloudFormation templates
npm run synth
```

### AWS Configuration
```bash
# Configure AWS CLI
aws configure

# Verify access
aws sts get-caller-identity

# Bootstrap CDK (first time only)
cdk bootstrap
```

## Deployment Commands

### Development Deployment
```bash
# Set environment
export NODE_ENV=development

# Deploy all stacks
npm run deploy

# Deploy specific stack
cdk deploy Brain2Stack/Database
cdk deploy Brain2Stack/Compute
cdk deploy Brain2Stack/Api
cdk deploy Brain2Stack/Frontend
```

### Staging Deployment
```bash
# Set environment
export NODE_ENV=staging

# Deploy to staging
npm run deploy

# Verify deployment
cdk list
```

### Production Deployment
```bash
# Set environment
export NODE_ENV=production

# Deploy to production
npm run deploy

# Monitor deployment
aws cloudformation describe-stacks --region us-east-1
```

## Deployment Order

The CDK automatically manages stack dependencies, but the logical order is:

1. **Database Stack**: DynamoDB tables and indexes
2. **Compute Stack**: Lambda functions and EventBridge
3. **API Stack**: HTTP API Gateway
4. **Frontend Stack**: S3 bucket and CloudFront distribution

## Environment-Specific Configurations

### Development
- **Stack Name**: `b2-dev`
- **Removal Policy**: `DESTROY` (data deleted on stack deletion)
- **Monitoring**: Basic CloudWatch metrics
- **CORS**: Allows localhost origins
- **CloudFront**: Price class 100 (North America/Europe)

### Staging
- **Stack Name**: `b2-staging`
- **Removal Policy**: `RETAIN` (data preserved)
- **Monitoring**: Enhanced monitoring with dashboards
- **CORS**: Staging domain only
- **CloudFront**: Price class 100

### Production
- **Stack Name**: `b2-prod`
- **Removal Policy**: `RETAIN`
- **Monitoring**: Full monitoring with alarms
- **CORS**: Production domain only
- **CloudFront**: Price class ALL (global distribution)
- **S3**: Versioning enabled with lifecycle rules

## Pre-Deployment Checklist

### Code Quality
- [ ] All tests pass (`npm test`)
- [ ] TypeScript compiles without errors (`npm run build`)
- [ ] CDK synthesizes successfully (`npm run synth`)
- [ ] No security vulnerabilities (`npm audit`)

### Configuration
- [ ] Environment variables set correctly
- [ ] Supabase credentials configured
- [ ] AWS credentials and region configured
- [ ] Target environment specified (`NODE_ENV`)

### Dependencies
- [ ] Backend Lambda functions built (`/backend/build/` directory)
- [ ] Frontend assets built (`/frontend/dist/` directory)
- [ ] Lambda authorizer code present (`/lambda/authorizer/`)

## Backend Lambda Deployment

### Go Lambda Functions
```bash
# Navigate to backend directory
cd ../backend

# Build all Lambda functions for AWS
./build.sh

# Return to infrastructure directory
cd infra

# Deploy with updated functions
npm run deploy
```

### Node.js Authorizer
The JWT authorizer is automatically packaged and deployed with the CDK stack.

## Frontend Deployment

### Build and Deploy
```bash
# Navigate to frontend directory
cd ../frontend

# Install dependencies
npm install

# Build for production
npm run build

# Return to infrastructure directory
cd infra

# Deploy frontend stack
cdk deploy Brain2Stack/Frontend
```

The frontend is automatically deployed to S3 and invalidated in CloudFront.

## Post-Deployment Verification

### Health Checks
```bash
# Check stack status
aws cloudformation describe-stacks --stack-name b2-dev

# Test API endpoints
curl https://your-api-id.execute-api.us-east-1.amazonaws.com/prod/health

# Test WebSocket connection
wscat -c wss://your-websocket-id.execute-api.us-east-1.amazonaws.com/prod
```

### Monitoring
- **CloudWatch Logs**: Check Lambda function logs
- **API Gateway Metrics**: Monitor request counts and errors
- **DynamoDB Metrics**: Verify table access patterns
- **CloudFront**: Check cache hit ratios

## Rollback Procedures

### Infrastructure Rollback
```bash
# Rollback specific stack
aws cloudformation cancel-update-stack --stack-name b2-dev-compute

# Rollback to previous version
aws cloudformation update-stack --stack-name b2-dev-compute --use-previous-template
```

### Application Rollback
```bash
# Revert to previous Lambda version
aws lambda update-function-code --function-name backend-lambda --s3-bucket previous-bucket --s3-key previous-code.zip
```

## Troubleshooting

### Common Issues

#### Stack Update Failures
```bash
# Check stack events
aws cloudformation describe-stack-events --stack-name b2-dev

# Continue rollback if stuck
aws cloudformation continue-update-rollback --stack-name b2-dev
```

#### Lambda Function Errors
```bash
# View function logs
aws logs tail /aws/lambda/backend-lambda --follow

# Update function configuration
aws lambda update-function-configuration --function-name backend-lambda --timeout 60
```

#### API Gateway Issues
```bash
# Test API deployment
aws apigatewayv2 get-stages --api-id your-api-id

# Redeploy API stage
aws apigatewayv2 create-deployment --api-id your-api-id --stage-name prod
```

### Performance Issues
- **Cold Starts**: Consider provisioned concurrency
- **Timeout Errors**: Increase Lambda timeout settings
- **Memory Errors**: Increase Lambda memory allocation
- **Database Throttling**: Check DynamoDB capacity settings

## CI/CD Integration

### GitHub Actions Example
```yaml
name: Deploy Infrastructure

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      
      - name: Install dependencies
        run: npm install
        working-directory: infra
      
      - name: Run tests
        run: npm test
        working-directory: infra
      
      - name: Deploy to staging
        env:
          NODE_ENV: staging
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
        run: npm run deploy
        working-directory: infra
```

## Security Considerations

### Secrets Management
- Use AWS Secrets Manager for sensitive configuration
- Never commit credentials to version control
- Use IAM roles for service-to-service communication

### Network Security
- Deploy Lambda functions in VPC for sensitive operations
- Use security groups to restrict access
- Enable VPC Flow Logs for monitoring

### Compliance
- Enable CloudTrail for API logging
- Use AWS Config for compliance monitoring
- Implement least privilege access principles

## Cost Optimization

### Development
- Use smaller Lambda memory allocations
- Enable DynamoDB on-demand billing
- Use CloudFront price class 100

### Production
- Monitor costs with AWS Cost Explorer
- Set up billing alerts
- Optimize Lambda execution time
- Use CloudFront for reduced data transfer costs

## Maintenance

### Regular Tasks
- [ ] Update CDK version monthly
- [ ] Review and rotate access keys quarterly
- [ ] Monitor and optimize costs monthly
- [ ] Update Lambda runtime versions
- [ ] Review security groups and permissions

### Backup and Recovery
- DynamoDB: Point-in-time recovery enabled
- S3: Versioning and lifecycle rules configured
- Lambda: Version management for rollbacks
- Infrastructure: CloudFormation templates in version control