// Package models contains the data structures used throughout the application.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Media represents a media item that can be played in a room.
type Media struct {
	// ID is the unique identifier for the media.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// Type is the source type of the media (e.g., "youtube", "soundcloud").
	Type string `json:"type" bson:"type" validate:"required,oneof=youtube soundcloud"`

	// SourceID is the ID of the media on the original platform.
	SourceID string `json:"sourceId" bson:"sourceId" validate:"required"`

	// Title is the title of the media.
	Title string `json:"title" bson:"title" validate:"required,max=200"`

	// Artist is the artist/creator of the media.
	Artist string `json:"artist" bson:"artist" validate:"max=200"`

	// Thumbnail is the URL of the media's thumbnail image.
	Thumbnail string `json:"thumbnail" bson:"thumbnail" validate:"url"`

	// Duration is the duration of the media in seconds.
	Duration int `json:"duration" bson:"duration" validate:"min=1,max=3600"`

	// Metadata contains additional information about the media.
	Metadata MediaMetadata `json:"metadata" bson:"metadata"`

	// Stats contains the media's statistics.
	Stats MediaStats `json:"stats" bson:"stats"`

	// AddedBy is the ID of the user who added the media.
	AddedBy bson.ObjectID `json:"addedBy" bson:"addedBy"`

	// ObjectTimes contains timestamps for this media.
	ObjectTimes
}

// MediaMetadata contains additional information about a media item.
type MediaMetadata struct {
	// Views is the number of views on the original platform.
	Views int64 `json:"views" bson:"views"`

	// Likes is the number of likes on the original platform.
	Likes int64 `json:"likes" bson:"likes"`

	// PublishedAt is the time the media was published on the original platform.
	PublishedAt time.Time `json:"publishedAt" bson:"publishedAt"`

	// ChannelID is the ID of the channel/user who uploaded the media.
	ChannelID string `json:"channelId" bson:"channelId"`

	// ChannelTitle is the name of the channel/user who uploaded the media.
	ChannelTitle string `json:"channelTitle" bson:"channelTitle"`

	// Description is the description of the media.
	Description string `json:"description" bson:"description"`

	// Tags are the tags assigned to the media.
	Tags []string `json:"tags" bson:"tags"`

	// Categories are the categories the media belongs to.
	Categories []string `json:"categories" bson:"categories"`

	// ContentRating is the content rating of the media.
	ContentRating string `json:"contentRating" bson:"contentRating"`

	// Restricted indicates whether the media has content restrictions.
	Restricted bool `json:"restricted" bson:"restricted"`
}

// MediaStats contains statistics for a media item within the app.
type MediaStats struct {
	// PlayCount is the number of times the media has been played.
	PlayCount int `json:"playCount" bson:"playCount"`

	// WootCount is the number of woots (likes) the media has received.
	WootCount int `json:"wootCount" bson:"wootCount"`

	// MehCount is the number of mehs (dislikes) the media has received.
	MehCount int `json:"mehCount" bson:"mehCount"`

	// GrabCount is the number of times the media has been added to playlists.
	GrabCount int `json:"grabCount" bson:"grabCount"`

	// SkipCount is the number of times the media has been skipped.
	SkipCount int `json:"skipCount" bson:"skipCount"`

	// LastPlayed is the time the media was last played.
	LastPlayed time.Time `json:"lastPlayed" bson:"lastPlayed"`

	// AggregateRating is the overall rating of the media.
	AggregateRating float64 `json:"aggregateRating" bson:"aggregateRating"`

	// LastUpdated is the time the stats were last updated.
	LastUpdated time.Time `json:"lastUpdated" bson:"lastUpdated"`
}

// MediaInfo represents a simplified media object for public display and realtime updates.
type MediaInfo struct {
	// ID is the unique identifier for the media.
	ID bson.ObjectID `json:"id"`

	// Type is the source type of the media (e.g., "youtube", "soundcloud").
	Type string `json:"type"`

	// SourceID is the ID of the media on the original platform.
	SourceID string `json:"sourceId"`

	// Title is the title of the media.
	Title string `json:"title"`

	// Artist is the artist/creator of the media.
	Artist string `json:"artist"`

	// Thumbnail is the URL of the media's thumbnail image.
	Thumbnail string `json:"thumbnail"`

	// Duration is the duration of the media in seconds.
	Duration int `json:"duration"`

	// PlayCount is the number of times the media has been played.
	PlayCount int `json:"playCount"`

	// AddedBy is information about the user who added the media.
	AddedBy *PublicUser `json:"addedBy,omitempty"`
}

// ToMediaInfo converts a Media to a MediaInfo.
func (m *Media) ToMediaInfo(addedByUser *User) *MediaInfo {
	info := &MediaInfo{
		ID:        m.ID,
		Type:      m.Type,
		SourceID:  m.SourceID,
		Title:     m.Title,
		Artist:    m.Artist,
		Thumbnail: m.Thumbnail,
		Duration:  m.Duration,
		PlayCount: m.Stats.PlayCount,
	}

	if addedByUser != nil {
		pubUser := addedByUser.ToPublicUser()
		info.AddedBy = &pubUser
	}

	return info
}

