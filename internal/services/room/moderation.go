// Package room provides functionality for managing rooms and their state.
package room

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/utils"
)

// ReportReason represents the reason for a user report.
type ReportReason string

const (
	// ReportReasonSpam indicates spam content.
	ReportReasonSpam ReportReason = "spam"
	// ReportReasonHarassment indicates harassment.
	ReportReasonHarassment ReportReason = "harassment"
	// ReportReasonHateSpeech indicates hate speech.
	ReportReasonHateSpeech ReportReason = "hate_speech"
	// ReportReasonInappropriateContent indicates inappropriate content.
	ReportReasonInappropriateContent ReportReason = "inappropriate_content"
	// ReportReasonOther indicates other reasons.
	ReportReasonOther ReportReason = "other"
)

// BanDuration represents the duration of a ban.
type BanDuration string

const (
	// BanDuration1Hour represents a 1-hour ban.
	BanDuration1Hour BanDuration = "1h"
	// BanDuration24Hours represents a 24-hour ban.
	BanDuration24Hours BanDuration = "24h"
	// BanDuration7Days represents a 7-day ban.
	BanDuration7Days BanDuration = "7d"
	// BanDuration30Days represents a 30-day ban.
	BanDuration30Days BanDuration = "30d"
	// BanDurationPermanent represents a permanent ban.
	BanDurationPermanent BanDuration = "permanent"
)

// ModerationAction represents a moderation action.
type ModerationAction string

const (
	// ModerationActionWarn indicates a warning.
	ModerationActionWarn ModerationAction = "warn"
	// ModerationActionMute indicates a mute.
	ModerationActionMute ModerationAction = "mute"
	// ModerationActionKick indicates a kick from the room.
	ModerationActionKick ModerationAction = "kick"
	// ModerationActionBan indicates a ban.
	ModerationActionBan ModerationAction = "ban"
	// ModerationActionUnban indicates an unban.
	ModerationActionUnban ModerationAction = "unban"
	// ModerationActionDeleteMessage indicates message deletion.
	ModerationActionDeleteMessage ModerationAction = "delete_message"
)

// UserReport represents a report submitted by a user.
type UserReport struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	ReporterID  string        `bson:"reporter_id" json:"reporter_id"`
	ReportedID  string        `bson:"reported_id" json:"reported_id"`
	RoomID      string        `bson:"room_id" json:"room_id"`
	Reason      ReportReason  `bson:"reason" json:"reason"`
	Description string        `bson:"description" json:"description"`
	Timestamp   time.Time     `bson:"timestamp" json:"timestamp"`
	Status      string        `bson:"status" json:"status"` // "pending", "resolved", "rejected"
	Resolution  string        `bson:"resolution,omitempty" json:"resolution,omitempty"`
	ResolvedBy  string        `bson:"resolved_by,omitempty" json:"resolved_by,omitempty"`
	ResolvedAt  time.Time     `bson:"resolved_at,omitempty" json:"resolved_at,omitzero"`
}

// UserBan represents a ban applied to a user.
type UserBan struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	UserID      string        `bson:"user_id" json:"user_id"`
	RoomID      string        `bson:"room_id,omitempty" json:"room_id,omitempty"` // Empty for global bans
	ModeratorID string        `bson:"moderator_id" json:"moderator_id"`
	Reason      string        `bson:"reason" json:"reason"`
	Duration    BanDuration   `bson:"duration" json:"duration"`
	StartTime   time.Time     `bson:"start_time" json:"start_time"`
	EndTime     time.Time     `bson:"end_time,omitempty" json:"end_time,omitzero"` // Empty for permanent bans
	Active      bool          `bson:"active" json:"active"`
}

// ModerationLog represents a log entry for a moderation action.
type ModerationLog struct {
	ID          bson.ObjectID    `bson:"_id,omitempty" json:"id,omitempty"`
	Action      ModerationAction `bson:"action" json:"action"`
	UserID      string           `bson:"user_id" json:"user_id"`
	ModeratorID string           `bson:"moderator_id" json:"moderator_id"`
	RoomID      string           `bson:"room_id,omitempty" json:"room_id,omitempty"`
	Reason      string           `bson:"reason" json:"reason"`
	Timestamp   time.Time        `bson:"timestamp" json:"timestamp"`
	Details     string           `bson:"details,omitempty" json:"details,omitempty"`
}

