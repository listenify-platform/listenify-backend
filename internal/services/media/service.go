// Package media provides media resolution and search functionality.
package media

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// MediaService provides a unified interface for media operations.
type MediaService struct {
	resolver      *Resolver
	searchService *SearchService
	mediaRepo     repositories.MediaRepository
	logger        *utils.Logger
}

// NewMediaService creates a new media service.
func NewMediaService(
	resolver *Resolver,
	searchService *SearchService,
	mediaRepo repositories.MediaRepository,
	logger *utils.Logger,
) *MediaService {
	return &MediaService{
		resolver:      resolver,
		searchService: searchService,
		mediaRepo:     mediaRepo,
		logger:        logger.Named("media_service"),
	}
}

// ResolveMedia resolves a media item by its URL.
func (s *MediaService) ResolveMedia(ctx context.Context, url string) (*models.Media, error) {
	s.logger.Debug("Resolving media", "url", url)

	// Extract source and sourceID from URL
	source, sourceID, err := ExtractSourceInfo(url)
	if err != nil {
		return nil, fmt.Errorf("failed to extract source info: %w", err)
	}

	// Use a placeholder user ID for now
	// In a real implementation, this would be the actual user ID
	userID := bson.NewObjectID()

	// Resolve the media
	return s.resolver.Resolve(ctx, source, sourceID, userID)
}

// SearchMedia searches for media across providers.
func (s *MediaService) SearchMedia(ctx context.Context, query string, limit int) ([]models.Media, error) {
	s.logger.Debug("Searching media", "query", query, "limit", limit)

	// Create search request
	req := models.MediaSearchRequest{
		Query:  query,
		Source: "all", // Search all providers
		Limit:  limit,
	}

	// Perform search
	resp, err := s.searchService.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search media: %w", err)
	}

	// Convert search results to Media objects
	var mediaItems []models.Media
	for _, result := range resp.Results {
		// Create a media object from the search result
		media := models.Media{
			Type:      result.Type,
			SourceID:  result.SourceID,
			Title:     result.Title,
			Artist:    result.Artist,
			Thumbnail: result.Thumbnail,
			Duration:  result.Duration,
			Metadata: models.MediaMetadata{
				Views:        result.Views,
				PublishedAt:  result.PublishedAt,
				Description:  result.Description,
				ChannelTitle: result.ChannelTitle,
				Restricted:   result.Restricted,
			},
			// Set a placeholder ID
			ID: bson.NewObjectID(),
		}

		mediaItems = append(mediaItems, media)
	}

	return mediaItems, nil
}

// GetMediaByID retrieves a media item by its ID.
func (s *MediaService) GetMediaByID(ctx context.Context, id bson.ObjectID) (*models.Media, error) {
	s.logger.Debug("Getting media by ID", "id", id.Hex())
	return s.resolver.GetMediaByID(ctx, id)
}

// GetStreamURL retrieves the streaming URL for a media item.
func (s *MediaService) GetStreamURL(ctx context.Context, source string, sourceID string) (string, error) {
	s.logger.Debug("Getting stream URL", "source", source, "sourceID", sourceID)
	return s.resolver.GetStreamURL(ctx, source, sourceID)
}

// ExtractSourceInfo extracts the source and sourceID from a URL.
func ExtractSourceInfo(url string) (string, string, error) {
	// This is a simplified implementation
	// In a real implementation, this would use regex or URL parsing to extract the source and sourceID

	// Check for YouTube
	if ytID, err := extractYouTubeID(url); err == nil {
		return "youtube", ytID, nil
	}

	// Check for SoundCloud
	if scID, err := extractSoundCloudID(url); err == nil {
		return "soundcloud", scID, nil
	}

	return "", "", fmt.Errorf("unsupported URL format: %s", url)
}

// extractYouTubeID extracts the video ID from a YouTube URL.
func extractYouTubeID(url string) (string, error) {
	// This is a simplified implementation
	// In a real implementation, this would use regex or URL parsing to extract the video ID
	return "placeholder_youtube_id", nil
}

// extractSoundCloudID extracts the track ID from a SoundCloud URL.
func extractSoundCloudID(url string) (string, error) {
	// This is a simplified implementation
	// In a real implementation, this would use regex or URL parsing to extract the track ID
	return "placeholder_soundcloud_id", nil
}
