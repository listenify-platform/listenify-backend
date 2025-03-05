// Package room provides services for room management and operations.
package room

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// QueueManager handles DJ queue operations for a room.
type QueueManager struct {
	roomManager RoomManager
	logger      *utils.Logger
	mutex       sync.RWMutex
}

// NewQueueManager creates a new QueueManager.
func NewQueueManager(roomManager RoomManager, logger *utils.Logger) *QueueManager {
	return &QueueManager{
		roomManager: roomManager,
		logger:      logger,
	}
}

// AddToQueue adds a user to the DJ queue.
func (m *QueueManager) AddToQueue(ctx context.Context, roomID, userID bson.ObjectID) (*models.RoomState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// Check if user is already in the queue
	for _, entry := range roomState.DJQueue {
		if entry.User.ID == userID {
			return nil, errors.New("user is already in the queue")
		}
	}

	// Check if queue is full
	if len(roomState.DJQueue) >= roomState.Settings.WaitlistMax {
		return nil, errors.New("queue is full")
	}

	// Check if user is in the room
	var user *models.PublicUser
	for i, u := range roomState.Users {
		if u.ID == userID {
			user = &roomState.Users[i]
			break
		}
	}
	if user == nil {
		return nil, errors.New("user is not in the room")
	}

	// Add user to queue
	position := len(roomState.DJQueue)
	entry := models.QueueEntry{
		User:      *user,
		Position:  position,
		JoinTime:  time.Now(),
		PlayCount: 0,
		JoinedAt:  time.Now(),
	}
	roomState.DJQueue = append(roomState.DJQueue, entry)

	// Update room state
	err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
	if err != nil {
		return nil, err
	}

	// If there's no current DJ and this is the first person in the queue, make them the DJ
	if roomState.CurrentDJ == nil && len(roomState.DJQueue) == 1 {
		return m.AdvanceQueue(ctx, roomID)
	}

	return roomState, nil
}

