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

// Collection name
const (
	userCollection = "users"
)

// UserRepository defines the interface for user data access operations.
type UserRepository interface {
	// Create creates a new user.
	Create(ctx context.Context, user *models.User) error

	// FindByID finds a user by their ID.
	FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error)

	// FindByEmail finds a user by their email address.
	FindByEmail(ctx context.Context, email string) (*models.User, error)

	// FindByUsername finds a user by their username.
	FindByUsername(ctx context.Context, username string) (*models.User, error)

	// FindMany finds multiple users based on query filters.
	FindMany(ctx context.Context, filter bson.M, options options.Lister[options.FindOptions]) ([]*models.User, error)

	// Update updates an existing user.
	Update(ctx context.Context, user *models.User) error

	// UpdateLastLogin updates a user's last login time.
	UpdateLastLogin(ctx context.Context, id bson.ObjectID) error

	// Delete deletes a user by their ID.
	Delete(ctx context.Context, id bson.ObjectID) error

	// CountUsers counts the number of users that match the given filter.
	CountUsers(ctx context.Context, filter bson.M) (int64, error)

	// FindFollowing finds users that the given user is following.
	FindFollowing(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.User, error)

	// FindFollowers finds users that follow the given user.
	FindFollowers(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.User, error)

	// IsFollowing checks if a user is following another user.
	IsFollowing(ctx context.Context, userID, targetID bson.ObjectID) (bool, error)

	// Follow makes a user follow another user.
	Follow(ctx context.Context, userID, targetID bson.ObjectID) error

	// Unfollow makes a user unfollow another user.
	Unfollow(ctx context.Context, userID, targetID bson.ObjectID) error

	// UpdateAvatar updates a user's avatar configuration.
	UpdateAvatar(ctx context.Context, userID bson.ObjectID, avatar models.AvatarConfig) error

	// UpdateSettings updates a user's settings.
	UpdateSettings(ctx context.Context, userID bson.ObjectID, settings models.UserSettings) error

	// AddBadge adds a badge to a user.
	AddBadge(ctx context.Context, userID bson.ObjectID, badge string) error

	// RemoveBadge removes a badge from a user.
	RemoveBadge(ctx context.Context, userID bson.ObjectID, badge string) error

	// UpdateStats updates a user's statistics.
	UpdateStats(ctx context.Context, userID bson.ObjectID, updates bson.M) error

	// SetActive sets a user's active status.
	SetActive(ctx context.Context, userID bson.ObjectID, active bool) error

	// SetVerified sets a user's verified status.
	SetVerified(ctx context.Context, userID bson.ObjectID, verified bool) error

	// FindInactive finds users who haven't logged in for the specified duration.
	FindInactive(ctx context.Context, duration time.Duration, limit int) ([]*models.User, error)
}

// userRepository is the MongoDB implementation of UserRepository.
type userRepository struct {
	collection *mongo.Collection
	logger     *utils.Logger
}

// NewUserRepository creates a new instance of UserRepository.
func NewUserRepository(db *mongo.Database, logger *utils.Logger) UserRepository {
	return &userRepository{
		collection: db.Collection(userCollection),
		logger:     logger.Named("user_repository"),
	}
}

// Create creates a new user.
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}

	now := time.Now()
	user.TimeCreate(now)

	if user.Profile.JoinDate.IsZero() {
		user.Profile.JoinDate = now
	}

	_, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Check which field is duplicated
			if strings.Contains(err.Error(), "email") {
				return models.ErrEmailAlreadyExists
			}
			if strings.Contains(err.Error(), "username") {
				return models.ErrUsernameAlreadyExists
			}
			return models.ErrUserAlreadyExists
		}
		r.logger.Error("Failed to create user", err, "email", user.Email, "username", user.Username)
		return models.NewInternalError(err, "Failed to create user")
	}

	return nil
}

