# Brain2 Infrastructure

[![CDK Version](https://img.shields.io/badge/CDK-2.118.0-orange.svg)](https://github.com/aws/aws-cdk)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.3-blue.svg)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Enterprise-grade AWS CDK infrastructure for Brain2, a graph-based knowledge management system. Built with TypeScript using modern cloud architecture patterns and best practices.

## 🏗️ Architecture Overview

Brain2 uses a modular, nested-stack architecture deployed to **us-west-2** region. The infrastructure follows AWS best practices with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────────┐
│                        Brain2Stack (Parent)                      │
│                           b2-dev (Root)                          │
└─────────────────────────────────────────────────────────────────┘
                                 │
        ┌────────────────┬───────┴────────┬──────────────┐
        ▼                ▼                ▼              ▼
┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
│   Database   │ │   Compute    │ │     API      │ │   Frontend   │
│    Stack     │ │    Stack     │ │    Stack     │ │    Stack     │
│ b2-dev-      │ │ b2-dev-      │ │ b2-dev-api   │ │ b2-dev-      │
│ database     │ │ compute      │ │              │ │ frontend     │
└──────────────┘ └──────────────┘ └──────────────┘ └──────────────┘
       │                │                │                │
   DynamoDB        Lambda + WS       HTTP API         S3 + CDN
```

### 📦 Stack Architecture

#### **1. Database Stack** (`b2-dev-database`)
**Purpose**: Data persistence layer with optimized NoSQL tables

**Resources**:
- **Memory Table** (`brain2`): Main DynamoDB table for nodes, edges, and keywords
  - Partition Key: `PK` (user-based partitioning)
  - Sort Key: `SK` (entity type and ID)
  - GSI1: `KeywordIndex` for keyword-based searches
  - GSI2: `EdgeIndex` for edge relationship queries
- **Connections Table**: WebSocket connection tracking
  - Manages real-time client connections
  - TTL-based cleanup for disconnected clients

**Configuration**:
- Billing: Pay-per-request (on-demand)
- Point-in-time recovery: Enabled for production
- Removal policy: DESTROY for dev, RETAIN for staging/prod

#### **2. Compute Stack** (`b2-dev-compute`)
**Purpose**: Business logic processing and real-time communication

**Resources**:
- **Lambda Functions** (7 total):
  - `BackendLambda` (Go): Main API handler for CRUD operations
  - `JWTAuthorizerLambda` (Node.js): Token validation with Supabase
  - `ConnectNodeLambda` (Go): Graph connection analyzer
  - `CleanupLambda` (Go): Scheduled maintenance tasks
  - `WSConnectLambda` (Go): WebSocket connection handler
  - `WSDisconnectLambda` (Go): WebSocket disconnection cleanup
  - `WSSendMessageLambda` (Go): WebSocket message broadcasting
- **EventBridge Event Bus** (`B2EventBus`): Asynchronous event orchestration
- **WebSocket API Gateway**: Real-time bidirectional communication
  - Routes: `$connect`, `$disconnect`, `$default`
  - Authorizer integration for secure connections

**Configuration**:
- Memory: 512MB (dev), 1024MB (prod)
- Timeout: 60 seconds
- Environment-specific log retention

#### **3. API Stack** (`b2-dev-api`)
**Purpose**: RESTful HTTP API endpoint management

**Resources**:
- **HTTP API Gateway V2**: Modern, cost-optimized API
  - JWT authorizer integration
  - CORS configuration
  - Throttling and rate limiting
- **Routes**: 
  - `/nodes/*` → Backend Lambda
  - `/graph/*` → Backend Lambda  
  - `/health` → Direct response
- **Stages**: Environment-specific deployments

**Configuration**:
- Authorization: JWT tokens from Supabase
- CORS: Environment-specific origins
- Payload size limit: 10MB

#### **4. Frontend Stack** (`b2-dev-frontend`)
**Purpose**: Static website hosting with global CDN

**Resources**:
- **S3 Bucket**: Static asset storage
  - Versioning enabled
  - Server-side encryption
  - Public access blocked (CloudFront only)
- **CloudFront Distribution**: Global content delivery
  - Origin Access Identity (OAI) for secure S3 access
  - Custom error pages for SPA routing
  - HTTPS only with TLS 1.2+
  - Caching optimizations

**Configuration**:
- Default cache: 1 day
- Error page caching: 5 minutes
- Geo-restriction: None (global access)

#### **5. Monitoring Stack** (`b2-dev-monitoring`) - *Optional*
**Purpose**: Observability and alerting (created when enabled)

**Resources**:
- CloudWatch Dashboards
- Alarms for Lambda errors, API latency, DynamoDB throttling
- Log aggregation and insights
- Custom metrics

**Configuration**:
- Only created when `enableDashboards` or `enableAlarms` is true
- Environment-specific thresholds

### 🔄 Stack Dependencies

```
Database → Compute → API → Frontend
         ↘        ↗
          Monitoring
```

- **Compute** depends on **Database** (table references)
- **API** depends on **Compute** (Lambda function integrations)
- **Frontend** is independent (static hosting)
- **Monitoring** depends on all stacks (observes their metrics)

## 🚀 Quick Start

### Prerequisites

- **Node.js** 18+
- **AWS CLI** 2+ (configured)
- **AWS CDK** 2.118+
- **TypeScript** 5.3+

### Installation

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

# Build and test
npm run build
npm test

# Deploy to development
npm run deploy
```

## 📁 Project Structure

```
infra/
├── bin/
│   └── infra.ts              # CDK app entry point (ts-node executable)
├── lib/
│   ├── config/
│   │   ├── constants.ts      # Application constants & resource names
│   │   └── environments.ts   # Environment configurations (dev/staging/prod)
│   ├── constructs/
│   │   ├── api-gateway.ts    # HTTP API Gateway V2 construct
│   │   ├── lambda-function.ts # Reusable Lambda function construct
│   │   └── websocket-api.ts  # WebSocket API Gateway construct
│   ├── stacks/
│   │   ├── database-stack.ts # DynamoDB tables and indexes
│   │   ├── compute-stack.ts  # Lambda functions, EventBridge, WebSocket API
│   │   ├── api-stack.ts      # HTTP API Gateway and routes
│   │   ├── frontend-stack.ts # S3 bucket and CloudFront distribution
│   │   └── monitoring-stack.ts # CloudWatch dashboards and alarms
│   └── main-stack.ts         # Parent stack orchestrator
├── lambda/
│   └── authorizer/           # JWT authorizer Lambda (Node.js)
│       ├── index.ts          # Supabase JWT validation logic
│       ├── package.json      # Lambda dependencies
│       └── clean.sh          # Build cleanup script
├── test/
│   ├── unit/                 
│   │   ├── config/           # Environment configuration tests
│   │   │   └── environments.test.ts
│   │   ├── constructs/       # Construct unit tests
│   │   │   ├── api-gateway.test.ts
│   │   │   ├── lambda-function.test.ts
│   │   │   └── websocket-api.test.ts
│   │   └── stacks/           # Stack unit tests
│   │       ├── compute-stack.test.ts
│   │       └── database-stack.test.ts
│   └── setup.ts              # Jest test configuration
├── docs/                     # Comprehensive documentation
│   ├── architecture.md       # System design and patterns
│   ├── deployment.md         # Step-by-step deployment guide
│   ├── development.md        # Developer workflow and standards
│   ├── troubleshooting.md    # Common issues and solutions
│   ├── api-reference.md      # API endpoint documentation
│   ├── bootstrap-guide.md    # AWS CDK bootstrap instructions
│   └── migration-guide.md    # Migration from previous versions
├── cdk.json                  # CDK app configuration (ts-node setup)
├── tsconfig.json             # TypeScript compiler configuration
├── jest.config.js            # Jest test runner configuration
├── package.json              # Dependencies and npm scripts
├── package-lock.json         # Locked dependency versions
├── README.md                 # This file
└── .gitignore               # Git ignore patterns (excludes .js/.d.ts)
```

## 🛠️ Available Commands

### TypeScript Workflow (Recommended)

Brain2 uses **ts-node** for zero-compilation development. TypeScript files are executed directly without generating JavaScript files.

```bash
# Development workflow (no compilation needed)
npm run typecheck      # Validate TypeScript without generating files
npm test               # Run tests directly from TypeScript (via ts-jest)
npm run synth          # Synthesize CloudFormation templates (via ts-node)
npm run deploy         # Deploy all stacks (via ts-node)

# Build pipeline
npm run build          # Full pipeline: typecheck → test → synth
npm run build-lambda   # Build Node.js Lambda functions (authorizer)
npm run clean          # Remove all generated JS/d.ts files and CDK artifacts
```

### Testing

```bash
npm test               # Run all tests
npm run test:unit      # Run unit tests only
npm run test:coverage  # Generate coverage report
npm run test:watch     # Watch mode testing
```

### Compilation (When Needed)

```bash
# Generate JavaScript files (only when explicitly needed)
npm run compile        # Compile TypeScript to JavaScript
npm run watch          # Watch and run tests (replaces tsc -w)
```

### CDK Operations

```bash
# These work directly with TypeScript (no build step required)
npm run synth          # Synthesize CloudFormation templates
npm run deploy         # Deploy all stacks
npm run destroy        # Destroy all stacks
cdk diff               # Show differences
cdk list               # List all stacks
```

### Development Workflow

#### Everyday Development (Zero-Compilation)
```bash
# 1. Clean workspace (optional)
npm run clean

# 2. Validate TypeScript
npm run typecheck

# 3. Run tests
npm test

# 4. Deploy changes
npm run deploy
```

#### When JavaScript Files Are Needed
```bash
# For CI/CD or when JS artifacts are required
npm run compile        # Generates all .js and .d.ts files
```

> **Note**: CDK executes TypeScript directly via ts-node configuration in `cdk.json`. No compilation step is required for normal development or deployment.

### Build Artifact Management

The project uses a **zero-compilation workflow** by default. Generated JavaScript files are automatically cleaned to keep the repository tidy:

```bash
npm run clean          # Removes all generated files:
                      # - lib/**/*.js and lib/**/*.d.ts
                      # - bin/**/*.js and bin/**/*.d.ts  
                      # - test/**/*.js and test/**/*.d.ts
                      # - lambda/authorizer/*.js and lambda/authorizer/*.d.ts
                      # - cdk.out/ directory
```

**Files preserved during cleaning:**
- All TypeScript source files (`.ts`)
- Configuration files (`jest.config.js`, `*.json`)
- Dependencies (`node_modules/`)

**Generated files are excluded from Git** via `.gitignore` and only created when explicitly needed via `npm run compile`.

## 🌍 Environment Management

### Supported Environments

| Environment | Stack Prefix | Region | Monitoring | Data Retention | CORS Origins |
|-------------|-------------|---------|------------|----------------|--------------|
| Development | `b2-dev` | us-west-2 | Basic | Destroy on delete | All origins (`*`) |
| Staging | `b2-staging` | us-west-2 | Enhanced + Dashboards | Retained | `*.brain2-staging.com` |
| Production | `b2-prod` | us-west-2 | Full + Alarms | Retained | `brain2.com`, `www.brain2.com` |

### Stack Management

#### Deploying Individual Stacks

```bash
# Deploy all stacks (recommended)
npm run deploy

# Deploy specific stack
cdk deploy Brain2Stack/Database
cdk deploy Brain2Stack/Compute
cdk deploy Brain2Stack/Api
cdk deploy Brain2Stack/Frontend

# Deploy with approval bypass (CI/CD)
cdk deploy --require-approval never
```

#### Stack Outputs

Each stack exports values for cross-stack references:

- **Database Stack**: Table names, ARNs
- **Compute Stack**: Lambda function ARNs, WebSocket API URL
- **API Stack**: HTTP API URL, API ID
- **Frontend Stack**: CloudFront distribution URL, S3 bucket name

#### Checking Stack Status

```bash
# List all stacks
cdk list

# Show stack differences
cdk diff

# View stack outputs
aws cloudformation describe-stacks --stack-name b2-dev-database --query 'Stacks[0].Outputs'

# Check stack resources
aws cloudformation list-stack-resources --stack-name b2-dev-compute
```

### Environment Configuration

Create a `.env` file:

```env
# AWS Configuration
AWS_REGION=us-west-2
NODE_ENV=development

# Supabase Configuration
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_ROLE_KEY=your-service-role-key
```

## 🧪 Testing

### Test Coverage

- **Unit Tests**: All constructs and stacks
- **Integration Tests**: Real AWS deployments (planned)
- **Snapshot Tests**: CloudFormation template verification
- **Configuration Tests**: Environment validation

### Running Tests

```bash
# All tests with coverage
npm run test:coverage

# Specific test patterns
npm test -- --testNamePattern="DatabaseStack"
npm test -- --testPathPattern="constructs"

# Update snapshots
npm test -- --updateSnapshot
```

## 📚 Documentation

- **[Architecture Guide](docs/architecture.md)**: Detailed system design and patterns
- **[Deployment Guide](docs/deployment.md)**: Step-by-step deployment procedures
- **[Development Guide](docs/development.md)**: Developer workflow and standards
- **[Troubleshooting](docs/troubleshooting.md)**: Common issues and solutions
- **[API Reference](docs/api-reference.md)**: Complete API documentation

## 🚀 Deployment

### Prerequisites Checklist

- [ ] AWS credentials configured
- [ ] Environment variables set
- [ ] Backend Lambda functions built (`../backend/build/`)
- [ ] Frontend assets built (`../frontend/dist/`)
- [ ] Tests passing (`npm test`)

### Deployment Steps

1. **Build Lambda Functions**
   ```bash
   # Build Node.js Lambda functions (authorizer)
   npm run build-lambda
   
   # Build Go Lambda functions
   cd ../backend
   ./build.sh
   cd ../infra
   ```

2. **Build Frontend Assets**
   ```bash
   cd ../frontend
   npm run build  # Creates dist/ directory
   cd ../infra
   ```

3. **Deploy Infrastructure**
   ```bash
   npm run deploy
   ```

## 🤝 Contributing

### Development Workflow

1. Create feature branch from `main`
2. Implement changes with tests
3. Run quality checks: `npm run build && npm test`
4. Update documentation if needed
5. Create pull request
6. Address review comments
7. Merge after approval

## 📋 Troubleshooting

### Common Issues

| Issue | Solution |
|-------|----------|
| CDK deployment fails | Check AWS credentials and region settings |
| Stack already exists | Delete existing stack or use different environment name |
| Lambda timeout errors | Increase timeout in `environments.ts` (default: 60s) |
| CORS errors | Verify allowed origins in environment config |
| DynamoDB throttling | Already using on-demand billing, check for hot partitions |
| Missing Supabase config | Set `SUPABASE_URL` and `SUPABASE_SERVICE_ROLE_KEY` in `.env` |

### Stack Deployment Order

If deploying stacks individually, follow this order:
1. **Database Stack** (no dependencies)
2. **Compute Stack** (requires Database)
3. **API Stack** (requires Compute) 
4. **Frontend Stack** (independent, can deploy anytime)
5. **Monitoring Stack** (requires all others if enabled)

### Clean Deployment

```bash
# Remove all stacks
npm run destroy

# Clean build artifacts
npm run clean

# Fresh deployment
npm run deploy
```

---

**Built with ❤️ using AWS CDK and TypeScript**

For detailed documentation, see the [docs](docs/) directory.