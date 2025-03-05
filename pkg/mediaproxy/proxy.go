// Package mediaproxy provides media content proxying and caching.
package mediaproxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Proxy errors
var (
	// ErrInvalidURL is returned when an invalid URL is provided.
	ErrInvalidURL = errors.New("invalid URL")

	// ErrFetchFailed is returned when fetching content from a source fails.
	ErrFetchFailed = errors.New("failed to fetch content")

	// ErrUnsupportedSource is returned when the source is not supported.
	ErrUnsupportedSource = errors.New("unsupported source")

	// ErrNotFound is returned when the requested content is not found.
	ErrNotFound = errors.New("content not found")

	// ErrForbidden is returned when access to the requested content is forbidden.
	ErrForbidden = errors.New("access forbidden")
)

// Source represents a media source.
type Source string

// Media sources
const (
	// YouTube is a media source.
	YouTube Source = "youtube"

	// SoundCloud is a media source.
	SoundCloud Source = "soundcloud"

	// Direct is a direct URL media source.
	Direct Source = "direct"
)

// ProxyRequest represents a request to proxy media content.
type ProxyRequest struct {
	// Source is the media source.
	Source Source

	// ID is the media ID.
	ID string

	// URL is the direct URL for Direct sources.
	URL string

	// Range is the HTTP range header value for range requests.
	Range string

	// Headers are additional HTTP headers to include in the request.
	Headers map[string]string
}

// ProxyResponse represents a response from a proxy request.
type ProxyResponse struct {
	// Content is the media content.
	Content io.ReadSeeker

	// ContentType is the MIME type of the content.
	ContentType string

	// ContentLength is the length of the content in bytes.
	ContentLength int64

	// StatusCode is the HTTP status code.
	StatusCode int

	// Headers are additional HTTP headers to include in the response.
	Headers map[string]string
}

// Proxy is an interface for proxying media content.
type Proxy interface {
	// Proxy proxies media content from a source.
	Proxy(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)

	// ServeHTTP implements the http.Handler interface.
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// MediaProxy is an implementation of the Proxy interface.
type MediaProxy struct {
	// cache is the cache used to store media content.
	cache Cache

	// httpClient is the HTTP client used to fetch content.
	httpClient *http.Client

	// sources is a map of source names to source handlers.
	sources map[Source]SourceHandler

	// defaultTTL is the default time-to-live for cached content.
	defaultTTL time.Duration
}

// SourceHandler is a function that handles a specific media source.
type SourceHandler func(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error)

// MediaProxyOption is a function that configures a MediaProxy.
type MediaProxyOption func(*MediaProxy)

// WithCache sets the cache used to store media content.
func WithCache(cache Cache) MediaProxyOption {
	return func(p *MediaProxy) {
		p.cache = cache
	}
}

// WithHTTPClient sets the HTTP client used to fetch content.
func WithHTTPClient(httpClient *http.Client) MediaProxyOption {
	return func(p *MediaProxy) {
		p.httpClient = httpClient
	}
}

// WithProxyDefaultTTL sets the default time-to-live for cached content.
func WithProxyDefaultTTL(ttl time.Duration) MediaProxyOption {
	return func(p *MediaProxy) {
		p.defaultTTL = ttl
	}
}

// WithSourceHandler adds a source handler for a specific media source.
func WithSourceHandler(source Source, handler SourceHandler) MediaProxyOption {
	return func(p *MediaProxy) {
		p.sources[source] = handler
	}
}

// NewMediaProxy creates a new media proxy.
func NewMediaProxy(options ...MediaProxyOption) *MediaProxy {
	proxy := &MediaProxy{
		cache:      NewMemoryCache(),
		httpClient: http.DefaultClient,
		sources:    make(map[Source]SourceHandler),
		defaultTTL: 24 * time.Hour,
	}

	// Register default source handlers
	proxy.sources[Direct] = proxy.handleDirect
	proxy.sources[YouTube] = proxy.handleYouTube
	proxy.sources[SoundCloud] = proxy.handleSoundCloud

	// Apply options
	for _, option := range options {
		option(proxy)
	}

	return proxy
}

// Proxy proxies media content from a source.
func (p *MediaProxy) Proxy(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
	// Validate request
	if req.Source == "" {
		return nil, ErrUnsupportedSource
	}

	// Check if the source is supported
	handler, ok := p.sources[req.Source]
	if !ok {
		return nil, ErrUnsupportedSource
	}

	// Generate cache key
	cacheKey := p.generateCacheKey(req)

	// Check if the content is cached
	if req.Range == "" {
		entry, ok := p.cache.Get(ctx, cacheKey)
		if ok {
			// Return cached content
			reader := NewCacheReader(entry)
			return &ProxyResponse{
				Content:       reader,
				ContentType:   entry.ContentType,
				ContentLength: int64(len(entry.Content)),
				StatusCode:    http.StatusOK,
				Headers:       make(map[string]string),
			}, nil
		}
	}

	// Fetch content from source
	res, err := handler(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache content if it's not a range request
	if req.Range == "" && res.StatusCode == http.StatusOK {
		// Read content
		content, err := io.ReadAll(res.Content)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
		}

		// Reset content reader
		if _, err := res.Content.Seek(0, io.SeekStart); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
		}

		// Cache content
		err = p.cache.Set(ctx, cacheKey, content, res.ContentType, p.defaultTTL)
		if err != nil {
			// Log error but continue
			fmt.Printf("Failed to cache content: %v\n", err)
		}
	}

	return res, nil
}

