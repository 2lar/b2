# Troubleshooting Guide

## Overview

This guide covers common issues, their solutions, and debugging techniques for the Brain2 infrastructure.

## Common Issues

### 1. CDK Deployment Issues

#### Stack Deployment Failures

**Issue**: Stack fails to deploy with CloudFormation errors
```
Error: The stack named 'b2-dev' failed to deploy: UPDATE_ROLLBACK_COMPLETE
```

**Solution**:
```bash
# Check stack events for specific error
aws cloudformation describe-stack-events --stack-name b2-dev

# Common fixes:
# 1. Resource name conflicts
cdk deploy --force

# 2. Permission issues
aws sts get-caller-identity  # Verify credentials

# 3. Resource limits
aws service-quotas get-service-quota --service-code lambda --quota-code L-B99A9384
```

#### Cyclic Dependencies

**Issue**: Cross-stack references creating circular dependencies
```
Error: 'Brain2Stack/Compute' depends on 'Brain2Stack/Api' ... would create a cyclic reference
```

**Solution**:
```typescript
// Move tightly coupled resources to the same stack
// Example: WebSocket API moved to Compute Stack
export class ComputeStack extends Stack {
  public readonly webSocketApi: Brain2WebSocketApi;
  
  constructor(scope: Construct, id: string, props: ComputeStackProps) {
    super(scope, id, props);
    
    // Create Lambda functions first
    const lambdaFunctions = this.createLambdaFunctions();
    
    // Then create WebSocket API using the functions
    this.webSocketApi = new Brain2WebSocketApi(this, 'WebSocketApi', {
      connectFunction: lambdaFunctions.connect,
      disconnectFunction: lambdaFunctions.disconnect
    });
  }
}
```

### 2. Lambda Function Issues

#### Cold Start Problems

**Issue**: Lambda functions timing out due to cold starts
```
Error: Task timed out after 30.00 seconds
```

**Solutions**:
```typescript
// 1. Increase timeout
const lambda = new lambda.Function(this, 'Function', {
  timeout: Duration.minutes(5), // Increase from 30s
  memorySize: 1024, // More memory = faster execution
});

// 2. Enable provisioned concurrency (production)
const version = lambda.currentVersion;
new lambda.Alias(this, 'ProdAlias', {
  aliasName: 'prod',
  version,
  provisionedConcurrencyConfig: {
    provisionedConcurrentExecutions: 5
  }
});

// 3. Optimize Go binary size
// In Makefile:
// GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o bootstrap ./cmd/main
```

#### Memory Issues

**Issue**: Lambda function running out of memory
```
Error: Runtime exited with error: exit status 137 (out of memory)
```

**Solution**:
```typescript
// Increase memory allocation
const lambda = new lambda.Function(this, 'Function', {
  memorySize: 512, // Increase from 128MB
  // Consider: 256, 512, 1024, or higher based on usage
});

// Monitor memory usage
const memoryUtilizationAlarm = new cloudwatch.Alarm(this, 'MemoryAlarm', {
  metric: lambda.metricDuration(),
  threshold: 80,
  evaluationPeriods: 2
});
```

#### Environment Variable Issues

**Issue**: Lambda cannot access required environment variables
```
Error: Environment variable TABLE_NAME not found
```

**Solution**:
```typescript
// Ensure all required variables are set
const lambda = new lambda.Function(this, 'Function', {
  environment: {
    TABLE_NAME: table.tableName,
    REGION: this.region,
    // Add all required variables
  }
});

// Validate in Lambda code (Go example):
// tableName := os.Getenv("TABLE_NAME")
// if tableName == "" {
//     return events.APIGatewayProxyResponse{
//         StatusCode: 500,
//         Body: "TABLE_NAME environment variable not set",
//     }, nil
// }
```

### 3. API Gateway Issues

#### CORS Configuration

**Issue**: Frontend cannot access API due to CORS errors
```
Error: Access to fetch at 'https://api.example.com' from origin 'http://localhost:3000' 
has been blocked by CORS policy
```

