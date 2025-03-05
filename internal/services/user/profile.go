// Package user provides services for user management and operations.
package user

import (
	"context"

	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// ProfileService provides functionality for managing user profiles.
type ProfileService struct {
	userManager *Manager
	logger      *utils.Logger
}

// NewProfileService creates a new profile service.
func NewProfileService(userManager *Manager, logger *utils.Logger) *ProfileService {
	return &ProfileService{
		userManager: userManager,
		logger:      logger.Named("profile_service"),
	}
}

// GetProfile retrieves a user's profile.
func (s *ProfileService) GetProfile(ctx context.Context, userID string) (*models.UserProfile, error) {
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user.Profile, nil
}

// UpdateProfile updates a user's profile information.
func (s *ProfileService) UpdateProfile(ctx context.Context, userID string, profile models.UserProfile) error {
	// Get current user
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// Preserve join date
	joinDate := user.Profile.JoinDate
	user.Profile = profile
	user.Profile.JoinDate = joinDate
	user.UpdateNow()

	// Create update request
	updateReq := models.UserUpdateRequest{
		Profile: &profile,
	}

	// Update user
	_, err = s.userManager.UpdateUser(ctx, userID, updateReq)
	if err != nil {
		s.logger.Error("Failed to update user profile", err, "userId", userID)
		return err
	}

	return nil
}

// UpdateStatus updates a user's status message.
func (s *ProfileService) UpdateStatus(ctx context.Context, userID string, status string) error {
	// Get current profile
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return err
	}

	// Update status
	profile.Status = status

	// Update profile
	return s.UpdateProfile(ctx, userID, *profile)
}

// UpdateLanguage updates a user's preferred language.
func (s *ProfileService) UpdateLanguage(ctx context.Context, userID string, language string) error {
	// Get current profile
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return err
	}

	// Update language
	profile.Language = language

	// Update profile
	return s.UpdateProfile(ctx, userID, *profile)
}

// UpdateBio updates a user's biography.
func (s *ProfileService) UpdateBio(ctx context.Context, userID string, bio string) error {
	// Get current profile
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return err
	}

	// Update bio
	profile.Bio = bio

	// Update profile
	return s.UpdateProfile(ctx, userID, *profile)
}

// UpdateLocation updates a user's location.
func (s *ProfileService) UpdateLocation(ctx context.Context, userID string, location string) error {
	// Get current profile
	profile, err := s.GetProfile(ctx, userID)
	if err != nil {
		return err
	}

	// Update location
	profile.Location = location

	// Update profile
	return s.UpdateProfile(ctx, userID, *profile)
}

// GetPublicProfile retrieves a user's public profile by ID.
func (s *ProfileService) GetPublicProfile(ctx context.Context, userID string) (*models.PublicUser, error) {
	return s.userManager.GetPublicUserByID(ctx, userID)
}

// GetPublicProfileByUsername retrieves a user's public profile by username.
func (s *ProfileService) GetPublicProfileByUsername(ctx context.Context, username string) (*models.PublicUser, error) {
	return s.userManager.GetPublicUserByUsername(ctx, username)
}

// SearchProfiles searches for user profiles by username or other criteria.
func (s *ProfileService) SearchProfiles(ctx context.Context, query string, skip, limit int) ([]*models.PublicUser, error) {
	return s.userManager.SearchUsers(ctx, query, skip, limit)
}

// GetUserSettings retrieves a user's settings.
func (s *ProfileService) GetUserSettings(ctx context.Context, userID string) (*models.UserSettings, error) {
	user, err := s.userManager.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &user.Settings, nil
}

// UpdateUserSettings updates a user's settings.
func (s *ProfileService) UpdateUserSettings(ctx context.Context, userID string, settings models.UserSettings) error {
	// Create update request
	updateReq := models.UserUpdateRequest{
		Settings: &settings,
	}

	// Update user
	_, err := s.userManager.UpdateUser(ctx, userID, updateReq)
	if err != nil {
		s.logger.Error("Failed to update user settings", err, "userId", userID)
		return err
	}

	return nil
}

// UpdateNotificationSettings updates a user's notification settings.
func (s *ProfileService) UpdateNotificationSettings(ctx context.Context, userID string, enableNotifications bool, notificationTypes map[string]bool) error {
	// Get current settings
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update notification settings
	settings.EnableNotifications = enableNotifications
	if notificationTypes != nil {
		settings.NotificationTypes = notificationTypes
	}

	// Update settings
	return s.UpdateUserSettings(ctx, userID, *settings)
}

// UpdateTheme updates a user's theme preference.
func (s *ProfileService) UpdateTheme(ctx context.Context, userID string, theme string) error {
	// Get current settings
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update theme
	settings.Theme = theme

	// Update settings
	return s.UpdateUserSettings(ctx, userID, *settings)
}

// UpdateVideoSize updates a user's video size preference.
func (s *ProfileService) UpdateVideoSize(ctx context.Context, userID string, size string) error {
	// Get current settings
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update video size
	settings.VideoSize = size

	// Update settings
	return s.UpdateUserSettings(ctx, userID, *settings)
}

// UpdateVolume updates a user's volume preference.
func (s *ProfileService) UpdateVolume(ctx context.Context, userID string, volume int) error {
	// Get current settings
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update volume
	settings.Volume = volume

	// Update settings
	return s.UpdateUserSettings(ctx, userID, *settings)
}

// UpdateChatSettings updates a user's chat-related settings.
func (s *ProfileService) UpdateChatSettings(ctx context.Context, userID string, showChatImages, chatMentions, languageFilter bool) error {
	// Get current settings
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update chat settings
	settings.ShowChatImages = showChatImages
	settings.ChatMentions = chatMentions
	settings.LanguageFilter = languageFilter

	// Update settings
	return s.UpdateUserSettings(ctx, userID, *settings)
}

// UpdateDJSettings updates a user's DJ-related settings.
func (s *ProfileService) UpdateDJSettings(ctx context.Context, userID string, autoJoinDJ, autoWoot bool) error {
	// Get current settings
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update DJ settings
	settings.AutoJoinDJ = autoJoinDJ
	settings.AutoWoot = autoWoot

	// Update settings
	return s.UpdateUserSettings(ctx, userID, *settings)
}

// UpdateHideAudience updates a user's hide audience preference.
func (s *ProfileService) UpdateHideAudience(ctx context.Context, userID string, hideAudience bool) error {
	// Get current settings
	settings, err := s.GetUserSettings(ctx, userID)
	if err != nil {
		return err
	}

	// Update hide audience setting
	settings.HideAudience = hideAudience

	// Update settings
	return s.UpdateUserSettings(ctx, userID, *settings)
}
