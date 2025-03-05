// Package repositories contains MongoDB repository implementations.
package repositories

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/utils"
)

// Collection names
const (
	historyCollection               = "history"
	histPlayHistoryCollection       = "play_history"
	histUserHistoryCollection       = "user_history"
	histRoomHistoryCollection       = "room_history"
	histDJHistoryCollection         = "dj_history"
	histSessionHistoryCollection    = "session_history"
	histModerationHistoryCollection = "moderation_history"
)

// HistoryRepository defines the interface for history data access operations.
type HistoryRepository interface {
	// Generic history operations
	CreateHistory(ctx context.Context, history *models.History) error
	FindHistoryByID(ctx context.Context, id bson.ObjectID) (*models.History, error)
	FindHistoryByType(ctx context.Context, historyType string, skip, limit int) ([]*models.History, error)
	FindHistoryByReference(ctx context.Context, referenceID bson.ObjectID, skip, limit int) ([]*models.History, error)

	// Play history operations
	CreatePlayHistory(ctx context.Context, playHistory *models.PlayHistory) error
	FindPlayHistoryByID(ctx context.Context, id bson.ObjectID) (*models.PlayHistory, error)
	FindPlayHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.PlayHistory, error)
	FindPlayHistoryByDJ(ctx context.Context, djID bson.ObjectID, skip, limit int) ([]*models.PlayHistory, error)
	FindPlayHistoryByMedia(ctx context.Context, mediaID bson.ObjectID, skip, limit int) ([]*models.PlayHistory, error)
	GetPlayHistorySummary(ctx context.Context, roomID bson.ObjectID) (*models.HistorySummary, error)

	// User history operations
	CreateUserHistory(ctx context.Context, userHistory *models.UserHistory) error
	FindUserHistoryByType(ctx context.Context, userID bson.ObjectID, historyType string, skip, limit int) ([]*models.UserHistory, error)
	FindUserHistoryByTimeRange(ctx context.Context, userID bson.ObjectID, startTime, endTime time.Time, skip, limit int) ([]*models.UserHistory, error)

	// Room history operations
	CreateRoomHistory(ctx context.Context, roomHistory *models.RoomHistory) error
	FindRoomHistoryByID(ctx context.Context, id bson.ObjectID) (*models.RoomHistory, error)
	FindRoomHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.RoomHistory, error)
	FindRoomHistoryByType(ctx context.Context, roomID bson.ObjectID, historyType string, skip, limit int) ([]*models.RoomHistory, error)

	// DJ history operations
	CreateDJHistory(ctx context.Context, djHistory *models.DJHistory) error
	FindDJHistoryByUser(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.DJHistory, error)
	FindDJHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.DJHistory, error)
	UpdateDJHistoryEndTime(ctx context.Context, id bson.ObjectID, endTime time.Time, leaveReason string) error

	// Session history operations
	CreateSessionHistory(ctx context.Context, sessionHistory *models.SessionHistory) error
	UpdateSessionHistoryEndTime(ctx context.Context, id bson.ObjectID, endTime time.Time) error
	FindSessionHistoryByUser(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.SessionHistory, error)

	// Moderation history operations
	CreateModerationHistory(ctx context.Context, moderationHistory *models.ModerationHistory) error
	FindModerationHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.ModerationHistory, error)
	FindModerationHistoryByModerator(ctx context.Context, moderatorID bson.ObjectID, skip, limit int) ([]*models.ModerationHistory, error)
	FindModerationHistoryByUser(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.ModerationHistory, error)

	// Statistics operations
	GetTopTracks(ctx context.Context, roomID bson.ObjectID, limit int) ([]models.TopTrackSummary, error)
	GetTopDJs(ctx context.Context, roomID bson.ObjectID, limit int) ([]models.TopDJSummary, error)
}

// historyRepository is the MongoDB implementation of HistoryRepository.
type historyRepository struct {
	historyCollection           *mongo.Collection
	playHistoryCollection       *mongo.Collection
	userHistoryCollection       *mongo.Collection
	roomHistoryCollection       *mongo.Collection
	djHistoryCollection         *mongo.Collection
	sessionHistoryCollection    *mongo.Collection
	moderationHistoryCollection *mongo.Collection
	logger                      *utils.Logger
}

// NewHistoryRepository creates a new instance of HistoryRepository.
func NewHistoryRepository(db *mongo.Database, logger *utils.Logger) HistoryRepository {
	return &historyRepository{
		historyCollection:           db.Collection(historyCollection),
		playHistoryCollection:       db.Collection(histPlayHistoryCollection),
		userHistoryCollection:       db.Collection(histUserHistoryCollection),
		roomHistoryCollection:       db.Collection(histRoomHistoryCollection),
		djHistoryCollection:         db.Collection(histDJHistoryCollection),
		sessionHistoryCollection:    db.Collection(histSessionHistoryCollection),
		moderationHistoryCollection: db.Collection(histModerationHistoryCollection),
		logger:                      logger.Named("history_repository"),
	}
}

