// Package redis provides Redis database connectivity and operations.
package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	r "github.com/go-redis/redis/v8"
	"norelock.dev/listenify/backend/internal/db/redis"
)

const (
	// RoomStateKeyPrefix is the prefix for room state keys
	RoomStateKeyPrefix = "room:state"

	// RoomUsersKeyPrefix is the prefix for room users keys
	RoomUsersKeyPrefix = "room:users"

	// RoomQueueKeyPrefix is the prefix for room DJ queue keys
	RoomQueueKeyPrefix = "room:queue"

	// RoomMediaKeyPrefix is the prefix for room current media keys
	RoomMediaKeyPrefix = "room:media"

	// RoomVotesKeyPrefix is the prefix for room votes keys
	RoomVotesKeyPrefix = "room:votes"

	// RoomHistoryKeyPrefix is the prefix for room history keys
	RoomHistoryKeyPrefix = "room:history"

	// Default expiration times
	RoomStateExpiry     = 12 * time.Hour
	RoomInactiveExpiry  = 7 * 24 * time.Hour // 7 days
	RoomHistoryMaxItems = 50
)

// RoomState represents the real-time state of a room in Redis
type RoomState struct {
	// RoomID is the ID of the room
	RoomID string `json:"roomId"`

	// IsActive indicates whether the room is active
	IsActive bool `json:"isActive"`

	// ActiveUsers is the number of active users in the room
	ActiveUsers int `json:"activeUsers"`

	// CurrentDJ is the ID of the current DJ
	CurrentDJ string `json:"currentDj,omitempty"`

	// CurrentMedia is the ID of the current media
	CurrentMedia string `json:"currentMedia,omitempty"`

	// MediaStartTime is when the current media started playing
	MediaStartTime time.Time `json:"mediaStartTime"`

	// MediaEndTime is when the current media is expected to end
	MediaEndTime time.Time `json:"mediaEndTime"`

	// LastActivity is when the last activity happened in the room
	LastActivity time.Time `json:"lastActivity"`

	// Data contains additional state data
	Data map[string]any `json:"data,omitempty"`
}

// QueueEntry represents a user in the DJ queue
type QueueEntry struct {
	// UserID is the ID of the user
	UserID string `json:"userId"`

	// Position is the position in the queue
	Position int `json:"position"`

	// JoinTime is when the user joined the queue
	JoinTime time.Time `json:"joinTime"`

	// LastPlay is when the user last played a track
	LastPlay time.Time `json:"lastPlay,omitzero"`

	// PlayCount is the number of tracks the user has played
	PlayCount int `json:"playCount"`
}

// RoomStateManager handles Redis operations for room state
type RoomStateManager struct {
	client *redis.Client
}

// NewRoomStateManager creates a new room state manager
func NewRoomStateManager(client *redis.Client) *RoomStateManager {
	return &RoomStateManager{
		client: client,
	}
}

// InitRoom initializes a room's state in Redis
func (m *RoomStateManager) InitRoom(ctx context.Context, roomID string) error {
	logger := m.client.Logger()

	// Check if room state already exists
	stateKey := formatRoomStateKey(roomID)
	exists, err := m.client.Exists(ctx, stateKey)
	if err != nil {
		logger.Error("Failed to check if room state exists", err, "roomId", roomID)
		return err
	}

	if !exists {
		// Create initial room state
		state := &RoomState{
			RoomID:       roomID,
			IsActive:     true,
			ActiveUsers:  0,
			LastActivity: time.Now(),
			Data:         make(map[string]any),
		}

		// Store room state in Redis
		err = m.client.SetObject(ctx, stateKey, state, RoomStateExpiry)
		if err != nil {
			logger.Error("Failed to store room state in Redis", err, "roomId", roomID)
			return err
		}

		// Initialize empty users set
		usersKey := formatRoomUsersKey(roomID)
		_, err = m.client.SCard(ctx, usersKey)
		if err != nil && err != r.Nil {
			logger.Error("Failed to init room users set", err, "roomId", roomID)
			return err
		}

		// Initialize empty queue list
		queueKey := formatRoomQueueKey(roomID)
		_, err = m.client.LLen(ctx, queueKey)
		if err != nil && err != r.Nil {
			logger.Error("Failed to init room queue list", err, "roomId", roomID)
			return err
		}

		logger.Info("Initialized room state", "roomId", roomID)
	} else {
		// Update room to be active
		err = m.SetRoomActive(ctx, roomID, true)
		if err != nil {
			logger.Error("Failed to set room active", err, "roomId", roomID)
			return err
		}

		logger.Info("Updated existing room state", "roomId", roomID)
	}

	return nil
}

