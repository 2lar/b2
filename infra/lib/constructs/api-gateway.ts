/**
 * Reusable API Gateway constructs for Brain2
 */

import { Duration } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as apigwv2 from 'aws-cdk-lib/aws-apigatewayv2';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import { HttpLambdaIntegration } from 'aws-cdk-lib/aws-apigatewayv2-integrations';
import { HttpLambdaAuthorizer, HttpLambdaResponseType } from 'aws-cdk-lib/aws-apigatewayv2-authorizers';
import { EnvironmentConfig } from '../config/environments';
import { API_CONFIG, getResourceName } from '../config/constants';

export interface Brain2HttpApiProps {
  config: EnvironmentConfig;
  authorizerFunction: lambda.Function;
  backendFunction: lambda.Function;
}

/**
 * HTTP API Gateway with CORS and JWT authorization
 */
export class Brain2HttpApi extends Construct {
  public readonly api: apigwv2.HttpApi;
  public readonly authorizer: HttpLambdaAuthorizer;

  constructor(scope: Construct, id: string, props: Brain2HttpApiProps) {
    super(scope, id);

    // Create HTTP API with CORS configuration - Match original b2-stack
    this.api = new apigwv2.HttpApi(this, 'b2HttpApi', {
      apiName: 'b2-http-api',
      description: 'Brain2 HTTP API for memory management',
      corsPreflight: {
        allowHeaders: [...API_CONFIG.CORS_HEADERS],
        allowMethods: API_CONFIG.CORS_METHODS.map(method => 
          apigwv2.CorsHttpMethod[method as keyof typeof apigwv2.CorsHttpMethod]
        ),
        allowOrigins: props.config.cors.allowOrigins,
        maxAge: Duration.days(1), // Match original pattern
      },
    });

    // Create JWT Lambda authorizer - Match original b2-stack
    this.authorizer = new HttpLambdaAuthorizer('SupabaseLambdaAuthorizer', props.authorizerFunction, {
      responseTypes: [HttpLambdaResponseType.SIMPLE],
      identitySource: ['$request.header.Authorization'],
      resultsCacheTtl: Duration.minutes(API_CONFIG.CACHE_TTL_MINUTES),
    });

    // Create backend integration
    const backendIntegration = new HttpLambdaIntegration('BackendIntegration', props.backendFunction);

    // Add API routes with authorization
    this.api.addRoutes({
      path: '/api/{proxy+}',
      methods: [
        apigwv2.HttpMethod.GET,
        apigwv2.HttpMethod.POST,
        apigwv2.HttpMethod.PUT,
        apigwv2.HttpMethod.DELETE,
      ],
      integration: backendIntegration,
      authorizer: this.authorizer,
    });

    // Add health check route without authorization
    this.api.addRoutes({
      path: '/health',
      methods: [apigwv2.HttpMethod.GET],
      integration: backendIntegration,
    });
  }

  /**
   * Get the API URL
   */
  public get url(): string {
    return this.api.url!;
  }
}