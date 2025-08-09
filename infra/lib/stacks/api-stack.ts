/**
 * API Stack - HTTP and WebSocket APIs for Brain2
 */

import { Stack, StackProps, CfnOutput } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import { EnvironmentConfig } from '../config/environments';
import { Brain2HttpApi } from '../constructs/api-gateway';

export interface ApiStackProps extends StackProps {
  config: EnvironmentConfig;
  backendLambda: lambda.Function;
  authorizerLambda: lambda.Function;
}

export class ApiStack extends Stack {
  public readonly httpApi: Brain2HttpApi;

  constructor(scope: Construct, id: string, props: ApiStackProps) {
    super(scope, id, props);

    const {
      config,
      backendLambda,
      authorizerLambda,
    } = props;

    // Create HTTP API Gateway with JWT authorization
    this.httpApi = new Brain2HttpApi(this, 'HttpApi', {
      config,
      authorizerFunction: authorizerLambda,
      backendFunction: backendLambda,
    });

    // Note: WebSocket API has been moved to Compute Stack to resolve cyclic dependencies

    // Output API endpoints for frontend configuration
    new CfnOutput(this, 'HttpApiUrl', {
      value: this.httpApi.url,
      description: 'The base URL for HTTP API calls (set as VITE_API_BASE_URL)',
      exportName: `${config.stackName}-http-api-url`,
    });

    // Note: WebSocket API URL output moved to Compute Stack
  }
}