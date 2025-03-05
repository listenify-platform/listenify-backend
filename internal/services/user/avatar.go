// Package user provides services for user management and operations.
package user

import (
	"errors"
	"fmt"
	"math/rand/v2"

	"github.com/samber/lo"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

var AVATAR_MAP = map[string]int{
	"2014hw":       15,
	"2014winter-s": 10,
	"80s":          15,
	"base":         15,
	"beach-e":      2,
	"beach-s":      7,
	"beach-t":      4,
	"classic":      11,
	"country":      15,
	"diner-e":      2,
	"diner-s":      10,
	"diner-t":      4,
	"dragon-e":     4,
	"hiphop-s":     2,
	"hiphop":       15,
	"island-e":     2,
	"island-s":     6,
	"island-t":     4,
	"nyc-e":        2,
	"nyc-s":        6,
	"nyc-t":        4,
	"rave":         15,
	"robot-s":      2,
	"robot":        15,
	"rock":         15,
	"sea-e":        2,
	"sea-s":        7,
	"sea-t":        4,
	"warrior-s":    6,
	"warrior":      9,
	"zoo-s":        6,
	"zoo":          15,
}

// AvatarService provides functionality for managing user avatars.
type AvatarService struct {
	//userManager *Manager
	logger *utils.Logger
	// Available options for avatar customization
	available map[string]int
	// Default avatar collection
	defaultCollection string
}

// NewAvatarService creates a new avatar service.
func NewAvatarService(logger *utils.Logger) *AvatarService {
	return &AvatarService{
		//userManager:  userManager,
		logger:            logger.Named("avatar_service"),
		available:         AVATAR_MAP,
		defaultCollection: "base",
	}
}

// GetAvatar retrieves a user's avatar configuration.
// func (s *AvatarService) GetAvatar(ctx context.Context, userID string) (*models.AvatarConfig, error) {
// 	user, err := s.userManager.GetUserByID(ctx, userID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &user.AvatarConfig, nil
// }

// UpdateAvatar updates a user's avatar configuration.
// func (s *AvatarService) UpdateAvatar(ctx context.Context, userID string, avatar models.AvatarConfig) error {
// 	// Validate avatar configuration
// 	if err := s.ValidateAvatarConfig(avatar); err != nil {
// 		return err
// 	}

// 	// Convert string ID to ObjectID
// 	objectID, err := primitive.ObjectIDFromHex(userID)
// 	if err != nil {
// 		return models.ErrInvalidID
// 	}

// 	// Update avatar
// 	if err := s.userManager.userRepo.UpdateAvatar(ctx, objectID, avatar); err != nil {
// 		s.logger.Error("Failed to update avatar", err, "userId", userID)
// 		return err
// 	}

// 	return nil
// }

// ValidateAvatarConfig validates an avatar configuration.
func (s *AvatarService) ValidateAvatarConfig(avatar models.AvatarConfig) error {
	// Validate avatar type
	if avatar.Type != "default" && avatar.Type != "custom" && avatar.Type != "uploaded" {
		return errors.New("invalid avatar type")
	}

	// For default and custom avatars, validate components
	if avatar.Type == "default" || avatar.Type == "custom" {
		// Validate the collection
		if n, ok := s.available[avatar.Collection]; ok {
			// Validate the number
			if avatar.Number < 1 || avatar.Number > n {
				return errors.New("invalid avatar number")
			}
		} else {
			return errors.New("invalid avatar collection")
		}
	}

	// For uploaded avatars, validate CustomImage
	if avatar.Type == "uploaded" && avatar.CustomImage == "" {
		return errors.New("uploaded avatar must have a custom image URL")
	}

	return nil
}

func avatarFrom(collection string, num int) string {
	return fmt.Sprintf("%s%02d", collection, num)
}

// GenerateRandomAvatar generates a random avatar configuration.
func (s *AvatarService) GenerateRandomAvatar() models.AvatarConfig {
	// Randomly select a collection
	collection := lo.Sample(lo.Keys(s.available))

	// Randomly select a number within the collection's range
	number := 1 + rand.IntN(s.available[collection])

	// Generate random avatar
	avatar := models.AvatarConfig{
		Type:       "custom",
		Collection: collection,
		Number:     number,
	}

	return avatar
}

// GenerateDefaultAvatar generates a default avatar configuration based on the username.
func (s *AvatarService) GenerateDefaultAvatar() models.AvatarConfig {

	// Use the default collection
	collection := s.defaultCollection

	// Select the number based on the random number
	num := rand.IntN(s.available[collection])

	// Create avatar
	avatar := models.AvatarConfig{
		Type:       "default",
		Collection: collection,
		Number:     num + 1,
	}

	return avatar
}

// GetAvatarCollections returns the available avatar collections.
func (s *AvatarService) GetAvatarCollections() []string {
	return lo.Keys(s.available)
}

// GetAvatarOptions returns the available options for avatar customization.
func (s *AvatarService) GetAvatarOptions() map[string][]string {
	m := make(map[string][]string, len(s.available))
	for k, v := range s.available {
		a := make([]string, v)
		for i := range v {
			a[i] = avatarFrom(k, i+1)
		}
	}
	return m
}

// GetAvatarURL generates a URL for an avatar.
func (s *AvatarService) GetAvatarURL(avatar models.AvatarConfig) string {
	if avatar.Type == "uploaded" && avatar.CustomImage != "" {
		return avatar.CustomImage
	}

	// For default and custom avatars, generate a URL based on the components
	return fmt.Sprintf("/api/avatars/%s/%d", avatar.Collection, avatar.Number)
}

// UpdateAvatarComponent updates a specific component of a user's avatar.
// func (s *AvatarService) UpdateAvatarComponent(ctx context.Context, userID, component, value string) error {
// 	// Get current avatar
// 	avatar, err := s.GetAvatar(ctx, userID)
// 	if err != nil {
// 		return err
// 	}

// 	// Update component
// 	switch component {
// 	case "type":
// 		avatar.Type = value
// 	case "id":
// 		avatar.ImageID = value
// 	case "customImage":
// 		avatar.CustomImage = value
// 	default:
// 		return errors.New("invalid avatar component")
// 	}

// 	// Validate and update avatar
// 	return s.UpdateAvatar(ctx, userID, *avatar)
// }

// ResetAvatar resets a user's avatar to the default.
// func (s *AvatarService) ResetAvatar(ctx context.Context, userID string) error {
// 	// Get user to access their username
// 	user, err := s.userManager.GetUserByID(ctx, userID)
// 	if err != nil {
// 		return err
// 	}

// 	// Generate default avatar
// 	avatar := s.GenerateDefaultAvatar(user.Username)

// 	// Update avatar
// 	return s.UpdateAvatar(ctx, userID, avatar)
// }
