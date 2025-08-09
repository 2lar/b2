// Jest setup file for AWS CDK testing
// This file is executed before each test suite

// Increase timeout for CDK synthesis operations
jest.setTimeout(30000);

// Mock AWS credentials for testing
process.env.AWS_ACCOUNT_ID = '123456789012';
process.env.AWS_DEFAULT_REGION = 'us-east-1';
process.env.CDK_DEFAULT_ACCOUNT = '123456789012';
process.env.CDK_DEFAULT_REGION = 'us-east-1';

// Mock Supabase credentials for testing
process.env.SUPABASE_URL = 'https://test.supabase.co';
process.env.SUPABASE_SERVICE_ROLE_KEY = 'test-service-role-key';