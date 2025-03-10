// Package rpc provides WebSocket-based RPC functionality.
package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"norelock.dev/listenify/backend/internal/utils"
)

// Client represents a WebSocket client connection.
type Client struct {
	// ID is a unique identifier for the client.
	ID string

	// UserID is the ID of the authenticated user.
	UserID string

	// Username is the username of the authenticated user.
	Username string

	// server is the WebSocket server that created this client.
	server *Server

	// conn is the WebSocket connection.
	conn *websocket.Conn

	// send is a channel of outbound messages.
	send chan []byte

	// rooms is a map of room IDs that the client is in.
	rooms map[string]bool

	// logger is the client's logger.
	logger *utils.Logger

	// mutex protects concurrent access to client properties
	mutex sync.RWMutex

	// closed indicates whether the send channel has been closed
	closed bool

	// connected indicates whether the client is currently connected
	connected bool

	// lastPing is the timestamp of the last ping received
	lastPing time.Time

	// done is a channel that is closed when the client is disconnected
	done chan struct{}
}

// NewClient creates a new client.
func NewClient(id, userID, username string, server *Server, conn *websocket.Conn, logger *utils.Logger) *Client {
	return &Client{
		ID:        id,
		UserID:    userID,
		Username:  username,
		server:    server,
		conn:      conn,
		send:      make(chan []byte, 64),
		rooms:     make(map[string]bool),
		logger:    logger,
		connected: true,
		lastPing:  time.Now(),
		done:      make(chan struct{}),
	}
}

// isConnected returns whether the client is currently connected.
func (c *Client) isConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connected && !c.closed && time.Since(c.lastPing) < pongWait
}

// disconnect marks the client as disconnected and ensures proper cleanup.
func (c *Client) disconnect() {
	c.mutex.Lock()
	if !c.connected {
		c.mutex.Unlock()
		return
	}
	c.connected = false
	c.mutex.Unlock()

	// Create context with timeout for cleanup operations
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Only cleanup if client is still connected
	if c.isConnected() {
		c.server.cleanupClientState(c)
	}

	// Close the done channel to signal disconnection
	close(c.done)

	// Trigger maintenance cleanup to catch any edge cases
	if err := c.server.maintenanceService.CleanupStaleClients(ctx); err != nil {
		c.logger.Error("Failed to run maintenance cleanup during disconnect", err)
	}

	c.logger.Info("Client disconnected and cleaned up", "userId", c.UserID)
}

// safelySendMessage sends a message only if the channel isn't closed
// Uses non-blocking send to prevent deadlocks if channel is full
func (c *Client) safelySendMessage(message []byte) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.closed {
		c.logger.Debug("Client send channel is closed", "clientID", c.ID)
		return false
	}

	// Use non-blocking send with select
	select {
	case c.send <- message:
		return true
	default:
		// Channel is full, log and return false to trigger unregistration
		c.logger.Warn("Client send channel is full, message dropped", "clientID", c.ID)
		return false
	}
}

// markAsClosed marks the client's send channel as closed
func (c *Client) markAsClosed() {
	c.mutex.Lock()
	c.closed = true
	c.mutex.Unlock()
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump() {
	defer func() {
		// Ensure cleanup happens before unregistering
		c.disconnect()
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.mutex.Lock()
		c.lastPing = time.Now()
		c.mutex.Unlock()
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("Unexpected close error", err)
			}
			break
		}

		message = bytes.TrimSpace(bytes.Replace(message, []byte{'\n'}, []byte{' '}, -1))
		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		// Ensure cleanup happens before unregistering
		c.disconnect()
		c.server.unregister <- c
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				c.logger.Error("Failed to get next writer", err, "clientID", c.ID)
				return
			}

			_, err = w.Write(message)
			if err != nil {
				c.logger.Error("Failed to write message", err, "clientID", c.ID)
				return
			}

			// Close the writer after writing the message
			if err := w.Close(); err != nil {
				c.logger.Error("Failed to close writer", err, "clientID", c.ID)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Error("Failed to write ping message", err, "clientID", c.ID)
				return
			}
		}
	}
}

// handleMessage processes incoming messages.
func (c *Client) handleMessage(message []byte) {
	// Parse the message as a JSON-RPC request
	var request Request
	if err := json.Unmarshal(message, &request); err != nil {
		c.logger.Error("Failed to parse message", err, "message", string(message))
		c.sendErrorResponse(request.ID, ErrParseError, "Invalid JSON")
		return
	}

	// Route the request to the appropriate handler
	response := c.server.router.Route(c, &request)

	// Only send response if client is still connected
	if response != nil && c.isConnected() {
		responseJSON, err := json.Marshal(response)
		if err != nil {
			c.logger.Error("Failed to marshal response", err, "response", response)
			c.sendErrorResponse(request.ID, ErrInternalError, "Failed to marshal response")
			return
		}
		c.safelySendMessage(responseJSON)
	}
}

// sendErrorResponse sends an error response to the client.
func (c *Client) sendErrorResponse(id any, code ErrorCode, message string) {
	response := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		c.logger.Error("Failed to marshal error response", err, "response", response)
		return
	}

	c.safelySendMessage(responseJSON)
}

// SendNotification sends a notification to the client.
func (c *Client) SendNotification(method string, params any) {
	notification := &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	notificationJSON, err := json.Marshal(notification)
	if err != nil {
		c.logger.Error("Failed to marshal notification", err, "notification", notification)
		return
	}

	c.safelySendMessage(notificationJSON)
}

// SendRoomNotification sends a notification to all clients in a room.
func (c *Client) SendRoomNotification(roomID string, method string, params any) {
	// Check if the client is in the room
	if !c.IsInRoom(roomID) {
		c.logger.Warn("Client not in room", "clientID", c.ID, "roomID", roomID)
		return
	}

	c.sendRoomMsg(roomID, &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}

func (c *Client) sendRoomMsg(roomID string, notify *Notification) {
	notifyJSON, err := json.Marshal(notify)
	if err != nil {
		c.logger.Error("Failed to marshal room message", err, "message", notify)
		return
	}

	c.server.BroadcastToRoom(roomID, notifyJSON)
}

// JoinRoom adds the client to a room.
func (c *Client) JoinRoom(roomID string, method string, params any) {
	c.rooms[roomID] = true
	c.sendRoomMsg(roomID, &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
	c.server.AddClientToRoom(c, roomID)
	c.logger.Debug("Client joined room", "clientID", c.ID, "roomID", roomID)
}

// LeaveRoom removes the client from a room.
func (c *Client) LeaveRoom(roomID string, method string, params any) {
	// Remove from local state first
	delete(c.rooms, roomID)

	// Send leave notification before removing from server
	c.sendRoomMsg(roomID, &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})

	// Remove from server and update room state
	c.server.RemoveClientFromRoom(c, roomID)

	// Run cleanup to ensure Redis state is updated
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.server.maintenanceService.CleanupStaleClients(ctx); err != nil {
		c.logger.Error("Failed to cleanup after leaving room", err, "roomID", roomID)
	}

	c.logger.Debug("Client left room", "clientID", c.ID, "roomID", roomID)
}

// IsInRoom checks if the client is in a room.
func (c *Client) IsInRoom(roomID string) bool {
	return c.rooms[roomID]
}

// GetRooms returns the rooms the client is in.
func (c *Client) GetRooms() []string {
	rooms := make([]string, 0, len(c.rooms))
	for room := range c.rooms {
		rooms = append(rooms, room)
	}
	return rooms
}