// FindByID finds a user by their ID.
func (r *userRepository) FindByID(ctx context.Context, id bson.ObjectID) (*models.User, error) {
	var user models.User

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrUserNotFound
		}
		r.logger.Error("Failed to find user by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find user")
	}

	return &user, nil
}

// FindByEmail finds a user by their email address.
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	err := r.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrUserNotFound
		}
		r.logger.Error("Failed to find user by email", err, "email", email)
		return nil, models.NewInternalError(err, "Failed to find user")
	}

	return &user, nil
}

// FindByUsername finds a user by their username.
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User

	// Case-insensitive search
	opts := options.FindOne().SetCollation(&options.Collation{
		Locale:    "en",
		Strength:  2, // Case-insensitive
		CaseLevel: false,
	})

	err := r.collection.FindOne(ctx, bson.M{"username": username}, opts).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, models.ErrUserNotFound
		}
		r.logger.Error("Failed to find user by username", err, "username", username)
		return nil, models.NewInternalError(err, "Failed to find user")
	}

	return &user, nil
}

// FindMany finds multiple users based on query filters.
func (r *userRepository) FindMany(ctx context.Context, filter bson.M, options options.Lister[options.FindOptions]) ([]*models.User, error) {
	cursor, err := r.collection.Find(ctx, filter, options)
	if err != nil {
		r.logger.Error("Failed to find users", err, "filter", filter)
		return nil, models.NewInternalError(err, "Failed to find users")
	}
	defer cursor.Close(ctx)

	var users []*models.User
	if err = cursor.All(ctx, &users); err != nil {
		r.logger.Error("Failed to decode users", err)
		return nil, models.NewInternalError(err, "Failed to decode users")
	}

	return users, nil
}

// Update updates an existing user.
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdateNow()

	result, err := r.collection.ReplaceOne(ctx, bson.M{"_id": user.ID}, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Check which field is duplicated
			if strings.Contains(err.Error(), "email") {
				return models.ErrEmailAlreadyExists
			}
			if strings.Contains(err.Error(), "username") {
				return models.ErrUsernameAlreadyExists
			}
			return models.ErrUserAlreadyExists
		}
		r.logger.Error("Failed to update user", err, "id", user.ID.Hex())
		return models.NewInternalError(err, "Failed to update user")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// UpdateLastLogin updates a user's last login time.
func (r *userRepository) UpdateLastLogin(ctx context.Context, id bson.ObjectID) error {
	now := time.Now()
	update := bson.D{
		cmdSet(bson.M{
			"lastLogin": now,
			"updatedAt": now,
		}),
	}

	result, err := r.collection.UpdateByID(ctx, id, update)
	if err != nil {
		r.logger.Error("Failed to update last login", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to update last login")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// Delete deletes a user by their ID.
func (r *userRepository) Delete(ctx context.Context, id bson.ObjectID) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		r.logger.Error("Failed to delete user", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to delete user")
	}

	if result.DeletedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// CountUsers counts the number of users that match the given filter.
func (r *userRepository) CountUsers(ctx context.Context, filter bson.M) (int64, error) {
	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		r.logger.Error("Failed to count users", err, "filter", filter)
		return 0, models.NewInternalError(err, "Failed to count users")
	}

	return count, nil
}

// FindFollowing finds users that the given user is following.
func (r *userRepository) FindFollowing(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.User, error) {
	// First get the user to retrieve their following list
	user, err := r.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(user.Connections.Following) == 0 {
		return []*models.User{}, nil
	}

	// Query users that are in the following list
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"username": 1})

	return r.FindMany(ctx, bson.M{"_id": bson.M{"$in": user.Connections.Following}}, opts)
}

// FindFollowers finds users that follow the given user.
func (r *userRepository) FindFollowers(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.User, error) {
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.M{"username": 1})

	return r.FindMany(ctx, bson.M{"connections.following": userID}, opts)
}

