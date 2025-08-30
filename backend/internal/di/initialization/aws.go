package initialization

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"brain2-backend/internal/infrastructure/concurrency"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	awsDynamodb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	awsEventbridge "github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

// AWSClients holds the initialized AWS service clients
type AWSClients struct {
	DynamoDBClient    *awsDynamodb.Client
	EventBridgeClient *awsEventbridge.Client
}

// InitializeAWSClients sets up AWS service clients with optimized timeouts
func InitializeAWSClients() (*AWSClients, error) {
	log.Println("Initializing AWS clients...")
	startTime := time.Now()

	// Create context with timeout for AWS config loading
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create shared HTTP client optimized for Lambda
	// Detect environment to tune connection pool
	env := concurrency.DetectEnvironment()
	
	var transport *http.Transport
	if env == concurrency.EnvironmentLambda {
		// Lambda-optimized settings: fewer connections, longer idle time
		transport = &http.Transport{
			DisableKeepAlives:   false, // IMPORTANT: Reuse TCP connections on warm starts
			MaxIdleConns:        100,   // Higher limit for connection reuse
			MaxIdleConnsPerHost: 10,    // More connections per host for parallel requests
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}
	} else {
		// ECS/Local settings: more aggressive connection pooling
		transport = &http.Transport{
			DisableKeepAlives:   false,
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}
	}
	
	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	clients := &AWSClients{
		// DynamoDB client with optimized HTTP client
		DynamoDBClient: awsDynamodb.NewFromConfig(awsCfg, func(o *awsDynamodb.Options) {
			o.HTTPClient = httpClient
			o.RetryMaxAttempts = 3
			o.RetryMode = aws.RetryModeAdaptive
		}),
		// EventBridge client with optimized HTTP client
		EventBridgeClient: awsEventbridge.NewFromConfig(awsCfg, func(o *awsEventbridge.Options) {
			o.HTTPClient = httpClient
			o.RetryMaxAttempts = 3
		}),
	}

	log.Printf("AWS clients initialized in %v", time.Since(startTime))
	return clients, nil
}