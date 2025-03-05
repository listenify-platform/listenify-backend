// Package user provides services for user management and operations.
package user

import (
	"context"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// StatsService provides functionality for tracking and managing user statistics.
type StatsService struct {
	userManager *Manager
	logger      *utils.Logger
}

// NewStatsService creates a new stats service.
func NewStatsService(userManager *Manager, logger *utils.Logger) *StatsService {
	return &StatsService{
		userManager: userManager,
		logger:      logger.Named("stats_service"),
	}
}

// GetUserStats retrieves a user's statistics.
func (s *StatsService) GetUserStats(ctx context.Context, userID string) (*models.UserStats, error) {
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user.Stats, nil
}

// AddExperience adds experience points to a user's stats and updates their level if necessary.
func (s *StatsService) AddExperience(ctx context.Context, userID string, amount int) error {
	// Get current user
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Update experience
	user.Stats.Experience += amount
	user.Stats.LastUpdated = time.Now()

	// Check if user should level up
	newLevel := s.calculateLevel(user.Stats.Experience)
	if newLevel > user.Stats.Level {
		// Level up!
		oldLevel := user.Stats.Level
		user.Stats.Level = newLevel
		s.logger.Info("User leveled up", "userId", userID, "oldLevel", oldLevel, "newLevel", newLevel)
	}

	// Update user stats with $inc operator
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Create updates map for experience and level if needed
	updates := bson.M{"experience": amount}
	if newLevel > user.Stats.Level {
		// Also update the level
		updates["level"] = newLevel - user.Stats.Level // Increment by the difference
	}

	if err := s.userManager.userRepo.UpdateStats(ctx, objectID, updates); err != nil {
		s.logger.Error("Failed to update user stats", err, "userId", userID)
		return err
	}

	return nil
}

// AddPoints adds points to a user's stats.
func (s *StatsService) AddPoints(ctx context.Context, userID string, amount int) error {
	// Get current user
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Update points
	user.Stats.Points += amount
	user.Stats.LastUpdated = time.Now()

	// Update user stats with $inc operator
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Create updates map for points
	updates := bson.M{"points": amount}

	if err := s.userManager.userRepo.UpdateStats(ctx, objectID, updates); err != nil {
		s.logger.Error("Failed to update user stats", err, "userId", userID)
		return err
	}

	return nil
}

// TrackListeningTime tracks a user's listening time and awards experience points.
func (s *StatsService) TrackListeningTime(ctx context.Context, userID string, minutes int) error {
	// Award 1 XP per minute of listening
	experienceToAdd := minutes

	// Add experience
	if err := s.AddExperience(ctx, userID, experienceToAdd); err != nil {
		return err
	}

	return nil
}

// TrackDJTime tracks a user's time as a DJ and awards experience points.
func (s *StatsService) TrackDJTime(ctx context.Context, userID string, minutes int) error {
	// Award 2 XP per minute of DJing (more than just listening)
	experienceToAdd := minutes * 2

	// Add experience
	if err := s.AddExperience(ctx, userID, experienceToAdd); err != nil {
		return err
	}

	return nil
}

// TrackSongPlayed tracks when a user plays a song as DJ and awards experience points.
func (s *StatsService) TrackSongPlayed(ctx context.Context, userID string) error {
	// Award 10 XP for playing a song
	experienceToAdd := 10

	// Add experience
	if err := s.AddExperience(ctx, userID, experienceToAdd); err != nil {
		return err
	}

	return nil
}

// TrackRoomCreated tracks when a user creates a room and awards experience points.
func (s *StatsService) TrackRoomCreated(ctx context.Context, userID string) error {
	// Award 50 XP for creating a room
	experienceToAdd := 50

	// Add experience
	if err := s.AddExperience(ctx, userID, experienceToAdd); err != nil {
		return err
	}

	return nil
}

// TrackPlaylistCreated tracks when a user creates a playlist and awards experience points.
func (s *StatsService) TrackPlaylistCreated(ctx context.Context, userID string) error {
	// Award 20 XP for creating a playlist
	experienceToAdd := 20

	// Add experience
	if err := s.AddExperience(ctx, userID, experienceToAdd); err != nil {
		return err
	}

	return nil
}

// TrackSongAdded tracks when a user adds a song to a playlist and awards experience points.
func (s *StatsService) TrackSongAdded(ctx context.Context, userID string) error {
	// Award 2 XP for adding a song to a playlist
	experienceToAdd := 2

	// Add experience
	if err := s.AddExperience(ctx, userID, experienceToAdd); err != nil {
		return err
	}

	return nil
}

// TrackUpvote tracks when a user receives an upvote for their song and awards experience points.
func (s *StatsService) TrackUpvote(ctx context.Context, userID string) error {
	// Award 5 XP for receiving an upvote
	experienceToAdd := 5

	// Add experience
	if err := s.AddExperience(ctx, userID, experienceToAdd); err != nil {
		return err
	}

	return nil
}

// GetTopUsers gets the top users by experience points.
func (s *StatsService) GetTopUsers(ctx context.Context, limit int) ([]*models.PublicUser, error) {
	// Create filter for active users
	filter := bson.M{"isActive": true}

	// Set options for sorting by experience points (descending) and limiting results
	opts := options.Find().
		SetSort(bson.M{"stats.experience": -1}).
		SetLimit(int64(limit))

	// Get top users from repository
	users, err := s.userManager.userRepo.FindMany(ctx, filter, opts)
	if err != nil {
		s.logger.Error("Failed to get top users", err)
		return nil, err
	}

	// Convert to public users
	publicUsers := make([]*models.PublicUser, 0, len(users))
	for _, user := range users {
		publicUser := user.ToPublicUser()

		// Check if user is online
		isOnline, err := s.userManager.IsUserOnline(ctx, user.ID.Hex())
		if err != nil {
			s.logger.Error("Failed to check if user is online", err, "userId", user.ID.Hex())
			// Continue anyway, default to false
		} else {
			publicUser.Online = isOnline
		}

		publicUsers = append(publicUsers, &publicUser)
	}

	return publicUsers, nil
}

// GetUserRank gets a user's rank based on experience points.
func (s *StatsService) GetUserRank(ctx context.Context, userID string) (int, error) {
	// Get the user to find their experience
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	// Count users with more experience
	filter := bson.M{
		"stats.experience": bson.M{"$gt": user.Stats.Experience},
		"isActive":         true,
	}

	higherCount, err := s.userManager.userRepo.CountUsers(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to count users with higher experience", err, "userId", userID)
		return 0, models.NewInternalError(err, "Failed to calculate user rank")
	}

	// Rank is count of users with more experience + 1
	return int(higherCount) + 1, nil
}

// GetExperienceForNextLevel calculates the experience required for the next level.
func (s *StatsService) GetExperienceForNextLevel(ctx context.Context, userID string) (int, error) {
	// Get current user
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	currentLevel := user.Stats.Level
	nextLevel := currentLevel + 1

	// Calculate experience required for next level
	expForNextLevel := s.calculateExperienceForLevel(nextLevel)

	return expForNextLevel, nil
}

// GetExperienceProgress calculates the user's progress towards the next level.
func (s *StatsService) GetExperienceProgress(ctx context.Context, userID string) (float64, error) {
	// Get current user
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	currentLevel := user.Stats.Level
	currentExp := user.Stats.Experience

	// Calculate experience required for current and next level
	expForCurrentLevel := s.calculateExperienceForLevel(currentLevel)
	expForNextLevel := s.calculateExperienceForLevel(currentLevel + 1)

	// Calculate progress
	expInCurrentLevel := currentExp - expForCurrentLevel
	expRequiredForNextLevel := expForNextLevel - expForCurrentLevel

	progress := float64(expInCurrentLevel) / float64(expRequiredForNextLevel)

	// Ensure progress is between 0 and 1
	if progress < 0 {
		progress = 0
	} else if progress > 1 {
		progress = 1
	}

	return progress, nil
}

// calculateLevel calculates the level based on experience points.
// The formula is: level = 1 + floor(sqrt(experience / 100))
func (s *StatsService) calculateLevel(experience int) int {
	if experience < 0 {
		return 1
	}

	level := 1 + int(math.Floor(math.Sqrt(float64(experience)/100)))
	return level
}

// calculateExperienceForLevel calculates the minimum experience required for a given level.
// The formula is: experience = 100 * (level - 1)^2
func (s *StatsService) calculateExperienceForLevel(level int) int {
	if level <= 1 {
		return 0
	}

	experience := 100 * int(math.Pow(float64(level-1), 2))
	return experience
}
