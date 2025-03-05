// Package redis provides Redis database connectivity and operations.
package managers

import (
	"context"
	"fmt"
	"time"

	r "github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/db/redis"
)

const (
	// PresenceKeyPrefix is the prefix for presence keys
	PresenceKeyPrefix = "presence"

	// OnlineUsersKey is the key for the set of online users
	OnlineUsersKey = "online:users"

	// PresenceTTL is the expiration time for presence keys
	PresenceTTL = 2 * time.Minute

	// PresenceUpdateInterval is the recommended interval for updating presence
	PresenceUpdateInterval = 1 * time.Minute
)

// PresenceInfo represents user presence information
type PresenceInfo struct {
	// UserID is the ID of the user
	UserID string `json:"userId"`

	// Username is the username of the user
	Username string `json:"username"`

	// Status is the user's current status (online, away, busy, etc.)
	Status string `json:"status"`

	// LastActivity is when the user was last active
	LastActivity time.Time `json:"lastActivity"`

	// CurrentRoomID is the ID of the room the user is currently in, if any
	CurrentRoomID string `json:"currentRoomId,omitempty"`

	// LastSeen is the last time the presence was updated
	LastSeen time.Time `json:"lastSeen"`

	// Data contains additional presence data
	Data map[string]any `json:"data,omitempty"`
}

// PresenceManager handles Redis operations for user presence
type PresenceManager struct {
	client *redis.Client
}

// NewPresenceManager creates a new presence manager
func NewPresenceManager(client *redis.Client) *PresenceManager {
	return &PresenceManager{
		client: client,
	}
}

// UpdatePresence updates a user's presence information
func (m *PresenceManager) UpdatePresence(ctx context.Context, userID bson.ObjectID, username, status string) error {
	logger := m.client.Logger()

	userIDStr := userID.Hex()
	now := time.Now()

	// Get existing presence info if any
	presenceKey := formatPresenceKey(userIDStr)
	var presence PresenceInfo

	err := m.client.GetObject(ctx, presenceKey, &presence)
	if err != nil && err != r.Nil {
		logger.Error("Failed to get presence info", err, "userId", userIDStr)
		return err
	}

	// If no existing presence or status changed, create new presence info
	if err == r.Nil || presence.Status != status {
		presence = PresenceInfo{
			UserID:       userIDStr,
			Username:     username,
			Status:       status,
			LastActivity: now,
			LastSeen:     now,
			Data:         make(map[string]any),
		}
	} else {
		// Update last seen time
		presence.LastSeen = now
	}

	// Store presence info in Redis
	err = m.client.SetObject(ctx, presenceKey, &presence, PresenceTTL)
	if err != nil {
		logger.Error("Failed to store presence info", err, "userId", userIDStr)
		return err
	}

	// Add user to online users set
	err = m.client.SAdd(ctx, OnlineUsersKey, userIDStr)
	if err != nil {
		logger.Error("Failed to add user to online users", err, "userId", userIDStr)
		return err
	}

	logger.Debug("Updated user presence", "userId", userIDStr, "status", status)
	return nil
}

// UpdateUserActivity updates a user's last activity time
func (m *PresenceManager) UpdateUserActivity(ctx context.Context, userID bson.ObjectID) error {
	logger := m.client.Logger()

	userIDStr := userID.Hex()
	now := time.Now()

	// Get existing presence info
	presenceKey := formatPresenceKey(userIDStr)
	var presence PresenceInfo

	err := m.client.GetObject(ctx, presenceKey, &presence)
	if err != nil {
		if err == r.Nil {
			logger.Debug("No presence info for activity update", "userId", userIDStr)
			return nil
		}
		logger.Error("Failed to get presence info for activity update", err, "userId", userIDStr)
		return err
	}

	// Update last activity and last seen times
	presence.LastActivity = now
	presence.LastSeen = now

	// Store updated presence info
	err = m.client.SetObject(ctx, presenceKey, &presence, PresenceTTL)
	if err != nil {
		logger.Error("Failed to update activity time", err, "userId", userIDStr)
		return err
	}

	logger.Debug("Updated user activity", "userId", userIDStr)
	return nil
}

