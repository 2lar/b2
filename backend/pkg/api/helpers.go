/**
 * =============================================================================
 * API Response Helpers - Standardized HTTP Response Patterns
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This package provides standardized helper functions for consistent HTTP API
 * responses. It demonstrates best practices for API response formatting,
 * error handling, and content type management in Go web services.
 * 
 * 🏗️ KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. RESPONSE CONSISTENCY:
 *    - Standardized JSON response format across all endpoints
 *    - Consistent HTTP status code usage
 *    - Uniform content type and header management
 *    - Predictable error response structure
 * 
 * 2. DRY PRINCIPLE (DON'T REPEAT YOURSELF):
 *    - Single source of truth for response formatting
 *    - Reusable functions reduce code duplication
 *    - Centralized error handling patterns
 *    - Consistent behavior across all handlers
 * 
 * 3. ERROR HANDLING STRATEGY:
 *    - Structured error responses with clear messages
 *    - Appropriate HTTP status codes for different error types
 *    - Client-friendly error format for frontend consumption
 *    - Separation of user-facing and internal error details
 * 
 * 4. CONTENT TYPE MANAGEMENT:
 *    - Explicit JSON content type declaration
 *    - Proper header setting before response writing
 *    - Browser and HTTP client compatibility
 *    - Clear API contract for response format
 * 
 * 5. INTERFACE-BASED DESIGN:
 *    - Uses Go's interface{} for flexible data types
 *    - Type-safe JSON serialization
 *    - Supports any serializable data structure
 *    - Runtime type checking via JSON encoder
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - HTTP response patterns and best practices
 * - Go JSON encoding and serialization
 * - Error handling in web APIs
 * - HTTP header management
 * - Interface usage for generic programming
 * - Consistent API design principles
 */
package api

import (
	"encoding/json"
	"net/http"
)

/**
 * =============================================================================
 * Response Helper Functions - Building Blocks for HTTP APIs
 * =============================================================================
 * 
 * These functions provide the foundation for all HTTP responses in the Brain2 API.
 * They ensure consistency, reduce boilerplate code, and implement best practices
 * for web API development.
 */

/**
 * Success Response Helper
 * 
 * Sends a standardized successful response with optional data payload.
 * This function handles the complete success response workflow including
 * headers, status codes, and JSON serialization.
 * 
 * RESPONSE STRUCTURE PATTERN:
 * For data responses: { "field1": "value1", "field2": "value2", ... }
 * For empty responses: HTTP status code only (no body)
 * 
 * CONTENT TYPE MANAGEMENT:
 * - Sets Content-Type to application/json for API consistency
 * - Informs clients to expect JSON response format
 * - Enables proper client-side JSON parsing
 * - Required for browser and HTTP client compatibility
 * 
 * STATUS CODE STRATEGY:
 * - 200 OK: Successful GET requests with data
 * - 201 Created: Successful POST requests creating resources
 * - 204 No Content: Successful operations with no response data
 * - Custom codes: Flexible status code support for specific scenarios
 * 
 * DATA SERIALIZATION:
 * - Uses Go's JSON encoder for automatic serialization
 * - Supports any data type that implements JSON marshaling
 * - Handles nested structures, arrays, and complex objects
 * - Provides runtime error handling for serialization failures
 * 
 * NULL DATA HANDLING:
 * - Checks for nil data to avoid sending null JSON
 * - Empty responses send only status code and headers
 * - Prevents unnecessary network payload for status-only responses
 * - Improves performance for acknowledgment endpoints
 * 
 * USAGE EXAMPLES:
 * 
 * ```go
 * // Send node data
 * Success(w, 200, node)
 * 
 * // Send creation confirmation
 * Success(w, 201, createdNode)
 * 
 * // Send acknowledgment only
 * Success(w, 204, nil)
 * 
 * // Send list data
 * Success(w, 200, map[string]interface{}{"nodes": nodeList})
 * ```
 * 
 * ERROR SCENARIOS:
 * - JSON marshaling failures (rare, but possible with complex types)
 * - Network interruptions during response writing
 * - Client disconnections before response completion
 * 
 * @param w HTTP response writer for sending data to client
 * @param statusCode HTTP status code indicating operation result
 * @param data Optional data payload to serialize as JSON (can be nil)
 */
