// Package models contains the data structures used throughout the application.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Playlist represents a collection of media items curated by a user.
type Playlist struct {
	// ID is the unique identifier for the playlist.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// Name is the display name of the playlist.
	Name string `json:"name" bson:"name" validate:"required,min=1,max=50"`

	// Description provides information about the playlist.
	Description string `json:"description" bson:"description" validate:"max=1000"`

	// Owner is the ID of the user who owns the playlist.
	Owner bson.ObjectID `json:"owner" bson:"owner"`

	// IsActive indicates whether this is the user's currently active playlist.
	IsActive bool `json:"isActive" bson:"isActive"`

	// IsPrivate indicates whether the playlist is private.
	IsPrivate bool `json:"isPrivate" bson:"isPrivate"`

	// Items are the media items in the playlist.
	Items []PlaylistItem `json:"items" bson:"items"`

	// Stats contains the playlist's statistics.
	Stats PlaylistStats `json:"stats" bson:"stats"`

	// Tags are keywords that describe the playlist.
	Tags []string `json:"tags" bson:"tags" validate:"dive,max=20"`

	// CoverImage is an optional URL for a playlist cover image.
	CoverImage string `json:"coverImage,omitempty" bson:"coverImage,omitempty" validate:"omitempty,url"`

	// ObjectTimes contains timestamps for this playlist.
	ObjectTimes

	// LastPlayed is the time the playlist was last played from.
	LastPlayed time.Time `json:"lastPlayed" bson:"lastPlayed"`
}

// PlaylistItem represents a media item in a playlist.
type PlaylistItem struct {
	// ID is a unique identifier for this item in the playlist.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// MediaID is the ID of the media item.
	MediaID bson.ObjectID `json:"mediaId" bson:"mediaId"`

	// Order is the position of the item in the playlist.
	Order int `json:"order" bson:"order"`

	// AddedAt is the time the item was added to the playlist.
	AddedAt time.Time `json:"addedAt" bson:"addedAt"`

	// LastPlayed is the time the item was last played from this playlist.
	LastPlayed time.Time `json:"lastPlayed" bson:"lastPlayed"`

	// PlayCount is the number of times the item has been played from this playlist.
	PlayCount int `json:"playCount" bson:"playCount"`

	// Media is the media item (populated when retrieving the playlist).
	Media *MediaInfo `json:"media,omitempty" bson:"-"`
}

// PlaylistStats contains statistics for a playlist.
type PlaylistStats struct {
	// TotalItems is the total number of items in the playlist.
	TotalItems int `json:"totalItems" bson:"totalItems"`

	// TotalDuration is the total duration of all items in the playlist in seconds.
	TotalDuration int `json:"totalDuration" bson:"totalDuration"`

	// TotalPlays is the number of times the playlist has been played.
	TotalPlays int `json:"totalPlays" bson:"totalPlays"`

	// Followers is the number of users following the playlist.
	Followers int `json:"followers" bson:"followers"`

	// LastCalculated is the time the stats were last calculated.
	LastCalculated time.Time `json:"lastCalculated" bson:"lastCalculated"`
}

