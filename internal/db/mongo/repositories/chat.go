// Package repositories contains MongoDB repository implementations.
package repositories

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// Collection name
const (
	chatMessagesCollection = "chat_messages"
)

// ChatRepository defines the interface for chat message data access operations.
type ChatRepository interface {
	// Message operations
	SaveMessage(ctx context.Context, message *models.ChatMessage) error
	FindMessageByID(ctx context.Context, id bson.ObjectID) (*models.ChatMessage, error)
	FindMessagesByRoom(ctx context.Context, roomID bson.ObjectID, limit int, before bson.ObjectID) ([]*models.ChatMessage, error)
	DeleteMessage(ctx context.Context, id bson.ObjectID) error
	UpdateMessage(ctx context.Context, message *models.ChatMessage) error

	// Moderation operations
	DeleteMessagesByUser(ctx context.Context, roomID, userID bson.ObjectID) (int64, error)
}

// chatRepository is the MongoDB implementation of ChatRepository.
type chatRepository struct {
	collection *mongo.Collection
	logger     *utils.Logger
}

// NewChatRepository creates a new instance of ChatRepository.
func NewChatRepository(db *mongo.Database, logger *utils.Logger) ChatRepository {
	return &chatRepository{
		collection: db.Collection(chatMessagesCollection),
		logger:     logger.Named("chat_repository"),
	}
}

// SaveMessage saves a chat message to the database.
func (r *chatRepository) SaveMessage(ctx context.Context, message *models.ChatMessage) error {
	if message.ID.IsZero() {
		message.ID = bson.NewObjectID()
	}

	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}

	_, err := r.collection.InsertOne(ctx, message)
	if err != nil {
		r.logger.Error("Failed to save chat message", err, "roomId", message.RoomID.Hex())
		return models.NewInternalError(err, "Failed to save chat message")
	}

	return nil
}

// FindMessageByID finds a chat message by its ID.
func (r *chatRepository) FindMessageByID(ctx context.Context, id bson.ObjectID) (*models.ChatMessage, error) {
	var message models.ChatMessage

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&message)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrMessageNotFound
		}
		r.logger.Error("Failed to find chat message by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find chat message")
	}

	return &message, nil
}

// FindMessagesByRoom finds chat messages for a room.
func (r *chatRepository) FindMessagesByRoom(ctx context.Context, roomID bson.ObjectID, limit int, before bson.ObjectID) ([]*models.ChatMessage, error) {
	if limit <= 0 {
		limit = 50 // Default limit
	}

	filter := bson.M{"roomId": roomID}

	// If before ID is provided, only get messages before that ID
	if !before.IsZero() {
		// Get the message to find its timestamp
		beforeMsg, err := r.FindMessageByID(ctx, before)
		if err != nil && !errors.Is(err, models.ErrMessageNotFound) {
			return nil, err
		}

		if beforeMsg != nil {
			filter["createdAt"] = bson.M{"$lt": beforeMsg.CreatedAt}
		}
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdAt": -1}) // Most recent first

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find chat messages", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to find chat messages")
	}
	defer cursor.Close(ctx)

	var messages []*models.ChatMessage
	if err = cursor.All(ctx, &messages); err != nil {
		r.logger.Error("Failed to decode chat messages", err)
		return nil, models.NewInternalError(err, "Failed to decode chat messages")
	}

	return messages, nil
}

// DeleteMessage deletes a chat message.
func (r *chatRepository) DeleteMessage(ctx context.Context, id bson.ObjectID) error {
	result, err := r.collection.UpdateByID(
		ctx,
		id,
		bson.D{
			cmdSet(bson.M{
				"isDeleted": true,
				"deletedAt": time.Now(),
			}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to delete chat message", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to delete chat message")
	}

	if result.MatchedCount == 0 {
		return models.ErrMessageNotFound
	}

	return nil
}

// UpdateMessage updates a chat message.
func (r *chatRepository) UpdateMessage(ctx context.Context, message *models.ChatMessage) error {
	message.IsEdited = true
	message.EditedAt = time.Now()

	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": message.ID}, message)
	if err != nil {
		r.logger.Error("Failed to update chat message", err, "id", message.ID.Hex())
		return models.NewInternalError(err, "Failed to update chat message")
	}

	if result.MatchedCount == 0 {
		return models.ErrMessageNotFound
	}

	return nil
}

// DeleteMessagesByUser deletes all messages from a user in a room.
func (r *chatRepository) DeleteMessagesByUser(ctx context.Context, roomID, userID bson.ObjectID) (int64, error) {
	result, err := r.collection.UpdateMany(
		ctx,
		roomAndUserIDs(roomID, userID),
		bson.D{
			cmdSet(bson.M{
				"isDeleted": true,
				"deletedAt": time.Now(),
			}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to delete user's chat messages", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return 0, models.NewInternalError(err, "Failed to delete user's chat messages")
	}

	return result.ModifiedCount, nil
}
