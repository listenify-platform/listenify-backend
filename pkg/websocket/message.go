// Package websocket provides WebSocket connection handling.
package websocket

import (
	"encoding/json"
	"errors"
	"time"
)

// Message errors
var (
	ErrInvalidMessage = errors.New("invalid message")
	ErrInvalidType    = errors.New("invalid message type")
)

// Message represents a WebSocket message.
type Message struct {
	// Type is the type of the message.
	Type MessageType `json:"type"`

	// Data is the message data.
	Data []byte `json:"data"`

	// Timestamp is the time the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// NewMessage creates a new message.
func NewMessage(messageType MessageType, data []byte) *Message {
	return &Message{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// NewTextMessage creates a new text message.
func NewTextMessage(data string) *Message {
	return NewMessage(TextMessage, []byte(data))
}

// NewBinaryMessage creates a new binary message.
func NewBinaryMessage(data []byte) *Message {
	return NewMessage(BinaryMessage, data)
}

// NewJSONMessage creates a new JSON message.
func NewJSONMessage(v any) (*Message, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return NewMessage(TextMessage, data), nil
}

// String returns the message as a string.
func (m *Message) String() string {
	return string(m.Data)
}

// JSON unmarshals the message data into the provided value.
func (m *Message) JSON(v any) error {
	if m.Type != TextMessage {
		return ErrInvalidType
	}

	return json.Unmarshal(m.Data, v)
}

// Envelope represents a message envelope with metadata.
type Envelope struct {
	// ID is a unique identifier for the message.
	ID string `json:"id,omitempty"`

	// From is the sender of the message.
	From string `json:"from,omitempty"`

	// To is the recipient of the message.
	To string `json:"to,omitempty"`

	// Channel is the channel the message was sent on.
	Channel string `json:"channel,omitempty"`

	// Type is the type of the message.
	Type string `json:"type"`

	// Data is the message data.
	Data any `json:"data"`

	// Timestamp is the time the message was created.
	Timestamp time.Time `json:"timestamp"`
}

// NewEnvelope creates a new message envelope.
func NewEnvelope(messageType string, data any) *Envelope {
	return &Envelope{
		Type:      messageType,
		Data:      data,
		Timestamp: time.Now(),
	}
}

// WithID sets the message ID.
func (e *Envelope) WithID(id string) *Envelope {
	e.ID = id
	return e
}

// WithSender sets the message sender.
func (e *Envelope) WithSender(from string) *Envelope {
	e.From = from
	return e
}

// WithRecipient sets the message recipient.
func (e *Envelope) WithRecipient(to string) *Envelope {
	e.To = to
	return e
}

// WithChannel sets the message channel.
func (e *Envelope) WithChannel(channel string) *Envelope {
	e.Channel = channel
	return e
}

// JSON marshals the envelope to JSON.
func (e *Envelope) JSON() ([]byte, error) {
	return json.Marshal(e)
}

// String returns the envelope as a JSON string.
func (e *Envelope) String() string {
	data, err := e.JSON()
	if err != nil {
		return ""
	}
	return string(data)
}

// Send sends the envelope to the connection.
func (e *Envelope) Send(conn *Connection) error {
	data, err := e.JSON()
	if err != nil {
		return err
	}

	return conn.WriteMessage(TextMessage, data)
}

// SendWithTimeout sends the envelope to the connection with a timeout.
func (e *Envelope) SendWithTimeout(conn *Connection, timeout time.Duration) error {
	data, err := e.JSON()
	if err != nil {
		return err
	}

	return conn.WriteWithTimeout(TextMessage, data, timeout)
}
