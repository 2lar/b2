# API Reference

## Overview

This document provides comprehensive API reference for the Brain2 CDK constructs, stacks, and configuration interfaces.

## Table of Contents

- [Configuration](#configuration)
- [Constructs](#constructs)
- [Stacks](#stacks)
- [Utilities](#utilities)

## Configuration

### EnvironmentConfig

Main configuration interface for environment-specific settings.

```typescript
interface EnvironmentConfig {
  readonly stackName: string;
  readonly region: string;
  readonly account?: string;
  readonly dynamodb: DynamoDBConfig;
  readonly supabase: SupabaseConfig;
  readonly monitoring: MonitoringConfig;
  readonly frontend: FrontendConfig;
}
```

#### Properties

| Property | Type | Description |
|----------|------|-------------|
| `stackName` | `string` | Base name for CloudFormation stacks |
| `region` | `string` | AWS region for deployment |
| `account` | `string?` | AWS account ID (optional) |
| `dynamodb` | `DynamoDBConfig` | DynamoDB-specific configuration |
| `supabase` | `SupabaseConfig` | Supabase authentication configuration |
| `monitoring` | `MonitoringConfig` | Monitoring and alerting settings |
| `frontend` | `FrontendConfig` | Frontend-specific settings |

### DynamoDBConfig

```typescript
interface DynamoDBConfig {
  readonly removalPolicy: 'DESTROY' | 'RETAIN';
}
```

### SupabaseConfig

```typescript
interface SupabaseConfig {
  readonly url?: string;
  readonly serviceRoleKey?: string;
}
```

### MonitoringConfig

```typescript
interface MonitoringConfig {
  readonly enableDashboards: boolean;
  readonly enableAlarms: boolean;
}
```

### FrontendConfig

```typescript
interface FrontendConfig {
  readonly corsOrigins: string[];
}
```

## Constructs

### Brain2HttpApi

HTTP API Gateway construct for RESTful endpoints.

```typescript
export class Brain2HttpApi extends Construct {
  public readonly api: apigwv2.HttpApi;
  public readonly stage: apigwv2.HttpStage;
  public readonly authorizer: apigwv2.HttpAuthorizer;
  
  constructor(scope: Construct, id: string, props: Brain2HttpApiProps)
}
```

#### Props

```typescript
interface Brain2HttpApiProps {
  readonly config: EnvironmentConfig;
  readonly authorizerFunction: lambda.Function;
  readonly backendFunction: lambda.Function;
}
```

#### Properties

| Property | Type | Description |
|----------|------|-------------|
| `api` | `apigwv2.HttpApi` | The HTTP API Gateway instance |
| `stage` | `apigwv2.HttpStage` | The deployment stage |
| `authorizer` | `apigwv2.HttpAuthorizer` | JWT authorizer instance |

#### Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `get url()` | `string` | Returns the API Gateway URL |

#### Example Usage

```typescript
const httpApi = new Brain2HttpApi(this, 'HttpApi', {
  config: environmentConfig,
  authorizerFunction: authorizerLambda,
  backendFunction: backendLambda
});

// Access the API URL
const apiUrl = httpApi.url;
```

### Brain2WebSocketApi

WebSocket API Gateway construct for real-time communication.

```typescript
export class Brain2WebSocketApi extends Construct {
  public readonly api: apigwv2.WebSocketApi;
  public readonly stage: apigwv2.WebSocketStage;
  
  constructor(scope: Construct, id: string, props: Brain2WebSocketApiProps)
}
```

#### Props

```typescript
interface Brain2WebSocketApiProps {
  readonly config: EnvironmentConfig;
  readonly connectFunction: lambda.Function;
  readonly disconnectFunction: lambda.Function;
  readonly sendMessageFunction: lambda.Function;
}
```

#### Properties

| Property | Type | Description |
|----------|------|-------------|
| `api` | `apigwv2.WebSocketApi` | WebSocket API instance |
| `stage` | `apigwv2.WebSocketStage` | Deployment stage |

#### Methods

| Method | Return Type | Description |
|--------|-------------|-------------|
| `get url()` | `string` | WebSocket connection URL |
| `get callbackUrl()` | `string` | API management callback URL |

#### Example Usage

```typescript
const webSocketApi = new Brain2WebSocketApi(this, 'WebSocketApi', {
  config: environmentConfig,
  connectFunction: wsConnectLambda,
  disconnectFunction: wsDisconnectLambda,
  sendMessageFunction: wsSendMessageLambda
});

// Grant management permissions
webSocketApi.api.grantManageConnections(sendMessageFunction);
```

### GoLambdaFunction

Lambda function construct optimized for Go runtime.

```typescript
export class GoLambdaFunction extends lambda.Function {
  constructor(scope: Construct, id: string, props: GoLambdaFunctionProps)
}
```

#### Props

```typescript
interface GoLambdaFunctionProps extends lambda.FunctionProps {
  readonly functionName: string;
  readonly codePath: string;
  readonly handler: string;
  readonly environment?: Record<string, string>;
  readonly timeout?: Duration;
  readonly memorySize?: number;
}
```

#### Default Values

| Property | Default Value | Description |
|----------|---------------|-------------|
| `runtime` | `PROVIDED_AL2` | Go custom runtime |
| `handler` | `bootstrap` | Go binary handler |
| `timeout` | `30 seconds` | Function timeout |
| `memorySize` | `128 MB` | Memory allocation |

#### Example Usage

```typescript
const goFunction = new GoLambdaFunction(this, 'BackendFunction', {
  functionName: 'backend-lambda',
  codePath: path.join(__dirname, '../../../backend/build/main'),
  handler: 'bootstrap',
  environment: {
    TABLE_NAME: table.tableName,
    EVENT_BUS_NAME: eventBus.eventBusName
  },
  timeout: Duration.seconds(30),
  memorySize: 256
});
```

### NodeLambdaFunction

Lambda function construct optimized for Node.js runtime.

```typescript
export class NodeLambdaFunction extends lambda.Function {
  constructor(scope: Construct, id: string, props: NodeLambdaFunctionProps)
}
```

#### Props

```typescript
interface NodeLambdaFunctionProps extends lambda.FunctionProps {
  readonly functionName: string;
  readonly codePath: string;
  readonly handler: string;
  readonly environment?: Record<string, string>;
  readonly timeout?: Duration;
  readonly memorySize?: number;
}
```

#### Default Values

| Property | Default Value | Description |
|----------|---------------|-------------|
| `runtime` | `NODEJS_20_X` | Node.js 20 runtime |
| `timeout` | `10 seconds` | Function timeout |
| `memorySize` | `128 MB` | Memory allocation |

#### Example Usage

```typescript
const nodeFunction = new NodeLambdaFunction(this, 'AuthorizerFunction', {
  functionName: 'jwt-authorizer',
  codePath: path.join(__dirname, '../../lambda/authorizer'),
  handler: 'index.handler',
  environment: {
    SUPABASE_URL: config.supabase.url!,
    SUPABASE_SERVICE_ROLE_KEY: config.supabase.serviceRoleKey!
  }
});
```

## Stacks

### MainStack

Primary orchestration stack that coordinates all infrastructure components.

```typescript
export class MainStack extends Stack {
  public readonly databaseStack: DatabaseStack;
  public readonly computeStack: ComputeStack;
  public readonly apiStack: ApiStack;
  public readonly frontendStack: FrontendStack;
  public readonly monitoringStack?: MonitoringStack;
  
  constructor(scope: Construct, id: string, props: MainStackProps)
}
```

#### Props

```typescript
interface MainStackProps extends StackProps {
  readonly config: EnvironmentConfig;
}
```

#### Example Usage

```typescript
const mainStack = new MainStack(app, 'Brain2Stack', {
  config: environmentConfig,
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: 'us-east-1'
  }
});
```

### DatabaseStack

Manages DynamoDB tables and indexes.

```typescript
export class DatabaseStack extends Stack {
  public readonly memoryTable: dynamodb.Table;
  public readonly connectionsTable: dynamodb.Table;
  
  constructor(scope: Construct, id: string, props: DatabaseStackProps)
}
```

#### Props

```typescript
interface DatabaseStackProps extends StackProps {
  readonly config: EnvironmentConfig;
}
```

#### Resources Created

- **MemoryTable**: Primary data storage
  - Partition Key: `id` (String)
  - GSI: `KeywordIndex` on `keyword`
- **ConnectionsTable**: WebSocket connections
  - Partition Key: `connectionId` (String)
  - GSI: `connection-id-index` on `userId`

#### Example Usage

```typescript
const databaseStack = new DatabaseStack(this, 'Database', {
  config: environmentConfig,
  stackName: `${config.stackName}-database`
});

// Access tables
const memoryTable = databaseStack.memoryTable;
const connectionsTable = databaseStack.connectionsTable;
```

### ComputeStack

Manages Lambda functions, EventBridge, and WebSocket API.

```typescript
export class ComputeStack extends Stack {
  public readonly backendLambda: lambda.Function;
  public readonly connectNodeLambda: lambda.Function;
  public readonly wsConnectLambda: lambda.Function;
  public readonly wsDisconnectLambda: lambda.Function;
  public readonly wsSendMessageLambda: lambda.Function;
  public readonly authorizerLambda: lambda.Function;
  public readonly eventBus: events.EventBus;
  public readonly webSocketApi: Brain2WebSocketApi;
  
  constructor(scope: Construct, id: string, props: ComputeStackProps)
}
```

#### Props

```typescript
interface ComputeStackProps extends StackProps {
  readonly config: EnvironmentConfig;
  readonly memoryTable: dynamodb.Table;
  readonly connectionsTable: dynamodb.Table;
}
```

#### Lambda Functions

| Function | Runtime | Purpose |
|----------|---------|---------|
| `backendLambda` | Go | Main API handler |
| `connectNodeLambda` | Go | Node connection logic |
| `wsConnectLambda` | Go | WebSocket connections |
| `wsDisconnectLambda` | Go | WebSocket disconnections |
| `wsSendMessageLambda` | Go | Message broadcasting |
| `authorizerLambda` | Node.js | JWT authorization |

#### Example Usage

```typescript
const computeStack = new ComputeStack(this, 'Compute', {
  config: environmentConfig,
  stackName: `${config.stackName}-compute`,
  memoryTable: databaseStack.memoryTable,
  connectionsTable: databaseStack.connectionsTable
});

// Access Lambda functions
const backendFunction = computeStack.backendLambda;
const webSocketApi = computeStack.webSocketApi;
```

### ApiStack

Manages HTTP API Gateway and routing.

```typescript
export class ApiStack extends Stack {
  public readonly httpApi: Brain2HttpApi;
  
  constructor(scope: Construct, id: string, props: ApiStackProps)
}
```

#### Props

```typescript
interface ApiStackProps extends StackProps {
  readonly config: EnvironmentConfig;
  readonly backendLambda: lambda.Function;
  readonly authorizerLambda: lambda.Function;
}
```

#### Example Usage

```typescript
const apiStack = new ApiStack(this, 'Api', {
  config: environmentConfig,
  stackName: `${config.stackName}-api`,
  backendLambda: computeStack.backendLambda,
  authorizerLambda: computeStack.authorizerLambda
});

// Access HTTP API
const httpApiUrl = apiStack.httpApi.url;
```

### FrontendStack

Manages S3 bucket, CloudFront distribution, and static asset deployment.

```typescript
export class FrontendStack extends Stack {
  public readonly bucket: s3.Bucket;
  public readonly distribution: cloudfront.Distribution;
  
  constructor(scope: Construct, id: string, props: FrontendStackProps)
}
```

#### Props

```typescript
interface FrontendStackProps extends StackProps {
  readonly config: EnvironmentConfig;
}
```

#### Resources

- **S3 Bucket**: Static asset storage
- **CloudFront Distribution**: CDN with SPA routing support
- **BucketDeployment**: Automated deployment from `frontend/dist`

#### Example Usage

```typescript
const frontendStack = new FrontendStack(this, 'Frontend', {
  config: environmentConfig,
  stackName: `${config.stackName}-frontend`
});

// Access resources
const bucketName = frontendStack.bucket.bucketName;
const distributionId = frontendStack.distribution.distributionId;
```

## Utilities

### Environment Management

#### `getEnvironmentConfig(environmentName: string): EnvironmentConfig`

Retrieves configuration for specified environment.

```typescript
const config = getEnvironmentConfig('production');
```

#### `getCurrentEnvironment(): string`

Gets current environment from `NODE_ENV` or defaults to 'development'.

```typescript
const env = getCurrentEnvironment(); // 'development' | 'staging' | 'production'
```

### Constants

#### Resource Names

```typescript
export const RESOURCE_NAMES = {
  MEMORY_TABLE: 'MemoryTable',
  CONNECTIONS_TABLE: 'ConnectionsTable',
  EVENT_BUS: 'B2EventBus',
  HTTP_API: 'B2HttpApi',
  WEBSOCKET_API: 'B2WebSocketApi'
} as const;
```

#### Lambda Configuration

```typescript
export const LAMBDA_CONFIG = {
  GO_TIMEOUT: Duration.seconds(30),
  NODE_TIMEOUT: Duration.seconds(10),
  MEMORY_SIZE: 128,
  GO_RUNTIME: lambda.Runtime.PROVIDED_AL2,
  NODE_RUNTIME: lambda.Runtime.NODEJS_20_X
} as const;
```

#### API Configuration

```typescript
export const API_CONFIG = {
  CORS_HEADERS: [
    'Content-Type',
    'X-Amz-Date',
    'Authorization',
    'X-Api-Key',
    'X-Amz-Security-Token'
  ],
  CORS_METHODS: [
    'GET',
    'POST',
    'PUT',
    'DELETE',
    'OPTIONS'
  ]
} as const;
```

### Helper Functions

#### `getResourceName(prefix: string, suffix: string): string`

Generates consistent resource names.

```typescript
const tableName = getResourceName('memory', 'table'); // 'MemoryTable'
```

#### `getBucketName(stackName: string, account: string, region: string): string`

Generates globally unique S3 bucket names.

```typescript
const bucketName = getBucketName('b2-prod', '123456789012', 'us-east-1');
// 'b2-prod-frontend-123456789012-us-east-1'
```

## Type Definitions

### Common Types

```typescript
type Environment = 'development' | 'staging' | 'production';
type RemovalPolicy = 'DESTROY' | 'RETAIN';

interface StackPropsWithConfig extends StackProps {
  readonly config: EnvironmentConfig;
}
```

### Event Patterns

```typescript
export const EVENT_PATTERNS = {
  NODE_CREATED: {
    source: ['brain2.api'],
    detailType: ['NodeCreated']
  },
  EDGES_CREATED: {
    source: ['brain2.connectNode'],
    detailType: ['EdgesCreated']
  }
} as const;
```

## Error Handling

### Custom Errors

```typescript
export class ConfigurationError extends Error {
  constructor(message: string) {
    super(`Configuration Error: ${message}`);
    this.name = 'ConfigurationError';
  }
}

export class DeploymentError extends Error {
  constructor(message: string, public readonly stackName: string) {
    super(`Deployment Error in ${stackName}: ${message}`);
    this.name = 'DeploymentError';
  }
}
```

### Validation Functions

```typescript
export function validateEnvironmentConfig(config: EnvironmentConfig): void {
  if (!config.stackName) {
    throw new ConfigurationError('stackName is required');
  }
  
  if (!config.region) {
    throw new ConfigurationError('region is required');
  }
  
  if (!config.supabase.url) {
    throw new ConfigurationError('SUPABASE_URL environment variable is required');
  }
}
```

## Testing Utilities

### Test Helpers

```typescript
export function createTestStack(): Stack {
  const app = new App();
  return new Stack(app, 'TestStack', {
    env: { account: '123456789012', region: 'us-east-1' }
  });
}

export function createMockConfig(): EnvironmentConfig {
  return {
    stackName: 'test-stack',
    region: 'us-east-1',
    dynamodb: { removalPolicy: 'DESTROY' },
    supabase: {
      url: 'https://test.supabase.co',
      serviceRoleKey: 'test-key'
    },
    monitoring: {
      enableDashboards: false,
      enableAlarms: false
    },
    frontend: {
      corsOrigins: ['http://localhost:3000']
    }
  };
}
```