// Package models contains the data structures used throughout the application.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// BaseUser contains the common fields for all user types.
type BaseUser struct {
	// ID is the unique identifier for the user.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// Username is the user's chosen username.
	Username string `json:"username" bson:"username" validate:"required,min=3,max=30,username"`

	// AvatarConfig stores the user's avatar customization options.
	AvatarConfig AvatarConfig `json:"avatarConfig" bson:"avatarConfig"`

	// Profile contains the user's profile information.
	Profile UserProfile `json:"profile" bson:"profile"`

	// Stats contains the user's statistics.
	Stats UserStats `json:"stats" bson:"stats"`

	// Badges is a list of badges the user has earned.
	Badges []string `json:"badges" bson:"badges"`

	// Roles contains the user's roles.
	Roles []string `json:"roles" bson:"roles"`
}

// User represents a user in the application.
type User struct {
	// UserBase embeds the base user information.
	BaseUser `bson:"inline"`

	// Email is the user's email address.
	Email string `json:"email" bson:"email" validate:"required,email"`

	// Password is the user's hashed password.
	Password string `json:"-" bson:"password"`

	// Connections contains the user's social connections.
	Connections UserConnections `json:"connections" bson:"connections"`

	// Settings contains the user's personal settings.
	Settings UserSettings `json:"settings" bson:"settings"`

	// IsActive indicates whether the user's account is active.
	IsActive bool `json:"isActive" bson:"isActive"`

	// IsVerified indicates whether the user's account has been verified.
	IsVerified bool `json:"isVerified" bson:"isVerified"`

	// LastLogin is the time of the user's last login.
	LastLogin time.Time `json:"lastLogin" bson:"lastLogin"`

	// ObjectTimes contains timestamps for this user.
	ObjectTimes
}

// AvatarConfig represents the customization options for a user's avatar.
type AvatarConfig struct {
	// Type is the type of avatar (e.g., "default", "custom").
	Type string `json:"type" bson:"type"`

	// Collection is the collection of avatar images.
	Collection string `json:"collection" bson:"collection"`

	// Number is the position of the avatar in the collection.
	Number int `json:"number" bson:"number"`

	// CustomImage is an optional custom image URL.
	CustomImage string `json:"customImage,omitempty" bson:"customImage,omitempty" validate:"omitempty,url"`
}

// UserProfile represents a user's profile information.
type UserProfile struct {
	// Bio is the user's biography.
	Bio string `json:"bio" bson:"bio" validate:"max=500"`

	// Location is the user's location.
	Location string `json:"location" bson:"location" validate:"max=100"`

	// Website is the user's website URL.
	Website string `json:"website" bson:"website" validate:"omitempty,url"`

	// Social contains the user's social media links.
	Social UserSocial `json:"social" bson:"social"`

	// Language is the user's preferred language.
	Language string `json:"language" bson:"language" validate:"max=10"`

	// JoinDate is when the user joined.
	JoinDate time.Time `json:"joinDate" bson:"joinDate"`

	// Status is the user's current status.
	Status string `json:"status" bson:"status" validate:"max=100"`
}

// UserSocial represents a user's social media links.
type UserSocial struct {
	// Twitter is the user's Twitter handle.
	Twitter string `json:"twitter" bson:"twitter" validate:"omitempty,max=30"`

	// Instagram is the user's Instagram handle.
	Instagram string `json:"instagram" bson:"instagram" validate:"omitempty,max=30"`

	// SoundCloud is the user's SoundCloud handle.
	SoundCloud string `json:"soundcloud" bson:"soundcloud" validate:"omitempty,max=30"`

	// YouTube is the user's YouTube channel.
	YouTube string `json:"youtube" bson:"youtube" validate:"omitempty,max=100"`

	// Spotify is the user's Spotify profile.
	Spotify string `json:"spotify" bson:"spotify" validate:"omitempty,max=100"`
}

// UserStats represents a user's statistics.
type UserStats struct {
	// Experience is the user's experience points.
	Experience int `json:"experience" bson:"experience"`

	// Level is the user's level.
	Level int `json:"level" bson:"level"`

	// Points is the user's points.
	Points int `json:"points" bson:"points"`

	// PlayCount is the number of tracks the user has played.
	PlayCount int `json:"playCount" bson:"playCount"`

	// Woots is the number of woots (likes) the user has received.
	Woots int `json:"woots" bson:"woots"`

	// Mehs is the number of mehs (dislikes) the user has received.
	Mehs int `json:"mehs" bson:"mehs"`

	// AudienceTime is the time spent as an audience member in rooms.
	AudienceTime int64 `json:"audienceTime" bson:"audienceTime"`

	// DJTime is the time spent as a DJ in rooms.
	DJTime int64 `json:"djTime" bson:"djTime"`

	// RoomsCreated is the number of rooms the user has created.
	RoomsCreated int `json:"roomsCreated" bson:"roomsCreated"`

	// RoomsJoined is the number of rooms the user has joined.
	RoomsJoined int `json:"roomsJoined" bson:"roomsJoined"`

	// ChatMessages is the number of chat messages the user has sent.
	ChatMessages int `json:"chatMessages" bson:"chatMessages"`

	// LastUpdated is when the user's stats were last updated.
	LastUpdated time.Time `json:"lastUpdated" bson:"lastUpdated"`
}