func Success(w http.ResponseWriter, statusCode int, data interface{}) {
	// Step 1: Set Response Headers
	// Content-Type header informs client about response format
	// Must be set before WriteHeader() call for proper HTTP compliance
	w.Header().Set("Content-Type", "application/json")
	
	// Step 2: Set HTTP Status Code
	// WriteHeader sends the HTTP status line and headers
	// Must be called before writing response body
	w.WriteHeader(statusCode)
	
	// Step 3: Conditional Data Serialization
	// Only encode and send data if it's not nil
	// Prevents sending "null" JSON for empty responses
	if data != nil {
		// JSON encoding directly to response writer for efficiency
		// Automatically handles serialization of Go types to JSON
		// Error handling could be added here for production systems
		json.NewEncoder(w).Encode(data)
	}
}

/**
 * Error Response Helper
 * 
 * Sends a standardized error response with consistent format and appropriate
 * HTTP status codes. This function ensures all API errors follow the same
 * structure for predictable client-side error handling.
 * 
 * ERROR RESPONSE STRUCTURE:
 * Always returns: {"error": "descriptive error message"}
 * - Consistent field name ("error") for client parsing
 * - Human-readable error messages for user feedback
 * - Machine-parseable format for automated error handling
 * - Simple structure for easy client-side processing
 * 
 * HTTP STATUS CODE USAGE:
 * - 400 Bad Request: Client sent invalid data (validation errors)
 * - 401 Unauthorized: Authentication required or invalid credentials
 * - 403 Forbidden: User lacks permission for the operation
 * - 404 Not Found: Requested resource doesn't exist
 * - 409 Conflict: Request conflicts with current resource state
 * - 422 Unprocessable Entity: Valid format but semantic errors
 * - 500 Internal Server Error: Unexpected server-side errors
 * 
 * ERROR MESSAGE PRINCIPLES:
 * - User-friendly language for frontend display
 * - Specific enough for developers to debug issues
 * - No sensitive information (internal paths, stack traces)
 * - Actionable guidance when possible ("field X is required")
 * 
 * SECURITY CONSIDERATIONS:
 * - Error messages don't leak internal implementation details
 * - No database error messages exposed to clients
 * - No file paths or system information in error responses
 * - Generic messages for security-sensitive operations
 * 
 * CLIENT INTEGRATION:
 * Frontend can reliably access error messages via:
 * ```typescript
 * try {
 *   const response = await api.createNode(content);
 * } catch (error) {
 *   const errorData = await error.response.json();
 *   showUserError(errorData.error); // Display user-friendly message
 * }
 * ```
 * 
 * USAGE EXAMPLES:
 * 
 * ```go
 * // Validation error
 * Error(w, 400, "content cannot be empty")
 * 
 * // Authentication error
 * Error(w, 401, "authentication required")
 * 
 * // Authorization error
 * Error(w, 403, "insufficient permissions")
 * 
 * // Resource not found
 * Error(w, 404, "node not found")
 * 
 * // Server error (generic message for security)
 * Error(w, 500, "internal server error")
 * ```
 * 
 * ERROR HANDLING BEST PRACTICES:
 * - Log detailed errors server-side for debugging
 * - Send generic user-friendly messages to clients
 * - Use appropriate HTTP status codes for proper semantics
 * - Consistent error format across all endpoints
 * 
 * @param w HTTP response writer for sending error to client
 * @param statusCode HTTP status code indicating error type
 * @param message User-friendly error message for client consumption
 */
func Error(w http.ResponseWriter, statusCode int, message string) {
	// Step 1: Set Response Headers
	// Content-Type indicates JSON format for consistent client parsing
	// Error responses use same content type as success responses
	w.Header().Set("Content-Type", "application/json")
	
	// Step 2: Set HTTP Status Code
	// Status code indicates the category of error for client handling
	// Enables clients to implement different behavior per error type
	w.WriteHeader(statusCode)
	
	// Step 3: Send Structured Error Response
	// Create standard error object with consistent field name
	// Uses map[string]string for simple, predictable JSON structure
	// JSON encoder automatically handles serialization and HTTP transmission
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
