// Package playlist provides playlist management functionality.
package playlist

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// Manager handles playlist operations.
type Manager struct {
	playlistRepo repositories.PlaylistRepository
	logger       *utils.Logger
}

// NewManager creates a new playlist manager.
func NewManager(playlistRepo repositories.PlaylistRepository, logger *utils.Logger) *Manager {
	return &Manager{
		playlistRepo: playlistRepo,
		logger:       logger.Named("playlist_manager"),
	}
}

// CreatePlaylist creates a new playlist.
func (m *Manager) CreatePlaylist(ctx context.Context, playlist *models.Playlist) (*models.Playlist, error) {
	m.logger.Debug("Creating playlist", "name", playlist.Name, "owner", playlist.Owner.Hex())

	err := m.playlistRepo.Create(ctx, playlist)
	if err != nil {
		return nil, err
	}

	return playlist, nil
}

// GetPlaylist gets a playlist by ID.
func (m *Manager) GetPlaylist(ctx context.Context, id bson.ObjectID) (*models.Playlist, error) {
	m.logger.Debug("Getting playlist", "id", id.Hex())
	return m.playlistRepo.FindByID(ctx, id)
}

// GetUserPlaylists gets all playlists for a user.
func (m *Manager) GetUserPlaylists(ctx context.Context, userID bson.ObjectID) ([]*models.Playlist, error) {
	m.logger.Debug("Getting user playlists", "userID", userID.Hex())
	return m.playlistRepo.FindUserPlaylists(ctx, userID)
}

// UpdatePlaylist updates a playlist.
func (m *Manager) UpdatePlaylist(ctx context.Context, playlist *models.Playlist) (*models.Playlist, error) {
	m.logger.Debug("Updating playlist", "id", playlist.ID.Hex(), "name", playlist.Name)

	err := m.playlistRepo.Update(ctx, playlist)
	if err != nil {
		return nil, err
	}

	return playlist, nil
}

// DeletePlaylist deletes a playlist.
func (m *Manager) DeletePlaylist(ctx context.Context, id bson.ObjectID) error {
	m.logger.Debug("Deleting playlist", "id", id.Hex())
	return m.playlistRepo.Delete(ctx, id)
}

// AddPlaylistItem adds an item to a playlist.
func (m *Manager) AddPlaylistItem(ctx context.Context, playlistID, mediaID bson.ObjectID, position int) (*models.Playlist, error) {
	m.logger.Debug("Adding item to playlist", "playlistID", playlistID.Hex(), "mediaID", mediaID.Hex(), "position", position)

	err := m.playlistRepo.AddItem(ctx, playlistID, mediaID, position)
	if err != nil {
		return nil, err
	}

	// Return the updated playlist
	return m.playlistRepo.FindByID(ctx, playlistID)
}

// RemovePlaylistItem removes an item from a playlist.
func (m *Manager) RemovePlaylistItem(ctx context.Context, playlistID, itemID bson.ObjectID) (*models.Playlist, error) {
	m.logger.Debug("Removing item from playlist", "playlistID", playlistID.Hex(), "itemID", itemID.Hex())

	err := m.playlistRepo.RemoveItem(ctx, playlistID, itemID)
	if err != nil {
		return nil, err
	}

	// Return the updated playlist
	return m.playlistRepo.FindByID(ctx, playlistID)
}

// ImportPlaylist imports a playlist from an external source.
func (m *Manager) ImportPlaylist(ctx context.Context, ownerID bson.ObjectID, source string, externalID string) (*models.Playlist, error) {
	m.logger.Debug("Importing playlist", "ownerID", ownerID.Hex(), "source", source, "externalID", externalID)

	// Implementation would depend on the external source
	// For now, just create an empty playlist
	now := time.Now()
	playlist := &models.Playlist{
		Owner:       ownerID,
		Name:        "Imported Playlist",
		Description: "Imported from " + source,
		Items:       []models.PlaylistItem{},
		ObjectTimes: models.NewObjectTimes(now),
		Stats: models.PlaylistStats{
			TotalItems:     0,
			TotalDuration:  0,
			TotalPlays:     0,
			Followers:      0,
			LastCalculated: now,
		},
	}

	err := m.playlistRepo.Create(ctx, playlist)
	if err != nil {
		return nil, err
	}

	return playlist, nil
}

// SetActivePlaylist sets a playlist as the user's active playlist.
func (m *Manager) SetActivePlaylist(ctx context.Context, userID, playlistID bson.ObjectID) error {
	m.logger.Debug("Setting active playlist", "userID", userID.Hex(), "playlistID", playlistID.Hex())
	return m.playlistRepo.SetActivePlaylist(ctx, userID, playlistID)
}

// GetActivePlaylist gets the user's active playlist.
func (m *Manager) GetActivePlaylist(ctx context.Context, userID bson.ObjectID) (*models.Playlist, error) {
	m.logger.Debug("Getting active playlist", "userID", userID.Hex())
	return m.playlistRepo.GetActivePlaylist(ctx, userID)
}

// ShufflePlaylist randomizes the order of items in a playlist.
func (m *Manager) ShufflePlaylist(ctx context.Context, playlistID bson.ObjectID) (*models.Playlist, error) {
	m.logger.Debug("Shuffling playlist", "playlistID", playlistID.Hex())

	err := m.playlistRepo.ShufflePlaylist(ctx, playlistID)
	if err != nil {
		return nil, err
	}

	return m.playlistRepo.FindByID(ctx, playlistID)
}

// SearchPlaylists searches for playlists based on criteria.
func (m *Manager) SearchPlaylists(ctx context.Context, criteria models.PlaylistSearchCriteria) ([]*models.Playlist, int64, error) {
	m.logger.Debug("Searching playlists", "query", criteria.Query, "tags", criteria.Tags)
	return m.playlistRepo.SearchPlaylists(ctx, criteria)
}
