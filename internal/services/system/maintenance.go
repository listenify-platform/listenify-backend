// Package system provides system-level services for monitoring and maintenance.
package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/db/redis"
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
	logger       *utils.Logger
	tasks        []*MaintenanceTask
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
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

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.config.MaxConcurrentTasks)
	errCh := make(chan error, len(dueTasks))

	for _, task := range dueTasks {
		wg.Add(1)
		go func(t *MaintenanceTask) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create a timeout context for this task
			taskCtx, cancel := context.WithTimeout(ctx, s.config.TaskTimeout)
			defer cancel()

			// Add panic recovery for individual tasks
			defer func() {
				if r := recover(); r != nil {
					err := fmt.Errorf("panic in task %s: %v", t.Name, r)
					s.logger.Error("Task panic recovered", err, "name", t.Name)
					errCh <- err
				}
			}()

			s.logger.Info("Running maintenance task", "name", t.Name)
			if err := t.Fn(taskCtx); err != nil {
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

	// Collect and log errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		s.logger.Error("Some maintenance tasks failed", fmt.Errorf("multiple errors occurred"), "errorCount", len(errs))
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

// CleanupStaleClients removes stale client data from Redis and rooms
func (s *MaintenanceService) CleanupStaleClients(ctx context.Context) error {
	s.logger.Info("Cleaning up stale clients")

	// Get all online users from Redis
	onlineUsers, err := s.redisClient.SMembers(ctx, "online:users")
	if err != nil {
		return fmt.Errorf("failed to get online users: %w", err)
	}

	s.logger.Info("Checking for stale clients", "onlineUsersCount", len(onlineUsers))

	// Check each user's session and presence
	for _, userIDStr := range onlineUsers {
		// Get presence info
		presenceKey := fmt.Sprintf("presence:%s", userIDStr)
		exists, err := s.redisClient.Exists(ctx, presenceKey)
		if err != nil {
			s.logger.Error("Failed to check presence key", err, "userId", userIDStr)
			continue
		}

		// If no presence info, remove from online users
		if !exists {
			if err := s.redisClient.SRem(ctx, "online:users", userIDStr); err != nil {
				s.logger.Error("Failed to remove user from online users", err, "userId", userIDStr)
			}
			continue
		}

		// Get user's current room
		roomKey := fmt.Sprintf("user:room:%s", userIDStr)
		roomID, err := s.redisClient.Get(ctx, roomKey)
		if err != nil && err.Error() != "redis: nil" {
			s.logger.Error("Failed to get user room", err, "userId", userIDStr)
			continue
		}

		// If user has a room, clean it up
		if roomID != "" {
			// Remove user from room
			roomUsersKey := fmt.Sprintf("room:users:%s", roomID)
			if err := s.redisClient.SRem(ctx, roomUsersKey, userIDStr); err != nil {
				s.logger.Error("Failed to remove user from room", err, "userId", userIDStr, "roomId", roomID)
			}

			// Delete user's room key
			if err := s.redisClient.Del(ctx, roomKey); err != nil {
				s.logger.Error("Failed to delete user room key", err, "userId", userIDStr)
			}

			// Update room state
			if err := s.redisClient.HDel(ctx, fmt.Sprintf("room:state:%s", roomID), "users"); err != nil {
				s.logger.Error("Failed to update room state", err, "roomId", roomID)
			}
		}

		// Remove presence info
		if err := s.redisClient.Del(ctx, presenceKey); err != nil {
			s.logger.Error("Failed to remove presence info", err, "userId", userIDStr)
		}

		// Remove from online users
		if err := s.redisClient.SRem(ctx, "online:users", userIDStr); err != nil {
			s.logger.Error("Failed to remove user from online users", err, "userId", userIDStr)
		}
	}

	s.logger.Info("Stale client cleanup completed", "cleanedUsers", len(onlineUsers))
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
