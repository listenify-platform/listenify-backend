// Package room provides services for room management and operations.
package room

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// RoomManager handles room operations and state management.
type RoomManager interface {
	// Room CRUD operations
	CreateRoom(ctx context.Context, room *models.Room) (*models.Room, error)
	GetRoom(ctx context.Context, roomID bson.ObjectID) (*models.Room, error)
	GetRoomBySlug(ctx context.Context, slug string) (*models.Room, error)
	UpdateRoom(ctx context.Context, room *models.Room) (*models.Room, error)
	DeleteRoom(ctx context.Context, roomID bson.ObjectID) error

	// Room state operations
	GetRoomState(ctx context.Context, roomID bson.ObjectID) (*models.RoomState, error)
	UpdateRoomState(ctx context.Context, roomID bson.ObjectID, state *models.RoomState) error

	// Room user operations
	JoinRoom(ctx context.Context, roomID, userID bson.ObjectID) error
	LeaveRoom(ctx context.Context, roomID, userID bson.ObjectID) error
	IsUserInRoom(ctx context.Context, roomID, userID bson.ObjectID) (bool, error)
	GetRoomUsers(ctx context.Context, roomID bson.ObjectID) ([]models.PublicUser, error)

	// Room search and discovery
	SearchRooms(ctx context.Context, criteria models.RoomSearchCriteria) ([]*models.Room, int64, error)
	GetActiveRooms(ctx context.Context, limit int) ([]*models.Room, error)
	GetPopularRooms(ctx context.Context, limit int) ([]*models.Room, error)
}

// Manager implements the RoomManager interface.
type Manager struct {
	roomRepo        repositories.RoomRepository
	userRepo        repositories.UserRepository
	stateManager    managers.RoomStateManager
	presenceManager managers.PresenceManager
	logger          *utils.Logger
	mutex           sync.RWMutex
}

// NewManager creates a new room manager.
func NewManager(
	roomRepo repositories.RoomRepository,
	userRepo repositories.UserRepository,
	stateManager managers.RoomStateManager,
	presenceManager managers.PresenceManager,
	logger *utils.Logger,
) *Manager {
	return &Manager{
		roomRepo:        roomRepo,
		userRepo:        userRepo,
		stateManager:    stateManager,
		presenceManager: presenceManager,
		logger:          logger,
	}
}

// CreateRoom creates a new room.
func (m *Manager) CreateRoom(ctx context.Context, room *models.Room) (*models.Room, error) {
	// Set creation time
	now := time.Now()
	room.TimeCreate(now)
	room.LastActivity = now

	// Initialize stats
	room.Stats = models.RoomStats{
		LastStatsReset: now,
	}

	// Set active flag
	room.IsActive = true

	// Create room in database
	err := m.roomRepo.Create(ctx, room)
	if err != nil {
		return nil, err
	}

	// Initialize room state in Redis

	// Initialize room state
	err = m.stateManager.InitRoom(ctx, room.ID.Hex())
	if err != nil {
		m.logger.Error("Failed to initialize room state", err, "roomId", room.ID.Hex())
		// Continue anyway, the room was created successfully
	}

	return room, nil
}

// GetRoom gets a room by ID.
func (m *Manager) GetRoom(ctx context.Context, roomID bson.ObjectID) (*models.Room, error) {
	return m.roomRepo.FindByID(ctx, roomID)
}

// GetRoomBySlug gets a room by slug.
func (m *Manager) GetRoomBySlug(ctx context.Context, slug string) (*models.Room, error) {
	return m.roomRepo.FindBySlug(ctx, slug)
}

