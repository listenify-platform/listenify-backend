// Package methods contains RPC method handlers for the application.
package methods

import (
	"context"
	"errors"

	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// UserHandler handles user-related RPC methods.
type UserHandler struct {
	userManager  user.Manager
	statsService *user.StatsService
	logger       *utils.Logger
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(userManager user.Manager, statsService *user.StatsService, logger *utils.Logger) *UserHandler {
	return &UserHandler{
		userManager:  userManager,
		statsService: statsService,
		logger:       logger,
	}
}

// RegisterMethods registers user-related RPC methods with the router.
func (h *UserHandler) RegisterMethods(hr rpc.HandlerRegistry) {
	auth := hr.Wrap(rpc.AuthMiddleware)
	rpc.Register(hr, "user.login", h.Login)
	rpc.Register(hr, "user.register", h.Register)
	rpc.RegisterNoParams(auth, "user.logout", h.Logout)
	rpc.Register(hr, "user.getProfile", h.GetProfile)
	rpc.Register(auth, "user.updateProfile", h.UpdateProfile)
	rpc.Register(auth, "user.changePassword", h.ChangePassword)
	rpc.RegisterNoParams(hr, "user.getOnlineUsers", h.GetOnlineUsers)
	rpc.Register(hr, "user.searchUsers", h.SearchUsers)

	// Stats methods
	rpc.Register(hr, "user.getStats", h.GetUserStats)
	rpc.Register(hr, "user.getTopUsers", h.GetTopUsers)
	rpc.Register(hr, "user.getRank", h.GetUserRank)
	rpc.Register(hr, "user.getExperienceProgress", h.GetExperienceProgress)
}

// GetUserStats handles retrieving a user's statistics.
func (h *UserHandler) GetUserStats(ctx context.Context, client *rpc.Client, p *UserIDParam) (any, error) {
	// If no user ID is provided, use the authenticated user's ID
	userID := p.UserID
	if userID == "" {
		userID = client.UserID
		if userID == "" {
			return nil, &rpc.Error{
				Code:    rpc.ErrAuthenticationRequired,
				Message: "Authentication required",
			}
		}
	}

	// Get user stats
	stats, err := h.statsService.GetUserStats(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "User not found",
			}
		}
		h.logger.Error("Failed to get user stats", err, "userID", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get user stats",
		}
	}

	// Get experience for next level
	nextLevelExp, err := h.statsService.GetExperienceForNextLevel(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get experience for next level", err, "userID", userID)
		// Continue anyway, we'll just return the stats without next level info
	}

	// Get experience progress
	progress, err := h.statsService.GetExperienceProgress(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get experience progress", err, "userID", userID)
		// Continue anyway, we'll just return the stats without progress info
	}

	// Get user rank
	rank, err := h.statsService.GetUserRank(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user rank", err, "userID", userID)
		// Continue anyway, we'll just return the stats without rank info
	}

	// Create response with additional info
	response := struct {
		Stats            *models.UserStats `json:"stats"`
		NextLevelExp     int               `json:"nextLevelExp,omitempty"`
		ExperienceNeeded int               `json:"experienceNeeded,omitempty"`
		Progress         float64           `json:"progress,omitempty"`
		Rank             int               `json:"rank,omitempty"`
	}{
		Stats:    stats,
		Progress: progress,
		Rank:     rank,
	}

	if nextLevelExp > 0 {
		response.NextLevelExp = nextLevelExp
		response.ExperienceNeeded = nextLevelExp - stats.Experience
	}

	return response, nil
}

// GetTopUsersParams represents the parameters for the getTopUsers method.
type GetTopUsersParams struct {
	Limit int `json:"limit,omitempty"`
}

// GetTopUsers handles retrieving the top users by experience points.
func (h *UserHandler) GetTopUsers(ctx context.Context, client *rpc.Client, p *GetTopUsersParams) (any, error) {
	// Set default limit if not provided
	if p.Limit <= 0 {
		p.Limit = 10
	}

	// Get top users
	users, err := h.statsService.GetTopUsers(ctx, p.Limit)
	if err != nil {
		h.logger.Error("Failed to get top users", err)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get top users",
		}
	}

	// Convert pointers to values
	publicUsers := make([]models.PublicUser, len(users))
	for i, user := range users {
		publicUsers[i] = *user
	}

	// Return top users
	return struct {
		Users []models.PublicUser `json:"users"`
	}{
		Users: publicUsers,
	}, nil
}

// GetUserRank handles retrieving a user's rank based on experience points.
func (h *UserHandler) GetUserRank(ctx context.Context, client *rpc.Client, p *UserIDParam) (any, error) {
	// If no user ID is provided, use the authenticated user's ID
	userID := p.UserID
	if userID == "" {
		userID = client.UserID
		if userID == "" {
			return nil, &rpc.Error{
				Code:    rpc.ErrAuthenticationRequired,
				Message: "Authentication required",
			}
		}
	}

	// Get user rank
	rank, err := h.statsService.GetUserRank(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "User not found",
			}
		}
		h.logger.Error("Failed to get user rank", err, "userID", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get user rank",
		}
	}

	// Return user rank
	return struct {
		Rank int `json:"rank"`
	}{
		Rank: rank,
	}, nil
}

