import { auth } from './authClient';
import { components, operations } from './generated-types';

// const API_BASE_URL = 'YOUR_API_GATEWAY_URL'; // Replace with your actual API Gateway URL
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL;

// Validate that the API URL is set
if (!API_BASE_URL || API_BASE_URL === 'undefined') {
    console.error('VITE_API_BASE_URL is not defined. Please check your .env file.');
    // Don't throw here since we might want to show the auth page at least
}

// Type aliases for easier usage
type Node = components['schemas']['Node'];
type NodeDetails = components['schemas']['NodeDetails'];
type BulkDeleteRequest = components['schemas']['BulkDeleteRequest'];
type BulkDeleteResponse = components['schemas']['BulkDeleteResponse'];
type GraphDataResponse = components['schemas']['GraphDataResponse'];
// type CreateNodeRequest = components['schemas']['CreateNodeRequest'];
// type UpdateNodeRequest = components['schemas']['UpdateNodeRequest'];

// Response types from operations
type ListNodesResponse = operations['listNodes']['responses']['200']['content']['application/json'];

// API client class
class ApiClient {
    private async request<T>(method: string, path: string, body: unknown = null): Promise<T> {
        const token = await auth.getJwtToken();
        if (!token) {
            throw new Error('Not authenticated');
        }

        const options: RequestInit = {
            method,
            headers: {
                'Authorization': `Bearer ${token}`,
                'Content-Type': 'application/json',
            },
        };

        if (body) {
            options.body = JSON.stringify(body);
        }

        try {
            const response = await fetch(`${API_BASE_URL}${path}`, options);
            
            if (!response.ok) {
                const errorData = await response.json().catch(() => ({ error: 'Request failed' }));
                throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
            }
            
            return await response.json() as T;
        } catch (error) {
            console.error('API request failed:', method, path, error);
            throw error;
        }
    }

    // Node operations with strong typing
    public createNode(content: string): Promise<Node> {
        return this.request<Node>('POST', '/api/nodes', { content });
    }

    public listNodes(): Promise<ListNodesResponse> {
        return this.request<ListNodesResponse>('GET', '/api/nodes');
    }

    public getNode(nodeId: string): Promise<NodeDetails> {
        return this.request<NodeDetails>('GET', `/api/nodes/${nodeId}`);
    }

    public deleteNode(nodeId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('DELETE', `/api/nodes/${nodeId}`);
    }

    public getGraphData(): Promise<GraphDataResponse> {
        return this.request<GraphDataResponse>('GET', '/api/graph-data');
    }

    public updateNode(nodeId: string, content: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('PUT', `/api/nodes/${nodeId}`, { content });
    }

    public bulkDeleteNodes(nodeIds: string[]): Promise<BulkDeleteResponse> {
        return this.request<BulkDeleteResponse>('POST', '/api/nodes/bulk-delete', { nodeIds });
    }
}

export const api = new ApiClient();
