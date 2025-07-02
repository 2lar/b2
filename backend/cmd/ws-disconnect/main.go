/**
 * =============================================================================
 * WebSocket Disconnection Lambda - Connection Cleanup & Resource Management
 * =============================================================================
 * 
 * 📚 EDUCATIONAL OVERVIEW:
 * This Lambda function handles WebSocket disconnection events, implementing
 * proper connection cleanup and state management for the Brain2 real-time
 * communication system. It demonstrates database cleanup patterns, error
 * handling, and resource management in serverless environments.
 * 
 * 🏗️ KEY ARCHITECTURAL CONCEPTS:
 * 
 * 1. CONNECTION LIFECYCLE MANAGEMENT:
 *    - Automatic cleanup when clients disconnect
 *    - Resource cleanup for closed connections
 *    - State consistency maintenance in distributed system
 *    - Graceful handling of unexpected disconnections
 * 
 * 2. REVERSE LOOKUP PATTERNS:
 *    - Global Secondary Index for connection-to-user mapping
 *    - Efficient queries without full table scans
 *    - Single-table design with multiple access patterns
 *    - Performance-optimized database operations
 * 
 * 3. ERROR HANDLING & RESILIENCE:
 *    - Graceful handling of missing connections
 *    - Database operation error recovery
 *    - Logging for monitoring and debugging
 *    - Idempotent cleanup operations
 * 
 * 4. SERVERLESS RESOURCE CLEANUP:
 *    - Immediate cleanup on disconnect events
 *    - Prevention of resource leaks
 *    - Cost optimization through proper cleanup
 *    - State management without persistent servers
 * 
 * 5. DYNAMODB ADVANCED PATTERNS:
 *    - Global Secondary Index queries
 *    - Attribute value marshaling/unmarshaling
 *    - Conditional operations and error handling
 *    - Single-table design implementation
 * 
 * 🔄 DISCONNECTION WORKFLOW:
 * 1. Client disconnects (browser close, network issue, etc.)
 * 2. API Gateway triggers this Lambda function
 * 3. Lambda queries GSI to find user from connection ID
 * 4. Connection record deleted from DynamoDB
 * 5. Resources cleaned up and state updated
 * 
 * 🎯 LEARNING OBJECTIVES:
 * - WebSocket disconnection handling patterns
 * - DynamoDB Global Secondary Index usage
 * - Resource cleanup in serverless architectures
 * - Error handling for distributed systems
 * - Connection state management strategies
 * - Performance optimization for database queries
 */
package main

import (
	"context"  // Context for request lifecycle management
	"log"      // Structured logging for monitoring
	"net/http" // HTTP status codes for responses
	"os"       // Environment variable access

	// AWS Lambda and API Gateway integration
	"github.com/aws/aws-lambda-go/events" // WebSocket event structures
	"github.com/aws/aws-lambda-go/lambda" // Lambda runtime integration

	// AWS SDK v2 for modern, efficient AWS service integration
	"github.com/aws/aws-sdk-go-v2/aws"                            // Core AWS configuration
	awsConfig "github.com/aws/aws-sdk-go-v2/config"               // Configuration loading
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue" // Type conversion utilities
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"               // DynamoDB client
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"         // DynamoDB data types
)

/**
 * =============================================================================
 * Global Service Clients - Optimized for Lambda Performance
 * =============================================================================
 * 
 * LAMBDA PERFORMANCE OPTIMIZATION:
 * Global variables are initialized once per container and reused across
 * function invocations, providing significant performance benefits for
 * database connections and client initialization.
 */

// Global DynamoDB client for connection state operations
// REUSE PATTERN: Single client instance across all function invocations
// CONNECTION POOLING: AWS SDK manages efficient connection reuse
var dbClient *dynamodb.Client

// Global configuration for connections table name
// ENVIRONMENT CONFIG: Table name configured via CDK deployment
var connectionsTable string

// Global configuration for Global Secondary Index name
// GSI PATTERN: Enables reverse lookup from connection ID to user ID
// QUERY OPTIMIZATION: GSI allows efficient connection-based queries
var gsiName string

