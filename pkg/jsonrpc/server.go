// Package jsonrpc provides JSON-RPC 2.0 functionality.
package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sync"
)

// Handler is a function that handles a JSON-RPC request.
type Handler func(ctx context.Context, params json.RawMessage) (any, error)

// Server is a JSON-RPC 2.0 server.
type Server struct {
	// handlers is a map of method names to handlers.
	handlers map[string]Handler

	// middleware is a list of middleware functions to apply to handlers.
	middleware []MiddlewareFunc

	// mutex is used to synchronize access to the handlers map.
	mutex sync.RWMutex
}

// MiddlewareFunc is a function that wraps a Handler.
type MiddlewareFunc func(Handler) Handler

// NewServer creates a new JSON-RPC 2.0 server.
func NewServer() *Server {
	return &Server{
		handlers: make(map[string]Handler),
	}
}

// RegisterMethod registers a method handler.
func (s *Server) RegisterMethod(method string, handler Handler) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.handlers[method] = handler
}

// RegisterFunc registers a function as a method handler.
// The function must have the signature:
//
//	func(ctx context.Context, params *T) (R, error)
//
// where T is the type of the parameters and R is the type of the result.
func (s *Server) RegisterFunc(method string, fn any) error {
	// Get the function type
	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("handler must be a function")
	}

	// Check the function signature
	if fnType.NumIn() != 2 {
		return fmt.Errorf("handler must have 2 input parameters")
	}
	if fnType.NumOut() != 2 {
		return fmt.Errorf("handler must have 2 output parameters")
	}
	if fnType.In(0) != reflect.TypeOf((*context.Context)(nil)).Elem() {
		return fmt.Errorf("first parameter must be context.Context")
	}
	if fnType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return fmt.Errorf("second output parameter must be error")
	}

	// Create a handler that calls the function
	handler := func(ctx context.Context, params json.RawMessage) (any, error) {
		// Create a new instance of the parameter type
		paramType := fnType.In(1)
		paramValue := reflect.New(paramType)
		if paramType.Kind() != reflect.Ptr {
			paramValue = reflect.New(paramType).Elem()
		}

		// Unmarshal the parameters
		if params != nil {
			if err := json.Unmarshal(params, paramValue.Interface()); err != nil {
				return nil, &Error{
					Code:    ErrInvalidParams,
					Message: fmt.Sprintf("Invalid params: %v", err),
				}
			}
		}

		// Call the function
		fnValue := reflect.ValueOf(fn)
		args := []reflect.Value{reflect.ValueOf(ctx), paramValue}
		results := fnValue.Call(args)

		// Check for error
		if !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}

		// Return the result
		return results[0].Interface(), nil
	}

	// Register the handler
	s.RegisterMethod(method, handler)
	return nil
}

// Use adds middleware to the server.
func (s *Server) Use(middleware ...MiddlewareFunc) {
	s.middleware = append(s.middleware, middleware...)
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check HTTP method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}

	// Parse request
	var req any
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, nil, &Error{
			Code:    ErrParseError,
			Message: fmt.Sprintf("Parse error: %v", err),
		})
		return
	}

	// Check if it's a batch request
	if batch, ok := req.([]any); ok {
		s.handleBatch(w, r, batch)
		return
	}

	// Handle single request
	reqObj, err := ParseRequest(body)
	if err != nil {
		writeError(w, nil, err.(*Error))
		return
	}

	// Handle request
	res := s.handleRequest(r.Context(), reqObj)

	// Write response
	if res != nil {
		writeResponse(w, res)
	} else {
		// No response for notifications
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleBatch handles a batch of requests.
func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request, batch []any) {
	// Parse batch requests
	requests := make([]*Request, 0, len(batch))
	for _, item := range batch {
		data, err := json.Marshal(item)
		if err != nil {
			writeError(w, nil, &Error{
				Code:    ErrInternalError,
				Message: fmt.Sprintf("Internal error: %v", err),
			})
			return
		}

		req, err := ParseRequest(data)
		if err != nil {
			// Skip invalid requests in batch
			continue
		}

		requests = append(requests, req)
	}

	// Handle each request
	responses := make([]*Response, 0, len(requests))
	for _, req := range requests {
		res := s.handleRequest(r.Context(), req)
		if res != nil {
			responses = append(responses, res)
		}
	}

	// Write responses
	if len(responses) > 0 {
		writeResponses(w, responses)
	} else {
		// No responses for all notifications
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleRequest handles a single request.
func (s *Server) handleRequest(ctx context.Context, req *Request) *Response {
	// Check if it's a notification
	if req.IsNotification() {
		// Handle notification
		s.handleNotification(ctx, req)
		return nil
	}

	// Find handler
	s.mutex.RLock()
	handler, ok := s.handlers[req.Method]
	s.mutex.RUnlock()

	if !ok {
		// Method not found
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    ErrMethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}

	// Apply middleware
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}

	// Call handler
	result, err := handler(ctx, req.Params)
	if err != nil {
		// Convert error to Error
		var rpcErr *Error
		if errors.As(err, &rpcErr) {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   rpcErr,
			}
		}

		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    ErrInternalError,
				Message: err.Error(),
			},
		}
	}

	// Marshal result to json.RawMessage
	var resultJSON json.RawMessage
	if result != nil {
		var err error
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    ErrInternalError,
					Message: fmt.Sprintf("Error marshaling result: %v", err),
				},
			}
		}
	}

	// Return response
	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}
}

// handleNotification handles a notification.
func (s *Server) handleNotification(ctx context.Context, req *Request) {
	// Find handler
	s.mutex.RLock()
	handler, ok := s.handlers[req.Method]
	s.mutex.RUnlock()

	if !ok {
		// Method not found, ignore notification
		return
	}

	// Apply middleware
	for i := len(s.middleware) - 1; i >= 0; i-- {
		handler = s.middleware[i](handler)
	}

	// Call handler
	_, _ = handler(ctx, req.Params)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, id any, err *Error) {
	res := &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}
	writeResponse(w, res)
}

// writeResponse writes a response.
func writeResponse(w http.ResponseWriter, res *Response) {
	// Marshal response
	data, err := json.Marshal(res)
	if err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// writeResponses writes a batch of responses.
func writeResponses(w http.ResponseWriter, responses []*Response) {
	// Marshal responses
	data, err := json.Marshal(responses)
	if err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}

	// Write responses
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
