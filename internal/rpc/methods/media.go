// Package methods contains RPC method handlers for the application.
package methods

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/services/media"
	"norelock.dev/listenify/backend/internal/utils"
)

// MediaHandler handles media-related RPC methods.
type MediaHandler struct {
	mediaResolver *media.Resolver
	logger        *utils.Logger
}

// NewMediaHandler creates a new MediaHandler.
func NewMediaHandler(mediaResolver *media.Resolver, logger *utils.Logger) *MediaHandler {
	return &MediaHandler{
		mediaResolver: mediaResolver,
		logger:        logger,
	}
}

// RegisterMethods registers media-related RPC methods with the router.
func (h *MediaHandler) RegisterMethods(hr rpc.HandlerRegistry) {
	auth := hr.Wrap(rpc.AuthMiddleware)
	rpc.Register(hr, "media.search", h.SearchMedia)
	rpc.Register(auth, "media.getInfo", h.GetMediaInfo)
	rpc.Register(auth, "media.getStreamURL", h.GetStreamURL)
}

// SearchMediaParams represents the parameters for the searchMedia method.
type SearchMediaParams struct {
	Query  string `json:"query" validate:"required,min=2,max=100"`
	Source string `json:"source" validate:"required,oneof=youtube soundcloud all"`
	Limit  int    `json:"limit" validate:"min=1,max=50"`
}

// SearchMediaResult represents the result of the searchMedia method.
type SearchMediaResult struct {
	Results       []models.MediaSearchResult `json:"results"`
	NextPageToken string                     `json:"nextPageToken,omitempty"`
	TotalResults  int                        `json:"totalResults"`
	Source        string                     `json:"source"`
	Query         string                     `json:"query"`
}

// SearchMedia handles searching for media across different providers.
func (h *MediaHandler) SearchMedia(ctx context.Context, client *rpc.Client, p *SearchMediaParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Search for media
	response, err := h.mediaResolver.Search(ctx, p.Query, p.Source, p.Limit)
	if err != nil {
		h.logger.Error("Failed to search media", err, "query", p.Query, "source", p.Source)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to search media",
		}
	}

	// Return search results
	return SearchMediaResult{
		Results:       response.Results,
		NextPageToken: response.NextPageToken,
		TotalResults:  response.TotalResults,
		Source:        response.Source,
		Query:         response.Query,
	}, nil
}

// GetMediaInfoParams represents the parameters for the getMediaInfo method.
type GetMediaInfoParams struct {
	Source   string `json:"source" validate:"required,oneof=youtube soundcloud"`
	SourceID string `json:"sourceId" validate:"required"`
}

// GetMediaInfoResult represents the result of the getMediaInfo method.
type GetMediaInfoResult struct {
	Media *models.Media `json:"media"`
}

// GetMediaInfo handles retrieving information about a media item.
func (h *MediaHandler) GetMediaInfo(ctx context.Context, client *rpc.Client, p *GetMediaInfoParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert user ID to ObjectID
	userObjID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid user ID",
		}
	}

	// Resolve media
	mediaItem, err := h.mediaResolver.Resolve(ctx, p.Source, p.SourceID, userObjID)
	if err != nil {
		h.logger.Error("Failed to resolve media", err, "source", p.Source, "sourceId", p.SourceID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to resolve media",
		}
	}

	// Return media info
	return GetMediaInfoResult{
		Media: mediaItem,
	}, nil
}

// GetStreamURLParams represents the parameters for the getStreamURL method.
type GetStreamURLParams struct {
	Source   string `json:"source" validate:"required,oneof=youtube soundcloud"`
	SourceID string `json:"sourceId" validate:"required"`
}

// GetStreamURLResult represents the result of the getStreamURL method.
type GetStreamURLResult struct {
	URL string `json:"url"`
}

// GetStreamURL handles retrieving the streaming URL for a media item.
func (h *MediaHandler) GetStreamURL(ctx context.Context, client *rpc.Client, p *GetStreamURLParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Get stream URL
	url, err := h.mediaResolver.GetStreamURL(ctx, p.Source, p.SourceID)
	if err != nil {
		h.logger.Error("Failed to get stream URL", err, "source", p.Source, "sourceId", p.SourceID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get stream URL",
		}
	}

	// Return stream URL
	return GetStreamURLResult{
		URL: url,
	}, nil
}
