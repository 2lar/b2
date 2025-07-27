/**
 * API Client - Type-Safe HTTP Communication Layer
 * 
 * Provides type-safe HTTP communication with the backend API using
 * generated TypeScript types from OpenAPI specification.
 * Handles authentication, error handling, and request/response processing.
 */

import { auth } from './authClient';
import { components, operations } from './generated-types';

// Dynamic API Configuration
function getApiBaseUrl(): string {
    // Environment detection for automatic URL selection
    const isLocal = import.meta.env.DEV || 
                   window.location.hostname === 'localhost' || 
                   window.location.hostname === '127.0.0.1' ||
                   window.location.hostname.includes('local');

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

// Request/Response Types for Bulk Operations
type BulkDeleteRequest = components['schemas']['BulkDeleteRequest'];
type BulkDeleteResponse = components['schemas']['BulkDeleteResponse'];

// Graph Visualization Types
type GraphDataResponse = components['schemas']['GraphDataResponse'];

// Request types are used inline for simplicity

// Operation Response Types - Extract specific response shapes
type ListNodesResponse = operations['listNodes']['responses']['200']['content']['application/json'];

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
            throw new Error('Not authenticated');
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
                const errorData = await response.json().catch(() => ({ error: 'Request failed' }));
                throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
            }
            
            // Parse and return response
            return await response.json() as T;
        } catch (error) {
            console.error('API request failed:', method, path, error);
            throw error;
        }
    }

    // Public API methods for memory management operations

    /**
     * Create a new memory node
     * @param content The text content of the memory
     * @returns Promise resolving to the created Node
     */
    public createNode(content: string): Promise<Node> {
        return this.request<Node>('POST', '/api/nodes', { content });
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
     * Update memory node content
     * @param nodeId Unique identifier of the memory node to update
     * @param content New text content for the memory
     * @returns Promise resolving to success message
     */
    public updateNode(nodeId: string, content: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('PUT', `/api/nodes/${nodeId}`, { content });
    }

    /**
     * Delete multiple memory nodes in a single operation
     * @param nodeIds Array of node IDs to delete
     * @returns Promise resolving to deletion results
     */
    public bulkDeleteNodes(nodeIds: string[]): Promise<BulkDeleteResponse> {
        return this.request<BulkDeleteResponse>('POST', '/api/nodes/bulk-delete', { nodeIds });
    }
}

// Singleton API client instance for use throughout the application
export const api = new ApiClient();
