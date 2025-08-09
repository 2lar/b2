import { 
  getEnvironmentConfig, 
  getCurrentEnvironment, 
  environments 
} from '../../../lib/config/environments';

// Mock process.env for testing
const originalEnv = process.env;

describe('Environment Configuration', () => {
  beforeEach(() => {
    // Reset environment variables
    jest.resetModules();
    process.env = { ...originalEnv };
  });

  afterAll(() => {
    // Restore original environment
    process.env = originalEnv;
  });

  describe('environments object', () => {
    test('contains development environment configuration', () => {
      expect(environments.development).toBeDefined();
      expect(environments.development.stackName).toBe('b2-dev');
      expect(environments.development.region).toBe('us-east-1');
      expect(environments.development.dynamodb.removalPolicy).toBe('DESTROY');
    });

    test('contains staging environment configuration', () => {
      expect(environments.staging).toBeDefined();
      expect(environments.staging.stackName).toBe('b2-staging');
      expect(environments.staging.region).toBe('us-east-1');
      expect(environments.staging.dynamodb.removalPolicy).toBe('RETAIN');
    });

    test('contains production environment configuration', () => {
      expect(environments.production).toBeDefined();
      expect(environments.production.stackName).toBe('b2-prod');
      expect(environments.production.region).toBe('us-east-1');
      expect(environments.production.dynamodb.removalPolicy).toBe('RETAIN');
    });

    test('all environments have required properties', () => {
      Object.values(environments).forEach(env => {
        expect(env.stackName).toBeDefined();
        expect(env.region).toBeDefined();
        expect(env.dynamodb).toBeDefined();
        expect(env.dynamodb.removalPolicy).toBeDefined();
        expect(env.supabase).toBeDefined();
        expect(env.monitoring).toBeDefined();
        expect(env.cors).toBeDefined();
      });
    });
  });

  describe('getCurrentEnvironment', () => {
    test('returns development by default when no NODE_ENV is set', () => {
      delete process.env.NODE_ENV;
      
      const env = getCurrentEnvironment();
      
      expect(env).toBe('development');
    });

    test('returns correct environment when NODE_ENV is set', () => {
      process.env.NODE_ENV = 'production';
      
      const env = getCurrentEnvironment();
      
      expect(env).toBe('production');
    });

    test('returns development for unrecognized NODE_ENV values', () => {
      process.env.NODE_ENV = 'unknown-environment';
      
      const env = getCurrentEnvironment();
      
      expect(env).toBe('development');
    });

    test('handles staging environment', () => {
      process.env.NODE_ENV = 'staging';
      
      const env = getCurrentEnvironment();
      
      expect(env).toBe('staging');
    });
  });

  describe('getEnvironmentConfig', () => {
    test('returns correct configuration for development', () => {
      const config = getEnvironmentConfig('development');
      
      expect(config.stackName).toBe('b2-dev');
      expect(config.region).toBe('us-east-1');
      expect(config.dynamodb.removalPolicy).toBe('DESTROY');
    });

    test('returns correct configuration for production', () => {
      const config = getEnvironmentConfig('production');
      
      expect(config.stackName).toBe('b2-prod');
      expect(config.region).toBe('us-east-1');
      expect(config.dynamodb.removalPolicy).toBe('RETAIN');
    });

    test('returns correct configuration for staging', () => {
      const config = getEnvironmentConfig('staging');
      
      expect(config.stackName).toBe('b2-staging');
      expect(config.region).toBe('us-east-1');
      expect(config.dynamodb.removalPolicy).toBe('RETAIN');
    });

    test('includes all required configuration properties', () => {
      const config = getEnvironmentConfig('development');
      
      expect(config.stackName).toBeDefined();
      expect(config.region).toBeDefined();
      expect(config.dynamodb).toBeDefined();
      expect(config.supabase).toBeDefined();
      expect(config.monitoring).toBeDefined();
      expect(config.cors).toBeDefined();
    });

    test('supabase configuration includes URL and keys from environment', () => {
      process.env.SUPABASE_URL = 'https://test.supabase.co';
      process.env.SUPABASE_SERVICE_ROLE_KEY = 'test-key';
      
      const config = getEnvironmentConfig('development');
      
      expect(config.supabase.url).toBe('https://test.supabase.co');
      expect(config.supabase.serviceRoleKey).toBe('test-key');
    });

    test('monitoring configuration varies by environment', () => {
      const devConfig = getEnvironmentConfig('development');
      const prodConfig = getEnvironmentConfig('production');
      
      expect(devConfig.monitoring.enableDashboards).toBe(false);
      expect(devConfig.monitoring.enableAlarms).toBe(false);
      
      expect(prodConfig.monitoring.enableDashboards).toBe(true);
      expect(prodConfig.monitoring.enableAlarms).toBe(true);
    });

    test('CORS configuration includes allowed origins', () => {
      const config = getEnvironmentConfig('development');
      
      expect(config.cors.allowOrigins).toContain('http://localhost:5173');
      expect(Array.isArray(config.cors.allowOrigins)).toBe(true);
    });
  });

  describe('environment-specific behavior', () => {
    test('development environment allows localhost origins', () => {
      const config = getEnvironmentConfig('development');
      
      expect(config.cors.allowOrigins).toContain('http://localhost:5173');
    });

    test('production environment has stricter settings', () => {
      const config = getEnvironmentConfig('production');
      
      expect(config.dynamodb.removalPolicy).toBe('RETAIN');
      expect(config.monitoring.enableAlarms).toBe(true);
    });

    test('staging environment mirrors production settings', () => {
      const stagingConfig = getEnvironmentConfig('staging');
      const prodConfig = getEnvironmentConfig('production');
      
      expect(stagingConfig.dynamodb.removalPolicy).toBe(prodConfig.dynamodb.removalPolicy);
      expect(stagingConfig.monitoring.enableAlarms).toBe(prodConfig.monitoring.enableAlarms);
    });
  });
});