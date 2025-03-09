// Package methods contains RPC method handlers for the application.
package methods

import (
	"context"
	"errors"

	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// RoomHandler handles room-related RPC methods.
type RoomHandler struct {
	roomManager room.RoomManager
	userMgr     *user.Manager
	logger      *utils.Logger
}

// NewRoomHandler creates a new RoomHandler.
func NewRoomHandler(roomManager room.RoomManager, userMgr *user.Manager, logger *utils.Logger) *RoomHandler {
	return &RoomHandler{
		roomManager: roomManager,
		userMgr:     userMgr,
		logger:      logger,
	}
}

// RegisterMethods registers all room-related RPC methods.
func (h *RoomHandler) RegisterMethods(hr rpc.HandlerRegistry) {
	auth := hr.Wrap(rpc.AuthMiddleware)
	rpc.Register(auth, rpc.MethodRoomCreate, h.CreateRoom)
	rpc.Register(hr, rpc.MethodRoomGet, h.GetRoom)
	rpc.Register(hr, rpc.MethodRoomGetBySlug, h.GetRoomBySlug)
	rpc.Register(auth, rpc.MethodRoomUpdate, h.UpdateRoom)
	rpc.Register(auth, rpc.MethodRoomDelete, h.DeleteRoom)
	rpc.Register(auth, rpc.MethodRoomJoin, h.JoinRoom)
	rpc.Register(auth, rpc.MethodRoomLeave, h.LeaveRoom)
	rpc.Register(hr, rpc.MethodRoomGetUsers, h.GetRoomUsers)
	rpc.Register(hr, rpc.MethodRoomIsUserInRoom, h.IsUserInRoom)
	rpc.Register(hr, rpc.MethodRoomGetState, h.GetRoomState)
	rpc.Register(hr, rpc.MethodRoomSearch, h.SearchRooms)
	rpc.Register(hr, rpc.MethodRoomGetActive, h.GetActiveRooms)
	rpc.Register(hr, rpc.MethodRoomGetPopular, h.GetPopularRooms)
}

// CreateRoomParams represents the parameters for the CreateRoom method.
type CreateRoomParams struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Slug        string              `json:"slug"`
	Settings    models.RoomSettings `json:"settings"`
}

// CreateRoom creates a new room.
func (h *RoomHandler) CreateRoom(ctx context.Context, client *rpc.Client, p *CreateRoomParams) (any, error) {
	// Validate parameters
	if p.Name == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "name is required", nil)
	}
	if p.Slug == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "slug is required", nil)
	}

	// Convert user ID to ObjectID
	userID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid userId", nil)
	}

	// Create room
	room := &models.Room{
		Name:        p.Name,
		Description: p.Description,
		Slug:        p.Slug,
		CreatedBy:   userID,
		Settings:    p.Settings,
		Moderators:  []bson.ObjectID{userID}, // Creator is automatically a moderator
		BannedUsers: []bson.ObjectID{},
		IsActive:    true,
	}

	// Create room
	createdRoom, err := h.roomManager.CreateRoom(ctx, room)
	if err != nil {
		h.logger.Error("Failed to create room", err, "name", p.Name, "slug", p.Slug, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Get user info for notifications
	user, err := h.userMgr.GetPublicUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user info", err, "userId", client.UserID)
		// Continue anyway, we have the room
	}

	// Get initial room state
	state, err := h.roomManager.GetRoomState(ctx, createdRoom.ID)
	if err != nil {
		h.logger.Error("Failed to get room state", err, "roomId", createdRoom.ID.Hex())
		// Continue anyway, we have the room
	}

	// Send single room created notification with complete state
	client.SendNotification(rpc.EventRoomCreated, struct {
		Room  *models.Room       `json:"room"`
		User  *models.PublicUser `json:"user,omitempty"`
		State *models.RoomState  `json:"state,omitempty"`
	}{
		Room:  createdRoom,
		User:  user,
		State: state,
	})

	// Join the room
	err = h.roomManager.JoinRoom(ctx, createdRoom.ID, userID)
	if err != nil {
		h.logger.Error("Failed to join room after creation", err, "roomId", createdRoom.ID.Hex(), "userId", client.UserID)
		// Continue anyway, the room was created successfully
	}

	return createdRoom, nil
}

