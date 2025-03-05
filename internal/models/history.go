// Package models contains the data structures used throughout the application.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// History represents the top-level history record, which can be used to access
// other types of historical data.
type History struct {
	// ID is the unique identifier for the history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// Type is the type of history record.
	Type string `json:"type" bson:"type" validate:"required,oneof=media room user dj chat moderation"`

	// ReferenceID is the ID of the referenced entity.
	ReferenceID bson.ObjectID `json:"referenceId" bson:"referenceId"`

	// Timestamp is when the history record was created.
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`

	// Metadata contains additional information about the record.
	Metadata map[string]any `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// PlayHistory represents a record of a media item being played.
type PlayHistory struct {
	// ID is the unique identifier for the play history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room where the media was played.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// MediaID is the ID of the media that was played.
	MediaID bson.ObjectID `json:"mediaId" bson:"mediaId"`

	// DjID is the ID of the user who played the media.
	DjID bson.ObjectID `json:"djId" bson:"djId"`

	// Media contains details about the media that was played.
	Media MediaInfo `json:"media" bson:"media"`

	// DJ contains details about the user who played the media.
	DJ PublicUser `json:"dj" bson:"dj"`

	// StartTime is when the media started playing.
	StartTime time.Time `json:"startTime" bson:"startTime"`

	// EndTime is when the media finished playing.
	EndTime time.Time `json:"endTime" bson:"endTime"`

	// Duration is the actual duration the media played in seconds.
	Duration int `json:"duration" bson:"duration"`

	// Skipped indicates whether the media was skipped.
	Skipped bool `json:"skipped" bson:"skipped"`

	// SkipReason is the reason for skipping, if applicable.
	SkipReason string `json:"skipReason,omitempty" bson:"skipReason,omitempty"`

	// SkippedBy is the ID of the user who skipped the media, if applicable.
	SkippedBy bson.ObjectID `json:"skippedBy,omitempty" bson:"skippedBy,omitempty"`

	// Votes contains the voting information.
	Votes MediaVotes `json:"votes" bson:"votes"`

	// UserCount is the number of users in the room when the media was played.
	UserCount int `json:"userCount" bson:"userCount"`
}

// UserHistory represents a record of a user's activities.
type UserHistory struct {
	// ID is the unique identifier for the user history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// UserID is the ID of the user.
	UserID bson.ObjectID `json:"userId" bson:"userId"`

	// Type is the type of activity.
	Type string `json:"type" bson:"type" validate:"required,oneof=login logout join leave dj queue vote chat moderation profile playlist"`

	// RoomID is the ID of the room, if applicable.
	RoomID bson.ObjectID `json:"roomId,omitempty" bson:"roomId,omitempty"`

	// Timestamp is when the activity occurred.
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`

	// Details contains additional information about the activity.
	Details map[string]any `json:"details,omitempty" bson:"details,omitempty"`

	// IP is the user's IP address.
	IP string `json:"-" bson:"ip"`

	// UserAgent is the user's browser/client information.
	UserAgent string `json:"-" bson:"userAgent"`
}

// RoomHistory represents a record of a room's activities.
type RoomHistory struct {
	// ID is the unique identifier for the room history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// Type is the type of activity.
	Type string `json:"type" bson:"type" validate:"required,oneof=create update activate deactivate join leave dj queue chat moderation"`

	// UserID is the ID of the user who performed the activity, if applicable.
	UserID bson.ObjectID `json:"userId,omitempty" bson:"userId,omitempty"`

	// Timestamp is when the activity occurred.
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`

	// Details contains additional information about the activity.
	Details map[string]any `json:"details,omitempty" bson:"details,omitempty"`

	// UserCount is the number of users in the room at the time.
	UserCount int `json:"userCount" bson:"userCount"`
}

// SessionHistory represents a user's session.
type SessionHistory struct {
	// ID is the unique identifier for the session history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// UserID is the ID of the user.
	UserID bson.ObjectID `json:"userId" bson:"userId"`

	// StartTime is when the session started.
	StartTime time.Time `json:"startTime" bson:"startTime"`

	// EndTime is when the session ended.
	EndTime time.Time `json:"endTime,omitzero" bson:"endTime,omitempty"`

	// Duration is the duration of the session in seconds.
	Duration int `json:"duration" bson:"duration"`

	// IP is the user's IP address.
	IP string `json:"-" bson:"ip"`

	// UserAgent is the user's browser/client information.
	UserAgent string `json:"-" bson:"userAgent"`

	// Device is the user's device information.
	Device string `json:"device" bson:"device"`

	// Platform is the user's operating system.
	Platform string `json:"platform" bson:"platform"`

	// Country is the user's country based on IP.
	Country string `json:"country" bson:"country"`

	// Activities is a count of different activities during the session.
	Activities map[string]int `json:"activities" bson:"activities"`
}

// DJHistory represents a record of a user's DJ activity.
type DJHistory struct {
	// ID is the unique identifier for the DJ history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// UserID is the ID of the user.
	UserID bson.ObjectID `json:"userId" bson:"userId"`

	// RoomID is the ID of the room.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// StartTime is when the user started DJing.
	StartTime time.Time `json:"startTime" bson:"startTime"`

	// EndTime is when the user stopped DJing.
	EndTime time.Time `json:"endTime,omitzero" bson:"endTime,omitempty"`

	// Duration is the duration of the DJ session in seconds.
	Duration int `json:"duration" bson:"duration"`

	// TracksPlayed is the number of tracks played.
	TracksPlayed int `json:"tracksPlayed" bson:"tracksPlayed"`

	// Tracks is a list of tracks played.
	Tracks []PlayHistorySummary `json:"tracks" bson:"tracks"`

	// TotalWoots is the total number of woots received.
	TotalWoots int `json:"totalWoots" bson:"totalWoots"`

	// TotalMehs is the total number of mehs received.
	TotalMehs int `json:"totalMehs" bson:"totalMehs"`

	// TotalGrabs is the total number of grabs received.
	TotalGrabs int `json:"totalGrabs" bson:"totalGrabs"`

	// AverageAudience is the average number of users during the DJ session.
	AverageAudience float64 `json:"averageAudience" bson:"averageAudience"`

	// WasSkipped indicates whether the user was skipped at any point.
	WasSkipped bool `json:"wasSkipped" bson:"wasSkipped"`

	// LeaveReason is the reason the user stopped DJing.
	LeaveReason string `json:"leaveReason,omitempty" bson:"leaveReason,omitempty"`
}

// PlayHistorySummary represents a condensed version of play history for inclusion in other records.
type PlayHistorySummary struct {
	// MediaID is the ID of the media.
	MediaID bson.ObjectID `json:"mediaId" bson:"mediaId"`

	// Title is the title of the media.
	Title string `json:"title" bson:"title"`

	// Artist is the artist of the media.
	Artist string `json:"artist" bson:"artist"`

	// Duration is the duration of the media in seconds.
	Duration int `json:"duration" bson:"duration"`

	// StartTime is when the media started playing.
	StartTime time.Time `json:"startTime" bson:"startTime"`

	// Woots is the number of woots received.
	Woots int `json:"woots" bson:"woots"`

	// Mehs is the number of mehs received.
	Mehs int `json:"mehs" bson:"mehs"`

	// Grabs is the number of grabs received.
	Grabs int `json:"grabs" bson:"grabs"`

	// Skipped indicates whether the media was skipped.
	Skipped bool `json:"skipped" bson:"skipped"`
}

// ChatHistory represents a record of chat activity in a room.
type ChatHistory struct {
	// ID is the unique identifier for the chat history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// StartTime is the start of the chat history period.
	StartTime time.Time `json:"startTime" bson:"startTime"`

	// EndTime is the end of the chat history period.
	EndTime time.Time `json:"endTime" bson:"endTime"`

	// MessageCount is the total number of messages in the period.
	MessageCount int `json:"messageCount" bson:"messageCount"`

	// UserCount is the number of unique users who chatted.
	UserCount int `json:"userCount" bson:"userCount"`

	// UserMessageCounts maps user IDs to message counts.
	UserMessageCounts map[string]int `json:"userMessageCounts" bson:"userMessageCounts"`

	// MessageTypes maps message types to counts.
	MessageTypes map[string]int `json:"messageTypes" bson:"messageTypes"`

	// PopularCommands maps chat commands to usage counts.
	PopularCommands map[string]int `json:"popularCommands" bson:"popularCommands"`

	// ModActions is the number of moderation actions taken.
	ModActions int `json:"modActions" bson:"modActions"`
}

// ModerationHistory represents a record of moderation actions in a room.
type ModerationHistory struct {
	// ID is the unique identifier for the moderation history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// ModeratorID is the ID of the moderator.
	ModeratorID bson.ObjectID `json:"moderatorId" bson:"moderatorId"`

	// TargetUserID is the ID of the user who was moderated.
	TargetUserID bson.ObjectID `json:"targetUserId" bson:"targetUserId"`

	// Action is the type of moderation action.
	Action string `json:"action" bson:"action" validate:"required,oneof=warn mute unmute kick ban unban delete"`

	// Reason is the reason for the moderation action.
	Reason string `json:"reason,omitempty" bson:"reason,omitempty"`

	// Timestamp is when the action occurred.
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`

	// Duration is the duration of the action (for mutes and bans) in minutes.
	Duration int `json:"duration,omitempty" bson:"duration,omitempty"`

	// ExpiresAt is when the action expires (for mutes and bans).
	ExpiresAt time.Time `json:"expiresAt,omitzero" bson:"expiresAt,omitempty"`

	// MessageID is the ID of the message that was moderated (for delete actions).
	MessageID bson.ObjectID `json:"messageId,omitempty" bson:"messageId,omitempty"`

	// MessageContent is the content of the deleted message (if applicable).
	MessageContent string `json:"-" bson:"messageContent,omitempty"`
}

// SystemHistory represents a record of system events.
type SystemHistory struct {
	// ID is the unique identifier for the system history record.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// Type is the type of system event.
	Type string `json:"type" bson:"type" validate:"required,oneof=startup shutdown error warning config deployment migration"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`

	// Component is the system component that generated the event.
	Component string `json:"component" bson:"component"`

	// Message is a description of the event.
	Message string `json:"message" bson:"message"`

	// Level is the severity level of the event.
	Level string `json:"level" bson:"level" validate:"required,oneof=info warn error fatal"`

	// Details contains additional information about the event.
	Details map[string]any `json:"details,omitempty" bson:"details,omitempty"`
}

// PlayHistoryRequest represents the data needed to retrieve play history.
type PlayHistoryRequest struct {
	// RoomID is the ID of the room to get history for.
	RoomID bson.ObjectID `json:"roomId,omitempty"`

	// UserID is the ID of the user to get history for.
	UserID bson.ObjectID `json:"userId,omitempty"`

	// MediaID is the ID of the media to get history for.
	MediaID bson.ObjectID `json:"mediaId,omitempty"`

	// StartTime is the start of the time range.
	StartTime time.Time `json:"startTime,omitzero"`

	// EndTime is the end of the time range.
	EndTime time.Time `json:"endTime,omitzero"`

	// Limit is the maximum number of records to return.
	Limit int `json:"limit" validate:"min=1,max=100"`

	// Skip is the number of records to skip.
	Skip int `json:"skip" validate:"min=0"`
}

// PlayHistoryResponse represents the response to a play history request.
type PlayHistoryResponse struct {
	// History is the list of play history records.
	History []PlayHistory `json:"history"`

	// TotalItems is the total number of records matching the criteria.
	TotalItems int `json:"totalItems"`

	// HasMore indicates whether there are more records to retrieve.
	HasMore bool `json:"hasMore"`
}

// HistorySummary represents a summary of history statistics.
type HistorySummary struct {
	// TotalPlays is the total number of tracks played.
	TotalPlays int `json:"totalPlays"`

	// TotalUniqueTracks is the number of unique tracks played.
	TotalUniqueTracks int `json:"totalUniqueTracks"`

	// TotalDJs is the number of unique DJs.
	TotalDJs int `json:"totalDJs"`

	// TotalPlayTime is the total play time in seconds.
	TotalPlayTime int64 `json:"totalPlayTime"`

	// AverageVotes contains average voting statistics.
	AverageVotes struct {
		// Woots is the average number of woots per play.
		Woots float64 `json:"woots"`

		// Mehs is the average number of mehs per play.
		Mehs float64 `json:"mehs"`

		// Grabs is the average number of grabs per play.
		Grabs float64 `json:"grabs"`
	} `json:"averageVotes"`

	// TopTracks is a list of the most played tracks.
	TopTracks []TopTrackSummary `json:"topTracks"`

	// TopDJs is a list of the most active DJs.
	TopDJs []TopDJSummary `json:"topDJs"`

	// LastUpdated is when the summary was last calculated.
	LastUpdated time.Time `json:"lastUpdated"`
}

// TopTrackSummary represents a summary of a popular track.
type TopTrackSummary struct {
	// MediaID is the ID of the media.
	MediaID bson.ObjectID `json:"mediaId"`

	// Type is the type of media (youtube, soundcloud, etc).
	Type string `json:"type"`

	// SourceID is the ID on the source platform.
	SourceID string `json:"sourceId"`

	// Title is the title of the track.
	Title string `json:"title"`

	// Artist is the artist of the track.
	Artist string `json:"artist"`

	// PlayCount is the number of times the track was played.
	PlayCount int `json:"playCount"`

	// WootCount is the total number of woots received.
	WootCount int `json:"wootCount"`

	// MehCount is the total number of mehs received.
	MehCount int `json:"mehCount"`

	// GrabCount is the total number of grabs received.
	GrabCount int `json:"grabCount"`

	// SkipCount is the number of times the track was skipped.
	SkipCount int `json:"skipCount"`

	// AverageAudience is the average number of users during plays.
	AverageAudience float64 `json:"averageAudience"`

	// LastPlayed is when the track was last played.
	LastPlayed time.Time `json:"lastPlayed"`
}

// TopDJSummary represents a summary of an active DJ.
type TopDJSummary struct {
	// UserID is the ID of the user.
	UserID bson.ObjectID `json:"userId"`

	// Username is the username of the DJ.
	Username string `json:"username"`

	// PlayCount is the number of tracks played.
	PlayCount int `json:"playCount"`

	// TotalDJTime is the total time spent as DJ in seconds.
	TotalDJTime int64 `json:"totalDJTime"`

	// WootCount is the total number of woots received.
	WootCount int `json:"wootCount"`

	// MehCount is the total number of mehs received.
	MehCount int `json:"mehCount"`

	// GrabCount is the total number of grabs received.
	GrabCount int `json:"grabCount"`

	// SkipCount is the number of times the DJ was skipped.
	SkipCount int `json:"skipCount"`

	// LastDJTime is when the user last DJ'd.
	LastDJTime time.Time `json:"lastDJTime"`
}
