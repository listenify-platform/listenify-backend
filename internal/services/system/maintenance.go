// Package system provides system-level services for monitoring and maintenance.
package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/db/redis"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/utils"
)

// MaintenanceTask represents a maintenance task to be executed.
type MaintenanceTask struct {
	Name     string
	Interval time.Duration
	LastRun  time.Time
	Fn       func(context.Context) error
}

// MaintenanceConfig contains configuration for the maintenance service.
type MaintenanceConfig struct {
	// Whether to enable automatic maintenance tasks
	Enabled bool
	// Directory for temporary files that can be cleaned up
	TempDir string
	// Maximum age of temporary files before cleanup
	TempFileMaxAge time.Duration
	// Maximum age of logs before rotation
	LogMaxAge time.Duration
	// Maximum age of history records before cleanup
	HistoryMaxAge time.Duration
	// Maximum age of inactive rooms before cleanup
	InactiveRoomMaxAge time.Duration
	// Interval for running maintenance tasks
	MaintenanceInterval time.Duration
	// Maximum number of concurrent maintenance tasks
	MaxConcurrentTasks int
	// Timeout for individual maintenance tasks
	TaskTimeout time.Duration
}

// DefaultMaintenanceConfig returns the default maintenance configuration.
func DefaultMaintenanceConfig() MaintenanceConfig {
	return MaintenanceConfig{
		Enabled:             true,
		TempDir:             os.TempDir(),
		TempFileMaxAge:      24 * time.Hour,
		LogMaxAge:           7 * 24 * time.Hour,
		HistoryMaxAge:       30 * 24 * time.Hour,
		InactiveRoomMaxAge:  7 * 24 * time.Hour,
		MaintenanceInterval: 1 * time.Hour,
		MaxConcurrentTasks:  3,
		TaskTimeout:         30 * time.Minute,
	}
}

// WebSocketServer interface defines methods needed for WebSocket connection verification
type WebSocketServer interface {
	IsUserConnected(userID string) bool
	GetUserConnections(userID string) int
}

// MaintenanceService manages system maintenance tasks.
type MaintenanceService struct {
	config       MaintenanceConfig
	mongoDB      *mongo.Database
	redisClient  *redis.Client
	roomRepo     repositories.RoomRepository
	historyRepo  repositories.HistoryRepository
	mediaRepo    repositories.MediaRepository
	playlistRepo repositories.PlaylistRepository
	userRepo     repositories.UserRepository
	roomManager  room.RoomManager
	logger       *utils.Logger
	tasks        []*MaintenanceTask
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	wsServer     WebSocketServer // WebSocket server for connection verification
}

// NewMaintenanceService creates a new maintenance service.
func NewMaintenanceService(
	config MaintenanceConfig,
	mongoDB *mongo.Database,
	redisClient *redis.Client,
	roomRepo repositories.RoomRepository,
	historyRepo repositories.HistoryRepository,
	mediaRepo repositories.MediaRepository,
	playlistRepo repositories.PlaylistRepository,
	userRepo repositories.UserRepository,
	roomManager room.RoomManager,
	logger *utils.Logger,
) *MaintenanceService {
	s := &MaintenanceService{
		config:       config,
		mongoDB:      mongoDB,
		redisClient:  redisClient,
		roomRepo:     roomRepo,
		historyRepo:  historyRepo,
		mediaRepo:    mediaRepo,
		playlistRepo: playlistRepo,
		userRepo:     userRepo,
		roomManager:  roomManager,
		logger:       logger,
		stopCh:       make(chan struct{}),
		tasks:        make([]*MaintenanceTask, 0),
	}

	// Register default maintenance tasks
	s.RegisterTask("temp_file_cleanup", config.MaintenanceInterval, s.CleanupTempFiles)
	s.RegisterTask("inactive_room_cleanup", config.MaintenanceInterval, s.CleanupInactiveRooms)
	s.RegisterTask("history_cleanup", config.MaintenanceInterval, s.CleanupHistory)
	s.RegisterTask("database_optimization", 24*time.Hour, s.OptimizeDatabase)
	s.RegisterTask("cache_cleanup", config.MaintenanceInterval, s.CleanupCache)
	s.RegisterTask("stale_client_cleanup", 5*time.Minute, s.CleanupStaleClients)

	return s
}

