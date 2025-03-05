// Package media provides media resolution and search functionality.
package media

import (
	"context"
	"fmt"
	"sync"

	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// SearchService handles searching for media across different providers.
type SearchService struct {
	providers map[string]Provider
	logger    *utils.Logger
}

// NewSearchService creates a new search service.
func NewSearchService(providers map[string]Provider, logger *utils.Logger) *SearchService {
	return &SearchService{
		providers: providers,
		logger:    logger.Named("search_service"),
	}
}

// Search searches for media across all providers or a specific provider.
func (s *SearchService) Search(ctx context.Context, req models.MediaSearchRequest) (*models.MediaSearchResponse, error) {
	s.logger.Debug("Searching for media", "query", req.Query, "source", req.Source, "limit", req.Limit)

	// If no limit is specified, use a default limit
	if req.Limit <= 0 {
		req.Limit = 20
	}

	// If source is "all", search all providers
	if req.Source == "all" {
		return s.searchAllProviders(ctx, req.Query, req.Limit)
	}

	// Otherwise, search the specified provider
	provider, ok := s.providers[req.Source]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", req.Source)
	}

	results, nextPageToken, err := provider.Search(ctx, req.Query, req.Limit)
	if err != nil {
		s.logger.Error("Failed to search provider", err, "provider", req.Source, "query", req.Query)
		return nil, fmt.Errorf("failed to search provider %s: %w", req.Source, err)
	}

	response := &models.MediaSearchResponse{
		Results:       results,
		NextPageToken: nextPageToken,
		TotalResults:  len(results),
		Source:        req.Source,
		Query:         req.Query,
	}

	return response, nil
}

// searchAllProviders searches for media across all providers.
func (s *SearchService) searchAllProviders(ctx context.Context, query string, limit int) (*models.MediaSearchResponse, error) {
	s.logger.Debug("Searching all providers", "query", query, "limit", limit)

	// Calculate limit per provider
	providersCount := len(s.providers)
	if providersCount == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	limitPerProvider := max(limit/providersCount, 1)

	// Search all providers concurrently
	var wg sync.WaitGroup
	resultsChan := make(chan []models.MediaSearchResult, providersCount)
	errorsChan := make(chan error, providersCount)

	for name, provider := range s.providers {
		wg.Add(1)
		go func(name string, provider Provider) {
			defer wg.Done()

			results, _, err := provider.Search(ctx, query, limitPerProvider)
			if err != nil {
				s.logger.Error("Failed to search provider", err, "provider", name, "query", query)
				errorsChan <- fmt.Errorf("failed to search provider %s: %w", name, err)
				return
			}

			resultsChan <- results
		}(name, provider)
	}

	// Wait for all searches to complete
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Check for errors
	if len(errorsChan) > 0 {
		// Log errors but continue with results from successful providers
		for err := range errorsChan {
			s.logger.Error("Provider search error", err)
		}
	}

	// Combine results
	var allResults []models.MediaSearchResult
	for results := range resultsChan {
		allResults = append(allResults, results...)
	}

	// Limit the total number of results
	if len(allResults) > limit {
		allResults = allResults[:limit]
	}

	response := &models.MediaSearchResponse{
		Results:      allResults,
		TotalResults: len(allResults),
		Source:       "all",
		Query:        query,
	}

	return response, nil
}