// ModerationService provides moderation functionality for rooms.
type ModerationService struct {
	db             *mongo.Database
	roomRepo       repositories.RoomRepository
	userRepo       repositories.UserRepository
	roomState      *managers.RoomStateManager
	pubsub         *managers.PubSubManager
	logger         *utils.Logger
	activeBans     map[string]map[string]*UserBan // roomID -> userID -> ban
	bansMutex      sync.RWMutex
	reportHandlers []func(context.Context, *UserReport) error
}

// NewModerationService creates a new moderation service.
func NewModerationService(
	db *mongo.Database,
	roomRepo repositories.RoomRepository,
	userRepo repositories.UserRepository,
	roomState *managers.RoomStateManager,
	pubsub *managers.PubSubManager,
	logger *utils.Logger,
) *ModerationService {
	return &ModerationService{
		db:         db,
		roomRepo:   roomRepo,
		userRepo:   userRepo,
		roomState:  roomState,
		pubsub:     pubsub,
		logger:     logger.Named("moderation_service"),
		activeBans: make(map[string]map[string]*UserBan),
	}
}

// Start initializes the moderation service.
func (s *ModerationService) Start(ctx context.Context) error {
	s.logger.Info("Starting moderation service")

	// Load active bans
	if err := s.loadActiveBans(ctx); err != nil {
		return fmt.Errorf("failed to load active bans: %w", err)
	}

	// Subscribe to moderation events
	err := s.pubsub.Subscribe("moderation:*")
	if err != nil {
		return fmt.Errorf("failed to subscribe to moderation events: %w", err)
	}

	// Add handler for moderation events
	s.pubsub.AddHandler("moderation:*", func(channel string, payload []byte) {
		var event map[string]any
		if err := json.Unmarshal(payload, &event); err != nil {
			s.logger.Error("Failed to unmarshal moderation event", err)
			return
		}

		// Handle different event types
		eventType, ok := event["type"].(string)
		if !ok {
			s.logger.Error("Invalid moderation event type", fmt.Errorf("invalid event type"))
			return
		}

		switch eventType {
		case "ban_user":
			s.handleBanUserEvent(ctx, event)
		case "unban_user":
			s.handleUnbanUserEvent(ctx, event)
		case "report_user":
			s.handleReportUserEvent(ctx, event)
		}
	})

	return nil
}

// loadActiveBans loads all active bans from the database.
func (s *ModerationService) loadActiveBans(ctx context.Context) error {
	s.bansMutex.Lock()
	defer s.bansMutex.Unlock()

	// Clear existing bans
	s.activeBans = make(map[string]map[string]*UserBan)

	// Query for active bans
	filter := bson.M{"active": true}
	cursor, err := s.db.Collection("user_bans").Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to query active bans: %w", err)
	}
	defer cursor.Close(ctx)

	// Process each ban
	for cursor.Next(ctx) {
		var ban UserBan
		if err := cursor.Decode(&ban); err != nil {
			s.logger.Error("Failed to decode ban", err)
			continue
		}

		// Check if ban has expired
		if ban.EndTime.Before(time.Now()) && ban.Duration != BanDurationPermanent {
			// Update ban to inactive
			update := bson.M{"$set": bson.M{"active": false}}
			_, err := s.db.Collection("user_bans").UpdateByID(ctx, ban.ID, update)
			if err != nil {
				s.logger.Error("Failed to update expired ban", err)
			}
			continue
		}

		// Add to active bans
		roomID := ban.RoomID
		if roomID == "" {
			roomID = "global"
		}

		if _, exists := s.activeBans[roomID]; !exists {
			s.activeBans[roomID] = make(map[string]*UserBan)
		}

		s.activeBans[roomID][ban.UserID] = &ban
	}

	s.logger.Info("Loaded active bans", "count", len(s.activeBans))
	return nil
}

