import { App, Stack } from 'aws-cdk-lib';
import { Template, Match } from 'aws-cdk-lib/assertions';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import { Brain2WebSocketApi } from '../../../lib/constructs/websocket-api';
import { getEnvironmentConfig } from '../../../lib/config/environments';

describe('Brain2WebSocketApi Construct', () => {
  let app: App;
  let stack: Stack;
  let mockConnectFunction: lambda.Function;
  let mockDisconnectFunction: lambda.Function;
  let mockSendMessageFunction: lambda.Function;

  beforeEach(() => {
    app = new App();
    stack = new Stack(app, 'TestStack', {
      env: { account: '123456789012', region: 'us-east-1' }
    });

    // Create mock Lambda functions
    mockConnectFunction = new lambda.Function(stack, 'MockConnect', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset('/tmp'),
      handler: 'bootstrap'
    });

    mockDisconnectFunction = new lambda.Function(stack, 'MockDisconnect', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset('/tmp'),
      handler: 'bootstrap'
    });

    mockSendMessageFunction = new lambda.Function(stack, 'MockSendMessage', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset('/tmp'),
      handler: 'bootstrap'
    });
  });

  test('creates WebSocket API with correct configuration', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2WebSocketApi(stack, 'TestWebSocketApi', {
      config,
      connectFunction: mockConnectFunction,
      disconnectFunction: mockDisconnectFunction,
      sendMessageFunction: mockSendMessageFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::ApiGatewayV2::Api', {
      Name: 'B2WebSocketApi',
      ProtocolType: 'WEBSOCKET',
      Description: 'Brain2 WebSocket API for real-time updates'
    });
  });

  test('creates connect and disconnect routes', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2WebSocketApi(stack, 'TestWebSocketApi', {
      config,
      connectFunction: mockConnectFunction,
      disconnectFunction: mockDisconnectFunction,
      sendMessageFunction: mockSendMessageFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Check for $connect route
    template.hasResourceProperties('AWS::ApiGatewayV2::Route', {
      RouteKey: '$connect'
    });

    // Check for $disconnect route
    template.hasResourceProperties('AWS::ApiGatewayV2::Route', {
      RouteKey: '$disconnect'
    });
  });

  test('creates Lambda integrations for routes', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2WebSocketApi(stack, 'TestWebSocketApi', {
      config,
      connectFunction: mockConnectFunction,
      disconnectFunction: mockDisconnectFunction,
      sendMessageFunction: mockSendMessageFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::ApiGatewayV2::Integration', {
      IntegrationType: 'AWS_PROXY'
    });
  });

  test('creates deployment stage with auto-deploy', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2WebSocketApi(stack, 'TestWebSocketApi', {
      config,
      connectFunction: mockConnectFunction,
      disconnectFunction: mockDisconnectFunction,
      sendMessageFunction: mockSendMessageFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::ApiGatewayV2::Stage', {
      StageName: 'prod',
      AutoDeploy: true
    });
  });

  test('grants management permissions to send message function', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2WebSocketApi(stack, 'TestWebSocketApi', {
      config,
      connectFunction: mockConnectFunction,
      disconnectFunction: mockDisconnectFunction,
      sendMessageFunction: mockSendMessageFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Check that IAM policy is created for API management permissions
    template.hasResourceProperties('AWS::IAM::Policy', {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Effect: 'Allow',
            Action: 'execute-api:ManageConnections'
          })
        ])
      }
    });
  });

  test('exposes correct URL and callback URL getters', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const webSocketApi = new Brain2WebSocketApi(stack, 'TestWebSocketApi', {
      config,
      connectFunction: mockConnectFunction,
      disconnectFunction: mockDisconnectFunction,
      sendMessageFunction: mockSendMessageFunction
    });

    // Assert
    expect(webSocketApi.url).toBeDefined();
    expect(webSocketApi.callbackUrl).toBeDefined();
    expect(typeof webSocketApi.url).toBe('string');
    expect(typeof webSocketApi.callbackUrl).toBe('string');
  });

  test('matches snapshot', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2WebSocketApi(stack, 'TestWebSocketApi', {
      config,
      connectFunction: mockConnectFunction,
      disconnectFunction: mockDisconnectFunction,
      sendMessageFunction: mockSendMessageFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    expect(template.toJSON()).toMatchSnapshot();
  });
});