// ServeHTTP implements the http.Handler interface.
func (p *MediaProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req, err := p.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Proxy content
	res, err := p.Proxy(r.Context(), req)
	if err != nil {
		// Handle specific errors
		switch {
		case errors.Is(err, ErrNotFound):
			http.Error(w, "Not Found", http.StatusNotFound)
		case errors.Is(err, ErrForbidden):
			http.Error(w, "Forbidden", http.StatusForbidden)
		case errors.Is(err, ErrUnsupportedSource):
			http.Error(w, "Unsupported Source", http.StatusBadRequest)
		case errors.Is(err, ErrInvalidURL):
			http.Error(w, "Invalid URL", http.StatusBadRequest)
		default:
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	// Set headers
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(res.ContentLength, 10))
	for key, value := range res.Headers {
		w.Header().Set(key, value)
	}

	// Set status code
	w.WriteHeader(res.StatusCode)

	// Copy content
	_, _ = io.Copy(w, res.Content)
}

// parseRequest parses an HTTP request into a ProxyRequest.
func (p *MediaProxy) parseRequest(r *http.Request) (*ProxyRequest, error) {
	// Parse URL
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 {
		return nil, ErrInvalidURL
	}

	// Parse source
	source := Source(parts[0])
	if source == "" {
		return nil, ErrUnsupportedSource
	}

	// Create request
	req := &ProxyRequest{
		Source:  source,
		Range:   r.Header.Get("Range"),
		Headers: make(map[string]string),
	}

	// Parse ID or URL
	if source == Direct {
		if len(parts) < 2 {
			return nil, ErrInvalidURL
		}
		// URL is base64 encoded
		urlStr, err := url.QueryUnescape(parts[1])
		if err != nil {
			return nil, ErrInvalidURL
		}
		req.URL = urlStr
	} else {
		req.ID = parts[1]
	}

	// Copy headers
	for key, values := range r.Header {
		if len(values) > 0 {
			req.Headers[key] = values[0]
		}
	}

	return req, nil
}

// generateCacheKey generates a cache key for a proxy request.
func (p *MediaProxy) generateCacheKey(req *ProxyRequest) string {
	switch req.Source {
	case Direct:
		return fmt.Sprintf("direct:%s", req.URL)
	default:
		return fmt.Sprintf("%s:%s", req.Source, req.ID)
	}
}

// handleDirect handles direct URL sources.
func (p *MediaProxy) handleDirect(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
	// Validate URL
	if req.URL == "" {
		return nil, ErrInvalidURL
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	// Add headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Add range header if present
	if req.Range != "" {
		httpReq.Header.Set("Range", req.Range)
	}

	// Send request
	httpRes, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}

	// Check status code
	switch httpRes.StatusCode {
	case http.StatusOK, http.StatusPartialContent:
		// OK
	case http.StatusNotFound:
		httpRes.Body.Close()
		return nil, ErrNotFound
	case http.StatusForbidden:
		httpRes.Body.Close()
		return nil, ErrForbidden
	default:
		httpRes.Body.Close()
		return nil, fmt.Errorf("%w: status code %d", ErrFetchFailed, httpRes.StatusCode)
	}

	// Create response
	res := &ProxyResponse{
		Content:       &httpResponseReader{httpRes.Body},
		ContentType:   httpRes.Header.Get("Content-Type"),
		ContentLength: httpRes.ContentLength,
		StatusCode:    httpRes.StatusCode,
		Headers:       make(map[string]string),
	}

	// Copy headers
	for key, values := range httpRes.Header {
		if len(values) > 0 {
			res.Headers[key] = values[0]
		}
	}

	return res, nil
}

// handleYouTube handles YouTube sources.
func (p *MediaProxy) handleYouTube(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
	// This is a placeholder implementation
	// In a real implementation, you would use a YouTube API client to get the video URL
	// and then proxy the content from that URL
	return nil, ErrUnsupportedSource
}

// handleSoundCloud handles SoundCloud sources.
func (p *MediaProxy) handleSoundCloud(ctx context.Context, req *ProxyRequest) (*ProxyResponse, error) {
	// This is a placeholder implementation
	// In a real implementation, you would use a SoundCloud API client to get the track URL
	// and then proxy the content from that URL
	return nil, ErrUnsupportedSource
}

// httpResponseReader is a wrapper around an http.Response.Body that implements io.ReadSeeker.
type httpResponseReader struct {
	reader io.ReadCloser
}

// Read implements the io.Reader interface.
func (r *httpResponseReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

// Seek implements the io.Seeker interface.
// Note: This is a dummy implementation that always returns an error.
// HTTP response bodies cannot be seeked unless the entire response is buffered.
func (r *httpResponseReader) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.New("seek not supported")
}

// Close implements the io.Closer interface.
func (r *httpResponseReader) Close() error {
	return r.reader.Close()
}
