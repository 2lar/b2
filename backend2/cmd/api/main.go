package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend2/infrastructure/config"
	"backend2/infrastructure/di"
	"backend2/interfaces/http/rest"

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
	
	// Create router
	router := rest.NewRouter(
		container.CommandBus,
		container.QueryBus,
		container.Logger,
	)
	
	// Setup routes
	handler := router.Setup()
	
	// Create HTTP server
	srv := &http.Server{
		Addr:         cfg.ServerAddress,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		container.Logger.Info("Starting server", 
			zap.String("address", cfg.ServerAddress),
			zap.String("environment", cfg.Environment),
		)
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			container.Logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	container.Logger.Info("Shutting down server...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		container.Logger.Error("Server shutdown error", zap.Error(err))
	}

	// Clean up resources
	if err := container.Logger.Sync(); err != nil {
		log.Printf("Failed to sync logger: %v", err)
	}

	log.Println("Server stopped")
}