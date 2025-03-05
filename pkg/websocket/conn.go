// Package websocket provides WebSocket connection handling.
package websocket

import (
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection errors
var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrWriteTimeout     = errors.New("write timeout")
	ErrReadTimeout      = errors.New("read timeout")
)

// MessageType represents the type of a WebSocket message.
type MessageType int

// Message types
const (
	TextMessage   = MessageType(websocket.TextMessage)
	BinaryMessage = MessageType(websocket.BinaryMessage)
	CloseMessage  = MessageType(websocket.CloseMessage)
	PingMessage   = MessageType(websocket.PingMessage)
	PongMessage   = MessageType(websocket.PongMessage)
)

// Connection wraps a WebSocket connection with additional functionality.
type Connection struct {
	// conn is the underlying WebSocket connection.
	conn *websocket.Conn

	// sendMutex is used to synchronize writes to the connection.
	sendMutex sync.Mutex

	// readMutex is used to synchronize reads from the connection.
	readMutex sync.Mutex

	// closed indicates whether the connection is closed.
	closed bool

	// closeMutex is used to synchronize access to the closed flag.
	closeMutex sync.RWMutex
}

// NewConnection creates a new connection.
func NewConnection(conn *websocket.Conn) *Connection {
	return &Connection{
		conn: conn,
	}
}

// ReadMessage reads a message from the connection.
func (c *Connection) ReadMessage() (MessageType, []byte, error) {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	if c.IsClosed() {
		return 0, nil, ErrConnectionClosed
	}

	messageType, message, err := c.conn.ReadMessage()
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			c.Close()
			return 0, nil, ErrConnectionClosed
		}
		return 0, nil, err
	}

	return MessageType(messageType), message, nil
}

// ReadJSON reads a JSON message from the connection.
func (c *Connection) ReadJSON(v any) error {
	c.readMutex.Lock()
	defer c.readMutex.Unlock()

	if c.IsClosed() {
		return ErrConnectionClosed
	}

	err := c.conn.ReadJSON(v)
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			c.Close()
			return ErrConnectionClosed
		}
		return err
	}

	return nil
}

// WriteMessage writes a message to the connection.
func (c *Connection) WriteMessage(messageType MessageType, data []byte) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	if c.IsClosed() {
		return ErrConnectionClosed
	}

	err := c.conn.WriteMessage(int(messageType), data)
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			c.Close()
			return ErrConnectionClosed
		}
		return err
	}

	return nil
}

// WriteJSON writes a JSON message to the connection.
func (c *Connection) WriteJSON(v any) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	if c.IsClosed() {
		return ErrConnectionClosed
	}

	err := c.conn.WriteJSON(v)
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			c.Close()
			return ErrConnectionClosed
		}
		return err
	}

	return nil
}

// WriteWithTimeout writes a message to the connection with a timeout.
func (c *Connection) WriteWithTimeout(messageType MessageType, data []byte, timeout time.Duration) error {
	c.sendMutex.Lock()
	defer c.sendMutex.Unlock()

	if c.IsClosed() {
		return ErrConnectionClosed
	}

	err := c.conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		return err
	}
	defer c.conn.SetWriteDeadline(time.Time{})

	err = c.conn.WriteMessage(int(messageType), data)
	if err != nil {
		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			c.Close()
			return ErrConnectionClosed
		}
		if errors.Is(err, websocket.ErrCloseSent) {
			return ErrConnectionClosed
		}
		return err
	}

	return nil
}

// WriteJSONWithTimeout writes a JSON message to the connection with a timeout.
func (c *Connection) WriteJSONWithTimeout(v any, timeout time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return c.WriteWithTimeout(TextMessage, data, timeout)
}

// Close closes the connection.
func (c *Connection) Close() error {
	c.closeMutex.Lock()
	defer c.closeMutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return c.conn.Close()
}

// IsClosed returns whether the connection is closed.
func (c *Connection) IsClosed() bool {
	c.closeMutex.RLock()
	defer c.closeMutex.RUnlock()
	return c.closed
}

// SetReadDeadline sets the deadline for future Read calls.
func (c *Connection) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls.
func (c *Connection) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

// SetPingHandler sets the handler for ping messages.
func (c *Connection) SetPingHandler(h func(string) error) {
	c.conn.SetPingHandler(h)
}

// SetPongHandler sets the handler for pong messages.
func (c *Connection) SetPongHandler(h func(string) error) {
	c.conn.SetPongHandler(h)
}

// SetCloseHandler sets the handler for close messages.
func (c *Connection) SetCloseHandler(h func(int, string) error) {
	c.conn.SetCloseHandler(h)
}

// EnableWriteCompression enables or disables write compression.
func (c *Connection) EnableWriteCompression(enable bool) {
	c.conn.EnableWriteCompression(enable)
}

// SetCompressionLevel sets the compression level.
func (c *Connection) SetCompressionLevel(level int) error {
	return c.conn.SetCompressionLevel(level)
}

// Subprotocol returns the negotiated protocol.
func (c *Connection) Subprotocol() string {
	return c.conn.Subprotocol()
}

// RemoteAddr returns the remote network address.
func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// LocalAddr returns the local network address.
func (c *Connection) LocalAddr() string {
	return c.conn.LocalAddr().String()
}
