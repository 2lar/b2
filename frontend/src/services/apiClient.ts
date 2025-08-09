/**
 * API Client - Type-Safe HTTP Communication Layer
 * 
 * Provides type-safe HTTP communication with the backend API using
 * generated TypeScript types from OpenAPI specification.
 * Handles authentication, error handling, and request/response processing.
 */

import { auth } from './authClient';
import { components, operations } from '../types/generated/generated-types';

// Dynamic API Configuration
function getApiBaseUrl(): string {
    // Environment detection for automatic URL selection
    
    const isLocal = false;
    
    // const isLocal = import.meta.env.DEV || 
    //                window.location.hostname === 'localhost' || 
    //                window.location.hostname === '127.0.0.1' ||
    //                window.location.hostname.includes('local');

    if (isLocal) {
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
type CreateCategoryRequest = components['schemas']['CreateCategoryRequest'];
type UpdateCategoryRequest = components['schemas']['UpdateCategoryRequest'];
type AssignNodeToCategoryRequest = components['schemas']['AssignNodeToCategoryRequest'];

// Request/Response Types for Bulk Operations
type BulkDeleteRequest = components['schemas']['BulkDeleteRequest'];
type BulkDeleteResponse = components['schemas']['BulkDeleteResponse'];

// Graph Visualization Types
type GraphDataResponse = components['schemas']['GraphDataResponse'];

// Request types are used inline for simplicity

// Operation Response Types - Extract specific response shapes
type ListNodesResponse = operations['listNodes']['responses']['200']['content']['application/json'];
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
     * @returns Promise resolving to typed response data
     */
    private async request<T>(method: string, path: string, body: unknown = null): Promise<T> {
        // Get authentication token
        const token = await auth.getJwtToken();

        if (!token) {
            throw new Error('Not authenticated - please sign in to continue');
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
                
                console.error('API request failed:', {
                    status: response.status,
                    path,
                    error: errorData.error
                });
                
                throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
            }
            
            // Parse and return response
            const data = await response.json() as T;
            return data;
        } catch (error) {
            console.error('API request error:', (error as Error).message);
            throw error;
        }
    }

    // Public API methods for memory management operations

    /**
     * Create a new memory node
     * @param content The text content of the memory
     * @param tags Optional array of tags for the memory
     * @returns Promise resolving to the created Node
     */
    public createNode(content: string, tags?: string[]): Promise<Node> {
        return this.request<Node>('POST', '/api/nodes', { content, tags });
    }

    /**
     * List all user's memory nodes
     * @returns Promise resolving to array of memory nodes
     */
    public listNodes(): Promise<ListNodesResponse> {
        return this.request<ListNodesResponse>('GET', '/api/nodes');
    }

    /**
     * Get detailed information about a specific memory node
     * @param nodeId Unique identifier of the memory node
     * @returns Promise resolving to NodeDetails with content and connections
     */
    public getNode(nodeId: string): Promise<NodeDetails> {
        return this.request<NodeDetails>('GET', `/api/nodes/${nodeId}`);
    }

    /**
     * Delete a memory node permanently
     * @param nodeId Unique identifier of the memory node to delete
     * @returns Promise resolving to success message
     */
    public deleteNode(nodeId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('DELETE', `/api/nodes/${nodeId}`);
    }

    /**
     * Get complete graph visualization data
     * @returns Promise resolving to graph data with nodes and edges
     */
    public getGraphData(): Promise<GraphDataResponse> {
        return this.request<GraphDataResponse>('GET', '/api/graph-data');
    }

    /**
     * Update memory node content and tags
     * @param nodeId Unique identifier of the memory node to update
     * @param content New text content for the memory
     * @param tags Optional array of tags for the memory
     * @returns Promise resolving to success message
     */
    public updateNode(nodeId: string, content: string, tags?: string[]): Promise<{ message: string }> {
        return this.request<{ message: string }>('PUT', `/api/nodes/${nodeId}`, { content, tags });
    }

    /**
     * Delete multiple memory nodes in a single operation
     * @param nodeIds Array of node IDs to delete
     * @returns Promise resolving to deletion results
     */
    public bulkDeleteNodes(nodeIds: string[]): Promise<BulkDeleteResponse> {
        return this.request<BulkDeleteResponse>('POST', '/api/nodes/bulk-delete', { nodeIds });
    }

    // Category management operations

    /**
     * Create a new category
     * @param title The title of the category
     * @param description Optional description of the category
     * @returns Promise resolving to the created Category
     */
    public createCategory(title: string, description?: string): Promise<Category> {
        return this.request<Category>('POST', '/api/categories', { title, description });
    }

    /**
     * List all user's categories
     * @returns Promise resolving to array of categories
     */
    public listCategories(): Promise<ListCategoriesResponse> {
        return this.request<ListCategoriesResponse>('GET', '/api/categories');
    }

    /**
     * Get detailed information about a specific category
     * @param categoryId Unique identifier of the category
     * @returns Promise resolving to Category with details
     */
    public getCategory(categoryId: string): Promise<Category> {
        return this.request<Category>('GET', `/api/categories/${categoryId}`);
    }

    /**
     * Update a category's details
     * @param categoryId Unique identifier of the category to update
     * @param title New title for the category
     * @param description New description for the category
     * @returns Promise resolving to success message
     */
    public updateCategory(categoryId: string, title: string, description?: string): Promise<{ message: string; categoryId: string }> {
        return this.request<{ message: string; categoryId: string }>('PUT', `/api/categories/${categoryId}`, { title, description });
    }

    /**
     * Delete a category permanently
     * @param categoryId Unique identifier of the category to delete
     * @returns Promise resolving to success message
     */
    public deleteCategory(categoryId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('DELETE', `/api/categories/${categoryId}`);
    }

    /**
     * Get all nodes in a specific category
     * @param categoryId Unique identifier of the category
     * @returns Promise resolving to array of nodes in the category
     */
    public getNodesInCategory(categoryId: string): Promise<GetNodesInCategoryResponse> {
        return this.request<GetNodesInCategoryResponse>('GET', `/api/categories/${categoryId}/nodes`);
    }

    /**
     * Assign a node to a category
     * @param categoryId Unique identifier of the category
     * @param nodeId Unique identifier of the node to assign
     * @returns Promise resolving to success message
     */
    public assignNodeToCategory(categoryId: string, nodeId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('POST', `/api/categories/${categoryId}/nodes`, { nodeId });
    }

    /**
     * Remove a node from a category
     * @param categoryId Unique identifier of the category
     * @param nodeId Unique identifier of the node to remove
     * @returns Promise resolving to success message
     */
    public removeNodeFromCategory(categoryId: string, nodeId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('DELETE', `/api/categories/${categoryId}/nodes/${nodeId}`);
    }

    // Enhanced category operations

    /**
     * Get hierarchical category tree
     * @returns Promise resolving to category hierarchy with parent-child relationships
     */
    public getCategoryHierarchy(): Promise<operations['getCategoryHierarchy']['responses']['200']['content']['application/json']> {
        return this.request('GET', '/api/categories/hierarchy');
    }

    /**
     * Get AI-powered category suggestions for content
     * @param content The content to analyze for category suggestions
     * @returns Promise resolving to array of category suggestions with confidence scores
     */
    public suggestCategories(content: string): Promise<operations['suggestCategories']['responses']['200']['content']['application/json']> {
        return this.request('POST', '/api/categories/suggest', { content });
    }

    /**
     * Rebuild and optimize category structure
     * @returns Promise resolving to rebuild results and statistics
     */
    public rebuildCategories(): Promise<operations['rebuildCategories']['responses']['200']['content']['application/json']> {
        return this.request('POST', '/api/categories/rebuild');
    }

    /**
     * Get category usage insights and analytics
     * @returns Promise resolving to comprehensive category insights
     */
    public getCategoryInsights(): Promise<operations['getCategoryInsights']['responses']['200']['content']['application/json']> {
        return this.request('GET', '/api/categories/insights');
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
        return this.request('GET', `/api/nodes/${nodeId}/categories`);
    }

    /**
     * Auto-categorize a node using AI
     * @param nodeId Unique identifier of the memory node to categorize
     * @returns Promise resolving to array of assigned categories
     */
    public categorizeNode(nodeId: string): Promise<operations['categorizeNode']['responses']['200']['content']['application/json']> {
        return this.request('POST', `/api/nodes/${nodeId}/categories`);
    }
}

// Singleton API client instance for use throughout the application
export const api = new ApiClient();

// Add global debug trigger for development
if (typeof window !== 'undefined') {
    (window as any).debugAuth = () => api.debugAuth();
    (window as any).testHealth = () => api.testHealth();
}
