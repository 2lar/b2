import { App, Stack } from 'aws-cdk-lib';
import { Template, Match } from 'aws-cdk-lib/assertions';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import { Brain2HttpApi } from '../../../lib/constructs/api-gateway';
import { getEnvironmentConfig } from '../../../lib/config/environments';

describe('Brain2HttpApi Construct', () => {
  let app: App;
  let stack: Stack;
  let mockBackendFunction: lambda.Function;
  let mockAuthorizerFunction: lambda.Function;

  beforeEach(() => {
    app = new App();
    stack = new Stack(app, 'TestStack', {
      env: { account: '123456789012', region: 'us-east-1' }
    });

    // Create mock Lambda functions
    mockBackendFunction = new lambda.Function(stack, 'MockBackend', {
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset('/tmp'),
      handler: 'bootstrap'
    });

    mockAuthorizerFunction = new lambda.Function(stack, 'MockAuthorizer', {
      runtime: lambda.Runtime.NODEJS_20_X,
      code: lambda.Code.fromAsset('/tmp'),
      handler: 'index.handler'
    });
  });

  test('creates HTTP API Gateway with correct configuration', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2HttpApi(stack, 'TestHttpApi', {
      config,
      authorizerFunction: mockAuthorizerFunction,
      backendFunction: mockBackendFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::ApiGatewayV2::Api', {
      Name: 'B2HttpApi',
      ProtocolType: 'HTTP',
      CorsConfiguration: {
        AllowMethods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
        AllowHeaders: Match.arrayWith(['Content-Type', 'Authorization']),
        AllowOrigins: ['http://localhost:5173']
      }
    });
  });

  test('creates JWT authorizer with correct configuration', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2HttpApi(stack, 'TestHttpApi', {
      config,
      authorizerFunction: mockAuthorizerFunction,
      backendFunction: mockBackendFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::ApiGatewayV2::Authorizer', {
      AuthorizerType: 'REQUEST',
      Name: 'JWTAuthorizer'
    });
  });

  test('creates API routes with Lambda integrations', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2HttpApi(stack, 'TestHttpApi', {
      config,
      authorizerFunction: mockAuthorizerFunction,
      backendFunction: mockBackendFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Check for Lambda integration
    template.hasResourceProperties('AWS::ApiGatewayV2::Integration', {
      IntegrationType: 'AWS_PROXY',
      PayloadFormatVersion: '2.0'
    });

    // Check for routes
    template.hasResourceProperties('AWS::ApiGatewayV2::Route', {
      RouteKey: '{proxy+}'
    });
  });

  test('creates deployment stage', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2HttpApi(stack, 'TestHttpApi', {
      config,
      authorizerFunction: mockAuthorizerFunction,
      backendFunction: mockBackendFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::ApiGatewayV2::Stage', {
      StageName: 'prod',
      AutoDeploy: true
    });
  });

  test('matches snapshot', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    new Brain2HttpApi(stack, 'TestHttpApi', {
      config,
      authorizerFunction: mockAuthorizerFunction,
      backendFunction: mockBackendFunction
    });

    // Assert
    const template = Template.fromStack(stack);
    expect(template.toJSON()).toMatchSnapshot();
  });
});