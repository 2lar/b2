# Development Guide

## Overview

This guide covers the development workflow, coding standards, and best practices for working with the Brain2 infrastructure codebase.

## Development Environment Setup

### Prerequisites
```bash
# Required tools
node --version    # v18+
npm --version     # v9+
aws --version     # v2+
cdk --version     # v2.118+
```

### Project Setup
```bash
# Clone repository
git clone <repository-url>
cd b2/infra

# Install dependencies
npm install

# Set up environment variables
cp .env.example .env
# Edit .env with your configuration

# Bootstrap CDK (first time only)
cdk bootstrap

# Verify setup
npm run build
npm test
npm run synth
```

## Project Structure

```
infra/
├── bin/
│   └── infra.ts              # CDK app entry point
├── lib/
│   ├── config/
│   │   ├── constants.ts      # Application constants
│   │   └── environments.ts   # Environment configurations
│   ├── constructs/
│   │   ├── api-gateway.ts    # HTTP API construct
│   │   ├── lambda-function.ts # Lambda constructs
│   │   └── websocket-api.ts  # WebSocket API construct
│   ├── stacks/
│   │   ├── api-stack.ts      # API Gateway stack
│   │   ├── compute-stack.ts  # Lambda and EventBridge stack
│   │   ├── database-stack.ts # DynamoDB stack
│   │   └── frontend-stack.ts # S3 and CloudFront stack
│   └── main-stack.ts         # Main orchestration stack
├── test/
│   ├── unit/                 # Unit tests
│   └── integration/          # Integration tests (future)
├── docs/                     # Documentation
├── lambda/                   # Lambda function code
├── package.json              # Dependencies and scripts
├── tsconfig.json             # TypeScript configuration
└── jest.config.js            # Jest testing configuration
```

## Development Workflow

### 1. Environment Configuration

#### Local Development
```bash
# Set development environment
export NODE_ENV=development

# Verify configuration
npm run synth
```

#### Environment Variables
```env
# .env file
NODE_ENV=development
AWS_REGION=us-east-1
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_ROLE_KEY=your-key
```

### 2. Code Development

#### Creating New Constructs
```typescript
// lib/constructs/my-construct.ts
import { Construct } from 'constructs';
import { EnvironmentConfig } from '../config/environments';

export interface MyConstructProps {
  config: EnvironmentConfig;
  // Add other props
}

export class MyConstruct extends Construct {
  constructor(scope: Construct, id: string, props: MyConstructProps) {
    super(scope, id);
    
    // Implementation
  }
}
```

#### Adding to Stacks
```typescript
// lib/stacks/my-stack.ts
import { Stack, StackProps } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { MyConstruct } from '../constructs/my-construct';

export class MyStack extends Stack {
  constructor(scope: Construct, id: string, props: StackProps) {
    super(scope, id, props);
    
    new MyConstruct(this, 'MyConstruct', {
      config: props.config
    });
  }
}
```

### 3. Testing

#### Unit Tests
```bash
# Run all tests
npm test

# Run specific test file
npm test -- lambda-function.test.ts

# Run tests in watch mode
npm run test:watch

# Generate coverage report
npm run test:coverage
```

#### Writing Tests
```typescript
// test/unit/constructs/my-construct.test.ts
import { App, Stack } from 'aws-cdk-lib';
import { Template } from '@aws-cdk/assertions';
import { MyConstruct } from '../../../lib/constructs/my-construct';

describe('MyConstruct', () => {
  let app: App;
  let stack: Stack;

  beforeEach(() => {
    app = new App();
    stack = new Stack(app, 'TestStack');
  });

  test('creates resource with correct properties', () => {
    // Arrange
    new MyConstruct(stack, 'TestConstruct', {
      config: mockConfig
    });

    // Act
    const template = Template.fromStack(stack);

    // Assert
    template.hasResourceProperties('AWS::Service::Resource', {
      Property: 'ExpectedValue'
    });
  });
});
```

### 4. Code Quality

#### TypeScript Standards
```typescript
// Use explicit types
export interface StackProps {
  config: EnvironmentConfig;
  resources: ResourceProps[];
}

// Use readonly for immutable data
export interface Config {
  readonly stackName: string;
  readonly region: string;
}

// Use enums for constants
export enum Environment {
  DEVELOPMENT = 'development',
  STAGING = 'staging',
  PRODUCTION = 'production'
}
```

#### ESLint Configuration
```json
{
  "extends": [
    "@typescript-eslint/recommended",
    "prettier"
  ],
  "rules": {
    "@typescript-eslint/no-unused-vars": "error",
    "@typescript-eslint/explicit-function-return-type": "warn"
  }
}
```

## Coding Standards

### 1. Naming Conventions

#### Resources
```typescript
// Use descriptive, PascalCase names
export class DatabaseStack extends Stack {}
export class Brain2HttpApi extends Construct {}

// Use consistent prefixes
const backendLambda = new lambda.Function(this, 'BackendLambda', {});
const memoryTable = new dynamodb.Table(this, 'MemoryTable', {});
```

#### Variables and Functions
```typescript
// Use camelCase for variables and functions
const environmentConfig = getEnvironmentConfig();
const resourceName = getResourceName('api', 'gateway');

// Use descriptive names
const isProductionEnvironment = config.stackName.includes('prod');
const shouldEnableMonitoring = config.monitoring.enableDashboards;
```

### 2. File Organization

#### Import Order
```typescript
// 1. Node.js built-ins
import * as path from 'path';

// 2. External libraries
import { Construct } from 'constructs';
import * as lambda from 'aws-cdk-lib/aws-lambda';

// 3. Internal imports
import { EnvironmentConfig } from '../config/environments';
import { MyConstruct } from '../constructs/my-construct';
```

