// Package system provides system-level services for monitoring and maintenance.
package system

import (
	"context"
	"runtime"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"norelock.dev/listenify/backend/internal/db/redis"
	"norelock.dev/listenify/backend/internal/utils"
)

// HealthStatus represents the health status of a component.
type HealthStatus string

const (
	// StatusUp indicates the component is healthy.
	StatusUp HealthStatus = "up"
	// StatusDown indicates the component is unhealthy.
	StatusDown HealthStatus = "down"
	// StatusDegraded indicates the component is functioning but with issues.
	StatusDegraded HealthStatus = "degraded"
)

// ComponentHealth represents the health of a system component.
type ComponentHealth struct {
	Name        string       `json:"name"`
	Status      HealthStatus `json:"status"`
	Description string       `json:"description,omitempty"`
	Latency     int64        `json:"latency_ms,omitempty"` // Response time in milliseconds
	LastChecked time.Time    `json:"last_checked"`
}

// SystemHealth represents the overall health of the system.
type SystemHealth struct {
	Status      HealthStatus      `json:"status"`
	Components  []ComponentHealth `json:"components"`
	Version     string            `json:"version"`
	Environment string            `json:"environment"`
	Uptime      int64             `json:"uptime_seconds"`
	StartTime   time.Time         `json:"start_time"`
	GoVersion   string            `json:"go_version"`
	GoRoutines  int               `json:"go_routines"`
	MemStats    MemoryStats       `json:"memory_stats"`
}

// MemoryStats represents memory usage statistics.
type MemoryStats struct {
	Alloc      uint64 `json:"alloc_bytes"`       // Bytes allocated and still in use
	TotalAlloc uint64 `json:"total_alloc_bytes"` // Bytes allocated (even if freed)
	Sys        uint64 `json:"sys_bytes"`         // Bytes obtained from system
	NumGC      uint32 `json:"num_gc"`            // Number of completed GC cycles
	HeapAlloc  uint64 `json:"heap_alloc_bytes"`  // Bytes allocated and still in use
	HeapSys    uint64 `json:"heap_sys_bytes"`    // Bytes obtained from system for heap
}

// HealthService provides health checking functionality.
type HealthService struct {
	mongoClient    *mongo.Client
	redisClient    *redis.Client
	logger         *utils.Logger
	startTime      time.Time
	version        string
	environment    string
	componentCache map[string]ComponentHealth
	cacheMutex     sync.RWMutex
	checkInterval  time.Duration
}

// HealthServiceConfig contains configuration for the health service.
type HealthServiceConfig struct {
	Version     string
	Environment string
}

// NewHealthService creates a new health service.
func NewHealthService(
	mongoClient *mongo.Client,
	redisClient *redis.Client,
	logger *utils.Logger,
	config HealthServiceConfig,
) *HealthService {
	return &HealthService{
		mongoClient:    mongoClient,
		redisClient:    redisClient,
		logger:         logger.Named("health_service"),
		startTime:      time.Now(),
		version:        config.Version,
		environment:    config.Environment,
		componentCache: make(map[string]ComponentHealth),
		checkInterval:  30 * time.Second, // Check components every 30 seconds
	}
}

// Start begins periodic health checks.
func (s *HealthService) Start(ctx context.Context) {
	s.logger.Info("Starting health service")

	// Perform initial health check
	s.CheckHealth(ctx)

	// Start periodic health checks
	go func() {
		ticker := time.NewTicker(s.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Stopping health service")
				return
			case <-ticker.C:
				s.CheckHealth(ctx)
			}
		}
	}()
}

// CheckHealth performs a health check on all system components.
func (s *HealthService) CheckHealth(ctx context.Context) {
	s.logger.Debug("Performing health check")

	// Check MongoDB
	s.checkMongoDB(ctx)

	// Check Redis
	s.checkRedis(ctx)
}

// GetHealth returns the current health status of the system.
func (s *HealthService) GetHealth(ctx context.Context) SystemHealth {
	s.cacheMutex.RLock()
	defer s.cacheMutex.RUnlock()

	// Collect all component statuses
	components := make([]ComponentHealth, 0, len(s.componentCache))
	for _, component := range s.componentCache {
		components = append(components, component)
	}

	// Determine overall status
	status := StatusUp
	for _, component := range components {
		if component.Status == StatusDown {
			status = StatusDown
			break
		} else if component.Status == StatusDegraded && status != StatusDown {
			status = StatusDegraded
		}
	}

	// Get memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return SystemHealth{
		Status:      status,
		Components:  components,
		Version:     s.version,
		Environment: s.environment,
		Uptime:      int64(time.Since(s.startTime).Seconds()),
		StartTime:   s.startTime,
		GoVersion:   runtime.Version(),
		GoRoutines:  runtime.NumGoroutine(),
		MemStats: MemoryStats{
			Alloc:      memStats.Alloc,
			TotalAlloc: memStats.TotalAlloc,
			Sys:        memStats.Sys,
			NumGC:      memStats.NumGC,
			HeapAlloc:  memStats.HeapAlloc,
			HeapSys:    memStats.HeapSys,
		},
	}
}

// checkMongoDB checks the health of the MongoDB connection.
func (s *HealthService) checkMongoDB(ctx context.Context) {
	componentName := "mongodb"
	start := time.Now()

	// Create a context with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Ping MongoDB
	err := s.mongoClient.Ping(pingCtx, nil)
	latency := time.Since(start).Milliseconds()

	// Update component status
	status := StatusUp
	description := "MongoDB connection is healthy"

	if err != nil {
		status = StatusDown
		description = "Failed to connect to MongoDB: " + err.Error()
		s.logger.Error("MongoDB health check failed", err)
	}

	s.updateComponentHealth(componentName, status, description, latency)
}

// checkRedis checks the health of the Redis connection.
func (s *HealthService) checkRedis(ctx context.Context) {
	componentName := "redis"
	start := time.Now()

	// Create a context with timeout
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Ping Redis
	err := s.redisClient.Ping(pingCtx)
	latency := time.Since(start).Milliseconds()

	// Update component status
	status := StatusUp
	description := "Redis connection is healthy"

	if err != nil {
		status = StatusDown
		description = "Failed to connect to Redis: " + err.Error()
		s.logger.Error("Redis health check failed", err)
	}

	s.updateComponentHealth(componentName, status, description, latency)
}

// updateComponentHealth updates the health status of a component in the cache.
func (s *HealthService) updateComponentHealth(name string, status HealthStatus, description string, latency int64) {
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.componentCache[name] = ComponentHealth{
		Name:        name,
		Status:      status,
		Description: description,
		Latency:     latency,
		LastChecked: time.Now(),
	}
}
