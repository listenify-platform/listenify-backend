// Package methods contains RPC method handlers for the application.
package methods

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/utils"
)

// ChatHandler handles chat-related RPC methods.
type ChatHandler struct {
	chatService room.ChatService
	logger      *utils.Logger
}

// NewChatHandler creates a new ChatHandler.
func NewChatHandler(chatService room.ChatService, logger *utils.Logger) *ChatHandler {
	return &ChatHandler{
		chatService: chatService,
		logger:      logger,
	}
}

// RegisterMethods registers chat-related RPC methods with the router.
func (h *ChatHandler) RegisterMethods(hr rpc.HandlerRegistry) {
	auth := hr.Wrap(rpc.AuthMiddleware)
	rpc.Register(auth, "chat.sendMessage", h.SendMessage)
	rpc.Register(auth, "chat.getMessages", h.GetMessages)
	rpc.Register(auth, "chat.deleteMessage", h.DeleteMessage)
}

// SendMessageParams represents the parameters for the sendMessage method.
type SendMessageParams struct {
	RoomID  string `json:"roomId" validate:"required"`
	Content string `json:"content" validate:"required,min=1,max=500"`
	Type    string `json:"type,omitempty"`
}

// SendMessageResult represents the result of the sendMessage method.
type SendMessageResult struct {
	Message models.ChatMessage `json:"message"`
}

// SendMessage handles sending a chat message.
func (h *ChatHandler) SendMessage(ctx context.Context, client *rpc.Client, p *SendMessageParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Set default message type if not provided
	messageType := p.Type
	if messageType == "" {
		messageType = "text"
	}

	// Convert string IDs to ObjectIDs
	userObjID, err := bson.ObjectIDFromHex(client.UserID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid user ID",
		}
	}

	roomObjID, err := bson.ObjectIDFromHex(p.RoomID)
	if err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid room ID",
		}
	}

	// Create message
	message := models.ChatMessage{
		UserID:    userObjID,
		RoomID:    roomObjID,
		Content:   p.Content,
		Type:      messageType,
		CreatedAt: time.Now(),
	}

	// Send message
	sentMessage, err := h.chatService.SendMessage(ctx, message)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "Room not found",
			}
		}
		if errors.Is(err, models.ErrUserNotInRoom) {
			return nil, &rpc.Error{
				Code:    rpc.ErrNotAuthorized,
				Message: "You are not in this room",
			}
		}
		h.logger.Error("Failed to send message", err, "roomId", p.RoomID, "userId", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to send message",
		}
	}

	// Return sent message
	return SendMessageResult{
		Message: sentMessage,
	}, nil
}

// GetMessagesParams represents the parameters for the getMessages method.
type GetMessagesParams struct {
	RoomID string `json:"roomId" validate:"required"`
	Limit  int    `json:"limit,omitempty"`
	Before string `json:"before,omitempty"`
}

// GetMessagesResult represents the result of the getMessages method.
type GetMessagesResult struct {
	Messages []models.ChatMessage `json:"messages"`
}

// GetMessages handles retrieving chat messages for a room.
func (h *ChatHandler) GetMessages(ctx context.Context, client *rpc.Client, p *GetMessagesParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Set default limit if not provided
	limit := p.Limit
	if limit <= 0 {
		limit = 50
	}

	// Get messages
	messages, err := h.chatService.GetMessages(ctx, p.RoomID, limit, p.Before)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "Room not found",
			}
		}
		if errors.Is(err, models.ErrUserNotInRoom) {
			return nil, &rpc.Error{
				Code:    rpc.ErrNotAuthorized,
				Message: "You are not in this room",
			}
		}
		h.logger.Error("Failed to get messages", err, "roomId", p.RoomID, "userId", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get messages",
		}
	}

	// Return messages
	return GetMessagesResult{
		Messages: messages,
	}, nil
}

// DeleteMessageParams represents the parameters for the deleteMessage method.
type DeleteMessageParams struct {
	RoomID    string `json:"roomId" validate:"required"`
	MessageID string `json:"messageId" validate:"required"`
}

// DeleteMessageResult represents the result of the deleteMessage method.
type DeleteMessageResult struct {
	Success bool `json:"success"`
}

// DeleteMessage handles deleting a chat message.
func (h *ChatHandler) DeleteMessage(ctx context.Context, client *rpc.Client, p *DeleteMessageParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Delete message
	err := h.chatService.DeleteMessage(ctx, p.RoomID, p.MessageID, client.UserID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "Room not found",
			}
		}
		if errors.Is(err, room.ErrMessageNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "Message not found",
			}
		}
		if errors.Is(err, room.ErrNotAuthorized) {
			return nil, &rpc.Error{
				Code:    rpc.ErrNotAuthorized,
				Message: "You are not authorized to delete this message",
			}
		}
		h.logger.Error("Failed to delete message", err, "roomId", p.RoomID, "messageId", p.MessageID, "userId", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to delete message",
		}
	}

	// Return success
	return DeleteMessageResult{
		Success: true,
	}, nil
}