// SetUserRoom updates the room a user is currently in
func (m *PresenceManager) SetUserRoom(ctx context.Context, userID bson.ObjectID, roomID string) error {
	logger := m.client.Logger()

	userIDStr := userID.Hex()

	// Get existing presence info
	presenceKey := formatPresenceKey(userIDStr)
	var presence PresenceInfo

	err := m.client.GetObject(ctx, presenceKey, &presence)
	if err != nil {
		if err == r.Nil {
			logger.Debug("No presence info for room update", "userId", userIDStr)
			return nil
		}
		logger.Error("Failed to get presence info for room update", err, "userId", userIDStr)
		return err
	}

	// Update room and last seen time
	presence.CurrentRoomID = roomID
	presence.LastSeen = time.Now()

	// Store updated presence info
	err = m.client.SetObject(ctx, presenceKey, &presence, PresenceTTL)
	if err != nil {
		logger.Error("Failed to update user room", err, "userId", userIDStr, "roomId", roomID)
		return err
	}

	logger.Debug("Updated user room", "userId", userIDStr, "roomId", roomID)
	return nil
}

// GetPresence gets a user's presence information
func (m *PresenceManager) GetPresence(ctx context.Context, userID bson.ObjectID) (*PresenceInfo, error) {
	logger := m.client.Logger()

	userIDStr := userID.Hex()
	presenceKey := formatPresenceKey(userIDStr)

	// Get presence info from Redis
	var presence PresenceInfo
	err := m.client.GetObject(ctx, presenceKey, &presence)
	if err != nil {
		if err == r.Nil {
			return nil, nil // User not present
		}
		logger.Error("Failed to get presence info", err, "userId", userIDStr)
		return nil, err
	}

	return &presence, nil
}

// GetUserStatus gets a user's current status
func (m *PresenceManager) GetUserStatus(ctx context.Context, userID bson.ObjectID) (string, error) {
	presence, err := m.GetPresence(ctx, userID)
	if err != nil {
		return "", err
	}

	if presence == nil {
		return "offline", nil
	}

	return presence.Status, nil
}

// IsUserOnline checks if a user is currently online
func (m *PresenceManager) IsUserOnline(ctx context.Context, userID bson.ObjectID) (bool, error) {
	logger := m.client.Logger()

	userIDStr := userID.Hex()

	// Check if user is in online users set
	isMember, err := m.client.SIsMember(ctx, OnlineUsersKey, userIDStr)
	if err != nil {
		logger.Error("Failed to check if user is online", err, "userId", userIDStr)
		return false, err
	}

	if !isMember {
		return false, nil
	}

	// Double-check by getting presence info
	presence, err := m.GetPresence(ctx, userID)
	if err != nil {
		return false, err
	}

	return presence != nil, nil
}

// SetUserStatus sets a user's status
func (m *PresenceManager) SetUserStatus(ctx context.Context, userID bson.ObjectID, status string) error {
	logger := m.client.Logger()

	userIDStr := userID.Hex()

	// Get existing presence info
	presenceKey := formatPresenceKey(userIDStr)
	var presence PresenceInfo

	err := m.client.GetObject(ctx, presenceKey, &presence)
	if err != nil && err != r.Nil {
		logger.Error("Failed to get presence info for status update", err, "userId", userIDStr)
		return err
	}

	if err == r.Nil {
		// No presence info available, can't update
		return fmt.Errorf("user not present: %s", userIDStr)
	}

	// Update status and last seen time
	presence.Status = status
	presence.LastSeen = time.Now()

	// Store updated presence info
	err = m.client.SetObject(ctx, presenceKey, &presence, PresenceTTL)
	if err != nil {
		logger.Error("Failed to update user status", err, "userId", userIDStr, "status", status)
		return err
	}

	logger.Debug("Updated user status", "userId", userIDStr, "status", status)
	return nil
}