// RegisterTask registers a new maintenance task.
func (s *MaintenanceService) RegisterTask(name string, interval time.Duration, fn func(context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task := &MaintenanceTask{
		Name:     name,
		Interval: interval,
		LastRun:  time.Now().Add(-interval), // Schedule to run immediately
		Fn:       fn,
	}

	s.tasks = append(s.tasks, task)
	s.logger.Info("Registered maintenance task", "name", name, "interval", interval)
}

// Start starts the maintenance service.
func (s *MaintenanceService) Start(ctx context.Context) error {
	if !s.config.Enabled {
		s.logger.Info("Maintenance service is disabled")
		return nil
	}

	s.logger.Info("Starting maintenance service")

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("Panic in maintenance service", fmt.Errorf("%v", r))
				// Attempt to restart the service if it panics
				go func() {
					s.logger.Info("Attempting to restart maintenance service after panic")
					if err := s.Start(ctx); err != nil {
						s.logger.Error("Failed to restart maintenance service", err)
					}
				}()
			}
		}()

		for {
			select {
			case <-ticker.C:
				s.runDueTasks(ctx)
			case <-s.stopCh:
				s.logger.Info("Stopping maintenance service")
				return
			case <-ctx.Done():
				s.logger.Info("Context cancelled, stopping maintenance service")
				return
			}
		}
	}()

	return nil
}

// Stop stops the maintenance service.
func (s *MaintenanceService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// RunAllTasks runs all maintenance tasks immediately.
func (s *MaintenanceService) RunAllTasks(ctx context.Context) error {
	s.logger.Info("Running all maintenance tasks")

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.config.MaxConcurrentTasks)
	errCh := make(chan error, len(s.tasks))

	for _, task := range s.tasks {
		wg.Add(1)
		go func(t *MaintenanceTask) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.logger.Info("Running maintenance task", "name", t.Name)
			if err := t.Fn(ctx); err != nil {
				s.logger.Error("Failed to run maintenance task", err, "name", t.Name)
				errCh <- fmt.Errorf("task %s failed: %w", t.Name, err)
				return
			}

			s.mu.Lock()
			t.LastRun = time.Now()
			s.mu.Unlock()

			s.logger.Info("Completed maintenance task", "name", t.Name)
		}(task)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("some maintenance tasks failed: %v", errs)
	}

	return nil
}

// Restart gracefully restarts the maintenance service.
func (s *MaintenanceService) Restart(ctx context.Context) error {
	s.logger.Info("Restarting maintenance service")

	// Stop the service
	s.Stop()

	// Start the service again
	return s.Start(ctx)
}

