// Package auth provides authentication and authorization functionality.
package auth

import (
	"context"
)

// Provider defines the interface for authentication operations.
type Provider interface {
	// HashPassword hashes a password for secure storage.
	HashPassword(password string) (string, error)

	// VerifyPassword checks if a password matches a hash.
	VerifyPassword(password, hash string) bool

	// GenerateToken creates a new JWT token for a user.
	GenerateToken(userID, username string, roles []string) (string, error)

	// ValidateToken validates a JWT token and returns the claims.
	ValidateToken(token string) (*Claims, error)

	// RefreshToken refreshes a JWT token.
	RefreshToken(token string) (string, error)

	// GetUserIDFromToken extracts the user ID from a token.
	GetUserIDFromToken(token string) (string, error)

	// GetUserRolesFromToken extracts the user roles from a token.
	GetUserRolesFromToken(token string) ([]string, error)

	// HasRole checks if a token has a specific role.
	HasRole(ctx context.Context, token, role string) bool

	// HasAnyRole checks if a token has any of the specified roles.
	HasAnyRole(ctx context.Context, token string, roles ...string) bool

	// HasAllRoles checks if a token has all of the specified roles.
	HasAllRoles(ctx context.Context, token string, roles ...string) bool
}

// BaseClaims represents the base claims in a JWT token.
// These are used in the application.
type BaseClaims struct {
	// UserID is the ID of the user.
	UserID string `json:"userId"`

	// Username is the username of the user.
	Username string `json:"username"`

	// Roles contains the user's roles.
	Roles []string `json:"roles"`
}

// Claims represents the JWT claims.
type Claims struct {
	// BaseClaims embeds the base claims.
	BaseClaims

	// StandardClaims contains the standard JWT claims.
	StandardClaims any `json:"standardClaims"`
}
