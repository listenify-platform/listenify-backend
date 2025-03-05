// Package room provides services for room management and operations.
package room

import (
	"context"
	"errors"
	"time"

	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// Common chat-related errors
var (
	ErrMessageNotFound = errors.New("message not found")
	ErrNotAuthorized   = errors.New("not authorized to perform this action")
)

// ChatService provides chat functionality for rooms.
type ChatService interface {
	// SendMessage sends a chat message to a room.
	SendMessage(ctx context.Context, message models.ChatMessage) (models.ChatMessage, error)

	// GetMessages retrieves chat messages for a room.
	GetMessages(ctx context.Context, roomID string, limit int, before string) ([]models.ChatMessage, error)

	// DeleteMessage deletes a chat message.
	DeleteMessage(ctx context.Context, roomID string, messageID string, userID string) error
}

// ChatRoomManager defines the minimal room management operations needed by the chat service.
type ChatRoomManager interface {
	// GetRoom retrieves a room by ID.
	GetRoom(ctx context.Context, roomID bson.ObjectID) (*models.Room, error)

	// IsUserInRoom checks if a user is in a room.
	IsUserInRoom(ctx context.Context, roomID, userID bson.ObjectID) (bool, error)
}

// chatService implements the ChatService interface.
type chatService struct {
	roomManager ChatRoomManager
	chatRepo    repositories.ChatRepository
	userRepo    repositories.UserRepository
	pubSub      *managers.PubSubManager
	logger      *utils.Logger
}

// NewChatService creates a new chat service.
func NewChatService(
	roomManager ChatRoomManager,
	chatRepo repositories.ChatRepository,
	userRepo repositories.UserRepository,
	pubSub *managers.PubSubManager,
	logger *utils.Logger,
) ChatService {
	return &chatService{
		roomManager: roomManager,
		chatRepo:    chatRepo,
		userRepo:    userRepo,
		pubSub:      pubSub,
		logger:      logger.Named("chat_service"),
	}
}

// SendMessage sends a chat message to a room.
func (s *chatService) SendMessage(ctx context.Context, message models.ChatMessage) (models.ChatMessage, error) {
	// Validate room ID
	roomID, err := bson.ObjectIDFromHex(message.RoomID.Hex())
	if err != nil {
		return models.ChatMessage{}, models.ErrInvalidID
	}

	// Check if room exists
	room, err := s.roomManager.GetRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return models.ChatMessage{}, models.ErrRoomNotFound
		}
		s.logger.Error("Failed to get room", err, "roomId", roomID.Hex())
		return models.ChatMessage{}, err
	}

	// Check if room is active
	if !room.IsActive {
		return models.ChatMessage{}, models.ErrRoomInactive
	}

	// Check if user is in room
	userID, err := bson.ObjectIDFromHex(message.UserID.Hex())
	if err != nil {
		return models.ChatMessage{}, models.ErrInvalidID
	}

	isInRoom, err := s.roomManager.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		s.logger.Error("Failed to check if user is in room", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return models.ChatMessage{}, err
	}

	if !isInRoom {
		return models.ChatMessage{}, models.ErrUserNotInRoom
	}

	// Check if user is banned
	if slices.Contains(room.BannedUsers, userID) {
		return models.ChatMessage{}, models.ErrUserBanned
	}

	// Check if chat is disabled in the room
	if !room.Settings.ChatEnabled {
		return models.ChatMessage{}, models.ErrChatDisabled
	}

	// Check if user is muted
	// TODO: Implement mute check

	// Set message ID and creation time
	message.ID = bson.NewObjectID()
	message.CreatedAt = time.Now()

	// Get user's role in the room
	var userRole string
	if room.CreatedBy == userID {
		userRole = "owner"
	} else {
		if slices.Contains(room.Moderators, userID) {
			userRole = "moderator"
		}
		if userRole == "" {
			userRole = "user"
		}
	}
	message.UserRole = userRole

	// Store message in database
	err = s.chatRepo.SaveMessage(ctx, &message)
	if err != nil {
		s.logger.Error("Failed to save message", err, "roomId", roomID.Hex())
		return models.ChatMessage{}, err
	}

	// Broadcast message to room
	err = s.broadcastMessage(ctx, room.ID.Hex(), "chat_message", message)
	if err != nil {
		s.logger.Error("Failed to broadcast message", err, "roomId", roomID.Hex())
		// Continue anyway, the message was saved
	}

	return message, nil
}

