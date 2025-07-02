/**
 * =============================================================================
 * WebSocket Message Broadcasting Lambda - Real-Time Event Distribution
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This Lambda function handles real-time message broadcasting to WebSocket
 * clients. It demonstrates event-driven architecture, message distribution
 * patterns, and connection management in a serverless environment. This is
 * the heart of the real-time collaboration system.
 * 
 * 🏗️ KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. EVENT-DRIVEN MESSAGE BROADCASTING:
 *    - EventBridge triggers for graph changes
 *    - Fan-out message delivery to multiple clients
 *    - Real-time notification system
 *    - Decoupled event processing architecture
 * 
 * 2. WEBSOCKET CONNECTION MANAGEMENT:
 *    - Active connection discovery via DynamoDB queries
 *    - Stale connection detection and cleanup
 *    - Message delivery failure handling
 *    - Connection health management
 * 
 * 3. API GATEWAY MANAGEMENT API:
 *    - Server-to-client message pushing
 *    - Connection status monitoring
 *    - Message delivery confirmation
 *    - WebSocket protocol handling
 * 
 * 4. RESILIENT MESSAGE DELIVERY:
 *    - Graceful handling of disconnected clients
 *    - Automatic cleanup of stale connections
 *    - Error isolation (one failure doesn't stop others)
 *    - Connection state synchronization
 * 
 * 5. REAL-TIME GRAPH SYNCHRONIZATION:
 *    - Graph update event processing
 *    - Multi-client state synchronization
 *    - Live collaboration features
 *    - Immediate UI updates across sessions
 * 
 * 🔄 MESSAGE FLOW:
 * 1. Backend creates/updates memory node
 * 2. EventBridge publishes graph change event
 * 3. This Lambda receives event and extracts user ID
 * 4. Query DynamoDB for user's active connections
 * 5. Broadcast update message to all connections
 * 6. Clean up any stale connections discovered
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - Event-driven messaging patterns
 * - WebSocket server-side message broadcasting
 * - Connection lifecycle management
 * - Real-time collaboration architecture
 * - Error handling in distributed systems
 * - AWS API Gateway Management API usage
 */
package main

import (
	"context"      // Request lifecycle and cancellation
	"encoding/json" // JSON event parsing and message formatting
	"errors"       // Error type checking and handling
	"log"          // Structured logging for monitoring
	"os"           // Environment variable access
	"strings"      // String manipulation for connection IDs

	// AWS Lambda and EventBridge integration
	"github.com/aws/aws-lambda-go/events" // EventBridge event structures
	"github.com/aws/aws-lambda-go/lambda" // Lambda runtime integration

	// AWS SDK v2 for modern, efficient service integration
	"github.com/aws/aws-sdk-go-v2/aws"                            // Core AWS configuration
	awsConfig "github.com/aws/aws-sdk-go-v2/config"               // Configuration loading
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi" // WebSocket message sending
	apigwTypes "github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/types" // API Gateway error types
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"               // Connection state queries
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"         // DynamoDB data types
)

/**
 * =============================================================================
 * Global Service Clients - Multi-Service Lambda Architecture
 * =============================================================================
 * 
 * DUAL-SERVICE ARCHITECTURE:
 * This Lambda requires both DynamoDB (for connection state) and API Gateway
 * Management API (for message delivery), demonstrating multi-service
 * serverless patterns.
 * 
 * PERFORMANCE OPTIMIZATION:
 * Global clients avoid repeated initialization overhead, crucial for
 * real-time message delivery performance requirements.
 */

// Global DynamoDB client for connection state queries
// PURPOSE: Query user's active WebSocket connections
// OPTIMIZATION: Reused across invocations for better performance
var dbClient *dynamodb.Client

// Global API Gateway Management client for WebSocket message delivery
// PURPOSE: Send messages to active WebSocket connections
// CONFIGURATION: Requires WebSocket API endpoint for message routing
var apiGatewayManagementClient *apigatewaymanagementapi.Client

// Global configuration for connections table name
// SOURCE: Environment variable from CDK deployment
var connectionsTable string

