# Brain2 Infrastructure

[![CDK Version](https://img.shields.io/badge/CDK-2.118.0-orange.svg)](https://github.com/aws/aws-cdk)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.3-blue.svg)](https://www.typescriptlang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Enterprise-grade AWS CDK infrastructure for Brain2, a graph-based knowledge management system. Built with TypeScript using modern cloud architecture patterns and best practices.

## 🏗️ Architecture Overview

Brain2 uses a modular, multi-stack architecture following AWS best practices for scalability, security, and maintainability:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Frontend      │    │   API Gateway    │    │   Lambda        │
│   (S3/CloudFront)│ ──▶│   (HTTP/WS)     │ ──▶│   Functions     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                          │
                       ┌──────────────────┐              ▼
                       │   EventBridge    │    ┌─────────────────┐
                       │   (Events)       │◀──▶│   DynamoDB      │
                       └──────────────────┘    │   Tables        │
                                               └─────────────────┘
```

### Core Components

- **Database Stack**: DynamoDB tables with GSI for efficient querying
- **Compute Stack**: Lambda functions (Go/Node.js) with EventBridge orchestration
- **API Stack**: HTTP API Gateway with JWT authentication
- **Frontend Stack**: S3 + CloudFront with SPA routing support

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
│   │   ├── constructs/       # Construct tests
│   │   ├── stacks/           # Stack tests
│   │   └── config/           # Configuration tests
│   └── setup.ts              # Test configuration
├── docs/
│   ├── architecture.md       # System architecture
│   ├── deployment.md         # Deployment procedures
│   ├── development.md        # Development guide
│   ├── troubleshooting.md    # Common issues and solutions
│   └── api-reference.md      # API documentation
├── lambda/                   # Lambda function code
└── package.json              # Dependencies and scripts
```

## 🛠️ Available Commands

### Development

```bash
npm run build          # Compile TypeScript
npm run watch          # Watch for changes
npm test               # Run all tests
npm run test:unit      # Run unit tests only
npm run test:coverage  # Generate coverage report
npm run test:watch     # Watch mode testing
```

### CDK Operations

```bash
npm run synth          # Synthesize CloudFormation templates
npm run deploy         # Deploy all stacks
npm run destroy        # Destroy all stacks
cdk diff               # Show differences
cdk list               # List all stacks
```

## 🌍 Environment Management

### Supported Environments

| Environment | Stack Prefix | Monitoring | Data Retention | CORS Origins |
|-------------|-------------|------------|----------------|--------------|
| Development | `b2-dev` | Basic | Destroy on delete | `localhost:*` |
| Staging | `b2-staging` | Enhanced | Retained | Staging domain |
| Production | `b2-prod` | Full + Alarms | Retained | Production domain |

### Environment Configuration

Create a `.env` file:

```env
# AWS Configuration
AWS_REGION=us-east-1
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

1. **Build Backend Functions**
   ```bash
   cd ../backend
   # Build Go functions for AWS Lambda
   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/main ./cmd/main
   cd infra
   ```

2. **Build Frontend Assets**
   ```bash
   cd ../frontend
   npm run build  # Creates dist/ directory
   cd infra
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
| CDK deployment fails | Check [Troubleshooting Guide](docs/troubleshooting.md#cdk-deployment-issues) |
| Lambda timeout errors | Increase timeout or memory allocation |
| CORS errors | Verify origin configuration |
| DynamoDB throttling | Enable on-demand billing |

---

**Built with ❤️ using AWS CDK and TypeScript**

For detailed documentation, see the [docs](docs/) directory.