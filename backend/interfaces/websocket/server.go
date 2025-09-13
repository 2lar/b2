package websocket

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"backend/pkg/auth"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Server represents the WebSocket server
type Server struct {
	hub        *Hub
	upgrader   websocket.Upgrader
	logger     *zap.Logger
	jwtService *auth.JWTService
}

// ServerConfig holds WebSocket server configuration
type ServerConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	CheckOrigin     func(r *http.Request) bool
	MaxConnections  int
}

// DefaultServerConfig returns default WebSocket server configuration
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// In production, implement proper origin checking
			// For now, allow all origins
			return true
		},
		MaxConnections: 10000,
	}
}

// NewServer creates a new WebSocket server
func NewServer(hub *Hub, jwtService *auth.JWTService, config *ServerConfig, logger *zap.Logger) *Server {
	if config == nil {
		config = DefaultServerConfig()
	}

	return &Server{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  config.ReadBufferSize,
			WriteBufferSize: config.WriteBufferSize,
			CheckOrigin:     config.CheckOrigin,
		},
		logger:     logger,
		jwtService: jwtService,
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract and validate JWT token
	userID, err := s.authenticateRequest(r)
	if err != nil {
		s.logger.Error("WebSocket authentication failed",
			zap.Error(err),
			zap.String("remoteAddr", r.RemoteAddr),
		)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Check connection limit for user
	if s.hub.GetConnectionCount(userID) >= 10 {
		s.logger.Warn("Connection limit exceeded for user",
			zap.String("userID", userID),
			zap.Int("currentConnections", s.hub.GetConnectionCount(userID)),
		)
		http.Error(w, "Connection limit exceeded", http.StatusTooManyRequests)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade connection",
			zap.Error(err),
			zap.String("remoteAddr", r.RemoteAddr),
		)
		return
	}

	// Create new client
	client := NewClient(userID, s.hub, conn, s.logger)

	// Start client
	client.Start()

	s.logger.Info("New WebSocket connection established",
		zap.String("userID", userID),
		zap.String("connectionID", client.GetID()),
		zap.String("remoteAddr", r.RemoteAddr),
	)
}

// authenticateRequest validates the JWT token from the request
func (s *Server) authenticateRequest(r *http.Request) (string, error) {
	// Try to get token from query parameter first (for WebSocket)
	token := r.URL.Query().Get("token")

	// If not in query, try Authorization header
	if token == "" {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// Remove "Bearer " prefix if present
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	// If still no token, check cookie
	if token == "" {
		cookie, err := r.Cookie("auth_token")
		if err == nil {
			token = cookie.Value
		}
	}

	if token == "" {
		return "", errors.New("no authentication token provided")
	}

	// Validate token and extract claims
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	// Extract user ID from claims
	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return "", errors.New("user_id not found in token claims")
	}

	return userID, nil
}

// Start starts the WebSocket server on the specified address
func (s *Server) Start(address string) error {
	// Start the hub
	go s.hub.Run()

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.HandleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)

	server := &http.Server{
		Addr:         address,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("Starting WebSocket server", zap.String("address", address))

	// Start server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("WebSocket server error: %w", err)
	}

	return nil
}

// StartWithContext starts the WebSocket server with context for graceful shutdown
func (s *Server) StartWithContext(ctx context.Context, address string) error {
	// Start the hub
	go s.hub.Run()

	// Set up HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.HandleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/metrics", s.handleMetrics)

	server := &http.Server{
		Addr:         address,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for errors
	serverErr := make(chan error, 1)

	// Start server in goroutine
	go func() {
		s.logger.Info("Starting WebSocket server", zap.String("address", address))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.logger.Info("Shutting down WebSocket server")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Shutdown the HTTP server
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("WebSocket server shutdown error: %w", err)
		}

		// Stop the hub
		s.hub.Stop()

		s.logger.Info("WebSocket server stopped gracefully")
		return nil

	case err := <-serverErr:
		return fmt.Errorf("WebSocket server error: %w", err)
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status":"healthy","service":"websocket"}`)
}

// handleMetrics handles metrics requests
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := s.hub.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{
		"activeConnections": %d,
		"messagesSent": %d,
		"messagesFailed": %d
	}`, metrics.ActiveConnections, metrics.MessagesSent, metrics.MessagesFailed)
}

// GetHub returns the WebSocket hub
func (s *Server) GetHub() *Hub {
	return s.hub
}