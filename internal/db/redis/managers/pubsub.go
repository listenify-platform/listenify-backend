// Package redis provides Redis database connectivity and operations.
package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	r "github.com/go-redis/redis/v8"
	"norelock.dev/listenify/backend/internal/db/redis"
	"norelock.dev/listenify/backend/internal/utils"
)

const (
	// Channel prefixes
	GlobalChannelPrefix = "global"
	RoomChannelPrefix   = "room"
	UserChannelPrefix   = "user"
)

// MessageHandler is a function that handles a message from a channel
type MessageHandler func(channel string, payload []byte)

// PubSubManager handles Redis publish/subscribe operations
type PubSubManager struct {
	client     *redis.Client
	logger     *utils.Logger
	pubSub     *r.PubSub
	handlers   map[string][]MessageHandler
	mutex      sync.RWMutex
	ctx        context.Context
	cancelFunc context.CancelFunc
	running    bool
}

// NewPubSubManager creates a new PubSub manager
func NewPubSubManager(client *redis.Client) *PubSubManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &PubSubManager{
		client:     client,
		logger:     client.Logger(),
		handlers:   make(map[string][]MessageHandler),
		ctx:        ctx,
		cancelFunc: cancel,
		running:    false,
	}
}

// Subscribe subscribes to a channel and starts listening for messages
func (m *PubSubManager) Subscribe(channels ...string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// If already subscribed, close existing subscription
	if m.pubSub != nil {
		if err := m.pubSub.Close(); err != nil {
			m.logger.Error("Failed to close existing PubSub", err)
		}
	}

	// Create new PubSub
	m.pubSub = m.client.Client().Subscribe(m.ctx, channels...)

	// Start listener if not already running
	if !m.running {
		go m.messageListener()
		m.running = true
	}

	m.logger.Info("Subscribed to channels", "channels", channels)
	return nil
}

// Unsubscribe unsubscribes from channels
func (m *PubSubManager) Unsubscribe(channels ...string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.pubSub == nil {
		return nil
	}

	err := m.pubSub.Unsubscribe(m.ctx, channels...)
	if err != nil {
		m.logger.Error("Failed to unsubscribe from channels", err, "channels", channels)
		return err
	}

	m.logger.Info("Unsubscribed from channels", "channels", channels)
	return nil
}

// AddHandler adds a message handler for a channel
func (m *PubSubManager) AddHandler(channel string, handler MessageHandler) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, ok := m.handlers[channel]; !ok {
		m.handlers[channel] = make([]MessageHandler, 0)
	}

	m.handlers[channel] = append(m.handlers[channel], handler)
	m.logger.Debug("Added message handler", "channel", channel)
}

// RemoveAllHandlers removes all handlers for a channel
func (m *PubSubManager) RemoveAllHandlers(channel string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.handlers, channel)
	m.logger.Debug("Removed all message handlers", "channel", channel)
}

// Publish publishes a message to a channel
func (m *PubSubManager) Publish(ctx context.Context, channel string, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		m.logger.Error("Failed to marshal message for publish", err, "channel", channel)
		return err
	}

	err = m.client.Publish(ctx, channel, string(data))
	if err != nil {
		m.logger.Error("Failed to publish message", err, "channel", channel)
		return err
	}

	m.logger.Debug("Published message", "channel", channel, "size", len(data))
	return nil
}

// PublishGlobal publishes a message to the global channel
func (m *PubSubManager) PublishGlobal(ctx context.Context, eventType string, data any) error {
	message := map[string]any{
		"type":      eventType,
		"data":      data,
		"timestamp": time.Now(),
	}

	channel := redis.FormatKey(GlobalChannelPrefix, eventType)
	return m.Publish(ctx, channel, message)
}

// PublishToRoom publishes a message to a room channel
func (m *PubSubManager) PublishToRoom(ctx context.Context, roomID, eventType string, data any) error {
	message := map[string]any{
		"type":      eventType,
		"roomId":    roomID,
		"data":      data,
		"timestamp": time.Now(),
	}

	channel := redis.FormatKey(RoomChannelPrefix, roomID)
	return m.Publish(ctx, channel, message)
}

// PublishToUser publishes a message to a user channel
func (m *PubSubManager) PublishToUser(ctx context.Context, userID, eventType string, data any) error {
	message := map[string]any{
		"type":      eventType,
		"userId":    userID,
		"data":      data,
		"timestamp": time.Now(),
	}

	channel := redis.FormatKey(UserChannelPrefix, userID)
	return m.Publish(ctx, channel, message)
}

// Close stops the message listener and closes the subscription
func (m *PubSubManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Stop context
	m.cancelFunc()
	m.running = false

	// Close subscription if it exists
	if m.pubSub != nil {
		err := m.pubSub.Close()
		if err != nil {
			m.logger.Error("Failed to close PubSub", err)
			return err
		}
		m.pubSub = nil
	}

	m.logger.Info("Closed PubSub manager")
	return nil
}

// messageListener listens for messages and dispatches them to handlers
func (m *PubSubManager) messageListener() {
	m.logger.Info("Starting PubSub message listener")

	// Create a new context for this goroutine
	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	channel := m.pubSub.Channel()

	for {
		select {
		case msg, ok := <-channel:
			if !ok {
				m.logger.Warn("PubSub channel closed")
				return
			}

			m.handleMessage(msg.Channel, []byte(msg.Payload))

		case <-ctx.Done():
			m.logger.Info("PubSub message listener stopped")
			return
		}
	}
}

// handleMessage dispatches a message to the appropriate handlers
func (m *PubSubManager) handleMessage(channel string, payload []byte) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Call handlers for exact channel match
	if handlers, ok := m.handlers[channel]; ok {
		for _, handler := range handlers {
			go func(h MessageHandler) {
				defer func() {
					if r := recover(); r != nil {
						m.logger.Error("Panic in message handler", nil, "panic", r, "channel", channel)
					}
				}()

				h(channel, payload)
			}(handler)
		}
	}

	// Call handlers for wildcard channels
	// For example, if message is on "room:123" and there's a handler for "room:*"
	parts := splitChannelParts(channel)
	if len(parts) >= 2 {
		wildcardChannel := fmt.Sprintf("%s:*", parts[0])

		if handlers, ok := m.handlers[wildcardChannel]; ok {
			for _, handler := range handlers {
				go func(h MessageHandler) {
					defer func() {
						if r := recover(); r != nil {
							m.logger.Error("Panic in wildcard message handler", nil, "panic", r, "channel", channel)
						}
					}()

					h(channel, payload)
				}(handler)
			}
		}
	}
}

// splitChannelParts splits a channel name into parts by colon
func splitChannelParts(channel string) []string {
	var result []string
	var current string

	for i := range len(channel) {
		if channel[i] == ':' {
			result = append(result, current)
			current = ""
		} else {
			current += string(channel[i])
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

// FormatRoomChannel formats a channel name for a room
func FormatRoomChannel(roomID string) string {
	return redis.FormatKey(RoomChannelPrefix, roomID)
}

// FormatUserChannel formats a channel name for a user
func FormatUserChannel(userID string) string {
	return redis.FormatKey(UserChannelPrefix, userID)
}

// FormatGlobalChannel formats a channel name for global events
func FormatGlobalChannel(eventType string) string {
	return redis.FormatKey(GlobalChannelPrefix, eventType)
}
