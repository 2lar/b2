/**
 * =============================================================================
 * WebSocket Connection Lambda - Real-Time Communication Infrastructure
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This Lambda function handles WebSocket connection establishment for the Brain2
 * real-time communication system. It demonstrates advanced serverless patterns,
 * JWT authentication for WebSocket connections, and real-time infrastructure
 * management using AWS services.
 * 
 * 🏗️ KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. WEBSOCKET SERVERLESS ARCHITECTURE:
 *    - AWS API Gateway WebSocket API for persistent connections
 *    - Lambda functions for connection lifecycle management
 *    - DynamoDB for connection state tracking and user mapping
 *    - Automatic scaling and connection management
 * 
 * 2. AUTHENTICATION FOR PERSISTENT CONNECTIONS:
 *    - JWT token validation for WebSocket connections
 *    - Supabase integration for user identity verification
 *    - Connection-to-user mapping for message routing
 *    - Secure real-time communication channels
 * 
 * 3. CONNECTION STATE MANAGEMENT:
 *    - DynamoDB as connection registry and state store
 *    - TTL-based automatic cleanup of stale connections
 *    - Bidirectional lookup patterns (user→connections, connection→user)
 *    - Global Secondary Index for efficient queries
 * 
 * 4. SERVERLESS EVENT-DRIVEN PATTERNS:
 *    - Lambda cold start optimization strategies
 *    - Environment-based configuration management
 *    - Error handling for distributed systems
 *    - Observability and logging best practices
 * 
 * 5. REAL-TIME COMMUNICATION LIFECYCLE:
 *    - Connection establishment with authentication
 *    - Connection tracking and user association
 *    - Message routing and delivery mechanisms
 *    - Connection cleanup and resource management
 * 
 * 🔄 CONNECTION WORKFLOW:
 * 1. Client initiates WebSocket connection with JWT token
 * 2. API Gateway triggers this Lambda function
 * 3. Lambda validates JWT with Supabase
 * 4. Connection stored in DynamoDB with user mapping
 * 5. Client receives connection confirmation
 * 6. Real-time message exchange begins
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - WebSocket serverless architecture patterns
 * - JWT authentication for persistent connections
 * - DynamoDB connection state management
 * - Real-time communication infrastructure
 * - AWS Lambda optimization techniques
 * - Error handling in distributed systems
 */
package main

import (
	"context"   // Context for request cancellation and timeouts
	"fmt"       // String formatting for DynamoDB values
	"log"       // Structured logging for monitoring and debugging
	"net/http"  // HTTP status codes for WebSocket responses
	"os"        // Environment variable access
	"time"      // Time operations for TTL and expiration

	// AWS Lambda and API Gateway integration
	"github.com/aws/aws-lambda-go/events" // Event structures for API Gateway WebSocket
	"github.com/aws/aws-lambda-go/lambda" // Lambda runtime and handler registration

	// AWS SDK v2 for modern, performant AWS service integration
	"github.com/aws/aws-sdk-go-v2/aws"             // Core AWS configuration
	awsConfig "github.com/aws/aws-sdk-go-v2/config" // Configuration loading
	"github.com/aws/aws-sdk-go-v2/service/dynamodb" // DynamoDB client
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types" // DynamoDB data types

	// Third-party authentication service integration
	"github.com/supabase-community/supabase-go" // Supabase Go client for JWT validation
)

/**
 * =============================================================================
 * Global Service Clients - Lambda Optimization Patterns
 * =============================================================================
 * 
 * LAMBDA COLD START OPTIMIZATION:
 * These global variables are initialized once during Lambda container creation
 * and reused across function invocations, dramatically improving performance
 * by avoiding repeated client initialization.
 * 
 * PERFORMANCE BENEFITS:
 * - Client initialization happens once per container lifecycle
 * - Database connections are pooled and reused
 * - Reduces latency from ~500ms to ~10ms for warm invocations
 * - AWS SDK automatically handles connection pooling and retries
 */

// Global DynamoDB client for connection state management
// SINGLETON PATTERN: Single client instance shared across invocations
// CONNECTION POOLING: AWS SDK manages connection reuse and retries
var dbClient *dynamodb.Client

// Global Supabase client for JWT authentication
// AUTHENTICATION SERVICE: Validates user tokens and retrieves user data
// RATE LIMITING: Shared client helps manage API rate limits efficiently
var supabaseClient *supabase.Client

// Global configuration for DynamoDB table name
// ENVIRONMENT CONFIGURATION: Set via AWS CDK deployment
// IMMUTABLE CONFIG: Table name doesn't change during Lambda execution
var connectionsTable string