// CreateHistory creates a new generic history record.
func (r *historyRepository) CreateHistory(ctx context.Context, history *models.History) error {
	if history.ID.IsZero() {
		history.ID = bson.NewObjectID()
	}

	if history.Timestamp.IsZero() {
		history.Timestamp = time.Now()
	}

	_, err := r.historyCollection.InsertOne(ctx, history)
	if err != nil {
		r.logger.Error("Failed to create history record", err, "type", history.Type)
		return models.NewInternalError(err, "Failed to create history record")
	}

	return nil
}

// FindHistoryByID finds a history record by its ID.
func (r *historyRepository) FindHistoryByID(ctx context.Context, id bson.ObjectID) (*models.History, error) {
	var history models.History

	err := r.historyCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&history)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("history record not found")
		}
		r.logger.Error("Failed to find history by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find history record")
	}

	return &history, nil
}

// FindHistoryByType finds history records by type.
func (r *historyRepository) FindHistoryByType(ctx context.Context, historyType string, skip, limit int) ([]*models.History, error) {
	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.historyCollection.Find(ctx, bson.M{"type": historyType}, opts)
	if err != nil {
		r.logger.Error("Failed to find history by type", err, "type", historyType)
		return nil, models.NewInternalError(err, "Failed to find history records")
	}
	defer cursor.Close(ctx)

	var histories []*models.History
	if err = cursor.All(ctx, &histories); err != nil {
		r.logger.Error("Failed to decode history records", err)
		return nil, models.NewInternalError(err, "Failed to decode history records")
	}

	return histories, nil
}

// FindHistoryByReference finds history records by reference ID.
func (r *historyRepository) FindHistoryByReference(ctx context.Context, referenceID bson.ObjectID, skip, limit int) ([]*models.History, error) {
	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.historyCollection.Find(ctx, bson.M{"referenceId": referenceID}, opts)
	if err != nil {
		r.logger.Error("Failed to find history by reference", err, "referenceId", referenceID.Hex())
		return nil, models.NewInternalError(err, "Failed to find history records")
	}
	defer cursor.Close(ctx)

	var histories []*models.History
	if err = cursor.All(ctx, &histories); err != nil {
		r.logger.Error("Failed to decode history records", err)
		return nil, models.NewInternalError(err, "Failed to decode history records")
	}

	return histories, nil
}

// CreatePlayHistory creates a new play history record.
func (r *historyRepository) CreatePlayHistory(ctx context.Context, playHistory *models.PlayHistory) error {
	if playHistory.ID.IsZero() {
		playHistory.ID = bson.NewObjectID()
	}

	if playHistory.StartTime.IsZero() {
		playHistory.StartTime = time.Now()
	}

	// Calculate play duration if end time is set
	if !playHistory.EndTime.IsZero() {
		playHistory.Duration = int(playHistory.EndTime.Sub(playHistory.StartTime).Seconds())
	}

	// Initialize votes object if nil
	if playHistory.Votes.Voters == nil {
		playHistory.Votes.Voters = make(map[string]string)
	}

	_, err := r.playHistoryCollection.InsertOne(ctx, playHistory)
	if err != nil {
		r.logger.Error("Failed to create play history", err, "mediaId", playHistory.MediaID.Hex(), "djId", playHistory.DjID.Hex())
		return models.NewInternalError(err, "Failed to create play history")
	}

	// Create generic history record
	history := &models.History{
		Type:        "media",
		ReferenceID: playHistory.MediaID,
		Timestamp:   playHistory.StartTime,
		Metadata: map[string]any{
			"playHistoryId": playHistory.ID,
			"roomId":        playHistory.RoomID,
			"djId":          playHistory.DjID,
		},
	}

	err = r.CreateHistory(ctx, history)
	if err != nil {
		r.logger.Error("Failed to create generic history for play", err)
		// Continue anyway, the play history was recorded
	}

	return nil
}

// FindPlayHistoryByID finds a play history record by its ID.
func (r *historyRepository) FindPlayHistoryByID(ctx context.Context, id bson.ObjectID) (*models.PlayHistory, error) {
	var playHistory models.PlayHistory

	err := r.playHistoryCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&playHistory)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("play history not found")
		}
		r.logger.Error("Failed to find play history by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find play history")
	}

	return &playHistory, nil
}

// FindPlayHistoryByRoom finds play history records for a room.
func (r *historyRepository) FindPlayHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.PlayHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"startTime": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.playHistoryCollection.Find(ctx, bson.M{"roomId": roomID}, opts)
	if err != nil {
		r.logger.Error("Failed to find play history by room", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to find play history")
	}
	defer cursor.Close(ctx)

	var playHistories []*models.PlayHistory
	if err = cursor.All(ctx, &playHistories); err != nil {
		r.logger.Error("Failed to decode play history records", err)
		return nil, models.NewInternalError(err, "Failed to decode play history")
	}

	return playHistories, nil
}

// FindPlayHistoryByDJ finds play history records for a DJ.
func (r *historyRepository) FindPlayHistoryByDJ(ctx context.Context, djID bson.ObjectID, skip, limit int) ([]*models.PlayHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"startTime": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.playHistoryCollection.Find(ctx, bson.M{"djId": djID}, opts)
	if err != nil {
		r.logger.Error("Failed to find play history by DJ", err, "djId", djID.Hex())
		return nil, models.NewInternalError(err, "Failed to find play history")
	}
	defer cursor.Close(ctx)

	var playHistories []*models.PlayHistory
	if err = cursor.All(ctx, &playHistories); err != nil {
		r.logger.Error("Failed to decode play history records", err)
		return nil, models.NewInternalError(err, "Failed to decode play history")
	}

	return playHistories, nil
}