// IsFollowing checks if a user is following another user.
func (r *userRepository) IsFollowing(ctx context.Context, userID, targetID bson.ObjectID) (bool, error) {
	count, err := r.collection.CountDocuments(ctx, bson.M{
		"_id":                   userID,
		"connections.following": targetID,
	})

	if err != nil {
		r.logger.Error("Failed to check if user is following", err, "userID", userID.Hex(), "targetID", targetID.Hex())
		return false, models.NewInternalError(err, "Failed to check following status")
	}

	return count > 0, nil
}

// Follow makes a user follow another user.
func (r *userRepository) Follow(ctx context.Context, userID, targetID bson.ObjectID) error {
	// Check if already following
	isFollowing, err := r.IsFollowing(ctx, userID, targetID)
	if err != nil {
		return err
	}

	if isFollowing {
		return nil // Already following, no error
	}

	// Update the user's following list
	result, err := r.collection.UpdateByID(ctx,
		userID,
		bson.D{
			cmdAddToSet(bson.M{"connections.following": targetID}),
			cmdSet(bson.M{"updatedAt": time.Now()}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to follow user", err, "userID", userID.Hex(), "targetID", targetID.Hex())
		return models.NewInternalError(err, "Failed to follow user")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	// Check if mutual follow and update friends list for both users
	isFollower, err := r.IsFollowing(ctx, targetID, userID)
	if err != nil {
		return err
	}

	if isFollower {
		// Mutual follow, add to friends lists
		_, err = r.collection.UpdateMany(ctx,
			bson.M{"_id": bson.M{"$in": []bson.ObjectID{userID, targetID}}},
			bson.M{
				"$addToSet": bson.M{"connections.friends": bson.M{"$cond": bson.A{
					bson.D{{Key: "$eq", Value: []any{"$_id", userID}}},
					targetID,
					userID,
				}}},
			},
		)

		if err != nil {
			r.logger.Error("Failed to update friends lists", err, "userID", userID.Hex(), "targetID", targetID.Hex())
			return models.NewInternalError(err, "Failed to update friends lists")
		}
	}

	// Update the target user's followers list
	_, err = r.collection.UpdateByID(ctx,
		targetID,
		bson.D{
			cmdAddToSet(bson.M{"connections.followers": userID}),
			cmdSet(bson.M{"updatedAt": time.Now()}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to update follower list", err, "targetID", targetID.Hex(), "userID", userID.Hex())
		return models.NewInternalError(err, "Failed to update follower list")
	}

	return nil
}

// Unfollow makes a user unfollow another user.
func (r *userRepository) Unfollow(ctx context.Context, userID, targetID bson.ObjectID) error {
	// Update the user's following list
	result, err := r.collection.UpdateByID(ctx,
		userID,
		bson.D{
			cmdPull(bson.M{"connections.following": targetID, "connections.friends": targetID}),
			cmdSet(bson.M{"updatedAt": time.Now()}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to unfollow user", err, "userID", userID.Hex(), "targetID", targetID.Hex())
		return models.NewInternalError(err, "Failed to unfollow user")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	// Update the target user's followers list and friends list
	_, err = r.collection.UpdateByID(ctx,
		targetID,
		bson.D{
			cmdPull(bson.M{"connections.followers": userID, "connections.friends": userID}),
			cmdSet(bson.M{"updatedAt": time.Now()}),
		},
	)

	if err != nil {
		r.logger.Error("Failed to update follower list", err, "targetID", targetID.Hex(), "userID", userID.Hex())
		return models.NewInternalError(err, "Failed to update follower list")
	}

	return nil
}

// UpdateAvatar updates a user's avatar configuration.
func (r *userRepository) UpdateAvatar(ctx context.Context, userID bson.ObjectID, avatar models.AvatarConfig) error {
	update := bson.D{
		cmdSet(bson.M{
			"avatarConfig": avatar,
			"updatedAt":    time.Now(),
		}),
	}

	result, err := r.collection.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to update avatar", err, "userID", userID.Hex())
		return models.NewInternalError(err, "Failed to update avatar")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// UpdateSettings updates a user's settings.
func (r *userRepository) UpdateSettings(ctx context.Context, userID bson.ObjectID, settings models.UserSettings) error {
	update := bson.D{
		cmdSet(bson.M{
			"settings":  settings,
			"updatedAt": time.Now(),
		}),
	}

	result, err := r.collection.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to update settings", err, "userID", userID.Hex())
		return models.NewInternalError(err, "Failed to update settings")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// AddBadge adds a badge to a user.
func (r *userRepository) AddBadge(ctx context.Context, userID bson.ObjectID, badge string) error {
	update := bson.D{
		cmdAddToSet(bson.M{"badges": badge}),
		cmdSet(bson.M{"updatedAt": time.Now()}),
	}

	result, err := r.collection.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to add badge", err, "userID", userID.Hex(), "badge", badge)
		return models.NewInternalError(err, "Failed to add badge")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// RemoveBadge removes a badge from a user.
func (r *userRepository) RemoveBadge(ctx context.Context, userID bson.ObjectID, badge string) error {
	update := bson.D{
		cmdPull(bson.M{"badges": badge}),
		cmdSet(bson.M{"updatedAt": time.Now()}),
	}

	result, err := r.collection.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to remove badge", err, "userID", userID.Hex(), "badge", badge)
		return models.NewInternalError(err, "Failed to remove badge")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// UpdateStats updates a user's statistics.
func (r *userRepository) UpdateStats(ctx context.Context, userID bson.ObjectID, updates bson.M) error {
	// Add timestamp to updates
	updateDoc := bson.D{
		cmdSet(bson.M{"stats.lastUpdated": time.Now(), "updatedAt": time.Now()}),
	}

	// Add all stat updates with $inc operator
	for key, value := range updates {
		if len(updateDoc) > 1 {
			// Add to existing $inc
			incMap := updateDoc[1].Value.(bson.M)
			incMap["stats."+key] = value
		} else {
			// Create new $inc
			updateDoc = append(updateDoc, cmdInc(bson.M{"stats." + key: value}))
		}
	}

	result, err := r.collection.UpdateByID(ctx, userID, updateDoc)
	if err != nil {
		r.logger.Error("Failed to update stats", err, "userID", userID.Hex(), "updates", updates)
		return models.NewInternalError(err, "Failed to update stats")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// SetActive sets a user's active status.
func (r *userRepository) SetActive(ctx context.Context, userID bson.ObjectID, active bool) error {
	update := bson.D{
		cmdSet(bson.M{
			"isActive":  active,
			"updatedAt": time.Now(),
		}),
	}

	result, err := r.collection.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to set active status", err, "userID", userID.Hex(), "active", active)
		return models.NewInternalError(err, "Failed to set active status")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// SetVerified sets a user's verified status.
func (r *userRepository) SetVerified(ctx context.Context, userID bson.ObjectID, verified bool) error {
	update := bson.D{
		cmdSet(bson.M{
			"isVerified": verified,
			"updatedAt":  time.Now(),
		}),
	}

	result, err := r.collection.UpdateByID(ctx, userID, update)
	if err != nil {
		r.logger.Error("Failed to set verified status", err, "userID", userID.Hex(), "verified", verified)
		return models.NewInternalError(err, "Failed to set verified status")
	}

	if result.MatchedCount == 0 {
		return models.ErrUserNotFound
	}

	return nil
}

// FindInactive finds users who haven't logged in for the specified duration.
func (r *userRepository) FindInactive(ctx context.Context, duration time.Duration, limit int) ([]*models.User, error) {
	cutoff := time.Now().Add(-duration)

	filter := bson.M{
		"lastLogin": bson.M{"$lt": cutoff},
		"isActive":  true,
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.M{"lastLogin": 1}) // Oldest login first

	return r.FindMany(ctx, filter, opts)
}
