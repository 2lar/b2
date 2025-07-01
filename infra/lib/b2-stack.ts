/**
 * Brain2 Infrastructure as Code - AWS CDK Stack
 * 
 * This file defines the complete cloud infrastructure for the Brain2 application
 * using AWS CDK (Cloud Development Kit). It demonstrates modern serverless 
 * architecture patterns and infrastructure as code best practices.
 * 
 * KEY ARCHITECTURAL CONCEPTS DEMONSTRATED:
 * 
 * 1. SERVERLESS-FIRST ARCHITECTURE:
 *    - No servers to manage, automatic scaling
 *    - Pay only for what you use
 *    - High availability and fault tolerance built-in
 *    - Lambda functions for compute, DynamoDB for storage
 * 
 * 2. EVENT-DRIVEN ARCHITECTURE:
 *    - Components communicate via events (EventBridge)
 *    - Loose coupling between services
 *    - Async processing for better performance
 *    - Real-time updates via WebSocket
 * 
 * 3. INFRASTRUCTURE AS CODE (IaC):
 *    - Infrastructure defined in TypeScript code
 *    - Version controlled, reviewable, and repeatable
 *    - Automatic resource provisioning and updates
 *    - Environment consistency (dev/staging/prod)
 * 
 * 4. SECURITY BY DESIGN:
 *    - JWT-based authentication with Supabase
 *    - IAM roles with least privilege principle
 *    - HTTPS/WSS encryption for all communication
 *    - No hardcoded secrets (environment variables)
 * 
 * 5. SCALABLE DATA ARCHITECTURE:
 *    - Single-table DynamoDB design for performance
 *    - Global Secondary Indexes for query flexibility
 *    - TTL for automatic cleanup of stale data
 *    - Optimized for read/write patterns
 * 
 * AWS SERVICES USED:
 * - Lambda: Serverless compute for business logic
 * - DynamoDB: NoSQL database for graph data
 * - API Gateway: HTTP and WebSocket API endpoints
 * - EventBridge: Event routing and orchestration
 * - S3: Static website hosting
 * - CloudFront: Global CDN for fast content delivery
 * - IAM: Identity and access management
 */

// Load environment variables from .env file for configuration
import 'dotenv/config';

// AWS CDK core constructs and utilities
import { Stack, StackProps, RemovalPolicy, Duration, CfnOutput } from 'aws-cdk-lib';
import { Construct } from 'constructs';

// AWS service constructs for building cloud infrastructure
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';    // NoSQL database
import * as lambda from 'aws-cdk-lib/aws-lambda';        // Serverless compute
import * as s3 from 'aws-cdk-lib/aws-s3';               // Object storage
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront'; // CDN
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as s3deploy from 'aws-cdk-lib/aws-s3-deployment'; // Asset deployment
import * as iam from 'aws-cdk-lib/aws-iam';             // Identity and access management
import * as events from 'aws-cdk-lib/aws-events';       // Event-driven architecture
import * as targets from 'aws-cdk-lib/aws-events-targets';
import * as path from 'path';                           // File path utilities

