package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/models"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/utils"
)

type RoomHandler struct {
	mgr    *room.Manager
	logger *utils.Logger
}

func NewRoomHandler(mgr *room.Manager, logger *utils.Logger) *RoomHandler {
	return &RoomHandler{
		mgr:    mgr,
		logger: logger,
	}
}

func (h *RoomHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := GetLimit(r, 20)
	// page := GetPage(r, 1)

	// Get only active rooms
	rooms, err := h.mgr.GetActiveRooms(r.Context(), limit)
	if err != nil {
		h.logger.Error("Failed to get active rooms", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if len(rooms) == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "No active rooms found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, rooms)
}

func (h *RoomHandler) ListPopular(w http.ResponseWriter, r *http.Request) {
	limit := GetLimit(r, 20)

	rooms, err := h.mgr.GetPopularRooms(r.Context(), limit)
	if err != nil {
		h.logger.Error("Failed to get popular rooms", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if len(rooms) == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "No popular rooms found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, rooms)
}

func (h *RoomHandler) ListFavorites(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) Search(w http.ResponseWriter, r *http.Request) {
	limit := GetLimit(r, 20)

	// Get query, skip, and sort parameters from the request
	query := r.URL.Query().Get("query")
	sort := r.URL.Query().Get("sort")
	skip, err := strconv.Atoi(r.URL.Query().Get("skip"))
	if err != nil {
		skip = 0
	}
	skip = max(0, skip)

	rooms, total, err := h.mgr.SearchRooms(r.Context(), models.RoomSearchCriteria{
		Query:  query,
		SortBy: sort,
		Limit:  limit,
		Page:   skip,
	})
	if err != nil {
		h.logger.Error("Failed to search rooms", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	if len(rooms) == 0 {
		utils.RespondWithError(w, http.StatusNotFound, "No rooms found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"rooms": rooms,
		"total": total,
	})
}

func (h *RoomHandler) Get(w http.ResponseWriter, r *http.Request, roomID bson.ObjectID) {
	room, err := h.mgr.GetRoom(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else {
			h.logger.Error("Failed to get room", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	if room == nil {
		utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, room)
}

func (h *RoomHandler) GetBySlug(w http.ResponseWriter, r *http.Request, slug string) {
	room, err := h.mgr.GetRoomBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else {
			h.logger.Error("Failed to get room by slug", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	if room == nil {
		utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, room)
}

func (h *RoomHandler) GetState(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {
	state, err := h.mgr.GetRoomState(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else {
			h.logger.Error("Failed to get room state", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	if state == nil {
		utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		return
	}
	utils.RespondWithJSON(w, http.StatusOK, state)
}

func (h *RoomHandler) HasUser(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {
	// The User ID can be either from the context or from the query
	userID := GetIDFromQuery(r, "userId")
	if userID.IsZero() {
		userID = GetUserIDFromContext(w, r)
		if userID.IsZero() {
			return
		}
	}

	inRoom, err := h.mgr.IsUserInRoom(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else {
			h.logger.Error("Failed to check if user is in room", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	utils.RespondWithJSON(w, http.StatusOK, map[string]any{
		"inRoom": inRoom,
	})
}

type RoomCreateRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Slug        string              `json:"slug"`
	Settings    models.RoomSettings `json:"settings"`
}

func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request, data *RoomCreateRequest) {
	// Validate the request
	if data.Name == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if data.Slug == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Slug is required")
		return
	}
	userID := GetUserIDFromContext(w, r)
	if userID.IsZero() {
		return
	}

	room := &models.Room{
		Name:        data.Name,
		Description: data.Description,
		Slug:        data.Slug,
		CreatedBy:   userID,
		Settings:    data.Settings,
		Moderators:  []bson.ObjectID{userID},
		BannedUsers: []bson.ObjectID{},
		IsActive:    true,
	}

	createdRoom, err := h.mgr.CreateRoom(r.Context(), room)
	if err != nil {
		h.logger.Error("Failed to create room", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	err = h.mgr.JoinRoom(r.Context(), createdRoom.ID, userID)
	if err != nil {
		h.logger.Error("Failed to join room after creation", err, "roomId", createdRoom.ID.Hex(), "userID", userID)
		// Continue anyway, the room was created successfully
	}

	utils.RespondWithJSON(w, http.StatusCreated, createdRoom)
}

type RoomUpdateRequest struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Slug        string              `json:"slug"`
	Settings    models.RoomSettings `json:"settings"`
}

func (h *RoomHandler) Update(w http.ResponseWriter, r *http.Request, id bson.ObjectID, data *RoomUpdateRequest) {
	// Validate the request
	if data.Name == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Name is required")
		return
	}
	if data.Slug == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "Slug is required")
		return
	}
	userID := GetUserIDFromContext(w, r)
	if userID.IsZero() {
		return
	}

	room, err := h.mgr.GetRoom(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else {
			h.logger.Error("Failed to get room", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Check if user is creator or moderator
	if room.CreatedBy != userID && !slices.Contains(room.Moderators, userID) {
		utils.RespondWithError(w, http.StatusForbidden, "You are not allowed to update this room")
		return
	}

	// Update the room
	room.Name = data.Name
	room.Description = data.Description
	room.Slug = data.Slug
	room.Settings = data.Settings

	updatedRoom, err := h.mgr.UpdateRoom(r.Context(), room)
	if err != nil {
		h.logger.Error("Failed to update room", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Respond with the updated room
	utils.RespondWithJSON(w, http.StatusOK, updatedRoom)
}

func (h *RoomHandler) Delete(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {
	userID := GetUserIDFromContext(w, r)
	if userID.IsZero() {
		return
	}

	room, err := h.mgr.GetRoom(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else {
			h.logger.Error("Failed to get room", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Check if user is creator
	if room.CreatedBy != userID {
		utils.RespondWithError(w, http.StatusForbidden, "You are not allowed to delete this room")
		return
	}

	// Delete the room
	err = h.mgr.DeleteRoom(r.Context(), id)
	if err != nil {
		h.logger.Error("Failed to delete room", err)
		utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusNoContent)
}

func (h *RoomHandler) PostJoin(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {
	userID := GetUserIDFromContext(w, r)
	if userID.IsZero() {
		return
	}

	err := h.mgr.JoinRoom(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else if errors.Is(err, models.ErrRoomFull) {
			utils.RespondWithError(w, http.StatusConflict, "Room is full")
		} else if errors.Is(err, models.ErrUserBanned) {
			utils.RespondWithError(w, http.StatusForbidden, "You are banned from this room")
		} else if errors.Is(err, models.ErrUserAlreadyInRoom) {
			utils.RespondWithError(w, http.StatusConflict, "You are already in this room")
		} else {
			h.logger.Error("Failed to join room", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusNoContent)
}

func (h *RoomHandler) PostLeave(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {
	userID := GetUserIDFromContext(w, r)
	if userID.IsZero() {
		return
	}

	err := h.mgr.LeaveRoom(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, models.ErrRoomNotFound) {
			utils.RespondWithError(w, http.StatusNotFound, "Room not found")
		} else if errors.Is(err, models.ErrUserNotInRoom) {
			utils.RespondWithError(w, http.StatusConflict, "You are not in this room")
		} else {
			h.logger.Error("Failed to leave room", err)
			utils.RespondWithError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusNoContent)
}

func (h *RoomHandler) PostSkip(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {

}

func (h *RoomHandler) PostVote(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {

}

func (h *RoomHandler) PostQueueJoin(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {

}

func (h *RoomHandler) PostQueueLeave(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {

}

func (h *RoomHandler) PostFavorite(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {

}

func (h *RoomHandler) DeleteFavorite(w http.ResponseWriter, r *http.Request, id bson.ObjectID) {

}
