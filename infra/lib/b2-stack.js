"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.b2Stack = void 0;
require("dotenv/config"); // Loads variables from .env into process.env
const aws_cdk_lib_1 = require("aws-cdk-lib");
const dynamodb = require("aws-cdk-lib/aws-dynamodb");
const lambda = require("aws-cdk-lib/aws-lambda");
const apigatewayv2 = require("aws-cdk-lib/aws-apigatewayv2");
const apigatewayv2Integrations = require("aws-cdk-lib/aws-apigatewayv2-integrations");
const apigatewayv2Authorizers = require("aws-cdk-lib/aws-apigatewayv2-authorizers");
const events = require("aws-cdk-lib/aws-events");
const eventsTargets = require("aws-cdk-lib/aws-events-targets");
const s3 = require("aws-cdk-lib/aws-s3");
const cloudfront = require("aws-cdk-lib/aws-cloudfront");
const cloudfrontOrigins = require("aws-cdk-lib/aws-cloudfront-origins");
const s3deploy = require("aws-cdk-lib/aws-s3-deployment");
const iam = require("aws-cdk-lib/aws-iam");
const path = require("path");
class b2Stack extends aws_cdk_lib_1.Stack {
    constructor(scope, id, props) {
        super(scope, id, props);
        // Validate required environment variables
        const SUPABASE_URL = process.env.SUPABASE_URL;
        const SUPABASE_SERVICE_ROLE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY;
        if (!SUPABASE_URL) {
            throw new Error('FATAL: SUPABASE_URL is not defined in your environment. Get it from Supabase dashboard > Settings > API > URL');
        }
        if (!SUPABASE_SERVICE_ROLE_KEY) {
            throw new Error('FATAL: SUPABASE_SERVICE_ROLE_KEY is not defined in your environment. Get it from Supabase dashboard > Settings > API > service_role key');
        }
        // ========================================================================
        // DATABASE LAYER - Using DynamoDB for scalable NoSQL storage
        // ========================================================================
        // Initializing DynamoDB Table with Single-Table Design
        // Single-table design stores multiple entity types in one table for better performance
        // and cost efficiency (fewer tables = fewer requests across tables)
        const memoryTable = new dynamodb.Table(this, 'MemoryTable', {
            tableName: 'brain2',
            // PK (Partition Key) determines which physical partition data goes to
            // DynamoDB distributes data across partitions based on PK hash
            partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING },
            // SK (Sort Key) allows multiple items per partition, sorted by this value
            // Together PK+SK create a composite primary key for uniqueness
            sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },
            // PAY_PER_REQUEST = serverless billing, only pay for actual reads/writes
            // Alternative is PROVISIONED where you pay for reserved capacity
            billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
            // DESTROY means table gets deleted when stack is deleted (good for dev/test)
            // Use RETAIN for production to prevent accidental data loss
            removalPolicy: aws_cdk_lib_1.RemovalPolicy.DESTROY,
        });
        // Global Secondary Index (GSI) - Alternative access pattern for the same data
        // Primary table: PK+SK, GSI: GSI1PK+GSI1SK - allows querying by different attributes
        // Example: Primary might be USER#123 + MEMORY#456, GSI might be KEYWORD#python + TIMESTAMP#2024
        memoryTable.addGlobalSecondaryIndex({
            indexName: 'KeywordIndex',
            // GSI has its own partition key - enables keyword-based searches
            partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING },
            // GSI sort key - enables sorting/filtering within keyword groups
            sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },
            // ALL projection = copy all item attributes to GSI (uses more storage but faster queries)
            // Alternative: KEYS_ONLY (just keys) or INCLUDE (specific attributes)
            projectionType: dynamodb.ProjectionType.ALL,
        });
        // Connections Table for WebSocket Connection Management
        const connectionsTable = new dynamodb.Table(this, 'ConnectionsTable', {
            tableName: `${memoryTable.tableName}-Connections`,
            partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING }, // USER#userID
            sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING }, // CONN#connectionID
            billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
            removalPolicy: aws_cdk_lib_1.RemovalPolicy.DESTROY,
        });
        // ========================================================================
        // EVENT-DRIVEN ARCHITECTURE - EventBridge for Decoupled Communication
        // ========================================================================
        // Custom EventBridge Bus for Real-Time Updates
        const eventBus = new events.EventBus(this, 'b2EventBus', {
            eventBusName: 'b2-event-bus',
        });
        // ========================================================================
        // AUTHENTICATION LAYER - JWT Token Validation via Lambda Authorizer
        // ========================================================================
        // Lambda Authorizer Function - Custom auth logic that runs before API requests
        // This validates JWT tokens from Supabase before allowing access to protected routes
        const authorizerLambda = new lambda.Function(this, 'JWTAuthorizerLambda', {
            functionName: `${this.stackName}-jwt-authorizer`,
            // Node.js 20 runtime - AWS managed runtime environment
            runtime: lambda.Runtime.NODEJS_20_X,
            // Entry point: index.js file, handler function
            handler: 'index.handler',
            // Code source: local directory containing the authorizer logic
            code: lambda.Code.fromAsset(path.join(__dirname, '../lambda/authorizer')),
            environment: {
                // Environment variables available to the function at runtime
                SUPABASE_URL: SUPABASE_URL,
                SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
                NODE_ENV: 'production'
            },
            // 10 second timeout - authorizers should be fast to avoid user delays
            timeout: aws_cdk_lib_1.Duration.seconds(10),
            // 128MB memory - minimal for JWT validation (more memory = higher cost but faster execution)
            memorySize: 128,
            // CloudWatch Logs retention - automatically delete old logs to save costs
            logRetention: 7, // Keep logs for 7 days
        });
        // Grant basic permissions to authorizer
        // IAM Policy: defines what AWS services this Lambda can access
        // Principle of least privilege: only grant necessary permissions
        authorizerLambda.addToRolePolicy(new iam.PolicyStatement({
            // CloudWatch Logs permissions - allows Lambda to write debug/error logs
            actions: ['logs:CreateLogGroup', 'logs:CreateLogStream', 'logs:PutLogEvents'],
            // '*' means all log groups - could be more restrictive in production
            resources: ['*'],
        }));
        // ========================================================================
        // BUSINESS LOGIC LAYER - Main API Backend Lambda
        // ========================================================================
        // Main Backend Lambda Function - Handles all API business logic
        const backendLambda = new lambda.Function(this, 'BackendLambda', {
            // PROVIDED_AL2 = bring your own runtime (Amazon Linux 2)
            // Used for compiled languages like Go, Rust, or custom runtimes
            runtime: lambda.Runtime.PROVIDED_AL2, // since backend API is in go
            // Pre-built Go binary from local build directory
            code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build')),
            // 'bootstrap' is the standard entry point for custom runtimes
            // Go builds create a 'bootstrap' executable file
            handler: 'bootstrap',
            // 128MB = minimum Lambda memory allocation (cheapest option)
            // Lambda CPU scales with memory: more memory = more CPU power
            memorySize: 128,
            // 30 second timeout - longer than authorizer since it does more work
            // API Gateway has 29 second limit, so this is close to maximum
            timeout: aws_cdk_lib_1.Duration.seconds(30),
            environment: {
                // Pass DynamoDB table info to the Lambda at runtime
                TABLE_NAME: memoryTable.tableName,
                KEYWORD_INDEX_NAME: 'KeywordIndex',
                EVENT_BUS_NAME: eventBus.eventBusName,
            },
        });
        // Grant Backend Lambda permissions to access DynamoDB
        // This CDK helper automatically creates IAM policies for DynamoDB operations
        // Includes: GetItem, PutItem, UpdateItem, DeleteItem, Query, Scan on table and indexes
        memoryTable.grantReadWriteData(backendLambda);
        // Grant Backend Lambda permissions to publish events to EventBridge
        eventBus.grantPutEventsTo(backendLambda);
        // ========================================================================
        // EVENT-DRIVEN LAMBDA FUNCTIONS - Asynchronous Processing
        // ========================================================================
        // Connect Node Lambda - Processes connection creation asynchronously
        const connectNodeLambda = new lambda.Function(this, 'ConnectNodeLambda', {
            functionName: `${this.stackName}-connect-node`,
            runtime: lambda.Runtime.PROVIDED_AL2,
            code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/connect-node')),
            handler: 'bootstrap',
            memorySize: 256, // More memory for processing connections
            timeout: aws_cdk_lib_1.Duration.seconds(60), // Longer timeout for connection processing
            environment: {
                TABLE_NAME: memoryTable.tableName,
                KEYWORD_INDEX_NAME: 'KeywordIndex',
                EVENT_BUS_NAME: eventBus.eventBusName,
            },
        });
        // Grant Connect Node Lambda permissions
        memoryTable.grantReadWriteData(connectNodeLambda);
        eventBus.grantPutEventsTo(connectNodeLambda);
        // WebSocket Connect Lambda - Handles new WebSocket connections
        const wsConnectLambda = new lambda.Function(this, 'WsConnectLambda', {
            functionName: `${this.stackName}-ws-connect`,
            runtime: lambda.Runtime.PROVIDED_AL2,
            code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-connect')),
            handler: 'bootstrap',
            memorySize: 128,
            timeout: aws_cdk_lib_1.Duration.seconds(30),
            environment: {
                // OLD: TABLE_NAME: memoryTable.tableName,
                // NEW: Add all required env vars for validation
                TABLE_NAME: connectionsTable.tableName, // Use the dedicated connections table name
                SUPABASE_URL: SUPABASE_URL,
                SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
            },
        });
        // Grant WebSocket Connect Lambda permissions
        connectionsTable.grantReadWriteData(wsConnectLambda);
        // WebSocket Disconnect Lambda - Handles WebSocket disconnections
        const wsDisconnectLambda = new lambda.Function(this, 'WsDisconnectLambda', {
            functionName: `${this.stackName}-ws-disconnect`,
            runtime: lambda.Runtime.PROVIDED_AL2,
            code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-disconnect')),
            handler: 'bootstrap',
            memorySize: 128,
            timeout: aws_cdk_lib_1.Duration.seconds(30),
            environment: {
                TABLE_NAME: memoryTable.tableName,
            },
        });
        // Grant WebSocket Disconnect Lambda permissions
        connectionsTable.grantReadWriteData(wsDisconnectLambda);
        // ========================================================================
        // WEBSOCKET API - Real-Time Communication
        // ========================================================================
        // WebSocket API Gateway for real-time updates
        const webSocketApi = new apigatewayv2.WebSocketApi(this, 'WebSocketApi', {
            apiName: 'b2-websocket-api',
            connectRouteOptions: {
                integration: new apigatewayv2Integrations.WebSocketLambdaIntegration('ConnectIntegration', wsConnectLambda),
                // REMOVE the authorizer from here
                // authorizer: new apigatewayv2Authorizers.WebSocketLambdaAuthorizer(...)
            },
            disconnectRouteOptions: {
                integration: new apigatewayv2Integrations.WebSocketLambdaIntegration('DisconnectIntegration', wsDisconnectLambda),
            },
        });
        // WebSocket API Stage
        const webSocketStage = new apigatewayv2.WebSocketStage(this, 'WebSocketStage', {
            webSocketApi,
            stageName: 'prod',
            autoDeploy: true,
        });
        // WebSocket Send Message Lambda - Sends messages to connected clients
        const wsSendMessageLambda = new lambda.Function(this, 'WsSendMessageLambda', {
            functionName: `${this.stackName}-ws-send-message`,
            runtime: lambda.Runtime.PROVIDED_AL2,
            code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-send-message')),
            handler: 'bootstrap',
            memorySize: 128,
            timeout: aws_cdk_lib_1.Duration.seconds(30),
            environment: {
                TABLE_NAME: memoryTable.tableName,
                WEBSOCKET_API_ENDPOINT: `https://${webSocketApi.apiId}.execute-api.${this.region}.amazonaws.com/${webSocketStage.stageName}`,
            },
        });
        // Grant WebSocket Send Message Lambda permissions
        connectionsTable.grantReadData(wsSendMessageLambda);
        // Grant permission to post messages to WebSocket connections
        wsSendMessageLambda.addToRolePolicy(new iam.PolicyStatement({
            actions: ['execute-api:ManageConnections'],
            resources: [
                `arn:aws:execute-api:${this.region}:${this.account}:${webSocketApi.apiId}/*/*`
            ],
        }));
        // ========================================================================
        // EVENTBRIDGE RULES - Event Routing
        // ========================================================================
        // Rule to trigger Connect Node Lambda when NodeCreated event occurs
        new events.Rule(this, 'NodeCreatedRule', {
            eventBus,
            eventPattern: {
                source: ['brain2.nodes'],
                detailType: ['NodeCreated'],
            },
            targets: [new eventsTargets.LambdaFunction(connectNodeLambda)],
        });
        // Rule to trigger WebSocket Send Message Lambda when EdgesCreated event occurs
        new events.Rule(this, 'EdgesCreatedRule', {
            eventBus,
            eventPattern: {
                source: ['brain2.edges'],
                detailType: ['EdgesCreated'],
            },
            targets: [new eventsTargets.LambdaFunction(wsSendMessageLambda)],
        });
        // ========================================================================
        // API GATEWAY LAYER - HTTP API for client-server communication
        // ========================================================================
        // HTTP API Gateway - Modern, faster, cheaper alternative to REST API Gateway
        // Acts as the front door for your backend Lambda functions
        const httpApi = new apigatewayv2.HttpApi(this, 'b2Api', {
            apiName: 'b2-api',
            // CORS (Cross-Origin Resource Sharing) - allows frontend to call API from different domain
            corsPreflight: {
                // Headers that browsers are allowed to send with requests
                allowHeaders: ['Content-Type', 'Authorization'],
                allowMethods: [
                    apigatewayv2.CorsHttpMethod.GET, // Read data
                    apigatewayv2.CorsHttpMethod.POST, // Create data
                    apigatewayv2.CorsHttpMethod.PUT, // Update data
                    apigatewayv2.CorsHttpMethod.DELETE, // Delete data
                    // OPTIONS is a preflight request browsers send automatically
                    // to check what methods/headers are allowed before sending actual request
                    // therefore this is also not needed in the addroutes part  
                    apigatewayv2.CorsHttpMethod.OPTIONS,
                ],
                // '*' allows requests from any domain - should be restricted in production
                // Configure with your CloudFront domain in production for security
                allowOrigins: ['*'],
                // How long browsers can cache CORS preflight responses (reduces requests)
                maxAge: aws_cdk_lib_1.Duration.days(1),
            },
        });
        // Lambda Integration - The bridge between API Gateway and Lambda
        // Think of it like a translator that converts HTTP requests to Lambda events
        // and Lambda responses back to HTTP responses
        // =====
        // can be thought of like the plug between a device and the wall socket
        // for the lambda and apigw to connect you need some way to standardize data format
        // Also handles permissions for API Gateway to invoke the Lambda
        const lambdaIntegration = new apigatewayv2Integrations.HttpLambdaIntegration('BackendIntegration', backendLambda);
        // Lambda Authorizer Configuration - Security layer for API Gateway
        // This runs BEFORE your main Lambda to validate authentication
        const authorizer = new apigatewayv2Authorizers.HttpLambdaAuthorizer('SupabaseLambdaAuthorizer', authorizerLambda, {
            authorizerName: 'SupabaseJWTAuthorizer',
            // Where to find the auth token - looks in Authorization header
            identitySource: ['$request.header.Authorization'],
            // SIMPLE response = just Allow/Deny (vs IAM policies)
            // This is defining a contract here, and thus in the actual code for the
            // lambda, it needs to be formatted into the SIMPLE type.
            responseTypes: [apigatewayv2Authorizers.HttpLambdaResponseType.SIMPLE],
            // Cache successful auth results to avoid re-validating same token
            // Trade-off: performance vs security (shorter cache = more secure)
            resultsCacheTtl: aws_cdk_lib_1.Duration.minutes(5),
        });
        // API Routes Configuration - Define which URLs map to which Lambda
        httpApi.addRoutes({
            // {proxy+} = catch-all route pattern, forwards everything after /api/ namespace to Lambda
            // Example: /api/memories/123 becomes proxy = "memories/123" in Lambda
            path: '/api/{proxy+}',
            // HTTP methods this route accepts - standard REST API operations
            methods: [
                apigatewayv2.HttpMethod.GET, // Read operations
                apigatewayv2.HttpMethod.POST, // Create operations  
                apigatewayv2.HttpMethod.PUT, // Update operations
                apigatewayv2.HttpMethod.DELETE, // Delete operations
            ],
            // Which Lambda to invoke when this route is hit
            integration: lambdaIntegration,
            // Security: require valid JWT token for all these routes
            authorizer: authorizer,
        });
        /*
        
        If you were creating more than one backend lambda or such, then you would
        instantiate more lambdas, configure them, and then add another addRoutes thing
        for each of the different routes that each lambda would take for example:
    
        // Route for all memory-related actions
        httpApi.addRoutes({
          path: '/api/nodes/{proxy+}', // Catches /api/nodes, /api/nodes/123, etc.
          methods: [  ..GET, ..POST, ..PUT, ..DELETE ],
          integration: lambdaIntegration, // <-- Points to the ORIGINAL backendLambda
          authorizer: authorizer,
        });
    
        // NEW Route for all user profile actions
        httpApi.addRoutes({
          path: '/api/profile', // Catches /api/profile
          methods: [ apigatewayv2.HttpMethod.GET, apigatewayv2.HttpMethod.PUT ],
          integration: userProfileIntegration, // <-- Points to the NEW userProfileLambda
          authorizer: authorizer, // You can reuse the same authorizer
        });
    
        but the current implementation takes in ALL the things that are under the
        "api" namespace
    
        */
        // ========================================================================
        // FRONTEND HOSTING LAYER - S3 + CloudFront for static website delivery
        // ========================================================================
        // S3 Bucket for Frontend Static Files (HTML, CSS, JS)
        // S3 = Simple Storage Service, like a file system in the cloud
        const frontendBucket = new s3.Bucket(this, 'FrontendBucket', {
            // Unique bucket name across ALL of AWS globally
            bucketName: `b2-frontend-${this.account}-${this.region}`,
            // Disable direct public access - CloudFront will access it instead
            // This is more secure and allows better caching/performance
            publicReadAccess: false,
            // Block all public access settings for security
            blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
            // Delete bucket when stack is deleted (good for dev/test)
            removalPolicy: aws_cdk_lib_1.RemovalPolicy.DESTROY,
            // Automatically delete all objects when bucket is deleted
            autoDeleteObjects: true,
        });
        // CloudFront Distribution - Global CDN (Content Delivery Network)
        // CDN = network of servers worldwide that cache your content closer to users
        const distribution = new cloudfront.Distribution(this, 'FrontendDistribution', {
            defaultBehavior: {
                // S3Origin automatically sets up secure access between CloudFront and S3
                // Creates Origin Access Control (OAC) and S3 bucket policy
                // This means only CloudFront can access S3, not direct public access
                origin: new cloudfrontOrigins.S3Origin(frontendBucket),
                // ViewerProtocolPolicy controls HTTP vs HTTPS for end users
                // REDIRECT_TO_HTTPS = accept HTTP requests but redirect to HTTPS
                // Alternative options:
                // - ALLOW_ALL: allow both HTTP and HTTPS
                // - HTTPS_ONLY: reject HTTP requests entirely  
                // HTTPS is essential for security (encrypts data, prevents tampering)
                viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
                // CACHING_OPTIMIZED = AWS managed cache policy for static content
                // Caches based on query strings, headers that affect content
                // Long cache times for static assets, shorter for dynamic content
                cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
            },
            // Default file to serve when users visit root domain (/)
            defaultRootObject: 'index.html',
            // Error handling for Single Page Applications (SPA)
            errorResponses: [
                {
                    // When S3 returns 404 (file not found)...
                    httpStatus: 404,
                    // Return 200 OK instead (so browser doesn't show error)
                    responseHttpStatus: 200,
                    // Serve index.html (let React Router handle the route)
                    responsePagePath: '/index.html',
                    // Cache this error response for 5 minutes
                    ttl: aws_cdk_lib_1.Duration.minutes(5),
                },
            ],
        });
        // Automated Frontend Deployment - Uploads built files to S3
        new s3deploy.BucketDeployment(this, 'DeployFrontend', {
            // Source: local build output directory (webpack/vite build creates this)
            sources: [s3deploy.Source.asset(path.join(__dirname, '../../frontend/dist'))],
            // Destination: the S3 bucket we created above
            destinationBucket: frontendBucket,
            // Invalidate CloudFront cache after deployment (so users get new version)
            distribution,
            // Invalidate all paths ('/*') - could be more specific for large sites
            distributionPaths: ['/*'],
            // Cache Control Headers - tell browsers and CloudFront how long to cache files
            cacheControl: [
                // 1 year cache for immutable assets (JS/CSS with hashed filenames)
                s3deploy.CacheControl.fromString('max-age=31536000,public,immutable'),
                // Mark as publicly cacheable (CDNs and browsers can cache)
                s3deploy.CacheControl.setPublic(),
                // 1 hour default cache (for files without specific cache headers)
                s3deploy.CacheControl.maxAge(aws_cdk_lib_1.Duration.hours(1)),
            ],
        });
        // ========================================================================
        // STACK OUTPUTS - Export important values for other stacks or external use
        // ========================================================================
        // Export API Gateway URL - other stacks or CI/CD can reference this
        this.exportValue(httpApi.url, { name: 'ApiUrl' });
        // Export WebSocket API URL - frontend will connect to this for real-time updates
        this.exportValue(webSocketStage.url, { name: 'WebSocketUrl' });
        // Export CloudFront domain - this is what users will visit in their browser
        this.exportValue(distribution.distributionDomainName, { name: 'CloudFrontUrl' });
    }
}
exports.b2Stack = b2Stack;
//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozLCJmaWxlIjoiYjItc3RhY2suanMiLCJzb3VyY2VSb290IjoiIiwic291cmNlcyI6WyJiMi1zdGFjay50cyJdLCJuYW1lcyI6W10sIm1hcHBpbmdzIjoiOzs7QUFBQSx5QkFBdUIsQ0FBQyw2Q0FBNkM7QUFFckUsNkNBQXlFO0FBRXpFLHFEQUFxRDtBQUNyRCxpREFBaUQ7QUFDakQsNkRBQTZEO0FBQzdELHNGQUFzRjtBQUN0RixvRkFBb0Y7QUFDcEYsaURBQWlEO0FBQ2pELGdFQUFnRTtBQUNoRSx5Q0FBeUM7QUFDekMseURBQXlEO0FBQ3pELHdFQUF3RTtBQUN4RSwwREFBMEQ7QUFDMUQsMkNBQTJDO0FBQzNDLDZCQUE2QjtBQUU3QixNQUFhLE9BQVEsU0FBUSxtQkFBSztJQUNoQyxZQUFZLEtBQWdCLEVBQUUsRUFBVSxFQUFFLEtBQWtCO1FBQzFELEtBQUssQ0FBQyxLQUFLLEVBQUUsRUFBRSxFQUFFLEtBQUssQ0FBQyxDQUFDO1FBRXhCLDBDQUEwQztRQUMxQyxNQUFNLFlBQVksR0FBRyxPQUFPLENBQUMsR0FBRyxDQUFDLFlBQVksQ0FBQztRQUM5QyxNQUFNLHlCQUF5QixHQUFHLE9BQU8sQ0FBQyxHQUFHLENBQUMseUJBQXlCLENBQUM7UUFFeEUsSUFBSSxDQUFDLFlBQVksRUFBRSxDQUFDO1lBQ2xCLE1BQU0sSUFBSSxLQUFLLENBQUMsK0dBQStHLENBQUMsQ0FBQztRQUNuSSxDQUFDO1FBRUQsSUFBSSxDQUFDLHlCQUF5QixFQUFFLENBQUM7WUFDL0IsTUFBTSxJQUFJLEtBQUssQ0FBQyx5SUFBeUksQ0FBQyxDQUFDO1FBQzdKLENBQUM7UUFFRCwyRUFBMkU7UUFDM0UsNkRBQTZEO1FBQzdELDJFQUEyRTtRQUUzRSx1REFBdUQ7UUFDdkQsdUZBQXVGO1FBQ3ZGLG9FQUFvRTtRQUNwRSxNQUFNLFdBQVcsR0FBRyxJQUFJLFFBQVEsQ0FBQyxLQUFLLENBQUMsSUFBSSxFQUFFLGFBQWEsRUFBRTtZQUMxRCxTQUFTLEVBQUUsUUFBUTtZQUNuQixzRUFBc0U7WUFDdEUsK0RB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Y2VzQ29udGVudCI6WyJpbXBvcnQgJ2RvdGVudi9jb25maWcnOyAvLyBMb2FkcyB2YXJpYWJsZXMgZnJvbSAuZW52IGludG8gcHJvY2Vzcy5lbnZcblxuaW1wb3J0IHsgU3RhY2ssIFN0YWNrUHJvcHMsIFJlbW92YWxQb2xpY3ksIER1cmF0aW9uIH0gZnJvbSAnYXdzLWNkay1saWInO1xuaW1wb3J0IHsgQ29uc3RydWN0IH0gZnJvbSAnY29uc3RydWN0cyc7XG5pbXBvcnQgKiBhcyBkeW5hbW9kYiBmcm9tICdhd3MtY2RrLWxpYi9hd3MtZHluYW1vZGInO1xuaW1wb3J0ICogYXMgbGFtYmRhIGZyb20gJ2F3cy1jZGstbGliL2F3cy1sYW1iZGEnO1xuaW1wb3J0ICogYXMgYXBpZ2F0ZXdheXYyIGZyb20gJ2F3cy1jZGstbGliL2F3cy1hcGlnYXRld2F5djInO1xuaW1wb3J0ICogYXMgYXBpZ2F0ZXdheXYySW50ZWdyYXRpb25zIGZyb20gJ2F3cy1jZGstbGliL2F3cy1hcGlnYXRld2F5djItaW50ZWdyYXRpb25zJztcbmltcG9ydCAqIGFzIGFwaWdhdGV3YXl2MkF1dGhvcml6ZXJzIGZyb20gJ2F3cy1jZGstbGliL2F3cy1hcGlnYXRld2F5djItYXV0aG9yaXplcnMnO1xuaW1wb3J0ICogYXMgZXZlbnRzIGZyb20gJ2F3cy1jZGstbGliL2F3cy1ldmVudHMnO1xuaW1wb3J0ICogYXMgZXZlbnRzVGFyZ2V0cyBmcm9tICdhd3MtY2RrLWxpYi9hd3MtZXZlbnRzLXRhcmdldHMnO1xuaW1wb3J0ICogYXMgczMgZnJvbSAnYXdzLWNkay1saWIvYXdzLXMzJztcbmltcG9ydCAqIGFzIGNsb3VkZnJvbnQgZnJvbSAnYXdzLWNkay1saWIvYXdzLWNsb3VkZnJvbnQnO1xuaW1wb3J0ICogYXMgY2xvdWRmcm9udE9yaWdpbnMgZnJvbSAnYXdzLWNkay1saWIvYXdzLWNsb3VkZnJvbnQtb3JpZ2lucyc7XG5pbXBvcnQgKiBhcyBzM2RlcGxveSBmcm9tICdhd3MtY2RrLWxpYi9hd3MtczMtZGVwbG95bWVudCc7XG5pbXBvcnQgKiBhcyBpYW0gZnJvbSAnYXdzLWNkay1saWIvYXdzLWlhbSc7XG5pbXBvcnQgKiBhcyBwYXRoIGZyb20gJ3BhdGgnO1xuXG5leHBvcnQgY2xhc3MgYjJTdGFjayBleHRlbmRzIFN0YWNrIHtcbiAgY29uc3RydWN0b3Ioc2NvcGU6IENvbnN0cnVjdCwgaWQ6IHN0cmluZywgcHJvcHM/OiBTdGFja1Byb3BzKSB7XG4gICAgc3VwZXIoc2NvcGUsIGlkLCBwcm9wcyk7XG5cbiAgICAvLyBWYWxpZGF0ZSByZXF1aXJlZCBlbnZpcm9ubWVudCB2YXJpYWJsZXNcbiAgICBjb25zdCBTVVBBQkFTRV9VUkwgPSBwcm9jZXNzLmVudi5TVVBBQkFTRV9VUkw7XG4gICAgY29uc3QgU1VQQUJBU0VfU0VSVklDRV9ST0xFX0tFWSA9IHByb2Nlc3MuZW52LlNVUEFCQVNFX1NFUlZJQ0VfUk9MRV9LRVk7XG4gICAgXG4gICAgaWYgKCFTVVBBQkFTRV9VUkwpIHtcbiAgICAgIHRocm93IG5ldyBFcnJvcignRkFUQUw6IFNVUEFCQVNFX1VSTCBpcyBub3QgZGVmaW5lZCBpbiB5b3VyIGVudmlyb25tZW50LiBHZXQgaXQgZnJvbSBTdXBhYmFzZSBkYXNoYm9hcmQgPiBTZXR0aW5ncyA+IEFQSSA+IFVSTCcpO1xuICAgIH1cbiAgICBcbiAgICBpZiAoIVNVUEFCQVNFX1NFUlZJQ0VfUk9MRV9LRVkpIHtcbiAgICAgIHRocm93IG5ldyBFcnJvcignRkFUQUw6IFNVUEFCQVNFX1NFUlZJQ0VfUk9MRV9LRVkgaXMgbm90IGRlZmluZWQgaW4geW91ciBlbnZpcm9ubWVudC4gR2V0IGl0IGZyb20gU3VwYWJhc2UgZGFzaGJvYXJkID4gU2V0dGluZ3MgPiBBUEkgPiBzZXJ2aWNlX3JvbGUga2V5Jyk7XG4gICAgfVxuXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG4gICAgLy8gREFUQUJBU0UgTEFZRVIgLSBVc2luZyBEeW5hbW9EQiBmb3Igc2NhbGFibGUgTm9TUUwgc3RvcmFnZVxuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuICAgIFxuICAgIC8vIEluaXRpYWxpemluZyBEeW5hbW9EQiBUYWJsZSB3aXRoIFNpbmdsZS1UYWJsZSBEZXNpZ25cbiAgICAvLyBTaW5nbGUtdGFibGUgZGVzaWduIHN0b3JlcyBtdWx0aXBsZSBlbnRpdHkgdHlwZXMgaW4gb25lIHRhYmxlIGZvciBiZXR0ZXIgcGVyZm9ybWFuY2VcbiAgICAvLyBhbmQgY29zdCBlZmZpY2llbmN5IChmZXdlciB0YWJsZXMgPSBmZXdlciByZXF1ZXN0cyBhY3Jvc3MgdGFibGVzKVxuICAgIGNvbnN0IG1lbW9yeVRhYmxlID0gbmV3IGR5bmFtb2RiLlRhYmxlKHRoaXMsICdNZW1vcnlUYWJsZScsIHtcbiAgICAgIHRhYmxlTmFtZTogJ2JyYWluMicsXG4gICAgICAvLyBQSyAoUGFydGl0aW9uIEtleSkgZGV0ZXJtaW5lcyB3aGljaCBwaHlzaWNhbCBwYXJ0aXRpb24gZGF0YSBnb2VzIHRvXG4gICAgICAvLyBEeW5hbW9EQiBkaXN0cmlidXRlcyBkYXRhIGFjcm9zcyBwYXJ0aXRpb25zIGJhc2VkIG9uIFBLIGhhc2hcbiAgICAgIHBhcnRpdGlvbktleTogeyBuYW1lOiAnUEsnLCB0eXBlOiBkeW5hbW9kYi5BdHRyaWJ1dGVUeXBlLlNUUklORyB9LFxuICAgICAgLy8gU0sgKFNvcnQgS2V5KSBhbGxvd3MgbXVsdGlwbGUgaXRlbXMgcGVyIHBhcnRpdGlvbiwgc29ydGVkIGJ5IHRoaXMgdmFsdWVcbiAgICAgIC8vIFRvZ2V0aGVyIFBLK1NLIGNyZWF0ZSBhIGNvbXBvc2l0ZSBwcmltYXJ5IGtleSBmb3IgdW5pcXVlbmVzc1xuICAgICAgc29ydEtleTogeyBuYW1lOiAnU0snLCB0eXBlOiBkeW5hbW9kYi5BdHRyaWJ1dGVUeXBlLlNUUklORyB9LFxuICAgICAgLy8gUEFZX1BFUl9SRVFVRVNUID0gc2VydmVybGVzcyBiaWxsaW5nLCBvbmx5IHBheSBmb3IgYWN0dWFsIHJlYWRzL3dyaXRlc1xuICAgICAgLy8gQWx0ZXJuYXRpdmUgaXMgUFJPVklTSU9ORUQgd2hlcmUgeW91IHBheSBmb3IgcmVzZXJ2ZWQgY2FwYWNpdHlcbiAgICAgIGJpbGxpbmdNb2RlOiBkeW5hbW9kYi5CaWxsaW5nTW9kZS5QQVlfUEVSX1JFUVVFU1QsXG4gICAgICAvLyBERVNUUk9ZIG1lYW5zIHRhYmxlIGdldHMgZGVsZXRlZCB3aGVuIHN0YWNrIGlzIGRlbGV0ZWQgKGdvb2QgZm9yIGRldi90ZXN0KVxuICAgICAgLy8gVXNlIFJFVEFJTiBmb3IgcHJvZHVjdGlvbiB0byBwcmV2ZW50IGFjY2lkZW50YWwgZGF0YSBsb3NzXG4gICAgICByZW1vdmFsUG9saWN5OiBSZW1vdmFsUG9saWN5LkRFU1RST1ksXG4gICAgfSk7XG5cbiAgICAvLyBHbG9iYWwgU2Vjb25kYXJ5IEluZGV4IChHU0kpIC0gQWx0ZXJuYXRpdmUgYWNjZXNzIHBhdHRlcm4gZm9yIHRoZSBzYW1lIGRhdGFcbiAgICAvLyBQcmltYXJ5IHRhYmxlOiBQSytTSywgR1NJOiBHU0kxUEsrR1NJMVNLIC0gYWxsb3dzIHF1ZXJ5aW5nIGJ5IGRpZmZlcmVudCBhdHRyaWJ1dGVzXG4gICAgLy8gRXhhbXBsZTogUHJpbWFyeSBtaWdodCBiZSBVU0VSIzEyMyArIE1FTU9SWSM0NTYsIEdTSSBtaWdodCBiZSBLRVlXT1JEI3B5dGhvbiArIFRJTUVTVEFNUCMyMDI0XG4gICAgbWVtb3J5VGFibGUuYWRkR2xvYmFsU2Vjb25kYXJ5SW5kZXgoe1xuICAgICAgaW5kZXhOYW1lOiAnS2V5d29yZEluZGV4JyxcbiAgICAgIC8vIEdTSSBoYXMgaXRzIG93biBwYXJ0aXRpb24ga2V5IC0gZW5hYmxlcyBrZXl3b3JkLWJhc2VkIHNlYXJjaGVzXG4gICAgICBwYXJ0aXRpb25LZXk6IHsgbmFtZTogJ0dTSTFQSycsIHR5cGU6IGR5bmFtb2RiLkF0dHJpYnV0ZVR5cGUuU1RSSU5HIH0sXG4gICAgICAvLyBHU0kgc29ydCBrZXkgLSBlbmFibGVzIHNvcnRpbmcvZmlsdGVyaW5nIHdpdGhpbiBrZXl3b3JkIGdyb3Vwc1xuICAgICAgc29ydEtleTogeyBuYW1lOiAnR1NJMVNLJywgdHlwZTogZHluYW1vZGIuQXR0cmlidXRlVHlwZS5TVFJJTkcgfSxcbiAgICAgIC8vIEFMTCBwcm9qZWN0aW9uID0gY29weSBhbGwgaXRlbSBhdHRyaWJ1dGVzIHRvIEdTSSAodXNlcyBtb3JlIHN0b3JhZ2UgYnV0IGZhc3RlciBxdWVyaWVzKVxuICAgICAgLy8gQWx0ZXJuYXRpdmU6IEtFWVNfT05MWSAoanVzdCBrZXlzKSBvciBJTkNMVURFIChzcGVjaWZpYyBhdHRyaWJ1dGVzKVxuICAgICAgcHJvamVjdGlvblR5cGU6IGR5bmFtb2RiLlByb2plY3Rpb25UeXBlLkFMTCxcbiAgICB9KTtcblxuICAgIC8vIENvbm5lY3Rpb25zIFRhYmxlIGZvciBXZWJTb2NrZXQgQ29ubmVjdGlvbiBNYW5hZ2VtZW50XG4gICAgY29uc3QgY29ubmVjdGlvbnNUYWJsZSA9IG5ldyBkeW5hbW9kYi5UYWJsZSh0aGlzLCAnQ29ubmVjdGlvbnNUYWJsZScsIHtcbiAgICAgIHRhYmxlTmFtZTogYCR7bWVtb3J5VGFibGUudGFibGVOYW1lfS1Db25uZWN0aW9uc2AsXG4gICAgICBwYXJ0aXRpb25LZXk6IHsgbmFtZTogJ1BLJywgdHlwZTogZHluYW1vZGIuQXR0cmlidXRlVHlwZS5TVFJJTkcgfSwgLy8gVVNFUiN1c2VySURcbiAgICAgIHNvcnRLZXk6IHsgbmFtZTogJ1NLJywgdHlwZTogZHluYW1vZGIuQXR0cmlidXRlVHlwZS5TVFJJTkcgfSwgLy8gQ09OTiNjb25uZWN0aW9uSURcbiAgICAgIGJpbGxpbmdNb2RlOiBkeW5hbW9kYi5CaWxsaW5nTW9kZS5QQVlfUEVSX1JFUVVFU1QsXG4gICAgICByZW1vdmFsUG9saWN5OiBSZW1vdmFsUG9saWN5LkRFU1RST1ksXG4gICAgfSk7XG5cbiAgICAvLyA9PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT1cbiAgICAvLyBFVkVOVC1EUklWRU4gQVJDSElURUNUVVJFIC0gRXZlbnRCcmlkZ2UgZm9yIERlY291cGxlZCBDb21tdW5pY2F0aW9uXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG5cbiAgICAvLyBDdXN0b20gRXZlbnRCcmlkZ2UgQnVzIGZvciBSZWFsLVRpbWUgVXBkYXRlc1xuICAgIGNvbnN0IGV2ZW50QnVzID0gbmV3IGV2ZW50cy5FdmVudEJ1cyh0aGlzLCAnYjJFdmVudEJ1cycsIHtcbiAgICAgIGV2ZW50QnVzTmFtZTogJ2IyLWV2ZW50LWJ1cycsXG4gICAgfSk7XG5cbiAgICAvLyA9PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT1cbiAgICAvLyBBVVRIRU5USUNBVElPTiBMQVlFUiAtIEpXVCBUb2tlbiBWYWxpZGF0aW9uIHZpYSBMYW1iZGEgQXV0aG9yaXplclxuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuXG4gICAgLy8gTGFtYmRhIEF1dGhvcml6ZXIgRnVuY3Rpb24gLSBDdXN0b20gYXV0aCBsb2dpYyB0aGF0IHJ1bnMgYmVmb3JlIEFQSSByZXF1ZXN0c1xuICAgIC8vIFRoaXMgdmFsaWRhdGVzIEpXVCB0b2tlbnMgZnJvbSBTdXBhYmFzZSBiZWZvcmUgYWxsb3dpbmcgYWNjZXNzIHRvIHByb3RlY3RlZCByb3V0ZXNcbiAgICBjb25zdCBhdXRob3JpemVyTGFtYmRhID0gbmV3IGxhbWJkYS5GdW5jdGlvbih0aGlzLCAnSldUQXV0aG9yaXplckxhbWJkYScsIHtcbiAgICAgIGZ1bmN0aW9uTmFtZTogYCR7dGhpcy5zdGFja05hbWV9LWp3dC1hdXRob3JpemVyYCxcbiAgICAgIC8vIE5vZGUuanMgMjAgcnVudGltZSAtIEFXUyBtYW5hZ2VkIHJ1bnRpbWUgZW52aXJvbm1lbnRcbiAgICAgIHJ1bnRpbWU6IGxhbWJkYS5SdW50aW1lLk5PREVKU18yMF9YLFxuICAgICAgLy8gRW50cnkgcG9pbnQ6IGluZGV4LmpzIGZpbGUsIGhhbmRsZXIgZnVuY3Rpb25cbiAgICAgIGhhbmRsZXI6ICdpbmRleC5oYW5kbGVyJyxcbiAgICAgIC8vIENvZGUgc291cmNlOiBsb2NhbCBkaXJlY3RvcnkgY29udGFpbmluZyB0aGUgYXV0aG9yaXplciBsb2dpY1xuICAgICAgY29kZTogbGFtYmRhLkNvZGUuZnJvbUFzc2V0KHBhdGguam9pbihfX2Rpcm5hbWUsICcuLi9sYW1iZGEvYXV0aG9yaXplcicpKSxcbiAgICAgIGVudmlyb25tZW50OiB7XG4gICAgICAgIC8vIEVudmlyb25tZW50IHZhcmlhYmxlcyBhdmFpbGFibGUgdG8gdGhlIGZ1bmN0aW9uIGF0IHJ1bnRpbWVcbiAgICAgICAgU1VQQUJBU0VfVVJMOiBTVVBBQkFTRV9VUkwsXG4gICAgICAgIFNVUEFCQVNFX1NFUlZJQ0VfUk9MRV9LRVk6IFNVUEFCQVNFX1NFUlZJQ0VfUk9MRV9LRVksXG4gICAgICAgIE5PREVfRU5WOiAncHJvZHVjdGlvbidcbiAgICAgIH0sXG4gICAgICAvLyAxMCBzZWNvbmQgdGltZW91dCAtIGF1dGhvcml6ZXJzIHNob3VsZCBiZSBmYXN0IHRvIGF2b2lkIHVzZXIgZGVsYXlzXG4gICAgICB0aW1lb3V0OiBEdXJhdGlvbi5zZWNvbmRzKDEwKSxcbiAgICAgIC8vIDEyOE1CIG1lbW9yeSAtIG1pbmltYWwgZm9yIEpXVCB2YWxpZGF0aW9uIChtb3JlIG1lbW9yeSA9IGhpZ2hlciBjb3N0IGJ1dCBmYXN0ZXIgZXhlY3V0aW9uKVxuICAgICAgbWVtb3J5U2l6ZTogMTI4LFxuICAgICAgLy8gQ2xvdWRXYXRjaCBMb2dzIHJldGVudGlvbiAtIGF1dG9tYXRpY2FsbHkgZGVsZXRlIG9sZCBsb2dzIHRvIHNhdmUgY29zdHNcbiAgICAgIGxvZ1JldGVudGlvbjogNywgLy8gS2VlcCBsb2dzIGZvciA3IGRheXNcbiAgICB9KTtcblxuICAgIC8vIEdyYW50IGJhc2ljIHBlcm1pc3Npb25zIHRvIGF1dGhvcml6ZXJcbiAgICAvLyBJQU0gUG9saWN5OiBkZWZpbmVzIHdoYXQgQVdTIHNlcnZpY2VzIHRoaXMgTGFtYmRhIGNhbiBhY2Nlc3NcbiAgICAvLyBQcmluY2lwbGUgb2YgbGVhc3QgcHJpdmlsZWdlOiBvbmx5IGdyYW50IG5lY2Vzc2FyeSBwZXJtaXNzaW9uc1xuICAgIGF1dGhvcml6ZXJMYW1iZGEuYWRkVG9Sb2xlUG9saWN5KG5ldyBpYW0uUG9saWN5U3RhdGVtZW50KHtcbiAgICAgIC8vIENsb3VkV2F0Y2ggTG9ncyBwZXJtaXNzaW9ucyAtIGFsbG93cyBMYW1iZGEgdG8gd3JpdGUgZGVidWcvZXJyb3IgbG9nc1xuICAgICAgYWN0aW9uczogWydsb2dzOkNyZWF0ZUxvZ0dyb3VwJywgJ2xvZ3M6Q3JlYXRlTG9nU3RyZWFtJywgJ2xvZ3M6UHV0TG9nRXZlbnRzJ10sXG4gICAgICAvLyAnKicgbWVhbnMgYWxsIGxvZyBncm91cHMgLSBjb3VsZCBiZSBtb3JlIHJlc3RyaWN0aXZlIGluIHByb2R1Y3Rpb25cbiAgICAgIHJlc291cmNlczogWycqJ10sXG4gICAgfSkpO1xuXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG4gICAgLy8gQlVTSU5FU1MgTE9HSUMgTEFZRVIgLSBNYWluIEFQSSBCYWNrZW5kIExhbWJkYVxuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuXG4gICAgLy8gTWFpbiBCYWNrZW5kIExhbWJkYSBGdW5jdGlvbiAtIEhhbmRsZXMgYWxsIEFQSSBidXNpbmVzcyBsb2dpY1xuICAgIGNvbnN0IGJhY2tlbmRMYW1iZGEgPSBuZXcgbGFtYmRhLkZ1bmN0aW9uKHRoaXMsICdCYWNrZW5kTGFtYmRhJywge1xuICAgICAgLy8gUFJPVklERURfQUwyID0gYnJpbmcgeW91ciBvd24gcnVudGltZSAoQW1hem9uIExpbnV4IDIpXG4gICAgICAvLyBVc2VkIGZvciBjb21waWxlZCBsYW5ndWFnZXMgbGlrZSBHbywgUnVzdCwgb3IgY3VzdG9tIHJ1bnRpbWVzXG4gICAgICBydW50aW1lOiBsYW1iZGEuUnVudGltZS5QUk9WSURFRF9BTDIsIC8vIHNpbmNlIGJhY2tlbmQgQVBJIGlzIGluIGdvXG4gICAgICAvLyBQcmUtYnVpbHQgR28gYmluYXJ5IGZyb20gbG9jYWwgYnVpbGQgZGlyZWN0b3J5XG4gICAgICBjb2RlOiBsYW1iZGEuQ29kZS5mcm9tQXNzZXQocGF0aC5qb2luKF9fZGlybmFtZSwgJy4uLy4uL2JhY2tlbmQvYnVpbGQnKSksXG4gICAgICAvLyAnYm9vdHN0cmFwJyBpcyB0aGUgc3RhbmRhcmQgZW50cnkgcG9pbnQgZm9yIGN1c3RvbSBydW50aW1lc1xuICAgICAgLy8gR28gYnVpbGRzIGNyZWF0ZSBhICdib290c3RyYXAnIGV4ZWN1dGFibGUgZmlsZVxuICAgICAgaGFuZGxlcjogJ2Jvb3RzdHJhcCcsXG4gICAgICAvLyAxMjhNQiA9IG1pbmltdW0gTGFtYmRhIG1lbW9yeSBhbGxvY2F0aW9uIChjaGVhcGVzdCBvcHRpb24pXG4gICAgICAvLyBMYW1iZGEgQ1BVIHNjYWxlcyB3aXRoIG1lbW9yeTogbW9yZSBtZW1vcnkgPSBtb3JlIENQVSBwb3dlclxuICAgICAgbWVtb3J5U2l6ZTogMTI4LFxuICAgICAgLy8gMzAgc2Vjb25kIHRpbWVvdXQgLSBsb25nZXIgdGhhbiBhdXRob3JpemVyIHNpbmNlIGl0IGRvZXMgbW9yZSB3b3JrXG4gICAgICAvLyBBUEkgR2F0ZXdheSBoYXMgMjkgc2Vjb25kIGxpbWl0LCBzbyB0aGlzIGlzIGNsb3NlIHRvIG1heGltdW1cbiAgICAgIHRpbWVvdXQ6IER1cmF0aW9uLnNlY29uZHMoMzApLFxuICAgICAgZW52aXJvbm1lbnQ6IHtcbiAgICAgICAgLy8gUGFzcyBEeW5hbW9EQiB0YWJsZSBpbmZvIHRvIHRoZSBMYW1iZGEgYXQgcnVudGltZVxuICAgICAgICBUQUJMRV9OQU1FOiBtZW1vcnlUYWJsZS50YWJsZU5hbWUsXG4gICAgICAgIEtFWVdPUkRfSU5ERVhfTkFNRTogJ0tleXdvcmRJbmRleCcsXG4gICAgICAgIEVWRU5UX0JVU19OQU1FOiBldmVudEJ1cy5ldmVudEJ1c05hbWUsXG4gICAgICB9LFxuICAgIH0pO1xuXG4gICAgLy8gR3JhbnQgQmFja2VuZCBMYW1iZGEgcGVybWlzc2lvbnMgdG8gYWNjZXNzIER5bmFtb0RCXG4gICAgLy8gVGhpcyBDREsgaGVscGVyIGF1dG9tYXRpY2FsbHkgY3JlYXRlcyBJQU0gcG9saWNpZXMgZm9yIER5bmFtb0RCIG9wZXJhdGlvbnNcbiAgICAvLyBJbmNsdWRlczogR2V0SXRlbSwgUHV0SXRlbSwgVXBkYXRlSXRlbSwgRGVsZXRlSXRlbSwgUXVlcnksIFNjYW4gb24gdGFibGUgYW5kIGluZGV4ZXNcbiAgICBtZW1vcnlUYWJsZS5ncmFudFJlYWRXcml0ZURhdGEoYmFja2VuZExhbWJkYSk7XG5cbiAgICAvLyBHcmFudCBCYWNrZW5kIExhbWJkYSBwZXJtaXNzaW9ucyB0byBwdWJsaXNoIGV2ZW50cyB0byBFdmVudEJyaWRnZVxuICAgIGV2ZW50QnVzLmdyYW50UHV0RXZlbnRzVG8oYmFja2VuZExhbWJkYSk7XG5cbiAgICAvLyA9PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT1cbiAgICAvLyBFVkVOVC1EUklWRU4gTEFNQkRBIEZVTkNUSU9OUyAtIEFzeW5jaHJvbm91cyBQcm9jZXNzaW5nXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG5cbiAgICAvLyBDb25uZWN0IE5vZGUgTGFtYmRhIC0gUHJvY2Vzc2VzIGNvbm5lY3Rpb24gY3JlYXRpb24gYXN5bmNocm9ub3VzbHlcbiAgICBjb25zdCBjb25uZWN0Tm9kZUxhbWJkYSA9IG5ldyBsYW1iZGEuRnVuY3Rpb24odGhpcywgJ0Nvbm5lY3ROb2RlTGFtYmRhJywge1xuICAgICAgZnVuY3Rpb25OYW1lOiBgJHt0aGlzLnN0YWNrTmFtZX0tY29ubmVjdC1ub2RlYCxcbiAgICAgIHJ1bnRpbWU6IGxhbWJkYS5SdW50aW1lLlBST1ZJREVEX0FMMixcbiAgICAgIGNvZGU6IGxhbWJkYS5Db2RlLmZyb21Bc3NldChwYXRoLmpvaW4oX19kaXJuYW1lLCAnLi4vLi4vYmFja2VuZC9idWlsZC9jb25uZWN0LW5vZGUnKSksXG4gICAgICBoYW5kbGVyOiAnYm9vdHN0cmFwJyxcbiAgICAgIG1lbW9yeVNpemU6IDI1NiwgLy8gTW9yZSBtZW1vcnkgZm9yIHByb2Nlc3NpbmcgY29ubmVjdGlvbnNcbiAgICAgIHRpbWVvdXQ6IER1cmF0aW9uLnNlY29uZHMoNjApLCAvLyBMb25nZXIgdGltZW91dCBmb3IgY29ubmVjdGlvbiBwcm9jZXNzaW5nXG4gICAgICBlbnZpcm9ubWVudDoge1xuICAgICAgICBUQUJMRV9OQU1FOiBtZW1vcnlUYWJsZS50YWJsZU5hbWUsXG4gICAgICAgIEtFWVdPUkRfSU5ERVhfTkFNRTogJ0tleXdvcmRJbmRleCcsXG4gICAgICAgIEVWRU5UX0JVU19OQU1FOiBldmVudEJ1cy5ldmVudEJ1c05hbWUsXG4gICAgICB9LFxuICAgIH0pO1xuXG4gICAgLy8gR3JhbnQgQ29ubmVjdCBOb2RlIExhbWJkYSBwZXJtaXNzaW9uc1xuICAgIG1lbW9yeVRhYmxlLmdyYW50UmVhZFdyaXRlRGF0YShjb25uZWN0Tm9kZUxhbWJkYSk7XG4gICAgZXZlbnRCdXMuZ3JhbnRQdXRFdmVudHNUbyhjb25uZWN0Tm9kZUxhbWJkYSk7XG5cbiAgICAvLyBXZWJTb2NrZXQgQ29ubmVjdCBMYW1iZGEgLSBIYW5kbGVzIG5ldyBXZWJTb2NrZXQgY29ubmVjdGlvbnNcbiAgICBjb25zdCB3c0Nvbm5lY3RMYW1iZGEgPSBuZXcgbGFtYmRhLkZ1bmN0aW9uKHRoaXMsICdXc0Nvbm5lY3RMYW1iZGEnLCB7XG4gICAgICBmdW5jdGlvbk5hbWU6IGAke3RoaXMuc3RhY2tOYW1lfS13cy1jb25uZWN0YCxcbiAgICAgIHJ1bnRpbWU6IGxhbWJkYS5SdW50aW1lLlBST1ZJREVEX0FMMixcbiAgICAgIGNvZGU6IGxhbWJkYS5Db2RlLmZyb21Bc3NldChwYXRoLmpvaW4oX19kaXJuYW1lLCAnLi4vLi4vYmFja2VuZC9idWlsZC93cy1jb25uZWN0JykpLFxuICAgICAgaGFuZGxlcjogJ2Jvb3RzdHJhcCcsXG4gICAgICBtZW1vcnlTaXplOiAxMjgsXG4gICAgICB0aW1lb3V0OiBEdXJhdGlvbi5zZWNvbmRzKDMwKSxcbiAgICAgIGVudmlyb25tZW50OiB7XG4gICAgICAgIC8vIE9MRDogVEFCTEVfTkFNRTogbWVtb3J5VGFibGUudGFibGVOYW1lLFxuICAgICAgICAvLyBORVc6IEFkZCBhbGwgcmVxdWlyZWQgZW52IHZhcnMgZm9yIHZhbGlkYXRpb25cbiAgICAgICAgVEFCTEVfTkFNRTogY29ubmVjdGlvbnNUYWJsZS50YWJsZU5hbWUsIC8vIFVzZSB0aGUgZGVkaWNhdGVkIGNvbm5lY3Rpb25zIHRhYmxlIG5hbWVcbiAgICAgICAgU1VQQUJBU0VfVVJMOiBTVVBBQkFTRV9VUkwsXG4gICAgICAgIFNVUEFCQVNFX1NFUlZJQ0VfUk9MRV9LRVk6IFNVUEFCQVNFX1NFUlZJQ0VfUk9MRV9LRVksXG4gICAgICB9LFxuICAgIH0pO1xuXG4gICAgLy8gR3JhbnQgV2ViU29ja2V0IENvbm5lY3QgTGFtYmRhIHBlcm1pc3Npb25zXG4gICAgY29ubmVjdGlvbnNUYWJsZS5ncmFudFJlYWRXcml0ZURhdGEod3NDb25uZWN0TGFtYmRhKTtcblxuICAgIC8vIFdlYlNvY2tldCBEaXNjb25uZWN0IExhbWJkYSAtIEhhbmRsZXMgV2ViU29ja2V0IGRpc2Nvbm5lY3Rpb25zXG4gICAgY29uc3Qgd3NEaXNjb25uZWN0TGFtYmRhID0gbmV3IGxhbWJkYS5GdW5jdGlvbih0aGlzLCAnV3NEaXNjb25uZWN0TGFtYmRhJywge1xuICAgICAgZnVuY3Rpb25OYW1lOiBgJHt0aGlzLnN0YWNrTmFtZX0td3MtZGlzY29ubmVjdGAsXG4gICAgICBydW50aW1lOiBsYW1iZGEuUnVudGltZS5QUk9WSURFRF9BTDIsXG4gICAgICBjb2RlOiBsYW1iZGEuQ29kZS5mcm9tQXNzZXQocGF0aC5qb2luKF9fZGlybmFtZSwgJy4uLy4uL2JhY2tlbmQvYnVpbGQvd3MtZGlzY29ubmVjdCcpKSxcbiAgICAgIGhhbmRsZXI6ICdib290c3RyYXAnLFxuICAgICAgbWVtb3J5U2l6ZTogMTI4LFxuICAgICAgdGltZW91dDogRHVyYXRpb24uc2Vjb25kcygzMCksXG4gICAgICBlbnZpcm9ubWVudDoge1xuICAgICAgICBUQUJMRV9OQU1FOiBtZW1vcnlUYWJsZS50YWJsZU5hbWUsXG4gICAgICB9LFxuICAgIH0pO1xuXG4gICAgLy8gR3JhbnQgV2ViU29ja2V0IERpc2Nvbm5lY3QgTGFtYmRhIHBlcm1pc3Npb25zXG4gICAgY29ubmVjdGlvbnNUYWJsZS5ncmFudFJlYWRXcml0ZURhdGEod3NEaXNjb25uZWN0TGFtYmRhKTtcblxuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuICAgIC8vIFdFQlNPQ0tFVCBBUEkgLSBSZWFsLVRpbWUgQ29tbXVuaWNhdGlvblxuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuXG4gICAgLy8gV2ViU29ja2V0IEFQSSBHYXRld2F5IGZvciByZWFsLXRpbWUgdXBkYXRlc1xuICAgIGNvbnN0IHdlYlNvY2tldEFwaSA9IG5ldyBhcGlnYXRld2F5djIuV2ViU29ja2V0QXBpKHRoaXMsICdXZWJTb2NrZXRBcGknLCB7XG4gICAgICBhcGlOYW1lOiAnYjItd2Vic29ja2V0LWFwaScsXG4gICAgICBjb25uZWN0Um91dGVPcHRpb25zOiB7XG4gICAgICAgIGludGVncmF0aW9uOiBuZXcgYXBpZ2F0ZXdheXYySW50ZWdyYXRpb25zLldlYlNvY2tldExhbWJkYUludGVncmF0aW9uKFxuICAgICAgICAgICdDb25uZWN0SW50ZWdyYXRpb24nLFxuICAgICAgICAgIHdzQ29ubmVjdExhbWJkYVxuICAgICAgICApLFxuICAgICAgICAvLyBSRU1PVkUgdGhlIGF1dGhvcml6ZXIgZnJvbSBoZXJlXG4gICAgICAgIC8vIGF1dGhvcml6ZXI6IG5ldyBhcGlnYXRld2F5djJBdXRob3JpemVycy5XZWJTb2NrZXRMYW1iZGFBdXRob3JpemVyKC4uLilcbiAgICAgIH0sXG4gICAgICBkaXNjb25uZWN0Um91dGVPcHRpb25zOiB7XG4gICAgICAgIGludGVncmF0aW9uOiBuZXcgYXBpZ2F0ZXdheXYySW50ZWdyYXRpb25zLldlYlNvY2tldExhbWJkYUludGVncmF0aW9uKFxuICAgICAgICAgICdEaXNjb25uZWN0SW50ZWdyYXRpb24nLFxuICAgICAgICAgIHdzRGlzY29ubmVjdExhbWJkYVxuICAgICAgICApLFxuICAgICAgfSxcbiAgICB9KTtcblxuICAgIC8vIFdlYlNvY2tldCBBUEkgU3RhZ2VcbiAgICBjb25zdCB3ZWJTb2NrZXRTdGFnZSA9IG5ldyBhcGlnYXRld2F5djIuV2ViU29ja2V0U3RhZ2UodGhpcywgJ1dlYlNvY2tldFN0YWdlJywge1xuICAgICAgd2ViU29ja2V0QXBpLFxuICAgICAgc3RhZ2VOYW1lOiAncHJvZCcsXG4gICAgICBhdXRvRGVwbG95OiB0cnVlLFxuICAgIH0pO1xuXG4gICAgLy8gV2ViU29ja2V0IFNlbmQgTWVzc2FnZSBMYW1iZGEgLSBTZW5kcyBtZXNzYWdlcyB0byBjb25uZWN0ZWQgY2xpZW50c1xuICAgIGNvbnN0IHdzU2VuZE1lc3NhZ2VMYW1iZGEgPSBuZXcgbGFtYmRhLkZ1bmN0aW9uKHRoaXMsICdXc1NlbmRNZXNzYWdlTGFtYmRhJywge1xuICAgICAgZnVuY3Rpb25OYW1lOiBgJHt0aGlzLnN0YWNrTmFtZX0td3Mtc2VuZC1tZXNzYWdlYCxcbiAgICAgIHJ1bnRpbWU6IGxhbWJkYS5SdW50aW1lLlBST1ZJREVEX0FMMixcbiAgICAgIGNvZGU6IGxhbWJkYS5Db2RlLmZyb21Bc3NldChwYXRoLmpvaW4oX19kaXJuYW1lLCAnLi4vLi4vYmFja2VuZC9idWlsZC93cy1zZW5kLW1lc3NhZ2UnKSksXG4gICAgICBoYW5kbGVyOiAnYm9vdHN0cmFwJyxcbiAgICAgIG1lbW9yeVNpemU6IDEyOCxcbiAgICAgIHRpbWVvdXQ6IER1cmF0aW9uLnNlY29uZHMoMzApLFxuICAgICAgZW52aXJvbm1lbnQ6IHtcbiAgICAgICAgVEFCTEVfTkFNRTogbWVtb3J5VGFibGUudGFibGVOYW1lLFxuICAgICAgICBXRUJTT0NLRVRfQVBJX0VORFBPSU5UOiBgaHR0cHM6Ly8ke3dlYlNvY2tldEFwaS5hcGlJZH0uZXhlY3V0ZS1hcGkuJHt0aGlzLnJlZ2lvbn0uYW1hem9uYXdzLmNvbS8ke3dlYlNvY2tldFN0YWdlLnN0YWdlTmFtZX1gLFxuICAgICAgfSxcbiAgICB9KTtcblxuICAgIC8vIEdyYW50IFdlYlNvY2tldCBTZW5kIE1lc3NhZ2UgTGFtYmRhIHBlcm1pc3Npb25zXG4gICAgY29ubmVjdGlvbnNUYWJsZS5ncmFudFJlYWREYXRhKHdzU2VuZE1lc3NhZ2VMYW1iZGEpO1xuICAgIFxuICAgIC8vIEdyYW50IHBlcm1pc3Npb24gdG8gcG9zdCBtZXNzYWdlcyB0byBXZWJTb2NrZXQgY29ubmVjdGlvbnNcbiAgICB3c1NlbmRNZXNzYWdlTGFtYmRhLmFkZFRvUm9sZVBvbGljeShuZXcgaWFtLlBvbGljeVN0YXRlbWVudCh7XG4gICAgICBhY3Rpb25zOiBbJ2V4ZWN1dGUtYXBpOk1hbmFnZUNvbm5lY3Rpb25zJ10sXG4gICAgICByZXNvdXJjZXM6IFtcbiAgICAgICAgYGFybjphd3M6ZXhlY3V0ZS1hcGk6JHt0aGlzLnJlZ2lvbn06JHt0aGlzLmFjY291bnR9OiR7d2ViU29ja2V0QXBpLmFwaUlkfS8qLypgXG4gICAgICBdLFxuICAgIH0pKTtcblxuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuICAgIC8vIEVWRU5UQlJJREdFIFJVTEVTIC0gRXZlbnQgUm91dGluZ1xuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuXG4gICAgLy8gUnVsZSB0byB0cmlnZ2VyIENvbm5lY3QgTm9kZSBMYW1iZGEgd2hlbiBOb2RlQ3JlYXRlZCBldmVudCBvY2N1cnNcbiAgICBuZXcgZXZlbnRzLlJ1bGUodGhpcywgJ05vZGVDcmVhdGVkUnVsZScsIHtcbiAgICAgIGV2ZW50QnVzLFxuICAgICAgZXZlbnRQYXR0ZXJuOiB7XG4gICAgICAgIHNvdXJjZTogWydicmFpbjIubm9kZXMnXSxcbiAgICAgICAgZGV0YWlsVHlwZTogWydOb2RlQ3JlYXRlZCddLFxuICAgICAgfSxcbiAgICAgIHRhcmdldHM6IFtuZXcgZXZlbnRzVGFyZ2V0cy5MYW1iZGFGdW5jdGlvbihjb25uZWN0Tm9kZUxhbWJkYSldLFxuICAgIH0pO1xuXG4gICAgLy8gUnVsZSB0byB0cmlnZ2VyIFdlYlNvY2tldCBTZW5kIE1lc3NhZ2UgTGFtYmRhIHdoZW4gRWRnZXNDcmVhdGVkIGV2ZW50IG9jY3Vyc1xuICAgIG5ldyBldmVudHMuUnVsZSh0aGlzLCAnRWRnZXNDcmVhdGVkUnVsZScsIHtcbiAgICAgIGV2ZW50QnVzLFxuICAgICAgZXZlbnRQYXR0ZXJuOiB7XG4gICAgICAgIHNvdXJjZTogWydicmFpbjIuZWRnZXMnXSxcbiAgICAgICAgZGV0YWlsVHlwZTogWydFZGdlc0NyZWF0ZWQnXSxcbiAgICAgIH0sXG4gICAgICB0YXJnZXRzOiBbbmV3IGV2ZW50c1RhcmdldHMuTGFtYmRhRnVuY3Rpb24od3NTZW5kTWVzc2FnZUxhbWJkYSldLFxuICAgIH0pO1xuXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG4gICAgLy8gQVBJIEdBVEVXQVkgTEFZRVIgLSBIVFRQIEFQSSBmb3IgY2xpZW50LXNlcnZlciBjb21tdW5pY2F0aW9uXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG4gICAgXG4gICAgLy8gSFRUUCBBUEkgR2F0ZXdheSAtIE1vZGVybiwgZmFzdGVyLCBjaGVhcGVyIGFsdGVybmF0aXZlIHRvIFJFU1QgQVBJIEdhdGV3YXlcbiAgICAvLyBBY3RzIGFzIHRoZSBmcm9udCBkb29yIGZvciB5b3VyIGJhY2tlbmQgTGFtYmRhIGZ1bmN0aW9uc1xuICAgIGNvbnN0IGh0dHBBcGkgPSBuZXcgYXBpZ2F0ZXdheXYyLkh0dHBBcGkodGhpcywgJ2IyQXBpJywge1xuICAgICAgYXBpTmFtZTogJ2IyLWFwaScsXG4gICAgICAvLyBDT1JTIChDcm9zcy1PcmlnaW4gUmVzb3VyY2UgU2hhcmluZykgLSBhbGxvd3MgZnJvbnRlbmQgdG8gY2FsbCBBUEkgZnJvbSBkaWZmZXJlbnQgZG9tYWluXG4gICAgICBjb3JzUHJlZmxpZ2h0OiB7XG4gICAgICAgIC8vIEhlYWRlcnMgdGhhdCBicm93c2VycyBhcmUgYWxsb3dlZCB0byBzZW5kIHdpdGggcmVxdWVzdHNcbiAgICAgICAgYWxsb3dIZWFkZXJzOiBbJ0NvbnRlbnQtVHlwZScsICdBdXRob3JpemF0aW9uJ10sXG4gICAgICAgIGFsbG93TWV0aG9kczogW1xuICAgICAgICAgIGFwaWdhdGV3YXl2Mi5Db3JzSHR0cE1ldGhvZC5HRVQsICAgIC8vIFJlYWQgZGF0YVxuICAgICAgICAgIGFwaWdhdGV3YXl2Mi5Db3JzSHR0cE1ldGhvZC5QT1NULCAgIC8vIENyZWF0ZSBkYXRhXG4gICAgICAgICAgYXBpZ2F0ZXdheXYyLkNvcnNIdHRwTWV0aG9kLlBVVCwgICAgLy8gVXBkYXRlIGRhdGFcbiAgICAgICAgICBhcGlnYXRld2F5djIuQ29yc0h0dHBNZXRob2QuREVMRVRFLCAvLyBEZWxldGUgZGF0YVxuICAgICAgICAgIC8vIE9QVElPTlMgaXMgYSBwcmVmbGlnaHQgcmVxdWVzdCBicm93c2VycyBzZW5kIGF1dG9tYXRpY2FsbHlcbiAgICAgICAgICAvLyB0byBjaGVjayB3aGF0IG1ldGhvZHMvaGVhZGVycyBhcmUgYWxsb3dlZCBiZWZvcmUgc2VuZGluZyBhY3R1YWwgcmVxdWVzdFxuICAgICAgICAgIC8vIHRoZXJlZm9yZSB0aGlzIGlzIGFsc28gbm90IG5lZWRlZCBpbiB0aGUgYWRkcm91dGVzIHBhcnQgIFxuICAgICAgICAgIGFwaWdhdGV3YXl2Mi5Db3JzSHR0cE1ldGhvZC5PUFRJT05TLFxuICAgICAgICBdLFxuICAgICAgICAvLyAnKicgYWxsb3dzIHJlcXVlc3RzIGZyb20gYW55IGRvbWFpbiAtIHNob3VsZCBiZSByZXN0cmljdGVkIGluIHByb2R1Y3Rpb25cbiAgICAgICAgLy8gQ29uZmlndXJlIHdpdGggeW91ciBDbG91ZEZyb250IGRvbWFpbiBpbiBwcm9kdWN0aW9uIGZvciBzZWN1cml0eVxuICAgICAgICBhbGxvd09yaWdpbnM6IFsnKiddLFxuICAgICAgICAvLyBIb3cgbG9uZyBicm93c2VycyBjYW4gY2FjaGUgQ09SUyBwcmVmbGlnaHQgcmVzcG9uc2VzIChyZWR1Y2VzIHJlcXVlc3RzKVxuICAgICAgICBtYXhBZ2U6IER1cmF0aW9uLmRheXMoMSksXG4gICAgICB9LFxuICAgIH0pO1xuXG4gICAgLy8gTGFtYmRhIEludGVncmF0aW9uIC0gVGhlIGJyaWRnZSBiZXR3ZWVuIEFQSSBHYXRld2F5IGFuZCBMYW1iZGFcbiAgICAvLyBUaGluayBvZiBpdCBsaWtlIGEgdHJhbnNsYXRvciB0aGF0IGNvbnZlcnRzIEhUVFAgcmVxdWVzdHMgdG8gTGFtYmRhIGV2ZW50c1xuICAgIC8vIGFuZCBMYW1iZGEgcmVzcG9uc2VzIGJhY2sgdG8gSFRUUCByZXNwb25zZXNcbiAgICAvLyA9PT09PVxuICAgIC8vIGNhbiBiZSB0aG91Z2h0IG9mIGxpa2UgdGhlIHBsdWcgYmV0d2VlbiBhIGRldmljZSBhbmQgdGhlIHdhbGwgc29ja2V0XG4gICAgLy8gZm9yIHRoZSBsYW1iZGEgYW5kIGFwaWd3IHRvIGNvbm5lY3QgeW91IG5lZWQgc29tZSB3YXkgdG8gc3RhbmRhcmRpemUgZGF0YSBmb3JtYXRcbiAgICAvLyBBbHNvIGhhbmRsZXMgcGVybWlzc2lvbnMgZm9yIEFQSSBHYXRld2F5IHRvIGludm9rZSB0aGUgTGFtYmRhXG4gICAgY29uc3QgbGFtYmRhSW50ZWdyYXRpb24gPSBuZXcgYXBpZ2F0ZXdheXYySW50ZWdyYXRpb25zLkh0dHBMYW1iZGFJbnRlZ3JhdGlvbihcbiAgICAgICdCYWNrZW5kSW50ZWdyYXRpb24nLFxuICAgICAgYmFja2VuZExhbWJkYVxuICAgICk7XG5cbiAgICAvLyBMYW1iZGEgQXV0aG9yaXplciBDb25maWd1cmF0aW9uIC0gU2VjdXJpdHkgbGF5ZXIgZm9yIEFQSSBHYXRld2F5XG4gICAgLy8gVGhpcyBydW5zIEJFRk9SRSB5b3VyIG1haW4gTGFtYmRhIHRvIHZhbGlkYXRlIGF1dGhlbnRpY2F0aW9uXG4gICAgY29uc3QgYXV0aG9yaXplciA9IG5ldyBhcGlnYXRld2F5djJBdXRob3JpemVycy5IdHRwTGFtYmRhQXV0aG9yaXplcihcbiAgICAgICdTdXBhYmFzZUxhbWJkYUF1dGhvcml6ZXInLFxuICAgICAgYXV0aG9yaXplckxhbWJkYSxcbiAgICAgIHtcbiAgICAgICAgYXV0aG9yaXplck5hbWU6ICdTdXBhYmFzZUpXVEF1dGhvcml6ZXInLFxuICAgICAgICAvLyBXaGVyZSB0byBmaW5kIHRoZSBhdXRoIHRva2VuIC0gbG9va3MgaW4gQXV0aG9yaXphdGlvbiBoZWFkZXJcbiAgICAgICAgaWRlbnRpdHlTb3VyY2U6IFsnJHJlcXVlc3QuaGVhZGVyLkF1dGhvcml6YXRpb24nXSxcbiAgICAgICAgLy8gU0lNUExFIHJlc3BvbnNlID0ganVzdCBBbGxvdy9EZW55ICh2cyBJQU0gcG9saWNpZXMpXG4gICAgICAgIC8vIFRoaXMgaXMgZGVmaW5pbmcgYSBjb250cmFjdCBoZXJlLCBhbmQgdGh1cyBpbiB0aGUgYWN0dWFsIGNvZGUgZm9yIHRoZVxuICAgICAgICAvLyBsYW1iZGEsIGl0IG5lZWRzIHRvIGJlIGZvcm1hdHRlZCBpbnRvIHRoZSBTSU1QTEUgdHlwZS5cbiAgICAgICAgcmVzcG9uc2VUeXBlczogW2FwaWdhdGV3YXl2MkF1dGhvcml6ZXJzLkh0dHBMYW1iZGFSZXNwb25zZVR5cGUuU0lNUExFXSxcbiAgICAgICAgLy8gQ2FjaGUgc3VjY2Vzc2Z1bCBhdXRoIHJlc3VsdHMgdG8gYXZvaWQgcmUtdmFsaWRhdGluZyBzYW1lIHRva2VuXG4gICAgICAgIC8vIFRyYWRlLW9mZjogcGVyZm9ybWFuY2UgdnMgc2VjdXJpdHkgKHNob3J0ZXIgY2FjaGUgPSBtb3JlIHNlY3VyZSlcbiAgICAgICAgcmVzdWx0c0NhY2hlVHRsOiBEdXJhdGlvbi5taW51dGVzKDUpLFxuICAgICAgfVxuICAgICk7XG5cbiAgICAvLyBBUEkgUm91dGVzIENvbmZpZ3VyYXRpb24gLSBEZWZpbmUgd2hpY2ggVVJMcyBtYXAgdG8gd2hpY2ggTGFtYmRhXG4gICAgaHR0cEFwaS5hZGRSb3V0ZXMoe1xuICAgICAgLy8ge3Byb3h5K30gPSBjYXRjaC1hbGwgcm91dGUgcGF0dGVybiwgZm9yd2FyZHMgZXZlcnl0aGluZyBhZnRlciAvYXBpLyBuYW1lc3BhY2UgdG8gTGFtYmRhXG4gICAgICAvLyBFeGFtcGxlOiAvYXBpL21lbW9yaWVzLzEyMyBiZWNvbWVzIHByb3h5ID0gXCJtZW1vcmllcy8xMjNcIiBpbiBMYW1iZGFcbiAgICAgIHBhdGg6ICcvYXBpL3twcm94eSt9JyxcbiAgICAgIC8vIEhUVFAgbWV0aG9kcyB0aGlzIHJvdXRlIGFjY2VwdHMgLSBzdGFuZGFyZCBSRVNUIEFQSSBvcGVyYXRpb25zXG4gICAgICBtZXRob2RzOiBbXG4gICAgICAgIGFwaWdhdGV3YXl2Mi5IdHRwTWV0aG9kLkdFVCwgICAgLy8gUmVhZCBvcGVyYXRpb25zXG4gICAgICAgIGFwaWdhdGV3YXl2Mi5IdHRwTWV0aG9kLlBPU1QsICAgLy8gQ3JlYXRlIG9wZXJhdGlvbnMgIFxuICAgICAgICBhcGlnYXRld2F5djIuSHR0cE1ldGhvZC5QVVQsICAgIC8vIFVwZGF0ZSBvcGVyYXRpb25zXG4gICAgICAgIGFwaWdhdGV3YXl2Mi5IdHRwTWV0aG9kLkRFTEVURSwgLy8gRGVsZXRlIG9wZXJhdGlvbnNcbiAgICAgIF0sXG4gICAgICAvLyBXaGljaCBMYW1iZGEgdG8gaW52b2tlIHdoZW4gdGhpcyByb3V0ZSBpcyBoaXRcbiAgICAgIGludGVncmF0aW9uOiBsYW1iZGFJbnRlZ3JhdGlvbixcbiAgICAgIC8vIFNlY3VyaXR5OiByZXF1aXJlIHZhbGlkIEpXVCB0b2tlbiBmb3IgYWxsIHRoZXNlIHJvdXRlc1xuICAgICAgYXV0aG9yaXplcjogYXV0aG9yaXplcixcbiAgICB9KTtcblxuICAgIC8qXG4gICAgXG4gICAgSWYgeW91IHdlcmUgY3JlYXRpbmcgbW9yZSB0aGFuIG9uZSBiYWNrZW5kIGxhbWJkYSBvciBzdWNoLCB0aGVuIHlvdSB3b3VsZCBcbiAgICBpbnN0YW50aWF0ZSBtb3JlIGxhbWJkYXMsIGNvbmZpZ3VyZSB0aGVtLCBhbmQgdGhlbiBhZGQgYW5vdGhlciBhZGRSb3V0ZXMgdGhpbmdcbiAgICBmb3IgZWFjaCBvZiB0aGUgZGlmZmVyZW50IHJvdXRlcyB0aGF0IGVhY2ggbGFtYmRhIHdvdWxkIHRha2UgZm9yIGV4YW1wbGU6XG5cbiAgICAvLyBSb3V0ZSBmb3IgYWxsIG1lbW9yeS1yZWxhdGVkIGFjdGlvbnNcbiAgICBodHRwQXBpLmFkZFJvdXRlcyh7XG4gICAgICBwYXRoOiAnL2FwaS9ub2Rlcy97cHJveHkrfScsIC8vIENhdGNoZXMgL2FwaS9ub2RlcywgL2FwaS9ub2Rlcy8xMjMsIGV0Yy5cbiAgICAgIG1ldGhvZHM6IFsgIC4uR0VULCAuLlBPU1QsIC4uUFVULCAuLkRFTEVURSBdLFxuICAgICAgaW50ZWdyYXRpb246IGxhbWJkYUludGVncmF0aW9uLCAvLyA8LS0gUG9pbnRzIHRvIHRoZSBPUklHSU5BTCBiYWNrZW5kTGFtYmRhXG4gICAgICBhdXRob3JpemVyOiBhdXRob3JpemVyLFxuICAgIH0pO1xuXG4gICAgLy8gTkVXIFJvdXRlIGZvciBhbGwgdXNlciBwcm9maWxlIGFjdGlvbnNcbiAgICBodHRwQXBpLmFkZFJvdXRlcyh7XG4gICAgICBwYXRoOiAnL2FwaS9wcm9maWxlJywgLy8gQ2F0Y2hlcyAvYXBpL3Byb2ZpbGVcbiAgICAgIG1ldGhvZHM6IFsgYXBpZ2F0ZXdheXYyLkh0dHBNZXRob2QuR0VULCBhcGlnYXRld2F5djIuSHR0cE1ldGhvZC5QVVQgXSxcbiAgICAgIGludGVncmF0aW9uOiB1c2VyUHJvZmlsZUludGVncmF0aW9uLCAvLyA8LS0gUG9pbnRzIHRvIHRoZSBORVcgdXNlclByb2ZpbGVMYW1iZGFcbiAgICAgIGF1dGhvcml6ZXI6IGF1dGhvcml6ZXIsIC8vIFlvdSBjYW4gcmV1c2UgdGhlIHNhbWUgYXV0aG9yaXplclxuICAgIH0pO1xuXG4gICAgYnV0IHRoZSBjdXJyZW50IGltcGxlbWVudGF0aW9uIHRha2VzIGluIEFMTCB0aGUgdGhpbmdzIHRoYXQgYXJlIHVuZGVyIHRoZSBcbiAgICBcImFwaVwiIG5hbWVzcGFjZVxuXG4gICAgKi9cblxuICAgIC8vID09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PVxuICAgIC8vIEZST05URU5EIEhPU1RJTkcgTEFZRVIgLSBTMyArIENsb3VkRnJvbnQgZm9yIHN0YXRpYyB3ZWJzaXRlIGRlbGl2ZXJ5XG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG5cbiAgICAvLyBTMyBCdWNrZXQgZm9yIEZyb250ZW5kIFN0YXRpYyBGaWxlcyAoSFRNTCwgQ1NTLCBKUylcbiAgICAvLyBTMyA9IFNpbXBsZSBTdG9yYWdlIFNlcnZpY2UsIGxpa2UgYSBmaWxlIHN5c3RlbSBpbiB0aGUgY2xvdWRcbiAgICBjb25zdCBmcm9udGVuZEJ1Y2tldCA9IG5ldyBzMy5CdWNrZXQodGhpcywgJ0Zyb250ZW5kQnVja2V0Jywge1xuICAgICAgLy8gVW5pcXVlIGJ1Y2tldCBuYW1lIGFjcm9zcyBBTEwgb2YgQVdTIGdsb2JhbGx5XG4gICAgICBidWNrZXROYW1lOiBgYjItZnJvbnRlbmQtJHt0aGlzLmFjY291bnR9LSR7dGhpcy5yZWdpb259YCxcbiAgICAgIC8vIERpc2FibGUgZGlyZWN0IHB1YmxpYyBhY2Nlc3MgLSBDbG91ZEZyb250IHdpbGwgYWNjZXNzIGl0IGluc3RlYWRcbiAgICAgIC8vIFRoaXMgaXMgbW9yZSBzZWN1cmUgYW5kIGFsbG93cyBiZXR0ZXIgY2FjaGluZy9wZXJmb3JtYW5jZVxuICAgICAgcHVibGljUmVhZEFjY2VzczogZmFsc2UsXG4gICAgICAvLyBCbG9jayBhbGwgcHVibGljIGFjY2VzcyBzZXR0aW5ncyBmb3Igc2VjdXJpdHlcbiAgICAgIGJsb2NrUHVibGljQWNjZXNzOiBzMy5CbG9ja1B1YmxpY0FjY2Vzcy5CTE9DS19BTEwsXG4gICAgICAvLyBEZWxldGUgYnVja2V0IHdoZW4gc3RhY2sgaXMgZGVsZXRlZCAoZ29vZCBmb3IgZGV2L3Rlc3QpXG4gICAgICByZW1vdmFsUG9saWN5OiBSZW1vdmFsUG9saWN5LkRFU1RST1ksXG4gICAgICAvLyBBdXRvbWF0aWNhbGx5IGRlbGV0ZSBhbGwgb2JqZWN0cyB3aGVuIGJ1Y2tldCBpcyBkZWxldGVkXG4gICAgICBhdXRvRGVsZXRlT2JqZWN0czogdHJ1ZSxcbiAgICB9KTtcblxuICAgIC8vIENsb3VkRnJvbnQgRGlzdHJpYnV0aW9uIC0gR2xvYmFsIENETiAoQ29udGVudCBEZWxpdmVyeSBOZXR3b3JrKVxuICAgIC8vIENETiA9IG5ldHdvcmsgb2Ygc2VydmVycyB3b3JsZHdpZGUgdGhhdCBjYWNoZSB5b3VyIGNvbnRlbnQgY2xvc2VyIHRvIHVzZXJzXG4gICAgY29uc3QgZGlzdHJpYnV0aW9uID0gbmV3IGNsb3VkZnJvbnQuRGlzdHJpYnV0aW9uKHRoaXMsICdGcm9udGVuZERpc3RyaWJ1dGlvbicsIHtcbiAgICAgIGRlZmF1bHRCZWhhdmlvcjoge1xuICAgICAgICAvLyBTM09yaWdpbiBhdXRvbWF0aWNhbGx5IHNldHMgdXAgc2VjdXJlIGFjY2VzcyBiZXR3ZWVuIENsb3VkRnJvbnQgYW5kIFMzXG4gICAgICAgIC8vIENyZWF0ZXMgT3JpZ2luIEFjY2VzcyBDb250cm9sIChPQUMpIGFuZCBTMyBidWNrZXQgcG9saWN5XG4gICAgICAgIC8vIFRoaXMgbWVhbnMgb25seSBDbG91ZEZyb250IGNhbiBhY2Nlc3MgUzMsIG5vdCBkaXJlY3QgcHVibGljIGFjY2Vzc1xuICAgICAgICBvcmlnaW46IG5ldyBjbG91ZGZyb250T3JpZ2lucy5TM09yaWdpbihmcm9udGVuZEJ1Y2tldCksXG4gICAgICAgIFxuICAgICAgICAvLyBWaWV3ZXJQcm90b2NvbFBvbGljeSBjb250cm9scyBIVFRQIHZzIEhUVFBTIGZvciBlbmQgdXNlcnNcbiAgICAgICAgLy8gUkVESVJFQ1RfVE9fSFRUUFMgPSBhY2NlcHQgSFRUUCByZXF1ZXN0cyBidXQgcmVkaXJlY3QgdG8gSFRUUFNcbiAgICAgICAgLy8gQWx0ZXJuYXRpdmUgb3B0aW9uczpcbiAgICAgICAgLy8gLSBBTExPV19BTEw6IGFsbG93IGJvdGggSFRUUCBhbmQgSFRUUFNcbiAgICAgICAgLy8gLSBIVFRQU19PTkxZOiByZWplY3QgSFRUUCByZXF1ZXN0cyBlbnRpcmVseSAgXG4gICAgICAgIC8vIEhUVFBTIGlzIGVzc2VudGlhbCBmb3Igc2VjdXJpdHkgKGVuY3J5cHRzIGRhdGEsIHByZXZlbnRzIHRhbXBlcmluZylcbiAgICAgICAgdmlld2VyUHJvdG9jb2xQb2xpY3k6IGNsb3VkZnJvbnQuVmlld2VyUHJvdG9jb2xQb2xpY3kuUkVESVJFQ1RfVE9fSFRUUFMsXG4gICAgICAgIFxuICAgICAgICAvLyBDQUNISU5HX09QVElNSVpFRCA9IEFXUyBtYW5hZ2VkIGNhY2hlIHBvbGljeSBmb3Igc3RhdGljIGNvbnRlbnRcbiAgICAgICAgLy8gQ2FjaGVzIGJhc2VkIG9uIHF1ZXJ5IHN0cmluZ3MsIGhlYWRlcnMgdGhhdCBhZmZlY3QgY29udGVudFxuICAgICAgICAvLyBMb25nIGNhY2hlIHRpbWVzIGZvciBzdGF0aWMgYXNzZXRzLCBzaG9ydGVyIGZvciBkeW5hbWljIGNvbnRlbnRcbiAgICAgICAgY2FjaGVQb2xpY3k6IGNsb3VkZnJvbnQuQ2FjaGVQb2xpY3kuQ0FDSElOR19PUFRJTUlaRUQsXG4gICAgICB9LFxuICAgICAgLy8gRGVmYXVsdCBmaWxlIHRvIHNlcnZlIHdoZW4gdXNlcnMgdmlzaXQgcm9vdCBkb21haW4gKC8pXG4gICAgICBkZWZhdWx0Um9vdE9iamVjdDogJ2luZGV4Lmh0bWwnLFxuICAgICAgLy8gRXJyb3IgaGFuZGxpbmcgZm9yIFNpbmdsZSBQYWdlIEFwcGxpY2F0aW9ucyAoU1BBKVxuICAgICAgZXJyb3JSZXNwb25zZXM6IFtcbiAgICAgICAge1xuICAgICAgICAgIC8vIFdoZW4gUzMgcmV0dXJucyA0MDQgKGZpbGUgbm90IGZvdW5kKS4uLlxuICAgICAgICAgIGh0dHBTdGF0dXM6IDQwNCxcbiAgICAgICAgICAvLyBSZXR1cm4gMjAwIE9LIGluc3RlYWQgKHNvIGJyb3dzZXIgZG9lc24ndCBzaG93IGVycm9yKVxuICAgICAgICAgIHJlc3BvbnNlSHR0cFN0YXR1czogMjAwLFxuICAgICAgICAgIC8vIFNlcnZlIGluZGV4Lmh0bWwgKGxldCBSZWFjdCBSb3V0ZXIgaGFuZGxlIHRoZSByb3V0ZSlcbiAgICAgICAgICByZXNwb25zZVBhZ2VQYXRoOiAnL2luZGV4Lmh0bWwnLFxuICAgICAgICAgIC8vIENhY2hlIHRoaXMgZXJyb3IgcmVzcG9uc2UgZm9yIDUgbWludXRlc1xuICAgICAgICAgIHR0bDogRHVyYXRpb24ubWludXRlcyg1KSxcbiAgICAgICAgfSxcbiAgICAgIF0sXG4gICAgfSk7XG5cbiAgICAvLyBBdXRvbWF0ZWQgRnJvbnRlbmQgRGVwbG95bWVudCAtIFVwbG9hZHMgYnVpbHQgZmlsZXMgdG8gUzNcbiAgICBuZXcgczNkZXBsb3kuQnVja2V0RGVwbG95bWVudCh0aGlzLCAnRGVwbG95RnJvbnRlbmQnLCB7XG4gICAgICAvLyBTb3VyY2U6IGxvY2FsIGJ1aWxkIG91dHB1dCBkaXJlY3RvcnkgKHdlYnBhY2svdml0ZSBidWlsZCBjcmVhdGVzIHRoaXMpXG4gICAgICBzb3VyY2VzOiBbczNkZXBsb3kuU291cmNlLmFzc2V0KHBhdGguam9pbihfX2Rpcm5hbWUsICcuLi8uLi9mcm9udGVuZC9kaXN0JykpXSxcbiAgICAgIC8vIERlc3RpbmF0aW9uOiB0aGUgUzMgYnVja2V0IHdlIGNyZWF0ZWQgYWJvdmVcbiAgICAgIGRlc3RpbmF0aW9uQnVja2V0OiBmcm9udGVuZEJ1Y2tldCxcbiAgICAgIC8vIEludmFsaWRhdGUgQ2xvdWRGcm9udCBjYWNoZSBhZnRlciBkZXBsb3ltZW50IChzbyB1c2VycyBnZXQgbmV3IHZlcnNpb24pXG4gICAgICBkaXN0cmlidXRpb24sXG4gICAgICAvLyBJbnZhbGlkYXRlIGFsbCBwYXRocyAoJy8qJykgLSBjb3VsZCBiZSBtb3JlIHNwZWNpZmljIGZvciBsYXJnZSBzaXRlc1xuICAgICAgZGlzdHJpYnV0aW9uUGF0aHM6IFsnLyonXSxcbiAgICAgIC8vIENhY2hlIENvbnRyb2wgSGVhZGVycyAtIHRlbGwgYnJvd3NlcnMgYW5kIENsb3VkRnJvbnQgaG93IGxvbmcgdG8gY2FjaGUgZmlsZXNcbiAgICAgIGNhY2hlQ29udHJvbDogW1xuICAgICAgICAvLyAxIHllYXIgY2FjaGUgZm9yIGltbXV0YWJsZSBhc3NldHMgKEpTL0NTUyB3aXRoIGhhc2hlZCBmaWxlbmFtZXMpXG4gICAgICAgIHMzZGVwbG95LkNhY2hlQ29udHJvbC5mcm9tU3RyaW5nKCdtYXgtYWdlPTMxNTM2MDAwLHB1YmxpYyxpbW11dGFibGUnKSxcbiAgICAgICAgLy8gTWFyayBhcyBwdWJsaWNseSBjYWNoZWFibGUgKENETnMgYW5kIGJyb3dzZXJzIGNhbiBjYWNoZSlcbiAgICAgICAgczNkZXBsb3kuQ2FjaGVDb250cm9sLnNldFB1YmxpYygpLFxuICAgICAgICAvLyAxIGhvdXIgZGVmYXVsdCBjYWNoZSAoZm9yIGZpbGVzIHdpdGhvdXQgc3BlY2lmaWMgY2FjaGUgaGVhZGVycylcbiAgICAgICAgczNkZXBsb3kuQ2FjaGVDb250cm9sLm1heEFnZShEdXJhdGlvbi5ob3VycygxKSksXG4gICAgICBdLFxuICAgIH0pO1xuXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG4gICAgLy8gU1RBQ0sgT1VUUFVUUyAtIEV4cG9ydCBpbXBvcnRhbnQgdmFsdWVzIGZvciBvdGhlciBzdGFja3Mgb3IgZXh0ZXJuYWwgdXNlXG4gICAgLy8gPT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09PT09XG4gICAgXG4gICAgLy8gRXhwb3J0IEFQSSBHYXRld2F5IFVSTCAtIG90aGVyIHN0YWNrcyBvciBDSS9DRCBjYW4gcmVmZXJlbmNlIHRoaXNcbiAgICB0aGlzLmV4cG9ydFZhbHVlKGh0dHBBcGkudXJsISwgeyBuYW1lOiAnQXBpVXJsJyB9KTtcbiAgICAvLyBFeHBvcnQgV2ViU29ja2V0IEFQSSBVUkwgLSBmcm9udGVuZCB3aWxsIGNvbm5lY3QgdG8gdGhpcyBmb3IgcmVhbC10aW1lIHVwZGF0ZXNcbiAgICB0aGlzLmV4cG9ydFZhbHVlKHdlYlNvY2tldFN0YWdlLnVybCwgeyBuYW1lOiAnV2ViU29ja2V0VXJsJyB9KTtcbiAgICAvLyBFeHBvcnQgQ2xvdWRGcm9udCBkb21haW4gLSB0aGlzIGlzIHdoYXQgdXNlcnMgd2lsbCB2aXNpdCBpbiB0aGVpciBicm93c2VyXG4gICAgdGhpcy5leHBvcnRWYWx1ZShkaXN0cmlidXRpb24uZGlzdHJpYnV0aW9uRG9tYWluTmFtZSwgeyBuYW1lOiAnQ2xvdWRGcm9udFVybCcgfSk7XG4gIH1cbn0iXX0=