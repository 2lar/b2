/**
 * Database Stack - DynamoDB tables and indexes for Brain2
 */

import { Stack, StackProps, RemovalPolicy, Tags } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import { EnvironmentConfig } from '../config/environments';
import { RESOURCE_NAMES, DYNAMODB_CONFIG, getResourceName } from '../config/constants';

export interface DatabaseStackProps extends StackProps {
  config: EnvironmentConfig;
}

export class DatabaseStack extends Stack {
  public readonly memoryTable: dynamodb.Table;
  public readonly connectionsTable: dynamodb.Table;

  constructor(scope: Construct, id: string, props: DatabaseStackProps) {
    super(scope, id, props);

    const { config } = props;
    const removalPolicy = config.dynamodb.removalPolicy === 'DESTROY' 
      ? RemovalPolicy.DESTROY 
      : RemovalPolicy.RETAIN;

    // DynamoDB table for memory storage (nodes, edges, keywords) - Match original b2-stack
    this.memoryTable = new dynamodb.Table(this, 'MemoryTable', {
      tableName: 'brain2',
      partitionKey: { 
        name: DYNAMODB_CONFIG.PARTITION_KEY, 
        type: dynamodb.AttributeType.STRING 
      },
      sortKey: { 
        name: DYNAMODB_CONFIG.SORT_KEY, 
        type: dynamodb.AttributeType.STRING 
      },
      billingMode: config.dynamodb.billingMode === 'PAY_PER_REQUEST' 
        ? dynamodb.BillingMode.PAY_PER_REQUEST 
        : dynamodb.BillingMode.PROVISIONED,
      removalPolicy,
      pointInTimeRecovery: config.stackName.includes('prod'),
    });

    // Global Secondary Index for keyword-based search - Match original b2-stack
    this.memoryTable.addGlobalSecondaryIndex({
      indexName: 'KeywordIndex',
      partitionKey: { 
        name: DYNAMODB_CONFIG.GSI1_PARTITION_KEY, 
        type: dynamodb.AttributeType.STRING 
      },
      sortKey: { 
        name: DYNAMODB_CONFIG.GSI1_SORT_KEY, 
        type: dynamodb.AttributeType.STRING 
      },
      projectionType: dynamodb.ProjectionType.ALL,
    });

    // DynamoDB table for tracking WebSocket connections - Match original b2-stack
    this.connectionsTable = new dynamodb.Table(this, 'ConnectionsTable', {
      tableName: 'B2-Connections',
      partitionKey: { 
        name: DYNAMODB_CONFIG.PARTITION_KEY, 
        type: dynamodb.AttributeType.STRING 
      }, // PK: USER#{userId}
      sortKey: { 
        name: DYNAMODB_CONFIG.SORT_KEY, 
        type: dynamodb.AttributeType.STRING 
      }, // SK: CONN#{connectionId}
      billingMode: config.dynamodb.billingMode === 'PAY_PER_REQUEST' 
        ? dynamodb.BillingMode.PAY_PER_REQUEST 
        : dynamodb.BillingMode.PROVISIONED,
      removalPolicy,
      // TTL for automatic cleanup of stale connections
      timeToLiveAttribute: DYNAMODB_CONFIG.TTL_ATTRIBUTE,
      pointInTimeRecovery: config.stackName.includes('prod'),
    });

    // Global Secondary Index for finding user by connectionId - Match original b2-stack
    this.connectionsTable.addGlobalSecondaryIndex({
      indexName: 'connection-id-index',
      partitionKey: { 
        name: DYNAMODB_CONFIG.GSI1_PARTITION_KEY, 
        type: dynamodb.AttributeType.STRING 
      }, // GSI1PK: CONN#{connectionId}
      sortKey: { 
        name: DYNAMODB_CONFIG.GSI1_SORT_KEY, 
        type: dynamodb.AttributeType.STRING 
      }, // GSI1SK: USER#{userId}
      projectionType: dynamodb.ProjectionType.ALL,
    });

    // Tags for cost allocation and management
    const tags = {
      Environment: config.stackName,
      Service: 'brain2',
      Component: 'database',
    };

    // Apply tags to both tables
    Object.entries(tags).forEach(([key, value]) => {
      Tags.of(this.memoryTable).add(key, value);
      Tags.of(this.connectionsTable).add(key, value);
    });
  }
}