// FindPlayHistoryByMedia finds play history records for a media item.
func (r *historyRepository) FindPlayHistoryByMedia(ctx context.Context, mediaID bson.ObjectID, skip, limit int) ([]*models.PlayHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"startTime": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.playHistoryCollection.Find(ctx, bson.M{"mediaId": mediaID}, opts)
	if err != nil {
		r.logger.Error("Failed to find play history by media", err, "mediaId", mediaID.Hex())
		return nil, models.NewInternalError(err, "Failed to find play history")
	}
	defer cursor.Close(ctx)

	var playHistories []*models.PlayHistory
	if err = cursor.All(ctx, &playHistories); err != nil {
		r.logger.Error("Failed to decode play history records", err)
		return nil, models.NewInternalError(err, "Failed to decode play history")
	}

	return playHistories, nil
}

// GetPlayHistorySummary retrieves a summary of play history stats for a room.
func (r *historyRepository) GetPlayHistorySummary(ctx context.Context, roomID bson.ObjectID) (*models.HistorySummary, error) {
	// First, count total plays
	totalPlays, err := r.playHistoryCollection.CountDocuments(ctx, bson.M{"roomId": roomID})
	if err != nil {
		r.logger.Error("Failed to count total plays", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to generate history summary")
	}

	// Pipeline for aggregating unique tracks
	uniqueTracksPipeline := mongo.Pipeline{
		{cmdMatch(bson.M{"roomId": roomID})},
		{cmdGroup(bson.M{
			"_id":   "$mediaId",
			"count": bson.M{"$sum": 1},
		})},
		{cmdCount("count")},
	}

	uniqueTracksResult, err := r.playHistoryCollection.Aggregate(ctx, uniqueTracksPipeline)
	if err != nil {
		r.logger.Error("Failed to aggregate unique tracks", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to generate history summary")
	}
	defer uniqueTracksResult.Close(ctx)

	var uniqueTracksDoc struct{ Count int64 }
	if uniqueTracksResult.Next(ctx) {
		if err := uniqueTracksResult.Decode(&uniqueTracksDoc); err != nil {
			r.logger.Error("Failed to count unique tracks", err, "roomId", roomID.Hex())
			return nil, models.NewInternalError(err, "Failed to generate history summary")
		}
	} else if err := uniqueTracksResult.Err(); err != nil {
		r.logger.Error("Failed to count unique tracks", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to generate history summary")
	}
	totalUniqueTracks := uniqueTracksDoc.Count

	// Pipeline for aggregating unique DJs
	uniqueDJsPipeline := mongo.Pipeline{
		{cmdMatch(bson.M{"roomId": roomID})},
		{cmdGroup(bson.M{
			"_id": "$djId",
		})},
		{cmdCount("count")},
	}

	uniqueDJsResult, err := r.playHistoryCollection.Aggregate(ctx, uniqueDJsPipeline)
	if err != nil {
		r.logger.Error("Failed to aggregate unique DJs", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to generate history summary")
	}
	defer uniqueDJsResult.Close(ctx)

	var uniqueDJsDoc struct{ Count int64 }
	if uniqueDJsResult.Next(ctx) {
		if err := uniqueDJsResult.Decode(&uniqueDJsDoc); err != nil {
			r.logger.Error("Failed to count unique DJs", err, "roomId", roomID.Hex())
			return nil, models.NewInternalError(err, "Failed to generate history summary")
		}
	}
	totalDJs := uniqueDJsDoc.Count

	// Pipeline for calculating total play time
	playTimePipeline := mongo.Pipeline{
		{cmdMatch(bson.M{"roomId": roomID})},
		{cmdGroup(bson.M{
			"_id":           nil,
			"totalDuration": bson.M{"$sum": "$duration"},
		})},
	}

	playTimeResult, err := r.playHistoryCollection.Aggregate(ctx, playTimePipeline)
	if err != nil {
		r.logger.Error("Failed to aggregate total play time", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to generate history summary")
	}
	defer playTimeResult.Close(ctx)

	var playTimeDoc struct{ TotalDuration int64 }
	if playTimeResult.Next(ctx) {
		if err := playTimeResult.Decode(&playTimeDoc); err != nil {
			r.logger.Error("Failed to calculate total play time", err, "roomId", roomID.Hex())
			return nil, models.NewInternalError(err, "Failed to generate history summary")
		}
	}
	totalPlayTime := playTimeDoc.TotalDuration

	// Pipeline for calculating average votes
	votesPipeline := mongo.Pipeline{
		{cmdMatch(bson.M{"roomId": roomID})},
		{cmdGroup(bson.M{
			"_id":        nil,
			"totalWoots": bson.M{"$sum": "$votes.woots"},
			"totalMehs":  bson.M{"$sum": "$votes.mehs"},
			"totalGrabs": bson.M{"$sum": "$votes.grabs"},
			"count":      bson.M{"$sum": 1},
		})},
	}

	votesResult, err := r.playHistoryCollection.Aggregate(ctx, votesPipeline)
	if err != nil {
		r.logger.Error("Failed to aggregate votes", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to generate history summary")
	}
	defer votesResult.Close(ctx)

	var votesDoc struct {
		TotalWoots int
		TotalMehs  int
		TotalGrabs int
		Count      int
	}
	if votesResult.Next(ctx) {
		if err := votesResult.Decode(&votesDoc); err != nil {
			r.logger.Error("Failed to decode votes result", err, "roomId", roomID.Hex())
			return nil, models.NewInternalError(err, "Failed to generate history summary")
		}
	} else if err := votesResult.Err(); err != nil {
		r.logger.Error("Failed to calculate average votes", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to generate history summary")
	}

	// Calculate averages
	var avgWoots, avgMehs, avgGrabs float64
	if votesDoc.Count > 0 {
		avgWoots = float64(votesDoc.TotalWoots) / float64(votesDoc.Count)
		avgMehs = float64(votesDoc.TotalMehs) / float64(votesDoc.Count)
		avgGrabs = float64(votesDoc.TotalGrabs) / float64(votesDoc.Count)
	}

	// Get top tracks and DJs
	topTracks, err := r.GetTopTracks(ctx, roomID, 10)
	if err != nil {
		r.logger.Error("Failed to get top tracks", err, "roomId", roomID.Hex())
		// Continue with empty list
		topTracks = []models.TopTrackSummary{}
	}

	topDJs, err := r.GetTopDJs(ctx, roomID, 10)
	if err != nil {
		r.logger.Error("Failed to get top DJs", err, "roomId", roomID.Hex())
		// Continue with empty list
		topDJs = []models.TopDJSummary{}
	}

	// Create summary
	summary := &models.HistorySummary{
		TotalPlays:        int(totalPlays),
		TotalUniqueTracks: int(totalUniqueTracks),
		TotalDJs:          int(totalDJs),
		TotalPlayTime:     totalPlayTime,
		AverageVotes: struct {
			Woots float64 `json:"woots"`
			Mehs  float64 `json:"mehs"`
			Grabs float64 `json:"grabs"`
		}{
			Woots: avgWoots,
			Mehs:  avgMehs,
			Grabs: avgGrabs,
		},
		TopTracks:   topTracks,
		TopDJs:      topDJs,
		LastUpdated: time.Now(),
	}

	return summary, nil
}

// CreateUserHistory creates a new user history record.
func (r *historyRepository) CreateUserHistory(ctx context.Context, userHistory *models.UserHistory) error {
	if userHistory.ID.IsZero() {
		userHistory.ID = bson.NewObjectID()
	}

	if userHistory.Timestamp.IsZero() {
		userHistory.Timestamp = time.Now()
	}

	_, err := r.userHistoryCollection.InsertOne(ctx, userHistory)
	if err != nil {
		r.logger.Error("Failed to create user history", err, "userId", userHistory.UserID.Hex(), "type", userHistory.Type)
		return models.NewInternalError(err, "Failed to create user history")
	}

	// Create generic history record
	history := &models.History{
		Type:        "user",
		ReferenceID: userHistory.UserID,
		Timestamp:   userHistory.Timestamp,
		Metadata: map[string]any{
			"userHistoryId": userHistory.ID,
			"type":          userHistory.Type,
		},
	}

	err = r.CreateHistory(ctx, history)
	if err != nil {
		r.logger.Error("Failed to create generic history for user", err)
		// Continue anyway, the user history was recorded
	}

	return nil
}

// FindUserHistoryByType finds user history records by type.
func (r *historyRepository) FindUserHistoryByType(ctx context.Context, userID bson.ObjectID, historyType string, skip, limit int) ([]*models.UserHistory, error) {
	filter := bson.M{"userId": userID}
	if historyType != "" {
		filter["type"] = historyType
	}

	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.userHistoryCollection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find user history", err, "userId", userID.Hex(), "type", historyType)
		return nil, models.NewInternalError(err, "Failed to find user history")
	}
	defer cursor.Close(ctx)

	var userHistories []*models.UserHistory
	if err = cursor.All(ctx, &userHistories); err != nil {
		r.logger.Error("Failed to decode user history records", err)
		return nil, models.NewInternalError(err, "Failed to decode user history")
	}

	return userHistories, nil
}

// FindUserHistoryByTimeRange finds user history records within a time range.
func (r *historyRepository) FindUserHistoryByTimeRange(ctx context.Context, userID bson.ObjectID, startTime, endTime time.Time, skip, limit int) ([]*models.UserHistory, error) {
	filter := bson.M{
		"userId": userID,
		"timestamp": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}

	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.userHistoryCollection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find user history by time range", err, "userId", userID.Hex())
		return nil, models.NewInternalError(err, "Failed to find user history")
	}
	defer cursor.Close(ctx)

	var userHistories []*models.UserHistory
	if err = cursor.All(ctx, &userHistories); err != nil {
		r.logger.Error("Failed to decode user history records", err)
		return nil, models.NewInternalError(err, "Failed to decode user history")
	}

	return userHistories, nil
}

// CreateRoomHistory creates a new room history record.
func (r *historyRepository) CreateRoomHistory(ctx context.Context, roomHistory *models.RoomHistory) error {
	if roomHistory.ID.IsZero() {
		roomHistory.ID = bson.NewObjectID()
	}

	if roomHistory.Timestamp.IsZero() {
		roomHistory.Timestamp = time.Now()
	}

	_, err := r.roomHistoryCollection.InsertOne(ctx, roomHistory)
	if err != nil {
		r.logger.Error("Failed to create room history", err, "roomId", roomHistory.RoomID.Hex(), "type", roomHistory.Type)
		return models.NewInternalError(err, "Failed to create room history")
	}

	// Create generic history record
	history := &models.History{
		Type:        "room",
		ReferenceID: roomHistory.RoomID,
		Timestamp:   roomHistory.Timestamp,
		Metadata: map[string]any{
			"roomHistoryId": roomHistory.ID,
			"type":          roomHistory.Type,
		},
	}

	err = r.CreateHistory(ctx, history)
	if err != nil {
		r.logger.Error("Failed to create generic history for room", err)
		// Continue anyway, the room history was recorded
	}

	return nil
}

// FindRoomHistoryByID finds a room history record by its ID.
func (r *historyRepository) FindRoomHistoryByID(ctx context.Context, id bson.ObjectID) (*models.RoomHistory, error) {
	var roomHistory models.RoomHistory

	err := r.roomHistoryCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&roomHistory)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("room history not found")
		}
		r.logger.Error("Failed to find room history by ID", err, "id", id.Hex())
		return nil, models.NewInternalError(err, "Failed to find room history")
	}

	return &roomHistory, nil
}

// FindRoomHistoryByRoom finds room history records for a room.
func (r *historyRepository) FindRoomHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.RoomHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.roomHistoryCollection.Find(ctx, bson.M{"roomId": roomID}, opts)
	if err != nil {
		r.logger.Error("Failed to find room history by room", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to find room history")
	}
	defer cursor.Close(ctx)

	var roomHistories []*models.RoomHistory
	if err = cursor.All(ctx, &roomHistories); err != nil {
		r.logger.Error("Failed to decode room history records", err)
		return nil, models.NewInternalError(err, "Failed to decode room history")
	}

	return roomHistories, nil
}

// FindRoomHistoryByType finds room history records by type.
func (r *historyRepository) FindRoomHistoryByType(ctx context.Context, roomID bson.ObjectID, historyType string, skip, limit int) ([]*models.RoomHistory, error) {
	filter := bson.M{
		"roomId": roomID,
		"type":   historyType,
	}

	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.roomHistoryCollection.Find(ctx, filter, opts)
	if err != nil {
		r.logger.Error("Failed to find room history by type", err, "roomId", roomID.Hex(), "type", historyType)
		return nil, models.NewInternalError(err, "Failed to find room history")
	}
	defer cursor.Close(ctx)

	var roomHistories []*models.RoomHistory
	if err = cursor.All(ctx, &roomHistories); err != nil {
		r.logger.Error("Failed to decode room history records", err)
		return nil, models.NewInternalError(err, "Failed to decode room history")
	}

	return roomHistories, nil
}

// CreateDJHistory creates a new DJ history record.
func (r *historyRepository) CreateDJHistory(ctx context.Context, djHistory *models.DJHistory) error {
	if djHistory.ID.IsZero() {
		djHistory.ID = bson.NewObjectID()
	}

	if djHistory.StartTime.IsZero() {
		djHistory.StartTime = time.Now()
	}

	_, err := r.djHistoryCollection.InsertOne(ctx, djHistory)
	if err != nil {
		r.logger.Error("Failed to create DJ history", err, "userId", djHistory.UserID.Hex(), "roomId", djHistory.RoomID.Hex())
		return models.NewInternalError(err, "Failed to create DJ history")
	}

	// Create generic history record
	history := &models.History{
		Type:        "dj",
		ReferenceID: djHistory.UserID,
		Timestamp:   djHistory.StartTime,
		Metadata: map[string]any{
			"djHistoryId": djHistory.ID,
			"roomId":      djHistory.RoomID,
		},
	}

	err = r.CreateHistory(ctx, history)
	if err != nil {
		r.logger.Error("Failed to create generic history for DJ", err)
		// Continue anyway, the DJ history was recorded
	}

	return nil
}

// FindDJHistoryByUser finds DJ history records for a user.
func (r *historyRepository) FindDJHistoryByUser(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.DJHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"startTime": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.djHistoryCollection.Find(ctx, bson.M{"userId": userID}, opts)
	if err != nil {
		r.logger.Error("Failed to find DJ history by user", err, "userId", userID.Hex())
		return nil, models.NewInternalError(err, "Failed to find DJ history")
	}
	defer cursor.Close(ctx)

	var djHistories []*models.DJHistory
	if err = cursor.All(ctx, &djHistories); err != nil {
		r.logger.Error("Failed to decode DJ history records", err)
		return nil, models.NewInternalError(err, "Failed to decode DJ history")
	}

	return djHistories, nil
}

// FindDJHistoryByRoom finds DJ history records for a room.
func (r *historyRepository) FindDJHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.DJHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"startTime": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.djHistoryCollection.Find(ctx, bson.M{"roomId": roomID}, opts)
	if err != nil {
		r.logger.Error("Failed to find DJ history by room", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to find DJ history")
	}
	defer cursor.Close(ctx)

	var djHistories []*models.DJHistory
	if err = cursor.All(ctx, &djHistories); err != nil {
		r.logger.Error("Failed to decode DJ history records", err)
		return nil, models.NewInternalError(err, "Failed to decode DJ history")
	}

	return djHistories, nil
}

// UpdateDJHistoryEndTime updates the end time and reason for a DJ history record.
func (r *historyRepository) UpdateDJHistoryEndTime(ctx context.Context, id bson.ObjectID, endTime time.Time, leaveReason string) error {
	// Get current record to calculate duration
	var djHistory models.DJHistory
	err := r.djHistoryCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&djHistory)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return errors.New("DJ history not found")
		}
		r.logger.Error("Failed to find DJ history for update", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to update DJ history")
	}

	// Calculate duration
	duration := int(endTime.Sub(djHistory.StartTime).Seconds())

	update := bson.D{
		cmdSet(bson.M{
			"endTime":     endTime,
			"duration":    duration,
			"leaveReason": leaveReason,
		}),
	}

	result, err := r.djHistoryCollection.UpdateByID(ctx, id, update)
	if err != nil {
		r.logger.Error("Failed to update DJ history end time", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to update DJ history")
	}

	if result.MatchedCount == 0 {
		return errors.New("DJ history not found")
	}

	return nil
}

// CreateSessionHistory creates a new session history record.
func (r *historyRepository) CreateSessionHistory(ctx context.Context, sessionHistory *models.SessionHistory) error {
	if sessionHistory.ID.IsZero() {
		sessionHistory.ID = bson.NewObjectID()
	}

	if sessionHistory.StartTime.IsZero() {
		sessionHistory.StartTime = time.Now()
	}

	_, err := r.sessionHistoryCollection.InsertOne(ctx, sessionHistory)
	if err != nil {
		r.logger.Error("Failed to create session history", err, "userId", sessionHistory.UserID.Hex())
		return models.NewInternalError(err, "Failed to create session history")
	}

	// Create generic history record
	history := &models.History{
		Type:        "user",
		ReferenceID: sessionHistory.UserID,
		Timestamp:   sessionHistory.StartTime,
		Metadata: map[string]any{
			"sessionHistoryId": sessionHistory.ID,
			"type":             "session",
		},
	}

	err = r.CreateHistory(ctx, history)
	if err != nil {
		r.logger.Error("Failed to create generic history for session", err)
		// Continue anyway, the session history was recorded
	}

	return nil
}

// UpdateSessionHistoryEndTime updates the end time for a session history record.
func (r *historyRepository) UpdateSessionHistoryEndTime(ctx context.Context, id bson.ObjectID, endTime time.Time) error {
	// Get current record to calculate duration
	var sessionHistory models.SessionHistory
	err := r.sessionHistoryCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&sessionHistory)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return errors.New("session history not found")
		}
		r.logger.Error("Failed to find session history for update", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to update session history")
	}

	// Calculate duration
	duration := int(endTime.Sub(sessionHistory.StartTime).Seconds())

	update := bson.D{
		cmdSet(bson.M{
			"endTime":  endTime,
			"duration": duration,
		}),
	}

	result, err := r.sessionHistoryCollection.UpdateByID(ctx, id, update)
	if err != nil {
		r.logger.Error("Failed to update session history end time", err, "id", id.Hex())
		return models.NewInternalError(err, "Failed to update session history")
	}

	if result.MatchedCount == 0 {
		return errors.New("session history not found")
	}

	return nil
}

