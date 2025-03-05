// Package user provides services for user management and operations.
package user

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/auth"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// Manager provides user management functionality.
type Manager struct {
	userRepo     repositories.UserRepository
	sessionMgr   managers.SessionManager
	presenceMgr  managers.PresenceManager
	authProvider auth.Provider
	logger       *utils.Logger
	avatarSvc    *AvatarService
}

// NewManager creates a new user manager.
func NewManager(
	userRepo repositories.UserRepository,
	sessionMgr managers.SessionManager,
	presenceMgr managers.PresenceManager,
	authProvider auth.Provider,
	logger *utils.Logger,
) *Manager {
	return &Manager{
		userRepo:     userRepo,
		sessionMgr:   sessionMgr,
		presenceMgr:  presenceMgr,
		authProvider: authProvider,
		logger:       logger.Named("user_manager"),
		avatarSvc:    NewAvatarService(logger),
	}
}

// Register creates a new user account.
func (m *Manager) Register(ctx context.Context, req models.UserRegisterRequest) (*models.User, string, error) {
	// Check if email already exists
	_, err := m.userRepo.FindByEmail(ctx, req.Email)
	if err == nil {
		return nil, "", models.ErrEmailAlreadyExists
	} else if !errors.Is(err, models.ErrUserNotFound) {
		m.logger.Error("Error checking email existence", err, "email", req.Email)
		return nil, "", err
	}

	// Check if username already exists
	_, err = m.userRepo.FindByUsername(ctx, req.Username)
	if err == nil {
		return nil, "", models.ErrUsernameAlreadyExists
	} else if !errors.Is(err, models.ErrUserNotFound) {
		m.logger.Error("Error checking username existence", err, "username", req.Username)
		return nil, "", err
	}

	// Hash password
	hashedPassword, err := m.authProvider.HashPassword(req.Password)
	if err != nil {
		m.logger.Error("Failed to hash password", err)
		return nil, "", models.NewInternalError(err, "Failed to process password")
	}

	// Create user
	now := time.Now()
	baseUser := models.BaseUser{
		ID:           bson.NewObjectID(),
		Username:     req.Username,
		AvatarConfig: m.avatarSvc.GenerateDefaultAvatar(),
		Profile: models.UserProfile{
			JoinDate: now,
			Language: "en", // Default language
			Status:   "Just joined!",
			Social:   models.UserSocial{},
		},
		Stats: models.UserStats{
			Level:       1,
			Experience:  0,
			Points:      0,
			LastUpdated: now,
		},
		Badges: []string{},
		Roles:  []string{"user"}, // Default role
	}
	user := &models.User{
		BaseUser:    baseUser,
		Email:       req.Email,
		Password:    hashedPassword,
		IsActive:    true,
		IsVerified:  false, // Requires email verification
		LastLogin:   now,
		ObjectTimes: models.NewObjectTimes(now),
		Connections: models.UserConnections{},
		Settings: models.UserSettings{
			Theme:               "dark",
			AutoJoinDJ:          false,
			AutoWoot:            false,
			ShowChatImages:      true,
			EnableNotifications: true,
			NotificationTypes:   map[string]bool{"mention": true, "follow": true},
			ChatMentions:        true,
			Volume:              50,
			HideAudience:        false,
			VideoSize:           "medium",
			LanguageFilter:      true,
		},
	}

	// Save user to database
	if err := m.userRepo.Create(ctx, user); err != nil {
		m.logger.Error("Failed to create user", err, "email", req.Email)
		return nil, "", err
	}

	// Generate JWT token
	token, err := m.authProvider.GenerateToken(user.ID.Hex(), user.Username, user.Roles)
	if err != nil {
		m.logger.Error("Failed to generate token", err, "userId", user.ID.Hex())
		return nil, "", models.NewInternalError(err, "Failed to generate authentication token")
	}

	// Create session
	_, err = m.sessionMgr.CreateSession(ctx, user, token, "unknown", "unknown") // IP and user agent not available here
	if err != nil {
		m.logger.Error("Failed to create session", err, "userId", user.ID.Hex())
		// Continue anyway, user can log in again
	}

	return user, token, nil
}

