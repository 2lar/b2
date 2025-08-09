/**
 * Frontend Stack - S3 bucket and CloudFront distribution for Brain2
 */

import { Stack, StackProps, RemovalPolicy, Duration, CfnOutput } from 'aws-cdk-lib';
import { Construct } from 'constructs';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as s3deploy from 'aws-cdk-lib/aws-s3-deployment';
import * as path from 'path';
import { EnvironmentConfig } from '../config/environments';
import { RESOURCE_NAMES, FRONTEND_CONFIG, getBucketName } from '../config/constants';

export interface FrontendStackProps extends StackProps {
  config: EnvironmentConfig;
}

export class FrontendStack extends Stack {
  public readonly bucket: s3.Bucket;
  public readonly distribution: cloudfront.Distribution;

  constructor(scope: Construct, id: string, props: FrontendStackProps) {
    super(scope, id, props);

    const { config } = props;
    const removalPolicy = config.dynamodb.removalPolicy === 'DESTROY' 
      ? RemovalPolicy.DESTROY 
      : RemovalPolicy.RETAIN;

    // S3 bucket for frontend static assets - Match original b2-stack
    this.bucket = new s3.Bucket(this, 'FrontendBucket', {
      bucketName: `b2-frontend-${this.account}-${this.region}`,
      publicReadAccess: false,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy,
      autoDeleteObjects: removalPolicy === RemovalPolicy.DESTROY,
      versioned: config.stackName.includes('prod'), // Enable versioning for production
      lifecycleRules: config.stackName.includes('prod') ? [{
        id: 'DeleteOldVersions',
        enabled: true,
        noncurrentVersionExpiration: Duration.days(30),
      }] : undefined,
    });

    // CloudFront cache behaviors
    const defaultBehavior: cloudfront.BehaviorOptions = {
      origin: new origins.S3Origin(this.bucket),
      viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
      cachePolicy: cloudfront.CachePolicy.CACHING_OPTIMIZED,
      compress: true,
    };

    // Additional cache behavior for API calls (no caching)
    const apiCacheBehavior: cloudfront.BehaviorOptions = {
      origin: new origins.S3Origin(this.bucket),
      viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
      cachePolicy: cloudfront.CachePolicy.CACHING_DISABLED,
      allowedMethods: cloudfront.AllowedMethods.ALLOW_ALL,
    };

    // CloudFront distribution for serving frontend content
    this.distribution = new cloudfront.Distribution(this, 'FrontendDistribution', {
      comment: `Brain2 Frontend Distribution - ${config.stackName}`,
      defaultBehavior,
      additionalBehaviors: {
        '/api/*': apiCacheBehavior,
      },
      defaultRootObject: FRONTEND_CONFIG.ROOT_OBJECT,
      // Error handling for SPA client-side routing - Match original b2-stack
      errorResponses: [{ 
        httpStatus: 404,
        responseHttpStatus: 200,
        responsePagePath: '/index.html',
        ttl: Duration.minutes(5)
      }],
      // Security headers (will add custom policy if needed)
      // Enable HTTP/2
      httpVersion: cloudfront.HttpVersion.HTTP2,
      // Price class based on environment
      priceClass: config.stackName.includes('prod') 
        ? cloudfront.PriceClass.PRICE_CLASS_ALL 
        : cloudfront.PriceClass.PRICE_CLASS_100,
    });

    // Automated frontend deployment to S3 and CloudFront - Match original b2-stack
    new s3deploy.BucketDeployment(this, 'DeployFrontend', {
      sources: [s3deploy.Source.asset(path.join(__dirname, '../../../frontend/dist'))],
      destinationBucket: this.bucket,
      distribution: this.distribution,
      distributionPaths: ['/*'],
    });

    // Output CloudFront URL
    new CfnOutput(this, 'CloudFrontUrl', {
      value: `https://${this.distribution.distributionDomainName}`,
      description: 'The public URL of your Brain2 application',
      exportName: `${config.stackName}-cloudfront-url`,
    });

    new CfnOutput(this, 'DistributionId', {
      value: this.distribution.distributionId,
      description: 'CloudFront distribution ID for cache invalidation',
      exportName: `${config.stackName}-distribution-id`,
    });
  }
}