// Package room provides services for room management and operations.
package room

import (
	"context"
	"errors"
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

	// If state doesn't exist, create a new one
	if managerState == nil {
		// Get room from database
		room, err := m.GetRoom(ctx, roomID)
		if err != nil {
			return nil, err
		}

		// Initialize room state
		err = m.stateManager.InitRoom(ctx, roomID.Hex())
		if err != nil {
			m.logger.Error("Failed to initialize room state", err, "roomId", roomID.Hex())
			// Continue anyway, we'll create a local state
		}

		// Create new state
		modelState := &models.RoomState{
			ID:          room.ID,
			Name:        room.Name,
			Settings:    room.Settings,
			DJQueue:     []models.QueueEntry{},
			ActiveUsers: 0,
			Users:       []models.PublicUser{},
			PlayHistory: []models.PlayHistoryEntry{},
		}

		return modelState, nil
	}

	// Convert managers.RoomState to models.RoomState
	modelState := &models.RoomState{
		ID:          roomID,
		ActiveUsers: managerState.ActiveUsers,
		DJQueue:     []models.QueueEntry{},
		Users:       []models.PublicUser{},
		PlayHistory: []models.PlayHistoryEntry{},
	}

	// Extract name and settings from Data map if available
	if managerState.Data != nil {
		if name, ok := managerState.Data["name"].(string); ok {
			modelState.Name = name
		}
		if settings, ok := managerState.Data["settings"].(models.RoomSettings); ok {
			modelState.Settings = settings
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

	// Add user to room
	publicUser := user.ToPublicUser()
	state.Users = append(state.Users, publicUser)
	state.ActiveUsers = len(state.Users)

	// Update room state
	err = m.UpdateRoomState(ctx, roomID, state)
	if err != nil {
		return err
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

// LeaveRoom removes a user from a room.
func (m *Manager) LeaveRoom(ctx context.Context, roomID, userID bson.ObjectID) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return err
	}

	// Find user in room
	index := -1
	for i, u := range state.Users {
		if u.ID == userID {
			index = i
			break
		}
	}

	// If user is not in room, return
	if index == -1 {
		return nil
	}

	// Remove user from room
	state.Users = slices.Delete(state.Users, index, index+1)
	state.ActiveUsers = len(state.Users)

	// Update room state
	err = m.UpdateRoomState(ctx, roomID, state)
	if err != nil {
		return err
	}

	// Update presence by removing room
	err = m.presenceManager.SetUserRoom(ctx, userID, "")
	if err != nil {
		m.logger.Error("Failed to clear user room", err, "userId", userID.Hex(), "roomId", roomID.Hex())
		// Continue anyway, the user was removed from the room successfully
	}

	return nil
}

// IsUserInRoom checks if a user is in a room.
func (m *Manager) IsUserInRoom(ctx context.Context, roomID, userID bson.ObjectID) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	state, err := m.GetRoomState(ctx, roomID)
	if err != nil {
		return false, err
	}

	// Check if user is in room
	for _, u := range state.Users {
		if u.ID == userID {
			return true, nil
		}
	}

	return false, nil
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
