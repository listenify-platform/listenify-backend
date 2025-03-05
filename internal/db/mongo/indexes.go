// Package mongo provides MongoDB database connectivity and repositories.
package mongo

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"norelock.dev/listenify/backend/internal/utils"
)

// Collection name constants for use throughout the application
const (
	UsersCollection          = "users"
	RoomsCollection          = "rooms"
	RoomUsersCollection      = "room_users"
	MediaCollection          = "media"
	PlaylistsCollection      = "playlists"
	ChatCollection           = "chat_messages"
	ChatEmoteCollection      = "chat_emotes"
	ChatCommandCollection    = "chat_commands"
	ChatModerationCollection = "chat_moderation"
	HistoryCollection        = "history"
	PlayHistoryCollection    = "play_history"
	UserHistoryCollection    = "user_history"
	RoomHistoryCollection    = "room_history"
	DJHistoryCollection      = "dj_history"
	SessionHistoryCollection = "session_history"
	ModHistoryCollection     = "moderation_history"
)

// IndexCreator defines a function type for index creation
type IndexCreator func(context.Context, *Client) error

// Index creators for different collections
var (
	indexCreators = map[string]IndexCreator{
		UsersCollection:     ensureUserIndexes,
		RoomsCollection:     ensureRoomIndexes,
		MediaCollection:     ensureMediaIndexes,
		PlaylistsCollection: ensurePlaylistIndexes,
		ChatCollection:      ensureChatIndexes,
		HistoryCollection:   ensureHistoryIndexes,
	}
)

// EnsureIndexes creates all necessary indexes for the application
func EnsureIndexes(ctx context.Context, client *Client) error {
	logger := client.Logger().With("operation", "EnsureIndexes")
	logger.Info("Starting index creation for all collections")

	// For sequential execution
	for collection, creator := range indexCreators {
		logger.Info("Creating indexes", "collection", collection)
		if err := creator(ctx, client); err != nil {
			logger.Error("Failed to create indexes", err, "collection", collection)
			return fmt.Errorf("failed to create indexes for %s: %w", collection, err)
		}
	}

	logger.Info("Successfully created all indexes")
	return nil
}

// EnsureIndexesParallel creates all necessary indexes for the application in parallel
func EnsureIndexesParallel(ctx context.Context, client *Client) error {
	logger := client.Logger().With("operation", "EnsureIndexesParallel")
	logger.Info("Starting parallel index creation for all collections")

	var wg sync.WaitGroup
	errChan := make(chan error, len(indexCreators))

	// Launch index creation in parallel
	for collection, creator := range indexCreators {
		wg.Add(1)
		go func(collName string, indexCreator IndexCreator) {
			defer wg.Done()
			logger.Info("Creating indexes", "collection", collName)
			if err := indexCreator(ctx, client); err != nil {
				logger.Error("Failed to create indexes", err, "collection", collName)
				errChan <- fmt.Errorf("failed to create indexes for %s: %w", collName, err)
			}
		}(collection, creator)
	}

	// Wait for all goroutines to finish
	wg.Wait()
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		err := <-errChan
		return err
	}

	logger.Info("Successfully created all indexes in parallel")
	return nil
}

// createIndexes is a helper function to create multiple indexes for a collection
func createIndexes(ctx context.Context, collection *mongo.Collection, indexes []mongo.IndexModel, logger *utils.Logger, collectionName string) error {
	if len(indexes) == 0 {
		return nil
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		logger.Error("Failed to create indexes", err, "collection", collectionName)
		return err
	}

	logger.Info("Successfully created indexes", "collection", collectionName, "count", len(indexes))
	return nil
}

// ensureUserIndexes creates indexes for the users collection
func ensureUserIndexes(ctx context.Context, client *Client) error {
	collection := client.Collection(UsersCollection)
	logger := client.Logger().With("operation", "ensureUserIndexes")

	indexes := []mongo.IndexModel{
		// Email index (unique)
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		// Username index (unique, case-insensitive)
		{
			Keys: bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true).SetCollation(&options.Collation{
				Locale:    "en",
				Strength:  2, // Case-insensitive
				CaseLevel: false,
			}),
		},
		// LastLogin index (for filtering and sorting inactive users)
		{
			Keys:    bson.D{{Key: "lastLogin", Value: -1}},
			Options: options.Index(),
		},
		// CreatedAt index (for sorting and filtering)
		{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index(),
		},
		// Social connections index
		{
			Keys: bson.D{
				{Key: "connections.following", Value: 1},
				{Key: "connections.followers", Value: 1},
			},
			Options: options.Index(),
		},
		// Roles index (for permission checks)
		{
			Keys:    bson.D{{Key: "roles", Value: 1}},
			Options: options.Index(),
		},
	}

	return createIndexes(ctx, collection, indexes, logger, UsersCollection)
}

