// Package auth provides authentication and authorization functionality.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"slices"

	"github.com/golang-jwt/jwt/v5"
	"norelock.dev/listenify/backend/internal/utils"
)

// JWT errors
var (
	ErrInvalidToken    = errors.New("invalid token")
	ErrExpiredToken    = errors.New("token has expired")
	ErrTokenGeneration = errors.New("failed to generate token")
	ErrInvalidClaims   = errors.New("invalid token claims")
)

// JWTConfig contains configuration for the JWT provider.
type JWTConfig struct {
	// Secret is the signing key for JWTs.
	Secret string `yaml:"secret" validate:"required"`

	// Issuer is the issuer of the JWT.
	Issuer string `yaml:"issuer" validate:"required"`

	// Audience is the audience of the JWT.
	Audience string `yaml:"audience" validate:"required"`

	// AccessTokenDuration is the duration for which access tokens are valid.
	AccessTokenDuration time.Duration `yaml:"accessTokenDuration" validate:"required"`

	// RefreshTokenDuration is the duration for which refresh tokens are valid.
	RefreshTokenDuration time.Duration `yaml:"refreshTokenDuration" validate:"required"`
}

// JWTClaims extends the standard JWT claims with custom fields.
type JWTClaims struct {
	// BaseClaims embeds the base claims.
	BaseClaims

	// StandardClaims contains the standard JWT claims.
	jwt.RegisteredClaims
}

// JWTProvider implements the Provider interface using JWT.
type JWTProvider struct {
	config    JWTConfig
	validator *jwt.Validator
	logger    *utils.Logger
}

// NewJWTProvider creates a new JWT provider.
func NewJWTProvider(config JWTConfig, logger *utils.Logger) *JWTProvider {
	return &JWTProvider{
		config:    config,
		validator: jwt.NewValidator(jwt.WithLeeway(time.Second)),
		logger:    logger.Named("jwt_provider"),
	}
}

// GenerateToken creates a new JWT token for a user.
func (p *JWTProvider) GenerateToken(userID, username string, roles []string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(p.config.AccessTokenDuration)

	claims := JWTClaims{
		BaseClaims: BaseClaims{
			UserID:   userID,
			Username: username,
			Roles:    roles,
		},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    p.config.Issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{p.config.Audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        fmt.Sprintf("%d", now.UnixNano()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(p.config.Secret))
	if err != nil {
		p.logger.Error("Failed to sign JWT token", err, "userId", userID)
		return "", fmt.Errorf("%w: %v", ErrTokenGeneration, err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (p *JWTProvider) ValidateToken(tokenString string) (*Claims, error) {
	clean_claims := JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &clean_claims, func(token *jwt.Token) (any, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(p.config.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			// Assume that the token is still valid, but expired and return the claims
			// This allows for token refresh without needing to re-authenticate
			return &Claims{
				BaseClaims:     clean_claims.BaseClaims,
				StandardClaims: clean_claims.RegisteredClaims,
			}, ErrExpiredToken
		}
		p.logger.Error("Failed to parse JWT token", err)
		return nil, ErrInvalidToken
	}

	if token == nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Validate the claims
	err = p.validator.Validate(&clean_claims)

	if err != nil {
		p.logger.Error("Failed to validate JWT token", err)
		return nil, ErrInvalidToken
	}

	claims := clean_claims

	return &Claims{
		BaseClaims:     claims.BaseClaims,
		StandardClaims: claims.RegisteredClaims,
	}, nil
}

// RefreshToken refreshes a JWT token.
func (p *JWTProvider) RefreshToken(tokenString string) (string, error) {
	claims, err := p.ValidateToken(tokenString)

	if err != nil {
		if errors.Is(err, ErrExpiredToken) {
			// Token is expired, the user should log in again
			return "", err
		} else {
			// Token is invalid for any other reason, reject it
			return "", err
		}
	}

	if claims == nil {
		// If the token is valid, but we don't have claims, reject it
		return "", ErrInvalidToken
	}

	// Generate a new token with the same claims but new expiration
	return p.GenerateToken(claims.UserID, claims.Username, claims.Roles)
}

// GetUserIDFromToken extracts the user ID from a token.
func (p *JWTProvider) GetUserIDFromToken(tokenString string) (string, error) {
	claims, err := p.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// GetUserRolesFromToken extracts the user roles from a token.
func (p *JWTProvider) GetUserRolesFromToken(tokenString string) ([]string, error) {
	claims, err := p.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	return claims.Roles, nil
}

// HasRole checks if a token has a specific role.
func (p *JWTProvider) HasRole(ctx context.Context, tokenString, role string) bool {
	roles, err := p.GetUserRolesFromToken(tokenString)
	if err != nil {
		return false
	}

	return slices.Contains(roles, role)
}

// HasAnyRole checks if a token has any of the specified roles.
func (p *JWTProvider) HasAnyRole(ctx context.Context, tokenString string, roles ...string) bool {
	userRoles, err := p.GetUserRolesFromToken(tokenString)
	if err != nil {
		return false
	}

	// Create a map for O(1) lookups
	roleMap := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleMap[r] = struct{}{}
	}

	// Check if any of the required roles are in the user's roles
	for _, role := range roles {
		if _, ok := roleMap[role]; ok {
			return true
		}
	}
	return false
}

// HasAllRoles checks if a token has all of the specified roles.
func (p *JWTProvider) HasAllRoles(ctx context.Context, tokenString string, roles ...string) bool {
	userRoles, err := p.GetUserRolesFromToken(tokenString)
	if err != nil {
		return false
	}

	// Create a map for O(1) lookups
	roleMap := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleMap[r] = struct{}{}
	}

	// Check if all of the required roles are in the user's roles
	for _, role := range roles {
		if _, ok := roleMap[role]; !ok {
			return false
		}
	}
	return true
}