// UpdateRoom updates a room.
func (m *Manager) UpdateRoom(ctx context.Context, room *models.Room) (*models.Room, error) {
	// Update timestamp
	room.UpdateNow()

	// Update room in database
	err := m.roomRepo.Update(ctx, room)
	if err != nil {
		return nil, err
	}

	// Get current room state
	managerState, err := m.stateManager.GetRoomState(ctx, room.ID.Hex())
	if err == nil && managerState != nil {
		// Update room state with new settings
		// Store room name and settings in the Data map
		if managerState.Data == nil {
			managerState.Data = make(map[string]any)
		}
		managerState.Data["name"] = room.Name
		managerState.Data["settings"] = room.Settings

		// Save updated room state
		err = m.stateManager.UpdateRoomState(ctx, managerState)
		if err != nil {
			m.logger.Error("Failed to update room state", err, "roomId", room.ID.Hex())
			// Continue anyway, the room was updated successfully
		}
	}

	return room, nil
}

// DeleteRoom deletes a room.
func (m *Manager) DeleteRoom(ctx context.Context, roomID bson.ObjectID) error {
	// Delete room from database
	err := m.roomRepo.Delete(ctx, roomID)
	if err != nil {
		return err
	}

	// Delete room state by setting it inactive
	err = m.stateManager.SetRoomActive(ctx, roomID.Hex(), false)
	if err != nil {
		m.logger.Error("Failed to deactivate room state", err, "roomId", roomID.Hex())
		// Continue anyway, the room was deleted successfully
	}

	return nil
}

// GetRoomState gets the current state of a room.
func (m *Manager) GetRoomState(ctx context.Context, roomID bson.ObjectID) (*models.RoomState, error) {
	// Get room state from state manager
	managerState, err := m.stateManager.GetRoomState(ctx, roomID.Hex())
	if err != nil {
		return nil, err
	}

	// Get room from database
	room, err := m.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// If state doesn't exist, initialize it
	if managerState == nil {
		err = m.stateManager.InitRoom(ctx, roomID.Hex())
		if err != nil {
			m.logger.Error("Failed to initialize room state", err, "roomId", roomID.Hex())
		}
		managerState, err = m.stateManager.GetRoomState(ctx, roomID.Hex())
		if err != nil {
			return nil, err
		}
	}

	// Get all users in the room from Redis
	userIDs, err := m.stateManager.GetRoomUsers(ctx, roomID.Hex())
	if err != nil {
		m.logger.Error("Failed to get room users", err, "roomId", roomID.Hex())
		return nil, err
	}

	// Fetch user details for each ID and verify online status
	users := make([]models.PublicUser, 0, len(userIDs))
	for _, userIDStr := range userIDs {
		userID, err := bson.ObjectIDFromHex(userIDStr)
		if err != nil {
			m.logger.Error("Invalid user ID in Redis", err, "userId", userIDStr)
			continue
		}

		// Check user's presence status
		presence, err := m.presenceManager.GetPresence(ctx, userID)
		if err != nil {
			m.logger.Error("Failed to get user presence", err, "userId", userIDStr)
			// Remove user from room since we can't verify their status
			if err := m.stateManager.RemoveUserFromRoom(ctx, roomID.Hex(), userIDStr); err != nil {
				m.logger.Error("Failed to remove stale user from room", err, "userId", userIDStr)
			}
			continue
		}

		// If user is not online or has no presence data, remove them from room
		if presence == nil || presence.Status != "online" {
			if err := m.stateManager.RemoveUserFromRoom(ctx, roomID.Hex(), userIDStr); err != nil {
				m.logger.Error("Failed to remove offline user from room", err, "userId", userIDStr)
			}
			continue
		}

		// Get user details
		user, err := m.userRepo.FindByID(ctx, userID)
		if err != nil {
			m.logger.Error("Failed to get user", err, "userId", userIDStr)
			continue
		}

		// Add user to room state
		publicUser := user.ToPublicUser()
		publicUser.Online = true
		users = append(users, publicUser)
	}

	// Create room state with all information
	modelState := &models.RoomState{
		ID:          roomID,
		Name:        room.Name,
		ActiveUsers: len(users),
		Settings:    room.Settings,
		DJQueue:     []models.QueueEntry{},
		Users:       users,
		PlayHistory: []models.PlayHistoryEntry{},
	}

	// Add additional data from Redis state
	if managerState.Data != nil {
		if name, ok := managerState.Data["name"].(string); ok {
			modelState.Name = name
		}
	}

	return modelState, nil
}

