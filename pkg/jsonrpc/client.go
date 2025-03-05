// Package jsonrpc provides JSON-RPC 2.0 functionality.
package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"sync"
	"sync/atomic"
)

// Client errors
var (
	ErrClientClosed = errors.New("client closed")
	ErrTimeout      = errors.New("request timeout")
	ErrCanceled     = errors.New("request canceled")
)

// Client is a JSON-RPC 2.0 client.
type Client struct {
	// endpoint is the URL of the JSON-RPC server.
	endpoint string

	// httpClient is the HTTP client used to make requests.
	httpClient *http.Client

	// headers are the HTTP headers to include in requests.
	headers map[string]string

	// nextID is the next request ID.
	nextID int64

	// closed indicates whether the client is closed.
	closed atomic.Bool

	// mutex is used to synchronize access to the headers map.
	mutex sync.RWMutex
}

// ClientOption is a function that configures a Client.
type ClientOption func(*Client)

// WithHTTPClient sets the HTTP client used to make requests.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithHeader adds an HTTP header to include in requests.
func WithHeader(key, value string) ClientOption {
	return func(c *Client) {
		c.headers[key] = value
	}
}

// WithHeaders sets the HTTP headers to include in requests.
func WithHeaders(headers map[string]string) ClientOption {
	return func(c *Client) {
		maps.Copy(c.headers, headers)
	}
}

// NewClient creates a new JSON-RPC 2.0 client.
func NewClient(endpoint string, options ...ClientOption) *Client {
	client := &Client{
		endpoint:   endpoint,
		httpClient: http.DefaultClient,
		headers:    make(map[string]string),
		nextID:     1,
	}

	// Set default headers
	client.headers["Content-Type"] = "application/json"
	client.headers["Accept"] = "application/json"

	// Apply options
	for _, option := range options {
		option(client)
	}

	return client
}

// Call makes a JSON-RPC 2.0 request and returns the response.
func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	if c.closed.Load() {
		return ErrClientClosed
	}

	// Create request
	id := atomic.AddInt64(&c.nextID, 1)
	req, err := NewRequest(method, params, id)
	if err != nil {
		return err
	}

	// Send request
	res, err := c.sendRequest(ctx, req)
	if err != nil {
		return err
	}

	// Check for error
	if res.Error != nil {
		return res.Error
	}

	// Unmarshal result
	if result != nil {
		if err := res.UnmarshalResult(result); err != nil {
			return err
		}
	}

	return nil
}

// Notify makes a JSON-RPC 2.0 notification (a request without an ID).
func (c *Client) Notify(ctx context.Context, method string, params any) error {
	if c.closed.Load() {
		return ErrClientClosed
	}

	// Create notification
	req, err := NewNotification(method, params)
	if err != nil {
		return err
	}

	// Send notification
	_, err = c.sendRequest(ctx, req)
	return err
}

// BatchCall makes a batch of JSON-RPC 2.0 requests and returns the responses.
func (c *Client) BatchCall(ctx context.Context, calls []BatchCall) ([]BatchResponse, error) {
	if c.closed.Load() {
		return nil, ErrClientClosed
	}

	// Create batch request
	batch := make([]*Request, len(calls))
	for i, call := range calls {
		id := atomic.AddInt64(&c.nextID, 1)
		req, err := NewRequest(call.Method, call.Params, id)
		if err != nil {
			return nil, err
		}
		batch[i] = req
	}

	// Send batch request
	resData, err := c.sendBatchRequest(ctx, batch)
	if err != nil {
		return nil, err
	}

	// Parse batch response
	var responses []*Response
	if err := json.Unmarshal(resData, &responses); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	// Map responses to batch calls
	results := make([]BatchResponse, len(calls))
	for _, res := range responses {
		// Find the corresponding call
		for j, call := range calls {
			if res.ID == batch[j].ID {
				// Check for error
				if res.Error != nil {
					results[j] = BatchResponse{
						Error: res.Error,
					}
				} else {
					// Unmarshal result
					result := call.Result
					if result != nil {
						if err := res.UnmarshalResult(result); err != nil {
							results[j] = BatchResponse{
								Error: &Error{
									Code:    ErrInternalError,
									Message: fmt.Sprintf("Failed to unmarshal result: %v", err),
								},
							}
							continue
						}
					}
					results[j] = BatchResponse{
						Result: result,
					}
				}
				break
			}
		}
	}

	return results, nil
}

// Close closes the client.
func (c *Client) Close() error {
	c.closed.Store(true)
	return nil
}

// sendRequest sends a JSON-RPC 2.0 request and returns the response.
func (c *Client) sendRequest(ctx context.Context, req *Request) (*Response, error) {
	// Marshal request
	reqData, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqData))
	if err != nil {
		return nil, err
	}

	// Add headers
	c.mutex.RLock()
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	c.mutex.RUnlock()

	// Send request
	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		if errors.Is(err, context.Canceled) {
			return nil, ErrCanceled
		}
		return nil, err
	}
	defer httpRes.Body.Close()

	// Read response body
	resData, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}

	// Check HTTP status
	if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", httpRes.StatusCode, http.StatusText(httpRes.StatusCode))
	}

	// If this is a notification, there's no response
	if req.IsNotification() {
		return nil, nil
	}

	// Parse response
	res, err := ParseResponse(resData)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// sendBatchRequest sends a batch of JSON-RPC 2.0 requests and returns the response data.
func (c *Client) sendBatchRequest(ctx context.Context, batch []*Request) ([]byte, error) {
	// Marshal batch request
	reqData, err := json.Marshal(batch)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(reqData))
	if err != nil {
		return nil, err
	}

	// Add headers
	c.mutex.RLock()
	for k, v := range c.headers {
		httpReq.Header.Set(k, v)
	}
	c.mutex.RUnlock()

	// Send request
	httpRes, err := c.httpClient.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		if errors.Is(err, context.Canceled) {
			return nil, ErrCanceled
		}
		return nil, err
	}
	defer httpRes.Body.Close()

	// Read response body
	resData, err := io.ReadAll(httpRes.Body)
	if err != nil {
		return nil, err
	}

	// Check HTTP status
	if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d %s", httpRes.StatusCode, http.StatusText(httpRes.StatusCode))
	}

	return resData, nil
}

// BatchCall represents a single call in a batch request.
type BatchCall struct {
	// Method is the name of the method to be invoked.
	Method string

	// Params is the parameter values to be used during the invocation of the method.
	Params any

	// Result is a pointer to a value to store the result of the call.
	Result any
}

// BatchResponse represents a single response in a batch response.
type BatchResponse struct {
	// Result is the result of the method invocation.
	Result any

	// Error is the error object if there was an error invoking the method.
	Error *Error
}
