// Package room provides functionality for managing rooms and their state.
package room

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
	"slices"
)

// SyncEvent represents a synchronization event type.
type SyncEvent string

const (
	// SyncEventPlay indicates playback has started.
	SyncEventPlay SyncEvent = "play"
	// SyncEventPause indicates playback has paused.
	SyncEventPause SyncEvent = "pause"
	// SyncEventSeek indicates a seek operation.
	SyncEventSeek SyncEvent = "seek"
	// SyncEventTrackChange indicates the current track has changed.
	SyncEventTrackChange SyncEvent = "track_change"
	// SyncEventVolumeChange indicates the volume has changed.
	SyncEventVolumeChange SyncEvent = "volume_change"
	// SyncEventQueueUpdate indicates the queue has been updated.
	SyncEventQueueUpdate SyncEvent = "queue_update"
	// SyncEventUserJoin indicates a user has joined.
	SyncEventUserJoin SyncEvent = "user_join"
	// SyncEventUserLeave indicates a user has left.
	SyncEventUserLeave SyncEvent = "user_leave"
	// SyncEventRoomUpdate indicates room settings have been updated.
	SyncEventRoomUpdate SyncEvent = "room_update"
)

// PlaybackState represents the current state of media playback.
type PlaybackState struct {
	// CurrentTime is the current playback position in seconds.
	CurrentTime float64 `json:"current_time"`
	// Duration is the total duration of the current track in seconds.
	Duration float64 `json:"duration"`
	// IsPlaying indicates whether playback is active.
	IsPlaying bool `json:"is_playing"`
	// CurrentTrack is the currently playing track.
	CurrentTrack *models.Media `json:"current_track"`
	// Volume is the current volume level (0-100).
	Volume int `json:"volume"`
	// LastUpdated is the timestamp of the last update.
	LastUpdated time.Time `json:"last_updated"`
}

// SyncMessage represents a synchronization message.
type SyncMessage struct {
	// Event is the type of synchronization event.
	Event SyncEvent `json:"event"`
	// RoomID is the ID of the room.
	RoomID string `json:"room_id"`
	// UserID is the ID of the user who triggered the event.
	UserID string `json:"user_id"`
	// Timestamp is the time the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// PlaybackState is the current playback state.
	PlaybackState *PlaybackState `json:"playback_state,omitempty"`
	// Data contains additional event-specific data.
	Data map[string]any `json:"data,omitempty"`
}

// SyncService manages real-time synchronization for rooms and media playback.
type SyncService struct {
	pubsub      *managers.PubSubManager
	roomState   *managers.RoomStateManager
	logger      *utils.Logger
	roomStates  map[string]*PlaybackState
	stateMutex  sync.RWMutex
	subscribers map[string][]chan *SyncMessage
	subMutex    sync.RWMutex
}

// NewSyncService creates a new synchronization service.
func NewSyncService(
	pubsub *managers.PubSubManager,
	roomState *managers.RoomStateManager,
	logger *utils.Logger,
) *SyncService {
	return &SyncService{
		pubsub:      pubsub,
		roomState:   roomState,
		logger:      logger.Named("sync_service"),
		roomStates:  make(map[string]*PlaybackState),
		subscribers: make(map[string][]chan *SyncMessage),
	}
}

// Start initializes the synchronization service.
func (s *SyncService) Start(ctx context.Context) error {
	s.logger.Info("Starting sync service")

	// Subscribe to sync events
	syncChannel := "room:sync:*"
	err := s.pubsub.Subscribe(syncChannel)
	if err != nil {
		return fmt.Errorf("failed to subscribe to sync channel: %w", err)
	}

	// Add message handler
	s.pubsub.AddHandler(syncChannel, func(channel string, payload []byte) {
		s.handleSyncMessage(ctx, string(payload))
	})

	return nil
}