// UpdateRoomState updates the state of a room.
func (m *Manager) UpdateRoomState(ctx context.Context, roomID bson.ObjectID, state *models.RoomState) error {
	// Convert models.RoomState to managers.RoomState
	managerState := &managers.RoomState{
		RoomID:       roomID.Hex(),
		IsActive:     true,
		ActiveUsers:  state.ActiveUsers,
		LastActivity: time.Now(),
		Data:         make(map[string]any),
	}

	// Store room name and settings in the Data map
	managerState.Data["name"] = state.Name
	managerState.Data["settings"] = state.Settings

	// Update room state in state manager
	return m.stateManager.UpdateRoomState(ctx, managerState)
}

// JoinRoom adds a user to a room.
func (m *Manager) JoinRoom(ctx context.Context, roomID, userID bson.ObjectID) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room
	room, err := m.GetRoom(ctx, roomID)
	if err != nil {
		return err
	}

	// Check if room is active
	if !room.IsActive {
		return errors.New("room is not active")
	}

	// Get user
	user, err := m.userRepo.FindByID(ctx, userID)
	if err != nil {
		return err
	}

	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	// Check if user is already in room
	for _, u := range state.Users {
		if u.ID == userID {
			return nil // User is already in room
		}
	}

	// Check if room is at capacity
	if len(state.Users) >= room.Settings.Capacity {
		return models.ErrRoomFull
	}

	// Check if user is banned
	if slices.Contains(room.BannedUsers, userID) {
		return errors.New("user is banned from this room")
	}

	// Add user to Redis first
	err = m.stateManager.AddUserToRoom(ctx, roomID.Hex(), userID.Hex())
	if err != nil {
		return fmt.Errorf("failed to add user to Redis: %w", err)
	}

	// Get updated room state with all users
	state, err = m.GetRoomState(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to get updated room state: %w", err)
	}

	// Update presence
	err = m.presenceManager.UpdatePresence(ctx, userID, user.Username, "online")
	if err != nil {
		m.logger.Error("Failed to update user presence", err, "userId", userID.Hex())
		// Continue anyway, the user was added to the room successfully
	}

	// Set user's current room
	err = m.presenceManager.SetUserRoom(ctx, userID, roomID.Hex())
	if err != nil {
		m.logger.Error("Failed to set user room", err, "userId", userID.Hex(), "roomId", roomID.Hex())
		// Continue anyway, the user was added to the room successfully
	}

	// Update room last activity
	room.LastActivity = time.Now()
	err = m.roomRepo.Update(ctx, room)
	if err != nil {
		m.logger.Error("Failed to update room last activity", err, "roomId", roomID.Hex())
		// Continue anyway, the user was added to the room successfully
	}

	return nil
}

// LeaveRoom removes a user from a room and performs cleanup.
func (m *Manager) LeaveRoom(ctx context.Context, roomID, userID bson.ObjectID) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state first to check if user is actually in the room
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			// If room doesn't exist, just clean up user presence
			if err := m.cleanupUserPresence(ctx, userID); err != nil {
				m.logger.Error("Failed to cleanup user presence", err, "userId", userID.Hex())
			}
			return nil
		}
		return err
	}

	// Check if user is in room
	userInRoom := false
	for _, u := range state.Users {
		if u.ID == userID {
			userInRoom = true
			break
		}
	}

	// Even if user is not in room, we should still clean up their presence
	if !userInRoom {
		if err := m.cleanupUserPresence(ctx, userID); err != nil {
			m.logger.Error("Failed to cleanup user presence", err, "userId", userID.Hex())
		}
		return nil
	}

	// Remove user from Redis first
	if err := m.stateManager.RemoveUserFromRoom(ctx, roomID.Hex(), userID.Hex()); err != nil {
		m.logger.Error("Failed to remove user from Redis", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		// Continue with cleanup even if Redis removal fails
	}

	// Get updated room state after user removal
	updatedState, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		m.logger.Error("Failed to get updated room state", err, "roomId", roomID.Hex())
		// Continue with cleanup even if state fetch fails
	} else {
		// Update room state with new user count
		updatedState.ActiveUsers = len(updatedState.Users)
		if err := m.UpdateRoomState(ctx, roomID, updatedState); err != nil {
			m.logger.Error("Failed to update room state", err, "roomId", roomID.Hex())
		}
	}

	// Clean up user presence
	if err := m.cleanupUserPresence(ctx, userID); err != nil {
		m.logger.Error("Failed to cleanup user presence", err, "userId", userID.Hex())
		// Continue with room update even if presence cleanup fails
	}

	// Update room last activity
	if err := m.updateRoomActivity(ctx, roomID); err != nil {
		m.logger.Error("Failed to update room activity", err, "roomId", roomID.Hex())
		// Continue anyway as the user has been removed
	}

	return nil
}

