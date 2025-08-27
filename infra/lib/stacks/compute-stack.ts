/**
 * Compute Stack - Lambda functions and EventBridge for Brain2
 */

import { Stack, StackProps, Duration, CfnOutput } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';
import * as path from 'path';
import { EnvironmentConfig } from '../config/environments';
import { Brain2WebSocketApi } from '../constructs/websocket-api';

export interface ComputeStackProps extends StackProps {
  config: EnvironmentConfig;
  memoryTable: dynamodb.Table;
  connectionsTable: dynamodb.Table;
}

export class ComputeStack extends Stack {
  public readonly backendLambda: lambda.Function;
  public readonly connectNodeLambda: lambda.Function;
  public readonly cleanupLambda: lambda.Function;
  public readonly wsConnectLambda: lambda.Function;
  public readonly wsDisconnectLambda: lambda.Function;
  public readonly wsSendMessageLambda: lambda.Function;
  public readonly authorizerLambda: lambda.Function;
  public readonly eventBus: events.EventBus;
  public readonly webSocketApi: Brain2WebSocketApi;

  constructor(scope: Construct, id: string, props: ComputeStackProps) {
    super(scope, id, props);

    const { config, memoryTable, connectionsTable } = props;

    // EventBridge event bus for decoupled communication - Match original b2-stack
    this.eventBus = new events.EventBus(this, 'B2EventBus', {
      eventBusName: 'B2EventBus',
    });

    // JWT Authorization Lambda (Node.js) - Match original b2-stack pattern
    this.authorizerLambda = new lambda.Function(this, 'JWTAuthorizerLambda', {
      functionName: `${config.stackName}-jwt-authorizer`,
      runtime: lambda.Runtime.NODEJS_20_X,
      handler: 'index.handler',
      code: lambda.Code.fromAsset(path.join(__dirname, '../../lambda/authorizer')),
      environment: {
        SUPABASE_URL: config.supabase.url!,
        SUPABASE_SERVICE_ROLE_KEY: config.supabase.serviceRoleKey!,
      },
      timeout: Duration.seconds(10),
      memorySize: 128,
    });

    // Main Backend Lambda (Go) - Match original b2-stack pattern
    this.backendLambda = new lambda.Function(this, 'BackendLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(path.join(__dirname, '../../../backend/build/main')),
      handler: 'bootstrap',
      memorySize: 128,
      timeout: Duration.seconds(30),
      environment: {
        TABLE_NAME: memoryTable.tableName,
        INDEX_NAME: 'KeywordIndex',
        EVENT_BUS_NAME: this.eventBus.eventBusName,
      },
    });