// GetRoomState gets a room's state from Redis
func (m *RoomStateManager) GetRoomState(ctx context.Context, roomID string) (*RoomState, error) {
	logger := m.client.Logger()

	// Get room state from Redis
	stateKey := formatRoomStateKey(roomID)
	var state RoomState
	err := m.client.GetObject(ctx, stateKey, &state)
	if err != nil {
		if err == r.Nil {
			logger.Debug("Room state not found", "roomId", roomID)
			return nil, nil
		}
		logger.Error("Failed to get room state from Redis", err, "roomId", roomID)
		return nil, err
	}

	return &state, nil
}

// UpdateRoomState updates a room's state in Redis
func (m *RoomStateManager) UpdateRoomState(ctx context.Context, state *RoomState) error {
	logger := m.client.Logger()

	// Update last activity time
	state.LastActivity = time.Now()

	// Store room state in Redis
	stateKey := formatRoomStateKey(state.RoomID)
	err := m.client.SetObject(ctx, stateKey, state, RoomStateExpiry)
	if err != nil {
		logger.Error("Failed to update room state in Redis", err, "roomId", state.RoomID)
		return err
	}

	logger.Debug("Updated room state", "roomId", state.RoomID)
	return nil
}

// SetRoomActive sets a room's active status
func (m *RoomStateManager) SetRoomActive(ctx context.Context, roomID string, isActive bool) error {
	logger := m.client.Logger()

	// Get current state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	if state == nil {
		// Room doesn't exist, initialize it
		return m.InitRoom(ctx, roomID)
	}

	// Update active status
	state.IsActive = isActive
	state.LastActivity = time.Now()

	// If deactivating, use longer expiry
	expiry := RoomStateExpiry
	if !isActive {
		expiry = RoomInactiveExpiry
	}

	// Store updated state
	stateKey := formatRoomStateKey(roomID)
	err = m.client.SetObject(ctx, stateKey, state, expiry)
	if err != nil {
		logger.Error("Failed to update room active status", err, "roomId", roomID)
		return err
	}

	logger.Info("Set room active status", "roomId", roomID, "isActive", isActive)
	return nil
}

// AddUserToRoom adds a user to a room
func (m *RoomStateManager) AddUserToRoom(ctx context.Context, roomID, userID string) error {
	logger := m.client.Logger()

	// Add user to room users set
	usersKey := formatRoomUsersKey(roomID)
	err := m.client.SAdd(ctx, usersKey, userID)
	if err != nil {
		logger.Error("Failed to add user to room", err, "roomId", roomID, "userId", userID)
		return err
	}

	// Update active users count in room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	if state == nil {
		// Room doesn't exist, initialize it
		err = m.InitRoom(ctx, roomID)
		if err != nil {
			return err
		}
		state, err = m.GetRoomState(ctx, roomID)
		if err != nil {
			return err
		}
	}

	// Get current user count
	userCount, err := m.client.SCard(ctx, usersKey)
	if err != nil {
		logger.Error("Failed to get room user count", err, "roomId", roomID)
		return err
	}

	// Update state with new user count
	state.ActiveUsers = int(userCount)
	err = m.UpdateRoomState(ctx, state)
	if err != nil {
		return err
	}

	logger.Info("Added user to room", "roomId", roomID, "userId", userID, "activeUsers", userCount)
	return nil
}