// MediaSearchRequest represents the data needed to search for media.
type MediaSearchRequest struct {
	// Query is the search query.
	Query string `json:"query" validate:"required,min=2,max=100"`

	// Source is the source to search (e.g., "youtube", "soundcloud").
	Source string `json:"source" validate:"required,oneof=youtube soundcloud all"`

	// Limit is the maximum number of results to return.
	Limit int `json:"limit" validate:"min=1,max=50"`
}

// MediaSearchResponse represents the response to a media search.
type MediaSearchResponse struct {
	// Results is the list of media search results.
	Results []MediaSearchResult `json:"results"`

	// NextPageToken is the token for retrieving the next page of results.
	NextPageToken string `json:"nextPageToken,omitempty"`

	// TotalResults is the total number of results.
	TotalResults int `json:"totalResults"`

	// Source is the source that was searched.
	Source string `json:"source"`

	// Query is the search query.
	Query string `json:"query"`
}

// MediaSearchResult represents a single result from a media search.
type MediaSearchResult struct {
	// Type is the source type of the media.
	Type string `json:"type"`

	// SourceID is the ID of the media on the original platform.
	SourceID string `json:"sourceId"`

	// Title is the title of the media.
	Title string `json:"title"`

	// Artist is the artist/creator of the media.
	Artist string `json:"artist"`

	// Thumbnail is the URL of the media's thumbnail image.
	Thumbnail string `json:"thumbnail"`

	// Duration is the duration of the media in seconds.
	Duration int `json:"duration"`

	// Views is the number of views on the original platform.
	Views int64 `json:"views"`

	// PublishedAt is the time the media was published.
	PublishedAt time.Time `json:"publishedAt"`

	// Description is a short description of the media.
	Description string `json:"description"`

	// ChannelTitle is the name of the channel that published the media.
	ChannelTitle string `json:"channelTitle"`

	// Restricted indicates whether the media has content restrictions.
	Restricted bool `json:"restricted"`
}

// MediaHistoryEntry represents a record of a media item being played in a room.
type MediaHistoryEntry struct {
	// ID is the unique identifier for this history entry.
	ID bson.ObjectID `json:"id" bson:"_id,omitempty"`

	// RoomID is the ID of the room where the media was played.
	RoomID bson.ObjectID `json:"roomId" bson:"roomId"`

	// MediaID is the ID of the media that was played.
	MediaID bson.ObjectID `json:"mediaId" bson:"mediaId"`

	// DjID is the ID of the user who played the media.
	DjID bson.ObjectID `json:"djId" bson:"djId"`

	// Title is the title of the media.
	Title string `json:"title" bson:"title"`

	// Artist is the artist/creator of the media.
	Artist string `json:"artist" bson:"artist"`

	// Type is the source type of the media.
	Type string `json:"type" bson:"type"`

	// SourceID is the ID of the media on the original platform.
	SourceID string `json:"sourceId" bson:"sourceId"`

	// StartTime is when the media started playing.
	StartTime time.Time `json:"startTime" bson:"startTime"`

	// EndTime is when the media finished playing.
	EndTime time.Time `json:"endTime" bson:"endTime"`

	// Duration is the actual duration the media played.
	Duration int `json:"duration" bson:"duration"`

	// Votes contains the vote counts for the media.
	Votes MediaVotes `json:"votes" bson:"votes"`

	// UserCount is the number of users in the room when the media was played.
	UserCount int `json:"userCount" bson:"userCount"`

	// WasSkipped indicates whether the media was skipped.
	WasSkipped bool `json:"wasSkipped" bson:"wasSkipped"`

	// SkipReason is the reason the media was skipped.
	SkipReason string `json:"skipReason,omitempty" bson:"skipReason,omitempty"`

	// SkippedBy is the ID of the user who skipped the media.
	SkippedBy bson.ObjectID `json:"skippedBy,omitempty" bson:"skippedBy,omitempty"`
}

// MediaVotes represents the voting stats for a media play event.
type MediaVotes struct {
	// Woots is the number of woots (likes) received.
	Woots int `json:"woots" bson:"woots"`

	// Mehs is the number of mehs (dislikes) received.
	Mehs int `json:"mehs" bson:"mehs"`

	// Grabs is the number of grabs received.
	Grabs int `json:"grabs" bson:"grabs"`

	// Voters is a map of user IDs to their votes.
	Voters map[string]string `json:"voters" bson:"voters"`
}

// MediaVoteRequest represents a request to vote on media.
type MediaVoteRequest struct {
	// Vote is the vote value ("woot", "meh", or "grab").
	Vote string `json:"vote" validate:"required,oneof=woot meh grab"`
}

// MediaVoteResponse represents the response to a vote request.
type MediaVoteResponse struct {
	// Success indicates whether the vote was successful.
	Success bool `json:"success"`

	// CurrentVotes is the updated vote counts.
	CurrentVotes MediaVotes `json:"currentVotes"`

	// Message is a message about the vote status.
	Message string `json:"message,omitempty"`
}