// handleBanUserEvent handles a ban user event.
func (s *ModerationService) handleBanUserEvent(ctx context.Context, event map[string]any) {
	// Extract event data
	userID, ok := event["user_id"].(string)
	if !ok {
		s.logger.Error("Invalid user ID in ban event", fmt.Errorf("invalid user ID"))
		return
	}

	moderatorID, ok := event["moderator_id"].(string)
	if !ok {
		s.logger.Error("Invalid moderator ID in ban event", fmt.Errorf("invalid moderator ID"))
		return
	}

	roomID, _ := event["room_id"].(string)
	reason, _ := event["reason"].(string)
	durationStr, _ := event["duration"].(string)

	// Convert duration
	duration := BanDuration(durationStr)
	if duration == "" {
		duration = BanDuration24Hours
	}

	// Ban the user
	_, err := s.BanUser(ctx, userID, roomID, moderatorID, reason, duration)
	if err != nil {
		s.logger.Error("Failed to ban user from event", err)
	}
}

// handleUnbanUserEvent handles an unban user event.
func (s *ModerationService) handleUnbanUserEvent(ctx context.Context, event map[string]any) {
	// Extract event data
	userID, ok := event["user_id"].(string)
	if !ok {
		s.logger.Error("Invalid user ID in unban event", fmt.Errorf("invalid user ID"))
		return
	}

	moderatorID, ok := event["moderator_id"].(string)
	if !ok {
		s.logger.Error("Invalid moderator ID in unban event", fmt.Errorf("invalid moderator ID"))
		return
	}

	roomID, _ := event["room_id"].(string)
	reason, _ := event["reason"].(string)

	// Unban the user
	err := s.UnbanUser(ctx, userID, roomID, moderatorID, reason)
	if err != nil {
		s.logger.Error("Failed to unban user from event", err)
	}
}

// handleReportUserEvent handles a report user event.
func (s *ModerationService) handleReportUserEvent(ctx context.Context, event map[string]any) {
	// Extract event data
	reporterID, ok := event["reporter_id"].(string)
	if !ok {
		s.logger.Error("Invalid reporter ID in report event", fmt.Errorf("invalid reporter ID"))
		return
	}

	reportedID, ok := event["reported_id"].(string)
	if !ok {
		s.logger.Error("Invalid reported ID in report event", fmt.Errorf("invalid reported ID"))
		return
	}

	roomID, _ := event["room_id"].(string)
	reasonStr, _ := event["reason"].(string)
	description, _ := event["description"].(string)

	// Convert reason
	reason := ReportReason(reasonStr)
	if reason == "" {
		reason = ReportReasonOther
	}

	// Create the report
	_, err := s.ReportUser(ctx, reporterID, reportedID, roomID, reason, description)
	if err != nil {
		s.logger.Error("Failed to create report from event", err)
	}
}

// ReportUser creates a new user report.
func (s *ModerationService) ReportUser(
	ctx context.Context,
	reporterID, reportedID, roomID string,
	reason ReportReason,
	description string,
) (*UserReport, error) {
	// Validate inputs
	if reporterID == "" || reportedID == "" {
		return nil, fmt.Errorf("reporter ID and reported ID are required")
	}

	// Create report
	report := &UserReport{
		ReporterID:  reporterID,
		ReportedID:  reportedID,
		RoomID:      roomID,
		Reason:      reason,
		Description: description,
		Timestamp:   time.Now(),
		Status:      "pending",
	}

	// Insert into database
	result, err := s.db.Collection("user_reports").InsertOne(ctx, report)
	if err != nil {
		return nil, fmt.Errorf("failed to insert report: %w", err)
	}

	// Set ID from insert result
	report.ID = result.InsertedID.(bson.ObjectID)

	s.logger.Info("Created user report", "id", report.ID, "reporter", reporterID, "reported", reportedID)

	// Notify report handlers
	for _, handler := range s.reportHandlers {
		go func(h func(context.Context, *UserReport) error) {
			if err := h(ctx, report); err != nil {
				s.logger.Error("Report handler failed", err)
			}
		}(handler)
	}

	return report, nil
}

