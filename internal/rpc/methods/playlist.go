// Package methods contains RPC method handlers for the application.
package methods

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/services/playlist"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// PlaylistHandler handles playlist-related RPC methods.
type PlaylistHandler struct {
	playlistManager *playlist.Manager
	userManager     *user.Manager
	logger          *utils.Logger
}

// NewPlaylistHandler creates a new PlaylistHandler.
func NewPlaylistHandler(playlistManager *playlist.Manager, userManager *user.Manager, logger *utils.Logger) *PlaylistHandler {
	return &PlaylistHandler{
		playlistManager: playlistManager,
		userManager:     userManager,
		logger:          logger,
	}
}

// RegisterMethods registers playlist-related RPC methods with the router.
func (h *PlaylistHandler) RegisterMethods(hr rpc.HandlerRegistry) {
	auth := hr.Wrap(rpc.AuthMiddleware)
	rpc.Register(auth, "playlist.create", h.CreatePlaylist)
	rpc.Register(hr, "playlist.get", h.GetPlaylist)
	rpc.Register(hr, "playlist.getUserPlaylists", h.GetUserPlaylists)
	rpc.Register(auth, "playlist.update", h.UpdatePlaylist)
	rpc.Register(auth, "playlist.delete", h.DeletePlaylist)
	rpc.Register(auth, "playlist.addItem", h.AddPlaylistItem)
	rpc.Register(auth, "playlist.removeItem", h.RemovePlaylistItem)
	rpc.Register(auth, "playlist.import", h.ImportPlaylist)
	rpc.Register(auth, "playlist.setActive", h.SetActivePlaylist)
	rpc.RegisterNoParams(auth, "playlist.getActive", h.GetActivePlaylist)
	rpc.Register(auth, "playlist.shuffle", h.ShufflePlaylist)
	rpc.Register(hr, "playlist.search", h.SearchPlaylists)
}

// CreatePlaylistParams represents the parameters for the createPlaylist method.
type CreatePlaylistParams struct {
	Name        string   `json:"name" validate:"required,min=1,max=50"`
	Description string   `json:"description" validate:"max=1000"`
	IsPrivate   bool     `json:"isPrivate"`
	Tags        []string `json:"tags" validate:"dive,max=20"`
	CoverImage  string   `json:"coverImage,omitempty" validate:"omitempty,url"`
}

// CreatePlaylistResult represents the result of the createPlaylist method.
type CreatePlaylistResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
}

// CreatePlaylist handles creating a new playlist.
func (h *PlaylistHandler) CreatePlaylist(ctx context.Context, client *rpc.Client, p *CreatePlaylistParams) (any, error) {
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

	// Create playlist
	playlist := &models.Playlist{
		Name:        p.Name,
		Description: p.Description,
		Owner:       userObjID,
		IsPrivate:   p.IsPrivate,
		Tags:        p.Tags,
		CoverImage:  p.CoverImage,
		Items:       []models.PlaylistItem{},
	}

	createdPlaylist, err := h.playlistManager.CreatePlaylist(ctx, playlist)
	if err != nil {
		h.logger.Error("Failed to create playlist", err, "userId", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to create playlist",
		}
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user for playlist info", err, "userId", client.UserID)
		// Continue anyway, we'll just return the playlist without owner info
	}

	// Return playlist info
	playlistInfo := createdPlaylist.ToPlaylistInfo(user)
	return CreatePlaylistResult{
		Playlist: playlistInfo,
	}, nil
}

// GetPlaylistParams represents the parameters for the getPlaylist method.
type GetPlaylistParams struct {
	PlaylistID string `json:"playlistId" validate:"required"`
}

// GetPlaylistResult represents the result of the getPlaylist method.
type GetPlaylistResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
	Items    []models.MediaInfo  `json:"items"`
}

