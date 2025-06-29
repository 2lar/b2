import 'dotenv/config'; // Loads variables from .env into process.env

import { Stack, StackProps, RemovalPolicy, Duration, CfnOutput } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as s3deploy from 'aws-cdk-lib/aws-s3-deployment';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';
import * as path from 'path';

// Using stable AWS CDK v2 imports for API Gateway
import * as apigwv2 from 'aws-cdk-lib/aws-apigatewayv2';
import { HttpLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { WebSocketLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { HttpLambdaAuthorizer, HttpLambdaResponseType } from 'aws-cdk-lib/aws-apigatewayv2-authorizers';


export class b2Stack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    // Validate required environment variables from .env file
    const SUPABASE_URL = process.env.SUPABASE_URL;
    const SUPABASE_SERVICE_ROLE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY;
    
    if (!SUPABASE_URL || !SUPABASE_SERVICE_ROLE_KEY ) {
      throw new Error('FATAL: SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY must be defined in your environment.');
    }

    // ========================================================================
    // EVENT BUS - Central hub for our event-driven architecture
    // ========================================================================
    const eventBus = new events.EventBus(this, 'B2EventBus', {
      eventBusName: 'B2EventBus',
    });

    // ========================================================================
    // DATABASE LAYER - DynamoDB Tables
    // ========================================================================
    
    // Original table for graph nodes, keywords, and edges
    const memoryTable = new dynamodb.Table(this, 'MemoryTable', {
      tableName: 'brain2',
      partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: RemovalPolicy.DESTROY,
    });
    memoryTable.addGlobalSecondaryIndex({
      indexName: 'KeywordIndex',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },
      projectionType: dynamodb.ProjectionType.ALL,
    });

    // NEW: DynamoDB Table for WebSocket Connections
    const connectionsTable = new dynamodb.Table(this, 'ConnectionsTable', {
        tableName: 'B2-Connections',
        partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING }, // PK: USER#{userId}
        sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },      // SK: CONN#{connectionId}
        billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
        removalPolicy: RemovalPolicy.DESTROY,
        // Automatically clean up stale connections if disconnect fails
        timeToLiveAttribute: 'expireAt',
    });
    // Add a GSI to look up a connection by its ID during the disconnect event
    connectionsTable.addGlobalSecondaryIndex({
      indexName: 'connection-id-index',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING }, // GSI1PK: CONN#{connectionId}
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },      // GSI1SK: USER#{userId}
      projectionType: dynamodb.ProjectionType.ALL,
    });


    // ========================================================================
    // BUSINESS LOGIC LAYER - Lambda Functions
    // ========================================================================

    // --- HTTP API Lambdas ---

    // This lambda validates JWT tokens from Supabase before allowing access
    const authorizerLambda = new lambda.Function(this, 'JWTAuthorizerLambda', {
      functionName: `${this.stackName}-jwt-authorizer`,
      runtime: lambda.Runtime.NODEJS_20_X,
      handler: 'index.handler',
      code: lambda.Code.fromAsset(path.join(__dirname, '../lambda/authorizer')),
      environment: {
        SUPABASE_URL: SUPABASE_URL,
        SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
      },
      timeout: Duration.seconds(10),
      memorySize: 128,
    });

    // Main Backend Lambda - Now only handles synchronous API requests
    const backendLambda = new lambda.Function(this, 'BackendLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/main')),
      handler: 'bootstrap',
      memorySize: 128,
      timeout: Duration.seconds(30),
      environment: {
        TABLE_NAME: memoryTable.tableName,
        KEYWORD_INDEX_NAME: 'KeywordIndex',
      },
    });
    memoryTable.grantReadWriteData(backendLambda);
    eventBus.grantPutEventsTo(backendLambda); // Grant permission to publish events

    // --- Event-Driven & WebSocket Lambdas (NEW) ---

    // ConnectNode Lambda - Triggered by EventBridge to create graph edges
    const connectNodeLambda = new lambda.Function(this, 'ConnectNodeLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/connect-node')),
      handler: 'bootstrap',
      memorySize: 128,
      timeout: Duration.seconds(30),
      environment: {
        TABLE_NAME: memoryTable.tableName,
        KEYWORD_INDEX_NAME: 'KeywordIndex',
      },
    });
    memoryTable.grantReadWriteData(connectNodeLambda);
    eventBus.grantPutEventsTo(connectNodeLambda);

    // WebSocket Connect Lambda - Handles new client connections
    const wsConnectLambda = new lambda.Function(this, 'wsConnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-connect')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        // CORRECTED: Added missing environment variables
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            SUPABASE_URL: SUPABASE_URL,
            SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
        },
    });
    connectionsTable.grantWriteData(wsConnectLambda);

    // WebSocket Disconnect Lambda - Handles client disconnections
    const wsDisconnectLambda = new lambda.Function(this, 'wsDisconnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-disconnect')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        // CORRECTED: Added missing environment variables
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            CONNECTIONS_GSI_NAME: 'connection-id-index',
        },
    });
    connectionsTable.grantReadWriteData(wsDisconnectLambda);

    // WebSocket Send Message Lambda - Pushes updates to clients
    const wsSendMessageLambda = new lambda.Function(this, 'wsSendMessageLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-send-message')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            // The endpoint URL will be added later after the WebSocket API is created
        },
    });
    connectionsTable.grantReadData(wsSendMessageLambda);

    // ========================================================================
    // API GATEWAY LAYER - HTTP and WebSocket APIs
    // ========================================================================
    
    // --- HTTP API ---
    const httpApi = new apigwv2.HttpApi(this, 'b2HttpApi', {
      apiName: 'b2-http-api',
      corsPreflight: {
        allowHeaders: ['Content-Type', 'Authorization'],
        allowMethods: [ apigwv2.CorsHttpMethod.GET, apigwv2.CorsHttpMethod.POST, apigwv2.CorsHttpMethod.PUT, apigwv2.CorsHttpMethod.DELETE, apigwv2.CorsHttpMethod.OPTIONS ],
        allowOrigins: ['*'],
        maxAge: Duration.days(1),
      },
    });

    const authorizer = new HttpLambdaAuthorizer('SupabaseLambdaAuthorizer', authorizerLambda, {
      responseTypes: [HttpLambdaResponseType.SIMPLE],
      identitySource: ['$request.header.Authorization'],
      resultsCacheTtl: Duration.minutes(5),
    });

    httpApi.addRoutes({
      path: '/api/{proxy+}',
      methods: [ apigwv2.HttpMethod.GET, apigwv2.HttpMethod.POST, apigwv2.HttpMethod.PUT, apigwv2.HttpMethod.DELETE ],
      integration: new HttpLambdaIntegration('BackendIntegration', backendLambda),
      authorizer: authorizer,
    });

    // --- WebSocket API (NEW) ---
    const webSocketApi = new apigwv2.WebSocketApi(this, 'B2WebSocketApi', {
        apiName: 'B2WebSocketApi',
        connectRouteOptions: { integration: new WebSocketLambdaIntegration('ConnectIntegration', wsConnectLambda) },
        disconnectRouteOptions: { integration: new WebSocketLambdaIntegration('DisconnectIntegration', wsDisconnectLambda) },
    });

    const webSocketStage = new apigwv2.WebSocketStage(this, 'B2WebSocketStage', {
        webSocketApi,
        stageName: 'prod',
        autoDeploy: true,
    });
    
    // Grant the SendMessageLambda permission to post to the WebSocket connections
    webSocketApi.grantManageConnections(wsSendMessageLambda);
    
    // Add the dynamically generated API endpoint to the SendMessageLambda's environment
    wsSendMessageLambda.addEnvironment('WEBSOCKET_API_ENDPOINT', webSocketStage.callbackUrl);


    // ========================================================================
    // EVENTBRIDGE RULES (NEW)
    // ========================================================================
    
    // Rule 1: When a node is created via the HTTP API, trigger the ConnectNodeLambda
    new events.Rule(this, 'NodeCreatedRule', {
        eventBus,
        eventPattern: {
            source: ['brain2.api'],
            detailType: ['NodeCreated'],
        },
        targets: [new targets.LambdaFunction(connectNodeLambda)],
    });

    // Rule 2: When edges are created by ConnectNodeLambda, trigger the WsSendMessageLambda
    new events.Rule(this, 'EdgesCreatedRule', {
        eventBus,
        eventPattern: {
            source: ['brain2.connectNode'],
            detailType: ['EdgesCreated'],
        },
        targets: [new targets.LambdaFunction(wsSendMessageLambda)],
    });


    // ========================================================================
    // FRONTEND HOSTING LAYER - S3 + CloudFront
    // ========================================================================
    const frontendBucket = new s3.Bucket(this, 'FrontendBucket', {
      bucketName: `b2-frontend-${this.account}-${this.region}`,
      publicReadAccess: false,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
    });

    const distribution = new cloudfront.Distribution(this, 'FrontendDistribution', {
      defaultBehavior: {
        origin: new origins.S3Origin(frontendBucket),
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
      },
      defaultRootObject: 'index.html',
      errorResponses: [ { httpStatus: 404, responseHttpStatus: 200, responsePagePath: '/index.html', ttl: Duration.minutes(5) } ],
    });

    new s3deploy.BucketDeployment(this, 'DeployFrontend', {
      sources: [s3deploy.Source.asset(path.join(__dirname, '../../frontend/dist'))],
      destinationBucket: frontendBucket,
      distribution,
      distributionPaths: ['/*'],
    });

    // ========================================================================
    // STACK OUTPUTS
    // ========================================================================
    new CfnOutput(this, 'HttpApiUrl', { value: httpApi.url!, description: 'The URL of the HTTP API' });
    new CfnOutput(this, 'WebSocketApiUrl', { value: webSocketStage.url, description: 'The URL of the WebSocket API' });
    new CfnOutput(this, 'CloudFrontUrl', { value: `https://${distribution.distributionDomainName}`, description: 'The URL of the frontend application' });
  }
}
