// Package playlist provides functionality for managing playlists.
package playlist

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/services/media"
	"norelock.dev/listenify/backend/internal/utils"
)

// ImportSource represents the source of an imported playlist.
type ImportSource string

// Import sources
const (
	ImportSourceYouTube    ImportSource = "youtube"
	ImportSourceSoundCloud ImportSource = "soundcloud"
)

// ImportStatus represents the status of an import request.
type ImportStatus string

// Import statuses
const (
	ImportStatusPending    ImportStatus = "pending"
	ImportStatusInProgress ImportStatus = "in_progress"
	ImportStatusCompleted  ImportStatus = "completed"
	ImportStatusFailed     ImportStatus = "failed"
)

// ImportRequest represents a request to import a playlist from an external source.
type ImportRequest struct {
	// ID is the unique identifier for the import request.
	ID string `json:"id"`

	// UserID is the ID of the user who initiated the import.
	UserID string `json:"userId"`

	// URL is the URL of the playlist to import.
	URL string `json:"url" validate:"required,url"`

	// Source is the source of the playlist.
	Source ImportSource `json:"source"`

	// Name is an optional name for the imported playlist.
	Name string `json:"name"`

	// Description is an optional description for the imported playlist.
	Description string `json:"description"`

	// Status is the current status of the import.
	Status ImportStatus `json:"status"`

	// Error is the error message if the import failed.
	Error string `json:"error,omitempty"`

	// PlaylistID is the ID of the created playlist.
	PlaylistID string `json:"playlistId,omitempty"`

	// ItemCount is the number of items in the playlist.
	ItemCount int `json:"itemCount"`

	// SuccessCount is the number of items successfully imported.
	SuccessCount int `json:"successCount"`

	// FailedCount is the number of items that failed to import.
	FailedCount int `json:"failedCount"`

	// CreatedAt is the time the import request was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the time the import request was last updated.
	UpdatedAt time.Time `json:"updatedAt"`

	// CompletedAt is the time the import was completed.
	CompletedAt time.Time `json:"completedAt,omitzero"`
}

// ImporterService provides functionality for importing playlists from external sources.
type ImporterService struct {
	playlistRepo repositories.PlaylistRepository
	mediaRepo    repositories.MediaRepository
	mediaService *media.MediaService
	httpClient   *http.Client
	logger       *utils.Logger
	apiKeys      map[ImportSource]string
}

// NewImporterService creates a new importer service.
func NewImporterService(
	playlistRepo repositories.PlaylistRepository,
	mediaRepo repositories.MediaRepository,
	mediaService *media.MediaService,
	logger *utils.Logger,
	apiKeys map[string]string,
) *ImporterService {
	// Convert API keys to ImportSource keys
	importAPIKeys := make(map[ImportSource]string)
	for k, v := range apiKeys {
		importAPIKeys[ImportSource(k)] = v
	}

	return &ImporterService{
		playlistRepo: playlistRepo,
		mediaRepo:    mediaRepo,
		mediaService: mediaService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:  logger.Named("importer_service"),
		apiKeys: importAPIKeys,
	}
}

// ImportPlaylist imports a playlist from an external source.
func (s *ImporterService) ImportPlaylist(ctx context.Context, req *ImportRequest) (*models.Playlist, error) {
	// Validate request
	if req.UserID == "" || req.URL == "" {
		return nil, fmt.Errorf("user ID and URL are required")
	}

	// Determine source if not provided
	if req.Source == "" {
		source, err := s.detectSourceFromURL(req.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to detect source: %w", err)
		}
		req.Source = source
	}

	// Set initial status
	req.Status = ImportStatusPending
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	// Create import request in database
	// This would typically be stored in a separate collection
	// For simplicity, we'll just log it
	s.logger.Info("Created import request", "id", req.ID, "user", req.UserID, "source", req.Source, "url", req.URL)

	// Update status to in progress
	req.Status = ImportStatusInProgress
	req.UpdatedAt = time.Now()

	// Import playlist based on source
	var playlist *models.Playlist
	var err error

	switch req.Source {
	case ImportSourceYouTube:
		playlist, err = s.importYouTubePlaylist(ctx, req)
	case ImportSourceSoundCloud:
		playlist, err = s.importSoundCloudPlaylist(ctx, req)
	default:
		err = fmt.Errorf("unsupported import source: %s", req.Source)
	}

	// Update status based on result
	if err != nil {
		req.Status = ImportStatusFailed
		req.Error = err.Error()
		req.UpdatedAt = time.Now()
		s.logger.Error("Failed to import playlist", err, "id", req.ID, "source", req.Source, "url", req.URL)
		return nil, fmt.Errorf("failed to import playlist: %w", err)
	}

	// Update request with success info
	req.Status = ImportStatusCompleted
	req.UpdatedAt = time.Now()
	req.CompletedAt = time.Now()
	req.PlaylistID = playlist.ID.Hex()
	req.ItemCount = len(playlist.Items)
	req.SuccessCount = req.ItemCount - req.FailedCount

	s.logger.Info("Imported playlist successfully",
		"id", req.ID,
		"playlist", playlist.ID,
		"source", req.Source,
		"items", req.ItemCount,
		"success", req.SuccessCount,
		"failed", req.FailedCount)

	return playlist, nil
}