/**
 * Lambda Container Initialization - Multi-Client Setup
 * 
 * Initializes both DynamoDB and API Gateway Management clients for
 * comprehensive WebSocket message broadcasting capabilities.
 * 
 * DUAL-CLIENT PATTERN:
 * 1. DynamoDB client for querying connection state
 * 2. API Gateway Management client for message delivery
 * 
 * CONFIGURATION REQUIREMENTS:
 * - CONNECTIONS_TABLE_NAME: DynamoDB table for connection tracking
 * - WEBSOCKET_API_ENDPOINT: API Gateway WebSocket endpoint for messaging
 * 
 * API GATEWAY MANAGEMENT API:
 * Special AWS service for server-side WebSocket operations
 * Requires specific endpoint configuration for message routing
 * Enables bidirectional communication in serverless WebSocket apps
 */
func init() {
	// Step 1: Load Environment Configuration
	connectionsTable = os.Getenv("CONNECTIONS_TABLE_NAME")
	wsApiEndpoint := os.Getenv("WEBSOCKET_API_ENDPOINT")
	
	// Step 2: AWS SDK Configuration
	// Load default configuration with Lambda execution role credentials
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}
	
	// Step 3: DynamoDB Client Initialization
	// Standard DynamoDB client for connection state queries
	dbClient = dynamodb.NewFromConfig(awsCfg)
	
	// Step 4: API Gateway Management Client Initialization
	// SPECIAL CONFIGURATION: Requires WebSocket API endpoint
	// This client enables server-to-client message pushing
	// BaseEndpoint must match the deployed WebSocket API
	apiGatewayManagementClient = apigatewaymanagementapi.NewFromConfig(awsCfg, func(o *apigatewaymanagementapi.Options) {
		o.BaseEndpoint = &wsApiEndpoint
	})
}

/**
 * =============================================================================
 * Event Data Structure - EventBridge Message Format
 * =============================================================================
 * 
 * EVENTBRIDGE INTEGRATION PATTERN:
 * This struct defines the expected format for events received from EventBridge
 * when graph changes occur in the Brain2 system. It demonstrates event-driven
 * architecture patterns and type-safe event processing.
 * 
 * EVENT SOURCE: Backend memory service
 * EVENT TRIGGER: Node creation, updates, or edge creation
 * EVENT PURPOSE: Notify WebSocket clients of graph changes in real-time
 * 
 * JSON MARSHALING:
 * The `json` tags enable automatic parsing of EventBridge event details
 * into this Go struct, providing type safety and validation.
 */
type EdgesCreatedEvent struct {
	// UserID identifies which user's graph was modified
	// PURPOSE: Query user's active WebSocket connections for targeted messaging
	// FORMAT: UUID string from Supabase authentication
	UserID string `json:"userId"`
	
	// NodeID identifies the specific memory node that triggered the event
	// PURPOSE: Clients can identify which part of their graph changed
	// USAGE: Future enhancement for targeted UI updates
	NodeID string `json:"nodeId"`
}

/**
 * Event Handler - Real-Time Message Broadcasting Workflow
 * 
 * This function implements the complete workflow for broadcasting graph update
 * notifications to all active WebSocket connections for a specific user.
 * It demonstrates event-driven architecture, connection management, and
 * error resilience patterns in serverless environments.
 * 
 * WORKFLOW OVERVIEW:
 * 1. Parse EventBridge event to extract user and node information
 * 2. Query DynamoDB for user's active WebSocket connections
 * 3. Broadcast update message to all found connections
 * 4. Handle stale connections with automatic cleanup
 * 5. Log errors for monitoring without failing the entire operation
 * 
 * ERROR HANDLING STRATEGY:
 * - Parse errors fail the function (data integrity)
 * - Database errors fail the function (critical infrastructure)
 * - Individual message delivery errors are logged but don't fail others
 * - Stale connections are automatically cleaned up
 * 
 * REAL-TIME COLLABORATION:
 * When one user modifies their knowledge graph, all their open browser
 * sessions receive immediate updates, enabling seamless multi-device
 * and multi-tab synchronization.
 * 
 * @param ctx Request context for cancellation and timeout handling
 * @param event EventBridge event containing graph change details
 * @return error if critical operations fail, nil for successful processing
 */