**Solution**:
```typescript
// Update CORS configuration
const api = new apigwv2.HttpApi(this, 'HttpApi', {
  corsPreflight: {
    allowOrigins: [
      'http://localhost:3000',
      'http://localhost:5173', // Vite dev server
      'https://yourdomain.com' // Production domain
    ],
    allowMethods: [
      apigwv2.CorsHttpMethod.GET,
      apigwv2.CorsHttpMethod.POST,
      apigwv2.CorsHttpMethod.PUT,
      apigwv2.CorsHttpMethod.DELETE,
      apigwv2.CorsHttpMethod.OPTIONS
    ],
    allowHeaders: ['Content-Type', 'Authorization'],
    maxAge: Duration.days(10)
  }
});
```

#### Authorization Issues

**Issue**: API requests failing with 401/403 errors
```
Error: {"message": "Forbidden"}
```

**Solution**:
```bash
# Debug authorization
# 1. Check JWT token format
curl -H "Authorization: Bearer <token>" https://your-api.com/endpoint

# 2. Verify authorizer function logs
aws logs tail /aws/lambda/jwt-authorizer --follow

# 3. Test authorizer independently
aws lambda invoke --function-name jwt-authorizer --payload '{"token": "your-jwt"}' response.json
```

```typescript
// Update authorizer configuration
const authorizer = new apigwv2.HttpLambdaAuthorizer('JWTAuthorizer', {
  authorizerFunction: authorizerLambda,
  identitySource: ['$request.header.Authorization'],
  responseTypes: [apigwv2.HttpLambdaResponseType.SIMPLE]
});
```

### 4. DynamoDB Issues

#### Throttling

**Issue**: DynamoDB operations being throttled
```
Error: ProvisionedThroughputExceededException
```

**Solution**:
```typescript
// Use on-demand billing mode
const table = new dynamodb.Table(this, 'Table', {
  billingMode: dynamodb.BillingMode.ON_DEMAND, // Automatically scales
  
  // Or increase provisioned capacity
  readCapacity: 10,
  writeCapacity: 10
});

// Add auto-scaling
const readScaling = table.autoScaleReadCapacity({
  minCapacity: 5,
  maxCapacity: 100
});
readScaling.scaleOnUtilization({
  targetUtilizationPercent: 70
});
```

#### Access Denied

**Issue**: Lambda cannot access DynamoDB table
```
Error: User: arn:aws:sts::123456789012:assumed-role/lambda-role is not authorized 
to perform: dynamodb:PutItem on resource: table/MyTable
```

**Solution**:
```typescript
// Grant proper permissions
const table = new dynamodb.Table(this, 'Table', {});
const lambda = new lambda.Function(this, 'Function', {});

// Use CDK's built-in grant methods
table.grantReadWriteData(lambda);

// Or for specific operations
table.grantReadData(lambda);
table.grantWriteData(lambda);
```

### 5. WebSocket Issues

#### Connection Failures

**Issue**: WebSocket connections failing to establish
```
Error: WebSocket connection to 'wss://your-websocket-api.com' failed
```

**Solution**:
```typescript
// Check WebSocket API configuration
const webSocketApi = new apigwv2.WebSocketApi(this, 'WebSocketApi', {
  connectRouteOptions: {
    integration: new WebSocketLambdaIntegration('ConnectIntegration', connectLambda)
  },
  disconnectRouteOptions: {
    integration: new WebSocketLambdaIntegration('DisconnectIntegration', disconnectLambda)
  }
});

// Ensure proper stage deployment
const stage = new apigwv2.WebSocketStage(this, 'Stage', {
  webSocketApi,
  stageName: 'prod',
  autoDeploy: true
});
```

#### Message Delivery Issues

**Issue**: WebSocket messages not being delivered
```
Error: Failed to send message to connection
```

**Solution**:
```typescript
// Grant management permissions
webSocketApi.grantManageConnections(sendMessageLambda);

// Add callback URL environment variable
sendMessageLambda.addEnvironment('WEBSOCKET_API_ENDPOINT', stage.callbackUrl);
```

### 6. Frontend Issues

#### CloudFront Distribution

**Issue**: Frontend assets not loading or showing old versions
```
Error: 404 Not Found for static assets
```

