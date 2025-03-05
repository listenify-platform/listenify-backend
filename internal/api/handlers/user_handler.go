// Package handlers contains HTTP handlers for the API.
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// UserHandler handles HTTP requests related to user operations.
type UserHandler struct {
	userManager   *user.Manager
	socialService *user.SocialService
	logger        *utils.Logger
}

// NewUserHandler creates a new user handler.
func NewUserHandler(userManager *user.Manager, logger *utils.Logger) *UserHandler {
	return &UserHandler{
		userManager:   userManager,
		socialService: user.NewSocialService(userManager, logger),
		logger:        logger.Named("user_handler"),
	}
}

// GetUser handles requests to get a user by ID.
func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID from URL parameter
	idStr := chi.URLParam(r, "id")
	if idStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	// Get public user
	publicUser, err := h.userManager.GetPublicUserByID(r.Context(), idStr)
	if err != nil {
		h.logger.Error("Failed to get user", err, "id", idStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, publicUser)
}

// UpdateUser handles requests to update a user's profile.
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Parse request body
	var req models.UserUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		utils.RespondWithValidationError(w, err)
		return
	}

	// Update user
	user, err := h.userManager.UpdateUser(r.Context(), userIDStr, req)
	if err != nil {
		h.logger.Error("Failed to update user", err, "id", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	// Convert to personal user
	personalUser := user.ToPersonalUser()

	utils.RespondWithJSON(w, http.StatusOK, personalUser)
}

// DeleteUser handles requests to delete a user's account.
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Delete user
	err := h.userManager.DeleteAccount(r.Context(), userIDStr)
	if err != nil {
		h.logger.Error("Failed to delete user", err, "id", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "User account deleted successfully",
	})
}

// SearchUsers handles requests to search for users.
func (h *UserHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	query := r.URL.Query().Get("q")
	if query == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	pageStr := r.URL.Query().Get("page")
	page := 1 // Default page
	if pageStr != "" {
		var err error
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid page parameter")
			return
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 50 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}

	// Calculate skip
	skip := (page - 1) * limit

	// Search for users
	users, err := h.userManager.SearchUsers(r.Context(), query, skip, limit)
	if err != nil {
		h.logger.Error("Failed to search for users", err, "query", query)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to search for users")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, users)
}

// GetOnlineUsers handles requests to get online users.
func (h *UserHandler) GetOnlineUsers(w http.ResponseWriter, r *http.Request) {
	// Get online users
	users, err := h.userManager.GetOnlineUsers(r.Context())
	if err != nil {
		h.logger.Error("Failed to get online users", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get online users")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, users)
}

// ChangePassword handles requests to change a user's password.
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Parse request body
	var req models.UserPasswordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		utils.RespondWithValidationError(w, err)
		return
	}

	// Change password
	err := h.userManager.ChangePassword(r.Context(), userIDStr, req)
	if err != nil {
		h.logger.Error("Failed to change password", err, "id", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to change password")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Password changed successfully",
	})
}

// VerifyEmail handles requests to verify a user's email.
func (h *UserHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Verify email
	err := h.userManager.VerifyUserEmail(r.Context(), userIDStr)
	if err != nil {
		h.logger.Error("Failed to verify email", err, "id", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to verify email")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "Email verified successfully",
	})
}

// GetUserCount handles requests to get the total number of users.
func (h *UserHandler) GetUserCount(w http.ResponseWriter, r *http.Request) {
	// Get user count
	count, err := h.userManager.GetUserCount(r.Context())
	if err != nil {
		h.logger.Error("Failed to get user count", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get user count")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"count": count,
	})
}

// ActivateUser handles requests to activate a user (admin only).
func (h *UserHandler) ActivateUser(w http.ResponseWriter, r *http.Request) {
	// Get target user ID from URL parameter
	targetIDStr := chi.URLParam(r, "id")
	if targetIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Target user ID is required")
		return
	}

	// Activate user
	err := h.userManager.ReactivateAccount(r.Context(), targetIDStr)
	if err != nil {
		h.logger.Error("Failed to activate user", err, "targetID", targetIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to activate user")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "User activated successfully",
	})
}

