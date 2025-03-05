// Package media provides media resolution and search functionality.
package media

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// YouTubeProvider implements the Provider interface for YouTube.
type YouTubeProvider struct {
	apiKey string
	logger *utils.Logger
}

// NewYouTubeProvider creates a new YouTube provider.
func NewYouTubeProvider(apiKey string, logger *utils.Logger) *YouTubeProvider {
	return &YouTubeProvider{
		apiKey: apiKey,
		logger: logger.Named("youtube_provider"),
	}
}

// Search searches for media on YouTube.
func (p *YouTubeProvider) Search(ctx context.Context, query string, limit int) ([]models.MediaSearchResult, string, error) {
	p.logger.Debug("Searching YouTube", "query", query, "limit", limit)

	// Create YouTube service
	service, err := youtube.NewService(ctx, option.WithAPIKey(p.apiKey))
	if err != nil {
		p.logger.Error("Failed to create YouTube service", err)
		return nil, "", fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Prepare search call
	call := service.Search.List([]string{"id", "snippet"}).
		Q(query).
		Type("video").
		MaxResults(int64(limit)).
		VideoCategoryId("10") // Music category

	// Execute search
	response, err := call.Do()
	if err != nil {
		p.logger.Error("Failed to search YouTube", err, "query", query)
		return nil, "", fmt.Errorf("failed to search YouTube: %w", err)
	}

	// Process results
	results := make([]models.MediaSearchResult, 0, len(response.Items))
	for _, item := range response.Items {
		if item.Id.Kind != "youtube#video" {
			continue
		}

		// Get video details to get duration
		videoResponse, err := service.Videos.List([]string{"contentDetails", "statistics"}).
			Id(item.Id.VideoId).
			Do()
		if err != nil {
			p.logger.Error("Failed to get video details", err, "videoId", item.Id.VideoId)
			continue
		}

		if len(videoResponse.Items) == 0 {
			continue
		}

		videoDetails := videoResponse.Items[0]

		// Parse duration
		duration, err := parseDuration(videoDetails.ContentDetails.Duration)
		if err != nil {
			p.logger.Error("Failed to parse duration", err, "duration", videoDetails.ContentDetails.Duration)
			duration = 0
		}

		// Convert view count from uint64 to int64
		viewCount := int64(videoDetails.Statistics.ViewCount)

		// Parse published time
		publishedAt, err := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		if err != nil {
			p.logger.Error("Failed to parse published time", err, "publishedAt", item.Snippet.PublishedAt)
			publishedAt = time.Now()
		}

		// Create search result
		result := models.MediaSearchResult{
			SourceID:     item.Id.VideoId,
			Type:         "youtube",
			Title:        item.Snippet.Title,
			Artist:       item.Snippet.ChannelTitle,
			Thumbnail:    getBestThumbnail(item.Snippet.Thumbnails),
			Duration:     duration,
			Views:        viewCount,
			PublishedAt:  publishedAt,
			Description:  item.Snippet.Description,
			ChannelTitle: item.Snippet.ChannelTitle,
		}

		results = append(results, result)
	}

	return results, response.NextPageToken, nil
}

// GetMediaInfo retrieves information about a YouTube video.
func (p *YouTubeProvider) GetMediaInfo(ctx context.Context, sourceID string) (*models.Media, error) {
	p.logger.Debug("Getting YouTube video info", "sourceID", sourceID)

	// Create YouTube service
	service, err := youtube.NewService(ctx, option.WithAPIKey(p.apiKey))
	if err != nil {
		p.logger.Error("Failed to create YouTube service", err)
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	// Get video details
	response, err := service.Videos.List([]string{"snippet", "contentDetails", "statistics"}).
		Id(sourceID).
		Do()
	if err != nil {
		p.logger.Error("Failed to get video details", err, "sourceID", sourceID)
		return nil, fmt.Errorf("failed to get video details: %w", err)
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("video not found: %s", sourceID)
	}

	video := response.Items[0]

	// Parse duration
	duration, err := parseDuration(video.ContentDetails.Duration)
	if err != nil {
		p.logger.Error("Failed to parse duration", err, "duration", video.ContentDetails.Duration)
		duration = 0
	}

	// Convert view count from uint64 to int64
	viewCount := int64(video.Statistics.ViewCount)

	// Convert like count from uint64 to int64
	likeCount := int64(video.Statistics.LikeCount)

	// Create media
	media := &models.Media{
		Type:      "youtube",
		SourceID:  sourceID,
		Title:     video.Snippet.Title,
		Artist:    video.Snippet.ChannelTitle,
		Thumbnail: getBestThumbnail(video.Snippet.Thumbnails),
		Duration:  duration,
		Metadata: models.MediaMetadata{
			Views:       viewCount,
			Likes:       likeCount,
			PublishedAt: parseTime(video.Snippet.PublishedAt),
			ChannelID:   video.Snippet.ChannelId,
			Categories:  []string{video.Snippet.CategoryId},
		},
		Stats: models.MediaStats{
			PlayCount: 0,
			WootCount: 0,
			MehCount:  0,
			GrabCount: 0,
		},
	}

	return media, nil
}

// parseTime parses a time string into a time.Time object.
func parseTime(timeStr string) time.Time {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		// Return current time if parsing fails
		return time.Now()
	}
	return t
}

// GetStreamURL retrieves the streaming URL for a YouTube video.
func (p *YouTubeProvider) GetStreamURL(ctx context.Context, sourceID string) (string, error) {
	p.logger.Debug("Getting YouTube stream URL", "sourceID", sourceID)

	// In a real implementation, this would use youtube-dl or a similar library
	// to extract the actual streaming URL. For now, we'll just return a proxy URL.
	return fmt.Sprintf("/api/media/proxy/youtube/%s", sourceID), nil
}

// GetType returns the provider type.
func (p *YouTubeProvider) GetType() string {
	return "youtube"
}

// Helper functions

// parseDuration parses an ISO 8601 duration string into seconds.
func parseDuration(isoDuration string) (int, error) {
	// Remove PT prefix
	duration := strings.TrimPrefix(isoDuration, "PT")

	// Parse hours, minutes, seconds
	var hours, minutes, seconds int

	// Find hours
	if idx := strings.Index(duration, "H"); idx != -1 {
		h, err := strconv.Atoi(duration[:idx])
		if err != nil {
			return 0, err
		}
		hours = h
		duration = duration[idx+1:]
	}

	// Find minutes
	if idx := strings.Index(duration, "M"); idx != -1 {
		m, err := strconv.Atoi(duration[:idx])
		if err != nil {
			return 0, err
		}
		minutes = m
		duration = duration[idx+1:]
	}

	// Find seconds
	if idx := strings.Index(duration, "S"); idx != -1 {
		s, err := strconv.Atoi(duration[:idx])
		if err != nil {
			return 0, err
		}
		seconds = s
	}

	// Calculate total seconds
	totalSeconds := hours*3600 + minutes*60 + seconds
	return totalSeconds, nil
}

// getBestThumbnail returns the best quality thumbnail URL.
func getBestThumbnail(thumbnails *youtube.ThumbnailDetails) string {
	if thumbnails == nil {
		return ""
	}

	// Try to get the highest quality thumbnail
	if thumbnails.Maxres != nil && thumbnails.Maxres.Url != "" {
		return thumbnails.Maxres.Url
	}
	if thumbnails.High != nil && thumbnails.High.Url != "" {
		return thumbnails.High.Url
	}
	if thumbnails.Medium != nil && thumbnails.Medium.Url != "" {
		return thumbnails.Medium.Url
	}
	if thumbnails.Standard != nil && thumbnails.Standard.Url != "" {
		return thumbnails.Standard.Url
	}
	if thumbnails.Default != nil && thumbnails.Default.Url != "" {
		return thumbnails.Default.Url
	}

	return ""
}
