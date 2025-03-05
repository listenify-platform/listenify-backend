// Package models contains the data structures used throughout the application.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ChatMessage represents a chat message sent in a room.
type ChatMessage struct {
	// ID is the unique identifier for the message.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room where the message was sent.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// UserID is the ID of the user who sent the message.
	UserID bson.ObjectID `json:"userId" bson:"userId"`

	// Type is the type of message.
	Type string `json:"type" bson:"type" validate:"required,oneof=text emote system command"`

	// Content is the text content of the message.
	Content string `json:"content" bson:"content" validate:"required,max=500"`

	// Mentions is a list of users mentioned in the message.
	Mentions []bson.ObjectID `json:"mentions" bson:"mentions"`

	// ReplyTo is the ID of the message being replied to (if any).
	ReplyTo bson.ObjectID `json:"replyTo,omitempty" bson:"replyTo,omitempty"`

	// IsDeleted indicates whether the message has been deleted.
	IsDeleted bool `json:"isDeleted" bson:"isDeleted"`

	// DeletedBy is the ID of the user who deleted the message.
	DeletedBy bson.ObjectID `json:"deletedBy,omitempty" bson:"deletedBy,omitempty"`

	// DeletedAt is the time the message was deleted.
	DeletedAt time.Time `json:"deletedAt,omitzero" bson:"deletedAt,omitempty"`

	// IsEdited indicates whether the message has been edited.
	IsEdited bool `json:"isEdited" bson:"isEdited"`

	// EditedAt is the time the message was last edited.
	EditedAt time.Time `json:"editedAt,omitzero" bson:"editedAt,omitempty"`

	// CreatedAt is the time the message was sent.
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`

	// UserRole is the role of the user at the time of sending.
	UserRole string `json:"userRole" bson:"userRole"`

	// Metadata contains additional information about the message.
	Metadata map[string]any `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// ChatMessageRequest represents the data needed to send a chat message.
type ChatMessageRequest struct {
	// Type is the type of message.
	Type string `json:"type" validate:"required,oneof=text emote command"`

	// Content is the text content of the message.
	Content string `json:"content" validate:"required,max=500"`

	// ReplyTo is the ID of the message being replied to (if any).
	ReplyTo string `json:"replyTo,omitempty"`
}

// ChatMessageResponse represents a chat message with additional information.
type ChatMessageResponse struct {
	// ID is the unique identifier for the message.
	ID string `json:"id"`

	// RoomID is the ID of the room where the message was sent.
	RoomID string `json:"roomId"`

	// User is information about the user who sent the message.
	User PublicUser `json:"user"`

	// Type is the type of message.
	Type string `json:"type"`

	// Content is the text content of the message.
	Content string `json:"content"`

	// Mentions is a list of users mentioned in the message.
	Mentions []PublicUser `json:"mentions,omitempty"`

	// ReplyTo is information about the message being replied to (if any).
	ReplyTo *ChatMessageResponse `json:"replyTo,omitempty"`

	// IsDeleted indicates whether the message has been deleted.
	IsDeleted bool `json:"isDeleted"`

	// IsEdited indicates whether the message has been edited.
	IsEdited bool `json:"isEdited"`

	// Timestamp is the time the message was sent.
	Timestamp time.Time `json:"timestamp"`

	// UserRole is the role of the user at the time of sending.
	UserRole string `json:"userRole"`

	// Metadata contains additional information about the message.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ChatCommand represents a command that can be executed in chat.
type ChatCommand struct {
	// Name is the name of the command.
	Name string `json:"name" bson:"name"`

	// Description is a description of what the command does.
	Description string `json:"description" bson:"description"`

	// Usage is an example of how to use the command.
	Usage string `json:"usage" bson:"usage"`

	// MinimumRole is the minimum role required to use the command.
	MinimumRole string `json:"minimumRole" bson:"minimumRole"`

	// Enabled indicates whether the command is enabled.
	Enabled bool `json:"enabled" bson:"enabled"`

	// CooldownSeconds is the cooldown between uses of the command.
	CooldownSeconds int `json:"cooldownSeconds" bson:"cooldownSeconds"`
}

// ChatEmote represents an emote that can be used in chat.
type ChatEmote struct {
	// ID is the unique identifier for the emote.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// Code is the text code for the emote.
	Code string `json:"code" bson:"code" validate:"required,min=2,max=20"`

	// ImageURL is the URL of the emote image.
	ImageURL string `json:"imageUrl" bson:"imageUrl" validate:"required,url"`

	// Creator is the ID of the user who created the emote.
	Creator bson.ObjectID `json:"creator" bson:"creator"`

	// IsGlobal indicates whether the emote is available globally.
	IsGlobal bool `json:"isGlobal" bson:"isGlobal"`

	// RoomID is the ID of the room the emote belongs to (if not global).
	RoomID bson.ObjectID `json:"roomId,omitempty" bson:"roomId,omitempty"`

	// CreatedAt is the time the emote was created.
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`

	// UpdatedAt is the time the emote was last updated.
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

// ChatModeration represents a moderation action taken on a chat message.
type ChatModeration struct {
	// ID is the unique identifier for the moderation action.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room where the action was taken.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// ModeratorID is the ID of the user who took the action.
	ModeratorID bson.ObjectID `json:"moderatorId" bson:"moderatorId"`

	// TargetUserID is the ID of the user who was moderated.
	TargetUserID bson.ObjectID `json:"targetUserId" bson:"targetUserId"`

	// MessageID is the ID of the message that was moderated (if applicable).
	MessageID bson.ObjectID `json:"messageId,omitempty" bson:"messageId,omitempty"`

	// Action is the type of moderation action.
	Action string `json:"action" bson:"action" validate:"required,oneof=warn mute unmute kick ban unban delete"`

	// Reason is the reason for the moderation action.
	Reason string `json:"reason" bson:"reason" validate:"max=500"`

	// Duration is the duration of the action (for mutes and bans).
	Duration time.Duration `json:"duration,omitempty" bson:"duration,omitempty"`

	// ExpiresAt is when the action expires (for mutes and bans).
	ExpiresAt time.Time `json:"expiresAt,omitzero" bson:"expiresAt,omitempty"`

	// CreatedAt is when the action was taken.
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`
}

// ChatModerationRequest represents the data needed to perform a moderation action.
type ChatModerationRequest struct {
	// Action is the type of moderation action.
	Action string `json:"action" validate:"required,oneof=warn mute unmute kick ban unban delete"`

	// TargetUserID is the ID of the user to moderate.
	TargetUserID bson.ObjectID `json:"targetUserId" validate:"required"`

	// MessageID is the ID of the message to moderate (for delete actions).
	MessageID bson.ObjectID `json:"messageId,omitempty"`

	// Reason is the reason for the moderation action.
	Reason string `json:"reason" validate:"max=500"`

	// Duration is the duration for mutes and bans in minutes.
	Duration int `json:"duration,omitempty" validate:"min=0,max=43200"`
}

// ChatModerationResponse represents the response to a moderation action.
type ChatModerationResponse struct {
	// Success indicates whether the action was successful.
	Success bool `json:"success"`

	// Action is the type of moderation action that was taken.
	Action string `json:"action"`

	// TargetUser is information about the user who was moderated.
	TargetUser *PublicUser `json:"targetUser,omitempty"`

	// Moderator is information about the user who performed the moderation.
	Moderator *PublicUser `json:"moderator,omitempty"`

	// Reason is the reason for the moderation action.
	Reason string `json:"reason,omitempty"`

	// ExpiresAt is when the action expires (for mutes and bans).
	ExpiresAt time.Time `json:"expiresAt,omitzero"`

	// Message is a message about the moderation action.
	Message string `json:"message,omitempty"`
}
