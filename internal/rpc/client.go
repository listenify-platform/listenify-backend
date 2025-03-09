// Package rpc provides WebSocket-based RPC functionality.
package rpc

import (
	"bytes"
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
	mutex sync.Mutex

	// closed indicates whether the send channel has been closed
	closed bool
}

// safelySendMessage sends a message only if the channel isn't closed
func (c *Client) safelySendMessage(message []byte) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return false
	}

	c.send <- message
	return true
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
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
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
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message.
			n := len(c.send)
			for range n {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
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

	// Send the response
	if response != nil {
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
	delete(c.rooms, roomID)
	c.server.RemoveClientFromRoom(c, roomID)
	c.sendRoomMsg(roomID, &Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
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