// DeactivateUser handles requests to deactivate a user (admin only).
func (h *UserHandler) DeactivateUser(w http.ResponseWriter, r *http.Request) {
	// Get target user ID from URL parameter
	targetIDStr := chi.URLParam(r, "id")
	if targetIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Target user ID is required")
		return
	}

	// Deactivate user
	err := h.userManager.DeactivateAccount(r.Context(), targetIDStr)
	if err != nil {
		h.logger.Error("Failed to deactivate user", err, "targetID", targetIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to deactivate user")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "User deactivated successfully",
	})
}

// AdminDeleteUser handles requests to delete a user (admin only).
func (h *UserHandler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	// Get target user ID from URL parameter
	targetIDStr := chi.URLParam(r, "id")
	if targetIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Target user ID is required")
		return
	}

	// Delete user
	err := h.userManager.DeleteAccount(r.Context(), targetIDStr)
	if err != nil {
		h.logger.Error("Failed to delete user", err, "targetID", targetIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "User deleted successfully",
	})
}

// GetFollowing handles requests to get the list of users that the current user follows.
func (h *UserHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Parse query parameters
	pageStr := r.URL.Query().Get("page")
	page := 1 // Default page
	if pageStr != "" {
		var err error
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid page parameter")
			return
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 50 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}

	// Calculate skip
	skip := (page - 1) * limit

	// Get following
	following, err := h.socialService.GetFollowing(r.Context(), userIDStr, skip, limit)
	if err != nil {
		h.logger.Error("Failed to get following", err, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get following")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, following)
}

// GetFollowers handles requests to get the list of users who follow the current user.
func (h *UserHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Parse query parameters
	pageStr := r.URL.Query().Get("page")
	page := 1 // Default page
	if pageStr != "" {
		var err error
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid page parameter")
			return
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 50 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}

	// Calculate skip
	skip := (page - 1) * limit

	// Get followers
	followers, err := h.socialService.GetFollowers(r.Context(), userIDStr, skip, limit)
	if err != nil {
		h.logger.Error("Failed to get followers", err, "userID", userIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get followers")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, followers)
}

// FollowUser handles requests to follow another user.
func (h *UserHandler) FollowUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Get target user ID from URL parameter
	targetIDStr := chi.URLParam(r, "id")
	if targetIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Target user ID is required")
		return
	}

	// Follow user
	err := h.socialService.FollowUser(r.Context(), userIDStr, targetIDStr)
	if err != nil {
		h.logger.Error("Failed to follow user", err, "userID", userIDStr, "targetID", targetIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to follow user")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "User followed successfully",
	})
}

// UnfollowUser handles requests to unfollow another user.
func (h *UserHandler) UnfollowUser(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context
	userIDStr := r.Context().Value("userID").(string)

	// Get target user ID from URL parameter
	targetIDStr := chi.URLParam(r, "id")
	if targetIDStr == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Target user ID is required")
		return
	}

	// Unfollow user
	err := h.socialService.UnfollowUser(r.Context(), userIDStr, targetIDStr)
	if err != nil {
		h.logger.Error("Failed to unfollow user", err, "userID", userIDStr, "targetID", targetIDStr)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to unfollow user")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "User unfollowed successfully",
	})
}

// GetAllUsers handles requests to get all users (admin only).
func (h *UserHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	pageStr := r.URL.Query().Get("page")
	page := 1 // Default page
	if pageStr != "" {
		var err error
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid page parameter")
			return
		}
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20 // Default limit
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 50 {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
			return
		}
	}

	// Calculate skip
	skip := (page - 1) * limit

	// Get all users
	users, err := h.userManager.SearchUsers(r.Context(), "", skip, limit)
	if err != nil {
		h.logger.Error("Failed to get all users", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get all users")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, users)
}
