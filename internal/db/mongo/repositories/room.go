// Package repositories contains MongoDB repository implementations.
package repositories

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// Collection names
const (
	roomCollection      = "rooms"
	roomUsersCollection = "room_users"
)

// RoomRepository defines the interface for room data access operations.
type RoomRepository interface {
	// Room operations
	Create(ctx context.Context, room *models.Room) error
	FindByID(ctx context.Context, id bson.ObjectID) (*models.Room, error)
	FindBySlug(ctx context.Context, slug string) (*models.Room, error)
	FindMany(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.Room, error)
	Update(ctx context.Context, room *models.Room) error
	Delete(ctx context.Context, id bson.ObjectID) error
	CountRooms(ctx context.Context, filter bson.M) (int64, error)

	// Room status operations
	SetActive(ctx context.Context, id bson.ObjectID, active bool) error
	UpdateLastActivity(ctx context.Context, id bson.ObjectID) error

	// Room user operations
	AddUserToRoom(ctx context.Context, roomUser *models.RoomUser) error
	RemoveUserFromRoom(ctx context.Context, roomID, userID bson.ObjectID) error
	FindRoomUsers(ctx context.Context, roomID bson.ObjectID) ([]*models.RoomUser, error)
	FindUserRoom(ctx context.Context, userID bson.ObjectID) (*models.RoomUser, error)
	UpdateRoomUser(ctx context.Context, roomUser *models.RoomUser) error

	// DJ queue operations
	UpdateDJQueue(ctx context.Context, roomID bson.ObjectID, queueEntries []models.QueueEntry) error
	SetCurrentDJ(ctx context.Context, roomID, userID bson.ObjectID) error
	SetCurrentMedia(ctx context.Context, roomID, mediaID bson.ObjectID) error

	// Moderation operations
	AddModerator(ctx context.Context, roomID, userID bson.ObjectID) error
	RemoveModerator(ctx context.Context, roomID, userID bson.ObjectID) error
	BanUser(ctx context.Context, roomID, userID bson.ObjectID) error
	UnbanUser(ctx context.Context, roomID, userID bson.ObjectID) error
	IsUserBanned(ctx context.Context, roomID, userID bson.ObjectID) (bool, error)

	// Room search and discovery
	SearchRooms(ctx context.Context, criteria models.RoomSearchCriteria) ([]*models.Room, int64, error)
	FindPopularRooms(ctx context.Context, limit int) ([]*models.Room, error)
	FindRecentRooms(ctx context.Context, limit int) ([]*models.Room, error)
}

// roomRepository is the MongoDB implementation of RoomRepository.
type roomRepository struct {
	roomCollection      *mongo.Collection
	roomUsersCollection *mongo.Collection
	logger              *utils.Logger
}

// NewRoomRepository creates a new instance of RoomRepository.
func NewRoomRepository(db *mongo.Database, logger *utils.Logger) RoomRepository {
	return &roomRepository{
		roomCollection:      db.Collection(roomCollection),
		roomUsersCollection: db.Collection(roomUsersCollection),
		logger:              logger.Named("room_repository"),
	}
}

// Create creates a new room.
func (r *roomRepository) Create(ctx context.Context, room *models.Room) error {
	if room.ID.IsZero() {
		room.ID = bson.NewObjectID()
	}

	// Generate a slug if not provided
	if room.Slug == "" {
		room.Slug = utils.SlugifyString(room.Name)
	}

	// Set default values
	now := time.Now()
	room.TimeCreate(now)
	room.LastActivity = now

	// Initialize stats if empty
	if room.Stats.LastStatsReset.IsZero() {
		room.Stats.LastStatsReset = now
	}

	// Insert room into database
	_, err := r.roomCollection.InsertOne(ctx, room)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			if strings.Contains(err.Error(), "slug") {
				// Try with a different slug
				randomString, err := utils.GenerateRandomString(4)
				if err != nil {
					r.logger.Error("Failed to generate random string", err)
					return models.NewInternalError(err, "Failed to generate slug")
				}
				room.Slug = utils.SlugifyString(room.Name) + "-" + randomString
				return r.Create(ctx, room)
			}
			return models.ErrRoomAlreadyExists
		}
		r.logger.Error("Failed to create room", err, "name", room.Name)
		return models.NewInternalError(err, "Failed to create room")
	}

	return nil
}