func handler(ctx context.Context, event events.EventBridgeEvent) error {
	
	// ==========================================================================
	// Step 1: Event Data Extraction and Validation
	// ==========================================================================
	//
	// EVENT PARSING:
	// EventBridge events contain a Detail field with JSON-encoded custom data
	// We unmarshal this into our strongly-typed struct for type safety and
	// easier processing throughout the function.
	//
	// ERROR STRATEGY:
	// Parse errors indicate malformed events from EventBridge, which suggests
	// a serious problem in the event publishing side. We fail fast to surface
	// this issue for debugging.
	
	var detail EdgesCreatedEvent
	if err := json.Unmarshal(event.Detail, &detail); err != nil {
		log.Printf("ERROR: could not unmarshal event detail: %v", err)
		// Critical error - malformed event data indicates upstream problem
		return err
	}

	// ==========================================================================
	// Step 2: Connection Discovery - Find User's Active WebSocket Sessions
	// ==========================================================================
	//
	// QUERY PATTERN:
	// Use DynamoDB Query operation with partition key (user) and sort key prefix
	// (connection type) to efficiently find all active connections for this user.
	//
	// SINGLE-TABLE DESIGN:
	// - PK: USER#<userID> groups all user data together
	// - SK: CONN#<connectionID> identifies WebSocket connections
	// - Query retrieves all items where PK matches and SK starts with "CONN#"
	//
	// EFFICIENCY BENEFITS:
	// - O(1) access time using partition key
	// - No need to scan entire table
	// - Scales with number of connections per user, not total users
	
	pk := "USER#" + detail.UserID
	result, err := dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(connectionsTable),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk_prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":        &types.AttributeValueMemberS{Value: pk},
			":sk_prefix": &types.AttributeValueMemberS{Value: "CONN#"},
		},
	})

	// ==========================================================================
	// Step 3: Database Error Handling
	// ==========================================================================
	//
	// CRITICAL ERROR HANDLING:
	// Database failures prevent us from finding connections, making message
	// delivery impossible. We fail the function to trigger retry mechanisms
	// and surface the infrastructure issue.
	//
	// MONITORING INTEGRATION:
	// Error logs enable CloudWatch alerts and operational visibility
	
	if err != nil {
		log.Printf("ERROR: Failed to query connections for user %s: %v", detail.UserID, err)
		// Critical error - can't proceed without connection list
		return err
	}

	// ==========================================================================
	// Step 4: Message Broadcasting Preparation
	// ==========================================================================
	//
	// MESSAGE FORMAT:
	// Simple JSON message indicating graph updates occurred
	// Clients receive this and know to refresh their graph visualization
	//
	// FUTURE ENHANCEMENTS:
	// - Include specific node/edge change details
	// - Support different message types (node added, deleted, updated)
	// - Include change metadata for smarter client updates
	
	message := []byte(`{"action": "graphUpdated"}`)
	
	// ==========================================================================
	// Step 5: Fan-Out Message Delivery to All Connections
	// ==========================================================================
	//
	// BROADCAST PATTERN:
	// Iterate through all active connections and send the same message
	// to each one. This implements a fan-out messaging pattern where
	// one event triggers multiple message deliveries.
	//
	// RESILIENCE STRATEGY:
	// Individual delivery failures don't stop processing of other connections
	// This ensures maximum message delivery even if some connections fail
	
	for _, item := range result.Items {
		// Extract connection ID from DynamoDB sort key
		// SK format: "CONN#<actual-connection-id>"
		connectionID := strings.TrimPrefix(item["SK"].(*types.AttributeValueMemberS).Value, "CONN#")
		
		// ==========================================================================
		// Step 6: Individual Message Delivery
		// ==========================================================================
		//
		// API GATEWAY MANAGEMENT API:
		// PostToConnection sends data to a specific WebSocket connection
		// This is the server-to-client communication mechanism in WebSocket APIs
		//
		// CONNECTION LIFECYCLE:
		// Connections can be closed at any time (browser close, network issues)
		// We handle both successful delivery and various failure scenarios
		
		_, err := apiGatewayManagementClient.PostToConnection(ctx, &apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: &connectionID,
			Data:         message,
		})

		// ==========================================================================
		// Step 7: Connection Error Handling and Cleanup
		// ==========================================================================
		//
		// ERROR TYPE DISCRIMINATION:
		// Different error types require different handling strategies:
		// - GoneException: Connection closed, needs cleanup
		// - Other errors: Temporary issues, log but continue
		//
		// AUTOMATIC CLEANUP:
		// Stale connections are automatically removed from the database
		// This prevents accumulation of dead connections over time
		//
		// NON-BLOCKING ERROR HANDLING:
		// Individual connection failures don't prevent delivery to other connections
		// This maximizes the success rate of message broadcasting
		
		if err != nil {
			// Check if this is a "connection gone" error (client disconnected)
			var goneErr *apigwTypes.GoneException
			if errors.As(err, &goneErr) {
				// CONNECTION CLEANUP:
				// Client disconnected but disconnect Lambda wasn't triggered
				// Clean up the stale connection record from database
				log.Printf("Found stale connection, deleting: %s", connectionID)
				
				// Asynchronous cleanup - don't block on deletion errors
				// If deletion fails, TTL will eventually clean up the record
				dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
					TableName: aws.String(connectionsTable),
					Key: map[string]types.AttributeValue{
						"PK": item["PK"], // Reuse the partition key from query result
						"SK": item["SK"], // Reuse the sort key from query result
					},
				})
			} else {
				// OTHER ERRORS:
				// Network issues, rate limiting, temporary API Gateway problems
				// Log for monitoring but continue processing other connections
				log.Printf("ERROR: Failed to post to connection %s: %v", connectionID, err)
			}
		}
		// SUCCESS CASE:
		// Message delivered successfully, continue to next connection
		// No logging needed for successful delivery (reduces log volume)
	}

	// ==========================================================================
	// Step 8: Successful Function Completion
	// ==========================================================================
	//
	// RETURN SUCCESS:
	// Return nil to indicate successful processing of the event
	// Lambda will not retry and EventBridge considers the event processed
	//
	// PARTIAL FAILURES:
	// Even if some individual message deliveries failed, we return success
	// because the core operation (processing the event) succeeded
	// Individual failures are logged for monitoring and alerting
	
	return nil
}

