package websocket

import (
	"bytes"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512KB

	// Send buffer size
	sendBufferSize = 256
)

// Client represents a WebSocket client connection
type Client struct {
	id     string          // Unique connection ID
	userID string          // User ID from JWT
	hub    *Hub            // Reference to hub
	conn   *websocket.Conn // WebSocket connection
	send   chan []byte     // Buffered channel of outbound messages
	logger *zap.Logger
}

// NewClient creates a new WebSocket client
func NewClient(userID string, hub *Hub, conn *websocket.Conn, logger *zap.Logger) *Client {
	return &Client{
		id:     uuid.New().String(),
		userID: userID,
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, sendBufferSize),
		logger: logger.With(
			zap.String("userID", userID),
			zap.String("connectionID", uuid.New().String()),
		),
	}
}

// Start begins the client's read and write pumps
func (c *Client) Start() {
	// Register with hub
	c.hub.register <- c

	// Start goroutines for reading and writing
	go c.writePump()
	go c.readPump()

	// Send initial connection established message
	c.sendConnectionEstablished()
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
		c.logger.Info("Read pump stopped")
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("WebSocket read error", zap.Error(err))
			}
			break
		}

		// Handle different message types
		switch messageType {
		case websocket.TextMessage:
			c.handleTextMessage(message)
		case websocket.BinaryMessage:
			c.logger.Warn("Binary messages not supported")
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
		c.logger.Info("Write pump stopped")
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write message to WebSocket
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Error("Failed to write message", zap.Error(err))
				return
			}

			// Add queued messages to the current message batch
			n := len(c.send)
			for i := 0; i < n; i++ {
				if err := c.conn.WriteMessage(websocket.TextMessage, <-c.send); err != nil {
					c.logger.Error("Failed to write batched message", zap.Error(err))
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Error("Failed to send ping", zap.Error(err))
				return
			}
		}
	}
}

// handleTextMessage processes incoming text messages
func (c *Client) handleTextMessage(message []byte) {
	// Trim whitespace
	message = bytes.TrimSpace(message)

	// For now, we only handle pong responses
	// In the future, could handle client commands if needed
	if string(message) == `{"type":"pong"}` {
		c.logger.Debug("Received pong")
		return
	}

	// Log unexpected messages
	c.logger.Debug("Received message from client", zap.String("message", string(message)))
}

// sendConnectionEstablished sends an initial connection message
func (c *Client) sendConnectionEstablished() {
	message := fmt.Sprintf(`{
		"type": "CONNECTION_ESTABLISHED",
		"timestamp": %d,
		"data": {
			"connectionId": "%s",
			"userId": "%s",
			"message": "WebSocket connection established"
		}
	}`, time.Now().Unix(), c.id, c.userID)

	select {
	case c.send <- []byte(message):
		c.logger.Info("Sent connection established message")
	default:
		c.logger.Error("Failed to send connection established message")
	}
}

// GetID returns the client's connection ID
func (c *Client) GetID() string {
	return c.id
}

// GetUserID returns the client's user ID
func (c *Client) GetUserID() string {
	return c.userID
}