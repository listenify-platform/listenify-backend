// Package models contains the data structures used throughout the application.
package models

import (
	"errors"
	"maps"
	"net/http"
	"os"
)

// Common error types for domain-specific errors
var (
	// User errors
	ErrUserNotFound          = errors.New("user not found")
	ErrUserNotInRoom         = errors.New("user not in room")
	ErrUserAlreadyExists     = errors.New("user already exists")
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrEmailAlreadyExists    = errors.New("email already taken")
	ErrUsernameAlreadyExists = errors.New("username already taken")
	ErrAccountLocked         = errors.New("account is locked")
	ErrAccountDisabled       = errors.New("account is disabled")
	ErrEmailNotVerified      = errors.New("email not verified")
	ErrPasswordTooWeak       = errors.New("password does not meet security requirements")
	ErrInvalidUsername       = errors.New("invalid username format")
	ErrUnauthorizedAction    = errors.New("unauthorized action")
	ErrPasswordResetExpired  = errors.New("password reset token expired")
	ErrInvalidID             = errors.New("invalid ID format")

	// Room errors
	ErrRoomNotFound        = errors.New("room not found")
	ErrRoomAlreadyExists   = errors.New("room already exists")
	ErrRoomFull            = errors.New("room is full")
	ErrRoomInactive        = errors.New("room is inactive")
	ErrInvalidRoomPassword = errors.New("invalid room password")
	ErrUserBanned          = errors.New("user is banned from this room")
	ErrUserAlreadyInRoom   = errors.New("user is already in this room")
	ErrMaxRoomsReached     = errors.New("maximum number of rooms reached")

	// DJ queue errors
	ErrQueueFull          = errors.New("DJ queue is full")
	ErrUserNotInQueue     = errors.New("user is not in the DJ queue")
	ErrUserAlreadyInQueue = errors.New("user is already in the DJ queue")
	ErrCannotSkipSelf     = errors.New("cannot skip yourself")
	ErrNotCurrentDJ       = errors.New("user is not the current DJ")

	// Media errors
	ErrMediaNotFound          = errors.New("media not found")
	ErrInvalidMediaType       = errors.New("invalid media type")
	ErrMediaTooLong           = errors.New("media exceeds maximum duration")
	ErrMediaAlreadyExists     = errors.New("media already exists")
	ErrMediaRestricted        = errors.New("media is age-restricted or restricted in some regions")
	ErrMediaSourceUnavailable = errors.New("media source is unavailable")
	ErrMediaCantBeResolved    = errors.New("media URL could not be resolved")

	// Playlist errors
	ErrPlaylistNotFound     = errors.New("playlist not found")
	ErrPlaylistFull         = errors.New("playlist is full")
	ErrPlaylistEmpty        = errors.New("playlist is empty")
	ErrPlaylistItemNotFound = errors.New("playlist item not found")
	ErrPlaylistPrivate      = errors.New("playlist is private")

	// Chat errors
	ErrMessageNotFound        = errors.New("message not found")
	ErrUserMuted              = errors.New("user is muted")
	ErrChatDisabled           = errors.New("chat is disabled in this room")
	ErrMessageTooLong         = errors.New("message exceeds maximum length")
	ErrMessageRateLimited     = errors.New("message rate limit exceeded")
	ErrInvalidCommand         = errors.New("invalid chat command")
	ErrCommandDisabled        = errors.New("command is disabled")
	ErrInsufficientPermission = errors.New("insufficient permission for this command")

	// Validation errors
	ErrInvalidInput         = errors.New("invalid input")
	ErrMissingRequiredField = errors.New("missing required field")
	ErrInvalidFormat        = errors.New("invalid format")

	// Authentication/authorization errors
	ErrAccessDenied    = errors.New("access denied")
	ErrInvalidToken    = errors.New("invalid token")
	ErrTokenExpired    = errors.New("token expired")
	ErrSessionExpired  = errors.New("session expired")
	ErrTooManyRequests = errors.New("too many requests")

	// System errors
	ErrInternalServer     = errors.New("internal server error")
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
	ErrDatabaseError      = errors.New("database error")
	ErrCacheError         = errors.New("cache error")
	ErrNetworkError       = errors.New("network error")
	ErrFeatureDisabled    = errors.New("feature is disabled")
)