// GetPlaylist handles retrieving a playlist.
func (h *PlaylistHandler) GetPlaylist(ctx context.Context, client *rpc.Client, p *GetPlaylistParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert playlist ID to ObjectID
	playlistObjID, err := bson.ObjectIDFromHex(p.PlaylistID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid playlist ID",
		}
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Playlist not found",
		}
	}

	// Check if playlist is private and user is not the owner
	if playlist.IsPrivate && (client.UserID == "" || client.UserID != playlist.Owner.Hex()) {
		return nil, &rpc.Error{
			Code:    rpc.ErrNotAuthorized,
			Message: "You do not have permission to view this playlist",
		}
	}

	// Get owner for playlist info
	var owner *models.User
	if client.UserID != "" {
		owner, err = h.userManager.GetUserByID(ctx, playlist.Owner.Hex())
		if err != nil {
			h.logger.Error("Failed to get owner for playlist info", err, "ownerId", playlist.Owner.Hex())
			// Continue anyway, we'll just return the playlist without owner info
		}
	}

	// Extract media info from playlist items
	items := make([]models.MediaInfo, 0, len(playlist.Items))
	for _, item := range playlist.Items {
		if item.Media != nil {
			items = append(items, *item.Media)
		}
	}

	// Return playlist info and items
	playlistInfo := playlist.ToPlaylistInfo(owner)
	return GetPlaylistResult{
		Playlist: playlistInfo,
		Items:    items,
	}, nil
}

// GetUserPlaylistsResult represents the result of the getUserPlaylists method.
type GetUserPlaylistsResult struct {
	Playlists []models.PlaylistInfo `json:"playlists"`
}

// GetUserPlaylists handles retrieving a user's playlists.
func (h *PlaylistHandler) GetUserPlaylists(ctx context.Context, client *rpc.Client, p *UserIDParam) (any, error) {
	// If no user ID is provided, use the authenticated user's ID
	userID := p.UserID
	if userID == "" {
		if client.UserID == "" {
			return nil, &rpc.Error{
				Code:    rpc.ErrAuthenticationRequired,
				Message: "Authentication required",
			}
		}
		userID = client.UserID
	}

	// Convert user ID to ObjectID
	userObjID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid user ID",
		}
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user", err, "userId", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "User not found",
		}
	}

	// Get user playlists
	playlists, err := h.playlistManager.GetUserPlaylists(ctx, userObjID)
	if err != nil {
		h.logger.Error("Failed to get user playlists", err, "userId", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get user playlists",
		}
	}

	// Filter out private playlists if the user is not the authenticated user
	var filteredPlaylists []*models.Playlist
	if client.UserID != userID {
		for _, playlist := range playlists {
			if !playlist.IsPrivate {
				filteredPlaylists = append(filteredPlaylists, playlist)
			}
		}
	} else {
		filteredPlaylists = playlists
	}

	// Convert playlists to playlist info
	playlistInfos := make([]models.PlaylistInfo, 0, len(filteredPlaylists))
	for _, playlist := range filteredPlaylists {
		playlistInfos = append(playlistInfos, playlist.ToPlaylistInfo(user))
	}

	// Return playlist infos
	return GetUserPlaylistsResult{
		Playlists: playlistInfos,
	}, nil
}

// UpdatePlaylistParams represents the parameters for the updatePlaylist method.
type UpdatePlaylistParams struct {
	PlaylistID  string   `json:"playlistId" validate:"required"`
	Name        string   `json:"name,omitempty" validate:"omitempty,min=1,max=50"`
	Description string   `json:"description,omitempty" validate:"max=1000"`
	IsPrivate   *bool    `json:"isPrivate,omitempty"`
	Tags        []string `json:"tags,omitempty" validate:"dive,max=20"`
	CoverImage  string   `json:"coverImage,omitempty" validate:"omitempty,url"`
}

// UpdatePlaylistResult represents the result of the updatePlaylist method.
type UpdatePlaylistResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
}

