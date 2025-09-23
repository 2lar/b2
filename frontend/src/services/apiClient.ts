import { auth } from './authClient';
import { components, operations } from '../types/generated/generated-types';

const isDevMode = import.meta.env.DEV;

class ApiError extends Error {
    public status: number;

    constructor(message: string, status: number) {
        super(message);
        this.name = 'ApiError';
        this.status = status;
    }
}

// Dynamic API Configuration
function getApiBaseUrl(): string {
    const forceProduction = import.meta.env.VITE_FORCE_PRODUCTION_API?.toLowerCase() === 'true';
    const useLocalServer = !forceProduction && import.meta.env.VITE_USE_LOCAL_API?.toLowerCase() === 'true';

    if (useLocalServer) {
        // Use local development URL from environment
        const localUrl = import.meta.env.VITE_API_BASE_URL_LOCAL;
        if (!localUrl || localUrl === 'undefined') {
            throw new Error('VITE_API_BASE_URL_LOCAL is not defined in .env file');
        }
        return localUrl;
    }

    // Use production URL from environment
    const prodUrl = import.meta.env.VITE_API_BASE_URL;
    if (!prodUrl || prodUrl === 'undefined') {
        throw new Error('VITE_API_BASE_URL is not defined in .env file');
    }
    return prodUrl;
}

// API Base URL with dynamic configuration
const API_BASE_URL = getApiBaseUrl();

// Type definitions generated from OpenAPI specification

// Core Memory Node Types
type Node = components['schemas']['Node'];
type NodeDetails = components['schemas']['NodeDetails'];

// Category Types
type Category = components['schemas']['Category'];

// Request/Response Types for Bulk Operations
type BulkDeleteResponse = components['schemas']['BulkDeleteResponse'];

// Graph Visualization Types
type GraphDataResponse = components['schemas']['GraphDataResponse'];

// Request types are used inline for simplicity

// Operation Response Types - Extract specific response shapes
// Custom type for ListNodes with pagination metadata
type ListNodesResponse = {
    nodes?: Node[];
    total?: number;
    hasMore?: boolean;
    nextToken?: string;
};
type ListCategoriesResponse = operations['listCategories']['responses']['200']['content']['application/json'];
type GetNodesInCategoryResponse = operations['getNodesInCategory']['responses']['200']['content']['application/json'];

/**
 * ApiClient class - Centralized HTTP communication
 * Handles all API requests with authentication and error handling
 */
class ApiClient {
    /**
     * Generic HTTP request method with authentication and error handling
     * @param method HTTP verb (GET, POST, PUT, DELETE)
     * @param path API endpoint path
     * @param body Request payload for POST/PUT operations
     * @param retryCount Number of retries attempted (internal use)
     * @returns Promise resolving to typed response data
     */
    private async request<T>(method: string, path: string, body: unknown = null, retryCount = 0): Promise<T> {
        // Get authentication token
        const token = await auth.getJwtToken();

        if (!token) {
            if (isDevMode) {
                console.error('API request failed: No valid authentication token available');
            }
            
            // Check if user has a session at all
            const session = await auth.getSession();
            if (!session) {
                throw new Error('Authentication required - please sign in to continue');
            } else {
                throw new Error('Authentication token expired - please refresh the page or sign in again');
            }
        }

        // Configure request with authentication headers
        const options: RequestInit = {
            method,
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json',
            },
        };

        // Add request body if provided
        if (body) {
            options.body = JSON.stringify(body);
        }

