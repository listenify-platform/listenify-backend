// Package middleware contains HTTP middleware for the API.
package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"norelock.dev/listenify/backend/internal/utils"
)

// CORSConfig contains configuration for CORS middleware.
type CORSConfig struct {
	// AllowedOrigins is a list of origins a cross-domain request can be executed from.
	// If the special "*" value is present in the list, all origins will be allowed.
	// Default value is ["*"]
	AllowedOrigins []string

	// AllowedMethods is a list of methods the client is allowed to use with
	// cross-domain requests. Default value is simple methods (GET, POST, PUT, DELETE)
	AllowedMethods []string

	// AllowedHeaders is a list of non-simple headers the client is allowed to use with
	// cross-domain requests. Default value is ["Origin", "Accept", "Content-Type", "Authorization"]
	AllowedHeaders []string

	// ExposedHeaders indicates which headers are safe to expose to the API of a CORS
	// API specification. Default value is []
	ExposedHeaders []string

	// AllowCredentials indicates whether the request can include user credentials like
	// cookies, HTTP authentication or client side SSL certificates.
	AllowCredentials bool

	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached. Default value is 0, which stands for no max age.
	MaxAge int
}

// DefaultCORSConfig returns a default CORS configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders: []string{
			"Origin", "Accept", "Content-Type", "Authorization",
			"sentry-trace", "baggage", // Required by Sentry
		},
		ExposedHeaders:   []string{},
		AllowCredentials: true,
		MaxAge:           86400, // 24 hours
	}
}

// CORSMiddleware handles CORS for the API.
type CORSMiddleware struct {
	config CORSConfig
	logger *utils.Logger
}

// NewCORSMiddleware creates a new CORS middleware.
func NewCORSMiddleware(config CORSConfig, logger *utils.Logger) *CORSMiddleware {
	return &CORSMiddleware{
		config: config,
		logger: logger.Named("cors_middleware"),
	}
}

// CORS is a middleware that handles CORS.
func (m *CORSMiddleware) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Check if the origin is allowed
		allowedOrigin := m.isOriginAllowed(origin)
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			m.handlePreflight(w, r)
			return
		}

		// Set standard CORS headers
		m.setStandardHeaders(w, r)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// isOriginAllowed checks if the origin is allowed.
func (m *CORSMiddleware) isOriginAllowed(origin string) string {
	if origin == "" {
		return ""
	}

	// Check if all origins are allowed
	for _, allowedOrigin := range m.config.AllowedOrigins {
		if allowedOrigin == "*" {
			if m.config.AllowCredentials {
				// If credentials are allowed, we can't use "*" as the value of the
				// Access-Control-Allow-Origin header, so we return the origin
				return origin
			}
			return "*"
		}

		// Check if the origin matches exactly
		if allowedOrigin == origin {
			return origin
		}

		// Check if the origin matches with a wildcard
		if strings.HasSuffix(allowedOrigin, "*") {
			prefix := strings.TrimSuffix(allowedOrigin, "*")
			if strings.HasPrefix(origin, prefix) {
				return origin
			}
		}
	}

	return ""
}

// handlePreflight handles preflight requests.
func (m *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request) {
	// Set standard CORS headers
	m.setStandardHeaders(w, r)

	// Set preflight headers
	if len(m.config.AllowedMethods) > 0 {
		w.Header().Set("Access-Control-Allow-Methods", strings.Join(m.config.AllowedMethods, ", "))
	}

	if len(m.config.AllowedHeaders) > 0 {
		w.Header().Set("Access-Control-Allow-Headers", strings.Join(m.config.AllowedHeaders, ", "))
	}

	if m.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.config.MaxAge))
	}

	// Return 204 No Content
	w.WriteHeader(http.StatusNoContent)
}

// setStandardHeaders sets standard CORS headers.
func (m *CORSMiddleware) setStandardHeaders(w http.ResponseWriter, r *http.Request) {
	if m.config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	if len(m.config.ExposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.config.ExposedHeaders, ", "))
	}
}