// UpdatePlaylist handles updating a playlist.
func (h *PlaylistHandler) UpdatePlaylist(ctx context.Context, client *rpc.Client, p *UpdatePlaylistParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert playlist ID to ObjectID
	playlistObjID, err := bson.ObjectIDFromHex(p.PlaylistID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid playlist ID",
		}
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Playlist not found",
		}
	}

	// Check if user is the owner
	if playlist.Owner.Hex() != client.UserID {
		return nil, &rpc.Error{
			Code:    rpc.ErrNotAuthorized,
			Message: "You do not have permission to update this playlist",
		}
	}

	// Update playlist fields
	if p.Name != "" {
		playlist.Name = p.Name
	}
	if p.Description != "" {
		playlist.Description = p.Description
	}
	if p.IsPrivate != nil {
		playlist.IsPrivate = *p.IsPrivate
	}
	if p.Tags != nil {
		playlist.Tags = p.Tags
	}
	if p.CoverImage != "" {
		playlist.CoverImage = p.CoverImage
	}

	// Update playlist
	updatedPlaylist, err := h.playlistManager.UpdatePlaylist(ctx, playlist)
	if err != nil {
		h.logger.Error("Failed to update playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to update playlist",
		}
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user for playlist info", err, "userId", client.UserID)
		// Continue anyway, we'll just return the playlist without owner info
	}

	// Return playlist info
	playlistInfo := updatedPlaylist.ToPlaylistInfo(user)
	return UpdatePlaylistResult{
		Playlist: playlistInfo,
	}, nil
}

// DeletePlaylistParams represents the parameters for the deletePlaylist method.
type DeletePlaylistParams struct {
	PlaylistID string `json:"playlistId" validate:"required"`
}

// DeletePlaylistResult represents the result of the deletePlaylist method.
type DeletePlaylistResult struct {
	Success bool `json:"success"`
}

// DeletePlaylist handles deleting a playlist.
func (h *PlaylistHandler) DeletePlaylist(ctx context.Context, client *rpc.Client, p *DeletePlaylistParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert playlist ID to ObjectID
	playlistObjID, err := bson.ObjectIDFromHex(p.PlaylistID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid playlist ID",
		}
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Playlist not found",
		}
	}

	// Check if user is the owner
	if playlist.Owner.Hex() != client.UserID {
		return nil, &rpc.Error{
			Code:    rpc.ErrNotAuthorized,
			Message: "You do not have permission to delete this playlist",
		}
	}

	// Delete playlist
	err = h.playlistManager.DeletePlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to delete playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to delete playlist",
		}
	}

	// Return success
	return DeletePlaylistResult{
		Success: true,
	}, nil
}

// AddPlaylistItemParams represents the parameters for the addPlaylistItem method.
type AddPlaylistItemParams struct {
	PlaylistID string `json:"playlistId" validate:"required"`
	MediaID    string `json:"mediaId" validate:"required"`
	Position   *int   `json:"position,omitempty"`
}

// AddPlaylistItemResult represents the result of the addPlaylistItem method.
type AddPlaylistItemResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
}

// AddPlaylistItem handles adding an item to a playlist.
func (h *PlaylistHandler) AddPlaylistItem(ctx context.Context, client *rpc.Client, p *AddPlaylistItemParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert IDs to ObjectIDs
	playlistObjID, err := bson.ObjectIDFromHex(p.PlaylistID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid playlist ID",
		}
	}

	mediaObjID, err := bson.ObjectIDFromHex(p.MediaID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid media ID",
		}
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Playlist not found",
		}
	}

	// Check if user is the owner
	if playlist.Owner.Hex() != client.UserID {
		return nil, &rpc.Error{
			Code:    rpc.ErrNotAuthorized,
			Message: "You do not have permission to modify this playlist",
		}
	}

	// Set default position if not provided
	position := -1 // Append to end
	if p.Position != nil {
		position = *p.Position
	}

	// Add item to playlist
	updatedPlaylist, err := h.playlistManager.AddPlaylistItem(ctx, playlistObjID, mediaObjID, position)
	if err != nil {
		h.logger.Error("Failed to add item to playlist", err, "playlistId", p.PlaylistID, "mediaId", p.MediaID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to add item to playlist",
		}
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user for playlist info", err, "userId", client.UserID)
		// Continue anyway, we'll just return the playlist without owner info
	}

	// Return playlist info
	playlistInfo := updatedPlaylist.ToPlaylistInfo(user)
	return AddPlaylistItemResult{
		Playlist: playlistInfo,
	}, nil
}