// RemoveUserFromRoom removes a user from a room
func (m *RoomStateManager) RemoveUserFromRoom(ctx context.Context, roomID, userID string) error {
	logger := m.client.Logger()

	// Remove user from room users set
	usersKey := formatRoomUsersKey(roomID)
	err := m.client.SRem(ctx, usersKey, userID)
	if err != nil {
		logger.Error("Failed to remove user from room", err, "roomId", roomID, "userId", userID)
		return err
	}

	// Update active users count in room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	if state == nil {
		// Room doesn't exist, nothing to do
		return nil
	}

	// Get current user count
	userCount, err := m.client.SCard(ctx, usersKey)
	if err != nil {
		logger.Error("Failed to get room user count", err, "roomId", roomID)
		return err
	}

	// Update state with new user count
	state.ActiveUsers = int(userCount)

	// If room is empty, handle cleanup
	if userCount == 0 {
		// Keep the state around but mark as inactive
		state.IsActive = false
		err = m.client.SetObject(ctx, formatRoomStateKey(roomID), state, RoomInactiveExpiry)
		if err != nil {
			logger.Error("Failed to update empty room state", err, "roomId", roomID)
			return err
		}

		logger.Info("Room is now empty", "roomId", roomID)
		return nil
	}

	// If the user was in the DJ queue, remove them
	m.RemoveUserFromQueue(ctx, roomID, userID)

	// If the user was the current DJ, advance to next DJ
	if state.CurrentDJ == userID {
		err = m.AdvanceDJ(ctx, roomID)
		if err != nil {
			logger.Error("Failed to advance DJ after user left", err, "roomId", roomID, "userId", userID)
			// Continue anyway, as we still want to update the state
		}
	}

	// Update state
	err = m.UpdateRoomState(ctx, state)
	if err != nil {
		return err
	}

	logger.Info("Removed user from room", "roomId", roomID, "userId", userID, "activeUsers", userCount)
	return nil
}

// GetRoomUsers gets all users in a room
func (m *RoomStateManager) GetRoomUsers(ctx context.Context, roomID string) ([]string, error) {
	logger := m.client.Logger()

	// Get users from room users set
	usersKey := formatRoomUsersKey(roomID)
	users, err := m.client.SMembers(ctx, usersKey)
	if err != nil {
		logger.Error("Failed to get room users", err, "roomId", roomID)
		return nil, err
	}

	return users, nil
}

// IsUserInRoom checks if a user is in a room
func (m *RoomStateManager) IsUserInRoom(ctx context.Context, roomID, userID string) (bool, error) {
	logger := m.client.Logger()

	// Check if user is in room users set
	usersKey := formatRoomUsersKey(roomID)
	isMember, err := m.client.SIsMember(ctx, usersKey, userID)
	if err != nil {
		logger.Error("Failed to check if user is in room", err, "roomId", roomID, "userId", userID)
		return false, err
	}

	return isMember, nil
}

// AddUserToQueue adds a user to the DJ queue
func (m *RoomStateManager) AddUserToQueue(ctx context.Context, roomID, userID string) error {
	logger := m.client.Logger()

	// Check if room exists
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	if state == nil {
		return fmt.Errorf("room not found: %s", roomID)
	}

	// Check if user is in the room
	inRoom, err := m.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		return err
	}

	if !inRoom {
		return fmt.Errorf("user is not in the room: %s", userID)
	}

	// Check if user is already in queue
	queueKey := formatRoomQueueKey(roomID)
	queueEntries, err := m.GetQueueEntries(ctx, roomID)
	if err != nil {
		return err
	}

	for _, entry := range queueEntries {
		if entry.UserID == userID {
			logger.Debug("User already in queue", "roomId", roomID, "userId", userID)
			return nil // Already in queue
		}
	}

	// Create new queue entry
	entry := QueueEntry{
		UserID:    userID,
		Position:  len(queueEntries),
		JoinTime:  time.Now(),
		PlayCount: 0,
	}

	// Add to queue
	entryJson, err := json.Marshal(entry)
	if err != nil {
		logger.Error("Failed to marshal queue entry", err, "roomId", roomID, "userId", userID)
		return err
	}

	err = m.client.RPush(ctx, queueKey, string(entryJson))
	if err != nil {
		logger.Error("Failed to add user to queue", err, "roomId", roomID, "userId", userID)
		return err
	}

	// If no current DJ, make this user the current DJ
	if state.CurrentDJ == "" {
		err = m.AdvanceDJ(ctx, roomID)
		if err != nil {
			logger.Error("Failed to advance DJ after adding first user", err, "roomId", roomID)
			// Continue anyway as the user was successfully added to the queue
		}
	}

	logger.Info("Added user to DJ queue", "roomId", roomID, "userId", userID, "position", entry.Position)
	return nil
}