// UserConnections represents a user's social connections within the app.
type UserConnections struct {
	// Following is a list of users that this user follows.
	Following []bson.ObjectID `json:"following" bson:"following"`

	// Followers is a list of users following this user.
	Followers []bson.ObjectID `json:"followers" bson:"followers"`

	// Friends is a list of users that have mutually followed each other.
	Friends []bson.ObjectID `json:"friends" bson:"friends"`

	// Blocked is a list of users that this user has blocked.
	Blocked []bson.ObjectID `json:"blocked" bson:"blocked"`

	// Favorites is a list of rooms that this user has favorited.
	Favorites []bson.ObjectID `json:"favorites" bson:"favorites"`
}

// UserSettings represents a user's personal settings.
type UserSettings struct {
	// Theme is the user's preferred theme.
	Theme string `json:"theme" bson:"theme"`

	// AutoJoinDJ indicates whether the user should automatically join the DJ queue.
	AutoJoinDJ bool `json:"autoJoinDJ" bson:"autoJoinDJ"`

	// AutoWoot indicates whether the user should automatically woot.
	AutoWoot bool `json:"autoWoot" bson:"autoWoot"`

	// ShowChatImages indicates whether to show images in chat.
	ShowChatImages bool `json:"showChatImages" bson:"showChatImages"`

	// EnableNotifications indicates whether to enable notifications.
	EnableNotifications bool `json:"enableNotifications" bson:"enableNotifications"`

	// NotificationTypes indicates which types of notifications to receive.
	NotificationTypes map[string]bool `json:"notificationTypes" bson:"notificationTypes"`

	// ChatMentions indicates whether to notify on chat mentions.
	ChatMentions bool `json:"chatMentions" bson:"chatMentions"`

	// Volume is the user's preferred volume level.
	Volume int `json:"volume" bson:"volume" validate:"min=0,max=100"`

	// HideAudience indicates whether to hide the audience.
	HideAudience bool `json:"hideAudience" bson:"hideAudience"`

	// VideoSize is the user's preferred video size.
	VideoSize string `json:"videoSize" bson:"videoSize"`

	// LanguageFilter indicates whether to enable language filtering.
	LanguageFilter bool `json:"languageFilter" bson:"languageFilter"`
}

// PublicUser represents a subset of user information that is safe to share publicly.
type PublicUser struct {
	// BaseUser embeds the base user information.
	BaseUser

	// Online indicates whether the user is currently online.
	Online bool `json:"online"`
}

// ToPublicUser converts a User to a PublicUser.
func (u *User) ToPublicUser() PublicUser {
	return PublicUser{
		BaseUser: u.BaseUser,
		Online:   true, // This should be set based on presence information
	}
}

// PersonalUser represents a subset of user information that is safe to share with user.
type PersonalUser struct {
	// BaseUser embeds the base user information.
	BaseUser

	// Email is the user's email address.
	Email string `json:"email"`

	// Settings contains the user's personal settings.
	Settings UserSettings `json:"settings"`

	// Connections contains the user's social connections.
	Connections UserConnections `json:"connections"`
}

// ToPersonalUser converts a User to a PersonalUser.
func (u *User) ToPersonalUser() PersonalUser {
	return PersonalUser{
		BaseUser:    u.BaseUser,
		Email:       u.Email,
		Settings:    u.Settings,
		Connections: u.Connections,
	}
}

// UserRegisterRequest represents the data needed to register a new user.
type UserRegisterRequest struct {
	// Username is the user's chosen username.
	Username string `json:"username" validate:"required,min=3,max=30,username"`

	// Email is the user's email address.
	Email string `json:"email" validate:"required,email"`

	// Password is the user's password.
	Password string `json:"password" validate:"required,min=8,max=72,password"`
}

// UserLoginRequest represents the data needed to log in a user.
type UserLoginRequest struct {
	// Email is the user's email address.
	Email string `json:"email" validate:"required,email"`

	// Password is the user's password.
	Password string `json:"password" validate:"required"`
}

// UserUpdateRequest represents the data needed to update a user.
type UserUpdateRequest struct {
	// Username is the user's chosen username.
	Username string `json:"username" validate:"omitempty,min=3,max=30,username"`

	// AvatarConfig stores the user's avatar customization options.
	AvatarConfig *AvatarConfig `json:"avatarConfig,omitempty"`

	// Profile contains the user's profile information.
	Profile *UserProfile `json:"profile,omitempty"`

	// Settings contains the user's personal settings.
	Settings *UserSettings `json:"settings,omitempty"`
}

// UserPasswordChangeRequest represents the data needed to change a user's password.
type UserPasswordChangeRequest struct {
	// CurrentPassword is the user's current password.
	CurrentPassword string `json:"currentPassword" validate:"required"`

	// NewPassword is the user's new password.
	NewPassword string `json:"newPassword" validate:"required,min=8,max=72,password"`
}

// UserPasswordResetRequest represents the data needed to request a password reset.
type UserPasswordResetRequest struct {
	// Email is the user's email address.
	Email string `json:"email" validate:"required,email"`
}

// UserPasswordResetConfirmRequest represents the data needed to confirm a password reset.
type UserPasswordResetConfirmRequest struct {
	// Token is the reset token.
	Token string `json:"token" validate:"required"`

	// Password is the user's new password.
	Password string `json:"password" validate:"required,min=8,max=72,password"`
}
