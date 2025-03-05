// Package utils provides utility functions used throughout the application.
package utils

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
)

// Common error types
var (
	ErrNotFound       = errors.New("resource not found")
	ErrUnauthorized   = errors.New("unauthorized access")
	ErrForbidden      = errors.New("forbidden access")
	ErrBadRequest     = errors.New("invalid request")
	ErrInternalServer = errors.New("internal server error")
	ErrConflict       = errors.New("resource conflict")
	ErrValidation     = errors.New("validation error")
	ErrRateLimited    = errors.New("rate limit exceeded")
)

// AppError represents an application error with context.
// It implements the error interface and can be used to provide
// additional information about an error.
type AppError struct {
	// Original is the underlying error that caused this error
	Original error
	// Message is a human-readable error message
	Message string
	// Code is the HTTP status code that should be returned
	Code int
	// Details contains additional error context
	Details map[string]any
}

// Error returns the error message, satisfying the error interface.
func (e *AppError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Original)
	}
	return e.Message
}

// Unwrap returns the underlying error, supporting errors.Is and errors.As.
func (e *AppError) Unwrap() error {
	return e.Original
}

// WithDetails adds context to the error.
func (e *AppError) WithDetails(details map[string]any) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	maps.Copy(e.Details, details)
	return e
}

// AddDetail adds a single detail to the error.
func (e *AppError) AddDetail(key string, value any) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// NewAppError creates a new AppError.
func NewAppError(err error, message string, code int) *AppError {
	return &AppError{
		Original: err,
		Message:  message,
		Code:     code,
		Details:  make(map[string]any),
	}
}

// NotFoundError creates a new 404 Not Found error.
func NotFoundError(message string, err error) *AppError {
	if message == "" {
		message = "Resource not found"
	}
	return NewAppError(err, message, http.StatusNotFound)
}

// UnauthorizedError creates a new 401 Unauthorized error.
func UnauthorizedError(message string, err error) *AppError {
	if message == "" {
		message = "Unauthorized access"
	}
	return NewAppError(err, message, http.StatusUnauthorized)
}

// ForbiddenError creates a new 403 Forbidden error.
func ForbiddenError(message string, err error) *AppError {
	if message == "" {
		message = "Forbidden access"
	}
	return NewAppError(err, message, http.StatusForbidden)
}

// BadRequestError creates a new 400 Bad Request error.
func BadRequestError(message string, err error) *AppError {
	if message == "" {
		message = "Invalid request"
	}
	return NewAppError(err, message, http.StatusBadRequest)
}

// InternalServerError creates a new 500 Internal Server Error.
func InternalServerError(message string, err error) *AppError {
	if message == "" {
		message = "Internal server error"
	}
	return NewAppError(err, message, http.StatusInternalServerError)
}

// ConflictError creates a new 409 Conflict error.
func ConflictError(message string, err error) *AppError {
	if message == "" {
		message = "Resource conflict"
	}
	return NewAppError(err, message, http.StatusConflict)
}

// ValidationError creates a new 422 Unprocessable Entity error.
func ValidationError(message string, err error) *AppError {
	if message == "" {
		message = "Validation error"
	}
	return NewAppError(err, message, http.StatusUnprocessableEntity)
}

// RateLimitError creates a new 429 Too Many Requests error.
func RateLimitError(message string, err error) *AppError {
	if message == "" {
		message = "Rate limit exceeded"
	}
	return NewAppError(err, message, http.StatusTooManyRequests)
}

// IsNotFound checks if an error is a "not found" error.
func IsNotFound(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == http.StatusNotFound
	}
	return errors.Is(err, ErrNotFound)
}

// IsUnauthorized checks if an error is an "unauthorized" error.
func IsUnauthorized(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == http.StatusUnauthorized
	}
	return errors.Is(err, ErrUnauthorized)
}

// IsForbidden checks if an error is a "forbidden" error.
func IsForbidden(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == http.StatusForbidden
	}
	return errors.Is(err, ErrForbidden)
}

// IsBadRequest checks if an error is a "bad request" error.
func IsBadRequest(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == http.StatusBadRequest
	}
	return errors.Is(err, ErrBadRequest)
}

// HttpError represents an error that contains an HTTP status code.
// This is a simpler error type that can be used when a full AppError is not needed.
type HttpError struct {
	Code    int
	Message string
	Err     error
}

// Error returns the error message, satisfying the error interface.
func (e *HttpError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error, supporting errors.Is and errors.As.
func (e *HttpError) Unwrap() error {
	return e.Err
}

// StatusCode returns the HTTP status code for the error.
func StatusCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}

	var httpErr *HttpError
	if errors.As(err, &httpErr) {
		return httpErr.Code
	}

	// Default error mappings
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrValidation):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrRateLimited):
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// ErrorResponse creates a standardized response format for API errors.
func ErrorResponse(err error) map[string]any {
	var appErr *AppError
	if errors.As(err, &appErr) {
		response := map[string]any{
			"error":   appErr.Message,
			"code":    appErr.Code,
			"details": appErr.Details,
		}
		return response
	}

	var httpErr *HttpError
	if errors.As(err, &httpErr) {
		return map[string]any{
			"error": httpErr.Message,
			"code":  httpErr.Code,
		}
	}

	// Default error response
	return map[string]any{
		"error": err.Error(),
		"code":  StatusCode(err),
	}
}
