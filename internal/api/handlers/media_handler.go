// Package handlers contains HTTP handlers for the API.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/services/media"
	"norelock.dev/listenify/backend/internal/utils"
)

// MediaHandler handles HTTP requests related to media operations.
type MediaHandler struct {
	mediaResolver *media.Resolver
	logger        *utils.Logger
}

// NewMediaHandler creates a new media handler.
func NewMediaHandler(mediaResolver *media.Resolver, logger *utils.Logger) *MediaHandler {
	return &MediaHandler{
		mediaResolver: mediaResolver,
		logger:        logger.Named("media_handler"),
	}
}

// Search handles requests to search for media.
func (h *MediaHandler) Search(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query().Get("q")
	if query == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		source = "all"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 50 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}

	// Search for media
	response, err := h.mediaResolver.Search(r.Context(), query, source, limit)
	if err != nil {
		h.logger.Error("Failed to search for media", err, "query", query, "source", source)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to search for media")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, response)
}

// Resolve handles requests to resolve a media item.
func (h *MediaHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	source := r.URL.Query().Get("source")
	if source == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Query parameter 'source' is required")
		return
	}

	sourceID := r.URL.Query().Get("id")
	if sourceID == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Query parameter 'id' is required")
		return
	}

	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		h.logger.Error("Invalid user ID in context", err, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Invalid user ID")
		return
	}

	// Resolve media
	media, err := h.mediaResolver.Resolve(r.Context(), source, sourceID, userID)
	if err != nil {
		h.logger.Error("Failed to resolve media", err, "source", source, "sourceID", sourceID)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to resolve media")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, media)
}

// Proxy handles requests to proxy media content.
func (h *MediaHandler) Proxy(w http.ResponseWriter, r *http.Request) {
	// Get provider and ID from URL parameters
	provider := chi.URLParam(r, "provider")
	id := chi.URLParam(r, "id")

	if provider == "" || id == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Provider and ID are required")
		return
	}

	// Get stream URL
	streamURL, err := h.mediaResolver.GetStreamURL(r.Context(), provider, id)
	if err != nil {
		h.logger.Error("Failed to get stream URL", err, "provider", provider, "id", id)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get stream URL")
		return
	}

	// Redirect to the stream URL
	http.Redirect(w, r, streamURL, http.StatusFound)
}

// GetMedia handles requests to get a media item by ID.
func (h *MediaHandler) GetMedia(w http.ResponseWriter, r *http.Request) {
	// Get media ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Media ID is required")
		return
	}

	// Convert ID string to ObjectID
	id, err := bson.ObjectIDFromHex(idStr)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid media ID")
		return
	}

	// Get media
	media, err := h.mediaResolver.GetMediaByID(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get media", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get media")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, media)
}

// AddMedia handles requests to add a new media item.
func (h *MediaHandler) AddMedia(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req models.MediaSearchResult
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)
	userID, err := bson.ObjectIDFromHex(userIDStr)
	if err != nil {
		h.logger.Error("Invalid user ID in context", err, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Invalid user ID")
		return
	}

	// Resolve media
	media, err := h.mediaResolver.Resolve(r.Context(), req.Type, req.SourceID, userID)
	if err != nil {
		h.logger.Error("Failed to add media", err, "source", req.Type, "sourceID", req.SourceID)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to add media")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, media)
}
