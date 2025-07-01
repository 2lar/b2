/**
 * =============================================================================
 * API Client - Type-Safe HTTP Communication Layer
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This module demonstrates modern frontend-backend communication patterns using
 * type-safe HTTP clients with generated TypeScript types. It showcases how to
 * build robust, maintainable API layers that prevent runtime errors and improve
 * developer experience.
 * 
 * 🏗️ KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. TYPE SAFETY ACROSS THE STACK:
 *    - Uses generated types from OpenAPI specification
 *    - Compile-time error detection for API mismatches
 *    - IntelliSense support for all API operations
 *    - Prevents data structure inconsistencies
 * 
 * 2. AUTHENTICATION INTEGRATION:
 *    - Automatic JWT token injection for all requests
 *    - Seamless integration with Supabase auth client
 *    - Centralized authentication error handling
 *    - Token refresh logic handled transparently
 * 
 * 3. ERROR HANDLING STRATEGY:
 *    - Structured error responses from API
 *    - Graceful degradation for network failures
 *    - User-friendly error messages
 *    - Detailed logging for debugging
 * 
 * 4. CONFIGURATION MANAGEMENT:
 *    - Environment-based API endpoint configuration
 *    - Development vs production URL handling
 *    - Validation of required configuration
 *    - Runtime configuration checking
 * 
 * 5. SINGLE RESPONSIBILITY PRINCIPLE:
 *    - Pure HTTP communication concerns
 *    - No business logic or UI state management
 *    - Reusable across different UI components
 *    - Clear separation of concerns
 * 
 * 🔄 CODE GENERATION WORKFLOW:
 * 1. OpenAPI spec defines API contract (openapi.yaml)
 * 2. Type generator creates TypeScript interfaces (npm run generate-api-types)
 * 3. This client uses generated types for compile-time safety
 * 4. Changes to API automatically update frontend types
 * 5. Build fails if frontend code doesn't match API contract
 * 
 * 📡 HTTP CLIENT PATTERNS:
 * - Centralized request configuration
 * - Automatic authentication header injection
 * - Consistent error handling across all endpoints
 * - Type-safe request/response handling
 * - RESTful endpoint mapping
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - Type-safe API client architecture
 * - OpenAPI code generation workflows
 * - Modern authentication patterns
 * - Error handling best practices
 * - Configuration management techniques
 * - HTTP client design patterns
 */

import { auth } from './authClient';
import { components, operations } from './generated-types';

/**
 * API Base URL Configuration
 * 
 * ENVIRONMENT-BASED CONFIGURATION:
 * Uses Vite's environment variable system for flexible deployment.
 * Different environments (dev, staging, prod) can use different API endpoints.
 * 
 * CONFIGURATION PATTERNS:
 * - Development: Local API server or staging environment
 * - Production: AWS API Gateway endpoint
 * - Testing: Mock API server or test environment
 * 
 * SECURITY CONSIDERATIONS:
 * - Public environment variables (VITE_*) are safe for client-side code
 * - Sensitive data should never be in frontend environment variables
 * - API endpoint URLs are not sensitive and can be public
 */
// const API_BASE_URL = 'YOUR_API_GATEWAY_URL'; // Replace with your actual API Gateway URL
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL;

/**
 * Configuration Validation
 * 
 * FAIL-FAST PRINCIPLE:
 * Validate required configuration at startup rather than at runtime.
 * Provides clear error messages for configuration issues.
 * 
 * GRACEFUL DEGRADATION:
 * Don't throw errors that would crash the entire application.
 * Allow partial functionality (like authentication UI) even with missing config.
 * 
 * DEVELOPER EXPERIENCE:
 * Clear error messages guide developers to fix configuration issues.
 * Console warnings help during development and debugging.
 */
if (!API_BASE_URL || API_BASE_URL === 'undefined') {
    console.error('VITE_API_BASE_URL is not defined. Please check your .env file.');
    // Don't throw here since we might want to show the auth page at least
}

/**
 * =============================================================================
 * Type Definitions - Generated from OpenAPI Specification
 * =============================================================================
 * 
 * TYPE SAFETY STRATEGY:
 * These type aliases provide a clean, documented interface to the generated types
 * while maintaining full type safety from the OpenAPI specification.
 * 
 * CODE GENERATION BENEFITS:
 * - Types automatically update when API spec changes
 * - Compile-time validation of data structures
 * - IntelliSense support in IDEs
 * - Refactoring safety across the codebase
 * 
 * IMPORT PATTERN EXPLANATION:
 * - components['schemas']['X']: Data structure definitions
 * - operations['operationId']['responses']['200']['content']['application/json']: Response types
 * 
 * WHY TYPE ALIASES:
 * - Shorter, more readable type names in application code
 * - Insulates application from generated type naming changes
 * - Provides a place to add application-specific type documentation
 * - Enables custom type composition when needed
 */