// Login authenticates a user and returns a JWT token.
func (m *Manager) Login(ctx context.Context, req models.UserLoginRequest) (*models.User, string, error) {
	// Find user by email
	user, err := m.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, "", models.ErrInvalidCredentials
		}
		m.logger.Error("Failed to find user by email", err, "email", req.Email)
		return nil, "", err
	}

	// Check if user is active
	if !user.IsActive {
		return nil, "", models.ErrAccountDisabled
	}

	// Verify password
	if !m.authProvider.VerifyPassword(req.Password, user.Password) {
		return nil, "", models.ErrInvalidCredentials
	}

	// Update last login
	if err := m.userRepo.UpdateLastLogin(ctx, user.ID); err != nil {
		m.logger.Error("Failed to update last login", err, "userId", user.ID.Hex())
		// Continue anyway, not critical
	}

	// Generate JWT token
	token, err := m.authProvider.GenerateToken(user.ID.Hex(), user.Username, user.Roles)
	if err != nil {
		m.logger.Error("Failed to generate token", err, "userId", user.ID.Hex())
		return nil, "", models.NewInternalError(err, "Failed to generate authentication token")
	}

	// Create session
	_, err = m.sessionMgr.CreateSession(ctx, user, token, "unknown", "unknown") // IP and user agent not available here
	if err != nil {
		m.logger.Error("Failed to create session", err, "userId", user.ID.Hex())
		// Continue anyway, user can log in again
	}

	// Set user as online
	if err := m.presenceMgr.UpdatePresence(ctx, user.ID, user.Username, "online"); err != nil {
		m.logger.Error("Failed to set user online", err, "userId", user.ID.Hex())
		// Continue anyway, not critical
	}

	return user, token, nil
}

// Logout invalidates a user's session.
func (m *Manager) Logout(ctx context.Context, userID string) error {
	// Get token for user (we need the token to destroy the session)
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Get user session
	_, token, err := m.sessionMgr.GetUserSession(ctx, objectID)
	if err != nil {
		m.logger.Error("Failed to get user session", err, "userId", userID)
		return models.NewInternalError(err, "Failed to logout")
	}

	// Destroy session
	if err := m.sessionMgr.DestroySession(ctx, token); err != nil {
		m.logger.Error("Failed to destroy session", err, "userId", userID)
		return models.NewInternalError(err, "Failed to logout")
	}

	// Set user as offline
	if err := m.presenceMgr.RemovePresence(ctx, objectID); err != nil {
		m.logger.Error("Failed to set user offline", err, "userId", userID)
		// Continue anyway, not critical
	}

	return nil
}

// GetUserByID retrieves a user by their ID.
func (m *Manager) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, models.ErrInvalidID
	}

	user, err := m.userRepo.FindByID(ctx, objectID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, models.ErrUserNotFound
		}
		m.logger.Error("Failed to get user by ID", err, "userId", id)
		return nil, models.NewInternalError(err, "Failed to retrieve user")
	}

	return user, nil
}

// GetUserByUsername retrieves a user by their username.
func (m *Manager) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user, err := m.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, models.ErrUserNotFound
		}
		m.logger.Error("Failed to get user by username", err, "username", username)
		return nil, models.NewInternalError(err, "Failed to retrieve user")
	}

	return user, nil
}

// GetPublicUserByID retrieves a public user profile by ID.
func (m *Manager) GetPublicUserByID(ctx context.Context, id string) (*models.PublicUser, error) {
	user, err := m.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	publicUser := user.ToPublicUser()

	// Check if user is online
	var isOnline bool
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		m.logger.Error("Failed to convert ID to ObjectID", err, "userId", id)
		// Continue anyway, default to false
	} else {
		var checkErr error
		isOnline, checkErr = m.presenceMgr.IsUserOnline(ctx, objectID)
		if checkErr != nil {
			m.logger.Error("Failed to check if user is online", checkErr, "userId", id)
			// Continue anyway, default to false
		}
	}
	publicUser.Online = isOnline

	return &publicUser, nil
}

// GetPublicUserByUsername retrieves a public user profile by username.
func (m *Manager) GetPublicUserByUsername(ctx context.Context, username string) (*models.PublicUser, error) {
	user, err := m.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	publicUser := user.ToPublicUser()

	// Check if user is online
	isOnline, err := m.presenceMgr.IsUserOnline(ctx, user.ID)
	if err != nil {
		m.logger.Error("Failed to check if user is online", err, "userId", user.ID.Hex())
		// Continue anyway, default to false
	} else {
		publicUser.Online = isOnline
	}

	return &publicUser, nil
}

