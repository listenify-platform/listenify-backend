// Package redis provides Redis database connectivity and operations.
package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"norelock.dev/listenify/backend/internal/utils"
)

const (
	// RateLimitKeyPrefix is the prefix for rate limit keys
	RateLimitKeyPrefix = "ratelimit"
)

// RateLimiter implements rate limiting using Redis
type RateLimiter struct {
	client *Client
	logger *utils.Logger
}

// RateLimit defines a rate limit constraint
type RateLimit struct {
	// Key is the identifier for this rate limit
	Key string

	// MaxRequests is the maximum number of requests allowed in the time window
	MaxRequests int

	// Window is the time window for rate limiting
	Window time.Duration
}

// RateLimitResult contains the result of a rate limit check
type RateLimitResult struct {
	// Allowed indicates whether the request is allowed
	Allowed bool

	// Remaining is the number of requests remaining in the current window
	Remaining int

	// RetryAfter is the time after which the client should retry (if rate limited)
	RetryAfter time.Duration

	// ResetAfter is the time after which the rate limit will reset
	ResetAfter time.Duration

	// Limit is the maximum number of requests allowed in the window
	Limit int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(client *Client) *RateLimiter {
	return &RateLimiter{
		client: client,
		logger: client.Logger(),
	}
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow(ctx context.Context, rateLimit RateLimit, identifier string) (*RateLimitResult, error) {
	logger := rl.logger

	// Format rate limit key
	rateLimitKey := formatRateLimitKey(rateLimit.Key, identifier)

	// Get current timestamp
	now := time.Now()
	windowStart := now.Add(-rateLimit.Window)
	windowStartMs := windowStart.UnixNano() / int64(time.Millisecond)

	// Execute rate limiting script
	pipe := rl.client.Pipeline()

	// Remove tokens older than the window
	pipe.ZRemRangeByScore(ctx, rateLimitKey, "0", strconv.FormatInt(windowStartMs, 10))

	// Count tokens in the current window
	countCmd := pipe.ZCard(ctx, rateLimitKey)

	// Get the oldest token timestamp
	oldestCmd := pipe.ZRange(ctx, rateLimitKey, 0, 0)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		logger.Error("Failed to execute rate limit pipeline", err, "key", rateLimitKey)
		return nil, err
	}

	// Get count of requests in the current window
	count, err := countCmd.Result()
	if err != nil && err != redis.Nil {
		logger.Error("Failed to get rate limit count", err, "key", rateLimitKey)
		return nil, err
	}

	// Check if rate limit is exceeded
	allowed := count < int64(rateLimit.MaxRequests)
	remaining := max(rateLimit.MaxRequests-int(count), 0)

	// Calculate retry-after time
	var retryAfter time.Duration
	var resetAfter time.Duration

	if !allowed {
		// Get the oldest token's timestamp
		oldest, err := oldestCmd.Result()
		if err != nil || len(oldest) == 0 {
			// If no tokens or error, use window duration as retry-after
			retryAfter = rateLimit.Window
			resetAfter = rateLimit.Window
		} else {
			// Parse oldest timestamp
			oldestMs, err := strconv.ParseInt(oldest[0], 10, 64)
			if err != nil {
				logger.Error("Failed to parse oldest token timestamp", err, "key", rateLimitKey)
				// Use window duration as fallback
				retryAfter = rateLimit.Window
				resetAfter = rateLimit.Window
			} else {
				// Calculate time when the oldest token will expire
				oldestTime := time.Unix(0, oldestMs*int64(time.Millisecond))
				expiryTime := oldestTime.Add(rateLimit.Window)
				retryAfter = expiryTime.Sub(now)
				resetAfter = retryAfter
			}
		}
	} else {
		// If allowed, add the current request token
		nowMs := now.UnixNano() / int64(time.Millisecond)
		err = rl.client.Client().ZAdd(ctx, rateLimitKey, &redis.Z{
			Score:  float64(nowMs),
			Member: strconv.FormatInt(nowMs, 10),
		}).Err()

		if err != nil {
			logger.Error("Failed to add token to rate limit", err, "key", rateLimitKey)
			// Still return allowed since we've already determined that
		}

		// Set expiration on the key to auto-cleanup
		err = rl.client.Expire(ctx, rateLimitKey, rateLimit.Window*2)
		if err != nil {
			logger.Error("Failed to set expiry on rate limit key", err, "key", rateLimitKey)
		}

		// Calculate reset time (window duration from now)
		resetAfter = rateLimit.Window
	}

	result := &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		RetryAfter: retryAfter,
		ResetAfter: resetAfter,
		Limit:      rateLimit.MaxRequests,
	}

	return result, nil
}

// Reset resets a rate limit for an identifier
func (rl *RateLimiter) Reset(ctx context.Context, rateLimit RateLimit, identifier string) error {
	logger := rl.logger

	// Format rate limit key
	rateLimitKey := formatRateLimitKey(rateLimit.Key, identifier)

	// Delete the rate limit key
	err := rl.client.Del(ctx, rateLimitKey)
	if err != nil {
		logger.Error("Failed to reset rate limit", err, "key", rateLimitKey)
		return err
	}

	logger.Debug("Reset rate limit", "key", rateLimitKey)
	return nil
}

// formatRateLimitKey formats a key for rate limiting
func formatRateLimitKey(key, identifier string) string {
	return FormatKey(RateLimitKeyPrefix, fmt.Sprintf("%s:%s", key, identifier))
}

// Common rate limit definitions

// RateLimitAuth defines rate limits for authentication actions
func RateLimitAuth() map[string]RateLimit {
	return map[string]RateLimit{
		"login": {
			Key:         "auth:login",
			MaxRequests: 5,
			Window:      time.Minute * 5,
		},
		"register": {
			Key:         "auth:register",
			MaxRequests: 3,
			Window:      time.Hour,
		},
		"password_reset": {
			Key:         "auth:password_reset",
			MaxRequests: 3,
			Window:      time.Hour * 24,
		},
	}
}

// RateLimitAPI defines rate limits for API actions
func RateLimitAPI() map[string]RateLimit {
	return map[string]RateLimit{
		"general": {
			Key:         "api:general",
			MaxRequests: 100,
			Window:      time.Minute,
		},
		"media_search": {
			Key:         "api:media_search",
			MaxRequests: 20,
			Window:      time.Minute,
		},
		"room_create": {
			Key:         "api:room_create",
			MaxRequests: 5,
			Window:      time.Hour,
		},
	}
}

// RateLimitWebSocket defines rate limits for WebSocket actions
func RateLimitWebSocket() map[string]RateLimit {
	return map[string]RateLimit{
		"connect": {
			Key:         "ws:connect",
			MaxRequests: 10,
			Window:      time.Minute,
		},
		"chat_message": {
			Key:         "ws:chat_message",
			MaxRequests: 30,
			Window:      time.Minute,
		},
		"dj_skip": {
			Key:         "ws:dj_skip",
			MaxRequests: 5,
			Window:      time.Minute * 5,
		},
		"media_vote": {
			Key:         "ws:media_vote",
			MaxRequests: 20,
			Window:      time.Minute,
		},
	}
}
