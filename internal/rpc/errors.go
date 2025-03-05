// Package rpc provides WebSocket-based RPC functionality.
package rpc

import (
	"fmt"
)

// ErrorCode is a type for JSON-RPC error codes.
type ErrorCode int

// JSON-RPC 2.0 error codes
const (
	// Parse error: Invalid JSON was received by the server.
	ErrParseError ErrorCode = -32700

	// Invalid Request: The JSON sent is not a valid Request object.
	ErrInvalidRequest ErrorCode = -32600

	// Method not found: The method does not exist / is not available.
	ErrMethodNotFound ErrorCode = -32601

	// Invalid params: Invalid method parameter(s).
	ErrInvalidParams ErrorCode = -32602

	// Internal error: Internal JSON-RPC error.
	ErrInternalError ErrorCode = -32603

	// Server error: Reserved for implementation-defined server-errors.
	ErrServerError ErrorCode = -32000

	// Authentication error: The client is not authenticated.
	ErrAuthenticationRequired ErrorCode = -32001

	// Authorization error: The client is not authorized to perform the requested action.
	ErrNotAuthorized ErrorCode = -32002

	// Rate limit exceeded: The client has exceeded the rate limit.
	ErrRateLimitExceeded ErrorCode = -32003

	// Invalid token: The provided token is invalid.
	ErrInvalidToken ErrorCode = -32004

	// Session expired: The client's session has expired.
	ErrSessionExpired ErrorCode = -32005

	// Room not found: The requested room does not exist.
	ErrRoomNotFound ErrorCode = -32100

	// Room full: The room is full and cannot accept more users.
	ErrRoomFull ErrorCode = -32101

	// Room closed: The room is closed and cannot be joined.
	ErrRoomClosed ErrorCode = -32102

	// User not in room: The user is not in the room.
	ErrUserNotInRoom ErrorCode = -32103

	// User already in room: The user is already in the room.
	ErrUserAlreadyInRoom ErrorCode = -32104

	// Media not found: The requested media does not exist.
	ErrMediaNotFound ErrorCode = -32200

	// Media unavailable: The media is unavailable.
	ErrMediaUnavailable ErrorCode = -32201

	// Playlist not found: The requested playlist does not exist.
	ErrPlaylistNotFound ErrorCode = -32300

	// Playlist item not found: The requested playlist item does not exist.
	ErrPlaylistItemNotFound ErrorCode = -32301

	// User not found: The requested user does not exist.
	ErrUserNotFound ErrorCode = -32400

	// User already exists: A user with the same username or email already exists.
	ErrUserAlreadyExists ErrorCode = -32401
)

// String returns a string representation of the error code.
func (c ErrorCode) String() string {
	switch c {
	case ErrParseError:
		return "Parse error"
	case ErrInvalidRequest:
		return "Invalid request"
	case ErrMethodNotFound:
		return "Method not found"
	case ErrInvalidParams:
		return "Invalid params"
	case ErrInternalError:
		return "Internal error"
	case ErrServerError:
		return "Server error"
	case ErrAuthenticationRequired:
		return "Authentication required"
	case ErrNotAuthorized:
		return "Not authorized"
	case ErrRateLimitExceeded:
		return "Rate limit exceeded"
	case ErrInvalidToken:
		return "Invalid token"
	case ErrSessionExpired:
		return "Session expired"
	case ErrRoomNotFound:
		return "Room not found"
	case ErrRoomFull:
		return "Room full"
	case ErrRoomClosed:
		return "Room closed"
	case ErrUserNotInRoom:
		return "User not in room"
	case ErrUserAlreadyInRoom:
		return "User already in room"
	case ErrMediaNotFound:
		return "Media not found"
	case ErrMediaUnavailable:
		return "Media unavailable"
	case ErrPlaylistNotFound:
		return "Playlist not found"
	case ErrPlaylistItemNotFound:
		return "Playlist item not found"
	case ErrUserNotFound:
		return "User not found"
	case ErrUserAlreadyExists:
		return "User already exists"
	default:
		return fmt.Sprintf("Error code %d", c)
	}
}

// Error conbines an error code, message, and no data.
func (c ErrorCode) Error() error {
	return &Error{
		Code:    c,
		Message: c.String(),
	}
}

// ErrorWith combines an error code, message, and data.
func (c ErrorCode) ErrorWith(data any) error {
	return &Error{
		Code:    c,
		Message: c.String(),
		Data:    data,
	}
}

