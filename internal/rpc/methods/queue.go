// Package methods contains RPC method handlers for the application.
package methods

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/utils"
)

// QueueHandler handles queue-related RPC methods.
type QueueHandler struct {
	queueManager *room.QueueManager
	logger       *utils.Logger
}

// NewQueueHandler creates a new QueueHandler.
func NewQueueHandler(queueManager *room.QueueManager, logger *utils.Logger) *QueueHandler {
	return &QueueHandler{
		queueManager: queueManager,
		logger:       logger,
	}
}

// RegisterMethods registers all queue-related RPC methods.
func (h *QueueHandler) RegisterMethods(hr rpc.HandlerRegistry) {
	auth := hr.Wrap(rpc.AuthMiddleware)
	rpc.Register(auth, "queue.join", h.JoinQueue)
	rpc.Register(auth, "queue.leave", h.LeaveQueue)
	rpc.Register(auth, "queue.move", h.MoveInQueue)
	rpc.Register(hr, "queue.get", h.GetQueue)
	rpc.Register(hr, "queue.getCurrentDJ", h.GetCurrentDJ)
	rpc.Register(hr, "queue.getCurrentMedia", h.GetCurrentMedia)
	rpc.Register(auth, "queue.advance", h.AdvanceQueue)
	rpc.Register(auth, "queue.playMedia", h.PlayMedia)
	rpc.Register(auth, "queue.skip", h.SkipCurrentMedia)
	rpc.Register(auth, "queue.clear", h.ClearQueue)
	rpc.Register(auth, "queue.shuffle", h.ShuffleQueue)
	rpc.Register(hr, "queue.getPosition", h.GetQueuePosition)
	rpc.Register(hr, "queue.isInQueue", h.IsUserInQueue)
	rpc.Register(hr, "queue.isCurrentDJ", h.IsUserCurrentDJ)
	rpc.Register(hr, "queue.getHistory", h.GetPlayHistory)
}

// JoinQueueParams represents the parameters for the JoinQueue method.
type JoinQueueParams struct {
	RoomID string `json:"roomId"`
}

