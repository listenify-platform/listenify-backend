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

func GetIDFromQuery(r *http.Request, param string) bson.ObjectID {
	id := r.URL.Query().Get(param)
	if id == "" {
		return bson.NilObjectID
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.NilObjectID
	}
	return oid
}

func GetQueryInt(r *http.Request, param string, def int) int {
	value := r.URL.Query().Get(param)
	if value == "" {
		return def
	}

	num, err := strconv.Atoi(value)
	if err != nil {
		return def
	}

	num = min(def, max(0, num))
	return num
}

func GetLimit(r *http.Request, def int) int {
	return GetQueryInt(r, "limit", def)
}

func GetPage(r *http.Request, def int) int {
	return GetQueryInt(r, "page", def)
}