// RemovePlaylistItemParams represents the parameters for the removePlaylistItem method.
type RemovePlaylistItemParams struct {
	PlaylistID string `json:"playlistId" validate:"required"`
	ItemID     string `json:"itemId" validate:"required"`
}

// RemovePlaylistItemResult represents the result of the removePlaylistItem method.
type RemovePlaylistItemResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
}

// RemovePlaylistItem handles removing an item from a playlist.
func (h *PlaylistHandler) RemovePlaylistItem(ctx context.Context, client *rpc.Client, p *RemovePlaylistItemParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert IDs to ObjectIDs
	playlistObjID, err := bson.ObjectIDFromHex(p.PlaylistID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid playlist ID",
		}
	}

	itemObjID, err := bson.ObjectIDFromHex(p.ItemID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid item ID",
		}
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Playlist not found",
		}
	}

	// Check if user is the owner
	if playlist.Owner.Hex() != client.UserID {
		return nil, &rpc.Error{
			Code:    rpc.ErrNotAuthorized,
			Message: "You do not have permission to modify this playlist",
		}
	}

	// Remove item from playlist
	updatedPlaylist, err := h.playlistManager.RemovePlaylistItem(ctx, playlistObjID, itemObjID)
	if err != nil {
		h.logger.Error("Failed to remove item from playlist", err, "playlistId", p.PlaylistID, "itemId", p.ItemID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to remove item from playlist",
		}
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user for playlist info", err, "userId", client.UserID)
		// Continue anyway, we'll just return the playlist without owner info
	}

	// Return playlist info
	playlistInfo := updatedPlaylist.ToPlaylistInfo(user)
	return RemovePlaylistItemResult{
		Playlist: playlistInfo,
	}, nil
}

// ImportPlaylistParams represents the parameters for the importPlaylist method.
type ImportPlaylistParams struct {
	Source    string `json:"source" validate:"required,oneof=youtube soundcloud"`
	SourceID  string `json:"sourceId" validate:"required"`
	Name      string `json:"name" validate:"required,min=1,max=50"`
	IsPrivate bool   `json:"isPrivate"`
}

// ImportPlaylistResult represents the result of the importPlaylist method.
type ImportPlaylistResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
	Success  bool                `json:"success"`
	Message  string              `json:"message,omitempty"`
}

// ImportPlaylist handles importing a playlist from an external source.
func (h *PlaylistHandler) ImportPlaylist(ctx context.Context, client *rpc.Client, p *ImportPlaylistParams) (any, error) {
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

	// Import playlist
	importedPlaylist, err := h.playlistManager.ImportPlaylist(ctx, userObjID, p.Source, p.SourceID)
	if err != nil {
		h.logger.Error("Failed to import playlist", err, "source", p.Source, "sourceId", p.SourceID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to import playlist",
		}
	}

	// Update playlist name and privacy
	importedPlaylist.Name = p.Name
	importedPlaylist.IsPrivate = p.IsPrivate
	importedPlaylist, err = h.playlistManager.UpdatePlaylist(ctx, importedPlaylist)
	if err != nil {
		h.logger.Error("Failed to update imported playlist", err, "playlistId", importedPlaylist.ID.Hex())
		// Continue anyway, we'll just return the playlist with the default name
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user for playlist info", err, "userId", client.UserID)
		// Continue anyway, we'll just return the playlist without owner info
	}

	// Return playlist info
	playlistInfo := importedPlaylist.ToPlaylistInfo(user)
	return ImportPlaylistResult{
		Playlist: playlistInfo,
		Success:  true,
		Message:  "Playlist imported successfully",
	}, nil
}

