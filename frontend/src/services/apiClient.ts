/**
 * API Client - Type-Safe HTTP Communication Layer
 * 
 * Provides type-safe HTTP communication with the backend API using
 * generated TypeScript types from OpenAPI specification.
 * Handles authentication, error handling, and request/response processing.
 * 
 * Configuration Notes:
 * - Server endpoint: Controlled by useLocalServer flag (which backend to connect to)
 * - Development logging: Controlled by import.meta.env.MODE (when to show detailed logs)
 * - These are independent: you can develop locally with production API + dev logging
 */

import { auth } from './authClient';
import { components, operations } from '../types/generated/generated-types';

// Dynamic API Configuration
function getApiBaseUrl(): string {
    // Server endpoint selection - controls which backend API to use
    // Note: This is separate from development mode logging (controlled by import.meta.env.MODE)
    
    // Always use production API for now since local backend isn't running
    // Set to false to always use the production API
    const useLocalServer = false;
    
    // Can override to force production API even in dev:
    // Add VITE_FORCE_PRODUCTION_API=true to .env.local to use production API

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
            // Only log in development mode
            if (import.meta.env.MODE === 'development') {
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
                if (import.meta.env.MODE === 'development') {
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
                    throw new Error('Authentication failed - your session has expired. Please sign in again.');
                }
                
                if (response.status === 403) {
                    throw new Error('Access denied - you do not have permission to perform this action.');
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
                    if (import.meta.env.MODE === 'development') {
                        console.log(`Retrying request in ${retryDelay}ms (attempt ${retryCount + 1}/${maxRetries + 1}) - ${retryReason}`);
                    }
                    
                    await new Promise(resolve => setTimeout(resolve, retryDelay));
                    return this.request<T>(method, path, body, retryCount + 1);
                }
                
                throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
            }
            
            // Log cold start information for successful requests in development
            if (import.meta.env.MODE === 'development') {
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
                if (import.meta.env.MODE === 'development') {
                    console.log(`Network error, retrying in ${retryDelay}ms (attempt ${retryCount + 1}/3): ${errorMessage}`);
                }
                
                await new Promise(resolve => setTimeout(resolve, retryDelay));
                return this.request<T>(method, path, body, retryCount + 1);
            }
            
            // Only log network errors in development mode
            if (import.meta.env.MODE === 'development') {
                console.error('API request error:', errorMessage);
            }
            throw error;
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
        console.log('DEBUG apiClient.createNode - title:', JSON.stringify(title));
        const body: { content: string; tags?: string[]; title: string } = { 
            content,
            // Generate title from content if not provided or empty
            title: (title && title.trim()) || content.substring(0, 50).trim() || 'Untitled'
        };
        if (tags && tags.length > 0) body.tags = tags;
        console.log('DEBUG apiClient.createNode - body:', JSON.stringify(body));
        return this.request<Node>('POST', '/api/v2/nodes', body);
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
        const path = queryString ? `/api/v2/nodes?${queryString}` : '/api/v2/nodes';
        
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
        return this.request<NodeDetails>('GET', `/api/v2/nodes/${nodeId}`);
    }

    /**
     * Delete a memory node permanently
     * @param nodeId Unique identifier of the memory node to delete
     * @returns Promise resolving when deletion is complete
     */
    public deleteNode(nodeId: string): Promise<void> {
        return this.request<void>('DELETE', `/api/v2/nodes/${nodeId}`);
    }

    /**
     * Get complete graph visualization data
     * @returns Promise resolving to graph data with nodes and edges
     */
    public getGraphData(): Promise<GraphDataResponse> {
        return this.request<GraphDataResponse>('GET', '/api/v2/graph-data');
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
        return this.request<{ message: string }>('PUT', `/api/v2/nodes/${nodeId}`, body);
    }

    /**
     * Delete multiple memory nodes in a single operation
     * @param nodeIds Array of node IDs to delete
     * @returns Promise resolving to deletion results
     */
    public bulkDeleteNodes(nodeIds: string[]): Promise<BulkDeleteResponse> {
        return this.request<BulkDeleteResponse>('POST', '/api/v2/nodes/bulk-delete', { node_ids: nodeIds });
    }

    // Category management operations
    // NOTE: Categories are not yet implemented in backend2 - commented out for now

    /**
     * Create a new category
     * @param title The title of the category
     * @param description Optional description of the category
     * @returns Promise resolving to the created Category
     * @stub Categories not yet implemented in backend2
     */
    public createCategory(title: string, description?: string): Promise<Category> {
        console.warn('Categories not yet implemented in backend2 - returning stub data');
        // Return mock category for now
        return Promise.resolve({
            id: `cat-${Date.now()}`,
            title,
            description: description || '',
            level: 0,
            parentId: undefined,
            color: undefined,
            icon: undefined,
            aiGenerated: false,
            noteCount: 0,
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString()
        });
    }

    /**
     * List all user's categories
     * @returns Promise resolving to array of categories
     * @stub Categories not yet implemented in backend2
     */
    public listCategories(): Promise<ListCategoriesResponse> {
        console.warn('Categories not yet implemented in backend2 - returning empty list');
        // Return empty categories list for now
        return Promise.resolve({
            categories: [],
            total: 0
        } as ListCategoriesResponse);
    }

    /**
     * Get detailed information about a specific category
     * @param categoryId Unique identifier of the category
     * @returns Promise resolving to Category with details
     * @stub Categories not yet implemented in backend2
     */
    public getCategory(categoryId: string): Promise<Category> {
        console.warn('Categories not yet implemented in backend2 - returning stub data');
        // Return mock category for now
        return Promise.resolve({
            id: categoryId,
            title: 'Stub Category',
            description: 'Categories coming soon',
            level: 0,
            parentId: undefined,
            color: undefined,
            icon: undefined,
            aiGenerated: false,
            noteCount: 0,
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString()
        });
    }

    /**
     * Update a category's details
     * @param categoryId Unique identifier of the category to update
     * @param title New title for the category
     * @param description New description for the category
     * @returns Promise resolving to success message
     * @stub Categories not yet implemented in backend2
     */
    public updateCategory(categoryId: string, title: string, description?: string): Promise<{ message: string; categoryId: string }> {
        console.warn('Categories not yet implemented in backend2 - returning stub response');
        return Promise.resolve({
            message: 'Category update stubbed',
            categoryId
        });
    }

    /**
     * Delete a category permanently
     * @param categoryId Unique identifier of the category to delete
     * @returns Promise resolving to success message
     * @stub Categories not yet implemented in backend2
     */
    public deleteCategory(categoryId: string): Promise<{ message: string }> {
        console.warn('Categories not yet implemented in backend2 - returning stub response');
        return Promise.resolve({ message: 'Category deletion stubbed' });
    }

    /**
     * Get all nodes in a specific category
     * @param categoryId Unique identifier of the category
     * @returns Promise resolving to array of nodes in the category
     * @stub Categories not yet implemented in backend2
     */
    public getNodesInCategory(categoryId: string): Promise<GetNodesInCategoryResponse> {
        console.warn('Categories not yet implemented in backend2 - returning empty nodes');
        return Promise.resolve({
            nodes: [],
            categoryId,
            total: 0
        } as GetNodesInCategoryResponse);
    }

    /**
     * Assign a node to a category
     * @param categoryId Unique identifier of the category
     * @param nodeId Unique identifier of the node to assign
     * @returns Promise resolving to success message
     * @stub Categories not yet implemented in backend2
     */
    public assignNodeToCategory(categoryId: string, nodeId: string): Promise<{ message: string }> {
        console.warn('Categories not yet implemented in backend2 - returning stub response');
        return Promise.resolve({ message: 'Node assignment stubbed' });
    }

    /**
     * Remove a node from a category
     * @param categoryId Unique identifier of the category
     * @param nodeId Unique identifier of the node to remove
     * @returns Promise resolving to success message
     * @stub Categories not yet implemented in backend2
     */
    public removeNodeFromCategory(categoryId: string, nodeId: string): Promise<{ message: string }> {
        console.warn('Categories not yet implemented in backend2 - returning stub response');
        return Promise.resolve({ message: 'Node removal stubbed' });
    }

    // Enhanced category operations

    /**
     * Get hierarchical category tree
     * @returns Promise resolving to category hierarchy with parent-child relationships
     * @stub Categories not yet implemented in backend2
     */
    public getCategoryHierarchy(): Promise<operations['getCategoryHierarchy']['responses']['200']['content']['application/json']> {
        console.warn('Categories not yet implemented in backend2 - returning empty hierarchy');
        return Promise.resolve({
            categories: [],
            rootCategories: [],
            totalCategories: 0
        } as any);
    }

    /**
     * Get AI-powered category suggestions for content
     * @param content The content to analyze for category suggestions
     * @returns Promise resolving to array of category suggestions with confidence scores
     * @stub Categories not yet implemented in backend2
     */
    public suggestCategories(content: string): Promise<operations['suggestCategories']['responses']['200']['content']['application/json']> {
        console.warn('Categories not yet implemented in backend2 - returning empty suggestions');
        return Promise.resolve({
            suggestions: [],
            confidence: 0
        } as any);
    }

    /**
     * Rebuild and optimize category structure
     * @returns Promise resolving to rebuild results and statistics
     */
    public rebuildCategories(): Promise<operations['rebuildCategories']['responses']['200']['content']['application/json']> {
        return this.request('POST', '/api/v2/categories/rebuild');
    }

    /**
     * Get category usage insights and analytics
     * @returns Promise resolving to comprehensive category insights
     */
    public getCategoryInsights(): Promise<any> {
        console.warn('Categories not yet implemented in backend2 - returning stub data');
        // Return mock insights for now
        return Promise.resolve({
            totalCategories: 0,
            totalNodes: 0,
            categorizedNodes: 0,
            uncategorizedNodes: 0,
            categoryUsage: [],
            recentActivity: []
        });
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
                console.error('Health check failed:', errorText);
                throw new Error(`Health check failed: ${response.status} - ${errorText}`);
            }
            
            const data = await response.json();
            return data;
        } catch (error) {
            console.error('Health check error:', error);
            throw error;
        }
    }

    /**
     * Debug authentication and API connectivity
     * @returns Promise resolving to debug information
     */
    public async debugAuth(): Promise<void> {
        console.log('Starting authentication debug...');
        
        // Test 1: Check if we have a session
        const session = await auth.getSession();
        console.log('Session check:', {
            hasSession: !!session,
            hasExpiration: !!session?.expires_at
        });
        
        // Test 2: Test health endpoint (no auth)
        try {
            await this.testHealth();
            console.log('Health endpoint working');
        } catch (error) {
            console.error('Health endpoint failed:', (error as Error).message);
        }
        
        // Test 3: Test JWT token retrieval
        try {
            const token = await auth.getJwtToken();
            console.log('JWT token check:', {
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
        return this.request('GET', `/api/v2/nodes/${nodeId}/categories`);
    }

    /**
     * Auto-categorize a node using AI
     * @param nodeId Unique identifier of the memory node to categorize
     * @returns Promise resolving to array of assigned categories
     */
    public categorizeNode(nodeId: string): Promise<operations['categorizeNode']['responses']['200']['content']['application/json']> {
        return this.request('POST', `/api/v2/nodes/${nodeId}/categories`);
    }
}

// Singleton API client instance for use throughout the application
export const api = new ApiClient();

// Add global debug trigger for development
if (typeof window !== 'undefined') {
    (window as any).debugAuth = () => api.debugAuth();
    (window as any).testHealth = () => api.testHealth();
}