// FindSessionHistoryByUser finds session history records for a user.
func (r *historyRepository) FindSessionHistoryByUser(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.SessionHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"startTime": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.sessionHistoryCollection.Find(ctx, bson.M{"userId": userID}, opts)
	if err != nil {
		r.logger.Error("Failed to find session history by user", err, "userId", userID.Hex())
		return nil, models.NewInternalError(err, "Failed to find session history")
	}
	defer cursor.Close(ctx)

	var sessionHistories []*models.SessionHistory
	if err = cursor.All(ctx, &sessionHistories); err != nil {
		r.logger.Error("Failed to decode session history records", err)
		return nil, models.NewInternalError(err, "Failed to decode session history")
	}

	return sessionHistories, nil
}

// CreateModerationHistory creates a new moderation history record.
func (r *historyRepository) CreateModerationHistory(ctx context.Context, moderationHistory *models.ModerationHistory) error {
	if moderationHistory.ID.IsZero() {
		moderationHistory.ID = bson.NewObjectID()
	}

	if moderationHistory.Timestamp.IsZero() {
		moderationHistory.Timestamp = time.Now()
	}

	_, err := r.moderationHistoryCollection.InsertOne(ctx, moderationHistory)
	if err != nil {
		r.logger.Error("Failed to create moderation history", err, "moderatorId", moderationHistory.ModeratorID.Hex(), "targetUserId", moderationHistory.TargetUserID.Hex())
		return models.NewInternalError(err, "Failed to create moderation history")
	}

	// Create generic history record
	history := &models.History{
		Type:        "moderation",
		ReferenceID: moderationHistory.RoomID,
		Timestamp:   moderationHistory.Timestamp,
		Metadata: map[string]any{
			"moderationHistoryId": moderationHistory.ID,
			"action":              moderationHistory.Action,
			"moderatorId":         moderationHistory.ModeratorID,
			"targetUserId":        moderationHistory.TargetUserID,
		},
	}

	err = r.CreateHistory(ctx, history)
	if err != nil {
		r.logger.Error("Failed to create generic history for moderation", err)
		// Continue anyway, the moderation history was recorded
	}

	return nil
}

