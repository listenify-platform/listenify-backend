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

	"slices"

	lom "github.com/samber/lo/mutable"
)

// Collection name
const (
	playlistCollection = "playlists"
)

// PlaylistRepository defines the interface for playlist data access operations.
type PlaylistRepository interface {
	// Playlist operations
	Create(ctx context.Context, playlist *models.Playlist) error
	FindByID(ctx context.Context, id bson.ObjectID) (*models.Playlist, error)
	FindMany(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.Playlist, error)
	Update(ctx context.Context, playlist *models.Playlist) error
	Delete(ctx context.Context, id bson.ObjectID) error

	// User playlist operations
	FindUserPlaylists(ctx context.Context, userID bson.ObjectID) ([]*models.Playlist, error)
	GetActivePlaylist(ctx context.Context, userID bson.ObjectID) (*models.Playlist, error)
	SetActivePlaylist(ctx context.Context, userID, playlistID bson.ObjectID) error
	CountUserPlaylists(ctx context.Context, userID bson.ObjectID) (int64, error)

	// Playlist item operations
	AddItem(ctx context.Context, playlistID, mediaID bson.ObjectID, position int) error
	RemoveItem(ctx context.Context, playlistID, itemID bson.ObjectID) error
	MoveItem(ctx context.Context, playlistID, itemID bson.ObjectID, newPosition int) error
	ShufflePlaylist(ctx context.Context, playlistID bson.ObjectID) error

	// Playlist search
	SearchPlaylists(ctx context.Context, criteria models.PlaylistSearchCriteria) ([]*models.Playlist, int64, error)
	FindPublicPlaylists(ctx context.Context, skip, limit int) ([]*models.Playlist, error)

	// Playlist stats
	RecordPlaylistPlay(ctx context.Context, playlistID, mediaID bson.ObjectID) error
	UpdatePlaylistStats(ctx context.Context, playlistID bson.ObjectID) error
}

// playlistRepository is the MongoDB implementation of PlaylistRepository.
type playlistRepository struct {
	collection *mongo.Collection
	logger     *utils.Logger
}

// NewPlaylistRepository creates a new instance of PlaylistRepository.
func NewPlaylistRepository(db *mongo.Database, logger *utils.Logger) PlaylistRepository {
	return &playlistRepository{
		collection: db.Collection(playlistCollection),
		logger:     logger.Named("playlist_repository"),
	}
}

// Create creates a new playlist.
func (r *playlistRepository) Create(ctx context.Context, playlist *models.Playlist) error {
	if playlist.ID.IsZero() {
		playlist.ID = bson.NewObjectID()
	}

	now := time.Now()
	playlist.TimeCreate(now)

	// Initialize items and stats if empty
	if playlist.Items == nil {
		playlist.Items = []models.PlaylistItem{}
	}

	if playlist.Stats.LastCalculated.IsZero() {
		playlist.Stats.LastCalculated = now
	}

	// Insert playlist into database
	_, err := r.collection.InsertOne(ctx, playlist)
	if err != nil {
		r.logger.Error("Failed to create playlist", err, "userId", playlist.Owner.Hex(), "name", playlist.Name)
		return models.NewInternalError(err, "Failed to create playlist")
	}

	// If this is set as active, deactivate all other playlists for this user
	if playlist.IsActive {
		err = r.deactivateOtherPlaylists(ctx, playlist.Owner, playlist.ID)
		if err != nil {
			r.logger.Error("Failed to deactivate other playlists", err, "userId", playlist.Owner.Hex())
			// Continue anyway, the playlist was created
		}
	}

	return nil
}

// deactivateOtherPlaylists deactivates all other playlists for a user.
func (r *playlistRepository) deactivateOtherPlaylists(ctx context.Context, userID, activePlaylistID bson.ObjectID) error {
	filter := bson.M{
		"owner":    userID,
		"_id":      bson.M{"$ne": activePlaylistID},
		"isActive": true,
	}

	update := bson.D{
		cmdSet(bson.M{
			"isActive":  false,
			"updatedAt": time.Now(),
		}),
	}

	_, err := r.collection.UpdateMany(ctx, filter, update)
	return err
}

