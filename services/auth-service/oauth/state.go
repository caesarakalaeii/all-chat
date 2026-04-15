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

// OAuthState represents the state parameter for OAuth flow
type OAuthState struct {
	CSRFToken string      `json:"csrf_token"` // Random string for CSRF protection
	OverlayID string      `json:"overlay_id,omitempty"` // Target overlay for source addition
	UserID    string      `json:"user_id,omitempty"` // Current user ID for account linking
	Action    OAuthAction `json:"action"` // Action to take after callback
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
