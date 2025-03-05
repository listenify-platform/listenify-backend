// Package rpc provides WebSocket-based RPC functionality.
package rpc

import "encoding/json"

// Request represents a JSON-RPC 2.0 request.
type Request struct {
	// JSONRPC is the version of the JSON-RPC protocol. Must be "2.0".
	JSONRPC string `json:"jsonrpc"`

	// Method is the name of the method to be invoked.
	Method string `json:"method"`

	// Params is the parameter values to be used during the invocation of the method.
	Params json.RawMessage `json:"params,omitempty"`

	// ID is the identifier established by the client. If omitted, the request is a notification.
	ID any `json:"id,omitempty"`
}

// Response represents a JSON-RPC 2.0 response.
type Response struct {
	// JSONRPC is the version of the JSON-RPC protocol. Must be "2.0".
	JSONRPC string `json:"jsonrpc"`

	// Result is the result of the method invocation. Must be null if there was an error.
	Result any `json:"result,omitempty"`

	// Error is the error object if there was an error invoking the method. Must be null if there was no error.
	Error *Error `json:"error,omitempty"`

	// ID is the identifier established by the client. Must be null if there was an error in detecting the id in the request.
	ID any `json:"id"`
}

type Notification struct {
	// JSONRPC is the version of the JSON-RPC protocol. Must be "2.0".
	JSONRPC string `json:"jsonrpc"`

	// Method is the name of the method to be invoked.
	Method string `json:"method"`

	// Params is the parameter values to be used during the invocation of the method.
	Params any `json:"params,omitempty"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	// Code is the error code.
	Code ErrorCode `json:"code"`

	// Message is a short description of the error.
	Message string `json:"message"`

	// Data is additional information about the error.
	Data any `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.Message
}

// NewResponse creates a new JSON-RPC 2.0 response.
func NewResponse(id any, result any) *Response {
	return &Response{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
}

// NewErrorResponse creates a new JSON-RPC 2.0 error response.
func NewErrorResponse(id any, code ErrorCode, message string, data any) *Response {
	return &Response{
		JSONRPC: "2.0",
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}
}

// IsNotification returns true if the request is a notification (no ID).
func (r *Request) IsNotification() bool {
	return r.ID == nil
}