// UpdateUser updates a user's profile information.
func (m *Manager) UpdateUser(ctx context.Context, userID string, req models.UserUpdateRequest) (*models.User, error) {
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, models.ErrInvalidID
	}

	// Get current user
	user, err := m.userRepo.FindByID(ctx, objectID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, models.ErrUserNotFound
		}
		m.logger.Error("Failed to get user for update", err, "userId", userID)
		return nil, models.NewInternalError(err, "Failed to retrieve user")
	}

	// Update username if provided
	if req.Username != "" && req.Username != user.Username {
		// Check if username already exists
		_, err = m.userRepo.FindByUsername(ctx, req.Username)
		if err == nil {
			return nil, models.ErrUsernameAlreadyExists
		} else if !errors.Is(err, models.ErrUserNotFound) {
			m.logger.Error("Error checking username existence", err, "username", req.Username)
			return nil, err
		}

		user.Username = req.Username
	}

	// Update avatar if provided
	if req.AvatarConfig != nil {
		user.AvatarConfig = *req.AvatarConfig
	}

	// Update profile if provided
	if req.Profile != nil {
		// Preserve join date
		joinDate := user.Profile.JoinDate
		user.Profile = *req.Profile
		user.Profile.JoinDate = joinDate
	}

	// Update settings if provided
	if req.Settings != nil {
		user.Settings = *req.Settings
	}

	// Save changes
	if err := m.userRepo.Update(ctx, user); err != nil {
		m.logger.Error("Failed to update user", err, "userId", userID)
		return nil, err
	}

	return user, nil
}

// ChangePassword changes a user's password.
func (m *Manager) ChangePassword(ctx context.Context, userID string, req models.UserPasswordChangeRequest) error {
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Get current user
	user, err := m.userRepo.FindByID(ctx, objectID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return models.ErrUserNotFound
		}
		m.logger.Error("Failed to get user for password change", err, "userId", userID)
		return models.NewInternalError(err, "Failed to retrieve user")
	}

	// Verify current password
	if !m.authProvider.VerifyPassword(req.CurrentPassword, user.Password) {
		return models.ErrInvalidCredentials
	}

	// Hash new password
	hashedPassword, err := m.authProvider.HashPassword(req.NewPassword)
	if err != nil {
		m.logger.Error("Failed to hash new password", err, "userId", userID)
		return models.NewInternalError(err, "Failed to process password")
	}

	// Update password
	user.Password = hashedPassword
	user.UpdateNow()

	// Save changes
	if err := m.userRepo.Update(ctx, user); err != nil {
		m.logger.Error("Failed to update password", err, "userId", userID)
		return err
	}

	// Invalidate all sessions
	// objectID already defined above
	if err := m.sessionMgr.DestroyUserSessions(ctx, objectID); err != nil {
		m.logger.Error("Failed to invalidate sessions after password change", err, "userId", userID)
		// Continue anyway, not critical
	}

	return nil
}

// DeactivateAccount deactivates a user's account.
func (m *Manager) DeactivateAccount(ctx context.Context, userID string) error {
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Set user as inactive
	if err := m.userRepo.SetActive(ctx, objectID, false); err != nil {
		m.logger.Error("Failed to deactivate account", err, "userId", userID)
		return err
	}

	// Invalidate all sessions
	if err := m.sessionMgr.DestroyUserSessions(ctx, objectID); err != nil {
		m.logger.Error("Failed to invalidate sessions after account deactivation", err, "userId", userID)
		// Continue anyway, not critical
	}

	// Set user as offline
	if err := m.presenceMgr.RemovePresence(ctx, objectID); err != nil {
		m.logger.Error("Failed to set user offline", err, "userId", userID)
		// Continue anyway, not critical
	}

	return nil
}

// ReactivateAccount reactivates a user's account.
func (m *Manager) ReactivateAccount(ctx context.Context, userID string) error {
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Set user as active
	if err := m.userRepo.SetActive(ctx, objectID, true); err != nil {
		m.logger.Error("Failed to reactivate account", err, "userId", userID)
		return err
	}

	return nil
}