**Solution**:
```bash
# Invalidate CloudFront cache
aws cloudfront create-invalidation --distribution-id E1234567890123 --paths "/*"

# Or in CDK deployment
const distribution = new cloudfront.Distribution(this, 'Distribution', {
  // ... configuration
});

new s3deploy.BucketDeployment(this, 'DeployFrontend', {
  sources: [s3deploy.Source.asset('../frontend/dist')],
  destinationBucket: bucket,
  distribution, // Automatically invalidates cache
  distributionPaths: ['/*']
});
```

#### SPA Routing

**Issue**: Client-side routing returns 404 for direct URLs
```
Error: 404 for https://yoursite.com/dashboard
```

**Solution**:
```typescript
const distribution = new cloudfront.Distribution(this, 'Distribution', {
  defaultBehavior: {
    origin: new origins.S3Origin(bucket)
  },
  defaultRootObject: 'index.html',
  
  // Handle SPA routing
  errorResponses: [{
    httpStatus: 404,
    responseHttpStatus: 200,
    responsePagePath: '/index.html',
    ttl: Duration.minutes(5)
  }]
});
```

## Debugging Techniques

### 1. CDK Debugging

#### Verbose Output
```bash
# Get detailed deployment information
cdk deploy --verbose --debug

# See what resources will be created
cdk diff

# Output CloudFormation template
cdk synth > template.yaml
```

#### Stack Inspection
```bash
# List all stacks
cdk list

# Show stack outputs
aws cloudformation describe-stacks --stack-name b2-dev --query 'Stacks[0].Outputs'

# Check stack resources
aws cloudformation list-stack-resources --stack-name b2-dev
```

### 2. Lambda Debugging

#### Log Analysis
```bash
# Real-time log streaming
aws logs tail /aws/lambda/function-name --follow --start-time 1h

# Search logs for errors
aws logs filter-log-events --log-group-name /aws/lambda/function-name --filter-pattern "ERROR"

# Get log insights
aws logs start-query --log-group-name /aws/lambda/function-name --start-time $(date -d '1 hour ago' +%s) --end-time $(date +%s) --query-string 'fields @timestamp, @message | filter @message like /ERROR/ | sort @timestamp desc'
```

#### Local Testing
```bash
# Test Lambda function locally with SAM
sam local invoke BackendFunction --event event.json

# Start local API
sam local start-api --template template.yaml
```

### 3. API Gateway Debugging

#### Request Tracing
```typescript
// Enable CloudWatch logs for API Gateway
const logGroup = new logs.LogGroup(this, 'ApiGatewayLogs', {
  logGroupName: `/aws/apigateway/${api.httpApiId}`,
  retention: logs.RetentionDays.ONE_WEEK
});

const stage = new apigwv2.HttpStage(this, 'Stage', {
  httpApi: api,
  stageName: 'prod',
  accessLogDestination: new apigwv2.CloudWatchLogsDestination(logGroup),
  accessLogFormat: apigwv2.AccessLogFormat.jsonWithStandardFields()
});
```

#### Test API Endpoints
```bash
# Test with curl
curl -X GET https://your-api-id.execute-api.region.amazonaws.com/stage/endpoint

# Test with authorization
curl -H "Authorization: Bearer your-jwt-token" https://your-api.com/protected-endpoint

# Test WebSocket connection
wscat -c wss://your-websocket-id.execute-api.region.amazonaws.com/stage
```

### 4. Database Debugging

#### DynamoDB Operations
```bash
# List tables
aws dynamodb list-tables

# Scan table (use sparingly)
aws dynamodb scan --table-name MemoryTable --max-items 10

# Query with GSI
aws dynamodb query --table-name MemoryTable --index-name KeywordIndex --key-condition-expression "keyword = :keyword" --expression-attribute-values '{":keyword":{"S":"example"}}'

# Check table metrics
aws cloudwatch get-metric-statistics --namespace AWS/DynamoDB --metric-name ConsumedReadCapacityUnits --dimensions Name=TableName,Value=MemoryTable --start-time 2023-01-01T00:00:00Z --end-time 2023-01-01T01:00:00Z --period 300 --statistics Sum
```

## Monitoring and Alerting

### 1. CloudWatch Alarms