// Core Memory Node Types
type Node = components['schemas']['Node'];
type NodeDetails = components['schemas']['NodeDetails'];

// Request/Response Types for Bulk Operations
type BulkDeleteRequest = components['schemas']['BulkDeleteRequest'];
type BulkDeleteResponse = components['schemas']['BulkDeleteResponse'];

// Graph Visualization Types
type GraphDataResponse = components['schemas']['GraphDataResponse'];

// Request Types (commented out as they're simple and used inline)
// type CreateNodeRequest = components['schemas']['CreateNodeRequest'];
// type UpdateNodeRequest = components['schemas']['UpdateNodeRequest'];

// Operation Response Types - Extract specific response shapes
type ListNodesResponse = operations['listNodes']['responses']['200']['content']['application/json'];

/**
 * =============================================================================
 * ApiClient Class - Centralized HTTP Communication
 * =============================================================================
 * 
 * DESIGN PATTERNS DEMONSTRATED:
 * - Singleton pattern for single API client instance
 * - Template method pattern for consistent request handling
 * - Strategy pattern for different HTTP methods
 * - Decorator pattern for automatic authentication
 * 
 * ARCHITECTURAL BENEFITS:
 * - Single point of configuration for all API calls
 * - Consistent error handling across the application
 * - Type safety for all HTTP operations
 * - Automatic authentication token management
 * - Centralized logging and debugging
 */
class ApiClient {
    /**
     * Generic HTTP Request Method - The Foundation of All API Calls
     * 
     * This private method implements the core HTTP communication logic used by
     * all public API methods. It demonstrates several important patterns:
     * 
     * AUTHENTICATION INTEGRATION:
     * - Automatically retrieves and injects JWT tokens
     * - Handles authentication failures gracefully
     * - Integrates with Supabase auth client seamlessly
     * - Provides clear error messages for auth issues
     * 
     * ERROR HANDLING STRATEGY:
     * - Structured error parsing from API responses
     * - Fallback error messages for parsing failures
     * - Comprehensive logging for debugging
     * - Propagates errors to calling code for handling
     * 
     * TYPE SAFETY IMPLEMENTATION:
     * - Generic type parameter <T> for return type safety
     * - Compile-time validation of response structure
     * - TypeScript's as assertion for JSON parsing
     * - Prevents runtime type errors
     * 
     * HTTP BEST PRACTICES:
     * - Proper Content-Type headers for JSON APIs
     * - Authorization header format (Bearer token)
     * - HTTP status code checking
     * - Request body serialization
     * 
     * PERFORMANCE CONSIDERATIONS:
     * - Minimal overhead for authentication
     * - Efficient JSON serialization/deserialization
     * - Error object reuse to prevent memory leaks
     * - Async/await for non-blocking operations
     * 
     * @param method HTTP verb (GET, POST, PUT, DELETE)
     * @param path API endpoint path (e.g., '/api/nodes')
     * @param body Request payload for POST/PUT operations
     * @returns Promise resolving to typed response data
     * @throws Error for authentication, network, or API errors
     */
    private async request<T>(method: string, path: string, body: unknown = null): Promise<T> {
        // Step 1: Authentication Token Retrieval
        // Integrate with Supabase auth to get current user's JWT token
        // This token proves user identity to the API Gateway
        const token = await auth.getJwtToken();
        if (!token) {
            throw new Error('Not authenticated');
        }

        // Step 2: HTTP Request Configuration
        // Build the request configuration with authentication and content type
        // Authorization header format follows Bearer token standard
        const options: RequestInit = {
            method,
            headers: {
                'Authorization': `Bearer ${token}`,    // JWT authentication
                'Content-Type': 'application/json',    // JSON content type
            },
        };

        // Step 3: Request Body Serialization
        // Convert JavaScript objects to JSON strings for transmission
        // Only add body for methods that support it (POST, PUT, PATCH)
        if (body) {
            options.body = JSON.stringify(body);
        }

        try {
            // Step 4: HTTP Request Execution
            // Use native fetch API for HTTP communication
            // Construct full URL by combining base URL and endpoint path
            const response = await fetch(`${API_BASE_URL}${path}`, options);
            
            // Step 5: Response Status Validation
            // Check HTTP status codes to determine if request succeeded
            // HTTP 2xx codes indicate success, others indicate errors
            if (!response.ok) {
                // Step 5a: Error Response Parsing
                // Try to extract error details from API response body
                // Fallback to generic error if parsing fails
                const errorData = await response.json().catch(() => ({ error: 'Request failed' }));
                throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
            }
            
            // Step 6: Success Response Processing
            // Parse JSON response and cast to expected type
            // TypeScript type assertion provides compile-time safety
            return await response.json() as T;
        } catch (error) {
            // Step 7: Error Logging and Propagation
            // Log errors for debugging while preserving original error
            // Provides context about which API call failed
            console.error('API request failed:', method, path, error);
            throw error;
        }
    }