// AddReportHandler adds a handler for new reports.
func (s *ModerationService) AddReportHandler(handler func(context.Context, *UserReport) error) {
	s.reportHandlers = append(s.reportHandlers, handler)
}

// GetReports retrieves reports based on the provided filter.
func (s *ModerationService) GetReports(
	ctx context.Context,
	filter bson.M,
	page, pageSize int,
	sortField string,
	sortOrder int,
) ([]*UserReport, int64, error) {
	// Set up options
	opts := options.Find()
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}

	if sortField != "" {
		opts.SetSort(bson.D{{Key: sortField, Value: sortOrder}})
	} else {
		opts.SetSort(bson.D{{Key: "timestamp", Value: -1}})
	}

	// Count total
	total, err := s.db.Collection("user_reports").CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count reports: %w", err)
	}

	// Execute query
	cursor, err := s.db.Collection("user_reports").Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query reports: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	reports := make([]*UserReport, 0)
	for cursor.Next(ctx) {
		var report UserReport
		if err := cursor.Decode(&report); err != nil {
			s.logger.Error("Failed to decode report", err)
			continue
		}
		reports = append(reports, &report)
	}

	return reports, total, nil
}

// ResolveReport resolves a user report.
func (s *ModerationService) ResolveReport(
	ctx context.Context,
	reportID bson.ObjectID,
	moderatorID, resolution, status string,
) error {
	// Validate inputs
	if reportID.IsZero() || moderatorID == "" {
		return fmt.Errorf("report ID and moderator ID are required")
	}

	if status != "resolved" && status != "rejected" {
		status = "resolved"
	}

	// Update report
	update := bson.M{
		"$set": bson.M{
			"status":      status,
			"resolution":  resolution,
			"resolved_by": moderatorID,
			"resolved_at": time.Now(),
		},
	}

	result, err := s.db.Collection("user_reports").UpdateByID(ctx, reportID, update)
	if err != nil {
		return fmt.Errorf("failed to update report: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("report not found: %s", reportID.Hex())
	}

	s.logger.Info("Resolved user report", "id", reportID, "moderator", moderatorID, "status", status)
	return nil
}

// BanUser bans a user from a room or globally.
func (s *ModerationService) BanUser(
	ctx context.Context,
	userID, roomID, moderatorID, reason string,
	duration BanDuration,
) (*UserBan, error) {
	// Validate inputs
	if userID == "" || moderatorID == "" {
		return nil, fmt.Errorf("user ID and moderator ID are required")
	}

	// Check if user exists
	userObjID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	user, err := s.userRepo.FindByID(ctx, userObjID)
	if err != nil {
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	if user == nil {
		return nil, fmt.Errorf("user not found: %s", userID)
	}

	// Check if room exists (if room-specific ban)
	if roomID != "" {
		roomObjID, err := bson.ObjectIDFromHex(roomID)
		if err != nil {
			return nil, fmt.Errorf("invalid room ID format: %w", err)
		}

		room, err := s.roomRepo.FindByID(ctx, roomObjID)
		if err != nil {
			return nil, fmt.Errorf("failed to find room: %w", err)
		}

		if room == nil {
			return nil, fmt.Errorf("room not found: %s", roomID)
		}
	}

	// Calculate end time based on duration
	startTime := time.Now()
	var endTime time.Time

	switch duration {
	case BanDuration1Hour:
		endTime = startTime.Add(time.Hour)
	case BanDuration24Hours:
		endTime = startTime.Add(24 * time.Hour)
	case BanDuration7Days:
		endTime = startTime.Add(7 * 24 * time.Hour)
	case BanDuration30Days:
		endTime = startTime.Add(30 * 24 * time.Hour)
	case BanDurationPermanent:
		// No end time for permanent bans
	default:
		// Default to 24 hours
		duration = BanDuration24Hours
		endTime = startTime.Add(24 * time.Hour)
	}

	// Create ban
	ban := &UserBan{
		UserID:      userID,
		RoomID:      roomID,
		ModeratorID: moderatorID,
		Reason:      reason,
		Duration:    duration,
		StartTime:   startTime,
		EndTime:     endTime,
		Active:      true,
	}

	// Insert into database
	result, err := s.db.Collection("user_bans").InsertOne(ctx, ban)
	if err != nil {
		return nil, fmt.Errorf("failed to insert ban: %w", err)
	}

	// Set ID from insert result
	ban.ID = result.InsertedID.(bson.ObjectID)

	// Add to active bans
	s.bansMutex.Lock()
	banRoomID := roomID
	if banRoomID == "" {
		banRoomID = "global"
	}

	if _, exists := s.activeBans[banRoomID]; !exists {
		s.activeBans[banRoomID] = make(map[string]*UserBan)
	}

	s.activeBans[banRoomID][userID] = ban
	s.bansMutex.Unlock()

	// If room-specific ban, remove user from room
	if roomID != "" {
		err = s.roomState.RemoveUserFromRoom(ctx, roomID, userID)
		if err != nil {
			s.logger.Error("Failed to remove banned user from room", err)
			// Continue anyway as the ban was successfully created
		}
	}

	// Log moderation action
	s.logModerationAction(ctx, ModerationActionBan, userID, moderatorID, roomID, reason, "")

	s.logger.Info("Banned user", "id", ban.ID, "user", userID, "room", roomID, "duration", duration)
	return ban, nil
}

// UnbanUser removes a ban for a user.
func (s *ModerationService) UnbanUser(
	ctx context.Context,
	userID, roomID, moderatorID, reason string,
) error {
	// Validate inputs
	if userID == "" || moderatorID == "" {
		return fmt.Errorf("user ID and moderator ID are required")
	}

	// Find active ban
	filter := bson.M{
		"user_id": userID,
		"active":  true,
	}

	if roomID != "" {
		filter["room_id"] = roomID
	} else {
		filter["room_id"] = ""
	}

	// Update ban to inactive
	update := bson.M{
		"$set": bson.M{
			"active": false,
		},
	}

	result, err := s.db.Collection("user_bans").UpdateMany(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update ban: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no active ban found for user: %s", userID)
	}

	// Remove from active bans
	s.bansMutex.Lock()
	banRoomID := roomID
	if banRoomID == "" {
		banRoomID = "global"
	}

	if bans, exists := s.activeBans[banRoomID]; exists {
		delete(bans, userID)
	}
	s.bansMutex.Unlock()

	// Log moderation action
	s.logModerationAction(ctx, ModerationActionUnban, userID, moderatorID, roomID, reason, "")

	s.logger.Info("Unbanned user", "user", userID, "room", roomID, "moderator", moderatorID)
	return nil
}

// IsUserBanned checks if a user is banned from a room or globally.
func (s *ModerationService) IsUserBanned(ctx context.Context, userID, roomID string) (bool, *UserBan, error) {
	s.bansMutex.RLock()
	defer s.bansMutex.RUnlock()

	// Check for global ban
	if globalBans, exists := s.activeBans["global"]; exists {
		if ban, banned := globalBans[userID]; banned {
			return true, ban, nil
		}
	}

	// If no room ID provided, only check global bans
	if roomID == "" {
		return false, nil, nil
	}

	// Check for room-specific ban
	if roomBans, exists := s.activeBans[roomID]; exists {
		if ban, banned := roomBans[userID]; banned {
			return true, ban, nil
		}
	}

	return false, nil, nil
}

// GetActiveBans retrieves active bans based on the provided filter.
func (s *ModerationService) GetActiveBans(
	ctx context.Context,
	roomID string,
	page, pageSize int,
) ([]*UserBan, int64, error) {
	// Set up filter
	filter := bson.M{"active": true}
	if roomID != "" {
		filter["room_id"] = roomID
	}

	// Set up options
	opts := options.Find().SetSort(bson.D{{Key: "start_time", Value: -1}})
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}

	// Count total
	total, err := s.db.Collection("user_bans").CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count bans: %w", err)
	}

	// Execute query
	cursor, err := s.db.Collection("user_bans").Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query bans: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	bans := make([]*UserBan, 0)
	for cursor.Next(ctx) {
		var ban UserBan
		if err := cursor.Decode(&ban); err != nil {
			s.logger.Error("Failed to decode ban", err)
			continue
		}
		bans = append(bans, &ban)
	}

	return bans, total, nil
}