// ensureRoomIndexes creates indexes for room-related collections
func ensureRoomIndexes(ctx context.Context, client *Client) error {
	roomCollection := client.Collection(RoomsCollection)
	roomUsersCollection := client.Collection(RoomUsersCollection)
	logger := client.Logger().With("operation", "ensureRoomIndexes")

	// Indexes for Rooms collection
	roomIndexes := []mongo.IndexModel{
		// Slug index (unique, case-insensitive)
		{
			Keys: bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true).SetCollation(&options.Collation{
				Locale:    "en",
				Strength:  2, // Case-insensitive
				CaseLevel: false,
			}),
		},
		// Name text index (for searching)
		{
			Keys:    bson.D{{Key: "name", Value: "text"}},
			Options: options.Index().SetWeights(bson.D{{Key: "name", Value: 10}}),
		},
		// CreatedBy index
		{
			Keys:    bson.D{{Key: "createdBy", Value: 1}},
			Options: options.Index(),
		},
		// IsActive index
		{
			Keys:    bson.D{{Key: "isActive", Value: 1}},
			Options: options.Index(),
		},
		// Active + LastActivity index
		{
			Keys: bson.D{
				{Key: "isActive", Value: 1},
				{Key: "lastActivity", Value: 1},
			},
			Options: options.Index(),
		},
		// Tags index
		{
			Keys:    bson.D{{Key: "tags", Value: 1}},
			Options: options.Index(),
		},
	}

	// Indexes for RoomUsers collection
	roomUserIndexes := []mongo.IndexModel{
		// TTL index for auto-cleanup
		{
			Keys:    bson.D{{Key: "lastActive", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(3600 * 24 * 7), // 7 days
		},
		// RoomId + UserId unique index
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "userId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		// RoomId + Position index
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "position", Value: 1},
			},
			Options: options.Index(),
		},
	}

	// Create indexes for Rooms collection
	if err := createIndexes(ctx, roomCollection, roomIndexes, logger, RoomsCollection); err != nil {
		return err
	}

	// Create indexes for RoomUsers collection
	return createIndexes(ctx, roomUsersCollection, roomUserIndexes, logger, RoomUsersCollection)
}

// ensureMediaIndexes creates indexes for the media collection
func ensureMediaIndexes(ctx context.Context, client *Client) error {
	collection := client.Collection(MediaCollection)
	logger := client.Logger().With("operation", "ensureMediaIndexes")

	indexes := []mongo.IndexModel{
		// Unique index for media source and ID
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "sourceId", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		// Text index for searching
		{
			Keys: bson.D{
				{Key: "title", Value: "text"},
				{Key: "artist", Value: "text"},
			},
			Options: options.Index().SetWeights(bson.D{
				{Key: "title", Value: 10},
				{Key: "artist", Value: 5},
			}),
		},
		// AddedBy index
		{
			Keys:    bson.D{{Key: "addedBy", Value: 1}},
			Options: options.Index(),
		},
		// CreatedAt index
		{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index(),
		},
		// PlayCount index
		{
			Keys:    bson.D{{Key: "stats.playCount", Value: -1}},
			Options: options.Index(),
		},
	}

	return createIndexes(ctx, collection, indexes, logger, MediaCollection)
}

// ensurePlaylistIndexes creates indexes for the playlists collection
func ensurePlaylistIndexes(ctx context.Context, client *Client) error {
	collection := client.Collection(PlaylistsCollection)
	logger := client.Logger().With("operation", "ensurePlaylistIndexes")

	indexes := []mongo.IndexModel{
		// Owner index
		{
			Keys:    bson.D{{Key: "owner", Value: 1}},
			Options: options.Index(),
		},
		// Active playlist index
		{
			Keys: bson.D{
				{Key: "owner", Value: 1},
				{Key: "isActive", Value: 1},
			},
			Options: options.Index(),
		},
		// Text index for searching
		{
			Keys: bson.D{
				{Key: "name", Value: "text"},
				{Key: "description", Value: "text"},
			},
			Options: options.Index().SetWeights(bson.D{
				{Key: "name", Value: 10},
				{Key: "description", Value: 5},
			}),
		},
		// Tags index
		{
			Keys:    bson.D{{Key: "tags", Value: 1}},
			Options: options.Index(),
		},
		// UpdatedAt index
		{
			Keys:    bson.D{{Key: "updatedAt", Value: -1}},
			Options: options.Index(),
		},
	}

	return createIndexes(ctx, collection, indexes, logger, PlaylistsCollection)
}