    /**
     * ==========================================================================
     * Public API Methods - Type-Safe Memory Management Operations
     * ==========================================================================
     * 
     * These methods provide the public interface for memory management operations.
     * Each method corresponds to a specific API endpoint defined in the OpenAPI
     * specification, ensuring consistency between frontend and backend.
     * 
     * DESIGN PRINCIPLES:
     * - One method per API endpoint for clarity
     * - Type-safe parameters and return values
     * - Self-documenting method names matching API operations
     * - Consistent error handling through base request method
     * - RESTful resource mapping (createNode → POST /api/nodes)
     */

    /**
     * Create New Memory Node
     * 
     * Creates a new memory node with automatic keyword extraction and connection discovery.
     * This triggers the entire Brain2 workflow: NLP processing, relationship discovery,
     * and real-time graph updates.
     * 
     * BUSINESS WORKFLOW TRIGGERED:
     * 1. Content validation on server
     * 2. Keyword extraction using NLP algorithms
     * 3. Connection discovery with existing memories
     * 4. Bidirectional relationship creation
     * 5. Real-time WebSocket notification to update graph
     * 
     * UI INTEGRATION PATTERNS:
     * - Call from memory creation forms
     * - Show loading state during processing
     * - Handle validation errors with user feedback
     * - Refresh graph after successful creation
     * 
     * @param content The text content of the memory to create
     * @returns Promise resolving to the created Node with ID and metadata
     * @throws Error if content is empty or creation fails
     */
    public createNode(content: string): Promise<Node> {
        return this.request<Node>('POST', '/api/nodes', { content });
    }

    /**
     * List All User's Memory Nodes
     * 
     * Retrieves all memory nodes belonging to the authenticated user.
     * Used for initial data loading and memory browsing interfaces.
     * 
     * SECURITY NOTES:
     * - User isolation handled by JWT authentication
     * - Only returns nodes owned by authenticated user
     * - No pagination currently (consider for large memory sets)
     * 
     * UI USAGE PATTERNS:
     * - Initial app loading to populate memory list
     * - Search and filter interfaces
     * - Memory dashboard and overview screens
     * - Backup and export functionality
     * 
     * @returns Promise resolving to array of user's memory nodes
     * @throws Error if authentication fails or request errors
     */
    public listNodes(): Promise<ListNodesResponse> {
        return this.request<ListNodesResponse>('GET', '/api/nodes');
    }

    /**
     * Get Detailed Node Information
     * 
     * Retrieves comprehensive information about a specific memory node,
     * including its content and all connected node relationships.
     * 
     * DETAILED DATA INCLUDES:
     * - Node content and metadata (timestamp, version)
     * - Array of connected node IDs (edges)
     * - All information needed for detailed memory view
     * 
     * UI INTEGRATION:
     * - Memory detail modals and side panels
     * - Related memories navigation
     * - Graph node click handlers
     * - Memory editing interfaces
     * 
     * @param nodeId Unique identifier of the memory node
     * @returns Promise resolving to NodeDetails with content and connections
     * @throws Error if node not found or access denied
     */
    public getNode(nodeId: string): Promise<NodeDetails> {
        return this.request<NodeDetails>('GET', `/api/nodes/${nodeId}`);
    }

    /**
     * Delete Memory Node
     * 
     * Permanently deletes a memory node and all its relationships.
     * This is an irreversible operation that maintains graph integrity.
     * 
     * DELETION WORKFLOW:
     * 1. Verify user owns the node
     * 2. Remove all edges where node is source or target
     * 3. Clean up keyword associations
     * 4. Delete node metadata
     * 5. Broadcast deletion via WebSocket for real-time updates
     * 
     * UI CONSIDERATIONS:
     * - Show confirmation dialog for irreversible action
     * - Update graph visualization immediately
     * - Handle deletion errors gracefully
     * - Consider implementing undo functionality
     * 
     * @param nodeId Unique identifier of the memory node to delete
     * @returns Promise resolving to success message
     * @throws Error if node not found, access denied, or deletion fails
     */
    public deleteNode(nodeId: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('DELETE', `/api/nodes/${nodeId}`);
    }