// handleSyncMessage processes a synchronization message from Redis.
func (s *SyncService) handleSyncMessage(ctx context.Context, msg string) {
	var syncMsg SyncMessage
	if err := json.Unmarshal([]byte(msg), &syncMsg); err != nil {
		s.logger.Error("Failed to unmarshal sync message", err)
		return
	}

	// Update room state
	if syncMsg.PlaybackState != nil {
		s.updateRoomState(syncMsg.RoomID, syncMsg.PlaybackState)
	}

	// Notify subscribers
	s.notifySubscribers(syncMsg.RoomID, &syncMsg)
}

// updateRoomState updates the playback state for a room.
func (s *SyncService) updateRoomState(roomID string, state *PlaybackState) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	s.roomStates[roomID] = state

	// Persist state to Redis
	stateJSON, err := json.Marshal(state)
	if err != nil {
		s.logger.Error("Failed to marshal playback state", err)
		return
	}

	// Get current room state
	roomState, err := s.roomState.GetRoomState(context.Background(), roomID)
	if err != nil {
		s.logger.Error("Failed to get room state", err)
		return
	}

	if roomState == nil {
		s.logger.Error("Room state not found", fmt.Errorf("room not found: %s", roomID))
		return
	}

	// Update playback state in room data
	if roomState.Data == nil {
		roomState.Data = make(map[string]any)
	}
	roomState.Data["playback_state"] = json.RawMessage(stateJSON)

	// Update room state
	if err := s.roomState.UpdateRoomState(context.Background(), roomState); err != nil {
		s.logger.Error("Failed to persist playback state", err)
	}
}

// GetPlaybackState returns the current playback state for a room.
func (s *SyncService) GetPlaybackState(ctx context.Context, roomID string) (*PlaybackState, error) {
	// First check in-memory cache
	s.stateMutex.RLock()
	state, exists := s.roomStates[roomID]
	s.stateMutex.RUnlock()

	if exists {
		return state, nil
	}

	// If not in cache, try to load from Redis
	roomState, err := s.roomState.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to get playback state: %w", err)
	}

	if roomState == nil {
		// No state exists yet, create a default one
		state = &PlaybackState{
			CurrentTime: 0,
			Duration:    0,
			IsPlaying:   false,
			Volume:      100,
			LastUpdated: time.Now(),
		}
		return state, nil
	}

	// Check if playback state exists in room data
	if roomState.Data == nil || roomState.Data["playback_state"] == nil {
		// No playback state exists yet, create a default one
		state = &PlaybackState{
			CurrentTime: 0,
			Duration:    0,
			IsPlaying:   false,
			Volume:      100,
			LastUpdated: time.Now(),
		}
		return state, nil
	}

	// Parse state from JSON
	stateJSON, ok := roomState.Data["playback_state"].(json.RawMessage)
	if !ok {
		return nil, fmt.Errorf("invalid playback state format")
	}

	state = &PlaybackState{}
	if err := json.Unmarshal(stateJSON, state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal playback state: %w", err)
	}

	// Cache the state
	s.stateMutex.Lock()
	s.roomStates[roomID] = state
	s.stateMutex.Unlock()

	return state, nil
}

// UpdatePlaybackState updates the playback state for a room and notifies all clients.
func (s *SyncService) UpdatePlaybackState(ctx context.Context, roomID, userID string, state *PlaybackState, event SyncEvent) error {
	// Update the state
	state.LastUpdated = time.Now()
	s.updateRoomState(roomID, state)

	// Create sync message
	syncMsg := &SyncMessage{
		Event:         event,
		RoomID:        roomID,
		UserID:        userID,
		Timestamp:     time.Now(),
		PlaybackState: state,
	}

	// Publish to Redis
	msgJSON, err := json.Marshal(syncMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal sync message: %w", err)
	}

	if err := s.pubsub.Publish(ctx, fmt.Sprintf("room:sync:%s", roomID), msgJSON); err != nil {
		return fmt.Errorf("failed to publish sync message: %w", err)
	}

	return nil
}