    // Node Connection Discovery Lambda (Go) - Match original b2-stack pattern
    this.connectNodeLambda = new lambda.Function(this, 'ConnectNodeLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(path.join(__dirname, '../../../backend/build/connect-node')),
      handler: 'bootstrap',
      memorySize: 128,
      timeout: Duration.seconds(30),
      environment: {
        TABLE_NAME: memoryTable.tableName,
        INDEX_NAME: 'KeywordIndex',
        EVENT_BUS_NAME: this.eventBus.eventBusName,
      },
    });

    // Cleanup Lambda (Go) - Async cleanup of node residuals
    this.cleanupLambda = new lambda.Function(this, 'CleanupLambda', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(path.join(__dirname, '../../../backend/build/cleanup-handler')),
      handler: 'bootstrap',
      memorySize: 256,  // More memory for batch operations
      timeout: Duration.seconds(60),  // Longer timeout for cleanup operations
      environment: {
        TABLE_NAME: memoryTable.tableName,
        INDEX_NAME: 'EdgeIndex',  // Use EdgeIndex for edge queries
        EVENT_BUS_NAME: this.eventBus.eventBusName,
      },
      // Note: No reserved concurrency - Lambda will auto-scale as needed
      // DynamoDB's built-in throttling will naturally limit request rate
    });

    // WebSocket Connect Lambda (Go) - Match original b2-stack pattern
    this.wsConnectLambda = new lambda.Function(this, 'wsConnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../../backend/build/ws-connect')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            SUPABASE_URL: config.supabase.url!,
            SUPABASE_SERVICE_ROLE_KEY: config.supabase.serviceRoleKey!,
        },
    });

    // WebSocket Disconnect Lambda (Go) - Match original b2-stack pattern
    this.wsDisconnectLambda = new lambda.Function(this, 'wsDisconnectLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../../backend/build/ws-disconnect')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
            CONNECTIONS_GSI_NAME: 'connection-id-index',
        },
    });

    // WebSocket Send Message Lambda (Go) - Match original b2-stack pattern
    this.wsSendMessageLambda = new lambda.Function(this, 'wsSendMessageLambda', {
        runtime: lambda.Runtime.PROVIDED_AL2,
        code: lambda.Code.fromAsset(path.join(__dirname, '../../../backend/build/ws-send-message')),
        handler: 'bootstrap',
        memorySize: 128,
        timeout: Duration.seconds(10),
        environment: {
            CONNECTIONS_TABLE_NAME: connectionsTable.tableName,
        },
    });

    // Grant DynamoDB permissions
    memoryTable.grantReadWriteData(this.backendLambda);
    memoryTable.grantReadWriteData(this.connectNodeLambda);
    memoryTable.grantReadWriteData(this.cleanupLambda);  // Cleanup needs table access
    connectionsTable.grantWriteData(this.wsConnectLambda);
    connectionsTable.grantReadWriteData(this.wsDisconnectLambda);
    connectionsTable.grantReadData(this.wsSendMessageLambda);

    // Create WebSocket API Gateway - Moved from API Stack to resolve cyclic dependency
    this.webSocketApi = new Brain2WebSocketApi(this, 'WebSocketApi', {
      config,
      connectFunction: this.wsConnectLambda,
      disconnectFunction: this.wsDisconnectLambda,
      sendMessageFunction: this.wsSendMessageLambda,
    });

    // Add WebSocket API endpoint to send message function environment
    this.wsSendMessageLambda.addEnvironment('WEBSOCKET_API_ENDPOINT', this.webSocketApi.callbackUrl);

    // Grant EventBridge permissions
    this.eventBus.grantPutEventsTo(this.backendLambda);
    this.eventBus.grantPutEventsTo(this.connectNodeLambda);
    this.eventBus.grantPutEventsTo(this.cleanupLambda);  // Cleanup might publish events

    // EventBridge rule for NodeCreated events - Match original b2-stack pattern
    new events.Rule(this, 'NodeCreatedRule', {
        eventBus: this.eventBus,
        eventPattern: {
            source: ['brain2.api'],
            detailType: ['NodeCreated'],
        },
        targets: [new targets.LambdaFunction(this.connectNodeLambda)],
    });

    // EventBridge rule for EdgesCreated events - Match original b2-stack pattern
    new events.Rule(this, 'EdgesCreatedRule', {
        eventBus: this.eventBus,
        eventPattern: {
            source: ['brain2.connectNode'],
            detailType: ['EdgesCreated'],
        },
        targets: [new targets.LambdaFunction(this.wsSendMessageLambda)],
    });

    // EventBridge rule for NodeDeleted events - Triggers async cleanup
    new events.Rule(this, 'NodeDeletedRule', {
        eventBus: this.eventBus,
        eventPattern: {
            source: ['brain2-backend'],  // Matches the source used in EventBridgePublisher
            detailType: ['NodeDeleted'],
        },
        targets: [
            new targets.LambdaFunction(this.cleanupLambda, {
                retryAttempts: 2,  // Retry failed cleanups
                maxEventAge: Duration.hours(1),  // Don't retry events older than 1 hour
            }),
        ],
    });

    // Output WebSocket API URL for frontend configuration
    new CfnOutput(this, 'WebSocketApiUrl', {
      value: this.webSocketApi.url,
      description: 'The WebSocket URL for real-time updates (set as VITE_WEBSOCKET_URL)',
      exportName: `${config.stackName}-websocket-api-url`,
    });
  }
}