// MuteUser mutes a user in a room.
func (s *ModerationService) MuteUser(
	ctx context.Context,
	userID, roomID, moderatorID, reason string,
	duration time.Duration,
) error {
	// Validate inputs
	if userID == "" || roomID == "" || moderatorID == "" {
		return fmt.Errorf("user ID, room ID, and moderator ID are required")
	}

	// Check if user is in the room
	inRoom, err := s.roomState.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to check if user is in room: %w", err)
	}

	if !inRoom {
		return fmt.Errorf("user is not in the room: %s", userID)
	}

	// Get room state
	roomState, err := s.roomState.GetRoomState(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to get room state: %w", err)
	}

	if roomState == nil {
		return fmt.Errorf("room state not found: %s", roomID)
	}

	// Update room state with muted user
	if roomState.Data == nil {
		roomState.Data = make(map[string]any)
	}

	// Get or create muted users map
	var mutedUsers map[string]time.Time
	if muteData, exists := roomState.Data["muted_users"]; exists {
		// Try to convert existing data
		if muteJSON, ok := muteData.(json.RawMessage); ok {
			if err := json.Unmarshal(muteJSON, &mutedUsers); err != nil {
				mutedUsers = make(map[string]time.Time)
			}
		} else {
			mutedUsers = make(map[string]time.Time)
		}
	} else {
		mutedUsers = make(map[string]time.Time)
	}

	// Set mute end time
	mutedUsers[userID] = time.Now().Add(duration)

	// Update room state
	muteJSON, err := json.Marshal(mutedUsers)
	if err != nil {
		return fmt.Errorf("failed to marshal muted users: %w", err)
	}

	roomState.Data["muted_users"] = json.RawMessage(muteJSON)
	if err := s.roomState.UpdateRoomState(ctx, roomState); err != nil {
		return fmt.Errorf("failed to update room state: %w", err)
	}

	// Log moderation action
	details := fmt.Sprintf("Duration: %s", duration.String())
	s.logModerationAction(ctx, ModerationActionMute, userID, moderatorID, roomID, reason, details)

	// Notify room of mute
	event := map[string]any{
		"type":         "user_muted",
		"user_id":      userID,
		"moderator_id": moderatorID,
		"room_id":      roomID,
		"duration":     duration.String(),
		"end_time":     time.Now().Add(duration),
	}

	if err := s.pubsub.PublishToRoom(ctx, roomID, "moderation", event); err != nil {
		s.logger.Error("Failed to publish mute event", err)
		// Continue anyway as the mute was successfully applied
	}

	s.logger.Info("Muted user", "user", userID, "room", roomID, "duration", duration.String())
	return nil
}

