import { App } from 'aws-cdk-lib';
import { Template, Match } from 'aws-cdk-lib/assertions';
import { ComputeStack } from '../../../lib/stacks/compute-stack';
import { DatabaseStack } from '../../../lib/stacks/database-stack';
import { getEnvironmentConfig } from '../../../lib/config/environments';

describe('ComputeStack', () => {
  let app: App;
  let databaseStack: DatabaseStack;

  beforeEach(() => {
    app = new App();
    const config = getEnvironmentConfig('development');
    
    databaseStack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });
  });

  test('creates all Lambda functions with correct configuration', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Should create 6 Lambda functions
    template.resourceCountIs('AWS::Lambda::Function', 6);
    
    // Check JWT Authorizer Lambda (Node.js)
    template.hasResourceProperties('AWS::Lambda::Function', {
      FunctionName: `${config.stackName}-jwt-authorizer`,
      Runtime: 'nodejs20.x',
      Handler: 'index.handler',
      MemorySize: 128,
      Timeout: 10
    });

    // Check Backend Lambda (Go)
    template.hasResourceProperties('AWS::Lambda::Function', {
      Runtime: 'provided.al2',
      Handler: 'bootstrap',
      MemorySize: 128,
      Timeout: 30
    });
  });

  test('creates EventBridge event bus', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::Events::EventBus', {
      Name: 'B2EventBus'
    });
  });

  test('creates EventBridge rules for event handling', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Should create 2 EventBridge rules
    template.resourceCountIs('AWS::Events::Rule', 2);
    
    // Check NodeCreated rule
    template.hasResourceProperties('AWS::Events::Rule', {
      EventPattern: {
        'source': ['brain2.api'],
        'detail-type': ['NodeCreated']
      }
    });

    // Check EdgesCreated rule
    template.hasResourceProperties('AWS::Events::Rule', {
      EventPattern: {
        'source': ['brain2.connectNode'],
        'detail-type': ['EdgesCreated']
      }
    });
  });

  test('creates WebSocket API with correct configuration', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::ApiGatewayV2::Api', {
      Name: 'B2WebSocketApi',
      ProtocolType: 'WEBSOCKET'
    });
  });

  test('grants correct DynamoDB permissions', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Check IAM policies are created for DynamoDB access
    template.hasResourceProperties('AWS::IAM::Policy', {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Effect: 'Allow',
            Action: Match.arrayWith(['dynamodb:*'])
          })
        ])
      }
    });
  });

  test('grants EventBridge permissions to Lambda functions', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Check IAM policies for EventBridge permissions
    template.hasResourceProperties('AWS::IAM::Policy', {
      PolicyDocument: {
        Statement: Match.arrayWith([
          Match.objectLike({
            Effect: 'Allow',
            Action: 'events:PutEvents'
          })
        ])
      }
    });
  });

  test('sets correct environment variables for Lambda functions', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Check environment variables are set
    template.hasResourceProperties('AWS::Lambda::Function', {
      Environment: {
        Variables: Match.objectLike({
          TABLE_NAME: Match.anyValue(),
          KEYWORD_INDEX_NAME: 'KeywordIndex',
          EVENT_BUS_NAME: 'B2EventBus'
        })
      }
    });

    // Check WebSocket functions have connections table environment
    template.hasResourceProperties('AWS::Lambda::Function', {
      Environment: {
        Variables: Match.objectLike({
          CONNECTIONS_TABLE_NAME: Match.anyValue()
        })
      }
    });
  });

  test('exposes all required public properties', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    expect(stack.backendLambda).toBeDefined();
    expect(stack.connectNodeLambda).toBeDefined();
    expect(stack.wsConnectLambda).toBeDefined();
    expect(stack.wsDisconnectLambda).toBeDefined();
    expect(stack.wsSendMessageLambda).toBeDefined();
    expect(stack.authorizerLambda).toBeDefined();
    expect(stack.eventBus).toBeDefined();
    expect(stack.webSocketApi).toBeDefined();
  });

  test('outputs WebSocket API URL', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasOutput('WebSocketApiUrl', {
      Description: 'The WebSocket URL for real-time updates (set as VITE_WEBSOCKET_URL)'
    });
  });

  test('matches snapshot', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new ComputeStack(app, 'TestComputeStack', {
      config,
      stackName: 'test-compute-stack',
      memoryTable: databaseStack.memoryTable,
      connectionsTable: databaseStack.connectionsTable
    });

    // Assert
    const template = Template.fromStack(stack);
    expect(template.toJSON()).toMatchSnapshot();
  });
});