// GetExperienceProgress handles retrieving a user's progress towards the next level.
func (h *UserHandler) GetExperienceProgress(ctx context.Context, client *rpc.Client, p *UserIDParam) (any, error) {
	// If no user ID is provided, use the authenticated user's ID
	userID := p.UserID
	if userID == "" {
		userID = client.UserID
		if userID == "" {
			return nil, &rpc.Error{
				Code:    rpc.ErrAuthenticationRequired,
				Message: "Authentication required",
			}
		}
	}

	// Get user stats
	stats, err := h.statsService.GetUserStats(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "User not found",
			}
		}
		h.logger.Error("Failed to get user stats", err, "userID", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get user stats",
		}
	}

	// Get experience for next level
	nextLevelExp, err := h.statsService.GetExperienceForNextLevel(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get experience for next level", err, "userID", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get experience for next level",
		}
	}

	// Get experience progress
	progress, err := h.statsService.GetExperienceProgress(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get experience progress", err, "userID", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get experience progress",
		}
	}

	// Return progress info
	return struct {
		CurrentLevel     int     `json:"currentLevel"`
		CurrentExp       int     `json:"currentExp"`
		NextLevelExp     int     `json:"nextLevelExp"`
		ExperienceNeeded int     `json:"experienceNeeded"`
		Progress         float64 `json:"progress"`
	}{
		CurrentLevel:     stats.Level,
		CurrentExp:       stats.Experience,
		NextLevelExp:     nextLevelExp,
		ExperienceNeeded: nextLevelExp - stats.Experience,
		Progress:         progress,
	}, nil
}

// LoginParams represents the parameters for the login method.
type LoginParams struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResult represents the result of the login method.
type LoginResult struct {
	User  models.PublicUser `json:"user"`
	Token string            `json:"token"`
}

// Login handles user login.
func (h *UserHandler) Login(ctx context.Context, client *rpc.Client, p *LoginParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Create login request
	req := models.UserLoginRequest{
		Email:    p.Email,
		Password: p.Password,
	}

	// Attempt login
	user, token, err := h.userManager.Login(ctx, req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidCredentials) {
			return nil, &rpc.Error{
				Code:    rpc.ErrNotAuthorized,
				Message: "Invalid email or password",
			}
		}
		h.logger.Error("Failed to login user", err, "email", p.Email)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to login user",
		}
	}

	// Set client user ID
	client.UserID = user.ID.Hex()
	client.Username = user.Username

	// Return user and token
	return LoginResult{
		User:  user.ToPublicUser(),
		Token: token,
	}, nil
}

// RegisterParams represents the parameters for the register method.
type RegisterParams struct {
	Username string `json:"username" validate:"required,min=3,max=20"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// RegisterResult represents the result of the register method.
type RegisterResult struct {
	User  models.PublicUser `json:"user"`
	Token string            `json:"token"`
}

// Register handles user registration.
func (h *UserHandler) Register(ctx context.Context, client *rpc.Client, p *RegisterParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Create registration request
	req := models.UserRegisterRequest{
		Username: p.Username,
		Email:    p.Email,
		Password: p.Password,
	}

	// Attempt registration
	user, token, err := h.userManager.Register(ctx, req)
	if err != nil {
		if errors.Is(err, models.ErrEmailAlreadyExists) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "Email already in use",
			}
		}
		if errors.Is(err, models.ErrUsernameAlreadyExists) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "Username already in use",
			}
		}
		h.logger.Error("Failed to register user", err, "email", p.Email, "username", p.Username)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to register user",
		}
	}

	// Set client user ID
	client.UserID = user.ID.Hex()
	client.Username = user.Username

	// Return user and token
	return RegisterResult{
		User:  user.ToPublicUser(),
		Token: token,
	}, nil
}

// LogoutResult represents the result of the logout method.
type LogoutResult struct {
	Success bool `json:"success"`
}

// Logout handles user logout.
func (h *UserHandler) Logout(ctx context.Context, client *rpc.Client) (any, error) {
	// Attempt logout
	err := h.userManager.Logout(ctx, client.UserID)
	if err != nil {
		h.logger.Error("Failed to logout user", err, "userID", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to logout user",
		}
	}

	// Clear client user ID
	client.UserID = ""
	client.Username = ""

	// Return success
	return LogoutResult{
		Success: true,
	}, nil
}

// GetProfile handles retrieving a user's profile.
func (h *UserHandler) GetProfile(ctx context.Context, client *rpc.Client, p *UserIDParam) (any, error) {
	// If no user ID is provided, use the authenticated user's ID
	userID := p.UserID
	if userID == "" {
		userID = client.UserID
		if userID == "" {
			return nil, &rpc.Error{
				Code:    rpc.ErrAuthenticationRequired,
				Message: "Authentication required",
			}
		}
	}

	// Get user by ID
	user, err := h.userManager.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "User not found",
			}
		}
		h.logger.Error("Failed to get user profile", err, "userID", userID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get user profile",
		}
	}

	// Return public user
	return user.ToPublicUser(), nil
}

// UpdateProfileParams represents the parameters for the updateProfile method.
type UpdateProfileParams struct {
	Username  string `json:"username,omitempty"`
	AvatarURL string `json:"avatarUrl,omitempty"`
	Profile   struct {
		Bio      string `json:"bio,omitempty"`
		Location string `json:"location,omitempty"`
		Website  string `json:"website,omitempty"`
		Status   string `json:"status,omitempty"`
	} `json:"profile"`
}

// UpdateProfile handles updating a user's profile.
func (h *UserHandler) UpdateProfile(ctx context.Context, client *rpc.Client, p *UpdateProfileParams) (any, error) {
	// Create update request
	req := models.UserUpdateRequest{
		Username: p.Username,
	}

	// Add profile if provided
	if p.Profile.Bio != "" || p.Profile.Location != "" || p.Profile.Website != "" || p.Profile.Status != "" {
		profile := models.UserProfile{
			Bio:      p.Profile.Bio,
			Location: p.Profile.Location,
			Website:  p.Profile.Website,
			Status:   p.Profile.Status,
		}
		req.Profile = &profile
	}

	// Update user
	user, err := h.userManager.UpdateUser(ctx, client.UserID, req)
	if err != nil {
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "User not found",
			}
		}
		h.logger.Error("Failed to update user profile", err, "userID", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to update user profile",
		}
	}

	// Return updated public user
	return user.ToPublicUser(), nil
}

// ChangePasswordParams represents the parameters for the changePassword method.
type ChangePasswordParams struct {
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required,min=8"`
}

