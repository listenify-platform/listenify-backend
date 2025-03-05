// Package rpc provides WebSocket-based RPC functionality.
package rpc

import (
	"context"
	"encoding/json"

	"norelock.dev/listenify/backend/internal/utils"
)

// Handler is an interface for handling RPC requests.
type Handler interface {
	// Handle handles an RPC request and returns a response.
	Handle(ctx context.Context, client *Client, request *Request) *Response
}

// RPCHandler is an implementation of the Handler interface.
type RPCHandler struct {
	// router is the router used to route requests to handlers.
	router *Router

	// logger is the handler's logger.
	logger *utils.Logger
}

// NewHandler creates a new RPC handler.
func NewHandler(logger *utils.Logger) *RPCHandler {
	return &RPCHandler{
		router: NewRouter(logger),
		logger: logger.Named("handler"),
	}
}

// Handle handles an RPC request and returns a response.
func (h *RPCHandler) Handle(ctx context.Context, client *Client, request *Request) *Response {
	// Validate request
	if request.JSONRPC != "2.0" {
		h.logger.Warn("Invalid JSON-RPC version", "version", request.JSONRPC)
		return NewErrorResponse(request.ID, ErrInvalidRequest, "Invalid JSON-RPC version", nil)
	}

	if request.Method == "" {
		h.logger.Warn("Missing method")
		return NewErrorResponse(request.ID, ErrInvalidRequest, "Missing method", nil)
	}

	// Route request to handler
	return h.router.Route(client, request)
}

// HandleRequest handles an RPC request and returns a response.
// This is a convenience function that creates a new handler and calls Handle.
func HandleRequest(ctx context.Context, client *Client, request *Request, logger *utils.Logger) *Response {
	handler := NewHandler(logger)
	return handler.Handle(ctx, client, request)
}

// HandleMessage handles an RPC message and returns a response.
// This is a convenience function that parses the message and calls HandleRequest.
func HandleMessage(ctx context.Context, client *Client, message []byte, logger *utils.Logger) *Response {
	// Parse message
	var request Request
	if err := json.Unmarshal(message, &request); err != nil {
		logger.Error("Failed to parse message", err)
		return NewErrorResponse(nil, ErrParseError, "Parse error", nil)
	}

	return HandleRequest(ctx, client, &request, logger)
}
