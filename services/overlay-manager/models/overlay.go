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
	"fmt"
	"time"
)

// Overlay kind (ADR-0064). Chat is the only kind with a renderer today; the
// other three are created and routed but render a placeholder until their own
// issues land.
const (
	OverlayTypeChat   = "chat"
	OverlayTypeAlerts = "alerts"
	OverlayTypeGoal   = "goal"
	OverlayTypeList   = "list"
)

// supportedOverlayTypes is the exact set the migrations' CHECK constraint
// allows. Wire values only: 'Chat' must not quietly become a chat overlay.
var supportedOverlayTypes = map[string]bool{
	OverlayTypeChat:   true,
	OverlayTypeAlerts: true,
	OverlayTypeGoal:   true,
	OverlayTypeList:   true,
}

// Overlay represents an overlay configuration
type Overlay struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"user_id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	IsActive           bool      `json:"is_active"`
	IsPublicForViewers bool      `json:"is_public_for_viewers"`
	// OverlayType is one of the four kinds above. Overlays created before the
	// column existed return it as the SQL default; an in-memory zero value is
	// resolved to chat by Validate.
	OverlayType string    `json:"overlay_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Kind returns the overlay's kind, resolving an empty value to chat. Read
// paths use this rather than the raw field: the DB column is NOT NULL with a
// default, but an in-memory Overlay that skipped Validate (internal callers,
// mocks) would otherwise leak "" to JSON consumers and gates that route on
// the kind.
func (o *Overlay) Kind() string {
	if o.OverlayType == "" {
		return OverlayTypeChat
	}
	return o.OverlayType
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

	// Absent resolves to chat: every overlay that predates the column must keep
	// behaving exactly as before, and callers that build an Overlay without a
	// kind (clone, delete-promotion) must not have to know about the types.
	if o.OverlayType == "" {
		o.OverlayType = OverlayTypeChat
	}

	if !supportedOverlayTypes[o.OverlayType] {
		return fmt.Errorf("overlay_type must be one of chat, alerts, goal, list (got %q)", o.OverlayType)
	}

	return nil
}
