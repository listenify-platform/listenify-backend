package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"norelock.dev/listenify/backend/internal/utils"
)

// CRUD is an interface that defines the basic CRUD operations for a resource.
// Each method corresponds to an HTTP method: List (GET), Get (GET by ID), Create (POST), Update (PUT), and Delete (DELETE).
type CRUD[C any, U any] interface {
	List(w http.ResponseWriter, r *http.Request)
	Get(w http.ResponseWriter, r *http.Request, id bson.ObjectID)
	Create(w http.ResponseWriter, r *http.Request, data *C)
	Update(w http.ResponseWriter, r *http.Request, id bson.ObjectID, data *U)
	Delete(w http.ResponseWriter, r *http.Request, id bson.ObjectID)
}

// AddCRUDRoutes adds the standard CRUD routes to the provided chi.Router.
// It maps the HTTP methods to the corresponding handler methods defined in the CRUD interface.
func AddCRUDRoutes[C any, U any](r chi.Router, handler CRUD[C, U]) {
	r.Get("/", handler.List)
	r.Get("/{id}", WithID(handler.Get))
	r.Post("/", WithBody(handler.Create))
	r.Put("/{id}", WithIDAndBody(handler.Update))
	r.Delete("/{id}", WithID(handler.Delete))
}

type HandlerFunc1[T any] func(w http.ResponseWriter, r *http.Request, data T)
type HandlerFunc2[T1 any, T2 any] func(w http.ResponseWriter, r *http.Request, data1 T1, data2 T2)

func idFromParam(w http.ResponseWriter, r *http.Request) bson.ObjectID {
	id := chi.URLParam(r, "id")
	if id == "" {
		utils.RespondWithError(w, http.StatusBadRequest, "ID is required")
		return bson.NilObjectID
	}
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		utils.RespondWithError(w, http.StatusBadRequest, "Invalid ID format")
		return bson.NilObjectID
	}
	return oid
}

func WithID(handler HandlerFunc1[bson.ObjectID]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := idFromParam(w, r)
		if id.IsZero() {
			return
		}
		handler(w, r, id)
	}
}

func WithBody[T any](handler HandlerFunc1[*T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data T
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		handler(w, r, &data)
	}
}

func WithIDAndBody[T any](handler HandlerFunc2[bson.ObjectID, *T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := idFromParam(w, r)
		if id.IsZero() {
			return
		}
		var data T
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			utils.RespondWithError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		handler(w, r, id, &data)
	}
}
