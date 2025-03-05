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
	mediaCollection       = "media"
	playHistoryCollection = "play_history"
)

// MediaRepository defines the interface for media data access operations.
type MediaRepository interface {
	// Media operations
	Create(ctx context.Context, media *models.Media) error
	FindByID(ctx context.Context, id bson.ObjectID) (*models.Media, error)
	FindBySourceID(ctx context.Context, sourceType, sourceID string) (*models.Media, error)
	FindMany(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.Media, error)
	Update(ctx context.Context, media *models.Media) error
	Delete(ctx context.Context, id bson.ObjectID) error

	// Media search operations
	Search(ctx context.Context, query string, sourceType string, skip, limit int) ([]*models.Media, int64, error)
	FindPopular(ctx context.Context, limit int) ([]*models.Media, error)
	FindRecent(ctx context.Context, limit int) ([]*models.Media, error)
	FindByArtist(ctx context.Context, artist string, skip, limit int) ([]*models.Media, error)

	// Media stats operations
	UpdateStats(ctx context.Context, id bson.ObjectID, updates bson.M) error
	RecordPlay(ctx context.Context, playHistory *models.PlayHistory) error
	FindPlayHistory(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.PlayHistory, error)

	// Media vote operations
	RecordVote(ctx context.Context, mediaID, userID bson.ObjectID, roomID bson.ObjectID, voteType string) error
	GetMediaVotes(ctx context.Context, mediaID, roomID bson.ObjectID) (*models.MediaVotes, error)
}

// mediaRepository is the MongoDB implementation of MediaRepository.
type mediaRepository struct {
	mediaCollection       *mongo.Collection
	playHistoryCollection *mongo.Collection
	logger                *utils.Logger
}

// NewMediaRepository creates a new instance of MediaRepository.
func NewMediaRepository(db *mongo.Database, logger *utils.Logger) MediaRepository {
	return &mediaRepository{
		mediaCollection:       db.Collection(mediaCollection),
		playHistoryCollection: db.Collection(playHistoryCollection),
		logger:                logger.Named("media_repository"),
	}
}

// Create creates a new media item.
func (r *mediaRepository) Create(ctx context.Context, media *models.Media) error {
	if media.ID.IsZero() {
		media.ID = bson.NewObjectID()
	}

	now := time.Now()
	media.TimeCreate(now)

	// Initialize stats if empty
	if media.Stats.LastUpdated.IsZero() {
		media.Stats.LastUpdated = now
	}

	// Insert media into database
	_, err := r.mediaCollection.InsertOne(ctx, media)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Already exists, return existing
			existing, findErr := r.FindBySourceID(ctx, media.Type, media.SourceID)
			if findErr == nil {
				*media = *existing
				return nil
			}
			return models.ErrMediaAlreadyExists
		}
		r.logger.Error("Failed to create media", err, "type", media.Type, "sourceId", media.SourceID)
		return models.NewInternalError(err, "Failed to create media")
	}

	return nil
}

// FindByID finds a media item by its ID.
func (r *mediaRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Media, error) {
	var media models.Media

	err := r.mediaCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&media)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrMediaNotFound
		}
		r.logger.Error("Failed to find media by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find media")
	}

	return &media, nil
}

// FindBySourceID finds a media item by its source type and ID.
func (r *mediaRepository) FindBySourceID(ctx context.Context, sourceType, sourceID string) (*models.Media, error) {
	var media models.Media

	err := r.mediaCollection.FindOne(ctx, bson.M{
		"type":     sourceType,
		"sourceId": sourceID,
	}).Decode(&media)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrMediaNotFound
		}
		r.logger.Error("Failed to find media by source", err, "type", sourceType, "sourceId", sourceID)
		return nil, models.NewInternalError(err, "Failed to find media")
	}

	return &media, nil
}

// FindMany finds multiple media items based on query filters.
func (r *mediaRepository) FindMany(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.Media, error) {
	cursor, err := r.mediaCollection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find media items", err, "filter", filter)
		return nil, models.NewInternalError(err, "Failed to find media items")
	}
	defer cursor.Close(ctx)

	var mediaItems []*models.Media
	if err = cursor.All(ctx, &mediaItems); err != nil {
		r.logger.Error("Failed to decode media items", err)
		return nil, models.NewInternalError(err, "Failed to decode media items")
	}

	return mediaItems, nil
}

// Update updates an existing media item.
func (r *mediaRepository) Update(ctx context.Context, media *models.Media) error {
	media.UpdateNow()

	result, err := r.mediaCollection.ReplaceOne(ctx, bson.M{"_id": media.ID}, media)
	if err != nil {
		r.logger.Error("Failed to update media", err, "id", media.ID.Hex())
		return models.NewInternalError(err, "Failed to update media")
	}

	if result.MatchedCount == 0 {
		return models.ErrMediaNotFound
	}

	return nil
}

