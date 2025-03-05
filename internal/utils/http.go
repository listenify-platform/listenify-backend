// Package utils provides utility functions used throughout the application.
package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-playground/validator/v10"
)

// APIResponse represents a standard API response.
type APIResponse struct {
	Success bool `json:"success"`
	Data    any  `json:"data,omitempty"`
	Error   any  `json:"error,omitempty"`
}

// ValidationErrorItem represents a single validation error.
type ValidationErrorItem struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// RespondWithJSON sends a JSON response with the given status code and data.
func RespondWithJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			// If encoding fails, log error and send simple error message
			GetLogger().Error("Failed to encode JSON response", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"Internal server error"}`))
		}
	}
}

// RespondWithError sends an error response with the given status code and message.
func RespondWithError(w http.ResponseWriter, statusCode int, message string) {
	response := APIResponse{
		Success: false,
		Error: map[string]string{
			"message": message,
		},
	}
	RespondWithJSON(w, statusCode, response)
}

// RespondWithValidationError sends a validation error response.
func RespondWithValidationError(w http.ResponseWriter, err error) {
	var validationErrors []ValidationErrorItem

	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrs {
			field := e.Field()
			// Convert field name to camelCase for JSON
			if len(field) > 0 {
				field = string(field[0]-'A'+'a') + field[1:]
			}

			var message string
			switch e.Tag() {
			case "required":
				message = field + " is required"
			case "email":
				message = field + " must be a valid email address"
			case "min":
				message = field + " must be at least " + e.Param() + " characters long"
			case "max":
				message = field + " must be at most " + e.Param() + " characters long"
			case "url":
				message = field + " must be a valid URL"
			case "username":
				message = field + " must be a valid username"
			case "password":
				message = field + " must meet the password requirements"
			default:
				message = field + " failed validation: " + e.Tag()
			}

			validationErrors = append(validationErrors, ValidationErrorItem{
				Field:   field,
				Message: message,
			})
		}
	} else {
		// If it's not a validation error, treat it as a general error
		validationErrors = append(validationErrors, ValidationErrorItem{
			Field:   "general",
			Message: err.Error(),
		})
	}

	response := APIResponse{
		Success: false,
		Error: map[string]any{
			"message": "Validation failed",
			"errors":  validationErrors,
		},
	}

	RespondWithJSON(w, http.StatusBadRequest, response)
}

// ExtractBearerToken extracts the Bearer token from the Authorization header.
// It returns the token and nil if successful, or an empty string and an error
// with a descriptive message if the token is missing or has invalid format.
func ExtractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", fmt.Errorf("no token provided")
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		return "", fmt.Errorf("invalid token format")
	}

	return tokenParts[1], nil
}