// PlaylistInfo represents a simplified playlist object for public display.
type PlaylistInfo struct {
	// ID is the unique identifier for the playlist.
	ID bson.ObjectID `json:"id"`

	// Name is the display name of the playlist.
	Name string `json:"name"`

	// Description provides information about the playlist.
	Description string `json:"description"`

	// Owner is information about the user who owns the playlist.
	Owner *PublicUser `json:"owner,omitempty"`

	// IsPrivate indicates whether the playlist is private.
	IsPrivate bool `json:"isPrivate"`

	// ItemCount is the number of items in the playlist.
	ItemCount int `json:"itemCount"`

	// TotalDuration is the total duration of all items in the playlist in seconds.
	TotalDuration int `json:"totalDuration"`

	// Tags are keywords that describe the playlist.
	Tags []string `json:"tags"`

	// CoverImage is an optional URL for a playlist cover image.
	CoverImage string `json:"coverImage,omitempty"`

	// CreatedAt is the time the playlist was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the time the playlist was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}

// ToPlaylistInfo converts a Playlist to a PlaylistInfo.
func (p *Playlist) ToPlaylistInfo(owner *User) PlaylistInfo {
	info := PlaylistInfo{
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		IsPrivate:     p.IsPrivate,
		ItemCount:     len(p.Items),
		TotalDuration: p.Stats.TotalDuration,
		Tags:          p.Tags,
		CoverImage:    p.CoverImage,
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
	}

	if owner != nil {
		pubUser := owner.ToPublicUser()
		info.Owner = &pubUser
	}

	return info
}

// PlaylistCreateRequest represents the data needed to create a new playlist.
type PlaylistCreateRequest struct {
	// Name is the display name of the playlist.
	Name string `json:"name" validate:"required,min=1,max=50"`

	// Description provides information about the playlist.
	Description string `json:"description" validate:"max=1000"`

	// IsPrivate indicates whether the playlist is private.
	IsPrivate bool `json:"isPrivate"`

	// Tags are keywords that describe the playlist.
	Tags []string `json:"tags" validate:"dive,max=20"`

	// CoverImage is an optional URL for a playlist cover image.
	CoverImage string `json:"coverImage,omitempty" validate:"omitempty,url"`
}

// PlaylistUpdateRequest represents the data needed to update a playlist.
type PlaylistUpdateRequest struct {
	// Name is the display name of the playlist.
	Name string `json:"name" validate:"omitempty,min=1,max=50"`

	// Description provides information about the playlist.
	Description string `json:"description" validate:"max=1000"`

	// IsPrivate indicates whether the playlist is private.
	IsPrivate *bool `json:"isPrivate,omitempty"`

	// Tags are keywords that describe the playlist.
	Tags []string `json:"tags" validate:"dive,max=20"`

	// CoverImage is an optional URL for a playlist cover image.
	CoverImage string `json:"coverImage,omitempty" validate:"omitempty,url"`
}

// PlaylistAddItemRequest represents the data needed to add an item to a playlist.
type PlaylistAddItemRequest struct {
	// MediaID is the ID of the media item.
	MediaID bson.ObjectID `json:"mediaId" validate:"required"`

	// Position is the position to insert the item at (zero-based).
	Position *int `json:"position,omitempty"`
}

// PlaylistMoveItemRequest represents the data needed to move an item within a playlist.
type PlaylistMoveItemRequest struct {
	// ItemID is the ID of the playlist item to move.
	ItemID bson.ObjectID `json:"itemId" validate:"required"`

	// NewPosition is the new position for the item (zero-based).
	NewPosition int `json:"newPosition" validate:"min=0"`
}

// PlaylistSetActiveRequest represents the data needed to set a playlist as active.
type PlaylistSetActiveRequest struct {
	// PlaylistID is the ID of the playlist to set as active.
	PlaylistID bson.ObjectID `json:"playlistId" validate:"required"`
}

// PlaylistImportRequest represents the data needed to import a playlist.
type PlaylistImportRequest struct {
	// Source is the source platform to import from (e.g., "youtube", "soundcloud").
	Source string `json:"source" validate:"required,oneof=youtube soundcloud"`

	// SourceID is the ID of the playlist on the source platform.
	SourceID string `json:"sourceId" validate:"required"`

	// Name is the display name for the new playlist.
	Name string `json:"name" validate:"required,min=1,max=50"`

	// IsPrivate indicates whether the playlist should be private.
	IsPrivate bool `json:"isPrivate"`
}

// PlaylistImportResponse represents the response to a playlist import request.
type PlaylistImportResponse struct {
	// PlaylistID is the ID of the newly created playlist.
	PlaylistID bson.ObjectID `json:"playlistId"`

	// Name is the name of the playlist.
	Name string `json:"name"`

	// ItemCount is the number of items imported.
	ItemCount int `json:"itemCount"`

	// FailedItems is the number of items that failed to import.
	FailedItems int `json:"failedItems"`

	// TotalItems is the total number of items found in the source playlist.
	TotalItems int `json:"totalItems"`

	// Success indicates whether the import was successful.
	Success bool `json:"success"`

	// Message provides additional information about the import.
	Message string `json:"message,omitempty"`
}

// PlaylistSearchCriteria represents the criteria for searching playlists.
type PlaylistSearchCriteria struct {
	// Query is the search query.
	Query string `json:"query"`

	// Tags are the tags to filter by.
	Tags []string `json:"tags"`

	// IncludePrivate indicates whether to include private playlists.
	IncludePrivate bool `json:"includePrivate"`

	// OwnerID is the ID of the owner to filter by.
	OwnerID bson.ObjectID `json:"ownerId,omitempty"`

	// SortBy is the field to sort by.
	SortBy string `json:"sortBy"`

	// SortDirection is the direction to sort (asc or desc).
	SortDirection string `json:"sortDirection"`

	// Page is the page number for pagination.
	Page int `json:"page"`

	// Limit is the number of results per page.
	Limit int `json:"limit"`
}

// PlaylistDetailedStats contains detailed statistics for a playlist.
type PlaylistDetailedStats struct {
	// TotalItems is the total number of items in the playlist.
	TotalItems int `json:"totalItems"`

	// TotalDuration is the total duration of all items in the playlist in seconds.
	TotalDuration int `json:"totalDuration"`

	// TotalPlays is the number of times the playlist has been played.
	TotalPlays int `json:"totalPlays"`

	// Followers is the number of users following the playlist.
	Followers int `json:"followers"`

	// AveragePlays is the average number of plays per item.
	AveragePlays float64 `json:"averagePlays"`

	// MediaTypes maps media types to their count in the playlist.
	MediaTypes map[string]int `json:"mediaTypes"`

	// CreatedAt is when the playlist was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the playlist was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}
