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

package oauth

import (
	"encoding/json"
	"fmt"
)

// OAuthAction represents the action to take after OAuth callback
type OAuthAction string

const (
	// ActionLogin is for user login/authentication
	ActionLogin OAuthAction = "login"
	// ActionAddSource is for adding a chat source to an overlay
	ActionAddSource OAuthAction = "add_source"
)

// PurposeModeration marks an add-source flow that is really an opt-in moderation
// re-consent (ADR-0017). It reuses the add-source action so the callback runs the
// same token/scope persistence, but the marker lets the callback return the user to
// the overlay monitor (/overlay/{id}/view) instead of the overlay settings page.
const PurposeModeration = "moderation"

// OAuthState represents the state parameter for OAuth flow
type OAuthState struct {
	CSRFToken string      `json:"csrf_token"` // Random string for CSRF protection
	OverlayID string      `json:"overlay_id,omitempty"` // Target overlay for source addition
	UserID    string      `json:"user_id,omitempty"` // Current user ID for account linking
	Action    OAuthAction `json:"action"` // Action to take after callback
	Purpose   string      `json:"purpose,omitempty"` // Sub-flow marker (e.g. moderation re-consent)
}

// NewLoginState creates a new state for login flow
func NewLoginState(csrfToken string) *OAuthState {
	return &OAuthState{
		CSRFToken: csrfToken,
		Action:    ActionLogin,
	}
}

// NewAddSourceState creates a new state for adding a source to an overlay
func NewAddSourceState(csrfToken, overlayID, userID string) *OAuthState {
	return &OAuthState{
		CSRFToken: csrfToken,
		OverlayID: overlayID,
		UserID:    userID,
		Action:    ActionAddSource,
	}
}

// NewModerationState creates a state for the opt-in moderation re-consent flow
// (ADR-0017). It is an add-source state — so the shared callback persists the new
// token/scopes and overlay link unchanged — tagged with PurposeModeration so the
// callback returns the user to the overlay monitor rather than overlay settings.
func NewModerationState(csrfToken, overlayID, userID string) *OAuthState {
	s := NewAddSourceState(csrfToken, overlayID, userID)
	s.Purpose = PurposeModeration
	return s
}

// IsModeration reports whether this state is an opt-in moderation re-consent flow.
func (s *OAuthState) IsModeration() bool {
	return s.Purpose == PurposeModeration
}

// Encode serializes the state to JSON string
func (s *OAuthState) Encode() (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to marshal state: %w", err)
	}
	return string(data), nil
}

// DecodeOAuthState deserializes a JSON string to OAuthState
func DecodeOAuthState(encoded string) (*OAuthState, error) {
	var state OAuthState
	if err := json.Unmarshal([]byte(encoded), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}
	return &state, nil
}

// IsAddSource checks if the action is to add a source
func (s *OAuthState) IsAddSource() bool {
	return s.Action == ActionAddSource
}

// IsLogin checks if the action is for login
func (s *OAuthState) IsLogin() bool {
	return s.Action == ActionLogin
}

// Validate checks if the state is valid
func (s *OAuthState) Validate() error {
	if s.CSRFToken == "" {
		return fmt.Errorf("csrf_token is required")
	}
	if s.Action != ActionLogin && s.Action != ActionAddSource {
		return fmt.Errorf("invalid action: %s", s.Action)
	}
	if s.Action == ActionAddSource && s.OverlayID == "" {
		return fmt.Errorf("overlay_id is required for add_source action")
	}
	return nil
}