/**
 * Lambda Container Initialization - Configuration & Client Setup
 * 
 * This initialization function runs once per Lambda container lifecycle,
 * setting up all required clients and configuration for efficient
 * connection cleanup operations.
 * 
 * INITIALIZATION STRATEGY:
 * 1. Load environment configuration from CDK deployment
 * 2. Initialize AWS SDK with default credential chain
 * 3. Create reusable DynamoDB client for connection operations
 * 
 * PERFORMANCE BENEFITS:
 * - One-time client initialization reduces latency
 * - AWS SDK connection pooling optimizes database access
 * - Environment validation happens once per container
 * 
 * ERROR HANDLING:
 * - Fail fast if configuration is missing or invalid
 * - Clear error messages for troubleshooting deployment issues
 * - Container termination prevents partially initialized state
 */
func init() {
	// Step 1: Load Environment Configuration
	// These values are set by the CDK deployment process
	connectionsTable = os.Getenv("CONNECTIONS_TABLE_NAME")
	gsiName = os.Getenv("CONNECTIONS_GSI_NAME")
	
	// Step 2: AWS SDK Configuration
	// Default config chain: Lambda execution role → environment → metadata service
	awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Unable to load SDK config, %v", err)
	}
	
	// Step 3: DynamoDB Client Initialization
	// Optimized client for connection state management operations
	dbClient = dynamodb.NewFromConfig(awsCfg)
}

/**
 * WebSocket Disconnection Handler - Connection Cleanup and State Management
 * 
 * This function handles WebSocket disconnection events by cleaning up connection
 * state from DynamoDB. It demonstrates advanced DynamoDB patterns including
 * Global Secondary Index queries and proper error handling.
 * 
 * DISCONNECTION SCENARIOS:
 * - User closes browser/tab
 * - Network connection lost
 * - Application navigation
 * - Session timeout
 * - Server-side connection termination
 * 
 * CLEANUP WORKFLOW:
 * 1. Extract connection ID from disconnect event
 * 2. Query GSI to find associated user record
 * 3. Delete connection record from main table
 * 4. Handle errors gracefully for resilience
 * 
 * DATABASE DESIGN PATTERNS:
 * - Global Secondary Index for reverse lookups
 * - Single-table design with multiple access patterns
 * - Efficient cleanup without full table scans
 * - Error handling for missing or invalid connections
 * 
 * @param ctx Request context for timeout and cancellation
 * @param req WebSocket disconnection event from API Gateway
 * @return HTTP response indicating cleanup success or failure
 */
