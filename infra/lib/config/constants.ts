/**
 * Shared constants and configuration values for Brain2 infrastructure
 */

export const RESOURCE_NAMES = {
  // DynamoDB
  MEMORY_TABLE: 'brain2',
  CONNECTIONS_TABLE: 'B2-Connections',
  KEYWORD_INDEX: 'KeywordIndex',
  EDGE_INDEX: 'EdgeIndex',
  CONNECTION_INDEX: 'connection-id-index',
  
  // EventBridge
  EVENT_BUS: 'B2EventBus',
  
  // Lambda Functions
  BACKEND_LAMBDA: 'backend',
  CONNECT_NODE_LAMBDA: 'connect-node',
  WS_CONNECT_LAMBDA: 'ws-connect',
  WS_DISCONNECT_LAMBDA: 'ws-disconnect',
  WS_SEND_MESSAGE_LAMBDA: 'ws-send-message',
  JWT_AUTHORIZER_LAMBDA: 'jwt-authorizer',
  
  // APIs
  HTTP_API: 'b2-http-api',
  WEBSOCKET_API: 'B2WebSocketApi',
  
  // Frontend
  FRONTEND_BUCKET_SUFFIX: 'frontend',
  DISTRIBUTION_SUFFIX: 'distribution',
} as const;

export const LAMBDA_CONFIG = {
  RUNTIME: 'provided.al2' as const,
  HANDLER: 'bootstrap',
  DEFAULT_MEMORY: 128,
  DEFAULT_TIMEOUT: 30,
  
  // Go Lambda specific paths
  BACKEND_PATH: '../../backend/build/lambda',
  CONNECT_NODE_PATH: '../../backend/build/connect-node',
  WS_CONNECT_PATH: '../../backend/build/ws-connect',
  WS_DISCONNECT_PATH: '../../backend/build/ws-disconnect',
  WS_SEND_MESSAGE_PATH: '../../backend/build/ws-send-message',
  CLEANUP_HANDLER_PATH: '../../backend/build/cleanup-handler',
  
  // Node.js Lambda paths  
  AUTHORIZER_PATH: '../../lambda/authorizer',
} as const;

export const API_CONFIG = {
  CORS_HEADERS: ['Content-Type', 'Authorization'],
  CORS_METHODS: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
  CACHE_TTL_MINUTES: 5,
  WEBSOCKET_STAGE: 'prod',
} as const;

export const DYNAMODB_CONFIG = {
  PARTITION_KEY: 'PK',
  SORT_KEY: 'SK',
  GSI1_PARTITION_KEY: 'GSI1PK',
  GSI1_SORT_KEY: 'GSI1SK',
  GSI2_PARTITION_KEY: 'GSI2PK',
  GSI2_SORT_KEY: 'GSI2SK',
  TTL_ATTRIBUTE: 'expireAt',
} as const;

export const FRONTEND_CONFIG = {
  ROOT_OBJECT: 'index.html',
  ERROR_CACHE_TTL_MINUTES: 5,
  FRONTEND_DIST_PATH: '../../frontend/dist',
} as const;

export const EVENT_PATTERNS = {
  NODE_CREATED: {
    source: ['brain2.api'],
    detailType: ['NodeCreated'],
  },
  EDGES_CREATED: {
    source: ['brain2.connectNode'],
    detailType: ['EdgesCreated'],
  },
} as const;

export const MONITORING_CONFIG = {
  DASHBOARD_PERIOD_MINUTES: 5,
  ALARM_EVALUATION_PERIODS: 2,
  ALARM_THRESHOLD_ERROR_RATE: 0.05, // 5%
  ALARM_THRESHOLD_DURATION_MS: 5000, // 5 seconds
} as const;

/**
 * Helper function to generate resource names with environment prefix
 */
export function getResourceName(baseName: string, prefix: string): string {
  return `${prefix}-${baseName}`;
}

/**
 * Helper function to generate unique bucket names
 */
export function getBucketName(baseName: string, account: string, region: string, prefix: string): string {
  return `${prefix}-${baseName}-${account}-${region}`;
}