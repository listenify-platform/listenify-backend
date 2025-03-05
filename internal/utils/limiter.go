// Package utils provides utility functions used throughout the application.
package utils

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimiter provides a simple in-memory rate limiting functionality.
type RateLimiter struct {
	// requests maps keys to the number of requests made
	requests map[string][]time.Time

	// window defines the time period for limiting
	window time.Duration

	// limit is the maximum number of requests allowed in the window
	limit int

	// mu synchronizes access to the requests map
	mu sync.RWMutex
}

// NewRateLimiter creates a new rate limiter with the specified window and limit.
func NewRateLimiter(window time.Duration, limit int) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		window:   window,
		limit:    limit,
		mu:       sync.RWMutex{},
	}
}

// Allow checks if a request with the given key is allowed.
// It returns true if the request is allowed, and false otherwise.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Create a new entry if the key doesn't exist
	if _, exists := rl.requests[key]; !exists {
		rl.requests[key] = []time.Time{now}
		return true
	}

	// Remove requests outside the window
	cutoff := now.Add(-rl.window)
	var validRequests []time.Time

	for _, t := range rl.requests[key] {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}

	// Update valid requests
	rl.requests[key] = validRequests

	// Check if we're over the limit
	if len(validRequests) >= rl.limit {
		return false
	}

	// Record this request
	rl.requests[key] = append(rl.requests[key], now)
	return true
}

// GetRemainingRequests returns the number of remaining requests for the given key.
func (rl *RateLimiter) GetRemainingRequests(key string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	var validCount int
	if requests, exists := rl.requests[key]; exists {
		for _, t := range requests {
			if t.After(cutoff) {
				validCount++
			}
		}
	}

	remaining := max(rl.limit-validCount, 0)

	return remaining
}

// GetResetTime returns the time when the rate limit will reset for the given key.
func (rl *RateLimiter) GetResetTime(key string) time.Time {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	now := time.Now()

	// If no requests, reset time is now
	if requests, exists := rl.requests[key]; !exists || len(requests) == 0 {
		return now
	}

	// Find the oldest request in the window
	cutoff := now.Add(-rl.window)
	var oldestInWindow time.Time

	for _, t := range rl.requests[key] {
		if t.After(cutoff) && (oldestInWindow.IsZero() || t.Before(oldestInWindow)) {
			oldestInWindow = t
		}
	}

	// If no requests in window, reset time is now
	if oldestInWindow.IsZero() {
		return now
	}

	// Reset time is when the oldest request in window expires
	return oldestInWindow.Add(rl.window)
}

// CleanupLoop periodically cleans up expired entries.
// It should be started in a goroutine.
func (rl *RateLimiter) CleanupLoop(ctx context.Context, cleanupInterval time.Duration) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

// cleanup removes expired entries from the requests map.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	for key, times := range rl.requests {
		var validTimes []time.Time

		for _, t := range times {
			if t.After(cutoff) {
				validTimes = append(validTimes, t)
			}
		}

		if len(validTimes) == 0 {
			delete(rl.requests, key)
		} else {
			rl.requests[key] = validTimes
		}
	}
}

// RateLimitMiddleware is an HTTP middleware that applies rate limiting.
func RateLimitMiddleware(limiter *RateLimiter, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			remaining := limiter.GetRemainingRequests(key)
			resetTime := limiter.GetResetTime(key)

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

			if !limiter.Allow(key) {
				// Set Retry-After header
				w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(resetTime).Seconds())))

				// Return rate limit exceeded error
				ErrorJSON(w, http.StatusTooManyRequests, ErrRateLimited)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// DefaultKeyFunc creates a rate limit key based on the client's IP address.
func DefaultKeyFunc(r *http.Request) string {
	return GetRequestIP(r)
}

// RouteKeyFunc creates a rate limit key based on the client's IP address and request path.
func RouteKeyFunc(r *http.Request) string {
	return fmt.Sprintf("%s:%s", GetRequestIP(r), r.URL.Path)
}