// FindByID finds a playlist by its ID.
func (r *playlistRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Playlist, error) {
	var playlist models.Playlist

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&playlist)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrPlaylistNotFound
		}
		r.logger.Error("Failed to find playlist by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find playlist")
	}

	return &playlist, nil
}

// FindMany finds multiple playlists based on query filters.
func (r *playlistRepository) FindMany(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.Playlist, error) {
	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find playlists", err, "filter", filter)
		return nil, models.NewInternalError(err, "Failed to find playlists")
	}
	defer cursor.Close(ctx)

	var playlists []*models.Playlist
	if err = cursor.All(ctx, &playlists); err != nil {
		r.logger.Error("Failed to decode playlists", err)
		return nil, models.NewInternalError(err, "Failed to decode playlists")
	}

	return playlists, nil
}

// Update updates an existing playlist.
func (r *playlistRepository) Update(ctx context.Context, playlist *models.Playlist) error {
	wasActive := playlist.IsActive
	playlist.UpdateNow()

	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": playlist.ID}, playlist)
	if err != nil {
		r.logger.Error("Failed to update playlist", err, "id", playlist.ID.Hex())
		return models.NewInternalError(err, "Failed to update playlist")
	}

	if result.MatchedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	// If playlist is now active, deactivate all other playlists for this user
	if playlist.IsActive && !wasActive {
		err = r.deactivateOtherPlaylists(ctx, playlist.Owner, playlist.ID)
		if err != nil {
			r.logger.Error("Failed to deactivate other playlists", err, "userId", playlist.Owner.Hex())
			// Continue anyway, the playlist was updated
		}
	}

	return nil
}

// Delete deletes a playlist by its ID.
func (r *playlistRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	// Get the playlist first to check if it's active
	playlist, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		r.logger.Error("Failed to delete playlist", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to delete playlist")
	}

	if result.DeletedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	// If this was the active playlist, try to set another playlist as active
	if playlist.IsActive {
		// Find another playlist from this user
		filter := bson.M{"owner": playlist.Owner}
		opts := options.FindOne().SetSort(bson.M{"updatedAt": -1})

		var nextPlaylist models.Playlist
		err = r.collection.FindOne(ctx, filter, opts).Decode(&nextPlaylist)
		if err == nil {
			// Set this playlist as active
			_, err = r.collection.UpdateByID(
				ctx,
				nextPlaylist.ID,
				bson.D{
					cmdSet(bson.M{
						"isActive":  true,
						"updatedAt": time.Now(),
					}),
				},
			)

			if err != nil {
				r.logger.Error("Failed to set new active playlist", err, "id", nextPlaylist.ID.Hex())
				// Continue anyway, the playlist was deleted
			}
		}
	}

	return nil
}

// FindUserPlaylists finds all playlists for a specific user.
func (r *playlistRepository) FindUserPlaylists(ctx context.Context, userID bson.ObjectID) ([]*models.Playlist, error) {
	opts := options.Find().SetSort(bson.M{"isActive": -1, "updatedAt": -1})

	return r.FindMany(ctx, bson.M{"owner": userID}, opts)
}

// GetActivePlaylist gets the user's active playlist.
func (r *playlistRepository) GetActivePlaylist(ctx context.Context, userID bson.ObjectID) (*models.Playlist, error) {
	var playlist models.Playlist

	err := r.collection.FindOne(ctx, bson.M{
		"owner":    userID,
		"isActive": true,
	}).Decode(&playlist)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// Try to find any playlist and set it as active
			opts := options.FindOne().SetSort(bson.M{"updatedAt": -1})

			err = r.collection.FindOne(ctx, bson.M{"owner": userID}, opts).Decode(&playlist)
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, models.ErrPlaylistNotFound
				}
				r.logger.Error("Failed to find user playlist", err, "userId", userID.Hex())
				return nil, models.NewInternalError(err, "Failed to find user playlist")
			}

			// Set this playlist as active
			playlist.IsActive = true
			playlist.UpdateNow()

			_, err = r.collection.UpdateByID(
				ctx,
				playlist.ID,
				bson.D{
					cmdSet(bson.M{
						"isActive":  true,
						"updatedAt": playlist.UpdatedAt,
					}),
				},
			)

			if err != nil {
				r.logger.Error("Failed to set active playlist", err, "id", playlist.ID.Hex())
				// Continue anyway, just return the playlist
			}
		} else {
			r.logger.Error("Failed to find active playlist", err, "userId", userID.Hex())
			return nil, models.NewInternalError(err, "Failed to find active playlist")
		}
	}

	return &playlist, nil
}