// FindModerationHistoryByRoom finds moderation history records for a room.
func (r *historyRepository) FindModerationHistoryByRoom(ctx context.Context, roomID bson.ObjectID, skip, limit int) ([]*models.ModerationHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.moderationHistoryCollection.Find(ctx, bson.M{"roomId": roomID}, opts)
	if err != nil {
		r.logger.Error("Failed to find moderation history by room", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to find moderation history")
	}
	defer cursor.Close(ctx)

	var moderationHistories []*models.ModerationHistory
	if err = cursor.All(ctx, &moderationHistories); err != nil {
		r.logger.Error("Failed to decode moderation history records", err)
		return nil, models.NewInternalError(err, "Failed to decode moderation history")
	}

	return moderationHistories, nil
}

// FindModerationHistoryByModerator finds moderation history records for a moderator.
func (r *historyRepository) FindModerationHistoryByModerator(ctx context.Context, moderatorID bson.ObjectID, skip, limit int) ([]*models.ModerationHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.moderationHistoryCollection.Find(ctx, bson.M{"moderatorId": moderatorID}, opts)
	if err != nil {
		r.logger.Error("Failed to find moderation history by moderator", err, "moderatorId", moderatorID.Hex())
		return nil, models.NewInternalError(err, "Failed to find moderation history")
	}
	defer cursor.Close(ctx)

	var moderationHistories []*models.ModerationHistory
	if err = cursor.All(ctx, &moderationHistories); err != nil {
		r.logger.Error("Failed to decode moderation history records", err)
		return nil, models.NewInternalError(err, "Failed to decode moderation history")
	}

	return moderationHistories, nil
}

// FindModerationHistoryByUser finds moderation history records for a target user.
func (r *historyRepository) FindModerationHistoryByUser(ctx context.Context, userID bson.ObjectID, skip, limit int) ([]*models.ModerationHistory, error) {
	opts := options.Find().
		SetSort(bson.M{"timestamp": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := r.moderationHistoryCollection.Find(ctx, bson.M{"targetUserId": userID}, opts)
	if err != nil {
		r.logger.Error("Failed to find moderation history by user", err, "userId", userID.Hex())
		return nil, models.NewInternalError(err, "Failed to find moderation history")
	}
	defer cursor.Close(ctx)

	var moderationHistories []*models.ModerationHistory
	if err = cursor.All(ctx, &moderationHistories); err != nil {
		r.logger.Error("Failed to decode moderation history records", err)
		return nil, models.NewInternalError(err, "Failed to decode moderation history")
	}

	return moderationHistories, nil
}

// GetTopTracks gets the most played tracks in a room.
func (r *historyRepository) GetTopTracks(ctx context.Context, roomID bson.ObjectID, limit int) ([]models.TopTrackSummary, error) {
	pipeline := mongo.Pipeline{
		{cmdMatch(bson.M{"roomId": roomID})},
		{cmdGroup(bson.M{
			"_id":         "$mediaId",
			"playCount":   bson.M{"$sum": 1},
			"wootCount":   bson.M{"$sum": "$votes.woots"},
			"mehCount":    bson.M{"$sum": "$votes.mehs"},
			"grabCount":   bson.M{"$sum": "$votes.grabs"},
			"skipCount":   bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []any{"$skipped", true}}, 1, 0}}},
			"audienceSum": bson.M{"$sum": "$userCount"},
			"title":       bson.M{"$first": "$media.title"},
			"artist":      bson.M{"$first": "$media.artist"},
			"type":        bson.M{"$first": "$media.type"},
			"sourceId":    bson.M{"$first": "$media.sourceId"},
			"lastPlayed":  bson.M{"$max": "$startTime"},
		})},
		{cmdProject(bson.M{
			"_id":             0,
			"mediaId":         "$_id",
			"title":           1,
			"artist":          1,
			"type":            1,
			"sourceId":        1,
			"playCount":       1,
			"wootCount":       1,
			"mehCount":        1,
			"grabCount":       1,
			"skipCount":       1,
			"averageAudience": bson.M{"$divide": []any{"$audienceSum", "$playCount"}},
			"lastPlayed":      1,
		})},
		{cmdSort(bson.M{"playCount": -1})},
		{cmdLimit(limit)},
	}

	cursor, err := r.playHistoryCollection.Aggregate(ctx, pipeline)
	if err != nil {
		r.logger.Error("Failed to get top tracks", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to calculate top tracks")
	}
	defer cursor.Close(ctx)

	var topTracks []models.TopTrackSummary
	if err = cursor.All(ctx, &topTracks); err != nil {
		r.logger.Error("Failed to decode top tracks", err)
		return nil, models.NewInternalError(err, "Failed to decode top tracks")
	}

	return topTracks, nil
}