// UserKeyFunc creates a rate limit key based on the user ID from context, falling back to IP.
func UserKeyFunc(r *http.Request) string {
	// Get user ID from context (typically set by authentication middleware)
	if userID, ok := r.Context().Value("userID").(string); ok && userID != "" {
		return fmt.Sprintf("user:%s", userID)
	}

	// Fall back to IP-based limiting
	return fmt.Sprintf("ip:%s", GetRequestIP(r))
}

// ActionKeyFunc creates a rate limit key based on the user ID (or IP) and a specific action.
func ActionKeyFunc(action string) func(*http.Request) string {
	return func(r *http.Request) string {
		// Get user ID from context
		if userID, ok := r.Context().Value("userID").(string); ok && userID != "" {
			return fmt.Sprintf("user:%s:action:%s", userID, action)
		}

		// Fall back to IP-based limiting
		return fmt.Sprintf("ip:%s:action:%s", GetRequestIP(r), action)
	}
}

// LimiterConfig defines configuration for different rate limit policies.
type LimiterConfig struct {
	// General API requests
	API *RateLimiter

	// Login attempts
	Login *RateLimiter

	// Registration
	Register *RateLimiter

	// Password reset
	PasswordReset *RateLimiter

	// Media searches
	MediaSearch *RateLimiter

	// WebSocket connections
	WebSocket *RateLimiter

	// Chat messages
	ChatMessages *RateLimiter

	// Media skip
	MediaSkip *RateLimiter

	// Room creation
	RoomCreate *RateLimiter
}

// NewDefaultLimiterConfig creates a default rate limiter configuration.
func NewDefaultLimiterConfig() *LimiterConfig {
	return &LimiterConfig{
		API:           NewRateLimiter(time.Minute, 100),   // 100 requests per minute
		Login:         NewRateLimiter(time.Minute*15, 10), // 10 login attempts per 15 minutes
		Register:      NewRateLimiter(time.Hour*24, 5),    // 5 registrations per day
		PasswordReset: NewRateLimiter(time.Hour, 3),       // 3 password resets per hour
		MediaSearch:   NewRateLimiter(time.Minute, 20),    // 20 searches per minute
		WebSocket:     NewRateLimiter(time.Minute*5, 10),  // 10 websocket connections per 5 minutes
		ChatMessages:  NewRateLimiter(time.Minute, 120),   // 120 chat messages per minute (2 per second)
		MediaSkip:     NewRateLimiter(time.Minute*5, 5),   // 5 skips per 5 minutes
		RoomCreate:    NewRateLimiter(time.Hour, 3),       // 3 room creations per hour
	}
}

// StartCleanupRoutines starts the cleanup routines for all rate limiters.
// It returns a function that should be called to stop the cleanup routines.
func (lc *LimiterConfig) StartCleanupRoutines(ctx context.Context) func() {
	// Create a cancellable context for the cleanup routines
	cleanupCtx, cancel := context.WithCancel(ctx)

	// Start cleanup routines for each limiter
	go lc.API.CleanupLoop(cleanupCtx, time.Minute*5)
	go lc.Login.CleanupLoop(cleanupCtx, time.Minute*5)
	go lc.Register.CleanupLoop(cleanupCtx, time.Hour)
	go lc.PasswordReset.CleanupLoop(cleanupCtx, time.Minute*15)
	go lc.MediaSearch.CleanupLoop(cleanupCtx, time.Minute*5)
	go lc.WebSocket.CleanupLoop(cleanupCtx, time.Minute*5)
	go lc.ChatMessages.CleanupLoop(cleanupCtx, time.Minute*5)
	go lc.MediaSkip.CleanupLoop(cleanupCtx, time.Minute*5)
	go lc.RoomCreate.CleanupLoop(cleanupCtx, time.Hour)

	// Return a function to stop all cleanup routines
	return cancel
}