// DeleteAccount permanently deletes a user's account.
func (m *Manager) DeleteAccount(ctx context.Context, userID string) error {
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Delete user
	if err := m.userRepo.Delete(ctx, objectID); err != nil {
		m.logger.Error("Failed to delete account", err, "userId", userID)
		return err
	}

	// Invalidate all sessions
	if err := m.sessionMgr.DestroyUserSessions(ctx, objectID); err != nil {
		m.logger.Error("Failed to invalidate sessions after account deletion", err, "userId", userID)
		// Continue anyway, not critical
	}

	// Set user as offline
	if err := m.presenceMgr.RemovePresence(ctx, objectID); err != nil {
		m.logger.Error("Failed to set user offline", err, "userId", userID)
		// Continue anyway, not critical
	}

	return nil
}

// SearchUsers searches for users by username or other criteria.
func (m *Manager) SearchUsers(ctx context.Context, query string, skip, limit int) ([]*models.PublicUser, error) {
	// Create filter for username search (case-insensitive)
	filter := bson.M{
		"username": bson.M{
			"$regex":   query,
			"$options": "i", // Case-insensitive
		},
		"isActive": true,
	}

	// Find users
	users, err := m.userRepo.FindMany(ctx, filter, nil)
	if err != nil {
		m.logger.Error("Failed to search users", err, "query", query)
		return nil, models.NewInternalError(err, "Failed to search users")
	}

	// Convert to public users
	publicUsers := make([]*models.PublicUser, 0, len(users))
	for _, user := range users {
		publicUser := user.ToPublicUser()

		// Check if user is online
		isOnline, err := m.presenceMgr.IsUserOnline(ctx, user.ID)
		if err != nil {
			m.logger.Error("Failed to check if user is online", err, "userId", user.ID.Hex())
			// Continue anyway, default to false
		} else {
			publicUser.Online = isOnline
		}

		publicUsers = append(publicUsers, &publicUser)
	}

	return publicUsers, nil
}

// GetOnlineUsers gets a list of currently online users.
func (m *Manager) GetOnlineUsers(ctx context.Context) ([]*models.PublicUser, error) {
	// Get online user IDs
	onlineIDs, err := m.presenceMgr.GetOnlineUsers(ctx)
	if err != nil {
		m.logger.Error("Failed to get online users", err)
		return nil, models.NewInternalError(err, "Failed to get online users")
	}

	if len(onlineIDs) == 0 {
		return []*models.PublicUser{}, nil
	}

	// Convert string IDs to ObjectIDs
	objectIDs := make([]bson.ObjectID, 0, len(onlineIDs))
	for _, id := range onlineIDs {
		objectID, err := bson.ObjectIDFromHex(id)
		if err != nil {
			m.logger.Error("Invalid user ID in online users", err, "userId", id)
			continue
		}
		objectIDs = append(objectIDs, objectID)
	}

	// Find users
	filter := bson.M{
		"_id":      bson.M{"$in": objectIDs},
		"isActive": true,
	}
	users, err := m.userRepo.FindMany(ctx, filter, nil)
	if err != nil {
		m.logger.Error("Failed to get online users", err)
		return nil, models.NewInternalError(err, "Failed to get online users")
	}

	// Convert to public users
	publicUsers := make([]*models.PublicUser, 0, len(users))
	for _, user := range users {
		publicUser := user.ToPublicUser()
		publicUser.Online = true // We know they're online
		publicUsers = append(publicUsers, &publicUser)
	}

	return publicUsers, nil
}

// VerifyUserEmail marks a user's email as verified.
func (m *Manager) VerifyUserEmail(ctx context.Context, userID string) error {
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return models.ErrInvalidID
	}

	// Set user as verified
	if err := m.userRepo.SetVerified(ctx, objectID, true); err != nil {
		m.logger.Error("Failed to verify user email", err, "userId", userID)
		return err
	}

	return nil
}

// IsUserOnline checks if a user is currently online.
func (m *Manager) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return false, models.ErrInvalidID
	}
	return m.presenceMgr.IsUserOnline(ctx, objectID)
}

// GetUserCount gets the total number of users.
func (m *Manager) GetUserCount(ctx context.Context) (int64, error) {
	return m.userRepo.CountUsers(ctx, bson.M{"isActive": true})
}
