// Package handlers contains HTTP handlers for the API.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/services/playlist"
	"norelock.dev/listenify/backend/internal/utils"
)

// PlaylistHandler handles HTTP requests related to playlist operations.
type PlaylistHandler struct {
	playlistManager *playlist.Manager
	logger          *utils.Logger
}

// NewPlaylistHandler creates a new playlist handler.
func NewPlaylistHandler(playlistManager *playlist.Manager, logger *utils.Logger) *PlaylistHandler {
	return &PlaylistHandler{
		playlistManager: playlistManager,
		logger:          logger.Named("playlist_handler"),
	}
}

// GetPlaylists handles requests to get a user's playlists.
func (h *PlaylistHandler) GetPlaylists(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get playlists
	playlists, err := h.playlistManager.GetUserPlaylists(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get playlists", err, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get playlists")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, playlists)
}

// CreatePlaylist handles requests to create a new playlist.
func (h *PlaylistHandler) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Parse request body
	var req models.PlaylistCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		utils.RespondWithValidationError(w, err)
		return
	}

	// Create playlist object
	now := time.Now()
	playlist := &models.Playlist{
		Owner:       userID,
		Name:        req.Name,
		Description: req.Description,
		IsPrivate:   req.IsPrivate,
		Tags:        req.Tags,
		CoverImage:  req.CoverImage,
		Items:       []models.PlaylistItem{},
		ObjectTimes: models.NewObjectTimes(now),
		Stats: models.PlaylistStats{
			TotalItems:     0,
			TotalDuration:  0,
			TotalPlays:     0,
			Followers:      0,
			LastCalculated: now,
		},
	}

	// Create playlist
	playlist, err = h.playlistManager.CreatePlaylist(r.Context(), playlist)
	if err != nil {
		h.logger.Error("Failed to create playlist", err, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to create playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusCreated, playlist)
}

// GetPlaylist handles requests to get a playlist by ID.
func (h *PlaylistHandler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	// Get playlist ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Playlist ID is required")
		return
	}

	playlistID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid playlist ID")
		return
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, playlist)
}

// UpdatePlaylist handles requests to update a playlist.
func (h *PlaylistHandler) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get playlist ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Playlist ID is required")
		return
	}

	playlistID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid playlist ID")
		return
	}

	// Get existing playlist
	existingPlaylist, err := h.playlistManager.GetPlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to get playlist for update", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get playlist")
		return
	}

	// Verify ownership
	if existingPlaylist.Owner != userID {
		utils.RespondWithError(w, http.StatusForbidden, "You don't have permission to update this playlist")
		return
	}

	// Parse request body
	var req models.PlaylistUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		utils.RespondWithValidationError(w, err)
		return
	}

	// Update playlist fields
	if req.Name != "" {
		existingPlaylist.Name = req.Name
	}
	existingPlaylist.Description = req.Description
	if req.IsPrivate != nil {
		existingPlaylist.IsPrivate = *req.IsPrivate
	}
	if req.Tags != nil {
		existingPlaylist.Tags = req.Tags
	}
	if req.CoverImage != "" {
		existingPlaylist.CoverImage = req.CoverImage
	}
	existingPlaylist.UpdateNow()

	// Update playlist
	updatedPlaylist, err := h.playlistManager.UpdatePlaylist(r.Context(), existingPlaylist)
	if err != nil {
		h.logger.Error("Failed to update playlist", err, "id", idStr, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, updatedPlaylist)
}

// DeletePlaylist handles requests to delete a playlist.
func (h *PlaylistHandler) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get playlist ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Playlist ID is required")
		return
	}

	playlistID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid playlist ID")
		return
	}

	// Get playlist to verify ownership
	playlist, err := h.playlistManager.GetPlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to get playlist for deletion", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get playlist")
		return
	}

	// Verify ownership
	if playlist.Owner != userID {
		utils.RespondWithError(w, http.StatusForbidden, "You don't have permission to delete this playlist")
		return
	}

	// Delete playlist
	err = h.playlistManager.DeletePlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to delete playlist", err, "id", idStr, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Playlist deleted successfully",
	})
}

// AddPlaylistItem handles requests to add an item to a playlist.
func (h *PlaylistHandler) AddPlaylistItem(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get playlist ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Playlist ID is required")
		return
	}

	playlistID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid playlist ID")
		return
	}

	// Get playlist to verify ownership
	playlist, err := h.playlistManager.GetPlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to get playlist for adding item", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get playlist")
		return
	}

	// Verify ownership
	if playlist.Owner != userID {
		utils.RespondWithError(w, http.StatusForbidden, "You don't have permission to modify this playlist")
		return
	}

	// Parse request body
	var req models.PlaylistAddItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		utils.RespondWithValidationError(w, err)
		return
	}

	// Get position
	position := -1 // Default to end of playlist
	if req.Position != nil {
		position = *req.Position
	}

	// Add item to playlist
	updatedPlaylist, err := h.playlistManager.AddPlaylistItem(r.Context(), playlistID, req.MediaID, position)
	if err != nil {
		h.logger.Error("Failed to add item to playlist", err, "playlistID", idStr, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to add item to playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, updatedPlaylist)
}