// runDueTasks runs all maintenance tasks that are due.
func (s *MaintenanceService) runDueTasks(ctx context.Context) {
	// Create a timeout context for the entire operation
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Get due tasks with minimal lock time
	s.mu.Lock()
	var dueTasks []*MaintenanceTask
	now := time.Now()
	for _, task := range s.tasks {
		if now.Sub(task.LastRun) >= task.Interval {
			dueTasks = append(dueTasks, task)
		}
	}
	s.mu.Unlock()

	if len(dueTasks) == 0 {
		return
	}

	s.logger.Info("Running due maintenance tasks", "count", len(dueTasks))

	// Create channels for task coordination
	taskCh := make(chan *MaintenanceTask, len(dueTasks))
	errCh := make(chan error, len(dueTasks))
	doneCh := make(chan struct{})

	// Start worker pool
	var wg sync.WaitGroup
	maxWorkers := s.config.MaxConcurrentTasks
	if maxWorkers <= 0 {
		maxWorkers = 3 // Default to 3 workers
	}

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for task := range taskCh {
				// Create task-specific context
				taskCtx, taskCancel := context.WithTimeout(opCtx, s.config.TaskTimeout)

				func() {
					defer taskCancel()
					defer func() {
						if r := recover(); r != nil {
							err := fmt.Errorf("panic in task %s (worker %d): %v", task.Name, workerID, r)
							s.logger.Error("Task panic recovered", err, "name", task.Name, "worker", workerID)
							errCh <- err
						}
					}()

					s.logger.Debug("Worker starting task", "worker", workerID, "task", task.Name)
					if err := task.Fn(taskCtx); err != nil {
						s.logger.Error("Task failed", err, "name", task.Name, "worker", workerID)
						errCh <- fmt.Errorf("task %s failed (worker %d): %w", task.Name, workerID, err)
						return
					}

					// Update LastRun with minimal lock time
					s.mu.Lock()
					task.LastRun = time.Now()
					s.mu.Unlock()

					s.logger.Debug("Worker completed task", "worker", workerID, "task", task.Name)
				}()

				// Check if context is done after each task
				select {
				case <-opCtx.Done():
					s.logger.Warn("Worker stopping due to context done", "worker", workerID)
					return
				default:
				}
			}
		}(i)
	}

	// Start completion monitor
	go func() {
		wg.Wait()
		close(errCh)
		close(doneCh)
	}()

	// Send tasks to workers
	go func() {
		for _, task := range dueTasks {
			select {
			case <-opCtx.Done():
				s.logger.Warn("Task distribution stopped due to context done")
				close(taskCh)
				return
			case taskCh <- task:
			}
		}
		close(taskCh)
	}()

	// Wait for completion or timeout
	select {
	case <-opCtx.Done():
		s.logger.Warn("Maintenance tasks timed out")
		return
	case <-doneCh:
		// Collect errors
		var errs []error
		for err := range errCh {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			s.logger.Error("Some maintenance tasks failed", fmt.Errorf("multiple errors occurred"), "errorCount", len(errs))
		}
		s.logger.Info("All maintenance tasks completed", "successful", len(dueTasks)-len(errs), "failed", len(errs))
	}
}

// CleanupTempFiles removes temporary files older than the configured max age.
func (s *MaintenanceService) CleanupTempFiles(ctx context.Context) error {
	s.logger.Info("Cleaning up temporary files", "dir", s.config.TempDir, "maxAge", s.config.TempFileMaxAge)

	cutoff := time.Now().Add(-s.config.TempFileMaxAge)
	var deletedCount int

	err := filepath.Walk(s.config.TempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is older than cutoff
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				s.logger.Error("Failed to delete temporary file", err, "path", path)
				return nil // Continue with other files
			}
			deletedCount++
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to cleanup temporary files: %w", err)
	}

	s.logger.Info("Temporary file cleanup completed", "deletedCount", deletedCount)
	return nil
}

// CleanupInactiveRooms removes inactive rooms older than the configured max age.
func (s *MaintenanceService) CleanupInactiveRooms(ctx context.Context) error {
	s.logger.Info("Cleaning up inactive rooms", "maxAge", s.config.InactiveRoomMaxAge)

	cutoff := time.Now().Add(-s.config.InactiveRoomMaxAge)
	filter := bson.M{
		"lastActivity": bson.M{"$lt": cutoff},
		"isActive":     false,
	}

	// Use the MongoDB collection directly since the repository doesn't have a DeleteMany method
	collection := s.mongoDB.Collection("rooms")
	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to cleanup inactive rooms: %w", err)
	}

	s.logger.Info("Inactive room cleanup completed", "deletedCount", result.DeletedCount)
	return nil
}

// CleanupHistory removes history records older than the configured max age.
func (s *MaintenanceService) CleanupHistory(ctx context.Context) error {
	s.logger.Info("Cleaning up history records", "maxAge", s.config.HistoryMaxAge)

	cutoff := time.Now().Add(-s.config.HistoryMaxAge)
	filter := bson.M{
		"timestamp": bson.M{"$lt": cutoff},
	}

	// Use the MongoDB collection directly since the repository doesn't have a DeleteMany method
	historyCollections := []string{
		"history",
		"play_history",
		"user_history",
		"room_history",
		"dj_history",
		"session_history",
		"moderation_history",
	}

	var totalDeleted int64
	for _, collName := range historyCollections {
		collection := s.mongoDB.Collection(collName)
		result, err := collection.DeleteMany(ctx, filter)
		if err != nil {
			s.logger.Error("Failed to cleanup history collection", err, "collection", collName)
			continue
		}
		totalDeleted += result.DeletedCount
	}

	s.logger.Info("History cleanup completed", "deletedCount", totalDeleted)
	return nil
}

