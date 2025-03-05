// Package media provides media resolution and search functionality.
package media

import (
	"context"
	"errors"
	"fmt"

	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// SoundCloudProvider implements the Provider interface for SoundCloud.
type SoundCloudProvider struct {
	apiKey string
	logger *utils.Logger
}

// NewSoundCloudProvider creates a new SoundCloud provider.
func NewSoundCloudProvider(apiKey string, logger *utils.Logger) *SoundCloudProvider {
	return &SoundCloudProvider{
		apiKey: apiKey,
		logger: logger.Named("soundcloud_provider"),
	}
}

// Search searches for media on SoundCloud.
func (p *SoundCloudProvider) Search(ctx context.Context, query string, limit int) ([]models.MediaSearchResult, string, error) {
	p.logger.Debug("Searching SoundCloud", "query", query, "limit", limit)

	// This would be implemented with the SoundCloud API
	// For now, we'll just return a placeholder implementation
	return []models.MediaSearchResult{}, "", errors.New("not implemented")
}

// GetMediaInfo retrieves information about a SoundCloud track.
func (p *SoundCloudProvider) GetMediaInfo(ctx context.Context, sourceID string) (*models.Media, error) {
	p.logger.Debug("Getting SoundCloud track info", "sourceID", sourceID)

	// This would be implemented with the SoundCloud API
	// For now, we'll just return a placeholder implementation
	return nil, errors.New("not implemented")
}

// GetStreamURL retrieves the streaming URL for a SoundCloud track.
func (p *SoundCloudProvider) GetStreamURL(ctx context.Context, sourceID string) (string, error) {
	p.logger.Debug("Getting SoundCloud stream URL", "sourceID", sourceID)

	// This would be implemented with the SoundCloud API
	// For now, we'll just return a proxy URL
	return fmt.Sprintf("/api/media/proxy/soundcloud/%s", sourceID), nil
}

// GetType returns the provider type.
func (p *SoundCloudProvider) GetType() string {
	return "soundcloud"
}