// detectSourceFromURL attempts to determine the import source from the URL.
func (s *ImporterService) detectSourceFromURL(urlStr string) (ImportSource, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Host

	// Check for YouTube
	if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be") {
		return ImportSourceYouTube, nil
	}

	// Check for SoundCloud
	if strings.Contains(host, "soundcloud.com") {
		return ImportSourceSoundCloud, nil
	}

	return "", fmt.Errorf("unable to determine source from URL: %s", urlStr)
}

// importYouTubePlaylist imports a playlist from YouTube.
func (s *ImporterService) importYouTubePlaylist(ctx context.Context, req *ImportRequest) (*models.Playlist, error) {
	// Extract playlist ID from URL
	playlistID, err := s.extractYouTubePlaylistID(req.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract YouTube playlist ID: %w", err)
	}

	// Check if API key is available
	apiKey, ok := s.apiKeys[ImportSourceYouTube]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("YouTube API key not configured")
	}

	// Fetch playlist details
	playlistURL := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/playlists?part=snippet&id=%s&key=%s",
		playlistID, apiKey,
	)

	playlistResp, err := s.httpClient.Get(playlistURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch YouTube playlist: %w", err)
	}
	defer playlistResp.Body.Close()

	if playlistResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(playlistResp.Body)
		return nil, fmt.Errorf("YouTube API error: %s - %s", playlistResp.Status, string(body))
	}

	var playlistData struct {
		Items []struct {
			Snippet struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				Thumbnails  struct {
					Default struct {
						URL string `json:"url"`
					} `json:"default"`
					High struct {
						URL string `json:"url"`
					} `json:"high"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(playlistResp.Body).Decode(&playlistData); err != nil {
		return nil, fmt.Errorf("failed to decode YouTube playlist data: %w", err)
	}

	if len(playlistData.Items) == 0 {
		return nil, fmt.Errorf("YouTube playlist not found or empty")
	}

	// Convert user ID string to ObjectID
	ownerID, err := bson.ObjectIDFromHex(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Create playlist
	playlist := &models.Playlist{
		Owner:       ownerID,
		Name:        playlistData.Items[0].Snippet.Title,
		Description: playlistData.Items[0].Snippet.Description,
		ObjectTimes: models.NewObjectTimes(time.Now()),
		Items:       []models.PlaylistItem{},
		Tags:        []string{string(req.Source), "imported"},
		CoverImage:  playlistData.Items[0].Snippet.Thumbnails.High.URL,
		Stats: models.PlaylistStats{
			TotalItems:     0,
			TotalDuration:  0,
			LastCalculated: time.Now(),
		},
	}

	// If name is provided in request, use it instead
	if req.Name != "" {
		playlist.Name = req.Name
	}

	if req.Description != "" {
		playlist.Description = req.Description
	}

	// Fetch playlist items
	itemsURL := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/playlistItems?part=snippet&maxResults=50&playlistId=%s&key=%s",
		playlistID, apiKey,
	)

	var nextPageToken string
	for {
		pageURL := itemsURL
		if nextPageToken != "" {
			pageURL += "&pageToken=" + nextPageToken
		}

		itemsResp, err := s.httpClient.Get(pageURL)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch YouTube playlist items: %w", err)
		}

		var itemsData struct {
			NextPageToken string `json:"nextPageToken"`
			Items         []struct {
				Snippet struct {
					Title      string `json:"title"`
					ResourceID struct {
						VideoID string `json:"videoId"`
					} `json:"resourceId"`
				} `json:"snippet"`
			} `json:"items"`
		}

		if err := json.NewDecoder(itemsResp.Body).Decode(&itemsData); err != nil {
			itemsResp.Body.Close()
			return nil, fmt.Errorf("failed to decode YouTube playlist items: %w", err)
		}
		itemsResp.Body.Close()

		// Process items
		for _, item := range itemsData.Items {
			videoID := item.Snippet.ResourceID.VideoID
			if videoID == "" {
				continue
			}

			// Resolve media using media service
			mediaURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
			media, err := s.mediaService.ResolveMedia(ctx, mediaURL)
			if err != nil {
				s.logger.Warn("Failed to resolve YouTube video", "error", err, "videoId", videoID)
				req.FailedCount++
				continue
			}

			// Add to playlist
			playlistItem := models.PlaylistItem{
				ID:        bson.NewObjectID(),
				MediaID:   media.ID,
				Order:     len(playlist.Items),
				AddedAt:   time.Now(),
				PlayCount: 0,
			}

			playlist.Items = append(playlist.Items, playlistItem)
			playlist.Stats.TotalDuration += media.Duration
		}

		// Check if there are more pages
		nextPageToken = itemsData.NextPageToken
		if nextPageToken == "" {
			break
		}
	}

	// Update playlist stats
	playlist.Stats.TotalItems = len(playlist.Items)

	// Save playlist to database
	err = s.playlistRepo.Create(ctx, playlist)
	if err != nil {
		return nil, fmt.Errorf("failed to save playlist: %w", err)
	}

	return playlist, nil
}

// extractYouTubePlaylistID extracts the playlist ID from a YouTube URL.
func (s *ImporterService) extractYouTubePlaylistID(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Extract from query parameter
	query := parsedURL.Query()
	if listID := query.Get("list"); listID != "" {
		return listID, nil
	}

	// Extract from path
	if strings.Contains(parsedURL.Path, "playlist/") {
		parts := strings.Split(parsedURL.Path, "/")
		for i, part := range parts {
			if part == "playlist" && i+1 < len(parts) {
				return parts[i+1], nil
			}
		}
	}

	return "", fmt.Errorf("could not extract YouTube playlist ID from URL: %s", urlStr)
}

// importSoundCloudPlaylist imports a playlist from SoundCloud.
func (s *ImporterService) importSoundCloudPlaylist(ctx context.Context, req *ImportRequest) (*models.Playlist, error) {
	// Check if API key is available
	apiKey, ok := s.apiKeys[ImportSourceSoundCloud]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("SoundCloud API key not configured")
	}

	// Resolve playlist URL to get playlist data
	resolveURL := fmt.Sprintf(
		"https://api.soundcloud.com/resolve?url=%s&client_id=%s",
		url.QueryEscape(req.URL), apiKey,
	)

	resolveResp, err := s.httpClient.Get(resolveURL)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve SoundCloud playlist: %w", err)
	}
	defer resolveResp.Body.Close()

	if resolveResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resolveResp.Body)
		return nil, fmt.Errorf("SoundCloud API error: %s - %s", resolveResp.Status, string(body))
	}

	var playlistData struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		ArtworkURL  string `json:"artwork_url"`
		Tracks      []struct {
			ID           int    `json:"id"`
			Title        string `json:"title"`
			Duration     int    `json:"duration"`
			PermalinkURL string `json:"permalink_url"`
		} `json:"tracks"`
	}

	if err := json.NewDecoder(resolveResp.Body).Decode(&playlistData); err != nil {
		return nil, fmt.Errorf("failed to decode SoundCloud playlist data: %w", err)
	}

	// Convert user ID string to ObjectID
	ownerID, err := bson.ObjectIDFromHex(req.UserID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Create playlist
	playlist := &models.Playlist{
		Owner:       ownerID,
		Name:        playlistData.Title,
		Description: playlistData.Description,
		ObjectTimes: models.NewObjectTimes(time.Now()),
		Items:       []models.PlaylistItem{},
		Tags:        []string{string(req.Source), "imported"},
		CoverImage:  playlistData.ArtworkURL,
		Stats: models.PlaylistStats{
			TotalItems:     0,
			TotalDuration:  0,
			LastCalculated: time.Now(),
		},
	}

	// If name is provided in request, use it instead
	if req.Name != "" {
		playlist.Name = req.Name
	}

	if req.Description != "" {
		playlist.Description = req.Description
	}

	// Process tracks
	for _, track := range playlistData.Tracks {
		// Resolve media using media service
		media, err := s.mediaService.ResolveMedia(ctx, track.PermalinkURL)
		if err != nil {
			s.logger.Warn("Failed to resolve SoundCloud track", "error", err, "trackId", track.ID)
			req.FailedCount++
			continue
		}

		// Add to playlist
		playlistItem := models.PlaylistItem{
			ID:        bson.NewObjectID(),
			MediaID:   media.ID,
			Order:     len(playlist.Items),
			AddedAt:   time.Now(),
			PlayCount: 0,
		}

		playlist.Items = append(playlist.Items, playlistItem)
		playlist.Stats.TotalDuration += media.Duration
	}

	// Update playlist stats
	playlist.Stats.TotalItems = len(playlist.Items)

	// Save playlist to database
	err = s.playlistRepo.Create(ctx, playlist)
	if err != nil {
		return nil, fmt.Errorf("failed to save playlist: %w", err)
	}

	return playlist, nil
}