// DomainError represents an error that occurs in the application domain.
type DomainError struct {
	// Original is the underlying error
	Original error

	// Message is a human-readable error message
	Message string

	// Code is the HTTP status code
	Code int

	// Domain is the area of the application where the error occurred
	Domain string

	// Details contains additional context for the error
	Details map[string]any
}

// Error returns the error message
func (e *DomainError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Original.Error()
}

// Unwrap returns the underlying error
func (e *DomainError) Unwrap() error {
	return e.Original
}

// NewDomainError creates a new DomainError
func NewDomainError(err error, message string, code int, domain string) *DomainError {
	if message == "" && err != nil {
		message = err.Error()
	}

	return &DomainError{
		Original: err,
		Message:  message,
		Code:     code,
		Domain:   domain,
		Details:  make(map[string]any),
	}
}

// WithDetails adds details to the error
func (e *DomainError) WithDetails(details map[string]any) *DomainError {
	maps.Copy(e.Details, details)
	return e
}

// AddDetail adds a single detail to the error
func (e *DomainError) AddDetail(key string, value any) *DomainError {
	e.Details[key] = value
	return e
}

// NewUserError creates a user-related domain error
func NewUserError(err error, message string, code int) *DomainError {
	return NewDomainError(err, message, code, "user")
}

// NewRoomError creates a room-related domain error
func NewRoomError(err error, message string, code int) *DomainError {
	return NewDomainError(err, message, code, "room")
}

// NewMediaError creates a media-related domain error
func NewMediaError(err error, message string, code int) *DomainError {
	return NewDomainError(err, message, code, "media")
}

// NewPlaylistError creates a playlist-related domain error
func NewPlaylistError(err error, message string, code int) *DomainError {
	return NewDomainError(err, message, code, "playlist")
}

// NewChatError creates a chat-related domain error
func NewChatError(err error, message string, code int) *DomainError {
	return NewDomainError(err, message, code, "chat")
}

// NewAuthError creates an authentication-related domain error
func NewAuthError(err error, message string, code int) *DomainError {
	return NewDomainError(err, message, code, "auth")
}

// NewValidationError creates a validation-related domain error
func NewValidationError(err error, message string) *DomainError {
	return NewDomainError(err, message, http.StatusUnprocessableEntity, "validation")
}

// NewInternalError creates an internal server error
func NewInternalError(err error, message string) *DomainError {
	if message == "" {
		message = "An internal server error occurred"
	}
	return NewDomainError(err, message, http.StatusInternalServerError, "system")
}

// ErrorResponse represents the standard error response format for APIs
type ErrorResponse struct {
	// Success is always false for error responses
	Success bool `json:"success"`

	// Error contains information about the error
	Error struct {
		// Code is the HTTP status code
		Code int `json:"code"`

		// Message is a human-readable error message
		Message string `json:"message"`

		// Domain is the area of the application where the error occurred
		Domain string `json:"domain,omitempty"`

		// Details contains additional context for the error
		Details map[string]any `json:"details,omitempty"`
	} `json:"error"`
}

// NewErrorResponse creates a new ErrorResponse from a DomainError
func NewErrorResponse(err error) ErrorResponse {
	response := ErrorResponse{
		Success: false,
	}

	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		response.Error.Code = domainErr.Code
		response.Error.Message = domainErr.Message
		response.Error.Domain = domainErr.Domain
		response.Error.Details = domainErr.Details
	} else {
		// Handle regular errors
		response.Error.Code = http.StatusInternalServerError
		response.Error.Message = "An unexpected error occurred"

		// Include the original error message in non-production environments
		if os.Getenv("APP_ENV") != "production" {
			response.Error.Details = map[string]any{
				"originalError": err.Error(),
			}
		}
	}

	return response
}