```typescript
// Lambda error rate alarm
const errorAlarm = new cloudwatch.Alarm(this, 'LambdaErrorAlarm', {
  metric: lambda.metricErrors(),
  threshold: 5,
  evaluationPeriods: 2,
  treatMissingData: cloudwatch.TreatMissingData.NOT_BREACHING
});

// DynamoDB throttling alarm
const throttleAlarm = new cloudwatch.Alarm(this, 'DynamoDBThrottleAlarm', {
  metric: table.metricThrottledRequests(),
  threshold: 1,
  evaluationPeriods: 1
});
```

### 2. Custom Dashboards

```typescript
const dashboard = new cloudwatch.Dashboard(this, 'Dashboard', {
  dashboardName: 'Brain2-Monitoring'
});

dashboard.addWidgets(
  new cloudwatch.GraphWidget({
    title: 'Lambda Invocations',
    left: [lambda.metricInvocations()],
    right: [lambda.metricErrors()]
  }),
  
  new cloudwatch.GraphWidget({
    title: 'DynamoDB Operations',
    left: [table.metricConsumedReadCapacityUnits()],
    right: [table.metricConsumedWriteCapacityUnits()]
  })
);
```

## Performance Optimization

### 1. Lambda Optimization

```bash
# Analyze Lambda performance
aws logs insights start-query --log-group-name /aws/lambda/function-name --start-time $(date -d '1 day ago' +%s) --end-time $(date +%s) --query-string 'filter @type="REPORT" | stats avg(@duration), max(@duration), min(@duration) by bin(5m)'
```

### 2. DynamoDB Optimization

```bash
# Check hot partitions
aws dynamodb describe-table --table-name MemoryTable --query 'Table.GlobalSecondaryIndexes[0].ItemCount'

# Monitor consumed capacity
aws cloudwatch get-metric-statistics --namespace AWS/DynamoDB --metric-name ConsumedReadCapacityUnits --dimensions Name=TableName,Value=MemoryTable --start-time $(date -d '1 hour ago' --iso-8601) --end-time $(date --iso-8601) --period 300 --statistics Average
```

## Emergency Procedures

### 1. Complete Service Outage

```bash
# Check all stack statuses
aws cloudformation list-stacks --stack-status-filter CREATE_COMPLETE UPDATE_COMPLETE

# Check service health
aws lambda list-functions --query 'Functions[?contains(FunctionName, `b2-dev`)].FunctionName'

# Rollback if needed
aws cloudformation update-stack --stack-name b2-dev --use-previous-template
```

### 2. Data Recovery

```bash
# DynamoDB point-in-time recovery
aws dynamodb restore-table-to-point-in-time --source-table-name MemoryTable --target-table-name MemoryTable-Restored --restore-date-time 2023-01-01T12:00:00Z

# S3 version recovery
aws s3api list-object-versions --bucket your-bucket --prefix path/to/file
```

### 3. Security Incident

```bash
# Disable API Gateway immediately
aws apigatewayv2 update-stage --api-id your-api-id --stage-name prod --throttle-settings RateLimit=1,BurstLimit=1

# Check CloudTrail for suspicious activity
aws logs filter-log-events --log-group-name CloudTrail/your-trail --filter-pattern "{ $.eventName = \"AssumeRole\" && $.sourceIPAddress != \"your-ip\" }"

# Rotate access keys
aws iam update-access-key --access-key-id AKIAIOSFODNN7EXAMPLE --status Inactive
```

## Getting Help

### 1. AWS Support

- **Developer Support**: Basic technical support
- **Business Support**: 24/7 support with 1-hour response time for production issues
- **Enterprise Support**: Technical Account Manager and 15-minute response time

### 2. Community Resources

- **AWS CDK GitHub**: https://github.com/aws/aws-cdk
- **AWS Developer Forums**: https://forums.aws.amazon.com/
- **Stack Overflow**: Tag questions with `aws-cdk`, `aws-lambda`, etc.

### 3. Documentation

- **AWS CDK API Reference**: https://docs.aws.amazon.com/cdk/api/v2/
- **AWS Service Documentation**: https://docs.aws.amazon.com/
- **Best Practices Guides**: AWS Well-Architected Framework