/**
 * Reusable WebSocket API construct for Brain2 real-time communication
 */

import { Construct } from 'constructs';
import * as apigwv2 from 'aws-cdk-lib/aws-apigatewayv2';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import { WebSocketLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { EnvironmentConfig } from '../config/environments';
import { API_CONFIG, getResourceName } from '../config/constants';

export interface Brain2WebSocketApiProps {
  config: EnvironmentConfig;
  connectFunction: lambda.Function;
  disconnectFunction: lambda.Function;
  sendMessageFunction: lambda.Function;
}

/**
 * WebSocket API Gateway for real-time communication
 */
export class Brain2WebSocketApi extends Construct {
  public readonly api: apigwv2.WebSocketApi;
  public readonly stage: apigwv2.WebSocketStage;

  constructor(scope: Construct, id: string, props: Brain2WebSocketApiProps) {
    super(scope, id);

    // Create WebSocket API - Match original b2-stack
    this.api = new apigwv2.WebSocketApi(this, 'B2WebSocketApi', {
      apiName: 'B2WebSocketApi',
      description: 'Brain2 WebSocket API for real-time updates',
      connectRouteOptions: {
        integration: new WebSocketLambdaIntegration('ConnectIntegration', props.connectFunction),
      },
      disconnectRouteOptions: {
        integration: new WebSocketLambdaIntegration('DisconnectIntegration', props.disconnectFunction),
      },
    });

    // Create deployment stage - Match original b2-stack  
    this.stage = new apigwv2.WebSocketStage(this, 'B2WebSocketStage', {
      webSocketApi: this.api,
      stageName: 'prod',
      autoDeploy: true,
    });

    // Grant management permissions to send message function
    this.api.grantManageConnections(props.sendMessageFunction);

    // Note: WEBSOCKET_API_ENDPOINT environment variable should be added by the caller 
    // after construction to avoid cyclic dependencies between stacks
  }

  /**
   * Get the WebSocket URL
   */
  public get url(): string {
    return this.stage.url;
  }

  /**
   * Get the callback URL for API management
   */
  public get callbackUrl(): string {
    return this.stage.callbackUrl;
  }
}