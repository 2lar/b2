package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/infrastructure/config"
	"backend/infrastructure/di"
	"backend/infrastructure/messaging"
	"backend/infrastructure/messaging/eventbridge"

	"go.uber.org/zap"
)

func main() {
	// Initialize context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize dependency container
	container, err := di.InitializeContainer(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}

	// Wire event handlers
	err = di.WireEventHandlers(
		container.EventHandlerRegistry,
		container.OperationEventListener,
		container.GraphStatsProjection,
		container.Logger,
	)
	if err != nil {
		log.Fatalf("Failed to wire event handlers: %v", err)
	}

	// Create event dispatcher for processing events
	dispatcher := messaging.NewEventDispatcher(container.EventHandlerRegistry, container.Logger)

	// Set up local event dispatcher for EventBridge if available
	if eventBus, ok := container.EventBus.(*eventbridge.EventBridgePublisher); ok {
		eventBus.SetLocalDispatcher(dispatcher)
		container.Logger.Info("Local event dispatcher configured for worker")
	}

	// Start background workers
	container.Logger.Info("Starting worker service",
		zap.String("environment", cfg.Environment),
	)

	// Start event processing worker
	go startEventProcessor(ctx, dispatcher, container.Logger)

	// Start saga processor worker (if we had saga infrastructure)
	// go startSagaProcessor(ctx, container, container.Logger)

	// Start periodic cleanup worker
	go startCleanupWorker(ctx, container.Logger)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	container.Logger.Info("Shutting down worker service...")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Cancel the main context to stop all workers
	cancel()

	// Wait for graceful shutdown or timeout
	select {
	case <-shutdownCtx.Done():
		container.Logger.Warn("Worker shutdown timeout exceeded")
	case <-time.After(5 * time.Second):
		container.Logger.Info("All workers stopped gracefully")
	}

	// Clean up resources
	if err := container.Logger.Sync(); err != nil {
		log.Printf("Failed to sync logger: %v", err)
	}

	log.Println("Worker service stopped")
}

// startEventProcessor starts a background worker to process domain events
func startEventProcessor(ctx context.Context, dispatcher *messaging.EventDispatcher, logger *zap.Logger) {
	logger.Info("Starting event processor worker")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Event processor shutting down")
			return
		case <-ticker.C:
			// In a real implementation, this would:
			// 1. Poll for unprocessed events from a queue
			// 2. Process them using the dispatcher
			// 3. Mark them as processed

			// For now, just log that we're running
			logger.Debug("Event processor tick")
		}
	}
}

// startCleanupWorker starts a background worker for periodic cleanup tasks
func startCleanupWorker(ctx context.Context, logger *zap.Logger) {
	logger.Info("Starting cleanup worker")

	// Run cleanup every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Cleanup worker shutting down")
			return
		case <-ticker.C:
			logger.Info("Running periodic cleanup tasks")

			// In a real implementation, this would:
			// 1. Clean up expired sessions
			// 2. Archive old events
			// 3. Perform database maintenance
			// 4. Clean up temporary files

			logger.Debug("Cleanup cycle completed")
		}
	}
}