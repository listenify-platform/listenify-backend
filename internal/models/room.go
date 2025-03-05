// Package models contains the data structures used throughout the application.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Room represents a virtual room where users can listen to music together.
type Room struct {
	// ID is the unique identifier for the room.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// Name is the display name of the room.
	Name string `json:"name" bson:"name" validate:"required,min=2,max=50"`

	// Slug is the URL-friendly identifier for the room.
	Slug string `json:"slug" bson:"slug"`

	// Description provides information about the room.
	Description string `json:"description" bson:"description" validate:"max=1000"`

	// CreatedBy is the ID of the user who created the room.
	CreatedBy bson.ObjectID `json:"createdBy" bson:"createdBy"`

	// Settings contains the room's configuration settings.
	Settings RoomSettings `json:"settings" bson:"settings"`

	// Stats contains the room's statistics.
	Stats RoomStats `json:"stats" bson:"stats"`

	// Moderators is a list of users who have moderation privileges.
	Moderators []bson.ObjectID `json:"moderators" bson:"moderators"`

	// BannedUsers is a list of users who are banned from joining the room.
	BannedUsers []bson.ObjectID `json:"bannedUsers" bson:"bannedUsers"`

	// Tags are keywords that describe the room.
	Tags []string `json:"tags" bson:"tags" validate:"dive,max=20"`

	// CurrentDJ is the ID of the user who is currently DJ'ing.
	CurrentDJ bson.ObjectID `json:"currentDJ,omitempty" bson:"currentDJ,omitempty"`

	// CurrentMedia is the ID of the media that is currently playing.
	CurrentMedia bson.ObjectID `json:"currentMedia,omitempty" bson:"currentMedia,omitempty"`

	// IsActive indicates whether the room is currently active.
	IsActive bool `json:"isActive" bson:"isActive"`

	// ObjectTimes contains timestamps for this room.
	ObjectTimes

	// LastActivity is the time of the last activity in the room.
	LastActivity time.Time `json:"lastActivity" bson:"lastActivity"`
}

// RoomSettings represents the configuration settings for a room.
type RoomSettings struct {
	// Private indicates whether the room is private.
	Private bool `json:"private" bson:"private"`

	// Capacity is the maximum number of users allowed in the room.
	Capacity int `json:"capacity" bson:"capacity" validate:"min=1,max=1000"`

	// WaitlistMax is the maximum number of users allowed in the DJ waitlist.
	WaitlistMax int `json:"waitlistMax" bson:"waitlistMax" validate:"min=1,max=100"`

	// Theme is the room's visual theme.
	Theme string `json:"theme" bson:"theme"`

	// Welcome is the welcome message shown to users when they join.
	Welcome string `json:"welcome" bson:"welcome" validate:"max=500"`

	// AllowedSources is a list of allowed media sources.
	AllowedSources []string `json:"allowedSources" bson:"allowedSources"`

	// MaxSongLength is the maximum length of a song in seconds.
	MaxSongLength int `json:"maxSongLength" bson:"maxSongLength" validate:"min=0,max=3600"`

	// ChatEnabled indicates whether chat is enabled.
	ChatEnabled bool `json:"chatEnabled" bson:"chatEnabled"`

	// ChatDelay is the delay in seconds between chat messages for a user.
	ChatDelay int `json:"chatDelay" bson:"chatDelay" validate:"min=0,max=60"`

	// AutoSkipDisconnect indicates whether to skip disconnected DJs.
	AutoSkipDisconnect bool `json:"autoSkipDisconnect" bson:"autoSkipDisconnect"`

	// AutoSkipAfterTime is the time after which to skip a disconnected DJ in seconds.
	AutoSkipAfterTime int `json:"autoSkipAfterTime" bson:"autoSkipAfterTime" validate:"min=0,max=600"`

	// GuestCanJoinQueue indicates whether guests can join the DJ queue.
	GuestCanJoinQueue bool `json:"guestCanJoinQueue" bson:"guestCanJoinQueue"`

	// PasswordProtected indicates whether a password is required to join.
	PasswordProtected bool `json:"passwordProtected" bson:"passwordProtected"`

	// Password is the password required to join the room.
	Password string `json:"-" bson:"password,omitempty"`
}

// RoomStats represents the statistics for a room.
type RoomStats struct {
	// TotalPlays is the total number of tracks played in the room.
	TotalPlays int `json:"totalPlays" bson:"totalPlays"`

	// TotalUsers is the total number of unique users who have joined the room.
	TotalUsers int `json:"totalUsers" bson:"totalUsers"`

	// PeakUsers is the highest number of concurrent users in the room.
	PeakUsers int `json:"peakUsers" bson:"peakUsers"`

	// TotalChatMessages is the total number of chat messages sent in the room.
	TotalChatMessages int `json:"totalChatMessages" bson:"totalChatMessages"`

	// CreatedDuration is the total time the room has been created.
	CreatedDuration time.Duration `json:"createdDuration" bson:"createdDuration"`

	// ActiveDuration is the total time the room has been active.
	ActiveDuration time.Duration `json:"activeDuration" bson:"activeDuration"`

	// TotalWoots is the total number of woots (likes) in the room.
	TotalWoots int `json:"totalWoots" bson:"totalWoots"`

	// TotalMehs is the total number of mehs (dislikes) in the room.
	TotalMehs int `json:"totalMehs" bson:"totalMehs"`

	// AggregateRating is the overall rating of the room.
	AggregateRating float64 `json:"aggregateRating" bson:"aggregateRating"`

	// LastStatsReset is the time when the stats were last reset.
	LastStatsReset time.Time `json:"lastStatsReset" bson:"lastStatsReset"`
}

