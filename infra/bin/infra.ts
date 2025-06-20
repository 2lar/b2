#!/usr/bin/env node
import 'dotenv/config'; // Loads variables from .env into process.env

import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { b2Stack } from '../lib/b2-stack';

const app = new cdk.App();

new b2Stack(app, 'b2Stack', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
  description: 'Brain2 - Graph-based knowledge management system',
});