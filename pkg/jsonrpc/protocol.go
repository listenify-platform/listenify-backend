// Package jsonrpc provides JSON-RPC 2.0 functionality.
package jsonrpc

import (
	"encoding/json"
	"errors"
	"fmt"
)

// JSON-RPC 2.0 error codes
const (
	// Parse error: Invalid JSON was received by the server.
	ErrParseError = -32700

	// Invalid Request: The JSON sent is not a valid Request object.
	ErrInvalidRequest = -32600

	// Method not found: The method does not exist / is not available.
	ErrMethodNotFound = -32601

	// Invalid params: Invalid method parameter(s).
	ErrInvalidParams = -32602

	// Internal error: Internal JSON-RPC error.
	ErrInternalError = -32603

	// Server error: Reserved for implementation-defined server-errors.
	ErrServerError = -32000
)

// Protocol errors
var (
	ErrInvalidJSON     = errors.New("invalid JSON")
	ErrInvalidVersion  = errors.New("invalid JSON-RPC version")
	ErrMissingMethod   = errors.New("missing method")
	ErrMissingID       = errors.New("missing ID")
	ErrInvalidResponse = errors.New("invalid response")
)

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
	Result json.RawMessage `json:"result,omitempty"`

	// Error is the error object if there was an error invoking the method. Must be null if there was no error.
	Error *Error `json:"error,omitempty"`

	// ID is the identifier established by the client. Must be null if there was an error in detecting the id in the request.
	ID any `json:"id"`
}

// Error represents a JSON-RPC 2.0 error object.
type Error struct {
	// Code is the error code.
	Code int `json:"code"`

	// Message is a short description of the error.
	Message string `json:"message"`

	// Data is additional information about the error.
	Data json.RawMessage `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// NewRequest creates a new JSON-RPC 2.0 request.
func NewRequest(method string, params any, id any) (*Request, error) {
	var paramsJSON json.RawMessage
	if params != nil {
		var err error
		paramsJSON, err = json.Marshal(params)
		if err != nil {
			return nil, err
		}
	}

	return &Request{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsJSON,
		ID:      id,
	}, nil
}

// NewNotification creates a new JSON-RPC 2.0 notification (a request without an ID).
func NewNotification(method string, params any) (*Request, error) {
	return NewRequest(method, params, nil)
}

// NewResponse creates a new JSON-RPC 2.0 response.
func NewResponse(id any, result any) (*Response, error) {
	var resultJSON json.RawMessage
	if result != nil {
		var err error
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return nil, err
		}
	}

	return &Response{
		JSONRPC: "2.0",
		Result:  resultJSON,
		ID:      id,
	}, nil
}

// NewErrorResponse creates a new JSON-RPC 2.0 error response.
func NewErrorResponse(id any, code int, message string, data any) (*Response, error) {
	var dataJSON json.RawMessage
	if data != nil {
		var err error
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}

	return &Response{
		JSONRPC: "2.0",
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    dataJSON,
		},
		ID: id,
	}, nil
}

// ParseRequest parses a JSON-RPC 2.0 request from a JSON string.
func ParseRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	if req.JSONRPC != "2.0" {
		return nil, ErrInvalidVersion
	}

	if req.Method == "" {
		return nil, ErrMissingMethod
	}

	return &req, nil
}

// ParseResponse parses a JSON-RPC 2.0 response from a JSON string.
func ParseResponse(data []byte) (*Response, error) {
	var res Response
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	if res.JSONRPC != "2.0" {
		return nil, ErrInvalidVersion
	}

	if res.Error != nil && res.Result != nil {
		return nil, ErrInvalidResponse
	}

	return &res, nil
}

// IsNotification returns true if the request is a notification (no ID).
func (r *Request) IsNotification() bool {
	return r.ID == nil
}

// UnmarshalParams unmarshals the request parameters into the provided value.
func (r *Request) UnmarshalParams(v any) error {
	if r.Params == nil {
		return nil
	}
	return json.Unmarshal(r.Params, v)
}

// UnmarshalResult unmarshals the response result into the provided value.
func (r *Response) UnmarshalResult(v any) error {
	if r.Result == nil {
		return nil
	}
	return json.Unmarshal(r.Result, v)
}

// UnmarshalErrorData unmarshals the error data into the provided value.
func (r *Response) UnmarshalErrorData(v any) error {
	if r.Error == nil || r.Error.Data == nil {
		return nil
	}
	return json.Unmarshal(r.Error.Data, v)
}

// Batch represents a batch of JSON-RPC 2.0 requests or responses.
type Batch struct {
	// Requests is the list of requests in the batch.
	Requests []*Request

	// Responses is the list of responses in the batch.
	Responses []*Response
}

// ParseBatch parses a JSON-RPC 2.0 batch from a JSON string.
func ParseBatch(data []byte) (*Batch, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	batch := &Batch{
		Requests:  make([]*Request, 0),
		Responses: make([]*Response, 0),
	}

	for _, item := range raw {
		// Try to parse as a request
		req, err := ParseRequest(item)
		if err == nil {
			batch.Requests = append(batch.Requests, req)
			continue
		}

		// Try to parse as a response
		res, err := ParseResponse(item)
		if err == nil {
			batch.Responses = append(batch.Responses, res)
			continue
		}

		// If we get here, the item is neither a valid request nor a valid response
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	return batch, nil
}