// ValidationErrorResponse represents validation errors for form fields
type ValidationErrorResponse struct {
	// Success is always false for error responses
	Success bool `json:"success"`

	// Error contains information about the validation error
	Error struct {
		// Code is the HTTP status code
		Code int `json:"code"`

		// Message is a human-readable error message
		Message string `json:"message"`

		// Fields maps field names to specific error messages
		Fields map[string]string `json:"fields"`
	} `json:"error"`
}

// NewValidationErrorResponse creates a new ValidationErrorResponse
func NewValidationErrorResponse(fieldErrors map[string]string) ValidationErrorResponse {
	response := ValidationErrorResponse{
		Success: false,
	}

	response.Error.Code = http.StatusUnprocessableEntity
	response.Error.Message = "Validation failed"
	response.Error.Fields = fieldErrors

	return response
}

// MapErrorToHTTPStatus maps common errors to HTTP status codes
func MapErrorToHTTPStatus(err error) int {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}

	switch {
	case errors.Is(err, ErrUserNotFound),
		errors.Is(err, ErrRoomNotFound),
		errors.Is(err, ErrMediaNotFound),
		errors.Is(err, ErrPlaylistNotFound),
		errors.Is(err, ErrPlaylistItemNotFound):
		return http.StatusNotFound

	case errors.Is(err, ErrInvalidCredentials),
		errors.Is(err, ErrInvalidToken),
		errors.Is(err, ErrTokenExpired),
		errors.Is(err, ErrSessionExpired),
		errors.Is(err, ErrEmailNotVerified):
		return http.StatusUnauthorized

	case errors.Is(err, ErrAccessDenied),
		errors.Is(err, ErrUserNotInRoom),
		errors.Is(err, ErrUnauthorizedAction),
		errors.Is(err, ErrInsufficientPermission),
		errors.Is(err, ErrUserBanned):
		return http.StatusForbidden

	case errors.Is(err, ErrUserAlreadyExists),
		errors.Is(err, ErrEmailAlreadyExists),
		errors.Is(err, ErrUsernameAlreadyExists),
		errors.Is(err, ErrUserAlreadyInRoom),
		errors.Is(err, ErrUserAlreadyInQueue):
		return http.StatusConflict

	case errors.Is(err, ErrInvalidInput),
		errors.Is(err, ErrMissingRequiredField),
		errors.Is(err, ErrInvalidFormat),
		errors.Is(err, ErrPasswordTooWeak),
		errors.Is(err, ErrInvalidUsername),
		errors.Is(err, ErrInvalidRoomPassword),
		errors.Is(err, ErrInvalidMediaType),
		errors.Is(err, ErrInvalidCommand):
		return http.StatusBadRequest

	case errors.Is(err, ErrTooManyRequests),
		errors.Is(err, ErrMessageRateLimited):
		return http.StatusTooManyRequests

	case errors.Is(err, ErrRoomFull),
		errors.Is(err, ErrQueueFull),
		errors.Is(err, ErrPlaylistFull),
		errors.Is(err, ErrMaxRoomsReached):
		return http.StatusServiceUnavailable

	default:
		return http.StatusInternalServerError
	}
}

// FormatValidationErrors formats validation errors into a map of field names to error messages
func FormatValidationErrors(err error, fieldErrors map[string]string) map[string]string {
	if fieldErrors == nil {
		fieldErrors = make(map[string]string)
	}

	// Add the general error if it's not already in the map
	if err != nil && len(fieldErrors) == 0 {
		fieldErrors["_error"] = err.Error()
	}

	return fieldErrors
}

// ErrorResponseJSON converts an error to an ErrorResponse in JSON format
func ErrorResponseJSON(err error) any {
	// Check if it's a validation error with field errors
	var validationErr *ValidationErrorResponse
	if errors.As(err, &validationErr) {
		return validationErr
	}

	// Use the standard error response for other errors
	return NewErrorResponse(err)
}
