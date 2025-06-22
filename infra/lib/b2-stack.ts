import 'dotenv/config'; // Loads variables from .env into process.env

import { Stack, StackProps, RemovalPolicy, Duration } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as apigatewayv2 from 'aws-cdk-lib/aws-apigatewayv2';
import * as apigatewayv2Integrations from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import * as apigatewayv2Authorizers from 'aws-cdk-lib/aws-apigatewayv2-authorizers';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as cloudfrontOrigins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as s3deploy from 'aws-cdk-lib/aws-s3-deployment';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as path from 'path';

export class b2Stack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
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
      removalPolicy: RemovalPolicy.DESTROY,
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
      timeout: Duration.seconds(10),
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
      timeout: Duration.seconds(30),
      environment: {
        // Pass DynamoDB table info to the Lambda at runtime
        TABLE_NAME: memoryTable.tableName,
        KEYWORD_INDEX_NAME: 'KeywordIndex',
      },
    });

    // Grant Backend Lambda permissions to access DynamoDB
    // This CDK helper automatically creates IAM policies for DynamoDB operations
    // Includes: GetItem, PutItem, UpdateItem, DeleteItem, Query, Scan on table and indexes
    memoryTable.grantReadWriteData(backendLambda);

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
          apigatewayv2.CorsHttpMethod.GET,    // Read data
          apigatewayv2.CorsHttpMethod.POST,   // Create data
          apigatewayv2.CorsHttpMethod.PUT,    // Update data
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
        maxAge: Duration.days(1),
      },
    });

    // Lambda Integration - The bridge between API Gateway and Lambda
    // Think of it like a translator that converts HTTP requests to Lambda events
    // and Lambda responses back to HTTP responses
    // =====
    // can be thought of like the plug between a device and the wall socket
    // for the lambda and apigw to connect you need some way to standardize data format
    // Also handles permissions for API Gateway to invoke the Lambda
    const lambdaIntegration = new apigatewayv2Integrations.HttpLambdaIntegration(
      'BackendIntegration',
      backendLambda
    );

    // Lambda Authorizer Configuration - Security layer for API Gateway
    // This runs BEFORE your main Lambda to validate authentication
    const authorizer = new apigatewayv2Authorizers.HttpLambdaAuthorizer(
      'SupabaseLambdaAuthorizer',
      authorizerLambda,
      {
        authorizerName: 'SupabaseJWTAuthorizer',
        // Where to find the auth token - looks in Authorization header
        identitySource: ['$request.header.Authorization'],
        // SIMPLE response = just Allow/Deny (vs IAM policies)
        // This is defining a contract here, and thus in the actual code for the
        // lambda, it needs to be formatted into the SIMPLE type.
        responseTypes: [apigatewayv2Authorizers.HttpLambdaResponseType.SIMPLE],
        // Cache successful auth results to avoid re-validating same token
        // Trade-off: performance vs security (shorter cache = more secure)
        resultsCacheTtl: Duration.minutes(5),
      }
    );

    // API Routes Configuration - Define which URLs map to which Lambda
    httpApi.addRoutes({
      // {proxy+} = catch-all route pattern, forwards everything after /api/ namespace to Lambda
      // Example: /api/memories/123 becomes proxy = "memories/123" in Lambda
      path: '/api/{proxy+}',
      // HTTP methods this route accepts - standard REST API operations
      methods: [
        apigatewayv2.HttpMethod.GET,    // Read operations
        apigatewayv2.HttpMethod.POST,   // Create operations  
        apigatewayv2.HttpMethod.PUT,    // Update operations
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
      removalPolicy: RemovalPolicy.DESTROY,
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
          ttl: Duration.minutes(5),
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
        s3deploy.CacheControl.maxAge(Duration.hours(1)),
      ],
    });

    // ========================================================================
    // STACK OUTPUTS - Export important values for other stacks or external use
    // ========================================================================
    
    // Export API Gateway URL - other stacks or CI/CD can reference this
    this.exportValue(httpApi.url!, { name: 'ApiUrl' });
    // Export CloudFront domain - this is what users will visit in their browser
    this.exportValue(distribution.distributionDomainName, { name: 'CloudFrontUrl' });
  }
}