// UnmuteUser removes a mute for a user in a room.
func (s *ModerationService) UnmuteUser(
	ctx context.Context,
	userID, roomID, moderatorID, reason string,
) error {
	// Validate inputs
	if userID == "" || roomID == "" || moderatorID == "" {
		return fmt.Errorf("user ID, room ID, and moderator ID are required")
	}

	// Get room state
	roomState, err := s.roomState.GetRoomState(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to get room state: %w", err)
	}

	if roomState == nil {
		return fmt.Errorf("room state not found: %s", roomID)
	}

	// Check if user is muted
	if roomState.Data == nil || roomState.Data["muted_users"] == nil {
		return fmt.Errorf("user is not muted: %s", userID)
	}

	// Get muted users map
	var mutedUsers map[string]time.Time
	if muteData, exists := roomState.Data["muted_users"]; exists {
		// Try to convert existing data
		if muteJSON, ok := muteData.(json.RawMessage); ok {
			if err := json.Unmarshal(muteJSON, &mutedUsers); err != nil {
				return fmt.Errorf("failed to unmarshal muted users: %w", err)
			}
		} else {
			return fmt.Errorf("invalid muted users format")
		}
	} else {
		return fmt.Errorf("user is not muted: %s", userID)
	}

	// Check if user is in muted list
	if _, exists := mutedUsers[userID]; !exists {
		return fmt.Errorf("user is not muted: %s", userID)
	}

	// Remove user from muted list
	delete(mutedUsers, userID)

	// Update room state
	muteJSON, err := json.Marshal(mutedUsers)
	if err != nil {
		return fmt.Errorf("failed to marshal muted users: %w", err)
	}

	roomState.Data["muted_users"] = json.RawMessage(muteJSON)
	if err := s.roomState.UpdateRoomState(ctx, roomState); err != nil {
		return fmt.Errorf("failed to update room state: %w", err)
	}

	// Log moderation action
	s.logModerationAction(ctx, ModerationActionUnban, userID, moderatorID, roomID, reason, "Unmuted user")

	// Notify room of unmute
	event := map[string]any{
		"type":         "user_unmuted",
		"user_id":      userID,
		"moderator_id": moderatorID,
		"room_id":      roomID,
	}

	if err := s.pubsub.PublishToRoom(ctx, roomID, "moderation", event); err != nil {
		s.logger.Error("Failed to publish unmute event", err)
		// Continue anyway as the unmute was successfully applied
	}

	s.logger.Info("Unmuted user", "user", userID, "room", roomID)
	return nil
}

