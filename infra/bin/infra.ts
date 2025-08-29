#!/usr/bin/env node
import { config as loadDotenv } from 'dotenv';
import { resolve } from 'path';

// Load environment variables from root .env file
loadDotenv({ path: resolve(__dirname, '../../.env') });

import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { MainStack } from '../lib/main-stack';
import { getEnvironmentConfig, getCurrentEnvironment } from '../lib/config/environments';

const app = new cdk.App();

// Get environment configuration
const environmentName = getCurrentEnvironment();
const config = getEnvironmentConfig(environmentName);

console.log(`Deploying Brain2 infrastructure for environment: ${environmentName}`);
console.log(`Stack name: ${config.stackName}`);
console.log(`Region: ${config.region}`);

new MainStack(app, 'Brain2Stack', {
  config,
  env: {
    account: config.account || process.env.CDK_DEFAULT_ACCOUNT,
    region: config.region,
  },
  description: `Brain2 - Graph-based knowledge management system (${environmentName})`,
  stackName: config.stackName,
});