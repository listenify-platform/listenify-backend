package handlers

import (
	"net/http"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/utils"
)

func GetUserIDFromContext(w http.ResponseWriter, r *http.Request) bson.ObjectID {
	userID := r.Context().Value("userID")
	if userID == nil {
		utils.RespondWithError(w, http.StatusUnauthorized, "User ID not found")
		return bson.NilObjectID
	}
	oid, ok := userID.(bson.ObjectID)
	if !ok || oid.IsZero() {
		utils.RespondWithError(w, http.StatusUnauthorized, "Invalid User ID format")
		return bson.NilObjectID
	}
	return oid
}

func GetLimit(r *http.Request, def int) int {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		return def
	}

	num_limit, err := strconv.Atoi(limit)
	if err != nil {
		return def
	}

	num_limit = min(def, max(0, num_limit))
	return num_limit
}