// RemoveUserFromQueue removes a user from the DJ queue
func (m *RoomStateManager) RemoveUserFromQueue(ctx context.Context, roomID, userID string) error {
	logger := m.client.Logger()

	// Get current queue
	queueEntries, err := m.GetQueueEntries(ctx, roomID)
	if err != nil {
		return err
	}

	// Check if user is in queue
	userFound := false
	userPosition := -1
	for i, entry := range queueEntries {
		if entry.UserID == userID {
			userFound = true
			userPosition = i
			break
		}
	}

	if !userFound {
		logger.Debug("User not in queue", "roomId", roomID, "userId", userID)
		return nil // Not in queue, nothing to do
	}

	// Remove from queue and update positions
	queueKey := formatRoomQueueKey(roomID)

	// Clear existing queue
	err = m.client.Del(ctx, queueKey)
	if err != nil {
		logger.Error("Failed to clear queue", err, "roomId", roomID)
		return err
	}

	// Rebuild queue without the user and update positions
	for i, entry := range queueEntries {
		if entry.UserID != userID {
			// Update position if necessary
			if i > userPosition {
				entry.Position = i - 1
			}

			entryJson, err := json.Marshal(entry)
			if err != nil {
				logger.Error("Failed to marshal queue entry", err, "roomId", roomID, "userId", entry.UserID)
				continue
			}

			err = m.client.RPush(ctx, queueKey, string(entryJson))
			if err != nil {
				logger.Error("Failed to add entry back to queue", err, "roomId", roomID, "userId", entry.UserID)
				continue
			}
		}
	}

	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	// If user was the current DJ, advance to next DJ
	if state != nil && state.CurrentDJ == userID {
		err = m.AdvanceDJ(ctx, roomID)
		if err != nil {
			logger.Error("Failed to advance DJ after removing current DJ", err, "roomId", roomID)
			// Continue anyway as the user was successfully removed from the queue
		}
	}

	logger.Info("Removed user from DJ queue", "roomId", roomID, "userId", userID)
	return nil
}

// AdvanceDJ advances to the next DJ in the queue
func (m *RoomStateManager) AdvanceDJ(ctx context.Context, roomID string) error {
	logger := m.client.Logger()

	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	if state == nil {
		return fmt.Errorf("room not found: %s", roomID)
	}

	// Get current queue
	queueEntries, err := m.GetQueueEntries(ctx, roomID)
	if err != nil {
		return err
	}

	// If queue is empty, clear current DJ and media
	if len(queueEntries) == 0 {
		state.CurrentDJ = ""
		state.CurrentMedia = ""
		state.MediaStartTime = time.Time{}
		state.MediaEndTime = time.Time{}

		err = m.UpdateRoomState(ctx, state)
		if err != nil {
			return err
		}

		logger.Info("No DJs in queue, cleared current DJ", "roomId", roomID)
		return nil
	}

	// Get the current position of previous DJ
	previousDJPosition := -1
	if state.CurrentDJ != "" {
		for i, entry := range queueEntries {
			if entry.UserID == state.CurrentDJ {
				previousDJPosition = i
				break
			}
		}
	}

	// Determine next DJ
	var nextDJ *QueueEntry
	queueKey := formatRoomQueueKey(roomID)

	if previousDJPosition == -1 || previousDJPosition >= len(queueEntries)-1 {
		// Start from beginning of queue
		nextDJ = &queueEntries[0]
	} else {
		// Take next DJ in queue
		nextDJ = &queueEntries[previousDJPosition+1]
	}

	// Update next DJ's play count and time
	nextDJ.PlayCount++
	nextDJ.LastPlay = time.Now()

	// Update in queue
	queueEntries = updateQueueEntry(queueEntries, *nextDJ)

	// Clear and rebuild queue with updated entry
	err = m.client.Del(ctx, queueKey)
	if err != nil {
		logger.Error("Failed to clear queue for update", err, "roomId", roomID)
		return err
	}

	for _, entry := range queueEntries {
		entryJson, err := json.Marshal(entry)
		if err != nil {
			logger.Error("Failed to marshal queue entry", err, "roomId", roomID, "userId", entry.UserID)
			continue
		}

		err = m.client.RPush(ctx, queueKey, string(entryJson))
		if err != nil {
			logger.Error("Failed to add entry back to queue", err, "roomId", roomID, "userId", entry.UserID)
			continue
		}
	}

	// Clear current media information
	m.client.Del(ctx, formatRoomMediaKey(roomID))

	// Update room state with new DJ
	state.CurrentDJ = nextDJ.UserID
	state.CurrentMedia = "" // Will be set when DJ plays a track
	state.MediaStartTime = time.Time{}
	state.MediaEndTime = time.Time{}

	err = m.UpdateRoomState(ctx, state)
	if err != nil {
		return err
	}

	logger.Info("Advanced to next DJ", "roomId", roomID, "djId", nextDJ.UserID, "playCount", nextDJ.PlayCount)
	return nil
}