// OptimizeDatabase performs database optimization tasks.
func (s *MaintenanceService) OptimizeDatabase(ctx context.Context) error {
	s.logger.Info("Optimizing database")

	if s.mongoDB == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Run database commands to optimize indexes and collections
	collections := []string{"users", "rooms", "playlists", "media", "history"}
	var errs []error

	for _, collection := range collections {
		s.logger.Info("Optimizing collection", "collection", collection)

		// Create a timeout context for this operation
		opCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		// Example: Run compact command
		command := bson.D{{Key: "compact", Value: collection}}
		result := s.mongoDB.RunCommand(opCtx, command)
		if result.Err() != nil {
			err := fmt.Errorf("failed to optimize collection %s: %w", collection, result.Err())
			s.logger.Error("Collection optimization failed", result.Err(), "collection", collection)
			errs = append(errs, err)
			// Continue with other collections
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("database optimization completed with errors: %v", errs)
	}

	s.logger.Info("Database optimization completed successfully")
	return nil
}

// SetWebSocketServer sets the WebSocket server for connection verification
func (s *MaintenanceService) SetWebSocketServer(server WebSocketServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wsServer = server
}

// userCleanupTask represents a user cleanup task
type userCleanupTask struct {
	userID      string
	presenceKey string
	roomID      string
}

// CleanupStaleClients removes stale client data from Redis and rooms
func (s *MaintenanceService) CleanupStaleClients(ctx context.Context) error {
	// Create a new background context for the cleanup process
	cleanupCtx := context.Background()

	// Run cleanup in a non-blocking goroutine with error handling
	go func() {
		// Create a channel to track completion
		done := make(chan struct{})
		errCh := make(chan error, 1)

		go func() {
			if err := s.performAsyncCleanup(cleanupCtx); err != nil {
				errCh <- err
			}
			close(done)
		}()

		// Wait for completion or parent context cancellation
		select {
		case <-ctx.Done():
			s.logger.Debug("Parent context cancelled, cleanup will continue in background")
		case err := <-errCh:
			s.logger.Error("Async cleanup failed", err)
		case <-done:
			s.logger.Debug("Cleanup completed successfully")
		}
	}()
	return nil
}

// performAsyncCleanup handles the actual cleanup process asynchronously
func (s *MaintenanceService) performAsyncCleanup(ctx context.Context) error {
	// Quick check for WebSocket server without lock
	if s.wsServer == nil {
		s.logger.Debug("WebSocket server not initialized, deferring cleanup")
		return nil
	}

	// Create a short timeout context for the operation
	opCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	s.logger.Info("Starting async stale client cleanup")

	// Get online users with retries
	var onlineUsers []string
	var err error
	for retries := 0; retries < 3; retries++ {
		listCtx, listCancel := context.WithTimeout(opCtx, 2*time.Second)
		onlineUsers, err = s.redisClient.SMembers(listCtx, "online:users")
		listCancel()

		if err == nil {
			break
		}

		if retries < 2 {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if ctx.Err() == context.DeadlineExceeded {
			s.logger.Warn("Timeout while fetching online users, will retry next cycle")
			return nil
		}
		return fmt.Errorf("failed to get online users after retries: %w", err)
	}

	// If we can't get the list, log and return - better to retry next cycle
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			s.logger.Warn("Timeout while fetching online users, will retry next cycle")
			return nil
		}
		return fmt.Errorf("failed to get online users: %w", err)
	}

	if len(onlineUsers) == 0 {
		return nil
	}

	s.logger.Info("Checking for stale clients", "onlineUsersCount", len(onlineUsers))

	// Create channels for task coordination
	taskCh := make(chan userCleanupTask, len(onlineUsers))
	errCh := make(chan error, len(onlineUsers))
	doneCh := make(chan struct{})

	// Start worker pool
	var wg sync.WaitGroup
	maxWorkers := s.config.MaxConcurrentTasks
	if maxWorkers <= 0 {
		maxWorkers = 3
	}

	// Get WebSocket server reference once
	s.mu.Lock()
	ws := s.wsServer
	s.mu.Unlock()

	// Double check after getting reference
	if ws == nil {
		s.logger.Debug("WebSocket server became unavailable, deferring cleanup")
		return nil
	}

	// Start workers
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int, ws WebSocketServer) {
			defer wg.Done()
			for task := range taskCh {
				// Use shorter timeout for individual tasks
				taskCtx, taskCancel := context.WithTimeout(opCtx, 5*time.Second)

				func() {
					defer taskCancel()
					defer func() {
						if r := recover(); r != nil {
							err := fmt.Errorf("panic in cleanup task (worker %d): %v", workerID, r)
							s.logger.Error("Task panic recovered", err, "worker", workerID)
							errCh <- err
						}
					}()

					// Quick check if user is still connected
					if ws.IsUserConnected(task.userID) {
						s.logger.Debug("User has active connection, skipping", "userId", task.userID, "worker", workerID)
						return
					}

					// Process cleanup in batches with timeouts
					if err := s.processUserCleanup(taskCtx, ws, task); err != nil {
						errCh <- fmt.Errorf("failed to cleanup user (worker %d): %w", workerID, err)
					}
				}()

				// Check context after each task
				select {
				case <-opCtx.Done():
					s.logger.Warn("Worker stopping due to context done", "worker", workerID)
					return
				default:
				}
			}
		}(i, ws)
	}

	// Start completion monitor
	go func() {
		wg.Wait()
		close(errCh)
		close(doneCh)
	}()

	// Prepare and send tasks
	go func() {
		// Process users in very small batches to prevent blocking
		batchSize := 5
		for i := 0; i < len(onlineUsers); i += batchSize {
			end := i + batchSize
			if end > len(onlineUsers) {
				end = len(onlineUsers)
			}

			// Process batch
			for _, userID := range onlineUsers[i:end] {
				presenceKey := fmt.Sprintf("presence:%s", userID)
				roomKey := fmt.Sprintf("user:room:%s", userID)

				// Get room ID with minimal timeout
				roomCtx, roomCancel := context.WithTimeout(opCtx, 500*time.Millisecond)
				roomID, err := s.redisClient.Get(roomCtx, roomKey)
				roomCancel()

				// Skip if operation takes too long
				if ctx.Err() == context.DeadlineExceeded {
					s.logger.Debug("Room lookup timed out, skipping", "userId", userID)
					continue
				}

				// Skip if we can't get room info due to timeout
				if err != nil && err.Error() != "redis: nil" && ctx.Err() == context.DeadlineExceeded {
					s.logger.Warn("Skipping user due to room lookup timeout", "userId", userID)
					continue
				}

				if err != nil && err.Error() != "redis: nil" {
					s.logger.Error("Failed to get user room", err, "userId", userID)
					continue
				}

				task := userCleanupTask{
					userID:      userID,
					presenceKey: presenceKey,
					roomID:      roomID,
				}

				select {
				case <-opCtx.Done():
					s.logger.Warn("Task distribution stopped due to context done")
					close(taskCh)
					return
				case taskCh <- task:
				}
			}
		}
		close(taskCh)
	}()

	// Wait for completion or timeout
	select {
	case <-opCtx.Done():
		s.logger.Warn("Stale client cleanup timed out")
		return fmt.Errorf("cleanup timed out")
	case <-doneCh:
		// Collect errors
		var errs []error
		for err := range errCh {
			errs = append(errs, err)
		}
		if len(errs) > 0 {
			s.logger.Error("Some cleanup tasks failed", fmt.Errorf("multiple errors occurred"), "errorCount", len(errs))
		}
		s.logger.Info("Stale client cleanup completed", "checkedUsers", len(onlineUsers), "errors", len(errs))
		return nil
	}
}

