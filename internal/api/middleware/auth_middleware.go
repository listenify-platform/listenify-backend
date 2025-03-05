// Package middleware contains HTTP middleware for the API.
package middleware

import (
	"context"
	"net/http"

	"norelock.dev/listenify/backend/internal/auth"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/utils"
)

// AuthMiddleware handles authentication for protected routes.
type AuthMiddleware struct {
	authProvider auth.Provider
	sessionMgr   managers.SessionManager
	logger       *utils.Logger
}

// NewAuthMiddleware creates a new auth middleware.
func NewAuthMiddleware(authProvider auth.Provider, sessionMgr managers.SessionManager, logger *utils.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authProvider: authProvider,
		sessionMgr:   sessionMgr,
		logger:       logger.Named("auth_middleware"),
	}
}

// RequireAuth is a middleware that requires authentication.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		token, err := utils.ExtractBearerToken(r)
		if err != nil {
			utils.RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Validate token
		claims, err := m.authProvider.ValidateToken(token)
		if err != nil {
			switch err {
			case auth.ErrInvalidToken:
				utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token")
			case auth.ErrExpiredToken:
				utils.RespondWithError(w, http.StatusUnauthorized, "Token has expired")
			default:
				m.logger.Error("Failed to validate token", err)
				utils.RespondWithError(w, http.StatusInternalServerError, "Failed to validate token")
			}
			return
		}

		// Verify session
		session, err := m.sessionMgr.GetSession(r.Context(), token)
		if err != nil {
			m.logger.Error("Failed to verify session", err, "userId", claims.UserID)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to verify session")
			return
		}

		if session == nil {
			utils.RespondWithError(w, http.StatusUnauthorized, "Session expired or invalid")
			return
		}

		// Add user ID and roles to context
		ctx := context.WithValue(r.Context(), "userID", claims.UserID)
		ctx = context.WithValue(ctx, "username", claims.Username)
		ctx = context.WithValue(ctx, "roles", claims.Roles)

		// Call the next handler with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole is a middleware that requires a specific role.
func (m *AuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			token, err := utils.ExtractBearerToken(r)
			if err != nil {
				utils.RespondWithError(w, http.StatusUnauthorized, err.Error())
				return
			}

			// Check if user has the required role
			hasRole := m.authProvider.HasRole(r.Context(), token, role)
			if !hasRole {
				utils.RespondWithError(w, http.StatusForbidden, "Insufficient permissions")
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyRole is a middleware that requires any of the specified roles.
func (m *AuthMiddleware) RequireAnyRole(roles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		token, err := utils.ExtractBearerToken(r)
		if err != nil {
			utils.RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Check if user has any of the required roles
		hasRole := m.authProvider.HasAnyRole(r.Context(), token, roles...)
		if !hasRole {
			utils.RespondWithError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// RequireAllRoles is a middleware that requires all of the specified roles.
func (m *AuthMiddleware) RequireAllRoles(roles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		token, err := utils.ExtractBearerToken(r)
		if err != nil {
			utils.RespondWithError(w, http.StatusUnauthorized, err.Error())
			return
		}

		// Check if user has all of the required roles
		hasRoles := m.authProvider.HasAllRoles(r.Context(), token, roles...)
		if !hasRoles {
			utils.RespondWithError(w, http.StatusForbidden, "Insufficient permissions")
			return
		}

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}
