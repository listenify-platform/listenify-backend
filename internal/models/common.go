// Package models contains the data structures used throughout the application.
package models

import (
	"time"
)

// ObjectTimes contains timestamps for created and updated objects.
// It should be embedded in other structs.
type ObjectTimes struct {
	// CreatedAt is the timestamp when the object was created.
	CreatedAt time.Time `json:"createdAt" bson:"createdAt"`

	// UpdatedAt is the timestamp when the object was last updated.
	UpdatedAt time.Time `json:"updatedAt" bson:"updatedAt"`
}

// NewObjectTimes creates a new ObjectTimes instance with the given time.
// The created and updated timestamps are set to the same value.
func NewObjectTimes(t time.Time) ObjectTimes {
	return ObjectTimes{
		CreatedAt: t,
		UpdatedAt: t,
	}
}

// TimeCreate sets the created and updated timestamps to the given time.
func (o *ObjectTimes) TimeCreate(t time.Time) {
	o.CreatedAt = t
	o.UpdatedAt = t
}

// TimeUpdate sets the updated timestamp to the given time.
func (o *ObjectTimes) TimeUpdate(t time.Time) {
	o.UpdatedAt = t
}

// CreateNow sets the created and updated timestamps to the current time.
func (o *ObjectTimes) CreateNow() {
	o.TimeCreate(time.Now())
}

// UpdateNow sets the updated timestamp to the current time.
func (o *ObjectTimes) UpdateNow() {
	o.TimeUpdate(time.Now())
}