// Delete deletes a media item by its ID.
func (r *mediaRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	result, err := r.mediaCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		r.logger.Error("Failed to delete media", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to delete media")
	}

	if result.DeletedCount == 0 {
		return models.ErrMediaNotFound
	}

	return nil
}

// Search searches for media items by text query.
func (r *mediaRepository) Search(ctx context.Context, query string, sourceType string, skip, limit int) ([]*models.Media, int64, error) {
	filter := bson.M{}

	// Apply text search
	if query != "" {
		filter["$text"] = bson.M{"$search": query}
	}

	// Filter by source type if specified
	if sourceType != "" && sourceType != "all" {
		filter["type"] = sourceType
	}

	// Count total matches
	total, err := r.mediaCollection.CountDocuments(ctx, filter)
	if err != nil {
		r.logger.Error("Failed to count media items", err, "query", query)
		return nil, 0, models.NewInternalError(err, "Failed to count media items")
	}

	// Set up options with pagination and sort
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	// Set sort order - if text search, sort by relevance, otherwise by play count
	if query != "" {
		opts.SetSort(bson.M{"score": bson.M{"$meta": "textScore"}})
	} else {
		opts.SetSort(bson.M{"stats.playCount": -1})
	}

	// Add projection for text score if using text search
	if query != "" {
		opts.SetProjection(bson.M{"score": bson.M{"$meta": "textScore"}})
	}

	mediaItems, err := r.FindMany(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	return mediaItems, total, nil
}

// FindPopular finds the most popular media items.
func (r *mediaRepository) FindPopular(ctx context.Context, limit int) ([]*models.Media, error) {
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.M{"stats.playCount": -1})

	return r.FindMany(ctx, bson.M{}, opts)
}

// FindRecent finds recently added media items.
func (r *mediaRepository) FindRecent(ctx context.Context, limit int) ([]*models.Media, error) {
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.M{"createdAt": -1})

	return r.FindMany(ctx, bson.M{}, opts)
}

// FindByArtist finds media items by artist name.
func (r *mediaRepository) FindByArtist(ctx context.Context, artist string, skip, limit int) ([]*models.Media, error) {
	// Case-insensitive search on artist field
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"stats.playCount": -1}).
		SetCollation(&options.Collation{
			Locale:    "en",
			Strength:  2, // Case-insensitive
			CaseLevel: false,
		})

	return r.FindMany(ctx, bson.M{"artist": artist}, opts)
}