// FindByID finds a room by its ID.
func (r *roomRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.Room, error) {
	var room models.Room

	err := r.roomCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&room)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrRoomNotFound
		}
		r.logger.Error("Failed to find room by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find room")
	}

	return &room, nil
}

// FindBySlug finds a room by its slug.
func (r *roomRepository) FindBySlug(ctx context.Context, slug string) (*models.Room, error) {
	var room models.Room

	// Case-insensitive search
	opts := options.FindOne().SetCollation(&options.Collation{
		Locale:    "en",
		Strength:  2, // Case-insensitive
		CaseLevel: false,
	})

	err := r.roomCollection.FindOne(ctx, bson.M{"slug": slug}, opts).Decode(&room)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrRoomNotFound
		}
		r.logger.Error("Failed to find room by slug", err, "slug", slug)
		return nil, models.NewInternalError(err, "Failed to find room")
	}

	return &room, nil
}

// FindMany finds multiple rooms based on query filters.
func (r *roomRepository) FindMany(ctx context.Context, filter bson.M, opts options.Lister[options.FindOptions]) ([]*models.Room, error) {
	cursor, err := r.roomCollection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find rooms", err, "filter", filter)
		return nil, models.NewInternalError(err, "Failed to find rooms")
	}
	defer cursor.Close(ctx)

	var rooms []*models.Room
	if err = cursor.All(ctx, &rooms); err != nil {
		r.logger.Error("Failed to decode rooms", err)
		return nil, models.NewInternalError(err, "Failed to decode rooms")
	}

	return rooms, nil
}

// Update updates an existing room.
func (r *roomRepository) Update(ctx context.Context, room *models.Room) error {
	room.UpdateNow()

	result, err := r.roomCollection.ReplaceOne(ctx, bson.M{"_id": room.ID}, room)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			if strings.Contains(err.Error(), "slug") {
				return models.NewRoomError(err, "Room slug already exists", 409)
			}
			return models.ErrRoomAlreadyExists
		}
		r.logger.Error("Failed to update room", err, "id", room.ID.Hex())
		return models.NewInternalError(err, "Failed to update room")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	return nil
}

// Delete deletes a room by its ID.
func (r *roomRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	// Delete the room
	result, err := r.roomCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		r.logger.Error("Failed to delete room", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to delete room")
	}

	if result.DeletedCount == 0 {
		return models.ErrRoomNotFound
	}

	// Delete all room users
	_, err = r.roomUsersCollection.DeleteMany(ctx, bson.M{"roomId": id})
	if err != nil {
		r.logger.Error("Failed to delete room users", err, "roomId", id.Hex())
		// Continue anyway, the room was already deleted
	}

	return nil
}

// CountRooms counts the number of rooms that match the given filter.
func (r *roomRepository) CountRooms(ctx context.Context, filter bson.M) (int64, error) {
	count, err := r.roomCollection.CountDocuments(ctx, filter)
	if err != nil {
		r.logger.Error("Failed to count rooms", err, "filter", filter)
		return 0, models.NewInternalError(err, "Failed to count rooms")
	}

	return count, nil
}