// SetActivePlaylist sets a playlist as the user's active playlist.
func (r *playlistRepository) SetActivePlaylist(ctx context.Context, userID, playlistID bson.ObjectID) error {
	// Check if playlist exists and belongs to user
	var playlist models.Playlist
	err := r.collection.FindOne(ctx, bson.M{
		"_id":   playlistID,
		"owner": userID,
	}).Decode(&playlist)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.ErrPlaylistNotFound
		}
		r.logger.Error("Failed to find playlist for activation", err, "id", playlistID.Hex())
		return models.NewInternalError(err, "Failed to find playlist")
	}

	// Deactivate all other playlists
	err = r.deactivateOtherPlaylists(ctx, userID, playlistID)
	if err != nil {
		r.logger.Error("Failed to deactivate other playlists", err, "userId", userID.Hex())
		// Continue anyway, still set this one as active
	}

	// Set this playlist as active
	now := time.Now()
	_, err = r.collection.UpdateByID(
		ctx,
		playlistID,
		bson.D{
			cmdSet(bson.M{
				"isActive":  true,
				"updatedAt": now,
			}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to set active playlist", err, "id", playlistID.Hex())
		return models.NewInternalError(err, "Failed to set active playlist")
	}

	return nil
}

// CountUserPlaylists counts the number of playlists owned by a user.
func (r *playlistRepository) CountUserPlaylists(ctx context.Context, userID bson.ObjectID) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{"owner": userID})
	if err != nil {
		r.logger.Error("Failed to count user playlists", err, "userId", userID.Hex())
		return 0, models.NewInternalError(err, "Failed to count playlists")
	}

	return count, nil
}

// AddItem adds a media item to a playlist.
func (r *playlistRepository) AddItem(ctx context.Context, playlistID, mediaID bson.ObjectID, position int) error {
	// Get current playlist to validate and get current item positions
	playlist, err := r.FindByID(ctx, playlistID)
	if err != nil {
		return err
	}

	// Create new item
	now := time.Now()
	newItem := models.PlaylistItem{
		ID:         bson.NewObjectID(),
		MediaID:    mediaID,
		AddedAt:    now,
		PlayCount:  0,
		LastPlayed: time.Time{},
	}

	// Determine position for new item
	itemCount := len(playlist.Items)

	if position < 0 || position > itemCount {
		position = itemCount // Append to end if position is invalid
	}

	// Update order of existing items
	for i := range itemCount {
		if i < position {
			playlist.Items[i].Order = i
		} else {
			playlist.Items[i].Order = i + 1
		}
	}

	// Set order for new item
	newItem.Order = position

	// Insert new item at specified position
	if position == itemCount {
		playlist.Items = append(playlist.Items, newItem)
	} else {
		// Create space for new item
		playlist.Items = append(playlist.Items, models.PlaylistItem{})
		// Shift items
		copy(playlist.Items[position+1:], playlist.Items[position:])
		// Insert new item
		playlist.Items[position] = newItem
	}

	// Update playlist stats
	playlist.Stats.TotalItems = len(playlist.Items)
	playlist.UpdateNow()

	// Calculate total duration (if we had media duration info)
	// This would require a media lookup, which we'll skip for now

	// Update the playlist
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": playlistID}, playlist)
	if err != nil {
		r.logger.Error("Failed to add item to playlist", err, "playlistId", playlistID.Hex(), "mediaId", mediaID.Hex())
		return models.NewInternalError(err, "Failed to add item to playlist")
	}

	if result.MatchedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	return nil
}