// ChangePasswordResult represents the result of the changePassword method.
type ChangePasswordResult struct {
	Success bool `json:"success"`
}

// ChangePassword handles changing a user's password.
func (h *UserHandler) ChangePassword(ctx context.Context, client *rpc.Client, p *ChangePasswordParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Create password change request
	req := models.UserPasswordChangeRequest{
		CurrentPassword: p.CurrentPassword,
		NewPassword:     p.NewPassword,
	}

	// Change password
	err := h.userManager.ChangePassword(ctx, client.UserID, req)
	if err != nil {
		if errors.Is(err, models.ErrInvalidCredentials) {
			return nil, &rpc.Error{
				Code:    rpc.ErrNotAuthorized,
				Message: "Current password is incorrect",
			}
		}
		if errors.Is(err, models.ErrUserNotFound) {
			return nil, &rpc.Error{
				Code:    rpc.ErrInvalidParams,
				Message: "User not found",
			}
		}
		h.logger.Error("Failed to change user password", err, "userID", client.UserID)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to change user password",
		}
	}

	// Return success
	return ChangePasswordResult{
		Success: true,
	}, nil
}

// GetOnlineUsersResult represents the result of the getOnlineUsers method.
type GetOnlineUsersResult struct {
	Users []models.PublicUser `json:"users"`
}

// GetOnlineUsers handles retrieving a list of online users.
func (h *UserHandler) GetOnlineUsers(ctx context.Context, client *rpc.Client) (any, error) {
	// Get online users
	users, err := h.userManager.GetOnlineUsers(ctx)
	if err != nil {
		h.logger.Error("Failed to get online users", err)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to get online users",
		}
	}

	// Convert pointers to values
	publicUsers := make([]models.PublicUser, len(users))
	for i, user := range users {
		publicUsers[i] = *user
	}

	// Return online users
	return GetOnlineUsersResult{
		Users: publicUsers,
	}, nil
}

// SearchUsersParams represents the parameters for the searchUsers method.
type SearchUsersParams struct {
	Query string `json:"query" validate:"required,min=2"`
	Skip  int    `json:"skip,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// SearchUsersResult represents the result of the searchUsers method.
type SearchUsersResult struct {
	Users []models.PublicUser `json:"users"`
	Total int                 `json:"total"`
}

// SearchUsers handles searching for users.
func (h *UserHandler) SearchUsers(ctx context.Context, client *rpc.Client, p *SearchUsersParams) (any, error) {
	// Validate parameters
	if err := utils.Validate(p); err != nil {
		return nil, &rpc.Error{
			Code:    rpc.ErrInvalidParams,
			Message: "Invalid parameters",
			Data:    err.Error(),
		}
	}

	// Set default limit if not provided
	if p.Limit <= 0 {
		p.Limit = 20
	}

	// Search users
	users, err := h.userManager.SearchUsers(ctx, p.Query, p.Skip, p.Limit)
	if err != nil {
		h.logger.Error("Failed to search users", err, "query", p.Query)
		return nil, &rpc.Error{
			Code:    rpc.ErrInternalError,
			Message: "Failed to search users",
		}
	}

	// Convert pointers to values
	publicUsers := make([]models.PublicUser, len(users))
	for i, user := range users {
		publicUsers[i] = *user
	}

	// Return search results
	return SearchUsersResult{
		Users: publicUsers,
		Total: len(publicUsers),
	}, nil
}