// SetActive sets a room's active status.
func (r *roomRepository) SetActive(ctx context.Context, id bson.ObjectID, active bool) error {
	now := time.Now()
	update := bson.D{
		cmdSet(bson.M{
			"isActive":     active,
			"updatedAt":    now,
			"lastActivity": now,
		}),
	}

	result, err := r.roomCollection.UpdateByID(ctx, id, update)
	if err != nil {
		r.logger.Error("Failed to set room active status", err, "id", id.Hex(), "active", active)
		return models.NewInternalError(err, "Failed to set room active status")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	return nil
}

// UpdateLastActivity updates a room's last activity time.
func (r *roomRepository) UpdateLastActivity(ctx context.Context, id bson.ObjectID) error {
	now := time.Now()
	update := bson.D{
		cmdSet(bson.M{
			"lastActivity": now,
			"updatedAt":    now,
		}),
	}

	result, err := r.roomCollection.UpdateByID(ctx, id, update)
	if err != nil {
		r.logger.Error("Failed to update room last activity", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to update room last activity")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	return nil
}

// AddUserToRoom adds a user to a room.
func (r *roomRepository) AddUserToRoom(ctx context.Context, roomUser *models.RoomUser) error {
	if roomUser.ID.IsZero() {
		roomUser.ID = bson.NewObjectID()
	}

	now := time.Now()
	roomUser.JoinedAt = now
	roomUser.LastActive = now

	// Check if room exists
	room, err := r.FindByID(ctx, roomUser.RoomID)
	if err != nil {
		return err
	}

	// Check if room is active
	if !room.IsActive {
		return models.ErrRoomInactive
	}

	// Check if user is banned
	isBanned, err := r.IsUserBanned(ctx, roomUser.RoomID, roomUser.UserID)
	if err != nil {
		return err
	}
	if isBanned {
		return models.ErrUserBanned
	}

	// Check if room is at capacity
	count, err := r.roomUsersCollection.CountDocuments(ctx, bson.M{"roomId": roomUser.RoomID})
	if err != nil {
		r.logger.Error("Failed to count room users", err, "roomId", roomUser.RoomID.Hex())
		return models.NewInternalError(err, "Failed to count room users")
	}

	if count >= int64(room.Settings.Capacity) {
		return models.ErrRoomFull
	}

	// Check if user is already in the room
	var existingUser models.RoomUser
	err = r.roomUsersCollection.FindOne(ctx, roomAndUserIDs(roomUser.RoomID, roomUser.UserID)).Decode(&existingUser)

	if err == nil {
		// User already in room, update last active time
		update := bson.D{
			cmdSet(bson.M{
				"lastActive": now,
			}),
		}
		_, err = r.roomUsersCollection.UpdateOne(ctx, roomAndUserIDs(roomUser.RoomID, roomUser.UserID), update)

		if err != nil {
			r.logger.Error("Failed to update room user", err, "roomId", roomUser.RoomID.Hex(), "userId", roomUser.UserID.Hex())
			return models.NewInternalError(err, "Failed to update room user")
		}

		return models.ErrUserAlreadyInRoom
	}

	// Insert into database
	_, err = r.roomUsersCollection.InsertOne(ctx, roomUser)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return models.ErrUserAlreadyInRoom
		}
		r.logger.Error("Failed to add user to room", err, "roomId", roomUser.RoomID.Hex(), "userId", roomUser.UserID.Hex())
		return models.NewInternalError(err, "Failed to add user to room")
	}

	// Update room stats
	update := bson.D{
		cmdInc(bson.M{
			"stats.totalUsers": 1,
		}),
		cmdSet(bson.M{
			"lastActivity": now,
			"updatedAt":    now,
		}),
		// Update peak users if needed
		cmdMax(bson.M{
			"stats.peakUsers": count + 1,
		}),
	}

	_, err = r.roomCollection.UpdateByID(ctx, roomUser.RoomID, update)
	if err != nil {
		r.logger.Error("Failed to update room stats", err, "roomId", roomUser.RoomID.Hex())
		// Continue anyway, the user was added to the room
	}

	return nil
}

// RemoveUserFromRoom removes a user from a room.
func (r *roomRepository) RemoveUserFromRoom(ctx context.Context, roomID, userID bson.ObjectID) error {
	result, err := r.roomUsersCollection.DeleteOne(ctx, roomAndUserIDs(roomID, userID))

	if err != nil {
		r.logger.Error("Failed to remove user from room", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return models.NewInternalError(err, "Failed to remove user from room")
	}

	if result.DeletedCount == 0 {
		return models.ErrUserNotInRoom
	}

	// If user was the current DJ, clear current DJ
	room, err := r.FindByID(ctx, roomID)
	if err != nil {
		return err
	}

	if room.CurrentDJ == userID {
		// Clear current DJ and media
		update := bson.D{
			cmdSet(bson.M{
				"currentDJ":    bson.NilObjectID,
				"currentMedia": bson.NilObjectID,
				"updatedAt":    time.Now(),
			}),
		}
		_, err = r.roomCollection.UpdateByID(ctx, roomID, update)
		if err != nil {
			r.logger.Error("Failed to clear current DJ", err, "roomId", roomID.Hex())
			// Continue anyway, the user was removed from the room
		}
	}

	return nil
}

// FindRoomUsers finds all users in a room.
func (r *roomRepository) FindRoomUsers(ctx context.Context, roomID bson.ObjectID) ([]*models.RoomUser, error) {
	cursor, err := r.roomUsersCollection.Find(ctx, bson.M{"roomId": roomID})
	if err != nil {
		r.logger.Error("Failed to find room users", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to find room users")
	}
	defer cursor.Close(ctx)

	var roomUsers []*models.RoomUser
	if err = cursor.All(ctx, &roomUsers); err != nil {
		r.logger.Error("Failed to decode room users", err)
		return nil, models.NewInternalError(err, "Failed to decode room users")
	}

	return roomUsers, nil
}

// FindUserRoom finds the room a user is currently in.
func (r *roomRepository) FindUserRoom(ctx context.Context, userID bson.ObjectID) (*models.RoomUser, error) {
	var roomUser models.RoomUser

	err := r.roomUsersCollection.FindOne(ctx, bson.M{"userId": userID}).Decode(&roomUser)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrUserNotInRoom
		}
		r.logger.Error("Failed to find user's room", err, "userId", userID.Hex())
		return nil, models.NewInternalError(err, "Failed to find user's room")
	}

	return &roomUser, nil
}

// UpdateRoomUser updates a room user's information.
func (r *roomRepository) UpdateRoomUser(ctx context.Context, roomUser *models.RoomUser) error {
	roomUser.LastActive = time.Now()

	result, err := r.roomUsersCollection.ReplaceOne(ctx, roomAndUserIDs(roomUser.RoomID, roomUser.UserID), roomUser)

	if err != nil {
		r.logger.Error("Failed to update room user", err, "roomId", roomUser.RoomID.Hex(), "userId", roomUser.UserID.Hex())
		return models.NewInternalError(err, "Failed to update room user")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotInRoom
	}

	return nil
}

// UpdateDJQueue updates the DJ queue for a room.
func (r *roomRepository) UpdateDJQueue(ctx context.Context, roomID bson.ObjectID, queueEntries []models.QueueEntry) error {
	// We'll store the DJ queue in Redis for real-time access, but also update the room state
	// to reflect the current queue state for persistence.

	// Update room's lastActivity
	update := bson.D{
		cmdSet(bson.M{
			"lastActivity": time.Now(),
			"updatedAt":    time.Now(),
		}),
	}

	result, err := r.roomCollection.UpdateByID(ctx, roomID, update)
	if err != nil {
		r.logger.Error("Failed to update DJ queue", err, "roomId", roomID.Hex())
		return models.NewInternalError(err, "Failed to update DJ queue")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	// Update user positions in roomUsers collection
	for _, entry := range queueEntries {
		_, err = r.roomUsersCollection.UpdateOne(
			ctx,
			roomAndUserIDs(roomID, entry.User.ID),
			bson.D{
				cmdSet(bson.M{
					"position": entry.Position,
					"isDJ":     true,
				}),
			},
		)

		if err != nil {
			r.logger.Error("Failed to update DJ position", err, "roomId", roomID.Hex(), "userId", entry.User.ID.Hex())
			// Continue with other updates
		}
	}

	return nil
}

// SetCurrentDJ sets the current DJ for a room.
func (r *roomRepository) SetCurrentDJ(ctx context.Context, roomID, userID bson.ObjectID) error {
	// Check if user is in the room
	if !userID.IsZero() {
		var roomUser models.RoomUser
		err := r.roomUsersCollection.FindOne(ctx, roomAndUserIDs(roomID, userID)).Decode(&roomUser)

		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return models.ErrUserNotInRoom
			}
			r.logger.Error("Failed to find room user", err, "roomId", roomID.Hex(), "userId", userID.Hex())
			return models.NewInternalError(err, "Failed to find room user")
		}
	}

	// Update room's current DJ
	update := bson.D{
		cmdSet(bson.M{
			"currentDJ":    userID,
			"updatedAt":    time.Now(),
			"lastActivity": time.Now(),
		}),
	}

	// If setting to nil, also clear current media
	if userID.IsZero() {
		update[0].Value.(bson.M)["currentMedia"] = bson.NilObjectID
	}

	result, err := r.roomCollection.UpdateByID(ctx, roomID, update)
	if err != nil {
		r.logger.Error("Failed to set current DJ", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return models.NewInternalError(err, "Failed to set current DJ")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	return nil
}

// SetCurrentMedia sets the current media for a room.
func (r *roomRepository) SetCurrentMedia(ctx context.Context, roomID, mediaID bson.ObjectID) error {
	now := time.Now()
	setMap := bson.M{
		"updatedAt":    now,
		"lastActivity": now,
	}
	var update bson.D

	// If setting to nil, clear the current media
	if mediaID.IsZero() {
		update = bson.D{
			cmdUnset(bson.M{"currentMedia": ""}),
			cmdSet(setMap),
		}
	} else {
		setMap["currentMedia"] = mediaID
		update = bson.D{
			cmdSet(setMap),
		}
	}

	result, err := r.roomCollection.UpdateByID(ctx, roomID, update)
	if err != nil {
		r.logger.Error("Failed to set current media", err, "roomId", roomID.Hex(), "mediaId", mediaID.Hex())
		return models.NewInternalError(err, "Failed to set current media")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	return nil
}

// AddModerator adds a moderator to a room.
func (r *roomRepository) AddModerator(ctx context.Context, roomID, userID bson.ObjectID) error {
	update := bson.D{
		cmdAddToSet(bson.M{"moderators": userID}),
		cmdSet(bson.M{
			"updatedAt": time.Now(),
		}),
	}

	result, err := r.roomCollection.UpdateByID(ctx, roomID, update)
	if err != nil {
		r.logger.Error("Failed to add moderator", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return models.NewInternalError(err, "Failed to add moderator")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	// Update user's role in the room
	_, err = r.roomUsersCollection.UpdateOne(
		ctx,
		roomAndUserIDs(roomID, userID),
		bson.D{
			cmdSet(bson.M{
				"role":       "moderator",
				"lastActive": time.Now(),
			}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to update user role", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		// Continue anyway, the moderator was added to the room
	}

	return nil
}

// RemoveModerator removes a moderator from a room.
func (r *roomRepository) RemoveModerator(ctx context.Context, roomID, userID bson.ObjectID) error {
	// Check if user is the room creator
	room, err := r.FindByID(ctx, roomID)
	if err != nil {
		return err
	}

	if room.CreatedBy == userID {
		return models.NewRoomError(errors.New("cannot remove room creator as moderator"), "Cannot remove room creator as moderator", 403)
	}

	update := bson.D{
		cmdPull(bson.M{"moderators": userID}),
		cmdSet(bson.M{
			"updatedAt": time.Now(),
		}),
	}

	result, err := r.roomCollection.UpdateByID(ctx, roomID, update)
	if err != nil {
		r.logger.Error("Failed to remove moderator", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return models.NewInternalError(err, "Failed to remove moderator")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	// Update user's role in the room
	_, err = r.roomUsersCollection.UpdateOne(
		ctx,
		roomAndUserIDs(roomID, userID),
		bson.D{
			cmdSet(bson.M{
				"role":       "user",
				"lastActive": time.Now(),
			}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to update user role", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		// Continue anyway, the moderator was removed from the room
	}

	return nil
}

// BanUser bans a user from a room.
func (r *roomRepository) BanUser(ctx context.Context, roomID, userID bson.ObjectID) error {
	// Check if user is the room creator
	room, err := r.FindByID(ctx, roomID)
	if err != nil {
		return err
	}

	if room.CreatedBy == userID {
		return models.NewRoomError(errors.New("cannot ban room creator"), "Cannot ban room creator", 403)
	}

	update := bson.D{
		cmdAddToSet(bson.M{"bannedUsers": userID}),
		cmdSet(bson.M{
			"updatedAt": time.Now(),
		}),
	}

	result, err := r.roomCollection.UpdateByID(ctx, roomID, update)
	if err != nil {
		r.logger.Error("Failed to ban user", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return models.NewInternalError(err, "Failed to ban user")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	// Remove user from room if present
	_, err = r.roomUsersCollection.DeleteOne(ctx, roomAndUserIDs(roomID, userID))

	if err != nil {
		r.logger.Error("Failed to remove banned user from room", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		// Continue anyway, the user was banned
	}

	// If user was the current DJ, clear current DJ
	if room.CurrentDJ == userID {
		update := bson.D{
			cmdSet(bson.M{
				"currentDJ":    bson.NilObjectID,
				"currentMedia": bson.NilObjectID,
				"updatedAt":    time.Now(),
			}),
		}
		_, err = r.roomCollection.UpdateByID(ctx, roomID, update)
		if err != nil {
			r.logger.Error("Failed to clear current DJ", err, "roomId", roomID.Hex())
			// Continue anyway, the user was banned
		}
	}

	return nil
}

// UnbanUser unbans a user from a room.
func (r *roomRepository) UnbanUser(ctx context.Context, roomID, userID bson.ObjectID) error {
	update := bson.D{
		cmdPull(bson.M{"bannedUsers": userID}),
		cmdSet(bson.M{
			"updatedAt": time.Now(),
		}),
	}

	result, err := r.roomCollection.UpdateByID(ctx, roomID, update)
	if err != nil {
		r.logger.Error("Failed to unban user", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return models.NewInternalError(err, "Failed to unban user")
	}

	if result.MatchedCount == 0 {
		return models.ErrRoomNotFound
	}

	return nil
}

// IsUserBanned checks if a user is banned from a room.
func (r *roomRepository) IsUserBanned(ctx context.Context, roomID, userID bson.ObjectID) (bool, error) {
	count, err := r.roomCollection.CountDocuments(ctx, bson.M{
		"_id":         roomID,
		"bannedUsers": userID,
	})

	if err != nil {
		r.logger.Error("Failed to check if user is banned", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return false, models.NewInternalError(err, "Failed to check ban status")
	}

	return count > 0, nil
}

// SearchRooms searches for rooms based on criteria.
func (r *roomRepository) SearchRooms(ctx context.Context, criteria models.RoomSearchCriteria) ([]*models.Room, int64, error) {
	filter := bson.M{}

	// Apply active filter
	if criteria.OnlyActive {
		filter["isActive"] = true
	}

	// Apply private filter
	if !criteria.IncludePrivate {
		filter["settings.private"] = false
	}

	// Apply user range filter
	if criteria.MinUsers > 0 {
		filter["stats.activeUsers"] = bson.M{"$gte": criteria.MinUsers}
	}
	if criteria.MaxUsers > 0 {
		if _, ok := filter["stats.activeUsers"]; ok {
			filter["stats.activeUsers"].(bson.M)["$lte"] = criteria.MaxUsers
		} else {
			filter["stats.activeUsers"] = bson.M{"$lte": criteria.MaxUsers}
		}
	}

	// Apply tag filter
	if len(criteria.Tags) > 0 {
		filter["tags"] = bson.M{"$all": criteria.Tags}
	}

	// Apply text search if query provided
	if criteria.Query != "" {
		filter["$text"] = bson.M{"$search": criteria.Query}
	}

	// Count total matches
	total, err := r.CountRooms(ctx, filter)
	if err != nil {
		return nil, 0, err
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
	case "active":
		sort["lastActivity"] = -1
	case "users":
		sort["stats.activeUsers"] = -1
	case "popularity":
		sort["stats.aggregateRating"] = -1
	default:
		// Default sort by activity
		if len(sort) == 0 {
			sort["lastActivity"] = -1
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

	// Find rooms
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(criteria.Limit)).
		SetSort(sort)

	// If text search, add projections
	if criteria.Query != "" && filter["$text"] != nil {
		opts.SetProjection(bson.M{"score": bson.M{"$meta": "textScore"}})
	}

	rooms, err := r.FindMany(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}

	return rooms, total, nil
}

// FindPopularRooms finds the most popular active rooms.
func (r *roomRepository) FindPopularRooms(ctx context.Context, limit int) ([]*models.Room, error) {
	filter := bson.M{
		"isActive": true,
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.M{"stats.activeUsers": -1})

	return r.FindMany(ctx, filter, opts)
}

// FindRecentRooms finds recently active rooms.
func (r *roomRepository) FindRecentRooms(ctx context.Context, limit int) ([]*models.Room, error) {
	filter := bson.M{
		"isActive": true,
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.M{"lastActivity": -1})

	return r.FindMany(ctx, filter, opts)
}

func roomAndUserIDs(roomID, userID bson.ObjectID) bson.D {
	return bson.D{
		{Key: "roomId", Value: roomID},
		{Key: "userId", Value: userID},
	}
}
