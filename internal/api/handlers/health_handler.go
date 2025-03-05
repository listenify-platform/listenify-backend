// Package handlers contains HTTP handlers for the API.
package handlers

import (
	"net/http"
	"runtime"
	"time"

	"norelock.dev/listenify/backend/internal/config"
	"norelock.dev/listenify/backend/internal/services/system"
	"norelock.dev/listenify/backend/internal/utils"
)

// HealthHandler handles HTTP requests related to system health.
type HealthHandler struct {
	logger    *utils.Logger
	healthSvc *system.HealthService
	config    *config.Config
	startTime time.Time
	version   string
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(
	logger *utils.Logger,
	healthSvc *system.HealthService,
	config *config.Config,
) *HealthHandler {
	return &HealthHandler{
		logger:    logger.Named("health_handler"),
		healthSvc: healthSvc,
		config:    config,
		startTime: time.Now(),
		version:   "1.0.0", // TODO: Get from build info
	}
}

// Check handles requests to check the health of the system.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	// Get system health from the health service
	health := h.healthSvc.GetHealth(r.Context())

	// Calculate uptime
	uptime := time.Since(h.startTime).String()

	// Create response
	response := map[string]any{
		"status":     health.Status,
		"version":    h.version,
		"uptime":     uptime,
		"memory":     health.MemStats,
		"components": health.Components,
		"goroutines": health.GoRoutines,
		"startTime":  health.StartTime,
	}

	// Set appropriate status code based on health status
	statusCode := http.StatusOK
	if health.Status != system.StatusUp {
		statusCode = http.StatusServiceUnavailable
	}

	utils.RespondWithJSON(w, statusCode, response)
}

// DetailedCheck handles requests for detailed health information.
// This endpoint is typically protected and only accessible to admins.
func (h *HealthHandler) DetailedCheck(w http.ResponseWriter, r *http.Request) {
	// Get detailed system health
	health := h.healthSvc.GetHealth(r.Context())

	// Add additional detailed information
	detailedResponse := map[string]any{
		"health":       health,
		"uptime":       time.Since(h.startTime).String(),
		"startTime":    h.startTime,
		"environment":  h.config.Environment,
		"buildInfo":    h.getBuildInfo(),
		"configStatus": h.getConfigStatus(),
	}

	// Set appropriate status code based on health status
	statusCode := http.StatusOK
	if health.Status != system.StatusUp {
		statusCode = http.StatusServiceUnavailable
	}

	utils.RespondWithJSON(w, statusCode, detailedResponse)
}

// getBuildInfo returns information about the build.
func (h *HealthHandler) getBuildInfo() map[string]any {
	return map[string]any{
		"version":   h.version,
		"goVersion": runtime.Version(),
	}
}

// getConfigStatus returns the status of the configuration.
func (h *HealthHandler) getConfigStatus() map[string]any {
	return map[string]any{
		"environment": h.config.Environment,
		"features":    h.config.Features,
		"loaded":      true,
	}
}
