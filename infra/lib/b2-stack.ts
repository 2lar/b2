/**
 * Brain2 Infrastructure as Code - AWS CDK Stack
 */

import 'dotenv/config';
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
import * as apigwv2 from 'aws-cdk-lib/aws-apigatewayv2';
import { HttpLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { WebSocketLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { HttpLambdaAuthorizer, HttpLambdaResponseType } from 'aws-cdk-lib/aws-apigatewayv2-authorizers';


/**
 * Brain2 CDK Stack
 */
export class b2Stack extends Stack {
  constructor(scope: Construct, id: string, props?: StackProps) {
    super(scope, id, props);

    // Environment configuration validation
    const SUPABASE_URL = process.env.SUPABASE_URL;
    const SUPABASE_SERVICE_ROLE_KEY = process.env.SUPABASE_SERVICE_ROLE_KEY;
    
    if (!SUPABASE_URL || !SUPABASE_SERVICE_ROLE_KEY ) {
      throw new Error('FATAL: SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY must be defined in your environment.');
    }

    // EventBridge event bus
    const eventBus = new events.EventBus(this, 'B2EventBus', {
      eventBusName: 'B2EventBus',
    });

    // DynamoDB table for memory storage
    const memoryTable = new dynamodb.Table(this, 'MemoryTable', {
      tableName: 'brain2',
      partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      // DESTROY policy for development - use RETAIN for production!
      removalPolicy: RemovalPolicy.DESTROY,
    });
    
    // Global Secondary Index for keyword-based search
    memoryTable.addGlobalSecondaryIndex({
      indexName: 'KeywordIndex',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },
      projectionType: dynamodb.ProjectionType.ALL,
    });

    // DynamoDB table for tracking WebSocket connections
    const connectionsTable = new dynamodb.Table(this, 'ConnectionsTable', {
        tableName: 'B2-Connections',
        partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING }, // PK: USER#{userId}
        sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },      // SK: CONN#{connectionId}
        billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
        removalPolicy: RemovalPolicy.DESTROY,
        // TTL for automatic cleanup of stale connections
        timeToLiveAttribute: 'expireAt',
    });
    
    // Global Secondary Index for finding user by connectionId
    connectionsTable.addGlobalSecondaryIndex({
      indexName: 'connection-id-index',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING }, // GSI1PK: CONN#{connectionId}
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },      // GSI1SK: USER#{userId}
      projectionType: dynamodb.ProjectionType.ALL,
    });


    // Lambda function for JWT token validation
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

    // Go Lambda function for handling HTTP API requests
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
    
    // Grant IAM permissions
    memoryTable.grantReadWriteData(backendLambda);
    eventBus.grantPutEventsTo(backendLambda);

    // Lambda function for finding connections between memories
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
    
    // Grant permissions for graph computation
    memoryTable.grantReadWriteData(connectNodeLambda);
    eventBus.grantPutEventsTo(connectNodeLambda);

    // Lambda function for handling new WebSocket connections
    const wsConnectLambda = new lambda.Function(this, 'wsConnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-connect')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            SUPABASE_URL: SUPABASE_URL,
            SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
        },
    });
    
    connectionsTable.grantWriteData(wsConnectLambda);

    // Lambda function for handling WebSocket disconnections
    const wsDisconnectLambda = new lambda.Function(this, 'wsDisconnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-disconnect')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            CONNECTIONS_GSI_NAME: 'connection-id-index',
        },
    });
    
    connectionsTable.grantReadWriteData(wsDisconnectLambda);

    // Lambda function for broadcasting WebSocket messages
    const wsSendMessageLambda = new lambda.Function(this, 'wsSendMessageLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build/ws-send-message')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
        },
    });
    
    connectionsTable.grantReadData(wsSendMessageLambda);

    // HTTP API with CORS support
    const httpApi = new apigwv2.HttpApi(this, 'b2HttpApi', {
      apiName: 'b2-http-api',
      corsPreflight: {
        allowHeaders: ['Content-Type', 'Authorization'],
        allowMethods: [ 
          apigwv2.CorsHttpMethod.GET,
          apigwv2.CorsHttpMethod.POST,
          apigwv2.CorsHttpMethod.PUT,
          apigwv2.CorsHttpMethod.DELETE,
          apigwv2.CorsHttpMethod.OPTIONS
        ],
        allowOrigins: ['*'], // TODO: Restrict in production
        maxAge: Duration.days(1),
      },
    });

    // JWT validation Lambda authorizer for API Gateway
    const authorizer = new HttpLambdaAuthorizer('SupabaseLambdaAuthorizer', authorizerLambda, {
      responseTypes: [HttpLambdaResponseType.SIMPLE],
      identitySource: ['$request.header.Authorization'],
      resultsCacheTtl: Duration.minutes(5),
    });

    // Proxy pattern route configuration for API Gateway
    httpApi.addRoutes({
      path: '/api/{proxy+}',
      methods: [ 
        apigwv2.HttpMethod.GET,
        apigwv2.HttpMethod.POST,
        apigwv2.HttpMethod.PUT,
        apigwv2.HttpMethod.DELETE
      ],
      integration: new HttpLambdaIntegration('BackendIntegration', backendLambda),
      authorizer: authorizer,
    });

    // WebSocket API for real-time communication
    const webSocketApi = new apigwv2.WebSocketApi(this, 'B2WebSocketApi', {
        apiName: 'B2WebSocketApi',
        connectRouteOptions: { 
          integration: new WebSocketLambdaIntegration('ConnectIntegration', wsConnectLambda) 
        },
        disconnectRouteOptions: { 
          integration: new WebSocketLambdaIntegration('DisconnectIntegration', wsDisconnectLambda) 
        },
    });

    // WebSocket API deployment stage
    const webSocketStage = new apigwv2.WebSocketStage(this, 'B2WebSocketStage', {
        webSocketApi,
        stageName: 'prod',
        autoDeploy: true,
    });
    
    // WebSocket API management permissions
    webSocketApi.grantManageConnections(wsSendMessageLambda);
    
    // Inject WebSocket API endpoint URL into Lambda
    wsSendMessageLambda.addEnvironment('WEBSOCKET_API_ENDPOINT', webSocketStage.callbackUrl);


    // EventBridge rule for NodeCreated events
    new events.Rule(this, 'NodeCreatedRule', {
        eventBus,
        eventPattern: {
            source: ['brain2.api'],
            detailType: ['NodeCreated'],
        },
        targets: [new targets.LambdaFunction(connectNodeLambda)],
    });

    // EventBridge rule for EdgesCreated events
    new events.Rule(this, 'EdgesCreatedRule', {
        eventBus,
        eventPattern: {
            source: ['brain2.connectNode'],
            detailType: ['EdgesCreated'],
        },
        targets: [new targets.LambdaFunction(wsSendMessageLambda)],
    });


    // S3 bucket for frontend static assets
    const frontendBucket = new s3.Bucket(this, 'FrontendBucket', {
      bucketName: `b2-frontend-${this.account}-${this.region}`,
      publicReadAccess: false,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
    });

    // CloudFront distribution for serving frontend content
    const distribution = new cloudfront.Distribution(this, 'FrontendDistribution', {
      defaultBehavior: {
        origin: new origins.S3Origin(frontendBucket),
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
      },
      defaultRootObject: 'index.html',
      // Error handling for SPA client-side routing
      errorResponses: [{ 
        httpStatus: 404,
        responseHttpStatus: 200,
        responsePagePath: '/index.html',
        ttl: Duration.minutes(5)
      }],
    });

    // Automated frontend deployment to S3 and CloudFront
    new s3deploy.BucketDeployment(this, 'DeployFrontend', {
      sources: [s3deploy.Source.asset(path.join(__dirname, '../../frontend/dist'))],
      destinationBucket: frontendBucket,
      distribution,
      distributionPaths: ['/*'],
    });

    // Stack outputs
    new CfnOutput(this, 'HttpApiUrl', { 
      value: httpApi.url!, 
      description: 'The base URL for HTTP API calls (set as VITE_API_BASE_URL)' 
    });
    
    new CfnOutput(this, 'WebSocketApiUrl', { 
      value: webSocketStage.url, 
      description: 'The WebSocket URL for real-time updates (set as VITE_WEBSOCKET_URL)' 
    });
    
    new CfnOutput(this, 'CloudFrontUrl', { 
      value: `https://${distribution.distributionDomainName}`, 
      description: 'The public URL of your Brain2 application' 
    });
  }
}
