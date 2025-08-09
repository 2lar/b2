import { App } from 'aws-cdk-lib';
import { Template, Match } from 'aws-cdk-lib/assertions';
import { DatabaseStack } from '../../../lib/stacks/database-stack';
import { getEnvironmentConfig } from '../../../lib/config/environments';

describe('DatabaseStack', () => {
  let app: App;

  beforeEach(() => {
    app = new App();
  });

  test('creates DynamoDB tables with correct configuration', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Check memory table
    template.hasResourceProperties('AWS::DynamoDB::Table', {
      TableName: 'MemoryTable',
      BillingMode: 'PAY_PER_REQUEST',
      AttributeDefinitions: Match.arrayWith([
        { AttributeName: 'id', AttributeType: 'S' },
        { AttributeName: 'keyword', AttributeType: 'S' }
      ]),
      KeySchema: [
        { AttributeName: 'id', KeyType: 'HASH' }
      ]
    });

    // Check connections table
    template.hasResourceProperties('AWS::DynamoDB::Table', {
      TableName: 'ConnectionsTable',
      BillingMode: 'PAY_PER_REQUEST',
      AttributeDefinitions: Match.arrayWith([
        { AttributeName: 'connectionId', AttributeType: 'S' },
        { AttributeName: 'userId', AttributeType: 'S' }
      ]),
      KeySchema: [
        { AttributeName: 'connectionId', KeyType: 'HASH' }
      ]
    });
  });

  test('creates Global Secondary Index for memory table', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::DynamoDB::Table', {
      TableName: 'MemoryTable',
      GlobalSecondaryIndexes: [
        {
          IndexName: 'KeywordIndex',
          KeySchema: [
            { AttributeName: 'keyword', KeyType: 'HASH' }
          ],
          Projection: { ProjectionType: 'ALL' }
        }
      ]
    });
  });

  test('creates Global Secondary Index for connections table', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.hasResourceProperties('AWS::DynamoDB::Table', {
      TableName: 'ConnectionsTable',
      GlobalSecondaryIndexes: [
        {
          IndexName: 'connection-id-index',
          KeySchema: [
            { AttributeName: 'userId', KeyType: 'HASH' }
          ],
          Projection: { ProjectionType: 'ALL' }
        }
      ]
    });
  });

  test('applies correct removal policy for development environment', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Development should have DESTROY removal policy
    template.hasResource('AWS::DynamoDB::Table', {
      DeletionPolicy: 'Delete',
      UpdateReplacePolicy: 'Delete'
    });
  });

  test('applies correct removal policy for production environment', () => {
    // Arrange
    const config = getEnvironmentConfig('production');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    const template = Template.fromStack(stack);
    
    // Production should have RETAIN removal policy
    template.hasResource('AWS::DynamoDB::Table', {
      DeletionPolicy: 'Retain',
      UpdateReplacePolicy: 'Retain'
    });
  });

  test('exposes table resources correctly', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    expect(stack.memoryTable).toBeDefined();
    expect(stack.connectionsTable).toBeDefined();
    expect(stack.memoryTable.tableName).toBeTruthy();
    expect(stack.connectionsTable.tableName).toBeTruthy();
  });

  test('creates exactly two DynamoDB tables', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    const template = Template.fromStack(stack);
    
    template.resourceCountIs('AWS::DynamoDB::Table', 2);
  });

  test('matches snapshot', () => {
    // Arrange
    const config = getEnvironmentConfig('development');

    // Act
    const stack = new DatabaseStack(app, 'TestDatabaseStack', {
      config,
      stackName: 'test-database-stack'
    });

    // Assert
    const template = Template.fromStack(stack);
    expect(template.toJSON()).toMatchSnapshot();
  });
});