// RemovePlaylistItem handles requests to remove an item from a playlist.
func (h *PlaylistHandler) RemovePlaylistItem(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get playlist ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Playlist ID is required")
		return
	}

	playlistID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid playlist ID")
		return
	}

	// Get playlist to verify ownership
	playlist, err := h.playlistManager.GetPlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to get playlist for removing item", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get playlist")
		return
	}

	// Verify ownership
	if playlist.Owner != userID {
		utils.RespondWithError(w, http.StatusForbidden, "You don't have permission to modify this playlist")
		return
	}

	// Get item ID from URL parameter
	itemIDStr := chi.URLParam(r, "itemId")
	if itemIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Item ID is required")
		return
	}

	itemID, err := bson.ObjectIDFromHex(itemIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid item ID")
		return
	}

	// Remove item from playlist
	updatedPlaylist, err := h.playlistManager.RemovePlaylistItem(r.Context(), playlistID, itemID)
	if err != nil {
		h.logger.Error("Failed to remove item from playlist", err, "playlistID", idStr, "itemID", itemIDStr, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to remove item from playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, updatedPlaylist)
}

// ImportPlaylist handles requests to import a playlist from an external source.
func (h *PlaylistHandler) ImportPlaylist(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Parse request body
	var req models.PlaylistImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		utils.RespondWithValidationError(w, err)
		return
	}

	// Import playlist
	playlist, err := h.playlistManager.ImportPlaylist(r.Context(), userID, req.Source, req.SourceID)
	if err != nil {
		h.logger.Error("Failed to import playlist", err, "source", req.Source, "sourceId", req.SourceID, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to import playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, playlist)
}

// SetActivePlaylist handles requests to set a playlist as the user's active playlist.
func (h *PlaylistHandler) SetActivePlaylist(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Parse request body
	var req models.PlaylistSetActiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		utils.RespondWithValidationError(w, err)
		return
	}

	// Set active playlist
	err = h.playlistManager.SetActivePlaylist(r.Context(), userID, req.PlaylistID)
	if err != nil {
		h.logger.Error("Failed to set active playlist", err, "playlistID", req.PlaylistID.Hex(), "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to set active playlist")
		return
	}

	// Get updated playlist
	playlist, err := h.playlistManager.GetPlaylist(r.Context(), req.PlaylistID)
	if err != nil {
		h.logger.Error("Failed to get updated playlist", err, "id", req.PlaylistID.Hex())
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get updated playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, playlist)
}

// GetActivePlaylist handles requests to get the user's active playlist.
func (h *PlaylistHandler) GetActivePlaylist(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get active playlist
	playlist, err := h.playlistManager.GetActivePlaylist(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get active playlist", err, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get active playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, playlist)
}

// ShufflePlaylist handles requests to shuffle a playlist.
func (h *PlaylistHandler) ShufflePlaylist(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Get playlist ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Playlist ID is required")
		return
	}

	playlistID, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid playlist ID")
		return
	}

	// Get playlist to verify ownership
	playlist, err := h.playlistManager.GetPlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to get playlist for shuffling", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get playlist")
		return
	}

	// Verify ownership
	if playlist.Owner != userID {
		utils.RespondWithError(w, http.StatusForbidden, "You don't have permission to modify this playlist")
		return
	}

	// Shuffle playlist
	shuffledPlaylist, err := h.playlistManager.ShufflePlaylist(r.Context(), playlistID)
	if err != nil {
		h.logger.Error("Failed to shuffle playlist", err, "id", idStr, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to shuffle playlist")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, shuffledPlaylist)
}

// SearchPlaylists handles requests to search for playlists.
func (h *PlaylistHandler) SearchPlaylists(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query().Get("q")
	tags := r.URL.Query()["tag"]
	includePrivate := r.URL.Query().Get("includePrivate") == "true"
	sortBy := r.URL.Query().Get("sortBy")
	sortDirection := r.URL.Query().Get("sortDirection")

	// Create search criteria
	criteria := models.PlaylistSearchCriteria{
		Query:          query,
		Tags:           tags,
		IncludePrivate: includePrivate,
		SortBy:         sortBy,
		SortDirection:  sortDirection,
		Page:           0,
		Limit:          50,
	}

	// Search playlists
	playlists, total, err := h.playlistManager.SearchPlaylists(r.Context(), criteria)
	if err != nil {
		h.logger.Error("Failed to search playlists", err, "query", query)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to search playlists")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"playlists": playlists,
		"total":     total,
	})
}