/**
 * Lambda Container Initialization
 * 
 * The init() function runs once per Lambda container lifecycle, before any
 * handler invocations. It's the perfect place for expensive initialization
 * operations like client creation and configuration loading.
 * 
 * INITIALIZATION PATTERNS:
 * 1. Environment variable validation with fail-fast behavior
 * 2. AWS SDK configuration loading with default credential chain
 * 3. External service client initialization
 * 4. Configuration validation and error handling
 * 
 * ERROR HANDLING STRATEGY:
 * - Fail fast if required configuration is missing
 * - Provide clear error messages for debugging
 * - Use log.Fatalf to terminate container on critical errors
 * - Prevent partially initialized Lambda from accepting requests
 * 
 * SECURITY CONSIDERATIONS:
 * - Service role key vs anon key for server-side operations
 * - Environment variables for sensitive configuration
 * - AWS IAM roles for DynamoDB access permissions
 */
func init() {
	// Step 1: Environment Configuration Loading
	// Load required configuration from environment variables set by CDK
	connectionsTable = os.Getenv("CONNECTIONS_TABLE_NAME")
	supabaseURL := os.Getenv("SUPABASE_URL")
	
	// SECURITY NOTE: Service role key for server-side JWT validation
	// Service role has elevated permissions vs client-side anon key
	// Enables server-side user data access and administrative operations
	supabaseKey := os.Getenv("SUPABASE_SERVICE_ROLE_KEY")

	// Step 2: Configuration Validation
	// FAIL-FAST PRINCIPLE: Validate all required config before proceeding
	// DEFENSIVE PROGRAMMING: Better to fail at startup than during runtime
	if connectionsTable == "" || supabaseURL == "" || supabaseKey == "" {
		log.Fatalf("FATAL: Environment variables CONNECTIONS_TABLE_NAME, SUPABASE_URL, and SUPABASE_SERVICE_ROLE_KEY must be set.")
	}

	// Step 3: AWS SDK Configuration
	// DEFAULT CREDENTIAL CHAIN: Lambda execution role, environment vars, metadata service
	// CONTEXT USAGE: TODO context is acceptable for initialization code
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}
	
	// Step 4: DynamoDB Client Initialization
	// AWS SDK V2: Modern, performance-optimized SDK with better error handling
	// CONNECTION POOLING: SDK automatically manages connection lifecycle
	dbClient = dynamodb.NewFromConfig(awsCfg)

	// Step 5: Supabase Client Initialization
	// THIRD-PARTY INTEGRATION: External authentication service client
	// JWT VALIDATION: Service role enables server-side token verification
	client, err := supabase.NewClient(supabaseURL, supabaseKey, nil)
	if err != nil {
		log.Fatalf("Unable to create Supabase client: %v", err)
	}
	supabaseClient = client
}

/**
 * WebSocket Connection Handler - Real-Time Authentication & State Management
 * 
 * This function handles WebSocket connection requests from clients, implementing
 * a complete authentication and connection tracking workflow for real-time
 * communication in a serverless environment.
 * 
 * CONNECTION ESTABLISHMENT WORKFLOW:
 * 1. Extract and validate JWT token from connection request
 * 2. Authenticate user identity with Supabase
 * 3. Store connection state in DynamoDB with user mapping
 * 4. Set up automatic connection cleanup with TTL
 * 5. Return success response to complete WebSocket handshake
 * 
 * AUTHENTICATION STRATEGY:
 * - JWT token passed as query parameter (WebSocket limitation)
 * - Server-side token validation with Supabase
 * - User identity extraction for connection mapping
 * - Secure connection establishment without session state
 * 
 * CONNECTION STATE MANAGEMENT:
 * - DynamoDB single-table design for connection tracking
 * - Bidirectional lookups (user→connections, connection→user)
 * - TTL-based automatic cleanup of stale connections
 * - Global Secondary Index for efficient queries
 * 
 * ERROR HANDLING PATTERNS:
 * - Structured error responses with appropriate HTTP status codes
 * - Comprehensive logging for debugging and monitoring
 * - Graceful degradation for authentication failures
 * - Clear error messages for troubleshooting
 * 
 * @param ctx Request context for cancellation and timeout handling
 * @param req WebSocket connection request with authentication token
 * @return HTTP response indicating connection success or failure
 */