// GetTopDJs gets the most active DJs in a room.
func (r *historyRepository) GetTopDJs(ctx context.Context, roomID bson.ObjectID, limit int) ([]models.TopDJSummary, error) {
	// First, get play counts by DJ
	playsPipeline := mongo.Pipeline{
		{cmdMatch(bson.M{"roomId": roomID})},
		{cmdGroup(bson.M{
			"_id":          "$djId",
			"playCount":    bson.M{"$sum": 1},
			"wootCount":    bson.M{"$sum": "$votes.woots"},
			"mehCount":     bson.M{"$sum": "$votes.mehs"},
			"grabCount":    bson.M{"$sum": "$votes.grabs"},
			"skipCount":    bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []any{"$skipped", true}}, 1, 0}}},
			"username":     bson.M{"$first": "$dj.username"},
			"lastPlayTime": bson.M{"$max": "$startTime"},
		})},
	}

	playCursor, err := r.playHistoryCollection.Aggregate(ctx, playsPipeline)
	if err != nil {
		r.logger.Error("Failed to get DJ play counts", err, "roomId", roomID.Hex())
		return nil, models.NewInternalError(err, "Failed to calculate top DJs")
	}
	defer playCursor.Close(ctx)

	var playStats []struct {
		ID           bson.ObjectID `bson:"_id"`
		PlayCount    int           `bson:"playCount"`
		WootCount    int           `bson:"wootCount"`
		MehCount     int           `bson:"mehCount"`
		GrabCount    int           `bson:"grabCount"`
		SkipCount    int           `bson:"skipCount"`
		Username     string        `bson:"username"`
		LastPlayTime time.Time     `bson:"lastPlayTime"`
	}

	if err = playCursor.All(ctx, &playStats); err != nil {
		r.logger.Error("Failed to decode DJ play stats", err)
		return nil, models.NewInternalError(err, "Failed to decode top DJs")
	}

	// Then, get DJ time
	djTimePipeline := mongo.Pipeline{
		{cmdMatch(bson.M{"roomId": roomID})},
		{cmdGroup(bson.M{
			"_id":         "$userId",
			"totalDJTime": bson.M{"$sum": "$duration"},
			"lastDJTime":  bson.M{"$max": "$endTime"},
		})},
	}

	djTimeCursor, err := r.djHistoryCollection.Aggregate(ctx, djTimePipeline)
	if err != nil {
		r.logger.Error("Failed to get DJ times", err, "roomId", roomID.Hex())
		// Continue with play stats only
	}

	djTimes := make(map[string]struct {
		TotalDJTime int64
		LastDJTime  time.Time
	})

	if djTimeCursor != nil {
		defer djTimeCursor.Close(ctx)

		var timeStats []struct {
			ID          bson.ObjectID `bson:"_id"`
			TotalDJTime int64         `bson:"totalDJTime"`
			LastDJTime  time.Time     `bson:"lastDJTime"`
		}

		if err = djTimeCursor.All(ctx, &timeStats); err != nil {
			r.logger.Error("Failed to decode DJ time stats", err)
			// Continue with play stats only
		} else {
			// Convert to map for easy lookup
			for _, stat := range timeStats {
				djTimes[stat.ID.Hex()] = struct {
					TotalDJTime int64
					LastDJTime  time.Time
				}{
					TotalDJTime: stat.TotalDJTime,
					LastDJTime:  stat.LastDJTime,
				}
			}
		}
	}

	// Combine play stats and DJ times
	topDJs := make([]models.TopDJSummary, 0, len(playStats))
	for _, stat := range playStats {
		djSummary := models.TopDJSummary{
			UserID:     stat.ID,
			Username:   stat.Username,
			PlayCount:  stat.PlayCount,
			WootCount:  stat.WootCount,
			MehCount:   stat.MehCount,
			GrabCount:  stat.GrabCount,
			SkipCount:  stat.SkipCount,
			LastDJTime: stat.LastPlayTime,
		}

		// Add DJ time if available
		if timeData, ok := djTimes[stat.ID.Hex()]; ok {
			djSummary.TotalDJTime = timeData.TotalDJTime
			if timeData.LastDJTime.After(djSummary.LastDJTime) {
				djSummary.LastDJTime = timeData.LastDJTime
			}
		}

		topDJs = append(topDJs, djSummary)
	}

	// Sort by play count and limit results
	// (We could have done this in the aggregation, but we need to combine data from two collections)
	utils.SortSlice(topDJs, func(i, j int) bool {
		return topDJs[i].PlayCount > topDJs[j].PlayCount
	})

	if len(topDJs) > limit {
		topDJs = topDJs[:limit]
	}

	return topDJs, nil
}
