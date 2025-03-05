// Package handlers contains HTTP handlers for the API.
package handlers

import (
	"encoding/json"
	"net/http"

	"norelock.dev/listenify/backend/internal/auth"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	userManager  *user.Manager
	authProvider auth.Provider
	logger       *utils.Logger
}

// NewAuthHandler creates a new auth handler.
func NewAuthHandler(userManager *user.Manager, authProvider auth.Provider, logger *utils.Logger) *AuthHandler {
	return &AuthHandler{
		userManager:  userManager,
		authProvider: authProvider,
		logger:       logger.Named("auth_handler"),
	}
}

// AuthResponse represents the response for successful authentication operations.
type AuthResponse struct {
	User  models.PersonalUser `json:"user"`
	Token string              `json:"token"`
}

// RefreshResponse represents the response for a successful token refresh.
type RefreshResponse struct {
	Token string `json:"token"`
}

// Register handles user registration.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req models.UserRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode register request", err)
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		h.logger.Debug("Invalid register request", "error", err)
		utils.RespondWithValidationError(w, err)
		return
	}

	// Register user
	user, token, err := h.userManager.Register(r.Context(), req)
	if err != nil {
		switch err {
		case models.ErrEmailAlreadyExists:
			utils.RespondWithError(w, http.StatusConflict, "Email already in use")
		case models.ErrUsernameAlreadyExists:
			utils.RespondWithError(w, http.StatusConflict, "Username already in use")
		default:
			h.logger.Error("Failed to register user", err, "email", req.Email)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to register user")
		}
		return
	}

	// Respond with user and token
	utils.RespondWithJSON(w, http.StatusCreated, AuthResponse{
		User:  user.ToPersonalUser(),
		Token: token,
	})
}

// Login handles user login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req models.UserLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("Failed to decode login request", err)
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := utils.Validate(req); err != nil {
		h.logger.Debug("Invalid login request", "error", err)
		utils.RespondWithValidationError(w, err)
		return
	}

	// Login user
	user, token, err := h.userManager.Login(r.Context(), req)
	if err != nil {
		switch err {
		case models.ErrInvalidCredentials:
			utils.RespondWithError(w, http.StatusUnauthorized, "Invalid email or password")
		case models.ErrAccountDisabled:
			utils.RespondWithError(w, http.StatusForbidden, "Account is disabled")
		default:
			h.logger.Error("Failed to login user", err, "email", req.Email)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to login")
		}
		return
	}

	// Respond with user and token
	utils.RespondWithJSON(w, http.StatusOK, AuthResponse{
		User:  user.ToPersonalUser(),
		Token: token,
	})
}

// Logout handles user logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Logout user
	if err := h.userManager.Logout(r.Context(), userID); err != nil {
		h.logger.Error("Failed to logout user", err, "userId", userID)
		utils.RespondWithError(w, http.StatusInternalServerError, "Failed to logout")
		return
	}

	// Respond with success
	utils.RespondWithJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

// Refresh handles token refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Extract token from Authorization header
	token, err := utils.ExtractBearerToken(r)
	if err != nil {
		utils.RespondWithError(w, http.StatusUnauthorized, err.Error())
		return
	}

	// Refresh token
	newToken, err := h.authProvider.RefreshToken(token)
	if err != nil {
		switch err {
		case auth.ErrInvalidToken:
			utils.RespondWithError(w, http.StatusUnauthorized, "Invalid token")
		case auth.ErrExpiredToken:
			utils.RespondWithError(w, http.StatusUnauthorized, "Token has expired")
		default:
			h.logger.Error("Failed to refresh token", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to refresh token")
		}
		return
	}

	// Respond with new token
	utils.RespondWithJSON(w, http.StatusOK, RefreshResponse{
		Token: newToken,
	})
}

// Me returns the current user's information.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (set by auth middleware)
	userID, ok := r.Context().Value("userID").(string)
	if !ok {
		utils.RespondWithError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	// Get user
	user, err := h.userManager.GetUserByID(r.Context(), userID)
	if err != nil {
		switch err {
		case models.ErrUserNotFound:
			utils.RespondWithError(w, http.StatusNotFound, "User not found")
		default:
			h.logger.Error("Failed to get user", err, "userId", userID)
			utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get user information")
		}
		return
	}

	// Respond with user
	utils.RespondWithJSON(w, http.StatusOK, user.ToPersonalUser())
}
