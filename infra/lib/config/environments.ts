/**
 * Environment-specific configuration for Brain2 infrastructure
 */

export interface EnvironmentConfig {
  region: string;
  account?: string;
  
  // Resource naming
  stackName: string;
  resourcePrefix: string;
  
  // DynamoDB settings
  dynamodb: {
    billingMode: 'PAY_PER_REQUEST' | 'PROVISIONED';
    removalPolicy: 'DESTROY' | 'RETAIN';
  };
  
  // Lambda settings
  lambda: {
    memorySize: number;
    timeout: number;
    logRetention: number; // days
  };
  
  // CORS settings
  cors: {
    allowOrigins: string[];
    maxAge: number;
  };
  
  // Monitoring
  monitoring: {
    enableDashboards: boolean;
    enableAlarms: boolean;
    logLevel: 'ERROR' | 'WARN' | 'INFO' | 'DEBUG';
  };
  
  // External services
  supabase: {
    url?: string;
    serviceRoleKey?: string;
  };
}

const commonConfig = {
  lambda: {
    memorySize: 512, // Increased from 128MB for faster cold starts
    timeout: 60, // Increased from 30s to handle cold start initialization
    logRetention: 14,
  },
  cors: {
    maxAge: 86400, // 1 day
  },
};

export const environments: Record<string, EnvironmentConfig> = {
  development: {
    ...commonConfig,
    region: 'us-east-1',
    stackName: 'b2-dev',
    resourcePrefix: 'b2-dev',
    dynamodb: {
      billingMode: 'PAY_PER_REQUEST',
      removalPolicy: 'DESTROY',
    },
    cors: {
      ...commonConfig.cors,
      allowOrigins: ['*'], // Allow all origins in dev
    },
    monitoring: {
      enableDashboards: false,
      enableAlarms: false,
      logLevel: 'DEBUG',
    },
    supabase: {
      url: process.env.SUPABASE_URL,
      serviceRoleKey: process.env.SUPABASE_SERVICE_ROLE_KEY,
    },
  },
  
  staging: {
    ...commonConfig,
    region: 'us-east-1',
    stackName: 'b2-staging',
    resourcePrefix: 'b2-staging',
    dynamodb: {
      billingMode: 'PAY_PER_REQUEST',
      removalPolicy: 'RETAIN',
    },
    cors: {
      ...commonConfig.cors,
      allowOrigins: ['https://*.brain2-staging.com'],
    },
    monitoring: {
      enableDashboards: true,
      enableAlarms: true,
      logLevel: 'INFO',
    },
    supabase: {
      url: process.env.SUPABASE_URL_STAGING,
      serviceRoleKey: process.env.SUPABASE_SERVICE_ROLE_KEY_STAGING,
    },
  },
  
  production: {
    ...commonConfig,
    region: 'us-east-1',
    stackName: 'b2-prod',
    resourcePrefix: 'b2-prod',
    lambda: {
      memorySize: 1024, // Increased from 256MB for production performance
      timeout: 60, // Increased from 30s to handle cold start initialization
      logRetention: 30,
    },
    dynamodb: {
      billingMode: 'PAY_PER_REQUEST',
      removalPolicy: 'RETAIN',
    },
    cors: {
      ...commonConfig.cors,
      allowOrigins: ['https://brain2.com', 'https://www.brain2.com'],
    },
    monitoring: {
      enableDashboards: true,
      enableAlarms: true,
      logLevel: 'WARN',
    },
    supabase: {
      url: process.env.SUPABASE_URL_PROD,
      serviceRoleKey: process.env.SUPABASE_SERVICE_ROLE_KEY_PROD,
    },
  },
};

/**
 * Get environment configuration
 */
export function getEnvironmentConfig(env: string = 'development'): EnvironmentConfig {
  const config = environments[env];
  
  if (!config) {
    throw new Error(`Unknown environment: ${env}. Available: ${Object.keys(environments).join(', ')}`);
  }
  
  // Validate required environment variables
  if (!config.supabase.url || !config.supabase.serviceRoleKey) {
    throw new Error(`Missing required Supabase configuration for environment: ${env}`);
  }
  
  return config;
}

/**
 * Get current environment from NODE_ENV or CDK context
 */
export function getCurrentEnvironment(): string {
  return process.env.NODE_ENV || process.env.CDK_DEFAULT_ENV || 'development';
}