// ensureChatIndexes creates indexes for chat-related collections
func ensureChatIndexes(ctx context.Context, client *Client) error {
	chatCollection := client.Collection(ChatCollection)
	emoteCollection := client.Collection(ChatEmoteCollection)
	commandCollection := client.Collection(ChatCommandCollection)
	moderationCollection := client.Collection(ChatModerationCollection)
	logger := client.Logger().With("operation", "ensureChatIndexes")

	// Indexes for main chat messages collection
	chatIndexes := []mongo.IndexModel{
		// Room index
		{
			Keys:    bson.D{{Key: "roomId", Value: 1}},
			Options: options.Index(),
		},
		// Room + CreatedAt index
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "createdAt", Value: -1},
			},
			Options: options.Index(),
		},
		// User index
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index(),
		},
		// Mentions index
		{
			Keys:    bson.D{{Key: "mentions", Value: 1}},
			Options: options.Index(),
		},
		// Type index
		{
			Keys:    bson.D{{Key: "type", Value: 1}},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "createdAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(3600 * 24 * 30), // 30 days
		},
	}

	// Indexes for chat emotes collection
	emoteIndexes := []mongo.IndexModel{
		// Code index (unique for global emotes)
		{
			Keys:    bson.D{{Key: "code", Value: 1}},
			Options: options.Index(),
		},
		// Room + Code index (unique per room)
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "code", Value: 1},
			},
			Options: options.Index(),
		},
		// Global flag index
		{
			Keys:    bson.D{{Key: "isGlobal", Value: 1}},
			Options: options.Index(),
		},
	}

	// Indexes for chat commands collection
	commandIndexes := []mongo.IndexModel{
		// Name index (unique)
		{
			Keys:    bson.D{{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		// Minimum role index
		{
			Keys:    bson.D{{Key: "minimumRole", Value: 1}},
			Options: options.Index(),
		},
		// Enabled index
		{
			Keys:    bson.D{{Key: "enabled", Value: 1}},
			Options: options.Index(),
		},
	}

	// Indexes for chat moderation collection
	moderationIndexes := []mongo.IndexModel{
		// Room index
		{
			Keys:    bson.D{{Key: "roomId", Value: 1}},
			Options: options.Index(),
		},
		// Target user index
		{
			Keys:    bson.D{{Key: "targetUserId", Value: 1}},
			Options: options.Index(),
		},
		// Moderator index
		{
			Keys:    bson.D{{Key: "moderatorId", Value: 1}},
			Options: options.Index(),
		},
		// Action index
		{
			Keys:    bson.D{{Key: "action", Value: 1}},
			Options: options.Index(),
		},
		// ExpiresAt index (for finding active bans/mutes)
		{
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index(),
		},
		// Combined index for searching active moderation by user in room
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "targetUserId", Value: 1},
				{Key: "action", Value: 1},
				{Key: "expiresAt", Value: 1},
			},
			Options: options.Index(),
		},
	}

	// Create all indexes
	if err := createIndexes(ctx, chatCollection, chatIndexes, logger, ChatCollection); err != nil {
		return err
	}

	if err := createIndexes(ctx, emoteCollection, emoteIndexes, logger, ChatEmoteCollection); err != nil {
		return err
	}

	if err := createIndexes(ctx, commandCollection, commandIndexes, logger, ChatCommandCollection); err != nil {
		return err
	}

	return createIndexes(ctx, moderationCollection, moderationIndexes, logger, ChatModerationCollection)
}

