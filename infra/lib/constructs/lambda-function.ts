/**
 * Reusable Lambda function construct for Brain2 Go Lambda functions
 */

import { Duration, RemovalPolicy } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as logs from 'aws-cdk-lib/aws-logs';
import * as path from 'path';
import { EnvironmentConfig } from '../config/environments';
import { LAMBDA_CONFIG } from '../config/constants';

export interface GoLambdaFunctionProps {
  functionName: string;
  codePath: string;
  environment?: Record<string, string>;
  memorySize?: number;
  timeout?: Duration;
  description?: string;
  config: EnvironmentConfig;
}

/**
 * Standardized Go Lambda function construct with common configuration
 */
export class GoLambdaFunction extends Construct {
  public readonly function: lambda.Function;
  public readonly logGroup: logs.LogGroup;

  constructor(scope: Construct, id: string, props: GoLambdaFunctionProps) {
    super(scope, id);

    const functionName = `${props.config.resourcePrefix}-${props.functionName}`;

    // Create CloudWatch Log Group with retention policy
    this.logGroup = new logs.LogGroup(this, 'LogGroup', {
      logGroupName: `/aws/lambda/${functionName}`,
      retention: logs.RetentionDays.TWO_WEEKS,
      removalPolicy: props.config.dynamodb.removalPolicy === 'DESTROY' 
        ? RemovalPolicy.DESTROY 
        : RemovalPolicy.RETAIN,
    });

    // Create Lambda function - use stack context for path resolution
    this.function = new lambda.Function(this, 'Function', {
      functionName,
      runtime: lambda.Runtime.PROVIDED_AL2,
      code: lambda.Code.fromAsset(props.codePath), // Path should be resolved by caller
      handler: LAMBDA_CONFIG.HANDLER,
      memorySize: props.memorySize || props.config.lambda.memorySize,
      timeout: props.timeout || Duration.seconds(props.config.lambda.timeout),
      environment: {
        LOG_LEVEL: props.config.monitoring.logLevel,
        ...props.environment,
      },
      description: props.description,
      logGroup: this.logGroup,
    });
  }
}

export interface NodeLambdaFunctionProps {
  functionName: string;
  codePath: string;
  environment?: Record<string, string>;
  memorySize?: number;
  timeout?: Duration;
  description?: string;
  config: EnvironmentConfig;
}

/**
 * Standardized Node.js Lambda function construct (for authorizer)
 */
export class NodeLambdaFunction extends Construct {
  public readonly function: lambda.Function;
  public readonly logGroup: logs.LogGroup;

  constructor(scope: Construct, id: string, props: NodeLambdaFunctionProps) {
    super(scope, id);

    const functionName = `${props.config.resourcePrefix}-${props.functionName}`;

    // Create CloudWatch Log Group with retention policy
    this.logGroup = new logs.LogGroup(this, 'LogGroup', {
      logGroupName: `/aws/lambda/${functionName}`,
      retention: logs.RetentionDays.TWO_WEEKS,
      removalPolicy: props.config.dynamodb.removalPolicy === 'DESTROY' 
        ? RemovalPolicy.DESTROY 
        : RemovalPolicy.RETAIN,
    });

    // Create Lambda function - use stack context for path resolution
    this.function = new lambda.Function(this, 'Function', {
      functionName,
      runtime: lambda.Runtime.NODEJS_20_X,
      code: lambda.Code.fromAsset(props.codePath), // Path should be resolved by caller
      handler: 'index.handler',
      memorySize: props.memorySize || props.config.lambda.memorySize,
      timeout: props.timeout || Duration.seconds(10), // Shorter timeout for auth
      environment: {
        LOG_LEVEL: props.config.monitoring.logLevel,
        ...props.environment,
      },
      description: props.description,
      logGroup: this.logGroup,
    });
  }
}