func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	
	// ==========================================================================
	// Step 1: Extract Connection Identity
	// ==========================================================================
	//
	// CONNECTION ID EXTRACTION:
	// API Gateway provides unique connection ID in the request context
	// This ID was used to store the connection during the connect phase
	
	connectionID := req.RequestContext.ConnectionID
	// Format connection ID to match storage pattern from connect Lambda
	sk := "CONN#" + connectionID

	// ==========================================================================
	// Step 2: Global Secondary Index Query for Reverse Lookup
	// ==========================================================================
	//
	// REVERSE LOOKUP CHALLENGE:
	// We have the connection ID but need to find the associated user record
	// to perform the deletion. This requires querying the GSI where connection
	// ID is the partition key instead of the sort key.
	//
	// GSI DESIGN PATTERN:
	// Main table: USER#<userID> (PK) + CONN#<connectionID> (SK)
	// GSI:        CONN#<connectionID> (GSI1PK) + USER#<userID> (GSI1SK)
	//
	// QUERY BENEFITS:
	// - O(1) lookup time instead of O(n) table scan
	// - Efficient even with millions of connections
	// - Leverages DynamoDB's indexing capabilities
	// - Cost-effective compared to scanning operations
	
	result, err := dbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &connectionsTable,
		IndexName:              &gsiName,
		KeyConditionExpression: aws.String("GSI1PK = :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sk": &types.AttributeValueMemberS{Value: sk},
		},
	})

	// ==========================================================================
	// Step 3: Query Error Handling
	// ==========================================================================
	//
	// DISTRIBUTED SYSTEM RESILIENCE:
	// Database operations can fail for various reasons (network, permissions,
	// throttling, etc.). Proper error handling ensures system stability.
	
	if err != nil {
		log.Printf("ERROR: Failed to query GSI for disconnect: %v", err)
		// Return 500 to indicate server error during cleanup
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// ==========================================================================
	// Step 4: Handle Missing Connection Records
	// ==========================================================================
	//
	// IDEMPOTENT CLEANUP PATTERN:
	// Connection might already be cleaned up by TTL or previous disconnect.
	// This is not an error condition - return success for idempotent behavior.
	//
	// GRACEFUL DEGRADATION:
	// Missing connections are warnings, not errors. This handles cases like:
	// - Duplicate disconnect events
	// - TTL cleanup already occurred
	// - Connection never fully established
	
	if len(result.Items) == 0 {
		log.Printf("WARN: Connection ID %s not found for disconnect.", connectionID)
		// Return 200 OK - disconnect is effectively successful
		return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
	}

	// ==========================================================================
	// Step 5: Data Unmarshaling and Type Conversion
	// ==========================================================================
	//
	// DYNAMODB TYPE CONVERSION:
	// DynamoDB returns items as map[string]types.AttributeValue
	// AWS SDK provides utilities to convert to Go structs for easier handling
	//
	// STRUCT TAGS:
	// `dynamodbav` tags specify how struct fields map to DynamoDB attributes
	// This enables automatic conversion between Go types and DynamoDB types
	
	var item struct {
		PK string `dynamodbav:"PK"` // Partition key (USER#<userID>)
		SK string `dynamodbav:"SK"` // Sort key (CONN#<connectionID>)
	}
	
	// UNMARSHALING PATTERN:
	// Convert first (and only) result item to Go struct
	// attributevalue package handles type conversion automatically
	if err := attributevalue.UnmarshalMap(result.Items[0], &item); err != nil {
		log.Printf("ERROR: Failed to unmarshal item: %v", err)
		// Data format error - return 500 for server error
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// ==========================================================================
	// Step 6: Connection Record Deletion
	// ==========================================================================
	//
	// ATOMIC DELETION OPERATION:
	// Delete the connection record using primary key (PK + SK)
	// This removes the connection from the user's active connections list
	//
	// CONSISTENCY CONSIDERATIONS:
	// - DynamoDB eventually consistent reads
	// - Immediate consistency for writes
	// - Deletion is atomic and isolated
	// - No partial failure states possible
	
	_, err = dbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &connectionsTable,
		Key: map[string]types.AttributeValue{
			// Use primary key components for exact record deletion
			"PK": &types.AttributeValueMemberS{Value: item.PK},
			"SK": &types.AttributeValueMemberS{Value: item.SK},
		},
	})

	// ==========================================================================
	// Step 7: Deletion Error Handling and Response
	// ==========================================================================
	
	if err != nil {
		log.Printf("ERROR: Failed to delete connection: %v", err)
		// Database error during deletion - return 500
		return events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	// ==========================================================================
	// Step 8: Successful Cleanup Response
	// ==========================================================================
	//
	// SUCCESS RESPONSE:
	// Return 200 OK to indicate successful connection cleanup
	// No response body needed for disconnect operations
	// Logging provides audit trail for monitoring and debugging
	
	log.Printf("Successfully cleaned up connection %s", connectionID)
	return events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}

/**
 * Lambda Function Entry Point - WebSocket Disconnect Handler Registration
 * 
 * Registers the disconnect handler with the AWS Lambda runtime for processing
 * WebSocket disconnection events from API Gateway.
 * 
 * LAMBDA RUNTIME INTEGRATION:
 * - Automatic event parsing from API Gateway WebSocket
 * - Built-in error handling and response formatting
 * - Integration with AWS CloudWatch for logging and monitoring
 * - Scalable execution based on disconnection event volume
 * 
 * EVENT TRIGGER:
 * This function is automatically invoked when:
 * - Client closes WebSocket connection
 * - Network connection is lost
 * - Browser/application is closed
 * - Connection timeout occurs
 * - Server forces disconnection
 */
func main() {
	lambda.Start(handler)
}
