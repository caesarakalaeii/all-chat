// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package models

import (
	"errors"
	"time"
)

// Overlay represents an overlay configuration
type Overlay struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	IsActive            bool      `json:"is_active"`
	IsPublicForViewers  bool      `json:"is_public_for_viewers"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
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
