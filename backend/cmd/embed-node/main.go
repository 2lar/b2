// Package main implements the Lambda handler for async embedding generation.
// Triggered by EventBridge on node.created and node.content.updated events.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	awsevents "github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	"backend/application/ports"
	"backend/domain/core/valueobjects"
	"backend/domain/events"
	"backend/domain/services"
	"backend/infrastructure/config"
	"backend/infrastructure/di"
	"backend/infrastructure/embeddings"
	"go.uber.org/zap"
)

var (
	nodeRepo         ports.NodeRepository
	embeddingService services.EmbeddingService
	logger           *zap.Logger
	cfg              *config.Config
)

func init() {
	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	container, err := di.InitializeContainer(context.Background(), cfg)
	if err != nil {
		log.Fatalf("Failed to initialize dependency container: %v", err)
	}

	logger = container.Logger
	nodeRepo = container.NodeRepo

	embeddingService = embeddings.NewOpenAICompatibleService(
		&embeddings.OpenAICompatibleConfig{
			BaseURL:    cfg.Embedding.BaseURL,
			APIKey:     cfg.Embedding.APIKey,
			Model:      cfg.Embedding.Model,
			Dimensions: cfg.Embedding.Dimensions,
			BatchSize:  64,
			Timeout:    30 * time.Second,
		},
		logger,
	)

	log.Println("Embed-node handler initialized successfully")
}

// nodeEventDetail is the minimal event payload needed to extract the node ID.
type nodeEventDetail struct {
	NodeID  string `json:"node_id"`
	UserID  string `json:"user_id"`
	GraphID string `json:"graph_id"`
}

func handler(ctx context.Context, event json.RawMessage) error {
	if !cfg.Embedding.Enabled {
		logger.Debug("Embedding generation is disabled, skipping")
		return nil
	}

	var cloudWatchEvent awsevents.CloudWatchEvent
	if err := json.Unmarshal(event, &cloudWatchEvent); err != nil {
		return fmt.Errorf("failed to parse event: %w", err)
	}

	// Only process node creation and content update events
	switch cloudWatchEvent.DetailType {
	case events.TypeNodeCreated, events.TypeNodeCreatedWithPending, events.TypeNodeUpdated:
		// proceed
	default:
		logger.Debug("Ignoring event type", zap.String("type", cloudWatchEvent.DetailType))
		return nil
	}

	var detail nodeEventDetail
	if err := json.Unmarshal(cloudWatchEvent.Detail, &detail); err != nil {
		return fmt.Errorf("failed to parse event detail: %w", err)
	}

	if detail.NodeID == "" {
		return fmt.Errorf("node_id is required in event detail")
	}

	return embedNode(ctx, detail.NodeID)
}

func embedNode(ctx context.Context, nodeID string) error {
	id, err := valueobjects.NewNodeIDFromString(nodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID %s: %w", nodeID, err)
	}

	node, err := nodeRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load node %s: %w", nodeID, err)
	}

	// Build text for embedding: title + body
	content := node.Content()
	text := content.Title()
	if body := content.Body(); body != "" {
		text += "\n" + body
	}

	if text == "" {
		logger.Warn("Node has no content to embed", zap.String("nodeID", nodeID))
		return nil
	}

	embedding, err := embeddingService.GenerateEmbedding(ctx, text)
	if err != nil {
		return fmt.Errorf("failed to generate embedding for node %s: %w", nodeID, err)
	}

	node.SetEmbedding(embedding)

	if err := nodeRepo.Save(ctx, node); err != nil {
		return fmt.Errorf("failed to save node %s with embedding: %w", nodeID, err)
	}

	logger.Info("Embedded node",
		zap.String("nodeID", nodeID),
		zap.Int("dimensions", embedding.Dimensions()),
	)

	return nil
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		log.Println("Starting embed-node Lambda")
		lambda.Start(handler)
	} else {
		if len(os.Args) > 1 && os.Args[1] == "backfill" {
			runBackfill(context.Background())
		} else {
			log.Println("Usage: embed-node backfill")
			log.Println("  Generates embeddings for all nodes that don't have one yet.")
		}
	}
}

func runBackfill(ctx context.Context) {
	if !cfg.Embedding.Enabled {
		log.Fatal("Embedding is disabled. Set EMBEDDING_ENABLED=true to run backfill.")
	}

	log.Println("Starting embedding backfill...")

	// Use Search with empty criteria to get all nodes
	nodes, err := nodeRepo.Search(ctx, ports.SearchCriteria{
		Limit: 10000,
	})
	if err != nil {
		log.Fatalf("Failed to load nodes: %v", err)
	}

	total := len(nodes)
	embedded := 0
	skipped := 0
	failed := 0

	log.Printf("Found %d nodes to process", total)

	for i, node := range nodes {
		if node.HasEmbedding() {
			skipped++
			continue
		}

		if err := embedNode(ctx, node.ID().String()); err != nil {
			logger.Error("Failed to embed node",
				zap.String("nodeID", node.ID().String()),
				zap.Error(err),
			)
			failed++
			continue
		}

		embedded++

		if (i+1)%50 == 0 {
			log.Printf("Progress: %d/%d (embedded: %d, skipped: %d, failed: %d)",
				i+1, total, embedded, skipped, failed)
		}
	}

	log.Printf("Backfill complete: %d embedded, %d skipped (already had embedding), %d failed out of %d total",
		embedded, skipped, failed, total)
}