// GetRoom gets a room by ID.
func (h *RoomHandler) GetRoom(ctx context.Context, client *rpc.Client, p *RoomIDParam) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert room ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Get room
	room, err := h.roomManager.GetRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to get room", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return room, nil
}

// GetRoomBySlugParams represents the parameters for the GetRoomBySlug method.
type GetRoomBySlugParams struct {
	Slug string `json:"slug"`
}

// GetRoomBySlug gets a room by slug.
func (h *RoomHandler) GetRoomBySlug(ctx context.Context, client *rpc.Client, p *GetRoomBySlugParams) (any, error) {
	// Validate parameters
	if p.Slug == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "slug is required", nil)
	}

	// Get room
	room, err := h.roomManager.GetRoomBySlug(ctx, p.Slug)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to get room by slug", err, "slug", p.Slug)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return room, nil
}

// UpdateRoomParams represents the parameters for the UpdateRoom method.
type UpdateRoomParams struct {
	RoomID      string              `json:"roomId"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Slug        string              `json:"slug"`
	Settings    models.RoomSettings `json:"settings"`
}

// UpdateRoom updates a room.
func (h *RoomHandler) UpdateRoom(ctx context.Context, client *rpc.Client, p *UpdateRoomParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}
	if p.Name == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "name is required", nil)
	}
	if p.Slug == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "slug is required", nil)
	}

	// Convert IDs to ObjectIDs
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	userID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid userId", nil)
	}

	// Get room
	room, err := h.roomManager.GetRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to get room", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Check if user is creator or moderator
	if room.CreatedBy != userID && !slices.Contains(room.Moderators, userID) {
		return nil, rpc.ErrNotAuthorized.Error()
	}

	// Update room
	room.Name = p.Name
	room.Description = p.Description
	room.Slug = p.Slug
	room.Settings = p.Settings

	// Update room
	updatedRoom, err := h.roomManager.UpdateRoom(ctx, room)
	if err != nil {
		h.logger.Error("Failed to update room", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Get updated room state
	state, err := h.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get room state", err, "roomId", p.RoomID)
		// Continue anyway, we have the updated room
	}

	// Send room updated notification with state
	client.SendRoomNotification(p.RoomID, rpc.EventRoomUpdated, struct {
		Room  *models.Room      `json:"room"`
		State *models.RoomState `json:"state,omitempty"`
	}{
		Room:  updatedRoom,
		State: state,
	})

	return updatedRoom, nil
}

// DeleteRoom deletes a room.
func (h *RoomHandler) DeleteRoom(ctx context.Context, client *rpc.Client, p *RoomIDParam) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert IDs to ObjectIDs
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	userID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid userId", nil)
	}

	// Get room
	room, err := h.roomManager.GetRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to get room", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Check if user is creator
	if room.CreatedBy != userID {
		return nil, rpc.ErrNotAuthorized.Error()
	}

	// Get final room state before deletion
	state, err := h.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get room state", err, "roomId", p.RoomID)
		// Continue anyway, we'll delete the room
	}

	// Send room deletion notification with final state
	if state != nil {
		client.SendRoomNotification(p.RoomID, rpc.EventRoomDeleted, struct {
			RoomID string            `json:"roomId"`
			State  *models.RoomState `json:"state"`
		}{
			RoomID: p.RoomID,
			State:  state,
		})
	}

	// Delete room
	err = h.roomManager.DeleteRoom(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to delete room", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return true, nil
}

// JoinRoom joins a room.
func (h *RoomHandler) JoinRoom(ctx context.Context, client *rpc.Client, p *RoomIDParam) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert IDs to ObjectIDs
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	userID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid userId", nil)
	}

	h.logger.Debug("User joining room", "roomId", p.RoomID, "userId", client.UserID)

	// Join room
	err = h.roomManager.JoinRoom(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		if errors.Is(err, errors.New("room is at capacity")) {
			return nil, rpc.ErrRoomFull.Error()
		}
		if errors.Is(err, errors.New("room is not active")) {
			return nil, rpc.ErrRoomClosed.Error()
		}
		if errors.Is(err, errors.New("user is banned from this room")) {
			return nil, rpc.NewError(rpc.ErrNotAuthorized, "user is banned from this room", nil)
		}
		h.logger.Error("Failed to join room", err, "roomId", p.RoomID, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Get user info before joining
	user, err := h.userMgr.GetPublicUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user info", err, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}
	if user == nil {
		h.logger.Error("User not found", nil, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrNotAuthorized, "user not found", nil)
	}

	// Get updated room state after join
	state, err := h.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get room state after joining", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Create notification payload with proper types and state
	joinPayload := struct {
		RoomID string             `json:"roomId"`
		User   *models.PublicUser `json:"user"`
		State  *models.RoomState  `json:"state"`
	}{
		RoomID: p.RoomID,
		User:   user,
		State:  state,
	}

	// Send single join notification with complete state
	client.JoinRoom(p.RoomID, rpc.EventUserJoinedRoom, joinPayload)

	h.logger.Debug("User joined room", "roomId", p.RoomID, "userId", client.UserID)

	// Return complete room state as method response
	return state, nil
}

// LeaveRoom leaves a room.
func (h *RoomHandler) LeaveRoom(ctx context.Context, client *rpc.Client, p *RoomIDParam) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert IDs to ObjectIDs
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	userID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid userId", nil)
	}

	h.logger.Debug("User leaving room", "roomId", p.RoomID, "userId", client.UserID)

	// Leave room
	err = h.roomManager.LeaveRoom(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to leave room", err, "roomId", p.RoomID, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Get final room state after leave
	state, err := h.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get room state after leaving", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Get user info before leaving for the notification
	user, err := h.userMgr.GetPublicUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user info for leave notification", err, "userId", client.UserID)
		// Continue with leave, but we won't have user info for notification
	}

	// Send single leave notification with complete state
	client.LeaveRoom(p.RoomID, rpc.EventUserLeftRoom, struct {
		RoomID string             `json:"roomId"`
		UserID string             `json:"userId"`
		User   *models.PublicUser `json:"user,omitempty"`
		State  *models.RoomState  `json:"state"`
	}{
		RoomID: p.RoomID,
		UserID: client.UserID,
		User:   user,
		State:  state,
	})

	h.logger.Debug("User left room", "roomId", p.RoomID, "userId", client.UserID)

	// Return complete room state as method response
	return state, nil
}

// HandleDisconnect handles cleanup when a user disconnects
func (h *RoomHandler) HandleDisconnect(ctx context.Context, client *rpc.Client) {
	h.logger.Debug("Handling user disconnect", "userId", client.UserID)

	// Get all rooms the user is in
	rooms := client.GetRooms()

	// Leave each room
	for _, roomID := range rooms {
		h.logger.Debug("Cleaning up user from room on disconnect", "roomId", roomID, "userId", client.UserID)

		// Leave room and send notifications
		if _, err := h.LeaveRoom(ctx, client, &RoomIDParam{RoomID: roomID}); err != nil {
			h.logger.Error("Failed to cleanup user from room", err, "roomId", roomID, "userId", client.UserID)
			// Continue with other rooms even if one fails
		}
	}

	// Get user for notifications
	user, err := h.userMgr.GetPublicUserByID(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to get user info for disconnect", err, "userId", client.UserID)
		// Continue with cleanup even without user info
	}

	// Send leave notifications for each room
	for _, roomID := range rooms {
		// Get room state for notification
		roomObjID, err := bson.ObjectIDFromHex(roomID)
		if err != nil {
			h.logger.Error("Invalid room ID during disconnect", err, "roomId", roomID)
			continue
		}

		state, err := h.roomManager.GetRoomState(ctx, roomObjID)
		if err != nil {
			h.logger.Error("Failed to get room state during disconnect", err, "roomId", roomID)
			continue
		}

		// Send single leave notification with complete state
		client.LeaveRoom(roomID, rpc.EventUserLeftRoom, struct {
			RoomID string             `json:"roomId"`
			UserID string             `json:"userId"`
			User   *models.PublicUser `json:"user,omitempty"`
			State  *models.RoomState  `json:"state"`
		}{
			RoomID: roomID,
			UserID: client.UserID,
			User:   user,
			State:  state,
		})
	}
}

// GetRoomUsers gets all users in a room.
func (h *RoomHandler) GetRoomUsers(ctx context.Context, client *rpc.Client, p *RoomIDParam) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert room ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Get room users
	users, err := h.roomManager.GetRoomUsers(ctx, roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to get room users", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return users, nil
}

// IsUserInRoomParams represents the parameters for the IsUserInRoom method.
type IsUserInRoomParams struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

// IsUserInRoom checks if a user is in a room.
func (h *RoomHandler) IsUserInRoom(ctx context.Context, client *rpc.Client, p *IsUserInRoomParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}
	if p.UserID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "userId is required", nil)
	}

	// Convert IDs to ObjectIDs
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	userID, err := bson.ObjectIDFromHex(p.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid userId", nil)
	}

	// Check if user is in room
	inRoom, err := h.roomManager.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to check if user is in room", err, "roomId", p.RoomID, "userId", p.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return inRoom, nil
}

// GetRoomState gets the current state of a room.
func (h *RoomHandler) GetRoomState(ctx context.Context, client *rpc.Client, p *RoomIDParam) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert room ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Get room state
	state, err := h.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, rpc.ErrRoomNotFound.Error()
		}
		h.logger.Error("Failed to get room state", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return state, nil
}

// SearchRoomsParams represents the parameters for the SearchRooms method.
type SearchRoomsParams struct {
	Query  string `json:"query"`
	Skip   int    `json:"skip"`
	Limit  int    `json:"limit"`
	SortBy string `json:"sortBy"`
}

// SearchRooms searches for rooms based on criteria.
func (h *RoomHandler) SearchRooms(ctx context.Context, client *rpc.Client, p *SearchRoomsParams) (any, error) {
	// Set default limit if not provided
	if p.Limit <= 0 {
		p.Limit = 20
	}

	// Create search criteria
	criteria := models.RoomSearchCriteria{
		Query:  p.Query,
		Page:   p.Skip,
		Limit:  p.Limit,
		SortBy: p.SortBy,
	}

	// Search rooms
	rooms, total, err := h.roomManager.SearchRooms(ctx, criteria)
	if err != nil {
		h.logger.Error("Failed to search rooms", err, "query", p.Query)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	// Create response
	response := struct {
		Rooms []*models.Room `json:"rooms"`
		Total int64          `json:"total"`
	}{
		Rooms: rooms,
		Total: total,
	}

	return response, nil
}

// GetActiveRoomsParams represents the parameters for the GetActiveRooms method.
type GetActiveRoomsParams struct {
	Limit int `json:"limit"`
}

// GetActiveRooms gets a list of active rooms.
func (h *RoomHandler) GetActiveRooms(ctx context.Context, client *rpc.Client, p *GetActiveRoomsParams) (any, error) {
	// Set default limit if not provided
	if p.Limit <= 0 {
		p.Limit = 20
	}

	// Get active rooms
	rooms, err := h.roomManager.GetActiveRooms(ctx, p.Limit)
	if err != nil {
		h.logger.Error("Failed to get active rooms", err)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return rooms, nil
}

// GetPopularRoomsParams represents the parameters for the GetPopularRooms method.
type GetPopularRoomsParams struct {
	Limit int `json:"limit"`
}

// GetPopularRooms gets a list of popular rooms.
func (h *RoomHandler) GetPopularRooms(ctx context.Context, client *rpc.Client, p *GetPopularRoomsParams) (any, error) {
	// Set default limit if not provided
	if p.Limit <= 0 {
		p.Limit = 20
	}

	// Get popular rooms
	rooms, err := h.roomManager.GetPopularRooms(ctx, p.Limit)
	if err != nil {
		h.logger.Error("Failed to get popular rooms", err)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return rooms, nil
}