        try {            
            // Execute HTTP request
            const response = await fetch(`${API_BASE_URL}${path}`, options);
            
            // Check response status
            if (!response.ok) {
                const errorText = await response.text();
                let errorData;
                try {
                    errorData = JSON.parse(errorText);
                } catch {
                    errorData = { error: errorText || 'Request failed' };
                }
                
                // Only log detailed errors in development mode
                if (isDevMode) {
                    console.error('API request failed:', {
                        status: response.status,
                        path,
                        error: errorData.error || errorText,
                        retryCount,
                        coldStart: response.headers.get('X-Cold-Start'),
                        coldStartAge: response.headers.get('X-Cold-Start-Age')
                    });
                }

                // Handle authentication errors specifically
                if (response.status === 401) {
                    throw new ApiError('Authentication failed - your session has expired. Please sign in again.', 401);
                }

                if (response.status === 403) {
                    throw new ApiError('Access denied - you do not have permission to perform this action.', 403);
                }

                // Check if this is a retryable error (503 Service Unavailable or 500 Internal Server Error)
                const isRetryable = response.status === 503 || response.status === 500;
                const isColdStartError = response.headers.get('X-Cold-Start') === 'true';
                
                // Be more patient with cold start errors
                const maxRetries = isColdStartError ? 4 : (method === 'GET' ? 3 : 1);
                
                if (isRetryable && retryCount < maxRetries) {
                    // Use longer delays for cold start errors
                    const baseDelay = isColdStartError ? 2000 : 1000;
                    const maxDelay = isColdStartError ? 8000 : 5000;
                    const retryDelay = Math.min(baseDelay * Math.pow(2, retryCount), maxDelay);
                    
                    const retryReason = isColdStartError ? 'cold start detected' : 'service unavailable';
                    if (isDevMode) {
                        console.log(`Retrying request in ${retryDelay}ms (attempt ${retryCount + 1}/${maxRetries + 1}) - ${retryReason}`);
                    }
                    
                    await new Promise(resolve => setTimeout(resolve, retryDelay));
                    return this.request<T>(method, path, body, retryCount + 1);
                }
                
                throw new ApiError(errorData.error || `HTTP error! status: ${response.status}`, response.status);
            }
            
            // Log cold start information for successful requests in development
            if (isDevMode) {
                const coldStart = response.headers.get('X-Cold-Start');
                const coldStartAge = response.headers.get('X-Cold-Start-Age');
                if (coldStart === 'true') {
                    console.log(`Request served after cold start: ${path} (cold start age: ${coldStartAge})`);
                }
            }
            
            // Handle 204 No Content responses (common for DELETE operations)
            if (response.status === 204) {
                return null as T;
            }
            
            // Parse and return response
            const data = await response.json() as T;
            return data;
        } catch (error) {
            // Handle network errors (timeouts, connection issues)
            const errorMessage = (error as Error).message;
            const isNetworkError = errorMessage.includes('fetch') || 
                                 errorMessage.includes('timeout') || 
                                 errorMessage.includes('network') ||
                                 errorMessage.includes('Failed to fetch');
            
            if (isNetworkError && retryCount < 2 && method === 'GET') {
                const retryDelay = Math.min(1000 * Math.pow(2, retryCount), 3000);
                if (isDevMode) {
                    console.log(`Network error, retrying in ${retryDelay}ms (attempt ${retryCount + 1}/3): ${errorMessage}`);
                }
                
                await new Promise(resolve => setTimeout(resolve, retryDelay));
                return this.request<T>(method, path, body, retryCount + 1);
            }
            
            // Only log network errors in development mode
            if (isDevMode) {
                console.error('API request error:', errorMessage);
            }
            throw new ApiError(errorMessage, 0);
        }
    }

    // Public API methods for memory management operations

    /**
     * Create a new memory node
     * @param content The text content of the memory
     * @param tags Optional array of tags for the memory
     * @param title Optional title for the memory
     * @returns Promise resolving to the created Node
     */
    public async createNode(content: string, tags?: string[], title?: string): Promise<Node> {
        const body: { content: string; tags?: string[]; title: string } = { 
            content,
            // Generate title from content if not provided or empty
            title: (title && title.trim()) || content.substring(0, 50).trim() || 'Untitled'
        };
        if (tags && tags.length > 0) body.tags = tags;
        return this.request<Node>('POST', '/api/v1/nodes', body);
    }

    /**
     * Create an edge between two nodes
     * @param sourceId Source node ID
     * @param targetId Target node ID
     * @param type Edge type (optional, defaults to 'similar')
     * @param weight Edge weight (optional, defaults to 1.0)
     * @returns Promise resolving to created edge
     */
    public async createEdge(sourceId: string, targetId: string, type?: string, weight?: number): Promise<any> {
        const body = {
            source_id: sourceId,
            target_id: targetId,
            type: type || 'similar',
            weight: weight ?? 1.0
        };
        return this.request<any>('POST', '/api/v1/edges', body);
    }

    /**
     * Delete an edge between two nodes
     * @param edgeId Edge ID to delete
     * @returns Promise resolving when edge is deleted
     */
    public async deleteEdge(edgeId: string): Promise<void> {
        return this.request<void>('DELETE', `/api/v1/edges/${edgeId}`);
    }

    /**
     * List user's memory nodes with pagination
     * @param limit Number of nodes per page (default: 20, max: 100)
     * @param nextToken Token for next page (for pagination)
     * @returns Promise resolving to paginated nodes response
     */
    public async listNodes(limit?: number, nextToken?: string): Promise<ListNodesResponse> {
        const params = new URLSearchParams();
        if (limit) params.append('limit', limit.toString());
        if (nextToken) params.append('nextToken', nextToken);
        
        const queryString = params.toString();
        const path = queryString ? `/api/v1/nodes?${queryString}` : '/api/v1/nodes';
        
        const response = await this.request<any>('GET', path);
        
        // Map backend response format to frontend expected format
        if (response.nodes) {
            response.nodes = response.nodes.map((node: any) => ({
                nodeId: node.id || node.nodeId,
                content: node.content || '',
                title: node.title,
                tags: node.tags || [],
                timestamp: node.createdAt || node.timestamp || new Date().toISOString(),
                version: node.version || 0
            }));
        }
        
        return response as ListNodesResponse;
    }

    /**
     * Get detailed information about a specific memory node
     * @param nodeId Unique identifier of the memory node
     * @returns Promise resolving to NodeDetails with content and connections
     */
    public getNode(nodeId: string): Promise<NodeDetails> {
        return this.request<NodeDetails>('GET', `/api/v1/nodes/${nodeId}`);
    }

    /**
     * Delete a memory node permanently
     * @param nodeId Unique identifier of the memory node to delete
     * @returns Promise resolving when deletion is complete
     */
    public deleteNode(nodeId: string): Promise<void> {
        return this.request<void>('DELETE', `/api/v1/nodes/${nodeId}`);
    }

    /**
     * Get complete graph visualization data
     * @returns Promise resolving to graph data with nodes and edges
     */
    public getGraphData(): Promise<GraphDataResponse> {
        return this.request<GraphDataResponse>('GET', '/api/v1/graph-data');
    }

    /**
     * Update memory node content and tags
     * @param nodeId Unique identifier of the memory node to update
     * @param content New text content for the memory
     * @param tags Optional array of tags for the memory
     * @param title Optional title for the memory
     * @returns Promise resolving to success message
     */
    public updateNode(nodeId: string, content: string, tags?: string[], title?: string): Promise<{ message: string }> {
        const body: { content: string; tags?: string[]; title?: string } = { content };
        if (tags && tags.length > 0) body.tags = tags;
        if (title !== undefined) body.title = title.trim();
        return this.request<{ message: string }>('PUT', `/api/v1/nodes/${nodeId}`, body);
    }

    /**
     * Delete multiple memory nodes in a single operation
     * @param nodeIds Array of node IDs to delete
     * @returns Promise resolving to deletion results
     */
    public bulkDeleteNodes(nodeIds: string[]): Promise<BulkDeleteResponse> {
        return this.request<BulkDeleteResponse>('POST', '/api/v1/nodes/bulk-delete', { node_ids: nodeIds });
    }

    // Category management operations

    public createCategory(title: string, description?: string): Promise<Category> {
        const payload: { title: string; description?: string } = {
            title: title.trim(),
        };

        if (description?.trim()) {
            payload.description = description.trim();
        }

        return this.request<Category>('POST', '/api/v1/categories', payload);
    }

    public async listCategories(): Promise<ListCategoriesResponse> {
        try {
            return await this.request<ListCategoriesResponse>('GET', '/api/v1/categories');
        } catch (error) {
            if (error instanceof ApiError && error.status === 404) {
                if (isDevMode) {
                    console.info('Categories endpoint unavailable. Returning empty result set.');
                }
                return { categories: [], total: 0 } as ListCategoriesResponse;
            }
            throw error;
        }
    }

    public getCategory(categoryId: string): Promise<Category> {
        return this.request<Category>('GET', `/api/v1/categories/${categoryId}`);
    }

    public updateCategory(categoryId: string, title: string, description?: string): Promise<{ message: string; categoryId: string }> {
        const payload: { title: string; description?: string } = {
            title: title.trim(),
        };

        if (description?.trim()) {
            payload.description = description.trim();
        }

        return this.request<{ message: string; categoryId: string }>('PUT', `/api/v1/categories/${categoryId}`, payload);
    }

    public deleteCategory(categoryId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('DELETE', `/api/v1/categories/${categoryId}`);
    }

    public getNodesInCategory(categoryId: string): Promise<GetNodesInCategoryResponse> {
        return this.request<GetNodesInCategoryResponse>('GET', `/api/v1/categories/${categoryId}/nodes`);
    }

    public assignNodeToCategory(categoryId: string, nodeId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('POST', `/api/v1/categories/${categoryId}/nodes`, { nodeId });
    }

    public removeNodeFromCategory(categoryId: string, nodeId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('DELETE', `/api/v1/categories/${categoryId}/nodes/${nodeId}`);
    }

    // Enhanced category operations

    /**
     * Get hierarchical category tree
     * @returns Promise resolving to category hierarchy with parent-child relationships
     * @stub Categories not yet implemented in backend
     */
    public async getCategoryHierarchy(): Promise<operations['getCategoryHierarchy']['responses']['200']['content']['application/json']> {
        try {
            return await this.request<operations['getCategoryHierarchy']['responses']['200']['content']['application/json']>('GET', '/api/v1/categories/hierarchy');
        } catch (error) {
            if (error instanceof ApiError && error.status === 404) {
                if (isDevMode) {
                    console.info('Category hierarchy endpoint unavailable. Returning empty hierarchy.');
                }
                return {
                    categories: [],
                    rootCategories: [],
                    totalCategories: 0,
                } as operations['getCategoryHierarchy']['responses']['200']['content']['application/json'];
            }
            throw error;
        }
    }

    /**
     * Get AI-powered category suggestions for content
     * @param content The content to analyze for category suggestions
     * @returns Promise resolving to array of category suggestions with confidence scores
     * @stub Categories not yet implemented in backend
     */
    public suggestCategories(content: string): Promise<operations['suggestCategories']['responses']['200']['content']['application/json']> {
        return this.request('POST', '/api/v1/categories/suggest', { content });
    }

    /**
     * Rebuild and optimize category structure
     * @returns Promise resolving to rebuild results and statistics
     */
    public rebuildCategories(): Promise<operations['rebuildCategories']['responses']['200']['content']['application/json']> {
        return this.request('POST', '/api/v1/categories/rebuild');
    }

    /**
     * Get category usage insights and analytics
     * @returns Promise resolving to comprehensive category insights
     */
    public getCategoryInsights(): Promise<any> {
        return this.request('GET', '/api/v1/categories/insights');
    }

    /**
     * Test API health endpoint (no authentication required)
     * @returns Promise resolving to health status
     */
    public async testHealth(): Promise<{ message: string }> {
        try {
            const response = await fetch(`${API_BASE_URL}/health`);

            if (!response.ok) {
                const errorText = await response.text();
                if (isDevMode) {
                    console.error('Health check failed:', errorText);
                }
                throw new ApiError(`Health check failed: ${response.status}`, response.status);
            }

            return await response.json();
        } catch (error) {
            const message = (error as Error).message || 'Health check failed';
            if (isDevMode) {
                console.error('Health check error:', message);
            }
            throw new ApiError(message, 0);
        }
    }

    /**
     * Debug authentication and API connectivity
     * @returns Promise resolving to debug information
     */
    public async debugAuth(): Promise<void> {
        if (!isDevMode) {
            return;
        }

        console.debug('Starting authentication debug...');
        
        // Test 1: Check if we have a session
        const session = await auth.getSession();
        console.debug('Session check:', {
            hasSession: !!session,
            hasExpiration: !!session?.expires_at
        });
        
        // Test 2: Test health endpoint (no auth)
        try {
            await this.testHealth();
            console.debug('Health endpoint working');
        } catch (error) {
            console.error('Health endpoint failed:', (error as Error).message);
        }
        
        // Test 3: Test JWT token retrieval
        try {
            const token = await auth.getJwtToken();
            console.debug('JWT token check:', {
                hasToken: !!token
            });
        } catch (error) {
            console.error('JWT token error:', (error as Error).message);
        }
    }

    /**
     * Get all categories assigned to a node
     * @param nodeId Unique identifier of the memory node
     * @returns Promise resolving to array of categories assigned to the node
     */
    public getNodeCategories(nodeId: string): Promise<operations['getNodeCategories']['responses']['200']['content']['application/json']> {
        return this.request('GET', `/api/v1/nodes/${nodeId}/categories`);
    }

    /**
     * Auto-categorize a node using AI
     * @param nodeId Unique identifier of the memory node to categorize
     * @returns Promise resolving to array of assigned categories
     */
    public categorizeNode(nodeId: string): Promise<operations['categorizeNode']['responses']['200']['content']['application/json']> {
        return this.request('POST', `/api/v1/nodes/${nodeId}/categories`);
    }
}

// Singleton API client instance for use throughout the application
export const api = new ApiClient();

export { ApiError };

// Add global debug trigger for development
if (typeof window !== 'undefined' && isDevMode) {
    (window as any).debugAuth = () => api.debugAuth();
    (window as any).testHealth = () => api.testHealth();
}
