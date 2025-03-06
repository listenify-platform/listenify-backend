package handlers

import (
	"net/http"

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