// RemoveItem removes an item from a playlist.
func (r *playlistRepository) RemoveItem(ctx context.Context, playlistID, itemID bson.ObjectID) error {
	// Get current playlist
	playlist, err := r.FindByID(ctx, playlistID)
	if err != nil {
		return err
	}

	// Find the item to remove
	itemIndex := -1
	for i, item := range playlist.Items {
		if item.ID == itemID {
			itemIndex = i
			break
		}
	}

	if itemIndex == -1 {
		return models.ErrPlaylistItemNotFound
	}

	// Remove the item
	playlist.Items = slices.Delete(playlist.Items, itemIndex, itemIndex+1)

	// Re-number the items
	for i := range playlist.Items {
		playlist.Items[i].Order = i
	}

	// Update playlist stats
	playlist.Stats.TotalItems = len(playlist.Items)
	playlist.UpdateNow()

	// Update the playlist
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": playlistID}, playlist)
	if err != nil {
		r.logger.Error("Failed to remove item from playlist", err, "playlistId", playlistID.Hex(), "itemId", itemID.Hex())
		return models.NewInternalError(err, "Failed to remove item from playlist")
	}

	if result.MatchedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	return nil
}

// MoveItem moves an item to a new position in a playlist.
func (r *playlistRepository) MoveItem(ctx context.Context, playlistID, itemID bson.ObjectID, newPosition int) error {
	// Get current playlist
	playlist, err := r.FindByID(ctx, playlistID)
	if err != nil {
		return err
	}

	// Find the item to move
	itemIndex := -1
	for i, item := range playlist.Items {
		if item.ID == itemID {
			itemIndex = i
			break
		}
	}

	if itemIndex == -1 {
		return models.ErrPlaylistItemNotFound
	}

	// Validate new position
	itemCount := len(playlist.Items)
	if newPosition < 0 || newPosition >= itemCount {
		newPosition = itemCount - 1 // Move to end if position is invalid
	}

	// Skip if position is the same
	if itemIndex == newPosition {
		return nil
	}

	// Store the item to move
	itemToMove := playlist.Items[itemIndex]

	// Remove the item from its current position
	playlist.Items = slices.Delete(playlist.Items, itemIndex, itemIndex+1)

	// Insert the item at the new position
	playlist.Items = append(playlist.Items[:newPosition], append([]models.PlaylistItem{itemToMove}, playlist.Items[newPosition:]...)...)

	// Re-number the items
	for i := range playlist.Items {
		playlist.Items[i].Order = i
	}

	playlist.UpdateNow()

	// Update the playlist
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": playlistID}, playlist)
	if err != nil {
		r.logger.Error("Failed to move item in playlist", err, "playlistId", playlistID.Hex(), "itemId", itemID.Hex())
		return models.NewInternalError(err, "Failed to move item")
	}

	if result.MatchedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	return nil
}

// ShufflePlaylist randomizes the order of items in a playlist.
func (r *playlistRepository) ShufflePlaylist(ctx context.Context, playlistID bson.ObjectID) error {
	// Get current playlist
	playlist, err := r.FindByID(ctx, playlistID)
	if err != nil {
		return err
	}

	// Shuffle the items
	lom.Shuffle(playlist.Items)

	// Re-number the items
	for i := range playlist.Items {
		playlist.Items[i].Order = i
	}

	playlist.UpdateNow()

	// Update the playlist
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": playlistID}, playlist)
	if err != nil {
		r.logger.Error("Failed to shuffle playlist", err, "playlistId", playlistID.Hex())
		return models.NewInternalError(err, "Failed to shuffle playlist")
	}

	if result.MatchedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	return nil
}

