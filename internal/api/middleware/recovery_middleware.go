// Package middleware contains HTTP middleware for the API.
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"norelock.dev/listenify/backend/internal/utils"
)

// RecoveryMiddleware handles panic recovery for the API.
type RecoveryMiddleware struct {
	logger *utils.Logger
}

// NewRecoveryMiddleware creates a new recovery middleware.
func NewRecoveryMiddleware(logger *utils.Logger) *RecoveryMiddleware {
	return &RecoveryMiddleware{
		logger: logger.Named("recovery"),
	}
}

// Recovery is a middleware that recovers from panics.
func (m *RecoveryMiddleware) Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error and stack trace
				stack := debug.Stack()
				// Convert the recovered value to an error
				recoveryErr := fmt.Errorf("panic: %v", err)

				m.logger.Error("Panic recovered", recoveryErr,
					"stack", string(stack),
					"method", r.Method,
					"path", r.URL.Path,
					"ip", utils.GetRequestIP(r),
				)

				// Respond with a 500 Internal Server Error
				utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
			}
		}()

		// Process the request
		next.ServeHTTP(w, r)
	})
}

// RecoveryWithCustomHandler is a middleware that recovers from panics with a custom handler.
func (m *RecoveryMiddleware) RecoveryWithCustomHandler(next http.Handler, handler func(w http.ResponseWriter, r *http.Request, err any)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error and stack trace
				stack := debug.Stack()
				// Convert the recovered value to an error
				recoveryErr := fmt.Errorf("panic: %v", err)

				m.logger.Error("Panic recovered", recoveryErr,
					"stack", string(stack),
					"method", r.Method,
					"path", r.URL.Path,
					"ip", utils.GetRequestIP(r),
				)

				// Call the custom handler
				handler(w, r, err)
			}
		}()

		// Process the request
		next.ServeHTTP(w, r)
	})
}

// DefaultPanicHandler is the default handler for panics.
func DefaultPanicHandler(w http.ResponseWriter, r *http.Request, err any) {
	utils.RespondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Internal server error: %v", err))
}