// IsUserMuted checks if a user is muted in a room.
func (s *ModerationService) IsUserMuted(ctx context.Context, userID, roomID string) (bool, time.Time, error) {
	// Validate inputs
	if userID == "" || roomID == "" {
		return false, time.Time{}, fmt.Errorf("user ID and room ID are required")
	}

	// Get room state
	roomState, err := s.roomState.GetRoomState(ctx, roomID)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to get room state: %w", err)
	}

	if roomState == nil {
		return false, time.Time{}, nil
	}

	// Check if user is muted
	if roomState.Data == nil || roomState.Data["muted_users"] == nil {
		return false, time.Time{}, nil
	}

	// Get muted users map
	var mutedUsers map[string]time.Time
	if muteData, exists := roomState.Data["muted_users"]; exists {
		// Try to convert existing data
		if muteJSON, ok := muteData.(json.RawMessage); ok {
			if err := json.Unmarshal(muteJSON, &mutedUsers); err != nil {
				return false, time.Time{}, fmt.Errorf("failed to unmarshal muted users: %w", err)
			}
		} else {
			return false, time.Time{}, nil
		}
	} else {
		return false, time.Time{}, nil
	}

	// Check if user is in muted list
	endTime, exists := mutedUsers[userID]
	if !exists {
		return false, time.Time{}, nil
	}

	// Check if mute has expired
	if endTime.Before(time.Now()) {
		// Remove expired mute
		delete(mutedUsers, userID)

		// Update room state
		muteJSON, err := json.Marshal(mutedUsers)
		if err != nil {
			return false, time.Time{}, fmt.Errorf("failed to marshal muted users: %w", err)
		}

		roomState.Data["muted_users"] = json.RawMessage(muteJSON)
		if err := s.roomState.UpdateRoomState(ctx, roomState); err != nil {
			s.logger.Error("Failed to update room state for expired mute", err)
		}

		return false, time.Time{}, nil
	}

	return true, endTime, nil
}