// API Gateway v2 constructs for modern HTTP and WebSocket APIs
import * as apigwv2 from 'aws-cdk-lib/aws-apigatewayv2';
import { HttpLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { WebSocketLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { HttpLambdaAuthorizer, HttpLambdaResponseType } from 'aws-cdk-lib/aws-apigatewayv2-authorizers';


/**
 * Brain2 CDK Stack - Complete Infrastructure Definition
 * 
 * This class defines all AWS resources needed for the Brain2 application.
 * CDK stacks are the unit of deployment - everything in this stack will be
 * deployed together as a CloudFormation template.
 * 
 * STACK ORGANIZATION:
 * 1. Environment validation and configuration
 * 2. Event-driven architecture setup (EventBridge)
 * 3. Database layer (DynamoDB tables)
 * 4. Compute layer (Lambda functions)
 * 5. API layer (HTTP and WebSocket APIs)
 * 6. Frontend hosting (S3 + CloudFront)
 * 7. Event rules and integrations
 * 8. Stack outputs for easy access to endpoints
 */
export class b2Stack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    /**
     * ======================
     * ENVIRONMENT CONFIGURATION & VALIDATION
     * ======================
     * 
     * Load and validate configuration from environment variables.
     * This ensures the stack fails fast if required configuration is missing.
     * 
     * CONFIGURATION SOURCES:
     * - .env file (loaded by dotenv/config)
     * - Environment variables
     * - CDK context values
     * 
     * SECURITY NOTE:
     * These environment variables should contain sensitive values:
     * - SUPABASE_URL: Your Supabase project URL
     * - SUPABASE_SERVICE_ROLE_KEY: Service role key for server-side operations
     */
    const SUPABASE_URL = process.env.SUPABASE_URL;
    const SUPABASE_SERVICE_ROLE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY;
    
    // Fail fast validation - better to catch configuration issues early
    if (!SUPABASE_URL || !SUPABASE_SERVICE_ROLE_KEY ) {
      throw new Error('FATAL: SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY must be defined in your environment.');
    }

    /**
     * ======================
     * EVENT-DRIVEN ARCHITECTURE FOUNDATION
     * ======================
     * 
     * EventBridge serves as the central nervous system of our application,
     * enabling loose coupling between components through event-driven communication.
     * 
     * HOW EVENT-DRIVEN ARCHITECTURE WORKS IN BRAIN2:
     * 
     * 1. USER CREATES MEMORY:
     *    Frontend → HTTP API → Main Lambda → Store in DynamoDB → Publish "NodeCreated" event
     * 
     * 2. BACKGROUND PROCESSING:
     *    "NodeCreated" event → ConnectNode Lambda → Find connections → Publish "EdgesCreated" event
     * 
     * 3. REAL-TIME UPDATES:
     *    "EdgesCreated" event → WebSocket Lambda → Push updates to connected clients
     * 
     * BENEFITS OF THIS PATTERN:
     * - RESPONSIVENESS: User gets immediate feedback, heavy processing happens async
     * - SCALABILITY: Each component can scale independently
     * - RELIABILITY: Failed events can be retried automatically
     * - EXTENSIBILITY: Easy to add new event consumers (analytics, notifications, etc.)
     * - DEBUGGING: Complete audit trail of all system events
     * 
     * ALTERNATIVE ARCHITECTURES:
     * - Synchronous: User waits for all processing (slow, poor UX)
     * - Direct coupling: Lambda calls Lambda directly (tight coupling, hard to test)
     * - Message queues: Good for reliability, but more complex for simple use cases
     */
    const eventBus = new events.EventBus(this, 'B2EventBus', {
      eventBusName: 'B2EventBus',
    });

    /**
     * ======================
     * DATABASE LAYER - SINGLE-TABLE DESIGN WITH DYNAMODB
     * ======================
     * 
     * DynamoDB is a fully managed NoSQL database that provides single-digit
     * millisecond latency at any scale. We use a single-table design pattern
     * to store all our data efficiently.
     * 
     * SINGLE-TABLE DESIGN PATTERN:
     * Instead of separate tables for nodes, edges, and keywords (like SQL),
     * we store everything in one table with a carefully designed key structure.
     * 
     * KEY STRUCTURE FOR MEMORY TABLE:
     * 
     * 1. USER NODES (Memories):
     *    PK: USER#{userId}#NODE#{nodeId}
     *    SK: METADATA#v{version}
     *    Data: {content, timestamp, keywords[]}
     * 
     * 2. EDGES (Connections between memories):
     *    PK: USER#{userId}#NODE#{nodeId}
     *    SK: EDGE#RELATES_TO#NODE#{otherNodeId}
     *    Data: {sharedKeywords[], strength}
     * 
     * 3. KEYWORDS (For fast keyword search):
     *    PK: USER#{userId}#NODE#{nodeId}
     *    SK: KEYWORD#{keyword}
     *    Data: {keyword, relevance}
     * 
     * WHY SINGLE-TABLE DESIGN:
     * - PERFORMANCE: One query can fetch node + edges + keywords
     * - COST: No expensive JOINs, fewer read operations
     * - CONSISTENCY: All related data in same partition
     * - SCALABILITY: DynamoDB scales better with fewer tables
     * 
     * LEARNING RESOURCES:
     * - DynamoDB single-table design by Rick Houlihan
     * - AWS re:Invent DynamoDB deep dive sessions
     */
    const memoryTable = new dynamodb.Table(this, 'MemoryTable', {
      tableName: 'brain2',
      // Partition key determines which physical partition stores the data
      partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING },
      // Sort key enables rich query patterns within a partition
      sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },
      // Pay-per-request billing: no capacity planning, scales automatically
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      // DESTROY policy for development - use RETAIN for production!
      removalPolicy: RemovalPolicy.DESTROY,
    });
    
    /**
     * Global Secondary Index - Alternative Query Patterns
     * 
     * GSIs allow us to query data by different attributes than the main table.
     * This index enables keyword-based searches across all memories.
     * 
     * KEYWORD INDEX STRUCTURE:
     * GSI1PK: USER#{userId}#KEYWORD#{keyword}
     * GSI1SK: NODE#{nodeId}
     * 
     * QUERY PATTERNS ENABLED:
     * - Find all memories containing a specific keyword
     * - Get keyword frequency across user's memories
     * - Search for memories with multiple keywords (intersection)
     * 
     * GSI BEST PRACTICES:
     * - Project only needed attributes (ALL vs KEYS_ONLY vs INCLUDE)
     * - Design for your query patterns, not just convenience
     * - Consider write amplification (GSI writes cost extra)
     */
    memoryTable.addGlobalSecondaryIndex({
      indexName: 'KeywordIndex',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },
      // Project all attributes so we can get full data from index queries
      projectionType: dynamodb.ProjectionType.ALL,
    });

    /**
     * WebSocket Connections Table - Real-Time Communication Management
     * 
     * This table tracks active WebSocket connections for real-time features.
     * When users connect via WebSocket, we store their connection info here
     * so we can push updates to the right users.
     * 
     * CONNECTION TRACKING PATTERN:
     * 
     * 1. USER CONNECTS:
     *    WebSocket API → Connect Lambda → Store connection in DynamoDB
     * 
     * 2. SEND UPDATES:
     *    Event occurs → SendMessage Lambda → Query connections → Push to WebSocket API
     * 
     * 3. USER DISCONNECTS:
     *    WebSocket API → Disconnect Lambda → Remove connection from DynamoDB
     * 
     * CONNECTION TABLE STRUCTURE:
     * 
     * MAIN TABLE ACCESS PATTERN (by user):
     * PK: USER#{userId}          - Find all connections for a user
     * SK: CONN#{connectionId}    - Specific connection identifier
     * Data: {connectionId, userId, connectedAt, expireAt}
     * 
     * GSI ACCESS PATTERN (by connection):
     * GSI1PK: CONN#{connectionId} - Find user for a specific connection (disconnect)
     * GSI1SK: USER#{userId}       - User who owns this connection
     * 
     * WHY SEPARATE TABLE:
     * - Different access patterns than main memory data
     * - TTL for automatic cleanup of stale connections
     * - Optimized for real-time lookup performance
     * - Isolation from main business data
     */
    const connectionsTable = new dynamodb.Table(this, 'ConnectionsTable', {
        tableName: 'B2-Connections',
        partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING }, // PK: USER#{userId}
        sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },      // SK: CONN#{connectionId}
        billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
        removalPolicy: RemovalPolicy.DESTROY,
        
        /**
         * Time-To-Live (TTL) for Automatic Cleanup
         * 
         * WebSocket connections can fail ungracefully (network issues, browser crashes),
         * leaving orphaned connection records. TTL automatically deletes stale records.
         * 
         * HOW TTL WORKS:
         * - Set expireAt timestamp when creating connection record
         * - DynamoDB automatically deletes items past their TTL
         * - No additional cost, happens in background
         * - Prevents accumulation of stale connection data
         */
        timeToLiveAttribute: 'expireAt',
    });
    
    /**
     * Global Secondary Index for Reverse Lookup
     * 
     * During disconnect events, we receive only the connectionId from API Gateway.
     * We need to find which user owns that connection to clean up properly.
     * 
     * DISCONNECT LOOKUP FLOW:
     * 1. API Gateway calls disconnect Lambda with connectionId
     * 2. Lambda queries GSI: GSI1PK = CONN#{connectionId}
     * 3. Gets back the userId from GSI1SK
     * 4. Deletes the connection record using PK + SK
     */
    connectionsTable.addGlobalSecondaryIndex({
      indexName: 'connection-id-index',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING }, // GSI1PK: CONN#{connectionId}
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },      // GSI1SK: USER#{userId}
      projectionType: dynamodb.ProjectionType.ALL,
    });


    /**
     * ======================
     * SERVERLESS COMPUTE LAYER - LAMBDA FUNCTIONS
     * ======================
     * 
     * Lambda functions provide the compute layer for our serverless application.
     * Each function has a specific responsibility in our architecture.
     * 
     * LAMBDA ARCHITECTURE PATTERNS:
     * 
     * 1. MICRO-FUNCTIONS: Small, focused functions that do one thing well
     * 2. EVENT-DRIVEN: Functions triggered by HTTP requests, events, or schedules
     * 3. STATELESS: No persistent state between invocations
     * 4. MANAGED RUNTIME: AWS handles scaling, patching, monitoring
     * 
     * FUNCTION CATEGORIES IN BRAIN2:
     * - HTTP API Functions: Handle user requests (CRUD operations)
     * - Event Processing: Background processing triggered by events
     * - WebSocket Functions: Real-time communication management
     * - Authorization: Security and access control
     */

    /**
     * JWT Authorization Lambda - Security Gateway
     * 
     * This function validates JWT tokens from Supabase before allowing access
     * to protected API endpoints. It implements a custom authorizer pattern
     * for API Gateway.
     * 
     * HOW JWT AUTHORIZATION WORKS:
     * 
     * 1. FRONTEND LOGIN:
     *    User logs in → Supabase → Returns JWT token → Frontend stores token
     * 
     * 2. API REQUEST:
     *    Frontend → API Gateway → Custom Authorizer → Validates JWT → Main Lambda
     * 
     * 3. TOKEN VALIDATION:
     *    - Extract token from Authorization header
     *    - Verify signature using Supabase public key
     *    - Check expiration and claims
     *    - Return authorization decision to API Gateway
     * 
     * JWT TOKEN STRUCTURE:
     * {
     *   "sub": "user_id",           // User identifier
     *   "email": "user@email.com",  // User email
     *   "iat": 1234567890,          // Issued at timestamp
     *   "exp": 1234567890,          // Expiration timestamp
     *   "iss": "supabase.url"       // Token issuer
     * }
     * 
     * SECURITY BENEFITS:
     * - Centralized authentication logic
     * - Stateless (no session storage needed)
     * - Cryptographically secure
     * - Built-in expiration handling
     * - User identity available to all backend functions
     * 
     * PERFORMANCE OPTIMIZATION:
     * - API Gateway caches authorization results (5 min TTL)
     * - Reduces repeated JWT validation overhead
     * - Configurable cache key based on token
     */
    const authorizerLambda = new lambda.Function(this, 'JWTAuthorizerLambda', {
      functionName: `${this.stackName}-jwt-authorizer`,
      runtime: lambda.Runtime.NODEJS_20_X,     // Latest stable Node.js runtime
      handler: 'index.handler',                 // Entry point: index.js exports.handler
      code: lambda.Code.fromAsset(path.join(__dirname, '../lambda/authorizer')),
      environment: {
        // Supabase configuration for JWT validation
        SUPABASE_URL: SUPABASE_URL,
        SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
      },
      timeout: Duration.seconds(10),   // Quick timeout for authorization
      memorySize: 128,                 // Minimal memory for simple JWT validation
    });

    /**
     * Main HTTP API Lambda - Synchronous Request Handler
     * 
     * This Go-based Lambda function handles all HTTP API requests for memory
     * management (CRUD operations). It's optimized for quick response times
     * and publishes events for async processing.
     * 
     * DESIGN DECISIONS:
     * 
     * 1. GO RUNTIME: Fast cold starts, efficient memory usage, strong typing
     * 2. SYNCHRONOUS ONLY: Quick responses, heavy processing moved to events
     * 3. EVENT PUBLISHING: Triggers background processing via EventBridge
     * 4. STATELESS: No persistent connections or state between requests
     * 
     * API ENDPOINTS HANDLED:
     * - POST /api/nodes         - Create new memory
     * - GET /api/nodes          - List user's memories
     * - GET /api/nodes/{id}     - Get specific memory details
     * - PUT /api/nodes/{id}     - Update memory content
     * - DELETE /api/nodes/{id}  - Delete memory
     * - POST /api/nodes/bulk-delete - Bulk delete memories
     * - GET /api/graph-data     - Get graph visualization data
     * 
     * REQUEST FLOW:
     * API Gateway → JWT Authorizer → This Lambda → DynamoDB → EventBridge → Response
     * 
     * PERFORMANCE CHARACTERISTICS:
     * - Cold start: ~100ms (Go is fast)
     * - Warm invocation: ~10-50ms
     * - Memory usage: ~20-40MB
     * - Concurrent executions: 1000+ (default limit)
     * 
     * PROVIDED_AL2 RUNTIME:
     * This is AWS's custom runtime for compiled languages like Go.
     * We provide a 'bootstrap' executable that AWS runs directly.
     */
    const backendLambda = new lambda.Function(this, 'BackendLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,    // Custom runtime for Go
      code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/main')),
      handler: 'bootstrap',                    // Standard name for custom runtime
      memorySize: 128,                        // Minimal memory for fast API responses
      timeout: Duration.seconds(30),          // Max time for API requests
      environment: {
        TABLE_NAME: memoryTable.tableName,          // Main data table
        KEYWORD_INDEX_NAME: 'KeywordIndex',         // GSI for keyword searches
      },
    });
    
    /**
     * IAM Permissions - Principle of Least Privilege
     * 
     * Grant only the minimum permissions needed for the function to operate.
     * This follows AWS security best practices.
     */
    memoryTable.grantReadWriteData(backendLambda);  // DynamoDB access
    eventBus.grantPutEventsTo(backendLambda);       // EventBridge publishing

    /**
     * ======================
     * EVENT-DRIVEN & REAL-TIME FUNCTIONS
     * ======================
     * 
     * These functions handle background processing and real-time communication.
     * They're triggered by events rather than direct HTTP requests.
     */

    /**
     * ConnectNode Lambda - Graph Intelligence Engine
     * 
     * This function is the brain of our memory connection system. It runs
     * asynchronously after a memory is created to find connections with
     * existing memories.
     * 
     * TRIGGERED BY: "NodeCreated" events from the main API Lambda
     * 
     * PROCESSING ALGORITHM:
     * 1. Receive NodeCreated event with new memory details
     * 2. Extract keywords from the new memory content
     * 3. Query existing memories with matching keywords (via GSI)
     * 4. Calculate connection strength based on keyword overlap
     * 5. Create edge records for strong connections
     * 6. Publish "EdgesCreated" event for real-time updates
     * 
     * KEYWORD MATCHING STRATEGY:
     * - Extract meaningful keywords (not stop words)
     * - Use stemming/lemmatization for better matching
     * - Weight keywords by importance (TF-IDF)
     * - Consider semantic similarity (future: word embeddings)
     * 
     * CONNECTION STRENGTH CALCULATION:
     * - Simple: Number of shared keywords
     * - Weighted: Sum of keyword importance scores
     * - Advanced: Semantic similarity + keyword overlap
     * 
     * WHY ASYNC PROCESSING:
     * - USER EXPERIENCE: User gets immediate feedback, processing happens in background
     * - SCALABILITY: Heavy computation doesn't block API responses
     * - RELIABILITY: Failed processing can be retried without affecting user
     * - COST: Can use larger memory allocation for complex algorithms
     * 
     * PERFORMANCE CONSIDERATIONS:
     * - Memory: 128MB is sufficient for most graphs
     * - Timeout: 30s allows for complex connection algorithms
     * - Concurrency: Multiple memories can be processed in parallel
     */
    const connectNodeLambda = new lambda.Function(this, 'ConnectNodeLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/connect-node')),
      handler: 'bootstrap',
      memorySize: 128,                    // Can increase for complex algorithms
      timeout: Duration.seconds(30),     // Allow time for graph computation
      environment: {
        TABLE_NAME: memoryTable.tableName,
        KEYWORD_INDEX_NAME: 'KeywordIndex',
      },
    });
    
    // Grant permissions for graph computation
    memoryTable.grantReadWriteData(connectNodeLambda);  // Read existing memories, write edges
    eventBus.grantPutEventsTo(connectNodeLambda);       // Publish edge creation events

    /**
     * WebSocket Connect Lambda - Real-Time Connection Manager
     * 
     * This function handles new WebSocket connections from clients.
     * It validates authentication and stores connection info for real-time updates.
     * 
     * TRIGGERED BY: WebSocket API Gateway when client connects
     * 
     * CONNECTION FLOW:
     * 1. Client opens WebSocket with JWT token in query string
     * 2. API Gateway calls this function with connection details
     * 3. Function validates JWT token with Supabase
     * 4. If valid, stores connection info in DynamoDB
     * 5. Returns success/failure to API Gateway
     * 
     * CONNECTION RECORD STRUCTURE:
     * {
     *   PK: "USER#{userId}",
     *   SK: "CONN#{connectionId}",
     *   connectionId: "abc123",
     *   userId: "user-uuid",
     *   connectedAt: "2024-01-01T12:00:00Z",
     *   expireAt: 1704110400  // Unix timestamp for TTL
     * }
     * 
     * AUTHENTICATION CONSIDERATIONS:
     * - JWT token passed as query parameter (WebSocket limitation)
     * - Token validated against Supabase before allowing connection
     * - Invalid tokens result in connection rejection
     * - Expired tokens handled gracefully
     * 
     * ERROR HANDLING:
     * - Invalid JWT: Return 401 Unauthorized
     * - Database error: Return 500 Internal Server Error
     * - Network issues: API Gateway handles retries
     * 
     * SECURITY NOTES:
     * - Query parameters may be logged - consider rotation frequency
     * - Connection records contain minimal sensitive data
     * - TTL ensures automatic cleanup of stale records
     */
    const wsConnectLambda = new lambda.Function(this, 'wsConnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-connect')),
        handler: 'bootstrap',
        memorySize: 128,                 // Minimal memory for connection handling
        timeout: Duration.seconds(10),  // Quick timeout for connection establishment
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            // Supabase config for JWT validation
            SUPABASE_URL: SUPABASE_URL,
            SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
        },
    });
    
    // Grant permission to store new connection records
    connectionsTable.grantWriteData(wsConnectLambda);

    /**
     * WebSocket Disconnect Lambda - Connection Cleanup Manager
     * 
     * This function handles WebSocket disconnections and cleans up
     * connection records from DynamoDB.
     * 
     * TRIGGERED BY: WebSocket API Gateway when client disconnects
     * 
     * DISCONNECT SCENARIOS:
     * - User closes browser tab/window
     * - Network connectivity lost
     * - User navigates away from page
     * - Server-side connection timeout
     * - Authentication token expires
     * 
     * CLEANUP PROCESS:
     * 1. API Gateway calls function with connectionId
     * 2. Query GSI to find which user owns this connection
     * 3. Delete the connection record from main table
     * 4. Log disconnection for debugging/analytics
     * 
     * WHY GSI LOOKUP IS NEEDED:
     * API Gateway only provides connectionId during disconnect.
     * We need to find the userId to construct the proper DynamoDB key:
     * - Main table key: PK=USER#{userId}, SK=CONN#{connectionId}
     * - GSI lookup: GSI1PK=CONN#{connectionId} → GSI1SK=USER#{userId}
     * 
     * ERROR HANDLING:
     * - Connection not found: Log warning, return success (idempotent)
     * - Database error: Log error, return failure
     * - Multiple connections: Clean up all (shouldn't happen but handle gracefully)
     * 
     * CLEANUP IMPORTANCE:
     * - Prevents accumulation of stale connection records
     * - Reduces DynamoDB storage costs
     * - Keeps connection queries fast and accurate
     * - Essential for accurate real-time user presence
     */
    const wsDisconnectLambda = new lambda.Function(this, 'wsDisconnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-disconnect')),
        handler: 'bootstrap',
        memorySize: 128,                 // Minimal memory for cleanup operation
        timeout: Duration.seconds(10),  // Quick timeout for disconnect handling
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            CONNECTIONS_GSI_NAME: 'connection-id-index',  // For reverse lookup
        },
    });
    
    // Grant permissions for connection cleanup (read to find, write to delete)
    connectionsTable.grantReadWriteData(wsDisconnectLambda);

    /**
     * WebSocket Send Message Lambda - Real-Time Update Broadcaster
     * 
     * This function pushes real-time updates to connected WebSocket clients.
     * It's triggered by events when the graph structure changes.
     * 
     * TRIGGERED BY: "EdgesCreated" events from ConnectNode Lambda
     * 
     * BROADCAST FLOW:
     * 1. Receive "EdgesCreated" event with userId and connection details
     * 2. Query connections table to find all active connections for the user
     * 3. Send "graphUpdated" message to each active connection
     * 4. Handle any failed message deliveries (stale connections)
     * 
     * MESSAGE DELIVERY PROCESS:
     * 1. Look up user's active WebSocket connections in DynamoDB
     * 2. For each connection, call API Gateway Management API
     * 3. Send JSON message: {"action": "graphUpdated"}
     * 4. Handle delivery failures (connection may be stale)
     * 
     * CONNECTION HEALTH MANAGEMENT:
     * - Successful delivery: Connection is healthy
     * - 410 GoneException: Connection is stale, remove from DynamoDB
     * - Other errors: Log for investigation, may retry
     * 
     * SCALING CONSIDERATIONS:
     * - Function can handle multiple concurrent invocations
     * - Each user's connections processed independently
     * - API Gateway Management API has rate limits
     * - Consider batching for users with many connections
     * 
     * MESSAGE TYPES (EXTENSIBLE):
     * Current: {"action": "graphUpdated"}
     * Future: {"action": "memoryCreated", "memory": {...}}
     *         {"action": "userJoined", "user": {...}}
     * 
     * ERROR HANDLING:
     * - Connection not found: Remove stale record
     * - Rate limit exceeded: Implement exponential backoff
     * - Timeout: Log error, event may be retried by EventBridge
     */
    const wsSendMessageLambda = new lambda.Function(this, 'wsSendMessageLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-send-message')),
        handler: 'bootstrap',
        memorySize: 128,                 // Minimal memory for message broadcasting
        timeout: Duration.seconds(10),  // Quick timeout for real-time updates
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            // WebSocket API endpoint URL will be added after API creation
        },
    });
    
    // Grant permission to read active connections
    connectionsTable.grantReadData(wsSendMessageLambda);

    /**
     * ======================
     * API GATEWAY LAYER - HTTP AND WEBSOCKET ENDPOINTS
     * ======================
     * 
     * API Gateway provides the entry point for all client communication.
     * We use both HTTP and WebSocket APIs for different use cases.
     */

    /**
     * HTTP API - RESTful CRUD Operations
     * 
     * This API handles traditional request-response interactions for memory
     * management. It's optimized for low latency and high throughput.
     * 
     * API GATEWAY V2 BENEFITS:
     * - Lower cost than REST API (v1)
     * - Better performance and lower latency
     * - Built-in CORS support
     * - Automatic OpenAPI documentation
     * - Native JWT authorizer support
     * 
     * CORS CONFIGURATION:
     * Cross-Origin Resource Sharing allows frontend to call API from browser.
     * 
     * WHY CORS IS NEEDED:
     * - Frontend hosted on CloudFront domain (e.g., d123456789.cloudfront.net)
     * - API hosted on API Gateway domain (e.g., api123.execute-api.region.amazonaws.com)
     * - Browser blocks cross-origin requests without CORS headers
     * 
     * CORS HEADERS EXPLAINED:
     * - allowHeaders: Client can send these headers in requests
     * - allowMethods: Client can use these HTTP methods
     * - allowOrigins: Domains allowed to make requests (* = any domain)
     * - maxAge: Browser caches preflight responses for 1 day
     * 
     * SECURITY CONSIDERATION:
     * allowOrigins: ['*'] is convenient for development but should be
     * restricted to specific domains in production for better security.
     */
    const httpApi = new apigwv2.HttpApi(this, 'b2HttpApi', {
      apiName: 'b2-http-api',
      corsPreflight: {
        allowHeaders: ['Content-Type', 'Authorization'],  // JWT token + JSON content
        allowMethods: [ 
          apigwv2.CorsHttpMethod.GET,     // Read operations
          apigwv2.CorsHttpMethod.POST,    // Create operations
          apigwv2.CorsHttpMethod.PUT,     // Update operations
          apigwv2.CorsHttpMethod.DELETE,  // Delete operations
          apigwv2.CorsHttpMethod.OPTIONS  // CORS preflight
        ],
        allowOrigins: ['*'],              // TODO: Restrict in production
        maxAge: Duration.days(1),         // Cache preflight for performance
      },
    });

    /**
     * Lambda Authorizer - JWT Validation Integration
     * 
     * This authorizer integrates our JWT validation Lambda with API Gateway.
     * It runs before every API request to verify user authentication.
     * 
     * AUTHORIZER CONFIGURATION:
     * 
     * 1. RESPONSE TYPE - SIMPLE:
     *    Returns a simple allow/deny decision rather than IAM policy
     *    - Simpler to implement and debug
     *    - Lower latency than IAM policy evaluation
     *    - Sufficient for boolean authorization decisions
     * 
     * 2. IDENTITY SOURCE:
     *    Tells API Gateway where to find the auth token
     *    - $request.header.Authorization extracts the Authorization header
     *    - Format expected: "Bearer <jwt_token>"
     *    - This value is passed to the authorizer Lambda
     * 
     * 3. RESULTS CACHE TTL:
     *    API Gateway caches authorization results for performance
     *    - 5 minutes balances security vs performance
     *    - Same token won't trigger Lambda again for 5 minutes
     *    - Reduces Lambda invocations and improves response time
     *    - Cache key includes the token, so different users cached separately
     * 
     * AUTHORIZATION FLOW:
     * 1. Client sends request with Authorization: Bearer <token>
     * 2. API Gateway extracts token from header
     * 3. Checks cache for previous authorization result
     * 4. If not cached, invokes authorizer Lambda
     * 5. Lambda validates JWT and returns allow/deny
     * 6. API Gateway caches result and either forwards or rejects request
     */
    const authorizer = new HttpLambdaAuthorizer('SupabaseLambdaAuthorizer', authorizerLambda, {
      responseTypes: [HttpLambdaResponseType.SIMPLE],      // Simple allow/deny response
      identitySource: ['$request.header.Authorization'],   // Extract JWT from header
      resultsCacheTtl: Duration.minutes(5),               // Cache for performance
    });

    /**
     * API Route Configuration - Proxy Pattern
     * 
     * This route configuration uses the "proxy+" pattern to forward all
     * API requests to our backend Lambda function.
     * 
     * PROXY PATTERN BENEFITS:
     * - Single Lambda handles multiple endpoints (cost efficient)
     * - Flexible routing logic in application code
     * - Easy to add new endpoints without infrastructure changes
     * - Consistent error handling and middleware across all routes
     * 
     * PATH PATTERN EXPLANATION:
     * - '/api/{proxy+}' matches any path starting with /api/
     * - {proxy+} captures the rest of the path as a parameter
     * - Examples:
     *   - /api/nodes → proxy = "nodes"
     *   - /api/nodes/123 → proxy = "nodes/123"
     *   - /api/graph-data → proxy = "graph-data"
     * 
     * INTEGRATION TYPE:
     * HttpLambdaIntegration automatically:
     * - Formats API Gateway event for Lambda
     * - Handles Lambda response transformation
     * - Manages error mapping
     * - Provides timeout and retry logic
     * 
     * AUTHORIZATION:
     * Every route uses the same authorizer, ensuring consistent security.
     * The authorizer runs before the Lambda, so only authenticated requests
     * reach our business logic.
     */
    httpApi.addRoutes({
      path: '/api/{proxy+}',              // Proxy all /api/* requests
      methods: [ 
        apigwv2.HttpMethod.GET,          // Read operations
        apigwv2.HttpMethod.POST,         // Create operations
        apigwv2.HttpMethod.PUT,          // Update operations
        apigwv2.HttpMethod.DELETE        // Delete operations
      ],
      integration: new HttpLambdaIntegration('BackendIntegration', backendLambda),
      authorizer: authorizer,            // Protect all routes with JWT auth
    });

    /**
     * WebSocket API - Real-Time Communication
     * 
     * WebSocket API enables bidirectional, real-time communication between
     * clients and our backend. Unlike HTTP's request-response model,
     * WebSockets maintain persistent connections.
     * 
     * WEBSOCKET vs HTTP COMPARISON:
     * 
     * HTTP (Request-Response):
     * - Client requests → Server responds → Connection closes
     * - Good for: CRUD operations, file uploads, traditional web apps
     * - Limitations: Server can't initiate communication
     * 
     * WebSocket (Bidirectional):
     * - Client connects → Persistent connection → Both can send messages
     * - Good for: Real-time updates, chat, live data, collaborative editing
     * - Benefits: Lower latency, server can push updates
     * 
     * WEBSOCKET LIFECYCLE IN BRAIN2:
     * 1. CONNECT: User opens app → WebSocket connection established
     * 2. AUTHENTICATE: JWT token validated during connection
     * 3. LISTEN: Client waits for real-time updates
     * 4. RECEIVE: Server pushes "graphUpdated" messages
     * 5. DISCONNECT: User closes app → Connection cleaned up
     * 
     * ROUTE HANDLERS:
     * - $connect: Called when client establishes connection
     * - $disconnect: Called when client closes connection
     * - $default: Called for custom message types (not used in Brain2)
     */
    const webSocketApi = new apigwv2.WebSocketApi(this, 'B2WebSocketApi', {
        apiName: 'B2WebSocketApi',
        // Connection establishment handler
        connectRouteOptions: { 
          integration: new WebSocketLambdaIntegration('ConnectIntegration', wsConnectLambda) 
        },
        // Connection termination handler
        disconnectRouteOptions: { 
          integration: new WebSocketLambdaIntegration('DisconnectIntegration', wsDisconnectLambda) 
        },
    });

    /**
     * WebSocket Stage - Deployment Environment
     * 
     * WebSocket APIs require a stage for deployment, similar to HTTP APIs.
     * The stage provides the actual endpoint URL that clients connect to.
     * 
     * STAGE CONFIGURATION:
     * - stageName: 'prod' creates a production deployment
     * - autoDeploy: true automatically deploys changes
     * - callbackUrl: The endpoint for server-to-client messaging
     */
    const webSocketStage = new apigwv2.WebSocketStage(this, 'B2WebSocketStage', {
        webSocketApi,
        stageName: 'prod',              // Production stage
        autoDeploy: true,               // Automatic deployment on changes
    });
    
    /**
     * WebSocket Management Permissions
     * 
     * The SendMessage Lambda needs permission to post messages to WebSocket
     * connections through the API Gateway Management API.
     * 
     * GRANTED PERMISSIONS:
     * - execute-api:ManageConnections: Send messages to specific connections
     * - execute-api:GetConnection: Check connection status
     * - execute-api:PostToConnection: Post data to connection
     */
    webSocketApi.grantManageConnections(wsSendMessageLambda);
    
    /**
     * Dynamic Configuration Injection
     * 
     * The WebSocket API endpoint URL is generated during deployment.
     * We inject it into the SendMessage Lambda's environment so it knows
     * where to send messages.
     * 
     * CALLBACK URL FORMAT:
     * wss://{api-id}.execute-api.{region}.amazonaws.com/{stage}
     * Example: wss://abc123.execute-api.us-east-1.amazonaws.com/prod
     */
    wsSendMessageLambda.addEnvironment('WEBSOCKET_API_ENDPOINT', webSocketStage.callbackUrl);


    /**
     * ======================
     * EVENT-DRIVEN ORCHESTRATION - EVENTBRIDGE RULES
     * ======================
     * 
     * EventBridge rules define which events trigger which functions.
     * This creates the event-driven workflow that powers Brain2's
     * asynchronous processing and real-time updates.
     * 
     * EVENT FLOW VISUALIZATION:
     * 
     * User Action          Event                    Triggered Function           Result
     * -----------          -----                    ------------------           ------
     * Create Memory   →    NodeCreated         →    ConnectNode Lambda      →    Find connections
     * Connections Found →  EdgesCreated        →    SendMessage Lambda      →    Push to WebSocket
     * WebSocket Message →  (none - end of flow) →   Client receives update  →    Refresh graph
     * 
     * WHY EVENT-DRIVEN ARCHITECTURE:
     * 1. DECOUPLING: Components don't know about each other directly
     * 2. SCALABILITY: Each step can scale independently
     * 3. RELIABILITY: Failed events can be retried automatically
     * 4. EXTENSIBILITY: Easy to add new event consumers
     * 5. OBSERVABILITY: Complete audit trail of all system events
     */
    
    /**
     * Event Rule 1: Memory Creation Triggers Connection Processing
     * 
     * When a user creates a new memory through the HTTP API, this rule
     * triggers the ConnectNode Lambda to find connections with existing memories.
     * 
     * EVENT PATTERN MATCHING:
     * - source: 'brain2.api' - Events from the main API Lambda
     * - detailType: 'NodeCreated' - Specifically memory creation events
     * 
     * EVENT PAYLOAD EXAMPLE:
     * {
     *   "version": "0",
     *   "id": "event-id",
     *   "detail-type": "NodeCreated",
     *   "source": "brain2.api",
     *   "account": "123456789012",
     *   "time": "2024-01-01T12:00:00Z",
     *   "region": "us-east-1",
     *   "detail": {
     *     "userId": "user-uuid",
     *     "nodeId": "node-uuid",
     *     "content": "My new memory",
     *     "keywords": ["memory", "new"]
     *   }
     * }
     * 
     * PROCESSING CHARACTERISTICS:
     * - Asynchronous: User doesn't wait for connection processing
     * - Automatic retry: EventBridge retries failed invocations
     * - Dead letter queue: Failed events can be captured for analysis
     */
    new events.Rule(this, 'NodeCreatedRule', {
        eventBus,
        eventPattern: {
            source: ['brain2.api'],           // Events from main API
            detailType: ['NodeCreated'],      // Memory creation events
        },
        targets: [new targets.LambdaFunction(connectNodeLambda)],
    });

    /**
     * Event Rule 2: Connection Discovery Triggers Real-Time Updates
     * 
     * When the ConnectNode Lambda finds new connections between memories,
     * this rule triggers the SendMessage Lambda to notify connected clients
     * via WebSocket.
     * 
     * EVENT PATTERN MATCHING:
     * - source: 'brain2.connectNode' - Events from ConnectNode Lambda
     * - detailType: 'EdgesCreated' - New connections discovered
     * 
     * EVENT PAYLOAD EXAMPLE:
     * {
     *   "version": "0",
     *   "id": "event-id",
     *   "detail-type": "EdgesCreated",
     *   "source": "brain2.connectNode",
     *   "account": "123456789012",
     *   "time": "2024-01-01T12:00:15Z",
     *   "region": "us-east-1",
     *   "detail": {
     *     "userId": "user-uuid",
     *     "sourceNodeId": "node-1-uuid",
     *     "connectedNodes": [
     *       {"nodeId": "node-2-uuid", "strength": 0.8},
     *       {"nodeId": "node-3-uuid", "strength": 0.6}
     *     ]
     *   }
     * }
     * 
     * REAL-TIME UPDATE FLOW:
     * 1. ConnectNode publishes EdgesCreated event
     * 2. This rule triggers SendMessage Lambda
     * 3. SendMessage looks up user's WebSocket connections
     * 4. Sends "graphUpdated" message to each connection
     * 5. Client receives message and refreshes graph visualization
     * 
     * USER EXPERIENCE:
     * User creates memory → Immediate feedback → Graph updates automatically
     * Total time from creation to graph update: typically 1-3 seconds
     */
    new events.Rule(this, 'EdgesCreatedRule', {
        eventBus,
        eventPattern: {
            source: ['brain2.connectNode'],   // Events from connection processing
            detailType: ['EdgesCreated'],     // New edges discovered
        },
        targets: [new targets.LambdaFunction(wsSendMessageLambda)],
    });


    /**
     * ======================
     * FRONTEND HOSTING LAYER - GLOBAL CONTENT DELIVERY
     * ======================
     * 
     * S3 + CloudFront provides secure, scalable, and fast hosting for our
     * single-page application (SPA). This architecture is common for modern
     * web applications and provides excellent performance worldwide.
     * 
     * HOSTING ARCHITECTURE:
     * User Request → CloudFront CDN → S3 Origin → Static Files
     * 
     * BENEFITS OF S3 + CLOUDFRONT:
     * 1. GLOBAL PERFORMANCE: CloudFront edge locations worldwide
     * 2. SECURITY: S3 bucket is private, CloudFront provides secure access
     * 3. SCALABILITY: Handles traffic spikes automatically
     * 4. COST: Pay only for what you use, caching reduces origin requests
     * 5. RELIABILITY: 99.99% availability SLA
     * 6. SSL/TLS: HTTPS encryption built-in
     */
    
    /**
     * S3 Bucket - Static Asset Storage
     * 
     * This bucket stores our compiled frontend assets (HTML, CSS, JavaScript).
     * It's configured for security and easy cleanup during development.
     * 
     * SECURITY CONFIGURATION:
     * - publicReadAccess: false - Bucket is not publicly accessible
     * - blockPublicAccess: BLOCK_ALL - Prevents accidental public exposure
     * - Access only through CloudFront Origin Access Control (OAC)
     * 
     * NAMING STRATEGY:
     * Bucket name includes account ID and region for uniqueness.
     * S3 bucket names must be globally unique across all AWS accounts.
     * 
     * DEVELOPMENT CONVENIENCE:
     * - removalPolicy: DESTROY - Bucket deleted when stack is destroyed
     * - autoDeleteObjects: true - Objects deleted automatically
     * 
     * PRODUCTION CONSIDERATIONS:
     * For production, consider:
     * - removalPolicy: RETAIN - Prevent accidental deletion
     * - Versioning enabled - Keep history of deployments
     * - Lifecycle policies - Manage storage costs
     */
    const frontendBucket = new s3.Bucket(this, 'FrontendBucket', {
      bucketName: `b2-frontend-${this.account}-${this.region}`,  // Globally unique name
      publicReadAccess: false,                    // Security: not publicly accessible
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,  // Prevent accidental exposure
      removalPolicy: RemovalPolicy.DESTROY,      // Development convenience
      autoDeleteObjects: true,                    // Clean up on stack deletion
    });

    /**
     * CloudFront Distribution - Global Content Delivery Network
     * 
     * CloudFront is AWS's CDN that caches and delivers content from edge
     * locations worldwide, providing fast access regardless of user location.
     * 
     * CDN BENEFITS:
     * - PERFORMANCE: Content served from nearest edge location
     * - REDUCED LOAD: Origin (S3) serves content less frequently
     * - DDOS PROTECTION: AWS Shield protection included
     * - COMPRESSION: Automatic gzip/brotli compression
     * - HTTP/2 SUPPORT: Modern protocol support
     * 
     * CONFIGURATION EXPLAINED:
     * 
     * 1. ORIGIN CONFIGURATION:
     *    - S3Origin: CloudFront pulls content from our S3 bucket
     *    - Origin Access Control (OAC): Secure access to private bucket
     *    - No public S3 access needed, only CloudFront can access
     * 
     * 2. SECURITY POLICY:
     *    - REDIRECT_TO_HTTPS: All HTTP requests redirected to HTTPS
     *    - Ensures all communication is encrypted
     *    - Required for modern security standards
     * 
     * 3. CACHING STRATEGY:
     *    - CACHING_OPTIMIZED: AWS managed policy for static websites
     *    - Caches based on file extensions and query strings
     *    - HTML files: short cache (for updates)
     *    - CSS/JS/Images: long cache (versioned filenames)
     * 
     * 4. SPA SUPPORT:
     *    - defaultRootObject: 'index.html' serves SPA entry point
     *    - errorResponses: 404 errors return index.html for client-side routing
     *    - Enables React Router / Vue Router style navigation
     *    - TTL 5 minutes prevents excessive caching of error responses
     */
    const distribution = new cloudfront.Distribution(this, 'FrontendDistribution', {
      defaultBehavior: {
        origin: new origins.S3Origin(frontendBucket),               // Pull from our S3 bucket
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS, // Force HTTPS
        cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,     // Optimized for static sites
      },
      defaultRootObject: 'index.html',          // SPA entry point
      
      /**
       * Single Page Application (SPA) Error Handling
       * 
       * SPAs use client-side routing, meaning URLs like /memories/123 don't
       * correspond to actual files on the server. When users navigate directly
       * to these URLs or refresh the page, the server returns 404.
       * 
       * SOLUTION: Return index.html for all 404 errors
       * - User visits /memories/123
       * - CloudFront looks for /memories/123 file (doesn't exist)
       * - Returns 404, but our rule serves index.html instead
       * - React Router takes over and shows the correct page
       * 
       * WHY 200 STATUS: Search engines and browsers treat this as success
       * WHY 5 MIN TTL: Don't cache errors too long (in case of real 404s)
       */
      errorResponses: [{ 
        httpStatus: 404,                    // File not found
        responseHttpStatus: 200,            // Return as success
        responsePagePath: '/index.html',    // Serve SPA entry point
        ttl: Duration.minutes(5)            // Short cache for errors
      }],
    });

    /**
     * Automated Frontend Deployment
     * 
     * This construct automatically deploys our built frontend assets to S3
     * and invalidates CloudFront cache during CDK deployment.
     * 
     * DEPLOYMENT FLOW:
     * 1. CDK reads files from frontend/dist directory
     * 2. Uploads files to S3 bucket
     * 3. Invalidates CloudFront cache for immediate updates
     * 4. New version available globally within minutes
     * 
     * SOURCE DIRECTORY:
     * frontend/dist contains the output of 'npm run build':
     * - index.html (entry point)
     * - assets/ (CSS, JS, images with content hashes)
     * - Optimized and minified for production
     * 
     * CACHE INVALIDATION:
     * distributionPaths: ['/*'] invalidates all cached content
     * - Ensures users get the latest version immediately
     * - Costs a small amount per invalidation
     * - Alternative: use versioned filenames to avoid invalidation
     * 
     * DEPLOYMENT AUTOMATION:
     * This deployment happens automatically when:
     * - Running 'cdk deploy'
     * - Files in frontend/dist have changed
     * - CDK detects changes and updates S3 + CloudFront
     * 
     * PRODUCTION CONSIDERATIONS:
     * - Consider blue/green deployments for zero downtime
     * - Implement rollback mechanisms for failed deployments
     * - Monitor deployment success with CloudWatch alarms
     */
    new s3deploy.BucketDeployment(this, 'DeployFrontend', {
      sources: [s3deploy.Source.asset(path.join(__dirname, '../../frontend/dist'))],
      destinationBucket: frontendBucket,         // Upload to our S3 bucket
      distribution,                              // Invalidate CloudFront cache
      distributionPaths: ['/*'],                 // Invalidate all cached content
    });

    /**
     * ======================
     * STACK OUTPUTS - DEPLOYMENT INFORMATION
     * ======================
     * 
     * Stack outputs provide important information after deployment.
     * These URLs are needed to configure the frontend and test the system.
     * 
     * OUTPUT USAGE:
     * 1. Copy these URLs after 'cdk deploy' completes
     * 2. Update .env files with the actual endpoint URLs
     * 3. Test each endpoint to verify deployment success
     * 4. Share URLs with team members for testing
     * 
     * AUTOMATION OPPORTUNITIES:
     * - Script to automatically update .env files
     * - Integration with CI/CD pipelines
     * - Automatic testing after deployment
     */
    
    /**
     * HTTP API Endpoint
     * 
     * This is the base URL for all REST API calls.
     * Format: https://{api-id}.execute-api.{region}.amazonaws.com
     * 
     * USAGE IN FRONTEND:
     * Set VITE_API_BASE_URL to this value in your .env file
     */
    new CfnOutput(this, 'HttpApiUrl', { 
      value: httpApi.url!, 
      description: 'The base URL for HTTP API calls (set as VITE_API_BASE_URL)' 
    });
    
    /**
     * WebSocket API Endpoint
     * 
     * This is the URL for WebSocket connections for real-time updates.
     * Format: wss://{api-id}.execute-api.{region}.amazonaws.com/{stage}
     * 
     * USAGE IN FRONTEND:
     * Set VITE_WEBSOCKET_URL to this value in your .env file
     */
    new CfnOutput(this, 'WebSocketApiUrl', { 
      value: webSocketStage.url, 
      description: 'The WebSocket URL for real-time updates (set as VITE_WEBSOCKET_URL)' 
    });
    
    /**
     * Frontend Application URL
     * 
     * This is the public URL where users access your application.
     * CloudFront provides global CDN distribution for fast loading.
     * 
     * FEATURES:
     * - Global CDN for fast access worldwide
     * - HTTPS encryption built-in
     * - Custom domain can be added later
     * - Automatic compression and optimization
     */
    new CfnOutput(this, 'CloudFrontUrl', { 
      value: `https://${distribution.distributionDomainName}`, 
      description: 'The public URL of your Brain2 application' 
    });
  }
}
