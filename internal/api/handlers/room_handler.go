package handlers

import (
	"errors"
	"net/http"
	"slices"

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

}

func (h *RoomHandler) ListPopular(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) ListFavorites(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) Search(w http.ResponseWriter, r *http.Request) {

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

func (h *RoomHandler) GetState(w http.ResponseWriter, r *http.Request) {

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

func (h *RoomHandler) PostJoin(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) PostLeave(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) PostSkip(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) PostVote(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) PostQueueJoin(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) PostQueueLeave(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) PostFavorite(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) DeleteFavorite(w http.ResponseWriter, r *http.Request) {

}