func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	
	// ==========================================================================
	// Step 1: Authentication Token Extraction and Validation
	// ==========================================================================
	//
	// WEBSOCKET AUTHENTICATION CHALLENGE:
	// Unlike HTTP requests, WebSocket connections cannot use standard HTTP headers
	// for authentication. JWT tokens must be passed as query parameters during
	// the initial handshake request.
	//
	// SECURITY CONSIDERATIONS:
	// - Tokens in query parameters are logged by API Gateway
	// - Use short-lived JWT tokens to minimize exposure risk
	// - Validate tokens server-side to prevent tampering
	
	token, ok := req.QueryStringParameters["token"]
	if !ok || token == "" {
		log.Println("WARN: Connection request missing token.")
		// Return 401 Unauthorized to reject connection without authentication
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
	}

	// ==========================================================================
	// Step 2: JWT Token Validation with Supabase
	// ==========================================================================
	//
	// SUPABASE AUTHENTICATION FLOW:
	// 1. Client obtains JWT token from Supabase Auth
	// 2. Token contains user identity and permissions
	// 3. Server validates token signature and expiration
	// 4. User information extracted for connection mapping
	//
	// SECURITY BENEFITS:
	// - Cryptographically signed tokens prevent forgery
	// - Expiration times limit window of token compromise
	// - Supabase handles complex JWT validation logic
	// - User permissions encoded in token claims
	
	// SUPABASE CLIENT PATTERN: WithToken() creates request-scoped authentication
	// The GetUser() method validates token and returns user profile
	// Context is implicitly used in underlying HTTP request to Supabase API
	user, err := supabaseClient.Auth.WithToken(token).GetUser()
	if err != nil {
		log.Printf("ERROR: Invalid token provided. %v", err)
		// Authentication failure - reject connection
		return events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
	}

	// ==========================================================================
	// Step 3: Connection Metadata Extraction and Processing
	// ==========================================================================
	//
	// CONNECTION IDENTITY MANAGEMENT:
	// - ConnectionID: Unique identifier for this WebSocket connection
	// - UserID: Authenticated user identity from JWT token
	// - TTL: Automatic connection cleanup for resource management
	
	// Extract unique connection identifier from API Gateway
	// ConnectionID is generated by API Gateway for each WebSocket connection
	connectionID := req.RequestContext.ConnectionID
	
	// Convert Supabase UUID to string for database storage
	// UUIDs provide globally unique user identification
	userID := user.ID.String()
	
	// AUTOMATIC CONNECTION CLEANUP STRATEGY:
	// Set TTL (Time To Live) for automatic cleanup of stale connections
	// Prevents accumulation of dead connections in the database
	// 2-hour TTL balances cleanup with typical session duration
	expireAt := time.Now().Add(2 * time.Hour).Unix()

	// ==========================================================================
	// Step 4: DynamoDB Single-Table Design for Connection Storage
	// ==========================================================================
	//
	// SINGLE-TABLE DESIGN PATTERNS:
	// - PK (Partition Key): Groups all user's connections together
	// - SK (Sort Key): Uniquely identifies each connection
	// - GSI1: Enables reverse lookup (connection → user)
	// - TTL: Automatic item deletion for resource management
	//
	// DATA ACCESS PATTERNS SUPPORTED:
	// 1. Get all connections for a user: Query by PK
	// 2. Get user for a connection: Query GSI1 by GSI1PK
	// 3. Delete specific connection: Delete by PK + SK
	// 4. Automatic cleanup: DynamoDB TTL feature
	
	// Primary access pattern: USER#<userID> → CONN#<connectionID>
	pk := "USER#" + userID
	sk := "CONN#" + connectionID

	// DynamoDB PutItem operation with comprehensive attribute set
	_, err = dbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(connectionsTable),
		Item: map[string]types.AttributeValue{
			// Primary key for user-based queries
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
			
			// Global Secondary Index for connection-based reverse lookup
			// Enables finding user from connection ID during disconnect
			"GSI1PK": &types.AttributeValueMemberS{Value: sk}, // Connection as partition key
			"GSI1SK": &types.AttributeValueMemberS{Value: pk}, // User as sort key
			
			// TTL attribute for automatic cleanup
			// DynamoDB automatically deletes items when TTL expires
			"expireAt": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expireAt)},
		},
	})

	// ==========================================================================
	// Step 5: Error Handling and Response Generation
	// ==========================================================================
	
	if err != nil {
		log.Printf("ERROR: Failed to save connection to DynamoDB: %v", err)
		// Database error - return 500 Internal Server Error
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// ==========================================================================
	// Step 6: Success Response and Connection Confirmation
	// ==========================================================================
	
	// LOG SUCCESS for monitoring and debugging
	// Include both user ID and connection ID for correlation
	log.Printf("Successfully connected user %s with connection ID %s", userID, connectionID)
	
	// Return 200 OK to complete WebSocket handshake
	// Client will receive connection confirmation and can begin sending messages
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}

/**
 * Lambda Function Entry Point
 * 
 * Registers the WebSocket connection handler with the AWS Lambda runtime.
 * This simple main function demonstrates the standard pattern for AWS Lambda
 * function initialization in Go.
 * 
 * LAMBDA RUNTIME INTEGRATION:
 * - lambda.Start() registers the handler function with AWS Lambda runtime
 * - Runtime handles HTTP event parsing and response formatting
 * - Automatic integration with API Gateway WebSocket events
 * - Built-in error handling and logging integration
 * 
 * SERVERLESS EXECUTION MODEL:
 * - Function runs in response to WebSocket connection events
 * - AWS manages scaling, availability, and resource allocation
 * - Pay-per-execution pricing model
 * - Automatic integration with AWS monitoring and logging
 */
func main() {
	lambda.Start(handler)
}
