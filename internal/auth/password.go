// Package auth provides authentication and authorization functionality.
package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
	"norelock.dev/listenify/backend/internal/utils"
)

// Password errors
var (
	ErrHashingPassword = errors.New("failed to hash password")
	ErrInvalidPassword = errors.New("invalid password")
)

// PasswordProvider implements password hashing and verification.
type PasswordProvider struct {
	logger *utils.Logger
}

// NewPasswordProvider creates a new password provider.
func NewPasswordProvider(logger *utils.Logger) *PasswordProvider {
	return &PasswordProvider{
		logger: logger.Named("password_provider"),
	}
}

// HashPassword hashes a password for secure storage.
func (p *PasswordProvider) HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		p.logger.Error("Failed to hash password", err)
		return "", ErrHashingPassword
	}
	return string(hashedBytes), nil
}

// VerifyPassword checks if a password matches a hash.
func (p *PasswordProvider) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		p.logger.Debug("Password verification failed", "error", err)
		return false
	}
	return true
}