// SetActivePlaylistParams represents the parameters for the setActivePlaylist method.
type SetActivePlaylistParams struct {
	PlaylistID string `json:"playlistId" validate:"required"`
}

// SetActivePlaylistResult represents the result of the setActivePlaylist method.
type SetActivePlaylistResult struct {
	Success bool `json:"success"`
}

// SetActivePlaylist handles setting a playlist as the user's active playlist.
func (h *PlaylistHandler) SetActivePlaylist(ctx context.Context, client *rpc.Client, p *SetActivePlaylistParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert IDs to ObjectIDs
	userObjID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid user ID",
		}
	}

	playlistObjID, err := bson.ObjectIDFromHex(p.PlaylistID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid playlist ID",
		}
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Playlist not found",
		}
	}

	// Check if user is the owner
	if playlist.Owner.Hex() != client.UserID {
		return nil, &rpc.Error{
			Code:    rpc.ErrNotAuthorized,
			Message: "You do not have permission to set this playlist as active",
		}
	}

	// Set active playlist
	err = h.playlistManager.SetActivePlaylist(ctx, userObjID, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to set active playlist", err, "userId", client.UserID, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to set active playlist",
		}
	}

	// Return success
	return SetActivePlaylistResult{
		Success: true,
	}, nil
}

// GetActivePlaylistResult represents the result of the getActivePlaylist method.
type GetActivePlaylistResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
	Items    []models.MediaInfo  `json:"items"`
}

// GetActivePlaylist handles retrieving the user's active playlist.
func (h *PlaylistHandler) GetActivePlaylist(ctx context.Context, client *rpc.Client) (any, error) {
	// Convert user ID to ObjectID
	userObjID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid user ID",
		}
	}

	// Get active playlist
	playlist, err := h.playlistManager.GetActivePlaylist(ctx, userObjID)
	if err != nil {
		h.logger.Error("Failed to get active playlist", err, "userId", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get active playlist",
		}
	}

	// If no active playlist, return an error
	if playlist == nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "No active playlist found",
		}
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user for playlist info", err, "userId", client.UserID)
		// Continue anyway, we'll just return the playlist without owner info
	}

	// Extract media info from playlist items
	items := make([]models.MediaInfo, 0, len(playlist.Items))
	for _, item := range playlist.Items {
		if item.Media != nil {
			items = append(items, *item.Media)
		}
	}

	// Return playlist info and items
	playlistInfo := playlist.ToPlaylistInfo(user)
	return GetActivePlaylistResult{
		Playlist: playlistInfo,
		Items:    items,
	}, nil
}

// ShufflePlaylistParams represents the parameters for the shufflePlaylist method.
type ShufflePlaylistParams struct {
	PlaylistID string `json:"playlistId" validate:"required"`
}

// ShufflePlaylistResult represents the result of the shufflePlaylist method.
type ShufflePlaylistResult struct {
	Playlist models.PlaylistInfo `json:"playlist"`
}

// ShufflePlaylist handles shuffling a playlist.
func (h *PlaylistHandler) ShufflePlaylist(ctx context.Context, client *rpc.Client, p *ShufflePlaylistParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Convert playlist ID to ObjectID
	playlistObjID, err := bson.ObjectIDFromHex(p.PlaylistID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid playlist ID",
		}
	}

	// Get playlist
	playlist, err := h.playlistManager.GetPlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to get playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Playlist not found",
		}
	}

	// Check if user is the owner
	if playlist.Owner.Hex() != client.UserID {
		return nil, &rpc.Error{
			Code:    rpc.ErrNotAuthorized,
			Message: "You do not have permission to shuffle this playlist",
		}
	}

	// Shuffle playlist
	shuffledPlaylist, err := h.playlistManager.ShufflePlaylist(ctx, playlistObjID)
	if err != nil {
		h.logger.Error("Failed to shuffle playlist", err, "playlistId", p.PlaylistID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to shuffle playlist",
		}
	}

	// Get user for playlist info
	user, err := h.userManager.GetUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user for playlist info", err, "userId", client.UserID)
		// Continue anyway, we'll just return the playlist without owner info
	}

	// Return playlist info
	playlistInfo := shuffledPlaylist.ToPlaylistInfo(user)
	return ShufflePlaylistResult{
		Playlist: playlistInfo,
	}, nil
}

