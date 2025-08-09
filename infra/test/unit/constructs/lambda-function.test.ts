import { App, Stack } from 'aws-cdk-lib';
import { Template } from 'aws-cdk-lib/assertions';
import { GoLambdaFunction, NodeLambdaFunction } from '../../../lib/constructs/lambda-function';
import { getEnvironmentConfig } from '../../../lib/config/environments';

describe('Lambda Function Constructs', () => {
  let app: App;
  let stack: Stack;
  let mockConfig: any;

  beforeEach(() => {
    app = new App();
    stack = new Stack(app, 'TestStack', {
      env: { account: '123456789012', region: 'us-east-1' }
    });
    mockConfig = getEnvironmentConfig('development');
  });

  describe('GoLambdaFunction', () => {
    test('creates Go Lambda function with correct properties', () => {
      // Arrange & Act
      new GoLambdaFunction(stack, 'TestGoFunction', {
        functionName: 'test-go-function',
        codePath: '/test/path',
        config: mockConfig,
        environment: {
          TEST_VAR: 'test-value'
        }
      });

      // Assert
      const template = Template.fromStack(stack);
      
      template.hasResourceProperties('AWS::Lambda::Function', {
        Runtime: 'provided.al2',
        Handler: 'bootstrap',
        Environment: {
          Variables: {
            TEST_VAR: 'test-value'
          }
        }
      });
    });

    test('creates CloudWatch Log Group', () => {
      // Arrange & Act
      new GoLambdaFunction(stack, 'TestGoFunction', {
        functionName: 'test-function',
        codePath: '/test/path',
        config: mockConfig
      });

      // Assert
      const template = Template.fromStack(stack);
      
      template.hasResourceProperties('AWS::Logs::LogGroup', {
        RetentionInDays: 14
      });
    });
  });

  describe('NodeLambdaFunction', () => {
    test('creates Node.js Lambda function with correct properties', () => {
      // Arrange & Act
      new NodeLambdaFunction(stack, 'TestNodeFunction', {
        functionName: 'test-node-function',
        codePath: '/test/path',
        config: mockConfig,
        environment: {
          NODE_ENV: 'test'
        }
      });

      // Assert
      const template = Template.fromStack(stack);
      
      template.hasResourceProperties('AWS::Lambda::Function', {
        Runtime: 'nodejs20.x',
        Handler: 'index.handler',
        Environment: {
          Variables: {
            NODE_ENV: 'test'
          }
        }
      });
    });

    test('creates CloudWatch Log Group', () => {
      // Arrange & Act
      new NodeLambdaFunction(stack, 'TestNodeFunction', {
        functionName: 'test-function',
        codePath: '/test/path',
        config: mockConfig
      });

      // Assert
      const template = Template.fromStack(stack);
      
      template.hasResourceProperties('AWS::Logs::LogGroup', {
        RetentionInDays: 14
      });
    });
  });

  describe('Snapshot Tests', () => {
    test('Go Lambda function matches snapshot', () => {
      // Arrange & Act
      new GoLambdaFunction(stack, 'SnapshotGoFunction', {
        functionName: 'snapshot-go-function',
        codePath: '/test/path',
        config: mockConfig
      });

      // Assert
      const template = Template.fromStack(stack);
      expect(template.toJSON()).toMatchSnapshot();
    });

    test('Node.js Lambda function matches snapshot', () => {
      // Arrange & Act
      new NodeLambdaFunction(stack, 'SnapshotNodeFunction', {
        functionName: 'snapshot-node-function',
        codePath: '/test/path',
        config: mockConfig
      });

      // Assert
      const template = Template.fromStack(stack);
      expect(template.toJSON()).toMatchSnapshot();
    });
  });
});