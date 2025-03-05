// Package redis provides Redis database connectivity and operations.
package managers

import (
	"context"
	"time"

	r "github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/db/redis"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

const (
	// SessionKeyPrefix is the prefix for session keys
	SessionKeyPrefix = "session"

	// DefaultSessionExpiry is the default session expiration time
	DefaultSessionExpiry = 24 * time.Hour

	// TokenKeyPrefix is the prefix for token-to-session mappings
	TokenKeyPrefix = "token"
)

// SessionManager handles Redis operations for user sessions
type SessionManager struct {
	client *redis.Client
	expiry time.Duration
}

// SessionData represents a user session
type SessionData struct {
	// UserID is the ID of the user
	UserID bson.ObjectID `json:"userId"`

	// Username is the username of the user
	Username string `json:"username"`

	// Roles contains the user's roles
	Roles []string `json:"roles"`

	// IP is the user's IP address
	IP string `json:"ip"`

	// UserAgent is the user's browser/client information
	UserAgent string `json:"userAgent"`

	// CreatedAt is when the session was created
	CreatedAt time.Time `json:"createdAt"`

	// ExpiresAt is when the session expires
	ExpiresAt time.Time `json:"expiresAt"`

	// LastActivity is when the user was last active
	LastActivity time.Time `json:"lastActivity"`

	// Data contains additional session data
	Data map[string]any `json:"data,omitempty"`
}

// NewSessionManager creates a new session manager
func NewSessionManager(client *redis.Client, expiry time.Duration) *SessionManager {
	if expiry <= 0 {
		expiry = DefaultSessionExpiry
	}

	return &SessionManager{
		client: client,
		expiry: expiry,
	}
}

// CreateSession creates a new session for a user
func (m *SessionManager) CreateSession(ctx context.Context, user *models.User, token, ip, userAgent string) (*SessionData, error) {
	logger := m.client.Logger()

	now := time.Now()
	expiresAt := now.Add(m.expiry)

	// Create session data
	session := &SessionData{
		UserID:       user.ID,
		Username:     user.Username,
		Roles:        user.Roles,
		IP:           ip,
		UserAgent:    userAgent,
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
		LastActivity: now,
		Data:         make(map[string]any),
	}

	// Generate session key
	sessionKey := redis.FormatKey(SessionKeyPrefix, token)

	// Store session in Redis
	err := m.client.SetObject(ctx, sessionKey, session, m.expiry)
	if err != nil {
		logger.Error("Failed to store session in Redis", err, "userId", user.ID.Hex())
		return nil, err
	}

	// Store token-to-session mapping
	tokenKey := redis.FormatKey(TokenKeyPrefix, user.ID.Hex())
	err = m.client.Set(ctx, tokenKey, token, m.expiry)
	if err != nil {
		logger.Error("Failed to store token mapping in Redis", err, "userId", user.ID.Hex())

		// Try to clean up session
		_ = m.client.Del(ctx, sessionKey)

		return nil, err
	}

	logger.Info("Created session", "userId", user.ID.Hex(), "token", utils.TruncateString(token, 8)+"...")
	return session, nil
}

// GetSession retrieves a session by token
func (m *SessionManager) GetSession(ctx context.Context, token string) (*SessionData, error) {
	logger := m.client.Logger()

	// Generate session key
	sessionKey := redis.FormatKey(SessionKeyPrefix, token)

	// Get session from Redis
	var session SessionData
	err := m.client.GetObject(ctx, sessionKey, &session)
	if err != nil {
		if err == r.Nil {
			logger.Debug("Session not found", "token", utils.TruncateString(token, 8)+"...")
			return nil, nil
		}
		logger.Error("Failed to get session from Redis", err, "token", utils.TruncateString(token, 8)+"...")
		return nil, err
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		logger.Debug("Session expired", "userId", session.UserID.Hex(), "token", utils.TruncateString(token, 8)+"...")

		// Clean up expired session
		_ = m.client.Del(ctx, sessionKey)

		return nil, nil
	}

	return &session, nil
}

// UpdateSession updates a session
func (m *SessionManager) UpdateSession(ctx context.Context, token string, session *SessionData) error {
	logger := m.client.Logger()

	// Update last activity time
	session.LastActivity = time.Now()

	// Generate session key
	sessionKey := redis.FormatKey(SessionKeyPrefix, token)

	// Update session in Redis
	remainingTTL, err := m.client.TTL(ctx, sessionKey)
	if err != nil {
		logger.Error("Failed to get session TTL", err, "userId", session.UserID.Hex())
		return err
	}

	if remainingTTL < 0 {
		// Session does not exist or has no expiry
		return r.Nil
	}

	err = m.client.SetObject(ctx, sessionKey, session, remainingTTL)
	if err != nil {
		logger.Error("Failed to update session in Redis", err, "userId", session.UserID.Hex())
		return err
	}

	logger.Debug("Updated session", "userId", session.UserID.Hex(), "token", utils.TruncateString(token, 8)+"...")
	return nil
}

// RefreshSession extends a session's expiration time
func (m *SessionManager) RefreshSession(ctx context.Context, token string) error {
	logger := m.client.Logger()

	// Generate session key
	sessionKey := redis.FormatKey(SessionKeyPrefix, token)

	// Get session from Redis
	var session SessionData
	err := m.client.GetObject(ctx, sessionKey, &session)
	if err != nil {
		if err == r.Nil {
			logger.Debug("Session not found for refresh", "token", utils.TruncateString(token, 8)+"...")
			return nil
		}
		logger.Error("Failed to get session for refresh", err, "token", utils.TruncateString(token, 8)+"...")
		return err
	}

	// Update expiration times
	now := time.Now()
	session.LastActivity = now
	session.ExpiresAt = now.Add(m.expiry)

	// Update session in Redis
	err = m.client.SetObject(ctx, sessionKey, &session, m.expiry)
	if err != nil {
		logger.Error("Failed to refresh session in Redis", err, "userId", session.UserID.Hex())
		return err
	}

	// Also refresh token mapping
	tokenKey := redis.FormatKey(TokenKeyPrefix, session.UserID.Hex())
	err = m.client.Expire(ctx, tokenKey, m.expiry)
	if err != nil {
		logger.Error("Failed to refresh token mapping in Redis", err, "userId", session.UserID.Hex())
	}

	logger.Debug("Refreshed session", "userId", session.UserID.Hex(), "token", utils.TruncateString(token, 8)+"...")
	return nil
}

// DestroySession removes a session
func (m *SessionManager) DestroySession(ctx context.Context, token string) error {
	logger := m.client.Logger()

	// Generate session key
	sessionKey := redis.FormatKey(SessionKeyPrefix, token)

	// Get session to find user ID
	var session SessionData
	err := m.client.GetObject(ctx, sessionKey, &session)
	if err != nil && err != r.Nil {
		logger.Error("Failed to get session for destruction", err, "token", utils.TruncateString(token, 8)+"...")
		return err
	}

	// Remove session from Redis
	err = m.client.Del(ctx, sessionKey)
	if err != nil {
		logger.Error("Failed to destroy session in Redis", err, "token", utils.TruncateString(token, 8)+"...")
		return err
	}

	// If we have the user ID, also clean up token mapping
	if session.UserID != bson.NilObjectID {
		tokenKey := redis.FormatKey(TokenKeyPrefix, session.UserID.Hex())
		err = m.client.Del(ctx, tokenKey)
		if err != nil {
			logger.Error("Failed to remove token mapping", err, "userId", session.UserID.Hex())
		}

		logger.Info("Destroyed session", "userId", session.UserID.Hex(), "token", utils.TruncateString(token, 8)+"...")
	} else {
		logger.Info("Destroyed session", "token", utils.TruncateString(token, 8)+"...")
	}

	return nil
}

// DestroyUserSessions removes all sessions for a user
func (m *SessionManager) DestroyUserSessions(ctx context.Context, userID bson.ObjectID) error {
	logger := m.client.Logger()

	// Get token for user
	tokenKey := redis.FormatKey(TokenKeyPrefix, userID.Hex())
	token, err := m.client.Get(ctx, tokenKey)
	if err != nil {
		if err == r.Nil {
			// No sessions found
			return nil
		}
		logger.Error("Failed to get token for user sessions", err, "userId", userID.Hex())
		return err
	}

	// Remove session
	sessionKey := redis.FormatKey(SessionKeyPrefix, token)
	err = m.client.Del(ctx, sessionKey)
	if err != nil {
		logger.Error("Failed to destroy user session", err, "userId", userID.Hex())
		return err
	}

	// Remove token mapping
	err = m.client.Del(ctx, tokenKey)
	if err != nil {
		logger.Error("Failed to remove token mapping", err, "userId", userID.Hex())
		return err
	}

	logger.Info("Destroyed all sessions for user", "userId", userID.Hex())
	return nil
}

// SetSessionData sets a value in the session data
func (m *SessionManager) SetSessionData(ctx context.Context, token string, key string, value any) error {
	// Get session
	session, err := m.GetSession(ctx, token)
	if err != nil {
		return err
	}

	if session == nil {
		return r.Nil
	}

	// Update session data
	if session.Data == nil {
		session.Data = make(map[string]any)
	}
	session.Data[key] = value

	// Update session in Redis
	return m.UpdateSession(ctx, token, session)
}

// GetSessionData gets a value from the session data
func (m *SessionManager) GetSessionData(ctx context.Context, token string, key string) (any, error) {
	// Get session
	session, err := m.GetSession(ctx, token)
	if err != nil {
		return nil, err
	}

	if session == nil {
		return nil, r.Nil
	}

	// Get data from session
	if session.Data == nil {
		return nil, nil
	}

	return session.Data[key], nil
}

// RemoveSessionData removes a value from the session data
func (m *SessionManager) RemoveSessionData(ctx context.Context, token string, key string) error {
	// Get session
	session, err := m.GetSession(ctx, token)
	if err != nil {
		return err
	}

	if session == nil {
		return r.Nil
	}

	// Remove data from session
	if session.Data != nil {
		delete(session.Data, key)
	}

	// Update session in Redis
	return m.UpdateSession(ctx, token, session)
}

// GetActiveSessions gets the count of active sessions
func (m *SessionManager) GetActiveSessions(ctx context.Context) (int64, error) {
	logger := m.client.Logger()

	// Count session keys
	count, err := m.client.Keys(ctx, redis.FormatKey(SessionKeyPrefix, "*"))
	if err != nil {
		logger.Error("Failed to count active sessions", err)
		return 0, err
	}

	return int64(len(count)), nil
}

// GetUserSession gets the current session for a user
func (m *SessionManager) GetUserSession(ctx context.Context, userID bson.ObjectID) (*SessionData, string, error) {
	logger := m.client.Logger()

	// Get token for user
	tokenKey := redis.FormatKey(TokenKeyPrefix, userID.Hex())
	token, err := m.client.Get(ctx, tokenKey)
	if err != nil {
		if err == r.Nil {
			// No session found
			return nil, "", nil
		}
		logger.Error("Failed to get token for user session", err, "userId", userID.Hex())
		return nil, "", err
	}

	// Get session
	session, err := m.GetSession(ctx, token)
	if err != nil {
		return nil, "", err
	}

	return session, token, nil
}

// CleanupExpiredSessions removes expired sessions
// This is typically called by a background task
func (m *SessionManager) CleanupExpiredSessions(ctx context.Context) (int, error) {
	logger := m.client.Logger()

	// This is not strictly necessary in Redis as we use TTL for sessions,
	// but it ensures any orphaned data is cleaned up

	// Get all session keys
	sessionKeys, err := m.client.Keys(ctx, redis.FormatKey(SessionKeyPrefix, "*"))
	if err != nil {
		logger.Error("Failed to get session keys for cleanup", err)
		return 0, err
	}

	cleanedCount := 0
	now := time.Now()

	// Check each session
	for _, key := range sessionKeys {
		var session SessionData
		err := m.client.GetObject(ctx, key, &session)
		if err != nil {
			if err != r.Nil {
				logger.Error("Failed to get session for cleanup check", err, "key", key)
			}
			continue
		}

		// If session is expired, remove it
		if now.After(session.ExpiresAt) {
			// Extract token from key
			token := key[len(redis.FormatKey(SessionKeyPrefix, "")):]

			err := m.DestroySession(ctx, token)
			if err != nil {
				logger.Error("Failed to destroy expired session", err, "userId", session.UserID.Hex())
				continue
			}

			cleanedCount++
		}
	}

	logger.Info("Cleaned up expired sessions", "count", cleanedCount)
	return cleanedCount, nil
}