// SearchPlaylistsParams represents the parameters for the searchPlaylists method.
type SearchPlaylistsParams struct {
	Query          string   `json:"query"`
	Tags           []string `json:"tags"`
	IncludePrivate bool     `json:"includePrivate"`
	OwnerID        string   `json:"ownerId,omitempty"`
	SortBy         string   `json:"sortBy,omitempty"`
	SortDirection  string   `json:"sortDirection,omitempty"`
	Page           int      `json:"page,omitempty"`
	Limit          int      `json:"limit,omitempty"`
}

// SearchPlaylistsResult represents the result of the searchPlaylists method.
type SearchPlaylistsResult struct {
	Playlists []models.PlaylistInfo `json:"playlists"`
	Total     int64                 `json:"total"`
	Page      int                   `json:"page"`
	Limit     int                   `json:"limit"`
}

// SearchPlaylists handles searching for playlists.
func (h *PlaylistHandler) SearchPlaylists(ctx context.Context, client *rpc.Client, p *SearchPlaylistsParams) (any, error) {
	// Set default values
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = 20
	} else if p.Limit > 50 {
		p.Limit = 50
	}
	if p.SortBy == "" {
		p.SortBy = "createdAt"
	}
	if p.SortDirection == "" {
		p.SortDirection = "desc"
	}

	// Only include private playlists if the user is authenticated and is the owner
	if p.IncludePrivate && (client.UserID == "" || (p.OwnerID != "" && p.OwnerID != client.UserID)) {
		p.IncludePrivate = false
	}

	// Convert owner ID to ObjectID if provided
	var ownerObjID bson.ObjectID
	if p.OwnerID != "" {
		var err error
		ownerObjID, err = bson.ObjectIDFromHex(p.OwnerID)
		if err != nil {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "Invalid owner ID",
			}
		}
	}

	// Create search criteria
	criteria := models.PlaylistSearchCriteria{
		Query:          p.Query,
		Tags:           p.Tags,
		IncludePrivate: p.IncludePrivate,
		SortBy:         p.SortBy,
		SortDirection:  p.SortDirection,
		Page:           p.Page,
		Limit:          p.Limit,
	}
	if !ownerObjID.IsZero() {
		criteria.OwnerID = ownerObjID
	}

	// Search playlists
	playlists, total, err := h.playlistManager.SearchPlaylists(ctx, criteria)
	if err != nil {
		h.logger.Error("Failed to search playlists", err, "query", p.Query)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to search playlists",
		}
	}

	// Convert playlists to playlist info
	playlistInfos := make([]models.PlaylistInfo, 0, len(playlists))
	for _, playlist := range playlists {
		// Get owner for playlist info
		var owner *models.User
		if client.UserID != "" {
			owner, err = h.userManager.GetUserByID(ctx, playlist.Owner.Hex())
			if err != nil {
				h.logger.Error("Failed to get owner for playlist info", err, "ownerId", playlist.Owner.Hex())
				// Continue anyway, we'll just return the playlist without owner info
			}
		}
		playlistInfos = append(playlistInfos, playlist.ToPlaylistInfo(owner))
	}

	// Return search results
	return SearchPlaylistsResult{
		Playlists: playlistInfos,
		Total:     total,
		Page:      p.Page,
		Limit:     p.Limit,
	}, nil
}