// SearchPlaylists searches for playlists based on criteria.
func (r *playlistRepository) SearchPlaylists(ctx context.Context, criteria models.PlaylistSearchCriteria) ([]*models.Playlist, int64, error) {
	filter := bson.M{}

	// Apply privacy filter
	if !criteria.IncludePrivate {
		filter["isPrivate"] = false
	}

	// Apply owner filter if specified
	if !criteria.OwnerID.IsZero() {
		filter["owner"] = criteria.OwnerID
	}

	// Apply tag filter
	if len(criteria.Tags) > 0 {
		filter["tags"] = bson.M{"$all": criteria.Tags}
	}

	// Apply text search
	if criteria.Query != "" {
		filter["$text"] = bson.M{"$search": criteria.Query}
	}

	// Count total matches
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		r.logger.Error("Failed to count playlists", err, "filter", filter)
		return nil, 0, models.NewInternalError(err, "Failed to count playlists")
	}

	// Set up pagination
	if criteria.Page < 1 {
		criteria.Page = 1
	}
	if criteria.Limit < 1 || criteria.Limit > 100 {
		criteria.Limit = 20
	}

	skip := (criteria.Page - 1) * criteria.Limit

	// Set up sort
	sort := bson.M{}
	if criteria.Query != "" && filter["$text"] != nil {
		// If text search, sort by text score first
		sort["score"] = bson.M{"$meta": "textScore"}
	}

	// Apply additional sort criteria
	switch criteria.SortBy {
	case "name":
		sort["name"] = 1
	case "created":
		sort["createdAt"] = -1
	case "updated":
		sort["updatedAt"] = -1
	case "items":
		sort["stats.totalItems"] = -1
	case "plays":
		sort["stats.totalPlays"] = -1
	case "followers":
		sort["stats.followers"] = -1
	default:
		// Default sort by activity
		if len(sort) == 0 {
			sort["updatedAt"] = -1
		}
	}

	// Apply sort direction
	if criteria.SortDirection == "asc" && criteria.SortBy != "" {
		// Reverse the sort direction for the specified field
		for k, v := range sort {
			if k != "score" { // Don't reverse text score sort
				if v == 1 {
					sort[k] = -1
				} else {
					sort[k] = 1
				}
			}
		}
	}

	// Find playlists
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(criteria.Limit)).
		SetSort(sort)

	// If text search, add projections
	if criteria.Query != "" && filter["$text"] != nil {
		opts.SetProjection(bson.M{"score": bson.M{"$meta": "textScore"}})
	}

	playlists, err := r.FindMany(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	return playlists, total, nil
}

// FindPublicPlaylists finds public playlists.
func (r *playlistRepository) FindPublicPlaylists(ctx context.Context, skip, limit int) ([]*models.Playlist, error) {
	filter := bson.M{
		"isPrivate": false,
	}

	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"stats.followers": -1, "updatedAt": -1})

	return r.FindMany(ctx, filter, opts)
}

// RecordPlaylistPlay records a play from a playlist.
func (r *playlistRepository) RecordPlaylistPlay(ctx context.Context, playlistID, mediaID bson.ObjectID) error {
	now := time.Now()

	// Update playlist stats
	update := bson.D{
		cmdInc(bson.M{
			"stats.totalPlays": 1,
		}),
		cmdSet(bson.M{
			"lastPlayed": now,
			"updatedAt":  now,
		}),
	}

	result, err := r.collection.UpdateByID(ctx, playlistID, update)
	if err != nil {
		r.logger.Error("Failed to record playlist play", err, "playlistId", playlistID.Hex())
		return models.NewInternalError(err, "Failed to record playlist play")
	}

	if result.MatchedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	// Update play count and last played for the specific item
	playlist, err := r.FindByID(ctx, playlistID)
	if err != nil {
		return err
	}

	// Find the played item
	for i, item := range playlist.Items {
		if item.MediaID == mediaID {
			// Update play count and last played time
			playlist.Items[i].PlayCount++
			playlist.Items[i].LastPlayed = now
			break
		}
	}

	// Update the playlist
	_, err = r.collection.ReplaceOne(ctx, bson.M{"_id": playlistID}, playlist)
	if err != nil {
		r.logger.Error("Failed to update playlist item play count", err, "playlistId", playlistID.Hex(), "mediaId", mediaID.Hex())
		// Continue anyway, the playlist stats were updated
	}

	return nil
}

// UpdatePlaylistStats recalculates and updates a playlist's statistics.
func (r *playlistRepository) UpdatePlaylistStats(ctx context.Context, playlistID bson.ObjectID) error {
	// Get current playlist
	playlist, err := r.FindByID(ctx, playlistID)
	if err != nil {
		return err
	}

	// Update stats
	playlist.Stats.TotalItems = len(playlist.Items)
	playlist.Stats.LastCalculated = time.Now()

	// Calculate average plays
	totalPlays := 0
	for _, item := range playlist.Items {
		totalPlays += item.PlayCount
	}

	if len(playlist.Items) > 0 {
		// Update derived stats
		playlist.Stats.TotalPlays = totalPlays
	}

	// Update the playlist
	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": playlistID}, playlist)
	if err != nil {
		r.logger.Error("Failed to update playlist stats", err, "playlistId", playlistID.Hex())
		return models.NewInternalError(err, "Failed to update playlist stats")
	}

	if result.MatchedCount == 0 {
		return models.ErrPlaylistNotFound
	}

	return nil
}