// NewError creates a new Error with the given code, message, and data.
func NewError(code ErrorCode, message string, data any) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewErrorWithCode creates a new Error with the given code and default message.
// func NewErrorWithCode(code ErrorCode) *Error {
// 	return &Error{
// 		Code:    code,
// 		Message: ErrorCode(code).String(),
// 	}
// }

// NewErrorWithData creates a new Error with the given code, default message, and data.
func NewErrorWithData(code ErrorCode, data any) *Error {
	return &Error{
		Code:    code,
		Message: ErrorCode(code).String(),
		Data:    data,
	}
}

// NewParseError creates a new parse error.
func NewParseError(err error) *Error {
	return &Error{
		Code:    ErrParseError,
		Message: fmt.Sprintf("Parse error: %v", err),
	}
}

// NewInvalidRequestError creates a new invalid request error.
func NewInvalidRequestError(message string) *Error {
	return &Error{
		Code:    ErrInvalidRequest,
		Message: fmt.Sprintf("Invalid request: %s", message),
	}
}

// NewMethodNotFoundError creates a new method not found error.
func NewMethodNotFoundError(method string) *Error {
	return &Error{
		Code:    ErrMethodNotFound,
		Message: fmt.Sprintf("Method not found: %s", method),
	}
}

// NewInvalidParamsError creates a new invalid params error.
func NewInvalidParamsError(err error) *Error {
	return &Error{
		Code:    ErrInvalidParams,
		Message: fmt.Sprintf("Invalid params: %v", err),
	}
}

// NewInternalError creates a new internal error.
func NewInternalError(err error) *Error {
	return &Error{
		Code:    ErrInternalError,
		Message: fmt.Sprintf("Internal error: %v", err),
	}
}

// NewAuthenticationRequiredError creates a new authentication required error.
func NewAuthenticationRequiredError() *Error {
	return &Error{
		Code:    ErrAuthenticationRequired,
		Message: "Authentication required",
	}
}

// NewNotAuthorizedError creates a new not authorized error.
func NewNotAuthorizedError() *Error {
	return &Error{
		Code:    ErrNotAuthorized,
		Message: "Not authorized",
	}
}

// NewRateLimitExceededError creates a new rate limit exceeded error.
func NewRateLimitExceededError() *Error {
	return &Error{
		Code:    ErrRateLimitExceeded,
		Message: "Rate limit exceeded",
	}
}

// NewInvalidTokenError creates a new invalid token error.
func NewInvalidTokenError() *Error {
	return &Error{
		Code:    ErrInvalidToken,
		Message: "Invalid token",
	}
}

// NewSessionExpiredError creates a new session expired error.
func NewSessionExpiredError() *Error {
	return &Error{
		Code:    ErrSessionExpired,
		Message: "Session expired",
	}
}

// IsParseError returns true if the error is a parse error.
func IsParseError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrParseError
	}
	return false
}

// IsInvalidRequestError returns true if the error is an invalid request error.
func IsInvalidRequestError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrInvalidRequest
	}
	return false
}

// IsMethodNotFoundError returns true if the error is a method not found error.
func IsMethodNotFoundError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrMethodNotFound
	}
	return false
}

// IsInvalidParamsError returns true if the error is an invalid params error.
func IsInvalidParamsError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrInvalidParams
	}
	return false
}

// IsInternalError returns true if the error is an internal error.
func IsInternalError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrInternalError
	}
	return false
}

// IsAuthenticationRequiredError returns true if the error is an authentication required error.
func IsAuthenticationRequiredError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrAuthenticationRequired
	}
	return false
}

// IsNotAuthorizedError returns true if the error is a not authorized error.
func IsNotAuthorizedError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrNotAuthorized
	}
	return false
}

// IsRateLimitExceededError returns true if the error is a rate limit exceeded error.
func IsRateLimitExceededError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrRateLimitExceeded
	}
	return false
}

// IsInvalidTokenError returns true if the error is an invalid token error.
func IsInvalidTokenError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrInvalidToken
	}
	return false
}

// IsSessionExpiredError returns true if the error is a session expired error.
func IsSessionExpiredError(err error) bool {
	if rpcErr, ok := err.(*Error); ok {
		return rpcErr.Code == ErrSessionExpired
	}
	return false
}