// GetMessages retrieves chat messages for a room.
func (s *chatService) GetMessages(ctx context.Context, roomID string, limit int, before string) ([]models.ChatMessage, error) {
	// Validate room ID
	roomObjID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return nil, models.ErrInvalidID
	}

	// Check if room exists
	_, err = s.roomManager.GetRoom(ctx, roomObjID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return nil, models.ErrRoomNotFound
		}
		s.logger.Error("Failed to get room", err, "roomId", roomID)
		return nil, err
	}

	// Parse before ID if provided
	var beforeObjID bson.ObjectID
	if before != "" {
		beforeObjID, err = bson.ObjectIDFromHex(before)
		if err != nil {
			return nil, models.ErrInvalidID
		}
	}

	// Retrieve messages from database
	messages, err := s.chatRepo.FindMessagesByRoom(ctx, roomObjID, limit, beforeObjID)
	if err != nil {
		s.logger.Error("Failed to get messages", err, "roomId", roomID)
		return nil, err
	}

	// Convert to response format
	result := make([]models.ChatMessage, len(messages))
	for i, msg := range messages {
		result[i] = *msg
	}

	return result, nil
}

// DeleteMessage deletes a chat message.
func (s *chatService) DeleteMessage(ctx context.Context, roomID string, messageID string, userID string) error {
	// Validate IDs
	roomObjID, err := bson.ObjectIDFromHex(roomID)
	if err != nil {
		return models.ErrInvalidID
	}

	messageObjID, err := bson.ObjectIDFromHex(messageID)
	if err != nil {
		return models.ErrInvalidID
	}

	userObjID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Check if room exists
	room, err := s.roomManager.GetRoom(ctx, roomObjID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			return models.ErrRoomNotFound
		}
		s.logger.Error("Failed to get room", err, "roomId", roomID)
		return err
	}

	// Check if message exists
	message, err := s.chatRepo.FindMessageByID(ctx, messageObjID)
	if err != nil {
		if errors.Is(err, models.ErrMessageNotFound) {
			return models.ErrMessageNotFound
		}
		s.logger.Error("Failed to get message", err, "messageId", messageID)
		return err
	}

	// Check if message belongs to the room
	if message.RoomID != roomObjID {
		return models.ErrMessageNotFound
	}

	// Check if user is authorized to delete the message
	isAuthorized := false

	// User can delete their own messages
	if message.UserID == userObjID {
		isAuthorized = true
	}

	// Room owner and moderators can delete any message
	if !isAuthorized {
		if room.CreatedBy == userObjID {
			isAuthorized = true
		} else {
			if slices.Contains(room.Moderators, userObjID) {
				isAuthorized = true
			}
		}
	}

	if !isAuthorized {
		return ErrNotAuthorized
	}

	// Delete message
	err = s.chatRepo.DeleteMessage(ctx, messageObjID)
	if err != nil {
		s.logger.Error("Failed to delete message", err, "messageId", messageID)
		return err
	}

	// Update message with deletion info
	message.IsDeleted = true
	message.DeletedBy = userObjID
	message.DeletedAt = time.Now()

	// Broadcast message deletion
	err = s.broadcastMessage(ctx, roomID, "chat_message_deleted", map[string]any{
		"messageId": messageID,
		"deletedBy": userID,
	})
	if err != nil {
		s.logger.Error("Failed to broadcast message deletion", err, "messageId", messageID)
		// Continue anyway, the message was deleted
	}

	return nil
}

// broadcastMessage broadcasts a message to a room channel.
func (s *chatService) broadcastMessage(ctx context.Context, roomID string, eventType string, data any) error {
	return s.pubSub.PublishToRoom(ctx, roomID, eventType, data)
}