// cleanupUserPresence handles all presence-related cleanup for a user
func (m *Manager) cleanupUserPresence(ctx context.Context, userID bson.ObjectID) error {
	// Clear user's current room
	if err := m.presenceManager.SetUserRoom(ctx, userID, ""); err != nil {
		return fmt.Errorf("failed to clear user room: %w", err)
	}

	// Update user presence to offline
	if err := m.presenceManager.UpdatePresence(ctx, userID, "", "offline"); err != nil {
		return fmt.Errorf("failed to update user presence: %w", err)
	}

	return nil
}

// updateRoomActivity updates the room's last activity timestamp
func (m *Manager) updateRoomActivity(ctx context.Context, roomID bson.ObjectID) error {
	room, err := m.GetRoom(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to get room: %w", err)
	}

	room.LastActivity = time.Now()
	if err := m.roomRepo.Update(ctx, room); err != nil {
		return fmt.Errorf("failed to update room: %w", err)
	}

	return nil
}

// IsUserInRoom checks if a user is in a room.
func (m *Manager) IsUserInRoom(ctx context.Context, roomID, userID bson.ObjectID) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// First check Redis directly for room membership
	inRoom, err := m.stateManager.IsUserInRoom(ctx, roomID.Hex(), userID.Hex())
	if err != nil {
		m.logger.Error("Failed to check room membership in Redis", err, "roomId", roomID.Hex(), "userId", userID.Hex())
		return false, err
	}

	if !inRoom {
		return false, nil
	}

	// If user is in room, verify their presence status
	presence, err := m.presenceManager.GetPresence(ctx, userID)
	if err != nil {
		// Log but don't fail the check just because we couldn't verify presence
		m.logger.Error("Failed to get user presence", err, "userId", userID.Hex())
		return true, nil
	}

	// If we can verify presence, ensure user is online
	if presence != nil && presence.Status == "online" {
		return true, nil
	}

	// User is in room but appears offline, schedule cleanup but still return true
	go func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := m.cleanupUserPresence(cleanupCtx, userID); err != nil {
			m.logger.Error("Failed to cleanup user presence", err, "userId", userID.Hex())
		}
	}()

	return true, nil
}

// GetRoomUsers gets all users in a room.
func (m *Manager) GetRoomUsers(ctx context.Context, roomID bson.ObjectID) ([]models.PublicUser, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	return state.Users, nil
}

// SearchRooms searches for rooms based on criteria.
func (m *Manager) SearchRooms(ctx context.Context, criteria models.RoomSearchCriteria) ([]*models.Room, int64, error) {
	return m.roomRepo.SearchRooms(ctx, criteria)
}

// GetActiveRooms gets a list of active rooms.
func (m *Manager) GetActiveRooms(ctx context.Context, limit int) ([]*models.Room, error) {
	return m.roomRepo.FindRecentRooms(ctx, limit)
}

// GetPopularRooms gets a list of popular rooms.
func (m *Manager) GetPopularRooms(ctx context.Context, limit int) ([]*models.Room, error) {
	return m.roomRepo.FindPopularRooms(ctx, limit)
}