#### Export Patterns
```typescript
// Named exports for multiple items
export { DatabaseStack } from './stacks/database-stack';
export { ComputeStack } from './stacks/compute-stack';

// Default export for single main item
export default class MainStack extends Stack {}
```

### 3. Configuration Management

#### Environment-Specific Settings
```typescript
export const environments: Record<string, EnvironmentConfig> = {
  development: {
    stackName: 'b2-dev',
    dynamodb: { removalPolicy: 'DESTROY' },
    monitoring: { enableAlarms: false }
  },
  production: {
    stackName: 'b2-prod',
    dynamodb: { removalPolicy: 'RETAIN' },
    monitoring: { enableAlarms: true }
  }
};
```

#### Constants
```typescript
export const RESOURCE_NAMES = {
  MEMORY_TABLE: 'MemoryTable',
  CONNECTIONS_TABLE: 'ConnectionsTable',
  EVENT_BUS: 'B2EventBus'
} as const;

export const LAMBDA_CONFIG = {
  TIMEOUT: Duration.seconds(30),
  MEMORY_SIZE: 128,
  RUNTIME: lambda.Runtime.PROVIDED_AL2
} as const;
```

## Development Best Practices

### 1. CDK Best Practices

#### Resource Naming
```typescript
// Use consistent naming with stack prefix
const functionName = `${config.stackName}-backend-lambda`;
const bucketName = `${config.stackName}-frontend-${this.account}-${this.region}`;
```

#### Environment Variables
```typescript
// Group related environment variables
const lambdaEnvironment = {
  TABLE_NAME: table.tableName,
  INDEX_NAME: 'KeywordIndex',
  EVENT_BUS_NAME: eventBus.eventBusName
};
```

#### IAM Permissions
```typescript
// Use least privilege principle
table.grantReadWriteData(lambda); // Specific permissions
// Instead of: lambda.addToRolePolicy(new iam.PolicyStatement({...}))
```

### 2. Testing Best Practices

#### Test Structure
```typescript
describe('ComponentName', () => {
  // Setup
  let app: App;
  let stack: Stack;

  beforeEach(() => {
    // Fresh instances for each test
    app = new App();
    stack = new Stack(app, 'TestStack');
  });

  describe('specific functionality', () => {
    test('should do something specific', () => {
      // Arrange - Act - Assert pattern
    });
  });
});
```

#### Snapshot Tests
```typescript
test('matches snapshot', () => {
  // Create construct
  new MyConstruct(stack, 'TestConstruct', props);
  
  // Generate template and compare
  const template = Template.fromStack(stack);
  expect(template.toJSON()).toMatchSnapshot();
});
```

### 3. Error Handling

#### Validation
```typescript
export function validateEnvironmentConfig(config: EnvironmentConfig): void {
  if (!config.stackName) {
    throw new Error('Stack name is required');
  }
  
  if (!config.region) {
    throw new Error('AWS region is required');
  }
}
```

#### Graceful Degradation
```typescript
const corsOrigins = config.frontend?.corsOrigins || ['http://localhost:3000'];
const monitoringEnabled = config.monitoring?.enableDashboards ?? false;
```

## Local Testing

### Unit Testing
```bash
# Test specific components
npm test -- --testNamePattern="DatabaseStack"

# Test with coverage
npm run test:coverage

# Update snapshots
npm test -- --updateSnapshot
```

### Integration Testing
```bash
# Deploy to development environment
export NODE_ENV=development
npm run deploy

# Test deployed resources
npm run test:integration
```

### Manual Testing
```bash
# Synthesize templates for review
npm run synth

# Deploy specific stack
cdk deploy Brain2Stack/Database --profile dev

# Check deployed resources
aws dynamodb list-tables --region us-east-1
```

## Debugging

### CDK Issues
```bash
# Verbose output
cdk deploy --verbose

# Debug CDK app
cdk synth --debug

# Check CDK version compatibility
cdk doctor
```

### Stack Issues
```bash
# View stack events
aws cloudformation describe-stack-events --stack-name b2-dev-database

# Check stack status
aws cloudformation describe-stacks --stack-name b2-dev-database
```

### Lambda Issues
```bash
# View function logs
aws logs tail /aws/lambda/b2-dev-backend --follow

# Test function locally
sam local invoke BackendFunction --event event.json
```

## Performance Optimization

### Build Performance
```bash
# Parallel builds
npm run build -- --parallel

# TypeScript incremental compilation
# Configure in tsconfig.json:
{
  "compilerOptions": {
    "incremental": true,
    "tsBuildInfoFile": ".tsbuildinfo"
  }
}
```

### Runtime Performance
```typescript
// Optimize Lambda cold starts
const lambda = new lambda.Function(this, 'Function', {
  runtime: lambda.Runtime.PROVIDED_AL2, // Faster than interpreted runtimes
  memorySize: 1024, // More memory = more CPU
  timeout: Duration.seconds(30),
  reservedConcurrentExecutions: 10 // Prevent cold starts
});
```

## Contributing

### Pull Request Process
1. Create feature branch from `main`
2. Implement changes with tests
3. Run quality checks: `npm run build && npm test`
4. Create pull request with description
5. Address review comments
6. Merge after approval

### Code Review Checklist
- [ ] Tests added/updated for changes
- [ ] Documentation updated
- [ ] No TypeScript errors
- [ ] CDK synthesizes successfully
- [ ] Security best practices followed
- [ ] Performance impact considered