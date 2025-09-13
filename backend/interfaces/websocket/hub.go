package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Hub maintains active WebSocket connections and broadcasts messages to users
type Hub struct {
	// User connections - one user can have multiple connections
	connections map[string]map[*Client]bool // userID -> set of clients
	mu          sync.RWMutex

	// Channels for client management
	register   chan *Client
	unregister chan *Client

	// Message broadcasting
	broadcast chan *BroadcastMessage

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger

	// Metrics
	metrics *HubMetrics
}

// HubMetrics tracks WebSocket metrics
type HubMetrics struct {
	ActiveConnections int64
	MessagesSent      int64
	MessagesFailed    int64
	mu                sync.RWMutex
}

// BroadcastMessage represents a message to be sent to specific users
type BroadcastMessage struct {
	UserID  string          `json:"-"`        // Target user ID
	Type    string          `json:"type"`     // Message type (e.g., NODE_CREATED)
	Data    json.RawMessage `json:"data"`     // Event data
	Timestamp int64         `json:"timestamp"` // Unix timestamp
}

// NewHub creates a new WebSocket hub
func NewHub(logger *zap.Logger) *Hub {
	ctx, cancel := context.WithCancel(context.Background())

	return &Hub{
		connections: make(map[string]map[*Client]bool),
		register:    make(chan *Client, 100),
		unregister:  make(chan *Client, 100),
		broadcast:   make(chan *BroadcastMessage, 1000),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		metrics:     &HubMetrics{},
	}
}

// Run starts the hub's main event loop
func (h *Hub) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			h.logger.Info("Hub shutting down")
			h.closeAllConnections()
			return

		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastToUser(message)

		case <-ticker.C:
			h.performHealthCheck()
		}
	}
}

// Stop gracefully shuts down the hub
func (h *Hub) Stop() {
	h.logger.Info("Stopping WebSocket hub")
	h.cancel()
}

// SendToUser sends a message to all connections of a specific user
func (h *Hub) SendToUser(userID string, messageType string, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	message := &BroadcastMessage{
		UserID:    userID,
		Type:      messageType,
		Data:      jsonData,
		Timestamp: time.Now().Unix(),
	}

	select {
	case h.broadcast <- message:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("broadcast channel full, message dropped")
	}
}

// registerClient adds a new client connection
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.connections[client.userID] == nil {
		h.connections[client.userID] = make(map[*Client]bool)
	}
	h.connections[client.userID][client] = true

	h.metrics.mu.Lock()
	h.metrics.ActiveConnections++
	h.metrics.mu.Unlock()

	h.logger.Info("Client registered",
		zap.String("userID", client.userID),
		zap.String("connectionID", client.id),
		zap.Int("userConnections", len(h.connections[client.userID])),
	)
}

// unregisterClient removes a client connection
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.connections[client.userID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			close(client.send)

			// Remove user entry if no more connections
			if len(clients) == 0 {
				delete(h.connections, client.userID)
			}

			h.metrics.mu.Lock()
			h.metrics.ActiveConnections--
			h.metrics.mu.Unlock()

			h.logger.Info("Client unregistered",
				zap.String("userID", client.userID),
				zap.String("connectionID", client.id),
				zap.Int("remainingConnections", len(clients)),
			)
		}
	}
}

// broadcastToUser sends a message to all connections of a user
func (h *Hub) broadcastToUser(message *BroadcastMessage) {
	h.mu.RLock()
	clients := h.connections[message.UserID]
	h.mu.RUnlock()

	if len(clients) == 0 {
		h.logger.Debug("No active connections for user",
			zap.String("userID", message.UserID),
			zap.String("messageType", message.Type),
		)
		return
	}

	// Marshal once for all clients
	data, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("Failed to marshal broadcast message",
			zap.Error(err),
			zap.String("messageType", message.Type),
		)
		return
	}

	successCount := 0
	failCount := 0

	for client := range clients {
		select {
		case client.send <- data:
			successCount++
			h.metrics.mu.Lock()
			h.metrics.MessagesSent++
			h.metrics.mu.Unlock()
		default:
			// Client's send channel is full, close it
			failCount++
			h.metrics.mu.Lock()
			h.metrics.MessagesFailed++
			h.metrics.mu.Unlock()

			h.logger.Warn("Closing slow client",
				zap.String("userID", client.userID),
				zap.String("connectionID", client.id),
			)

			go func(c *Client) {
				c.hub.unregister <- c
				c.conn.Close()
			}(client)
		}
	}

	h.logger.Debug("Broadcast complete",
		zap.String("userID", message.UserID),
		zap.String("messageType", message.Type),
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
	)
}

// performHealthCheck pings all connections to check if they're alive
func (h *Hub) performHealthCheck() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalConnections := 0
	for userID, clients := range h.connections {
		totalConnections += len(clients)
		for client := range clients {
			select {
			case client.send <- []byte(`{"type":"ping"}`):
				// Ping sent successfully
			default:
				// Connection might be dead
				h.logger.Warn("Failed to ping client",
					zap.String("userID", userID),
					zap.String("connectionID", client.id),
				)
			}
		}
	}

	h.logger.Debug("Health check performed",
		zap.Int("totalConnections", totalConnections),
		zap.Int("totalUsers", len(h.connections)),
	)
}

// closeAllConnections closes all active connections during shutdown
func (h *Hub) closeAllConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for userID, clients := range h.connections {
		for client := range clients {
			close(client.send)
			client.conn.Close()
		}
		delete(h.connections, userID)
	}

	h.logger.Info("All connections closed")
}

// GetMetrics returns current hub metrics
func (h *Hub) GetMetrics() HubMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()
	return *h.metrics
}

// GetConnectionCount returns the number of active connections for a user
func (h *Hub) GetConnectionCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections[userID])
}