// RemoveFromQueue removes a user from the DJ queue.
func (m *QueueManager) RemoveFromQueue(ctx context.Context, roomID, userID bson.ObjectID) (*models.RoomState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// Find user in queue
	index := -1
	for i, entry := range roomState.DJQueue {
		if entry.User.ID == userID {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, errors.New("user is not in the queue")
	}

	// Remove user from queue
	roomState.DJQueue = slices.Delete(roomState.DJQueue, index, index+1)

	// Update positions for remaining users
	for i := index; i < len(roomState.DJQueue); i++ {
		roomState.DJQueue[i].Position = i
	}

	// Update room state
	err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
	if err != nil {
		return nil, err
	}

	// If the current DJ was removed, advance to the next DJ
	if roomState.CurrentDJ != nil && roomState.CurrentDJ.ID == userID {
		return m.AdvanceQueue(ctx, roomID)
	}

	return roomState, nil
}

// MoveInQueue moves a user to a new position in the DJ queue.
func (m *QueueManager) MoveInQueue(ctx context.Context, roomID, userID bson.ObjectID, newPosition int) (*models.RoomState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// Find user in queue
	index := -1
	for i, entry := range roomState.DJQueue {
		if entry.User.ID == userID {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, errors.New("user is not in the queue")
	}

	// Validate new position
	if newPosition < 0 || newPosition >= len(roomState.DJQueue) {
		return nil, errors.New("invalid position")
	}

	// Move user in queue
	entry := roomState.DJQueue[index]
	// Remove from current position
	roomState.DJQueue = slices.Delete(roomState.DJQueue, index, index+1)
	// Insert at new position
	roomState.DJQueue = append(roomState.DJQueue[:newPosition], append([]models.QueueEntry{entry}, roomState.DJQueue[newPosition:]...)...)

	// Update positions for all users
	for i := range roomState.DJQueue {
		roomState.DJQueue[i].Position = i
	}

	// Update room state
	err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
	if err != nil {
		return nil, err
	}

	return roomState, nil
}

// GetQueue gets the current DJ queue for a room.
func (m *QueueManager) GetQueue(ctx context.Context, roomID bson.ObjectID) ([]models.QueueEntry, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	return roomState.DJQueue, nil
}

// GetCurrentDJ gets the current DJ for a room.
func (m *QueueManager) GetCurrentDJ(ctx context.Context, roomID bson.ObjectID) (*models.PublicUser, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	return roomState.CurrentDJ, nil
}

// GetCurrentMedia gets the currently playing media for a room.
func (m *QueueManager) GetCurrentMedia(ctx context.Context, roomID bson.ObjectID) (*models.MediaInfo, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	return roomState.CurrentMedia, nil
}

// AdvanceQueue advances to the next DJ in the queue.
func (m *QueueManager) AdvanceQueue(ctx context.Context, roomID bson.ObjectID) (*models.RoomState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// If there's a current DJ and media, add to play history
	if roomState.CurrentDJ != nil && roomState.CurrentMedia != nil {
		historyEntry := models.PlayHistoryEntry{
			Media:    *roomState.CurrentMedia,
			DJ:       *roomState.CurrentDJ,
			PlayTime: roomState.MediaStartTime,
			Woots:    0, // These would be populated from actual vote data
			Mehs:     0,
			Grabs:    0,
		}
		roomState.PlayHistory = append([]models.PlayHistoryEntry{historyEntry}, roomState.PlayHistory...)
		if len(roomState.PlayHistory) > 50 {
			roomState.PlayHistory = roomState.PlayHistory[:50]
		}
	}

	// If queue is empty, clear current DJ and media
	if len(roomState.DJQueue) == 0 {
		roomState.CurrentDJ = nil
		roomState.CurrentMedia = nil
		roomState.MediaStartTime = time.Time{}
		roomState.MediaProgress = 0
		roomState.MediaEndTime = time.Time{}

		// Update room state
		err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
		if err != nil {
			return nil, err
		}

		return roomState, nil
	}

	// Get next DJ from queue
	nextDJ := roomState.DJQueue[0]

	// Update play count for the DJ
	nextDJ.PlayCount++

	// Move DJ to end of queue
	roomState.DJQueue = append(roomState.DJQueue[1:], nextDJ)

	// Update positions
	for i := range roomState.DJQueue {
		roomState.DJQueue[i].Position = i
	}

	// Set current DJ
	roomState.CurrentDJ = &nextDJ.User

	// Clear current media (would be set by the DJ playing a track)
	roomState.CurrentMedia = nil
	roomState.MediaStartTime = time.Time{}
	roomState.MediaProgress = 0
	roomState.MediaEndTime = time.Time{}

	// Update room state
	err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
	if err != nil {
		return nil, err
	}

	return roomState, nil
}

// PlayMedia sets the currently playing media for a room.
func (m *QueueManager) PlayMedia(ctx context.Context, roomID bson.ObjectID, mediaInfo *models.MediaInfo) (*models.RoomState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// Check if there's a current DJ
	if roomState.CurrentDJ == nil {
		return nil, errors.New("no current DJ")
	}

	// Set current media
	roomState.CurrentMedia = mediaInfo
	roomState.MediaStartTime = time.Now()
	roomState.MediaProgress = 0
	if mediaInfo != nil {
		roomState.MediaEndTime = roomState.MediaStartTime.Add(time.Duration(mediaInfo.Duration) * time.Second)
	} else {
		roomState.MediaEndTime = time.Time{}
	}

	// Update room state
	err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
	if err != nil {
		return nil, err
	}

	return roomState, nil
}

// SkipCurrentMedia skips the currently playing media.
func (m *QueueManager) SkipCurrentMedia(ctx context.Context, roomID bson.ObjectID) (*models.RoomState, error) {
	// Simply advance to the next DJ
	return m.AdvanceQueue(ctx, roomID)
}

// ClearQueue clears the DJ queue for a room.
func (m *QueueManager) ClearQueue(ctx context.Context, roomID bson.ObjectID) (*models.RoomState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// Clear queue
	roomState.DJQueue = []models.QueueEntry{}

	// Update room state
	err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
	if err != nil {
		return nil, err
	}

	return roomState, nil
}

// ShuffleQueue randomly shuffles the DJ queue for a room.
func (m *QueueManager) ShuffleQueue(ctx context.Context, roomID bson.ObjectID) (*models.RoomState, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	// Shuffle queue
	queue := roomState.DJQueue
	rand.Shuffle(len(queue), func(i, j int) {
		queue[i], queue[j] = queue[j], queue[i]
	})

	// Update positions
	for i := range queue {
		queue[i].Position = i
	}
	roomState.DJQueue = queue

	// Update room state
	err = m.roomManager.UpdateRoomState(ctx, roomID, roomState)
	if err != nil {
		return nil, err
	}

	return roomState, nil
}

// GetQueuePosition gets a user's position in the DJ queue.
func (m *QueueManager) GetQueuePosition(ctx context.Context, roomID, userID bson.ObjectID) (int, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return -1, err
	}

	// Find user in queue
	for _, entry := range roomState.DJQueue {
		if entry.User.ID == userID {
			return entry.Position, nil
		}
	}

	// User is not in queue
	return -1, errors.New("user is not in the queue")
}

// IsUserInQueue checks if a user is in the DJ queue.
func (m *QueueManager) IsUserInQueue(ctx context.Context, roomID, userID bson.ObjectID) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return false, err
	}

	// Find user in queue
	for _, entry := range roomState.DJQueue {
		if entry.User.ID == userID {
			return true, nil
		}
	}

	return false, nil
}

// IsUserCurrentDJ checks if a user is the current DJ.
func (m *QueueManager) IsUserCurrentDJ(ctx context.Context, roomID, userID bson.ObjectID) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return false, err
	}

	return roomState.CurrentDJ != nil && roomState.CurrentDJ.ID == userID, nil
}

// GetPlayHistory gets the play history for a room.
func (m *QueueManager) GetPlayHistory(ctx context.Context, roomID bson.ObjectID) ([]models.PlayHistoryEntry, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Get room state
	roomState, err := m.roomManager.GetRoomState(ctx, roomID)
	if err != nil {
		return nil, err
	}

	return roomState.PlayHistory, nil
}