// KickUser kicks a user from a room.
func (s *ModerationService) KickUser(
	ctx context.Context,
	userID, roomID, moderatorID, reason string,
) error {
	// Validate inputs
	if userID == "" || roomID == "" || moderatorID == "" {
		return fmt.Errorf("user ID, room ID, and moderator ID are required")
	}

	// Check if user is in the room
	inRoom, err := s.roomState.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to check if user is in room: %w", err)
	}

	if !inRoom {
		return fmt.Errorf("user is not in the room: %s", userID)
	}

	// Remove user from room
	err = s.roomState.RemoveUserFromRoom(ctx, roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user from room: %w", err)
	}

	// Log moderation action
	s.logModerationAction(ctx, ModerationActionKick, userID, moderatorID, roomID, reason, "")

	// Notify room of kick
	event := map[string]any{
		"type":         "user_kicked",
		"user_id":      userID,
		"moderator_id": moderatorID,
		"room_id":      roomID,
		"reason":       reason,
	}

	if err := s.pubsub.PublishToRoom(ctx, roomID, "moderation", event); err != nil {
		s.logger.Error("Failed to publish kick event", err)
		// Continue anyway as the kick was successfully applied
	}

	s.logger.Info("Kicked user", "user", userID, "room", roomID)
	return nil
}

// DeleteMessage deletes a chat message.
func (s *ModerationService) DeleteMessage(
	ctx context.Context,
	messageID, roomID, moderatorID, reason string,
) error {
	// Validate inputs
	if messageID == "" || roomID == "" || moderatorID == "" {
		return fmt.Errorf("message ID, room ID, and moderator ID are required")
	}

	// Find message
	filter := bson.M{
		"_id":     messageID,
		"room_id": roomID,
	}

	// Update message to deleted
	update := bson.M{
		"$set": bson.M{
			"deleted":       true,
			"deleted_by":    moderatorID,
			"deleted_at":    time.Now(),
			"delete_reason": reason,
		},
	}

	result, err := s.db.Collection("chat_messages").UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("failed to update message: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("message not found: %s", messageID)
	}

	// Log moderation action
	details := fmt.Sprintf("Message ID: %s", messageID)
	s.logModerationAction(ctx, ModerationActionDeleteMessage, "", moderatorID, roomID, reason, details)

	// Notify room of message deletion
	event := map[string]any{
		"type":         "message_deleted",
		"message_id":   messageID,
		"moderator_id": moderatorID,
		"room_id":      roomID,
	}

	if err := s.pubsub.PublishToRoom(ctx, roomID, "moderation", event); err != nil {
		s.logger.Error("Failed to publish message deletion event", err)
		// Continue anyway as the message was successfully deleted
	}

	s.logger.Info("Deleted message", "message", messageID, "room", roomID)
	return nil
}

// logModerationAction logs a moderation action to the database.
func (s *ModerationService) logModerationAction(
	ctx context.Context,
	action ModerationAction,
	userID, moderatorID, roomID, reason, details string,
) {
	log := &ModerationLog{
		Action:      action,
		UserID:      userID,
		ModeratorID: moderatorID,
		RoomID:      roomID,
		Reason:      reason,
		Timestamp:   time.Now(),
		Details:     details,
	}

	_, err := s.db.Collection("moderation_logs").InsertOne(ctx, log)
	if err != nil {
		s.logger.Error("Failed to log moderation action", err)
	}
}

// GetModerationLogs retrieves moderation logs based on the provided filter.
func (s *ModerationService) GetModerationLogs(
	ctx context.Context,
	filter bson.M,
	page, pageSize int,
) ([]*ModerationLog, int64, error) {
	// Set up options
	opts := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	if page > 0 && pageSize > 0 {
		opts.SetSkip(int64((page - 1) * pageSize))
		opts.SetLimit(int64(pageSize))
	}

	// Count total
	total, err := s.db.Collection("moderation_logs").CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count logs: %w", err)
	}

	// Execute query
	cursor, err := s.db.Collection("moderation_logs").Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	logs := make([]*ModerationLog, 0)
	for cursor.Next(ctx) {
		var log ModerationLog
		if err := cursor.Decode(&log); err != nil {
			s.logger.Error("Failed to decode log", err)
			continue
		}
		logs = append(logs, &log)
	}

	return logs, total, nil
}