// SetCurrentMedia sets the current media being played
func (m *RoomStateManager) SetCurrentMedia(ctx context.Context, roomID string, mediaID string, duration int) error {
	logger := m.client.Logger()

	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	if state == nil {
		return fmt.Errorf("room not found: %s", roomID)
	}

	// Ensure there is a current DJ
	if state.CurrentDJ == "" {
		return fmt.Errorf("no current DJ in room: %s", roomID)
	}

	// Set current media
	now := time.Now()
	state.CurrentMedia = mediaID
	state.MediaStartTime = now
	state.MediaEndTime = now.Add(time.Duration(duration) * time.Second)

	// Store media info
	mediaKey := formatRoomMediaKey(roomID)
	err = m.client.Set(ctx, mediaKey, mediaID, time.Duration(duration+60)*time.Second)
	if err != nil {
		logger.Error("Failed to store media info", err, "roomId", roomID, "mediaId", mediaID)
		return err
	}

	// Update room state
	err = m.UpdateRoomState(ctx, state)
	if err != nil {
		return err
	}

	// Add to history
	err = m.AddToHistory(ctx, roomID, mediaID, state.CurrentDJ, duration)
	if err != nil {
		logger.Error("Failed to add media to history", err, "roomId", roomID, "mediaId", mediaID)
		// Continue anyway as this is not critical
	}

	logger.Info("Set current media", "roomId", roomID, "mediaId", mediaID, "duration", duration, "djId", state.CurrentDJ)
	return nil
}

// GetCurrentMedia gets the currently playing media
func (m *RoomStateManager) GetCurrentMedia(ctx context.Context, roomID string) (string, time.Time, time.Time, error) {
	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}

	if state == nil {
		return "", time.Time{}, time.Time{}, fmt.Errorf("room not found: %s", roomID)
	}

	return state.CurrentMedia, state.MediaStartTime, state.MediaEndTime, nil
}

// GetQueueEntries gets all entries in the DJ queue
func (m *RoomStateManager) GetQueueEntries(ctx context.Context, roomID string) ([]QueueEntry, error) {
	logger := m.client.Logger()

	// Get all entries from queue
	queueKey := formatRoomQueueKey(roomID)
	entries, err := m.client.LRange(ctx, queueKey, 0, -1)
	if err != nil {
		logger.Error("Failed to get queue entries", err, "roomId", roomID)
		return nil, err
	}

	// Parse entries
	queueEntries := make([]QueueEntry, 0, len(entries))
	for _, entryJson := range entries {
		var entry QueueEntry
		err := json.Unmarshal([]byte(entryJson), &entry)
		if err != nil {
			logger.Error("Failed to unmarshal queue entry", err, "roomId", roomID)
			continue
		}

		queueEntries = append(queueEntries, entry)
	}

	return queueEntries, nil
}