// ensureHistoryIndexes creates indexes for all history-related collections
func ensureHistoryIndexes(ctx context.Context, client *Client) error {
	logger := client.Logger().With("operation", "ensureHistoryIndexes")

	// Get all history collection references
	historyCollection := client.Collection(HistoryCollection)
	playHistoryCollection := client.Collection(PlayHistoryCollection)
	userHistoryCollection := client.Collection(UserHistoryCollection)
	roomHistoryCollection := client.Collection(RoomHistoryCollection)
	djHistoryCollection := client.Collection(DJHistoryCollection)
	sessionHistoryCollection := client.Collection(SessionHistoryCollection)
	modHistoryCollection := client.Collection(ModHistoryCollection)

	// TTL index for all history collections (reused)
	longTTL := options.Index().SetExpireAfterSeconds(3600 * 24 * 180) // 180 days
	shortTTL := options.Index().SetExpireAfterSeconds(3600 * 24 * 30) // 30 days

	// Main history collection indexes
	historyIndexes := []mongo.IndexModel{
		// Type index
		{
			Keys:    bson.D{{Key: "type", Value: 1}},
			Options: options.Index(),
		},
		// Reference ID index
		{
			Keys:    bson.D{{Key: "referenceId", Value: 1}},
			Options: options.Index(),
		},
		// Timestamp index
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index(),
		},
		// Type + Timestamp index
		{
			Keys: bson.D{
				{Key: "type", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "timestamp", Value: 1}},
			Options: longTTL,
		},
	}

	// Play history collection indexes
	playHistoryIndexes := []mongo.IndexModel{
		// Room index
		{
			Keys:    bson.D{{Key: "roomId", Value: 1}},
			Options: options.Index(),
		},
		// Media index
		{
			Keys:    bson.D{{Key: "mediaId", Value: 1}},
			Options: options.Index(),
		},
		// DJ index
		{
			Keys:    bson.D{{Key: "djId", Value: 1}},
			Options: options.Index(),
		},
		// Start time index
		{
			Keys:    bson.D{{Key: "startTime", Value: -1}},
			Options: options.Index(),
		},
		// Room + Start time index
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "startTime", Value: -1},
			},
			Options: options.Index(),
		},
		// DJ + Start time index
		{
			Keys: bson.D{
				{Key: "djId", Value: 1},
				{Key: "startTime", Value: -1},
			},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "startTime", Value: 1}},
			Options: longTTL,
		},
	}

	// User history collection indexes
	userHistoryIndexes := []mongo.IndexModel{
		// User ID index
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index(),
		},
		// Type index
		{
			Keys:    bson.D{{Key: "type", Value: 1}},
			Options: options.Index(),
		},
		// User + Type + Timestamp index
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "type", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "timestamp", Value: 1}},
			Options: longTTL,
		},
	}

	// Room history collection indexes
	roomHistoryIndexes := []mongo.IndexModel{
		// Room ID index
		{
			Keys:    bson.D{{Key: "roomId", Value: 1}},
			Options: options.Index(),
		},
		// Type index
		{
			Keys:    bson.D{{Key: "type", Value: 1}},
			Options: options.Index(),
		},
		// Room + Type + Timestamp index
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "type", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "timestamp", Value: 1}},
			Options: longTTL,
		},
	}

	// DJ history collection indexes
	djHistoryIndexes := []mongo.IndexModel{
		// User ID index
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index(),
		},
		// Room ID index
		{
			Keys:    bson.D{{Key: "roomId", Value: 1}},
			Options: options.Index(),
		},
		// Start time index
		{
			Keys:    bson.D{{Key: "startTime", Value: -1}},
			Options: options.Index(),
		},
		// End time index
		{
			Keys:    bson.D{{Key: "endTime", Value: -1}},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "startTime", Value: 1}},
			Options: longTTL,
		},
	}

	// Session history collection indexes
	sessionHistoryIndexes := []mongo.IndexModel{
		// User ID index
		{
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index(),
		},
		// Start time index
		{
			Keys:    bson.D{{Key: "startTime", Value: -1}},
			Options: options.Index(),
		},
		// IP index
		{
			Keys:    bson.D{{Key: "ip", Value: 1}},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "startTime", Value: 1}},
			Options: longTTL,
		},
	}

	// Moderation history collection indexes
	modHistoryIndexes := []mongo.IndexModel{
		// Room ID index
		{
			Keys:    bson.D{{Key: "roomId", Value: 1}},
			Options: options.Index(),
		},
		// Moderator ID index
		{
			Keys:    bson.D{{Key: "moderatorId", Value: 1}},
			Options: options.Index(),
		},
		// Target user ID index
		{
			Keys:    bson.D{{Key: "targetUserId", Value: 1}},
			Options: options.Index(),
		},
		// Action index
		{
			Keys:    bson.D{{Key: "action", Value: 1}},
			Options: options.Index(),
		},
		// Timestamp index
		{
			Keys:    bson.D{{Key: "timestamp", Value: -1}},
			Options: options.Index(),
		},
		// Room + Action + Timestamp index
		{
			Keys: bson.D{
				{Key: "roomId", Value: 1},
				{Key: "action", Value: 1},
				{Key: "timestamp", Value: -1},
			},
			Options: options.Index(),
		},
		// TTL index
		{
			Keys:    bson.D{{Key: "timestamp", Value: 1}},
			Options: shortTTL,
		},
	}

	// Create all the indexes
	collections := map[string]struct {
		collection *mongo.Collection
		indexes    []mongo.IndexModel
	}{
		HistoryCollection:        {historyCollection, historyIndexes},
		PlayHistoryCollection:    {playHistoryCollection, playHistoryIndexes},
		UserHistoryCollection:    {userHistoryCollection, userHistoryIndexes},
		RoomHistoryCollection:    {roomHistoryCollection, roomHistoryIndexes},
		DJHistoryCollection:      {djHistoryCollection, djHistoryIndexes},
		SessionHistoryCollection: {sessionHistoryCollection, sessionHistoryIndexes},
		ModHistoryCollection:     {modHistoryCollection, modHistoryIndexes},
	}

	for name, data := range collections {
		if err := createIndexes(ctx, data.collection, data.indexes, logger, name); err != nil {
			return err
		}
	}

	return nil
}
