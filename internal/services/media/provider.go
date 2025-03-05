// Package media provides media resolution and search functionality.
package media

import (
	"context"

	"norelock.dev/listenify/backend/internal/models"
)

// Provider defines the interface for media providers.
type Provider interface {
	// Search searches for media using the given query.
	Search(ctx context.Context, query string, limit int) ([]models.MediaSearchResult, string, error)

	// GetMediaInfo retrieves information about a media item.
	GetMediaInfo(ctx context.Context, sourceID string) (*models.Media, error)

	// GetStreamURL retrieves the streaming URL for a media item.
	GetStreamURL(ctx context.Context, sourceID string) (string, error)

	// GetType returns the provider type (e.g., "youtube", "soundcloud").
	GetType() string
}