// AddToHistory adds a media item to the room's play history
func (m *RoomStateManager) AddToHistory(ctx context.Context, roomID, mediaID, djID string, duration int) error {
	logger := m.client.Logger()

	// Create history entry
	entry := struct {
		MediaID  string    `json:"mediaId"`
		DJID     string    `json:"djId"`
		Time     time.Time `json:"time"`
		Duration int       `json:"duration"`
	}{
		MediaID:  mediaID,
		DJID:     djID,
		Time:     time.Now(),
		Duration: duration,
	}

	// Convert to JSON
	entryJson, err := json.Marshal(entry)
	if err != nil {
		logger.Error("Failed to marshal history entry", err, "roomId", roomID, "mediaId", mediaID)
		return err
	}

	// Add to history list
	historyKey := formatRoomHistoryKey(roomID)
	err = m.client.LPush(ctx, historyKey, string(entryJson))
	if err != nil {
		logger.Error("Failed to add to history", err, "roomId", roomID, "mediaId", mediaID)
		return err
	}

	// Trim history to maximum size
	err = m.client.Client().LTrim(ctx, historyKey, 0, RoomHistoryMaxItems-1).Err()
	if err != nil {
		logger.Error("Failed to trim history", err, "roomId", roomID)
		// Continue anyway as this is not critical
	}

	// Set expiry on history
	err = m.client.Expire(ctx, historyKey, RoomStateExpiry)
	if err != nil {
		logger.Error("Failed to set expiry on history", err, "roomId", roomID)
		// Continue anyway as this is not critical
	}

	logger.Debug("Added media to history", "roomId", roomID, "mediaId", mediaID, "djId", djID)
	return nil
}

// GetHistory gets the room's play history
func (m *RoomStateManager) GetHistory(ctx context.Context, roomID string, limit int) ([]map[string]any, error) {
	logger := m.client.Logger()

	if limit <= 0 || limit > RoomHistoryMaxItems {
		limit = RoomHistoryMaxItems
	}

	// Get history entries
	historyKey := formatRoomHistoryKey(roomID)
	entries, err := m.client.LRange(ctx, historyKey, 0, int64(limit-1))
	if err != nil {
		logger.Error("Failed to get history", err, "roomId", roomID)
		return nil, err
	}

	// Parse entries
	history := make([]map[string]any, 0, len(entries))
	for _, entryJson := range entries {
		var entry map[string]any
		err := json.Unmarshal([]byte(entryJson), &entry)
		if err != nil {
			logger.Error("Failed to unmarshal history entry", err, "roomId", roomID)
			continue
		}

		history = append(history, entry)
	}

	return history, nil
}

// RecordVote records a user's vote for the current media
func (m *RoomStateManager) RecordVote(ctx context.Context, roomID, userID, mediaID, voteType string) error {
	logger := m.client.Logger()

	// Validate vote type
	if voteType != "woot" && voteType != "meh" && voteType != "grab" {
		return fmt.Errorf("invalid vote type: %s", voteType)
	}

	// Check if room and media exist
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	if state == nil {
		return fmt.Errorf("room not found: %s", roomID)
	}

	if state.CurrentMedia != mediaID {
		return fmt.Errorf("media is not currently playing: %s", mediaID)
	}

	// Check if user is in the room
	inRoom, err := m.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		return err
	}

	if !inRoom {
		return fmt.Errorf("user is not in the room: %s", userID)
	}

	// Record vote
	votesKey := formatRoomVotesKey(roomID, mediaID)
	voterKey := fmt.Sprintf("%s:%s", votesKey, userID)

	// Get previous vote if any
	previousVote, err := m.client.Get(ctx, voterKey)
	if err != nil && err != r.Nil {
		logger.Error("Failed to get previous vote", err, "roomId", roomID, "userId", userID, "mediaId", mediaID)
		return err
	}

	// If vote is the same, do nothing
	if previousVote == voteType {
		return nil
	}

	// Pipeline commands for atomic updates
	pipe := m.client.Pipeline()

	// Remove previous vote count if there was one
	if previousVote != "" {
		previousCountKey := fmt.Sprintf("%s:%s:count", votesKey, previousVote)
		pipe.Decr(ctx, previousCountKey)
	}

	// Record new vote
	pipe.Set(ctx, voterKey, voteType, time.Hour*24)

	// Increment vote count
	countKey := fmt.Sprintf("%s:%s:count", votesKey, voteType)
	pipe.Incr(ctx, countKey)

	// Execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		logger.Error("Failed to record vote", err, "roomId", roomID, "userId", userID, "mediaId", mediaID, "voteType", voteType)
		return err
	}

	logger.Info("Recorded vote", "roomId", roomID, "userId", userID, "mediaId", mediaID, "voteType", voteType, "previousVote", previousVote)
	return nil
}

