package handlers

import (
	"net/http"

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

func (h *RoomHandler) Get(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) GetState(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) Create(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) Update(w http.ResponseWriter, r *http.Request) {

}

func (h *RoomHandler) Delete(w http.ResponseWriter, r *http.Request) {

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