// SubscribeToRoom subscribes to synchronization events for a room.
func (s *SyncService) SubscribeToRoom(roomID string) (<-chan *SyncMessage, func()) {
	s.subMutex.Lock()
	defer s.subMutex.Unlock()

	ch := make(chan *SyncMessage, 100)
	if _, exists := s.subscribers[roomID]; !exists {
		s.subscribers[roomID] = make([]chan *SyncMessage, 0)
	}

	s.subscribers[roomID] = append(s.subscribers[roomID], ch)

	// Return the channel and an unsubscribe function
	unsubscribe := func() {
		s.subMutex.Lock()
		defer s.subMutex.Unlock()

		subs := s.subscribers[roomID]
		for i, sub := range subs {
			if sub == ch {
				// Remove this subscriber
				s.subscribers[roomID] = slices.Delete(subs, i, i+1)
				close(ch)
				break
			}
		}

		// If no more subscribers for this room, remove the room entry
		if len(s.subscribers[roomID]) == 0 {
			delete(s.subscribers, roomID)
		}
	}

	return ch, unsubscribe
}

// notifySubscribers sends a sync message to all subscribers for a room.
func (s *SyncService) notifySubscribers(roomID string, msg *SyncMessage) {
	s.subMutex.RLock()
	defer s.subMutex.RUnlock()

	subs, exists := s.subscribers[roomID]
	if !exists {
		return
	}

	for _, ch := range subs {
		select {
		case ch <- msg:
			// Message sent successfully
		default:
			// Channel buffer is full, log warning
			s.logger.Warn("Subscriber channel is full, dropping message", "roomID", roomID)
		}
	}
}

// CalculateCurrentTime calculates the current playback time based on the last known state.
func (s *SyncService) CalculateCurrentTime(state *PlaybackState) float64 {
	if !state.IsPlaying {
		return state.CurrentTime
	}

	// Calculate elapsed time since last update
	elapsed := time.Since(state.LastUpdated).Seconds()
	currentTime := min(state.CurrentTime+elapsed, state.Duration)

	return currentTime
}

// SyncRoomState synchronizes the room state for a newly joined user.
func (s *SyncService) SyncRoomState(ctx context.Context, roomID, userID string) error {
	state, err := s.GetPlaybackState(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to get playback state: %w", err)
	}

	// Update current time based on elapsed time
	if state.IsPlaying {
		state.CurrentTime = s.CalculateCurrentTime(state)
		state.LastUpdated = time.Now()
	}

	// Create sync message
	syncMsg := &SyncMessage{
		Event:         SyncEventUserJoin,
		RoomID:        roomID,
		UserID:        userID,
		Timestamp:     time.Now(),
		PlaybackState: state,
		Data: map[string]any{
			"user_id": userID,
		},
	}

	// Publish to Redis
	msgJSON, err := json.Marshal(syncMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal sync message: %w", err)
	}

	if err := s.pubsub.Publish(ctx, fmt.Sprintf("room:sync:%s", roomID), msgJSON); err != nil {
		return fmt.Errorf("failed to publish sync message: %w", err)
	}

	return nil
}

// BroadcastRoomEvent broadcasts a room event to all users in a room.
func (s *SyncService) BroadcastRoomEvent(ctx context.Context, roomID, userID string, event SyncEvent, data map[string]any) error {
	state, err := s.GetPlaybackState(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to get playback state: %w", err)
	}

	// Create sync message
	syncMsg := &SyncMessage{
		Event:         event,
		RoomID:        roomID,
		UserID:        userID,
		Timestamp:     time.Now(),
		PlaybackState: state,
		Data:          data,
	}

	// Publish to Redis
	msgJSON, err := json.Marshal(syncMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal sync message: %w", err)
	}

	if err := s.pubsub.Publish(ctx, fmt.Sprintf("room:sync:%s", roomID), msgJSON); err != nil {
		return fmt.Errorf("failed to publish sync message: %w", err)
	}

	return nil
}
