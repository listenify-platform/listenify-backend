// Package rpc provides WebSocket-based RPC functionality.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"norelock.dev/listenify/backend/internal/utils"
)

// HandlerFunc is a function that handles an RPC request.
type HandlerFunc func(ctx context.Context, client *Client, params json.RawMessage) (any, error)

type HandlerFuncNoParams func(ctx context.Context, client *Client) (any, error)

func (h HandlerFuncNoParams) handlerFunc() HandlerFunc {
	return func(ctx context.Context, client *Client, params json.RawMessage) (any, error) {
		return h(ctx, client)
	}
}
func RegisterNoParams(hr HandlerRegistry, method string, h HandlerFuncNoParams) {
	hr.Register(method, h.handlerFunc())
}

type HandlerFuncWith[T any] func(ctx context.Context, client *Client, params *T) (any, error)

func (h HandlerFuncWith[T]) handlerFunc() HandlerFunc {
	return func(ctx context.Context, client *Client, params json.RawMessage) (any, error) {
		var p T
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &Error{
				Code:    ErrInvalidParams,
				Message: "Invalid parameters",
				Data:    err.Error(),
			}
		}
		return h(ctx, client, &p)
	}
}

type HandlerRegistry interface {
	Register(method string, handler HandlerFunc)
	Wrap(mw MiddlewareFunc) HandlerRegistry
}

func Register[T any](hr HandlerRegistry, method string, h HandlerFuncWith[T]) {
	hr.Register(method, h.handlerFunc())
}

// Router routes RPC requests to the appropriate handler.
type Router struct {
	// handlers is a map of method names to handler functions.
	handlers map[string]HandlerFunc

	// mutex is used to synchronize access to the handlers map.
	mutex sync.RWMutex

	// logger is the router's logger.
	logger *utils.Logger
}

// MiddlewareFunc is a function that wraps a handler function.
type MiddlewareFunc func(HandlerFunc) HandlerFunc

type HandlerRegWrapped struct {
	inner HandlerRegistry
	mw    MiddlewareFunc
}

// Register registers a handler for a method.
func (h HandlerRegWrapped) Register(method string, handler HandlerFunc) {
	h.inner.Register(method, h.mw(handler))
}

// Wrap wraps the handler registry with middleware.
func (h HandlerRegWrapped) Wrap(mw MiddlewareFunc) HandlerRegistry {
	return HandlerRegWrapped{
		inner: h,
		mw:    mw,
	}
}

// NewRouter creates a new router.
func NewRouter(logger *utils.Logger) *Router {
	return &Router{
		handlers: make(map[string]HandlerFunc),
		logger:   logger.Named("router"),
	}
}

// Register registers a handler for a method.
func (r *Router) Register(method string, handler HandlerFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.handlers[method] = handler
	r.logger.Debug("Registered handler", "method", method)
}

// Wrap wraps the router with middleware.
func (r *Router) Wrap(mw MiddlewareFunc) HandlerRegistry {
	return HandlerRegWrapped{
		inner: r,
		mw:    mw,
	}
}

// Route routes a request to the appropriate handler.
func (r *Router) Route(client *Client, request *Request) *Response {
	r.mutex.RLock()
	handler, ok := r.handlers[request.Method]
	r.mutex.RUnlock()

	if !ok {
		r.logger.Warn("Method not found", "method", request.Method)
		return NewErrorResponse(request.ID, ErrMethodNotFound, fmt.Sprintf("Method '%s' not found", request.Method), nil)
	}

	// Create context with client information
	ctx := context.WithValue(context.Background(), "client", client)
	ctx = context.WithValue(ctx, "userID", client.UserID)
	ctx = context.WithValue(ctx, "username", client.Username)

	// Call the handler
	result, err := handler(ctx, client, request.Params)
	if err != nil {
		r.logger.Error("Handler error", err, "method", request.Method)
		return handleError(request.ID, err)
	}

	// If this is a notification, don't return a response
	if request.IsNotification() {
		return nil
	}

	return NewResponse(request.ID, result)
}

// handleError converts an error to an appropriate error response.
func handleError(id any, err error) *Response {
	// Check if the error is an RPC error
	if rpcErr, ok := err.(*Error); ok {
		return NewErrorResponse(id, rpcErr.Code, rpcErr.Message, rpcErr.Data)
	}

	// Default to internal error
	return NewErrorResponse(id, ErrInternalError, err.Error(), nil)
}

// AuthMiddleware is a middleware that checks if the client is authenticated.
func AuthMiddleware(next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, client *Client, params json.RawMessage) (any, error) {
		if client.UserID == "" {
			return nil, ErrAuthenticationRequired.Error()
		}
		return next(ctx, client, params)
	}
}

// RoleMiddleware creates middleware that checks if the client has the required role.
func RoleMiddleware(role string) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, client *Client, params json.RawMessage) (any, error) {
			// TODO: Implement role checking
			return next(ctx, client, params)
		}
	}
}

// LoggingMiddleware creates middleware that logs requests and responses.
func LoggingMiddleware(logger *utils.Logger) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, client *Client, params json.RawMessage) (any, error) {
			logger.Debug("RPC request", "client", client.ID, "userID", client.UserID)
			result, err := next(ctx, client, params)
			if err != nil {
				logger.Error("RPC error", err, "client", client.ID, "userID", client.UserID)
			} else {
				logger.Debug("RPC response", "client", client.ID, "userID", client.UserID)
			}
			return result, err
		}
	}
}

// RecoveryMiddleware creates middleware that recovers from panics.
func RecoveryMiddleware(logger *utils.Logger) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, client *Client, params json.RawMessage) (any, error) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic recovered", fmt.Errorf("panic: %v", r), "client", client.ID, "userID", client.UserID)
				}
			}()
			return next(ctx, client, params)
		}
	}
}