// UpdateStats updates the statistics of a media item.
func (r *mediaRepository) UpdateStats(ctx context.Context, id bson.ObjectID, updates bson.M) error {
	now := time.Now()

	// Prepare update with $inc for stats and timestamp updates
	updateDoc := bson.D{
		cmdSet(bson.M{
			"stats.lastUpdated": now,
			"updatedAt":         now,
		}),
	}

	// Add stat increments
	incs := bson.M{}
	for field, value := range updates {
		incs["stats."+field] = value
	}

	if len(incs) > 0 {
		updateDoc = append(updateDoc, cmdInc(incs))
	}

	result, err := r.mediaCollection.UpdateByID(ctx, id, updateDoc)
	if err != nil {
		r.logger.Error("Failed to update media stats", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to update media stats")
	}

	if result.MatchedCount == 0 {
		return models.ErrMediaNotFound
	}

	return nil
}

// RecordPlay records a play history event.
func (r *mediaRepository) RecordPlay(ctx context.Context, playHistory *models.PlayHistory) error {
	if playHistory.ID.IsZero() {
		playHistory.ID = bson.NewObjectID()
	}

	// Insert play history
	_, err := r.playHistoryCollection.InsertOne(ctx, playHistory)
	if err != nil {
		r.logger.Error("Failed to record play history", err, "mediaId", playHistory.MediaID.Hex())
		return models.NewInternalError(err, "Failed to record play history")
	}

	// Update media stats
	err = r.UpdateStats(ctx, playHistory.MediaID, bson.M{
		"playCount": 1,
		"wootCount": playHistory.Votes.Woots,
		"mehCount":  playHistory.Votes.Mehs,
		"grabCount": playHistory.Votes.Grabs,
	})

	if err != nil {
		r.logger.Error("Failed to update media stats after play", err, "mediaId", playHistory.MediaID.Hex())
		// Continue anyway, the play history was recorded
	}

	// Update LastPlayed timestamp
	_, err = r.mediaCollection.UpdateByID(
		ctx,
		playHistory.MediaID,
		bson.D{
			cmdSet(bson.M{
				"stats.lastPlayed": playHistory.EndTime,
				"updatedAt":        time.Now(),
			}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to update LastPlayed timestamp", err, "mediaId", playHistory.MediaID.Hex())
		// Continue anyway, the play history was recorded
	}

	return nil
}

// FindPlayHistory finds play history records based on filters.
func (r *mediaRepository) FindPlayHistory(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.PlayHistory, error) {
	cursor, err := r.playHistoryCollection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find play history", err, "filter", filter)
		return nil, models.NewInternalError(err, "Failed to find play history")
	}
	defer cursor.Close(ctx)

	var playHistory []*models.PlayHistory
	if err = cursor.All(ctx, &playHistory); err != nil {
		r.logger.Error("Failed to decode play history", err)
		return nil, models.NewInternalError(err, "Failed to decode play history")
	}

	return playHistory, nil
}

// RecordVote records a vote for a media item.
func (r *mediaRepository) RecordVote(ctx context.Context, mediaID, userID bson.ObjectID, roomID bson.ObjectID, voteType string) error {
	// Find the latest play history record for this media in this room
	filter := bson.M{
		"mediaId": mediaID,
		"roomId":  roomID,
	}

	opts := options.FindOne().SetSort(bson.M{"startTime": -1})

	var playHistory models.PlayHistory
	err := r.playHistoryCollection.FindOne(ctx, filter, opts).Decode(&playHistory)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.NewMediaError(errors.New("no active play for this media"), "No active play found for this media", 404)
		}
		r.logger.Error("Failed to find play history for vote", err, "mediaId", mediaID.Hex(), "roomId", roomID.Hex())
		return models.NewInternalError(err, "Failed to find play history")
	}

	// Convert userID to string for map key
	userIDStr := userID.Hex()

	// Initialize voters map if nil
	if playHistory.Votes.Voters == nil {
		playHistory.Votes.Voters = make(map[string]string)
	}

	// Check if user has already voted
	previousVote, hasVoted := playHistory.Votes.Voters[userIDStr]

	// Update vote counts
	if hasVoted {
		// Remove previous vote count
		switch previousVote {
		case "woot":
			playHistory.Votes.Woots--
		case "meh":
			playHistory.Votes.Mehs--
		case "grab":
			playHistory.Votes.Grabs--
		}
	}

	// Add new vote
	playHistory.Votes.Voters[userIDStr] = voteType
	switch voteType {
	case "woot":
		playHistory.Votes.Woots++
	case "meh":
		playHistory.Votes.Mehs++
	case "grab":
		playHistory.Votes.Grabs++
	}

	// Update play history with new vote counts
	_, err = r.playHistoryCollection.UpdateByID(
		ctx,
		playHistory.ID,
		bson.D{
			cmdSet(bson.M{
				"votes": playHistory.Votes,
			}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to update play history vote", err, "historyId", playHistory.ID.Hex())
		return models.NewInternalError(err, "Failed to record vote")
	}

	// Update media stats
	updates := bson.M{}
	if !hasVoted {
		// Only update if this is a new vote, not a vote change
		switch voteType {
		case "woot":
			updates["wootCount"] = 1
		case "meh":
			updates["mehCount"] = 1
		case "grab":
			updates["grabCount"] = 1
		}
	} else if previousVote != voteType {
		// For vote changes, we need to update both stats
		switch voteType {
		case "woot":
			updates["wootCount"] = 1
		case "meh":
			updates["mehCount"] = 1
		case "grab":
			updates["grabCount"] = 1
		}

		// Decrement the previous vote type
		switch previousVote {
		case "woot":
			updates["wootCount"] = -1
		case "meh":
			updates["mehCount"] = -1
		case "grab":
			updates["grabCount"] = -1
		}
	}

	// Only update stats if there are actual changes
	if len(updates) > 0 {
		err = r.UpdateStats(ctx, mediaID, updates)
		if err != nil {
			r.logger.Error("Failed to update media stats after vote", err, "mediaId", mediaID.Hex())
			// Continue anyway, the vote was recorded
		}
	}

	return nil
}

// GetMediaVotes gets the current votes for a media item in a room.
func (r *mediaRepository) GetMediaVotes(ctx context.Context, mediaID, roomID bson.ObjectID) (*models.MediaVotes, error) {
	// Find the latest play history record for this media in this room
	filter := bson.M{
		"mediaId": mediaID,
		"roomId":  roomID,
	}

	opts := options.FindOne().SetSort(bson.M{"startTime": -1})

	var playHistory models.PlayHistory
	err := r.playHistoryCollection.FindOne(ctx, filter, opts).Decode(&playHistory)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.NewMediaError(errors.New("no active play for this media"), "No active play found for this media", 404)
		}
		r.logger.Error("Failed to find play history for votes", err, "mediaId", mediaID.Hex(), "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to find play history")
	}

	return &playHistory.Votes, nil
}