// JoinQueue adds the current user to the DJ queue.
func (h *QueueHandler) JoinQueue(ctx context.Context, client *rpc.Client, p *JoinQueueParams) (any, error) {
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

	// Add user to queue
	roomState, err := h.queueManager.AddToQueue(ctx, roomID, userID)
	if err != nil {
		h.logger.Error("Failed to add user to queue", err, "roomId", p.RoomID, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// LeaveQueueParams represents the parameters for the LeaveQueue method.
type LeaveQueueParams struct {
	RoomID string `json:"roomId"`
}

// LeaveQueue removes the current user from the DJ queue.
func (h *QueueHandler) LeaveQueue(ctx context.Context, client *rpc.Client, p *LeaveQueueParams) (any, error) {
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

	// Remove user from queue
	roomState, err := h.queueManager.RemoveFromQueue(ctx, roomID, userID)
	if err != nil {
		h.logger.Error("Failed to remove user from queue", err, "roomId", p.RoomID, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// MoveInQueueParams represents the parameters for the MoveInQueue method.
type MoveInQueueParams struct {
	RoomID      string `json:"roomId"`
	UserID      string `json:"userId"`
	NewPosition int    `json:"newPosition"`
}

// MoveInQueue moves a user to a new position in the DJ queue.
func (h *QueueHandler) MoveInQueue(ctx context.Context, client *rpc.Client, p *MoveInQueueParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}
	if p.UserID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "userId is required", nil)
	}
	if p.NewPosition < 0 {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "newPosition must be non-negative", nil)
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

	// Move user in queue
	roomState, err := h.queueManager.MoveInQueue(ctx, roomID, userID, p.NewPosition)
	if err != nil {
		h.logger.Error("Failed to move user in queue", err, "roomId", p.RoomID, "userId", p.UserID, "newPosition", p.NewPosition)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// GetQueueParams represents the parameters for the GetQueue method.
type GetQueueParams struct {
	RoomID string `json:"roomId"`
}

// GetQueue gets the current DJ queue for a room.
func (h *QueueHandler) GetQueue(ctx context.Context, client *rpc.Client, p *GetQueueParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Get queue
	queue, err := h.queueManager.GetQueue(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get queue", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return queue, nil
}

// GetCurrentDJParams represents the parameters for the GetCurrentDJ method.
type GetCurrentDJParams struct {
	RoomID string `json:"roomId"`
}

// GetCurrentDJ gets the current DJ for a room.
func (h *QueueHandler) GetCurrentDJ(ctx context.Context, client *rpc.Client, p *GetCurrentDJParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Get current DJ
	dj, err := h.queueManager.GetCurrentDJ(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get current DJ", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return dj, nil
}

// GetCurrentMediaParams represents the parameters for the GetCurrentMedia method.
type GetCurrentMediaParams struct {
	RoomID string `json:"roomId"`
}

// GetCurrentMedia gets the currently playing media for a room.
func (h *QueueHandler) GetCurrentMedia(ctx context.Context, client *rpc.Client, p *GetCurrentMediaParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Get current media
	media, err := h.queueManager.GetCurrentMedia(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get current media", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return media, nil
}

// AdvanceQueueParams represents the parameters for the AdvanceQueue method.
type AdvanceQueueParams struct {
	RoomID string `json:"roomId"`
}

// AdvanceQueue advances to the next DJ in the queue.
func (h *QueueHandler) AdvanceQueue(ctx context.Context, client *rpc.Client, p *AdvanceQueueParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Advance queue
	roomState, err := h.queueManager.AdvanceQueue(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to advance queue", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// PlayMediaParams represents the parameters for the PlayMedia method.
type PlayMediaParams struct {
	RoomID    string            `json:"roomId"`
	MediaInfo *models.MediaInfo `json:"mediaInfo"`
}

// PlayMedia sets the currently playing media for a room.
func (h *QueueHandler) PlayMedia(ctx context.Context, client *rpc.Client, p *PlayMediaParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Check if user is current DJ
	userID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid userId", nil)
	}

	isCurrentDJ, err := h.queueManager.IsUserCurrentDJ(ctx, roomID, userID)
	if err != nil {
		h.logger.Error("Failed to check if user is current DJ", err, "roomId", p.RoomID, "userId", client.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	if !isCurrentDJ {
		return nil, rpc.NewError(rpc.ErrNotAuthorized, "only the current DJ can play media", nil)
	}

	// Play media
	roomState, err := h.queueManager.PlayMedia(ctx, roomID, p.MediaInfo)
	if err != nil {
		h.logger.Error("Failed to play media", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// SkipCurrentMediaParams represents the parameters for the SkipCurrentMedia method.
type SkipCurrentMediaParams struct {
	RoomID string `json:"roomId"`
}

// SkipCurrentMedia skips the currently playing media.
func (h *QueueHandler) SkipCurrentMedia(ctx context.Context, client *rpc.Client, p *SkipCurrentMediaParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Skip current media
	roomState, err := h.queueManager.SkipCurrentMedia(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to skip current media", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// ClearQueueParams represents the parameters for the ClearQueue method.
type ClearQueueParams struct {
	RoomID string `json:"roomId"`
}

// ClearQueue clears the DJ queue for a room.
func (h *QueueHandler) ClearQueue(ctx context.Context, client *rpc.Client, p *ClearQueueParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Clear queue
	roomState, err := h.queueManager.ClearQueue(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to clear queue", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// ShuffleQueueParams represents the parameters for the ShuffleQueue method.
type ShuffleQueueParams struct {
	RoomID string `json:"roomId"`
}

// ShuffleQueue randomly shuffles the DJ queue for a room.
func (h *QueueHandler) ShuffleQueue(ctx context.Context, client *rpc.Client, p *ShuffleQueueParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Shuffle queue
	roomState, err := h.queueManager.ShuffleQueue(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to shuffle queue", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return roomState, nil
}

// GetQueuePositionParams represents the parameters for the GetQueuePosition method.
type GetQueuePositionParams struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

// GetQueuePosition gets a user's position in the DJ queue.
func (h *QueueHandler) GetQueuePosition(ctx context.Context, client *rpc.Client, p *GetQueuePositionParams) (any, error) {
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

	// Get queue position
	position, err := h.queueManager.GetQueuePosition(ctx, roomID, userID)
	if err != nil {
		if errors.Is(err, errors.New("user is not in the queue")) {
			return -1, nil
		}
		h.logger.Error("Failed to get queue position", err, "roomId", p.RoomID, "userId", p.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return position, nil
}

// IsUserInQueueParams represents the parameters for the IsUserInQueue method.
type IsUserInQueueParams struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

// IsUserInQueue checks if a user is in the DJ queue.
func (h *QueueHandler) IsUserInQueue(ctx context.Context, client *rpc.Client, p *IsUserInQueueParams) (any, error) {
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

	// Check if user is in queue
	inQueue, err := h.queueManager.IsUserInQueue(ctx, roomID, userID)
	if err != nil {
		h.logger.Error("Failed to check if user is in queue", err, "roomId", p.RoomID, "userId", p.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return inQueue, nil
}

// IsUserCurrentDJParams represents the parameters for the IsUserCurrentDJ method.
type IsUserCurrentDJParams struct {
	RoomID string `json:"roomId"`
	UserID string `json:"userId"`
}

// IsUserCurrentDJ checks if a user is the current DJ.
func (h *QueueHandler) IsUserCurrentDJ(ctx context.Context, client *rpc.Client, p *IsUserCurrentDJParams) (any, error) {
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

	// Check if user is current DJ
	isCurrentDJ, err := h.queueManager.IsUserCurrentDJ(ctx, roomID, userID)
	if err != nil {
		h.logger.Error("Failed to check if user is current DJ", err, "roomId", p.RoomID, "userId", p.UserID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return isCurrentDJ, nil
}

// GetPlayHistoryParams represents the parameters for the GetPlayHistory method.
type GetPlayHistoryParams struct {
	RoomID string `json:"roomId"`
}

// GetPlayHistory gets the play history for a room.
func (h *QueueHandler) GetPlayHistory(ctx context.Context, client *rpc.Client, p *GetPlayHistoryParams) (any, error) {
	// Validate parameters
	if p.RoomID == "" {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "roomId is required", nil)
	}

	// Convert ID to ObjectID
	roomID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, rpc.NewError(rpc.ErrInvalidParams, "invalid roomId", nil)
	}

	// Get play history
	history, err := h.queueManager.GetPlayHistory(ctx, roomID)
	if err != nil {
		h.logger.Error("Failed to get play history", err, "roomId", p.RoomID)
		return nil, rpc.NewError(rpc.ErrInternalError, err.Error(), nil)
	}

	return history, nil
}