    /**
     * Get Complete Graph Visualization Data
     * 
     * Retrieves the user's complete knowledge graph in a format optimized
     * for visualization libraries like Cytoscape.js. This is the primary
     * data source for graph rendering.
     * 
     * DATA FORMAT OPTIMIZATION:
     * - Nodes contain IDs and display labels
     * - Edges contain source/target references
     * - Format directly consumable by graph libraries
     * - Minimal payload size for performance
     * 
     * USAGE SCENARIOS:
     * - Initial graph rendering on page load
     * - Full graph refresh after major changes
     * - Export functionality for data visualization
     * - Fallback when WebSocket connection unavailable
     * 
     * PERFORMANCE CONSIDERATIONS:
     * - Single query retrieves complete graph
     * - May become large for users with many memories
     * - Consider implementing pagination or filtering
     * - Cache aggressively due to stable format
     * 
     * @returns Promise resolving to complete graph data with nodes and edges
     * @throws Error if graph data retrieval fails
     */
    public getGraphData(): Promise<GraphDataResponse> {
        return this.request<GraphDataResponse>('GET', '/api/graph-data');
    }

    /**
     * Update Memory Node Content
     * 
     * Updates an existing memory node's content and recalculates all its
     * connections based on the new keywords extracted from updated content.
     * 
     * UPDATE WORKFLOW:
     * 1. Validate user owns the node
     * 2. Extract keywords from new content
     * 3. Remove old connections that no longer apply
     * 4. Create new connections based on updated keywords
     * 5. Update node content and increment version
     * 6. Broadcast changes via WebSocket
     * 
     * CONNECTION RECALCULATION:
     * - Complete re-analysis of relationships
     * - Ensures connections stay relevant to content
     * - May create new connections or remove old ones
     * - Maintains graph accuracy and usefulness
     * 
     * UI INTEGRATION:
     * - Memory editing forms and inline editors
     * - Auto-save functionality
     * - Optimistic UI updates
     * - Version conflict handling
     * 
     * @param nodeId Unique identifier of the memory node to update
     * @param content New text content for the memory
     * @returns Promise resolving to success message with version info
     * @throws Error if validation fails, node not found, or update fails
     */
    public updateNode(nodeId: string, content: string): Promise<{ message: string }> {
        return this.request<{ message: string }>('PUT', `/api/nodes/${nodeId}`, { content });
    }

    /**
     * Bulk Delete Multiple Memory Nodes
     * 
     * Efficiently deletes multiple memory nodes in a single operation,
     * optimized for better user experience than individual deletions.
     * 
     * BULK OPERATION BENEFITS:
     * - Single API call reduces network overhead
     * - Better user experience with unified progress
     * - Efficient backend processing
     * - Partial success handling for resilience
     * 
     * PARTIAL SUCCESS HANDLING:
     * - Operation continues even if individual nodes fail
     * - Returns count of successful deletions
     * - Lists failed node IDs for retry logic
     * - Enables graceful error handling in UI
     * 
     * UI USAGE PATTERNS:
     * - Multi-select deletion in graph interface
     * - Batch cleanup operations
     * - Administrative memory management
     * - Bulk organization workflows
     * 
     * LIMITATIONS:
     * - Maximum 100 nodes per request (API constraint)
     * - No transaction guarantees across all nodes
     * - Individual failures don't rollback successful deletions
     * 
     * @param nodeIds Array of node IDs to delete (max 100)
     * @returns Promise resolving to deletion results with success/failure details
     * @throws Error if request validation fails or bulk operation errors
     */
    public bulkDeleteNodes(nodeIds: string[]): Promise<BulkDeleteResponse> {
        return this.request<BulkDeleteResponse>('POST', '/api/nodes/bulk-delete', { nodeIds });
    }
}

/**
 * =============================================================================
 * Singleton API Client Export
 * =============================================================================
 * 
 * SINGLETON PATTERN BENEFITS:
 * - Single shared instance across entire application
 * - Consistent configuration and state management
 * - Memory efficiency (one HTTP client, not per component)
 * - Centralized interceptors and middleware
 * 
 * USAGE PATTERN:
 * Import and use directly in any component or service:
 * 
 * ```typescript
 * import { api } from './apiClient';
 * 
 * // Create new memory
 * const node = await api.createNode("My new thought");
 * 
 * // Get graph data for visualization
 * const graph = await api.getGraphData();
 * 
 * // Delete multiple memories
 * const result = await api.bulkDeleteNodes(['id1', 'id2']);
 * ```
 * 
 * TESTING CONSIDERATIONS:
 * - Can be mocked easily for unit tests
 * - Singleton pattern enables global mock replacement
 * - All HTTP calls go through single point for testing
 * 
 * FUTURE ENHANCEMENTS:
 * - Request/response interceptors for logging
 * - Automatic retry logic for failed requests
 * - Request deduplication for identical calls
 * - Caching layer for read operations
 * - Request cancellation for navigation changes
 */
export const api = new ApiClient();
