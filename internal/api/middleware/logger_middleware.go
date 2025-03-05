// Package middleware contains HTTP middleware for the API.
package middleware

import (
	"net/http"
	"time"

	"norelock.dev/listenify/backend/internal/utils"
)

// LoggerMiddleware handles request logging for the API.
type LoggerMiddleware struct {
	logger *utils.Logger
}

// NewLoggerMiddleware creates a new logger middleware.
func NewLoggerMiddleware(logger *utils.Logger) *LoggerMiddleware {
	return &LoggerMiddleware{
		logger: logger.Named("http"),
	}
}

// Logger is a middleware that logs HTTP requests.
func (m *LoggerMiddleware) Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer that captures the status code
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default status code
		}

		// Process the request
		next.ServeHTTP(rw, r)

		// Calculate request duration
		duration := time.Since(start)

		// Log the request
		m.logger.Info("HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.statusCode,
			"duration", duration.String(),
			"ip", utils.GetRequestIP(r),
			"userAgent", r.UserAgent(),
		)
	})
}

// responseWriter is a wrapper around http.ResponseWriter that captures the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and calls the underlying ResponseWriter's WriteHeader.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write calls the underlying ResponseWriter's Write method.
func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

// Header calls the underlying ResponseWriter's Header method.
func (rw *responseWriter) Header() http.Header {
	return rw.ResponseWriter.Header()
}