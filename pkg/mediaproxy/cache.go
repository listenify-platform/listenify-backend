// Package mediaproxy provides media content proxying and caching.
package mediaproxy

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

// Cache errors
var (
	// ErrCacheTooLarge is returned when an item is too large to be stored in the cache.
	ErrCacheTooLarge = errors.New("item too large for cache")

	// ErrInvalidWhence is returned when an invalid whence value is provided to Seek.
	ErrInvalidWhence = errors.New("invalid whence value")

	// ErrNegativeOffset is returned when a negative offset is provided to Seek.
	ErrNegativeOffset = errors.New("negative offset")
)

// CacheEntry represents a cached media item.
type CacheEntry struct {
	// Content is the cached media content.
	Content []byte

	// ContentType is the MIME type of the content.
	ContentType string

	// ExpiresAt is the time when the cache entry expires.
	ExpiresAt time.Time
}

// IsExpired returns true if the cache entry has expired.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// Cache is an interface for caching media content.
type Cache interface {
	// Get retrieves a cached item by key.
	Get(ctx context.Context, key string) (*CacheEntry, bool)

	// Set stores an item in the cache with the given key.
	Set(ctx context.Context, key string, content []byte, contentType string, ttl time.Duration) error

	// Delete removes an item from the cache.
	Delete(ctx context.Context, key string) error

	// Clear removes all items from the cache.
	Clear(ctx context.Context) error
}

// MemoryCache is an in-memory implementation of the Cache interface.
type MemoryCache struct {
	// items is a map of cache keys to cache entries.
	items map[string]*CacheEntry

	// mutex is used to synchronize access to the items map.
	mutex sync.RWMutex

	// defaultTTL is the default time-to-live for cache entries.
	defaultTTL time.Duration

	// maxSize is the maximum size of the cache in bytes.
	maxSize int64

	// currentSize is the current size of the cache in bytes.
	currentSize int64
}

// MemoryCacheOption is a function that configures a MemoryCache.
type MemoryCacheOption func(*MemoryCache)

// WithDefaultTTL sets the default time-to-live for cache entries.
func WithDefaultTTL(ttl time.Duration) MemoryCacheOption {
	return func(c *MemoryCache) {
		c.defaultTTL = ttl
	}
}

// WithMaxSize sets the maximum size of the cache in bytes.
func WithMaxSize(maxSize int64) MemoryCacheOption {
	return func(c *MemoryCache) {
		c.maxSize = maxSize
	}
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(options ...MemoryCacheOption) *MemoryCache {
	cache := &MemoryCache{
		items:      make(map[string]*CacheEntry),
		defaultTTL: 1 * time.Hour,
		maxSize:    100 * 1024 * 1024, // 100 MB
	}

	// Apply options
	for _, option := range options {
		option(cache)
	}

	return cache
}

// Get retrieves a cached item by key.
func (c *MemoryCache) Get(ctx context.Context, key string) (*CacheEntry, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Check if the entry has expired
	if entry.IsExpired() {
		// Remove the expired entry
		go func() {
			c.mutex.Lock()
			defer c.mutex.Unlock()
			if entry, ok := c.items[key]; ok && entry.IsExpired() {
				c.currentSize -= int64(len(entry.Content))
				delete(c.items, key)
			}
		}()
		return nil, false
	}

	return entry, true
}

// Set stores an item in the cache with the given key.
func (c *MemoryCache) Set(ctx context.Context, key string, content []byte, contentType string, ttl time.Duration) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Use default TTL if not specified
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	// Calculate the size of the new entry
	newSize := int64(len(content))

	// Check if the entry would exceed the maximum cache size
	if newSize > c.maxSize {
		return ErrCacheTooLarge
	}

	// Remove the old entry if it exists
	if oldEntry, ok := c.items[key]; ok {
		c.currentSize -= int64(len(oldEntry.Content))
	}

	// Check if adding the new entry would exceed the maximum cache size
	if c.currentSize+newSize > c.maxSize {
		// Evict entries until there's enough space
		c.evict(newSize)
	}

	// Add the new entry
	c.items[key] = &CacheEntry{
		Content:     content,
		ContentType: contentType,
		ExpiresAt:   time.Now().Add(ttl),
	}
	c.currentSize += newSize

	return nil
}

// Delete removes an item from the cache.
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if entry, ok := c.items[key]; ok {
		c.currentSize -= int64(len(entry.Content))
		delete(c.items, key)
	}

	return nil
}

// Clear removes all items from the cache.
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*CacheEntry)
	c.currentSize = 0

	return nil
}

// evict removes entries from the cache until there's enough space for a new entry.
func (c *MemoryCache) evict(size int64) {
	// First, remove expired entries
	for key, entry := range c.items {
		if entry.IsExpired() {
			c.currentSize -= int64(len(entry.Content))
			delete(c.items, key)
		}
	}

	// If there's still not enough space, remove entries based on expiration time
	if c.currentSize+size > c.maxSize {
		// Sort entries by expiration time
		type keyExpiry struct {
			key       string
			expiresAt time.Time
		}
		entries := make([]keyExpiry, 0, len(c.items))
		for key, entry := range c.items {
			entries = append(entries, keyExpiry{key, entry.ExpiresAt})
		}

		// Sort entries by expiration time (oldest first)
		for i := range len(entries) - 1 {
			for j := i + 1; j < len(entries); j++ {
				if entries[i].expiresAt.After(entries[j].expiresAt) {
					entries[i], entries[j] = entries[j], entries[i]
				}
			}
		}

		// Remove entries until there's enough space
		for _, entry := range entries {
			if c.currentSize+size <= c.maxSize {
				break
			}
			if item, ok := c.items[entry.key]; ok {
				c.currentSize -= int64(len(item.Content))
				delete(c.items, entry.key)
			}
		}
	}
}

// CacheReader is a reader that reads from a cache entry.
type CacheReader struct {
	// entry is the cache entry being read.
	entry *CacheEntry

	// offset is the current read position.
	offset int
}

// NewCacheReader creates a new reader for a cache entry.
func NewCacheReader(entry *CacheEntry) *CacheReader {
	return &CacheReader{
		entry:  entry,
		offset: 0,
	}
}

// Read implements the io.Reader interface.
func (r *CacheReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.entry.Content) {
		return 0, io.EOF
	}
	n = copy(p, r.entry.Content[r.offset:])
	r.offset += n
	return n, nil
}

// Seek implements the io.Seeker interface.
func (r *CacheReader) Seek(offset int64, whence int) (int64, error) {
	var newOffset int64
	switch whence {
	case io.SeekStart:
		newOffset = offset
	case io.SeekCurrent:
		newOffset = int64(r.offset) + offset
	case io.SeekEnd:
		newOffset = int64(len(r.entry.Content)) + offset
	default:
		return 0, ErrInvalidWhence
	}

	if newOffset < 0 {
		return 0, ErrNegativeOffset
	}

	r.offset = int(newOffset)
	return newOffset, nil
}

// Close implements the io.Closer interface.
func (r *CacheReader) Close() error {
	return nil
}
