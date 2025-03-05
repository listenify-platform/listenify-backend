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

// Resolver handles media resolution and search across different providers.
type Resolver struct {
	providers    map[string]Provider
	mediaRepo    repositories.MediaRepository
	logger       *utils.Logger
	defaultLimit int
}

// NewResolver creates a new media resolver.
func NewResolver(mediaRepo repositories.MediaRepository, logger *utils.Logger) *Resolver {
	r := &Resolver{
		providers:    make(map[string]Provider),
		mediaRepo:    mediaRepo,
		logger:       logger.Named("media_resolver"),
		defaultLimit: 20,
	}

	// Register providers
	// These would be initialized with API keys and other configuration
	// In a real implementation, these would be created with proper API keys
	r.RegisterProvider(NewYouTubeProvider("", logger))
	r.RegisterProvider(NewSoundCloudProvider("", logger))

	return r
}

// RegisterProvider registers a media provider.
func (r *Resolver) RegisterProvider(provider Provider) {
	r.providers[provider.GetType()] = provider
	r.logger.Info("Registered media provider", "type", provider.GetType())
}

// Search searches for media across all providers or a specific provider.
func (r *Resolver) Search(ctx context.Context, query string, source string, limit int) (*models.MediaSearchResponse, error) {
	r.logger.Debug("Searching for media", "query", query, "source", source)

	if limit <= 0 {
		limit = r.defaultLimit
	}

	response := &models.MediaSearchResponse{
		Query:  query,
		Source: source,
	}

	// If source is "all", search across all providers
	if source == "all" {
		var allResults []models.MediaSearchResult
		totalResults := 0

		// Distribute the limit across providers
		providerLimit := max(limit/len(r.providers), 1)

		for providerType, provider := range r.providers {
			results, nextPageToken, err := provider.Search(ctx, query, providerLimit)
			if err != nil {
				r.logger.Error("Error searching provider", err, "provider", providerType)
				continue
			}

			allResults = append(allResults, results...)
			totalResults += len(results)

			// Store the next page token from the first provider that returns one
			if response.NextPageToken == "" && nextPageToken != "" {
				response.NextPageToken = fmt.Sprintf("%s:%s", providerType, nextPageToken)
			}
		}

		response.Results = allResults
		response.TotalResults = totalResults
	} else {
		// Search a specific provider
		provider, ok := r.providers[source]
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s", source)
		}

		results, nextPageToken, err := provider.Search(ctx, query, limit)
		if err != nil {
			r.logger.Error("Error searching provider", err, "provider", source)
			return nil, err
		}

		response.Results = results
		response.TotalResults = len(results)
		response.NextPageToken = nextPageToken
	}

	return response, nil
}

// Resolve resolves a media item by its source and ID.
func (r *Resolver) Resolve(ctx context.Context, source string, sourceID string, userID bson.ObjectID) (*models.Media, error) {
	r.logger.Debug("Resolving media", "source", source, "sourceID", sourceID)

	// Check if the media is already in the database
	media, err := r.mediaRepo.FindBySourceID(ctx, source, sourceID)
	if err == nil {
		// Media found in database
		return media, nil
	}

	// If not found, resolve it from the provider
	provider, ok := r.providers[source]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", source)
	}

	media, err = provider.GetMediaInfo(ctx, sourceID)
	if err != nil {
		r.logger.Error("Error resolving media", err, "source", source, "sourceID", sourceID)
		return nil, err
	}

	// Set the user who added the media
	media.AddedBy = userID
	media.CreateNow()

	// Save the media to the database
	err = r.mediaRepo.Create(ctx, media)
	if err != nil {
		r.logger.Error("Error saving media", err, "source", source, "sourceID", sourceID)
		return nil, err
	}

	return media, nil
}

// GetStreamURL retrieves the streaming URL for a media item.
func (r *Resolver) GetStreamURL(ctx context.Context, source string, sourceID string) (string, error) {
	r.logger.Debug("Getting stream URL", "source", source, "sourceID", sourceID)

	provider, ok := r.providers[source]
	if !ok {
		return "", fmt.Errorf("unknown provider: %s", source)
	}

	return provider.GetStreamURL(ctx, sourceID)
}

// GetMediaByID retrieves a media item by its ID.
func (r *Resolver) GetMediaByID(ctx context.Context, id bson.ObjectID) (*models.Media, error) {
	r.logger.Debug("Getting media by ID", "id", id.Hex())
	return r.mediaRepo.FindByID(ctx, id)
}