// RoomUser represents a user's status within a room.
type RoomUser struct {
	// ID is a unique identifier for this room-user relationship.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// UserID is the ID of the user.
	UserID bson.ObjectID `json:"userId" bson:"userId"`

	// Role is the user's role in the room.
	Role string `json:"role" bson:"role"`

	// Position is the user's position in the DJ queue.
	Position int `json:"position" bson:"position"`

	// JoinedAt is the time the user joined the room.
	JoinedAt time.Time `json:"joinedAt" bson:"joinedAt"`

	// LastActive is the time of the user's last activity in the room.
	LastActive time.Time `json:"lastActive" bson:"lastActive"`

	// WootsGiven is the number of woots (likes) the user has given.
	WootsGiven int `json:"wootsGiven" bson:"wootsGiven"`

	// MehsGiven is the number of mehs (dislikes) the user has given.
	MehsGiven int `json:"mehsGiven" bson:"mehsGiven"`

	// ChatMessages is the number of chat messages the user has sent.
	ChatMessages int `json:"chatMessages" bson:"chatMessages"`

	// IsDJ indicates whether the user is currently a DJ.
	IsDJ bool `json:"isDJ" bson:"isDJ"`
}

// RoomState represents the real-time state of a room.
type RoomState struct {
	// ID is the unique identifier for the room.
	ID bson.ObjectID `json:"id"`

	// Name is the display name of the room.
	Name string `json:"name"`

	// Settings contains the room's configuration settings.
	Settings RoomSettings `json:"settings"`

	// CurrentDJ contains information about the current DJ.
	CurrentDJ *PublicUser `json:"currentDJ,omitempty"`

	// CurrentMedia contains information about the currently playing media.
	CurrentMedia *MediaInfo `json:"currentMedia,omitempty"`

	// DJQueue is the list of users in the DJ queue.
	DJQueue []QueueEntry `json:"djQueue"`

	// ActiveUsers is the number of users currently in the room.
	ActiveUsers int `json:"activeUsers"`

	// Users is the list of users currently in the room.
	Users []PublicUser `json:"users"`

	// MediaStartTime is the time when the current media started playing.
	MediaStartTime time.Time `json:"mediaStartTime"`

	// MediaProgress is the current position in the media in seconds.
	MediaProgress int `json:"mediaProgress"`

	// MediaEndTime is the expected time when the current media will end.
	MediaEndTime time.Time `json:"mediaEndTime"`

	// PlayHistory is a list of recently played media.
	PlayHistory []PlayHistoryEntry `json:"playHistory"`
}

// QueueEntry represents a user in the DJ queue.
type QueueEntry struct {
	// User is the user in the queue.
	User PublicUser `json:"user"`

	// Position is the user's position in the queue.
	Position int `json:"position"`

	// JoinTime is the time the user joined the queue.
	JoinTime time.Time `json:"joinTime"`

	// PlayCount is the number of tracks the user has played since joining the queue.
	PlayCount int `json:"playCount"`

	// JoinedAt is the time the user joined the room.
	JoinedAt time.Time `json:"joinedAt"`
}

// PlayHistoryEntry represents a previously played track.
type PlayHistoryEntry struct {
	// Media is the media that was played.
	Media MediaInfo `json:"media"`

	// DJ is the user who played the media.
	DJ PublicUser `json:"dj"`

	// PlayTime is the time the media was played.
	PlayTime time.Time `json:"playTime"`

	// Woots is the number of woots (likes) the media received.
	Woots int `json:"woots"`

	// Mehs is the number of mehs (dislikes) the media received.
	Mehs int `json:"mehs"`

	// Grabs is the number of users who added the media to their playlists.
	Grabs int `json:"grabs"`
}

// RoomCreateRequest represents the data needed to create a new room.
type RoomCreateRequest struct {
	// Name is the display name of the room.
	Name string `json:"name" validate:"required,min=2,max=50"`

	// Description provides information about the room.
	Description string `json:"description" validate:"max=1000"`

	// Settings contains the room's configuration settings.
	Settings RoomSettings `json:"settings"`

	// Tags are keywords that describe the room.
	Tags []string `json:"tags" validate:"dive,max=20"`
}

// RoomUpdateRequest represents the data needed to update a room.
type RoomUpdateRequest struct {
	// Name is the display name of the room.
	Name string `json:"name" validate:"omitempty,min=2,max=50"`

	// Description provides information about the room.
	Description string `json:"description" validate:"max=1000"`

	// Settings contains the room's configuration settings.
	Settings *RoomSettings `json:"settings,omitempty"`

	// Tags are keywords that describe the room.
	Tags []string `json:"tags" validate:"dive,max=20"`
}

// RoomJoinRequest represents the data needed to join a room.
type RoomJoinRequest struct {
	// Password is the password required to join the room.
	Password string `json:"password,omitempty"`
}

// RoomSearchCriteria represents the criteria for searching rooms.
type RoomSearchCriteria struct {
	// Query is the search query.
	Query string `json:"query"`

	// Tags are the tags to filter by.
	Tags []string `json:"tags"`

	// OnlyActive indicates whether to show only active rooms.
	OnlyActive bool `json:"onlyActive"`

	// IncludePrivate indicates whether to include private rooms.
	IncludePrivate bool `json:"includePrivate"`

	// MinUsers is the minimum number of users.
	MinUsers int `json:"minUsers"`

	// MaxUsers is the maximum number of users.
	MaxUsers int `json:"maxUsers"`

	// SortBy is the field to sort by.
	SortBy string `json:"sortBy"`

	// SortDirection is the direction to sort (asc or desc).
	SortDirection string `json:"sortDirection"`

	// Page is the page number for pagination.
	Page int `json:"page"`

	// Limit is the number of results per page.
	Limit int `json:"limit"`
}
