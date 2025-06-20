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
    
    // DynamoDB Table with Single-Table Design
    const memoryTable = new dynamodb.Table(this, 'MemoryTable', {
      tableName: 'brain2',
      partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: RemovalPolicy.DESTROY,
    });

    // Global Secondary Index for Keyword Search
    memoryTable.addGlobalSecondaryIndex({
      indexName: 'KeywordIndex',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },
      projectionType: dynamodb.ProjectionType.ALL,
    });

    // ========================================================================

    // Lambda Authorizer Function
    const authorizerLambda = new lambda.Function(this, 'JWTAuthorizerLambda', {
      functionName: `${this.stackName}-jwt-authorizer`,
      runtime: lambda.Runtime.NODEJS_20_X,
      handler: 'index.handler',
      code: lambda.Code.fromAsset(path.join(__dirname, '../lambda/authorizer')),
      environment: {
        SUPABASE_URL: SUPABASE_URL,
        SUPABASE_SERVICE_ROLE_KEY: SUPABASE_SERVICE_ROLE_KEY,
        NODE_ENV: 'production'
      },
      timeout: Duration.seconds(10),
      memorySize: 128,
      logRetention: 7, // Keep logs for 7 days
    });

    // Grant basic permissions to authorizer
    authorizerLambda.addToRolePolicy(new iam.PolicyStatement({
      actions: ['logs:CreateLogGroup', 'logs:CreateLogStream', 'logs:PutLogEvents'],
      resources: ['*'],
    }));

    // ========================================================================

    // Lambda Function for Backend API
    const backendLambda = new lambda.Function(this, 'BackendLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(path.join(__dirname, '../../backend/build')),
      handler: 'bootstrap',
      memorySize: 128,
      timeout: Duration.seconds(30),
      environment: {
        TABLE_NAME: memoryTable.tableName,
        KEYWORD_INDEX_NAME: 'KeywordIndex',
      },
    });

    // Grant Lambda permissions to access DynamoDB
    memoryTable.grantReadWriteData(backendLambda);

    // ========================================================================
    
    // HTTP API Gateway
    const httpApi = new apigatewayv2.HttpApi(this, 'b2Api', {
      apiName: 'b2-api',
      corsPreflight: {
        allowHeaders: ['Content-Type', 'Authorization'],
        allowMethods: [
          apigatewayv2.CorsHttpMethod.GET,
          apigatewayv2.CorsHttpMethod.POST,
          apigatewayv2.CorsHttpMethod.PUT,
          apigatewayv2.CorsHttpMethod.DELETE,
          apigatewayv2.CorsHttpMethod.OPTIONS,
        ],
        allowOrigins: ['*'], // Configure with your CloudFront domain in production
        maxAge: Duration.days(1),
      },
    });

    // Lambda integration
    const lambdaIntegration = new apigatewayv2Integrations.HttpLambdaIntegration(
      'BackendIntegration',
      backendLambda
    );

    // Create Lambda Authorizer (replacing JWT Authorizer)
    const authorizer = new apigatewayv2Authorizers.HttpLambdaAuthorizer(
      'SupabaseLambdaAuthorizer',
      authorizerLambda,
      {
        authorizerName: 'SupabaseJWTAuthorizer',
        identitySource: ['$request.header.Authorization'],
        responseTypes: [apigatewayv2Authorizers.HttpLambdaResponseType.SIMPLE],
        resultsCacheTtl: Duration.minutes(5), // Cache auth results for 5 minutes
      }
    );

    // API Routes with Lambda Authorization
    httpApi.addRoutes({
      path: '/api/{proxy+}',
      methods: [
        apigatewayv2.HttpMethod.GET,
        apigatewayv2.HttpMethod.POST,
        apigatewayv2.HttpMethod.PUT,
        apigatewayv2.HttpMethod.DELETE,
      ],
      integration: lambdaIntegration,
      authorizer: authorizer,
    });

    // S3 Bucket for Frontend Hosting
    const frontendBucket = new s3.Bucket(this, 'FrontendBucket', {
      bucketName: `b2-frontend-${this.account}-${this.region}`,
      publicReadAccess: false,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
    });

    // CloudFront Distribution - The new, more robust way
    const distribution = new cloudfront.Distribution(this, 'FrontendDistribution', {
      defaultBehavior: {
        // This S3Origin construct is smarter. It automatically creates the necessary
        // Origin Access Identity/Control AND the S3 bucket policy
        origin: new cloudfrontOrigins.S3Origin(frontendBucket),
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
      },
      defaultRootObject: 'index.html',
      errorResponses: [
        {
          httpStatus: 404,
          responseHttpStatus: 200,
          responsePagePath: '/index.html',
          ttl: Duration.minutes(5),
        },
      ],
    });

    new s3deploy.BucketDeployment(this, 'DeployFrontend', {
      sources: [s3deploy.Source.asset(path.join(__dirname, '../../frontend/dist'))],
      destinationBucket: frontendBucket,
      distribution,
      distributionPaths: ['/*'],
      cacheControl: [
        s3deploy.CacheControl.fromString('max-age=31536000,public,immutable'),
        s3deploy.CacheControl.setPublic(),
        s3deploy.CacheControl.maxAge(Duration.hours(1)),
      ],
    });

    // Outputs
    this.exportValue(httpApi.url!, { name: 'ApiUrl' });
    this.exportValue(distribution.distributionDomainName, { name: 'CloudFrontUrl' });
  }
}