// SetPresenceData sets additional data in a user's presence info
func (m *PresenceManager) SetPresenceData(ctx context.Context, userID bson.ObjectID, key string, value any) error {
	logger := m.client.Logger()

	userIDStr := userID.Hex()

	// Get existing presence info
	presenceKey := formatPresenceKey(userIDStr)
	var presence PresenceInfo

	err := m.client.GetObject(ctx, presenceKey, &presence)
	if err != nil && err != r.Nil {
		logger.Error("Failed to get presence info for data update", err, "userId", userIDStr)
		return err
	}

	if err == r.Nil {
		// No presence info available, can't update
		return fmt.Errorf("user not present: %s", userIDStr)
	}

	// Update data and last seen time
	if presence.Data == nil {
		presence.Data = make(map[string]any)
	}
	presence.Data[key] = value
	presence.LastSeen = time.Now()

	// Store updated presence info
	err = m.client.SetObject(ctx, presenceKey, &presence, PresenceTTL)
	if err != nil {
		logger.Error("Failed to update presence data", err, "userId", userIDStr, "key", key)
		return err
	}

	logger.Debug("Updated presence data", "userId", userIDStr, "key", key)
	return nil
}

// RemovePresence removes a user's presence information
func (m *PresenceManager) RemovePresence(ctx context.Context, userID bson.ObjectID) error {
	logger := m.client.Logger()

	userIDStr := userID.Hex()
	presenceKey := formatPresenceKey(userIDStr)

	// Remove presence info from Redis
	err := m.client.Del(ctx, presenceKey)
	if err != nil {
		logger.Error("Failed to remove presence info", err, "userId", userIDStr)
		return err
	}

	// Remove from online users set
	err = m.client.SRem(ctx, OnlineUsersKey, userIDStr)
	if err != nil {
		logger.Error("Failed to remove user from online users", err, "userId", userIDStr)
		return err
	}

	logger.Info("Removed user presence", "userId", userIDStr)
	return nil
}

// GetOnlineUsers gets all online users
func (m *PresenceManager) GetOnlineUsers(ctx context.Context) ([]string, error) {
	logger := m.client.Logger()

	// Get all members of online users set
	userIDs, err := m.client.SMembers(ctx, OnlineUsersKey)
	if err != nil {
		logger.Error("Failed to get online users", err)
		return nil, err
	}

	return userIDs, nil
}

// GetOnlineUsersCount gets the count of online users
func (m *PresenceManager) GetOnlineUsersCount(ctx context.Context) (int64, error) {
	logger := m.client.Logger()

	// Get count of online users set
	count, err := m.client.SCard(ctx, OnlineUsersKey)
	if err != nil {
		logger.Error("Failed to get online users count", err)
		return 0, err
	}

	return count, nil
}

// CleanupExpiredPresence removes expired presence information
func (m *PresenceManager) CleanupExpiredPresence(ctx context.Context) (int, error) {
	logger := m.client.Logger()

	// Get all online users
	userIDs, err := m.GetOnlineUsers(ctx)
	if err != nil {
		return 0, err
	}

	removedCount := 0
	now := time.Now()
	expiryThreshold := now.Add(-PresenceTTL)

	// Check each user's presence
	for _, userID := range userIDs {
		presenceKey := formatPresenceKey(userID)
		var presence PresenceInfo

		err := m.client.GetObject(ctx, presenceKey, &presence)
		if err != nil {
			if err == r.Nil {
				// Presence info not found, remove from online users
				err = m.client.SRem(ctx, OnlineUsersKey, userID)
				if err != nil {
					logger.Error("Failed to remove user from online users during cleanup", err, "userId", userID)
				} else {
					removedCount++
				}
			} else {
				logger.Error("Failed to get presence info during cleanup", err, "userId", userID)
			}
			continue
		}

		// Check if presence is expired
		if presence.LastSeen.Before(expiryThreshold) {
			// Remove expired presence
			objectID, err := bson.ObjectIDFromHex(userID)
			if err != nil {
				logger.Error("Failed to convert userID to ObjectID during cleanup", err, "userId", userID)
				continue
			}
			err = m.RemovePresence(ctx, objectID)
			if err != nil {
				logger.Error("Failed to remove expired presence", err, "userId", userID)
				continue
			}

			removedCount++
		}
	}

	logger.Info("Cleaned up expired presence", "count", removedCount)
	return removedCount, nil
}

// formatPresenceKey formats a key for user presence
func formatPresenceKey(userID string) string {
	return redis.FormatKey(PresenceKeyPrefix, userID)
}