/**
 * Lambda Function Entry Point - Real-Time Message Broadcasting Service
 * 
 * Registers the message broadcasting handler with the AWS Lambda runtime.
 * This simple main function demonstrates the standard pattern for event-driven
 * Lambda functions in the Brain2 real-time communication system.
 * 
 * LAMBDA RUNTIME INTEGRATION:
 * - lambda.Start() registers the handler function with AWS Lambda runtime
 * - Runtime handles EventBridge event parsing and response formatting
 * - Automatic integration with CloudWatch for logging and monitoring
 * - Built-in error handling and retry mechanisms
 * 
 * EVENT-DRIVEN ARCHITECTURE:
 * - Function runs in response to EventBridge events from memory service
 * - Triggered when users create, update, or connect memory nodes
 * - Enables real-time collaboration across multiple client sessions
 * - Decoupled from memory creation logic for better scalability
 * 
 * SERVERLESS EXECUTION MODEL:
 * - Function scales automatically based on EventBridge event volume
 * - AWS manages resource allocation and availability
 * - Pay-per-execution pricing model optimizes cost
 * - Cold start optimization through global variable reuse
 * 
 * MONITORING AND OBSERVABILITY:
 * - Automatic integration with AWS CloudWatch for metrics and logs
 * - X-Ray tracing support for distributed system visibility
 * - Custom metrics and alarms for operational monitoring
 * - Error tracking and alerting capabilities
 */
func main() {
	lambda.Start(handler)
}
