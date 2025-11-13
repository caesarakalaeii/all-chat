package models

import (
	"errors"
	"time"
)

// Overlay represents an overlay configuration
type Overlay struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate validates the overlay fields
func (o *Overlay) Validate() error {
	if o.UserID == "" {
		return errors.New("user_id is required")
	}

	if o.Name == "" {
		return errors.New("name is required")
	}

	if len(o.Name) > 100 {
		return errors.New("name must be 100 characters or less")
	}

	if len(o.Description) > 500 {
		return errors.New("description must be 500 characters or less")
	}

	return nil
}
