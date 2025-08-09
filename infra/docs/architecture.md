# Brain2 Infrastructure Architecture

## Overview

Brain2 is a graph-based knowledge management system built using AWS CDK with a modular, multi-stack architecture. This document provides a comprehensive overview of the system architecture, design patterns, and infrastructure components.

## High-Level Architecture

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

## Stack Architecture

The infrastructure is organized into four primary stacks following AWS best practices:

### 1. Database Stack (`DatabaseStack`)
**Purpose**: Manages persistent data storage and indexes

**Components**:
- **Memory Table** (`MemoryTable`): Primary data storage for nodes and connections
  - Partition Key: `id` (String)
  - Global Secondary Index: `KeywordIndex` on `keyword` attribute
  - Billing: Pay-per-request
- **Connections Table** (`ConnectionsTable`): WebSocket connection management
  - Partition Key: `connectionId` (String)
  - Global Secondary Index: `connection-id-index` on `userId` attribute
  - Billing: Pay-per-request

**Environment-Specific Configuration**:
- Development: `DESTROY` removal policy (data is deleted on stack deletion)
- Staging/Production: `RETAIN` removal policy (data is preserved)

### 2. Compute Stack (`ComputeStack`)
**Purpose**: Serverless compute resources and event processing

**Components**:
- **Lambda Functions**:
  - `backendLambda`: Main API handler (Go runtime)
  - `connectNodeLambda`: Node connection discovery (Go runtime)
  - `wsConnectLambda`: WebSocket connection handler (Go runtime)
  - `wsDisconnectLambda`: WebSocket disconnection handler (Go runtime)
  - `wsSendMessageLambda`: WebSocket message broadcaster (Go runtime)
  - `authorizerLambda`: JWT authorization (Node.js runtime)

- **EventBridge**:
  - Custom event bus: `B2EventBus`
  - Event rules for decoupled communication between components

- **WebSocket API**:
  - Real-time communication gateway
  - Connect/disconnect route handlers
  - Message broadcasting capabilities

### 3. API Stack (`ApiStack`)
**Purpose**: HTTP API Gateway and request routing

**Components**:
- **HTTP API Gateway**:
  - RESTful API endpoints
  - JWT-based authorization
  - CORS configuration for frontend integration
  - Lambda proxy integration

**Security**:
- Request-based JWT authorizer
- CORS policies configured per environment
- API throttling and rate limiting

### 4. Frontend Stack (`FrontendStack`)
**Purpose**: Static asset hosting and content delivery

**Components**:
- **S3 Bucket**: Static asset storage with versioning (production only)
- **CloudFront Distribution**: Global content delivery network
- **Automated Deployment**: Build artifacts automatically deployed

**Features**:
- SPA routing support (404 → index.html)
- Asset compression and caching
- Environment-specific pricing tiers

## Design Patterns

### 1. Multi-Stack Architecture
- **Stateful/Stateless Separation**: Database resources separated from compute
- **Dependency Management**: Clear dependency hierarchy prevents circular references
- **Environment Isolation**: Stack-level environment configuration

### 2. Event-Driven Architecture
- **Decoupled Communication**: EventBridge for inter-service communication
- **Event Patterns**:
  - `NodeCreated` → triggers connection discovery
  - `EdgesCreated` → triggers WebSocket notifications

### 3. Infrastructure as Code
- **TypeScript CDK**: Type-safe infrastructure definitions
- **Construct Libraries**: Reusable infrastructure components
- **Configuration Management**: Environment-specific settings

### 4. Security Best Practices
- **Least Privilege IAM**: Granular permissions per service
- **JWT Authorization**: Stateless authentication
- **Network Security**: Private subnets and security groups
- **Data Encryption**: Encryption at rest and in transit

## Data Flow

### Node Creation Flow
```
Frontend → HTTP API → Backend Lambda → EventBridge → Connect Node Lambda → DynamoDB
                                         ↓
                                    WebSocket API ← Send Message Lambda
```

### Real-Time Updates Flow
```
User Action → Backend Lambda → EventBridge → Send Message Lambda → WebSocket API → Frontend
```

## Scalability Considerations

### Horizontal Scaling
- **Lambda Concurrency**: Automatic scaling based on demand
- **DynamoDB**: On-demand billing scales with usage
- **CloudFront**: Global edge locations for low latency

### Performance Optimization
- **Cold Start Mitigation**: Provisioned concurrency for critical functions
- **Database Indexing**: GSI for efficient queries
- **CDN Caching**: Static asset optimization

### Cost Optimization
- **Pay-per-request**: DynamoDB and Lambda billing model
- **Resource Right-sizing**: Memory allocation optimized per function
- **Environment-specific Configuration**: Different resource tiers per environment

## Monitoring & Observability

### CloudWatch Integration
- **Lambda Metrics**: Duration, errors, and throttling
- **API Gateway Metrics**: Request counts and latency
- **DynamoDB Metrics**: Read/write capacity and throttling

### Custom Dashboards
- **Production Environment**: Comprehensive monitoring enabled
- **Development Environment**: Basic monitoring for cost optimization

### Alerting
- **Error Rate Monitoring**: Automated alerts for high error rates
- **Performance Thresholds**: Latency and timeout alerts
- **Resource Utilization**: Capacity and billing alerts

## Security Architecture

### Authentication & Authorization
- **Supabase Integration**: User authentication and management
- **JWT Tokens**: Stateless authorization
- **API Gateway Authorizers**: Request-level security

### Network Security
- **VPC Configuration**: Private subnets for sensitive resources
- **Security Groups**: Restrictive inbound/outbound rules
- **WAF Integration**: Web application firewall (production)

### Data Protection
- **Encryption at Rest**: DynamoDB and S3 encryption
- **Encryption in Transit**: TLS/SSL for all communications
- **Access Logging**: Comprehensive audit trails

## Deployment Strategy

### Environment Promotion
1. **Development**: Feature development and testing
2. **Staging**: Integration testing and UAT
3. **Production**: Live system with full monitoring

### CI/CD Pipeline
- **Infrastructure**: CDK deployment pipeline
- **Application**: Lambda function deployment
- **Frontend**: S3/CloudFront deployment

### Rollback Strategy
- **Infrastructure**: CloudFormation stack rollback
- **Application**: Lambda version management
- **Database**: Point-in-time recovery

## Future Considerations

### Potential Enhancements
- **Multi-Region Deployment**: Global availability and disaster recovery
- **Advanced Caching**: ElastiCache for frequently accessed data
- **ML/AI Integration**: Amazon SageMaker for intelligent features
- **Advanced Analytics**: Amazon QuickSight for business intelligence

### Performance Improvements
- **Database Optimization**: Read replicas and caching layers
- **API Optimization**: GraphQL for efficient data fetching
- **Edge Computing**: Lambda@Edge for regional processing