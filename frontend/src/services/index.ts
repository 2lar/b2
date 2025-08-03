import type { components } from '../types/generated/generated-types';

// Barrel export for all services
export { api } from './apiClient';
export { auth } from './authClient';
export { webSocketClient } from './webSocketClient';

// Re-export commonly used types
export type { 
    components,
    operations 
} from '../types/generated/generated-types';

// Export specific types that components commonly use
export type Node = components['schemas']['Node'];
export type NodeDetails = components['schemas']['NodeDetails'];
export type Category = components['schemas']['Category'];
export type GraphDataResponse = components['schemas']['GraphDataResponse'];