// GetVotes gets the votes for a media item
func (m *RoomStateManager) GetVotes(ctx context.Context, roomID, mediaID string) (map[string]int, error) {
	logger := m.client.Logger()

	votesKey := formatRoomVotesKey(roomID, mediaID)

	// Get counts for each vote type
	wootKey := fmt.Sprintf("%s:woot:count", votesKey)
	mehKey := fmt.Sprintf("%s:meh:count", votesKey)
	grabKey := fmt.Sprintf("%s:grab:count", votesKey)

	// Pipeline commands
	pipe := m.client.Pipeline()
	wootCmd := pipe.Get(ctx, wootKey)
	mehCmd := pipe.Get(ctx, mehKey)
	grabCmd := pipe.Get(ctx, grabKey)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err != r.Nil {
		logger.Error("Failed to get votes", err, "roomId", roomID, "mediaId", mediaID)
		return nil, err
	}

	// Get results
	wootCount := 0
	mehCount := 0
	grabCount := 0

	// Parse woot count
	wootStr, err := wootCmd.Result()
	if err == nil {
		wootCount, _ = strconv.Atoi(wootStr)
	}

	// Parse meh count
	mehStr, err := mehCmd.Result()
	if err == nil {
		mehCount, _ = strconv.Atoi(mehStr)
	}

	// Parse grab count
	grabStr, err := grabCmd.Result()
	if err == nil {
		grabCount, _ = strconv.Atoi(grabStr)
	}

	// Create result map
	votes := map[string]int{
		"woot": wootCount,
		"meh":  mehCount,
		"grab": grabCount,
	}

	return votes, nil
}

// GetUserVote gets a user's vote for a media item
func (m *RoomStateManager) GetUserVote(ctx context.Context, roomID, userID, mediaID string) (string, error) {
	logger := m.client.Logger()

	votesKey := formatRoomVotesKey(roomID, mediaID)
	voterKey := fmt.Sprintf("%s:%s", votesKey, userID)

	vote, err := m.client.Get(ctx, voterKey)
	if err != nil {
		if err == r.Nil {
			return "", nil // No vote
		}
		logger.Error("Failed to get user vote", err, "roomId", roomID, "userId", userID, "mediaId", mediaID)
		return "", err
	}

	return vote, nil
}

// Helper functions

// formatRoomStateKey formats a key for room state
func formatRoomStateKey(roomID string) string {
	return redis.FormatKey(RoomStateKeyPrefix, roomID)
}

// formatRoomUsersKey formats a key for room users
func formatRoomUsersKey(roomID string) string {
	return redis.FormatKey(RoomUsersKeyPrefix, roomID)
}

// formatRoomQueueKey formats a key for room DJ queue
func formatRoomQueueKey(roomID string) string {
	return redis.FormatKey(RoomQueueKeyPrefix, roomID)
}

// formatRoomMediaKey formats a key for room current media
func formatRoomMediaKey(roomID string) string {
	return redis.FormatKey(RoomMediaKeyPrefix, roomID)
}

// formatRoomVotesKey formats a key for room votes
func formatRoomVotesKey(roomID, mediaID string) string {
	return redis.FormatKey(RoomVotesKeyPrefix, fmt.Sprintf("%s:%s", roomID, mediaID))
}

// formatRoomHistoryKey formats a key for room history
func formatRoomHistoryKey(roomID string) string {
	return redis.FormatKey(RoomHistoryKeyPrefix, roomID)
}

// updateQueueEntry updates an entry in a queue
func updateQueueEntry(queue []QueueEntry, entry QueueEntry) []QueueEntry {
	for i, e := range queue {
		if e.UserID == entry.UserID {
			queue[i] = entry
			break
		}
	}
	return queue
}
