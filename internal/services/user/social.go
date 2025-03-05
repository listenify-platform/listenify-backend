// Package user provides services for user management and operations.
package user

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// SocialSummary represents a summary of a user's social connections.
type SocialSummary struct {
	FollowersCount int `json:"followersCount"`
	FollowingCount int `json:"followingCount"`
	FriendsCount   int `json:"friendsCount"`
}

// SocialService provides functionality for managing user social connections.
type SocialService struct {
	userManager *Manager
	logger      *utils.Logger
}

// NewSocialService creates a new social service.
func NewSocialService(userManager *Manager, logger *utils.Logger) *SocialService {
	return &SocialService{
		userManager: userManager,
		logger:      logger.Named("social_service"),
	}
}

// FollowUser makes a user follow another user.
func (s *SocialService) FollowUser(ctx context.Context, userID, targetID string) error {
	// Convert string IDs to ObjectIDs
	userObjectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	targetObjectID, err := bson.ObjectIDFromHex(targetID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Check if user is trying to follow themselves
	if userID == targetID {
		return errors.New("cannot follow yourself")
	}

	// Check if target user exists
	_, err = s.userManager.GetUserByID(ctx, targetID)
	if err != nil {
		return err
	}

	// Follow user
	if err := s.userManager.userRepo.Follow(ctx, userObjectID, targetObjectID); err != nil {
		s.logger.Error("Failed to follow user", err, "userId", userID, "targetId", targetID)
		return err
	}

	return nil
}

// UnfollowUser makes a user unfollow another user.
func (s *SocialService) UnfollowUser(ctx context.Context, userID, targetID string) error {
	// Convert string IDs to ObjectIDs
	userObjectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	targetObjectID, err := bson.ObjectIDFromHex(targetID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Check if user is trying to unfollow themselves
	if userID == targetID {
		return errors.New("cannot unfollow yourself")
	}

	// Unfollow user
	if err := s.userManager.userRepo.Unfollow(ctx, userObjectID, targetObjectID); err != nil {
		s.logger.Error("Failed to unfollow user", err, "userId", userID, "targetId", targetID)
		return err
	}

	return nil
}

// IsFollowing checks if a user is following another user.
func (s *SocialService) IsFollowing(ctx context.Context, userID, targetID string) (bool, error) {
	// Convert string IDs to ObjectIDs
	userObjectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return false, models.ErrInvalidID
	}

	targetObjectID, err := bson.ObjectIDFromHex(targetID)
	if err != nil {
		return false, models.ErrInvalidID
	}

	// Check if user is following target
	isFollowing, err := s.userManager.userRepo.IsFollowing(ctx, userObjectID, targetObjectID)
	if err != nil {
		s.logger.Error("Failed to check if user is following", err, "userId", userID, "targetId", targetID)
		return false, err
	}

	return isFollowing, nil
}

// GetFollowers gets a list of users who follow the specified user.
func (s *SocialService) GetFollowers(ctx context.Context, userID string, skip, limit int) ([]*models.PublicUser, error) {
	// Convert string ID to ObjectID
	userObjectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, models.ErrInvalidID
	}

	// Get followers
	followers, err := s.userManager.userRepo.FindFollowers(ctx, userObjectID, skip, limit)
	if err != nil {
		s.logger.Error("Failed to get followers", err, "userId", userID)
		return nil, err
	}

	// Convert to public users
	publicUsers := make([]*models.PublicUser, 0, len(followers))
	for _, user := range followers {
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

// GetFollowing gets a list of users that the specified user follows.
func (s *SocialService) GetFollowing(ctx context.Context, userID string, skip, limit int) ([]*models.PublicUser, error) {
	// Convert string ID to ObjectID
	userObjectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, models.ErrInvalidID
	}

	// Get following
	following, err := s.userManager.userRepo.FindFollowing(ctx, userObjectID, skip, limit)
	if err != nil {
		s.logger.Error("Failed to get following", err, "userId", userID)
		return nil, err
	}

	// Convert to public users
	publicUsers := make([]*models.PublicUser, 0, len(following))
	for _, user := range following {
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

// GetFriends gets a list of users who are mutual followers (friends) with the specified user.
func (s *SocialService) GetFriends(ctx context.Context, userID string) ([]*models.PublicUser, error) {
	// Get user to access their friends list
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(user.Connections.Friends) == 0 {
		return []*models.PublicUser{}, nil
	}

	// Get friends
	friendUsers := make([]*models.PublicUser, 0, len(user.Connections.Friends))
	for _, friendID := range user.Connections.Friends {
		friend, err := s.userManager.GetUserByID(ctx, friendID.Hex())
		if err != nil {
			s.logger.Error("Failed to get friend", err, "userId", userID, "friendId", friendID.Hex())
			continue // Skip this friend and continue
		}

		publicUser := friend.ToPublicUser()

		// Check if user is online
		isOnline, err := s.userManager.IsUserOnline(ctx, friend.ID.Hex())
		if err != nil {
			s.logger.Error("Failed to check if user is online", err, "userId", friend.ID.Hex())
			// Continue anyway, default to false
		} else {
			publicUser.Online = isOnline
		}

		friendUsers = append(friendUsers, &publicUser)
	}

	return friendUsers, nil
}

// GetFollowersCount gets the number of followers for a user.
func (s *SocialService) GetFollowersCount(ctx context.Context, userID string) (int, error) {
	// Get user to access their followers count
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	return len(user.Connections.Followers), nil
}

// GetFollowingCount gets the number of users a user is following.
func (s *SocialService) GetFollowingCount(ctx context.Context, userID string) (int, error) {
	// Get user to access their following count
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	return len(user.Connections.Following), nil
}

// GetFriendsCount gets the number of friends a user has.
func (s *SocialService) GetFriendsCount(ctx context.Context, userID string) (int, error) {
	// Get user to access their friends count
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return 0, err
	}

	return len(user.Connections.Friends), nil
}

// GetSocialSummary gets a summary of a user's social connections.
func (s *SocialService) GetSocialSummary(ctx context.Context, userID string) (*SocialSummary, error) {
	// Get user to access their social connections
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary := &SocialSummary{
		FollowersCount: len(user.Connections.Followers),
		FollowingCount: len(user.Connections.Following),
		FriendsCount:   len(user.Connections.Friends),
	}

	return summary, nil
}

// GetMutualFollowers gets users who follow both the specified user and the target user.
func (s *SocialService) GetMutualFollowers(ctx context.Context, userID, targetID string) ([]*models.PublicUser, error) {
	// Get both users to access their followers
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	target, err := s.userManager.GetUserByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Find intersection of followers
	mutualFollowerIDs := make([]bson.ObjectID, 0)
	userFollowerMap := make(map[string]bool)

	for _, followerID := range user.Connections.Followers {
		userFollowerMap[followerID.Hex()] = true
	}

	for _, followerID := range target.Connections.Followers {
		if userFollowerMap[followerID.Hex()] {
			mutualFollowerIDs = append(mutualFollowerIDs, followerID)
		}
	}

	if len(mutualFollowerIDs) == 0 {
		return []*models.PublicUser{}, nil
	}

	// Get mutual followers
	mutualFollowers := make([]*models.PublicUser, 0, len(mutualFollowerIDs))
	for _, followerID := range mutualFollowerIDs {
		follower, err := s.userManager.GetUserByID(ctx, followerID.Hex())
		if err != nil {
			s.logger.Error("Failed to get mutual follower", err, "userId", userID, "targetId", targetID, "followerId", followerID.Hex())
			continue // Skip this follower and continue
		}

		publicUser := follower.ToPublicUser()

		// Check if user is online
		isOnline, err := s.userManager.IsUserOnline(ctx, follower.ID.Hex())
		if err != nil {
			s.logger.Error("Failed to check if user is online", err, "userId", follower.ID.Hex())
			// Continue anyway, default to false
		} else {
			publicUser.Online = isOnline
		}

		mutualFollowers = append(mutualFollowers, &publicUser)
	}

	return mutualFollowers, nil
}

// GetSuggestedUsers gets a list of suggested users to follow based on mutual connections.
func (s *SocialService) GetSuggestedUsers(ctx context.Context, userID string, limit int) ([]*models.PublicUser, error) {
	// Get user to access their following list
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get users that the user's following are following (friends of friends)
	suggestedUserMap := make(map[string]bool)
	alreadyFollowingMap := make(map[string]bool)

	// Mark users that the current user is already following
	for _, followingID := range user.Connections.Following {
		alreadyFollowingMap[followingID.Hex()] = true
	}

	// For each user that the current user is following
	for _, followingID := range user.Connections.Following {
		// Get the users that this following user is following
		following, err := s.userManager.GetUserByID(ctx, followingID.Hex())
		if err != nil {
			s.logger.Error("Failed to get following user", err, "userId", userID, "followingId", followingID.Hex())
			continue // Skip this following user and continue
		}

		// Add their following to suggestions if not already following
		for _, potentialSuggestionID := range following.Connections.Following {
			potentialID := potentialSuggestionID.Hex()

			// Skip if it's the current user or already following
			if potentialID == userID || alreadyFollowingMap[potentialID] {
				continue
			}

			suggestedUserMap[potentialID] = true
		}
	}

	// Convert map to slice of IDs
	suggestedUserIDs := make([]string, 0, len(suggestedUserMap))
	for id := range suggestedUserMap {
		suggestedUserIDs = append(suggestedUserIDs, id)
	}

	// Limit the number of suggestions
	if len(suggestedUserIDs) > limit {
		suggestedUserIDs = suggestedUserIDs[:limit]
	}

	// Get suggested users
	suggestedUsers := make([]*models.PublicUser, 0, len(suggestedUserIDs))
	for _, id := range suggestedUserIDs {
		suggestedUser, err := s.userManager.GetUserByID(ctx, id)
		if err != nil {
			s.logger.Error("Failed to get suggested user", err, "userId", userID, "suggestedId", id)
			continue // Skip this suggested user and continue
		}

		publicUser := suggestedUser.ToPublicUser()

		// Check if user is online
		isOnline, err := s.userManager.IsUserOnline(ctx, suggestedUser.ID.Hex())
		if err != nil {
			s.logger.Error("Failed to check if user is online", err, "userId", suggestedUser.ID.Hex())
			// Continue anyway, default to false
		} else {
			publicUser.Online = isOnline
		}

		suggestedUsers = append(suggestedUsers, &publicUser)
	}

	return suggestedUsers, nil
}