// processUserCleanup handles the cleanup of a single user
func (s *MaintenanceService) processUserCleanup(ctx context.Context, wsServer WebSocketServer, task userCleanupTask) error {
	// Check if user is connected
	if wsServer.IsUserConnected(task.userID) {
		// User is connected, remove any disconnect timestamp if it exists
		disconnectKey := fmt.Sprintf("disconnect:%s", task.userID)
		if err := s.redisClient.Del(ctx, disconnectKey); err != nil {
			s.logger.Error("Failed to remove disconnect timestamp", err, "userId", task.userID)
		}
		return nil
	}

	// Get or set disconnect timestamp
	disconnectKey := fmt.Sprintf("disconnect:%s", task.userID)
	disconnectTime, err := s.redisClient.Get(ctx, disconnectKey)
	if err != nil {
		if err.Error() == "redis: nil" {
			// First time seeing this disconnected user, set timestamp
			now := time.Now().Unix()
			if err := s.redisClient.Set(ctx, disconnectKey, strconv.FormatInt(now, 10), 24*time.Hour); err != nil {
				s.logger.Error("Failed to set disconnect timestamp", err, "userId", task.userID)
			}
			return nil // Wait for grace period before cleanup
		}
		return fmt.Errorf("failed to get disconnect timestamp: %w", err)
	}

	// Parse disconnect timestamp
	timestamp, err := strconv.ParseInt(disconnectTime, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid disconnect timestamp: %w", err)
	}

	// Check if grace period (30 seconds) has elapsed
	if time.Since(time.Unix(timestamp, 0)) < 30*time.Second {
		return nil // Still within grace period
	}

	// Double check connection status after grace period
	if wsServer.IsUserConnected(task.userID) {
		if err := s.redisClient.Del(ctx, disconnectKey); err != nil {
			s.logger.Error("Failed to remove disconnect timestamp", err, "userId", task.userID)
		}
		return nil
	}

	// Generate state key for cleanup coordination
	var stateKey string
	if task.roomID != "" {
		userID, err := bson.ObjectIDFromHex(task.userID)
		if err != nil {
			return fmt.Errorf("invalid user ID: %w", err)
		}

		roomID, err := bson.ObjectIDFromHex(task.roomID)
		if err != nil {
			return fmt.Errorf("invalid room ID: %w", err)
		}

		// Final connection check before room cleanup
		if wsServer.IsUserConnected(task.userID) {
			if err := s.redisClient.Del(ctx, disconnectKey); err != nil {
				s.logger.Error("Failed to remove disconnect timestamp", err, "userId", task.userID)
			}
			return nil
		}

		// Set state key for room cleanup
		stateKey = fmt.Sprintf("room:state:%s:%s", roomID, task.userID)

		// Check if we're already handling this room cleanup
		stateCtx, stateCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		hasState, err := s.redisClient.Exists(stateCtx, stateKey)
		stateCancel()

		if err == nil && hasState {
			s.logger.Debug("Room cleanup already in progress",
				"userId", task.userID,
				"roomId", task.roomID)
			return nil
		}

		// Set cleanup state with expiration
		stateCtx, stateCancel = context.WithTimeout(ctx, 500*time.Millisecond)
		err = s.redisClient.Set(stateCtx, stateKey, "cleaning", 30*time.Second)
		stateCancel()

		if err != nil {
			s.logger.Error("Failed to set cleanup state", err,
				"userId", task.userID,
				"roomId", task.roomID)
		}

		// Perform room cleanup with retries
		var cleanupErr error
		for retries := 0; retries < 3; retries++ {
			if wsServer.IsUserConnected(task.userID) {
				s.logger.Debug("User reconnected during cleanup",
					"userId", task.userID,
					"roomId", task.roomID)

				// Clear cleanup state
				if err := s.redisClient.Del(ctx, stateKey); err != nil {
					s.logger.Error("Failed to clear cleanup state", err,
						"userId", task.userID,
						"roomId", task.roomID)
				}
				return nil
			}

			roomCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			err := s.roomManager.LeaveRoom(roomCtx, roomID, userID)
			cancel()

			if err == nil {
				cleanupErr = nil
				break
			}

			cleanupErr = err
			if retries < 2 {
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}

		if cleanupErr != nil {
			s.logger.Error("Failed to remove user from room",
				cleanupErr,
				"userId", task.userID,
				"roomId", task.roomID)
			return cleanupErr
		}
	}

	// Final verification before cleanup
	if wsServer.IsUserConnected(task.userID) {
		if err := s.redisClient.Del(ctx, disconnectKey); err != nil {
			s.logger.Error("Failed to remove disconnect timestamp", err, "userId", task.userID)
		}
		return nil
	}

	// Clean up Redis state with proper state key handling
	redisCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	pipe := s.redisClient.Pipeline()
	pipe.Del(redisCtx, task.presenceKey)
	pipe.Del(redisCtx, disconnectKey)
	if stateKey != "" {
		pipe.Del(redisCtx, stateKey)
	}
	pipe.SRem(redisCtx, "online:users", task.userID)

	_, err = pipe.Exec(redisCtx)
	cancel()

	if err != nil {
		s.logger.Error("Failed to cleanup Redis state",
			err,
			"userId", task.userID)
		return fmt.Errorf("failed to cleanup Redis state: %w", err)
	}

	s.logger.Info("Cleaned up disconnected user after grace period", "userId", task.userID)
	return nil
}

// CleanupCache removes expired cache entries.
func (s *MaintenanceService) CleanupCache(ctx context.Context) error {
	s.logger.Info("Cleaning up cache")

	// Check if Redis client is nil
	if s.redisClient == nil {
		return fmt.Errorf("redis client is nil")
	}

	// Create a timeout context for this operation
	opCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Example: Clean up media cache older than 24 hours
	pattern := "media:cache:*"
	keys, err := s.redisClient.Keys(opCtx, pattern)
	if err != nil {
		return fmt.Errorf("failed to get cache keys: %w", err)
	}

	var deletedCount int
	var errs []error

	for _, key := range keys {
		// Check if key is older than 24 hours
		ttl, err := s.redisClient.TTL(opCtx, key)
		if err != nil {
			s.logger.Error("Failed to get TTL for key", err, "key", key)
			errs = append(errs, fmt.Errorf("failed to get TTL for key %s: %w", key, err))
			continue
		}

		// If TTL is negative, it means the key has no expiration
		if ttl < 0 {
			// Set expiration to 24 hours
			err = s.redisClient.Expire(opCtx, key, 24*time.Hour)
			if err != nil {
				s.logger.Error("Failed to set expiration for key", err, "key", key)
				errs = append(errs, fmt.Errorf("failed to set expiration for key %s: %w", key, err))
				continue
			}
			deletedCount++
		}
	}

	s.logger.Info("Cache cleanup completed", "updatedCount", deletedCount)

	if len(errs) > 0 {
		return fmt.Errorf("cache cleanup completed with errors: %v", errs)
	}

	return nil
}

// RunBackup performs a backup of the database.
func (s *MaintenanceService) RunBackup(ctx context.Context, backupDir string) error {
	s.logger.Info("Running database backup", "dir", backupDir)

	// Check if MongoDB client is nil
	if s.mongoDB == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Create a timeout context for this operation
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("backup_%s.gz", timestamp))

	// Simulate backup process
	s.logger.Info("Backing up database", "file", backupFile)

	// Create an empty backup file for demonstration
	var f *os.File
	var err error

	f, err = os.Create(backupFile)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}

	// Ensure file is closed properly
	defer func() {
		closeErr := f.Close()
		if closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close backup file: %w", closeErr)
		}
	}()

	// In a real implementation, you would use mongodump or similar tool here
	// For example:
	// cmd := exec.CommandContext(opCtx, "mongodump", "--uri", uri, "--gzip", "--archive="+backupFile)
	// if err := cmd.Run(); err != nil {
	//     return fmt.Errorf("mongodump failed: %w", err)
	// }

	// Simulate work
	select {
	case <-time.After(2 * time.Second):
		// Completed normally
	case <-opCtx.Done():
		return fmt.Errorf("backup operation timed out or was cancelled")
	}

	s.logger.Info("Database backup completed", "file", backupFile)
	return err
}

// PerformMaintenance runs a specific maintenance task by name.
func (s *MaintenanceService) PerformMaintenance(ctx context.Context, taskName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, task := range s.tasks {
		if task.Name == taskName {
			s.logger.Info("Running maintenance task", "name", taskName)
			if err := task.Fn(ctx); err != nil {
				return fmt.Errorf("failed to run maintenance task %s: %w", taskName, err)
			}
			task.LastRun = time.Now()
			s.logger.Info("Completed maintenance task", "name", taskName)
			return nil
		}
	}

	return fmt.Errorf("maintenance task not found: %s", taskName)
}
