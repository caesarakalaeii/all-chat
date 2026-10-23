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
	// ActionModConsent is a delegated moderator granting All-Chat their OWN moderation
	// scopes (ADR-0048).
	//
	// It is deliberately NOT an add-source action. The shared callback calls
	// addSourceToOverlay for every add-source state, and overlay-manager 404s that for a
	// non-owner — so a moderator would have their credential persisted and then be
	// redirected to an error. It also must not touch the moderator's login credential or
	// granted_scopes: the credential lands in its own table, keyed on the moderator.
	ActionModConsent OAuthAction = "mod_consent"
	// ActionBotConnect is a streamer or admin granting All-Chat the scopes of a BOT
	// chat account (ADR-0081): the All-Chat Bot (admin) or the streamer's custom bot
	// (premium). Like mod-consent it must never touch login machinery — the bot row
	// lands in chat_bot_accounts and the callback redirects straight back. The
	// Purpose field carries the scope flavor: "custom" (owner's own bot) or
	// "allchat" (the global bot, admin only).
	ActionBotConnect OAuthAction = "bot_connect"
)

// PurposeBotConnectCustom / PurposeBotConnectAllChat mark which flavor of bot
// consent a bot-connect state carries: the streamer's custom bot (premium gate,
// returns to /commands) or the global All-Chat Bot (admin only, returns to
// /admin/chat-bots).
const (
	PurposeBotConnectCustom  = "custom"
	PurposeBotConnectAllChat = "allchat"
)

// PurposeModeration marks an add-source flow that is really an opt-in moderation
// re-consent (ADR-0017). It reuses the add-source action so the callback runs the
// same token/scope persistence, but the marker lets the callback return the user to
// the overlay monitor (/overlay/{id}/view) instead of the overlay settings page.
const PurposeModeration = "moderation"

// ChatCommandsReturnPath is the frontend page the chat commands editor's clip-permission
// re-consent returns to (ADR-0079). The other ReturnPath an OAuthState may carry.
const ChatCommandsReturnPath = "/commands"

// ChatBotsAdminReturnPath is the frontend page the All-Chat Bot consent returns to
// (ADR-0081). The bot-connect callback appends ?bot=connected|error.
const ChatBotsAdminReturnPath = "/admin/chat-bots"

// OAuthState represents the state parameter for OAuth flow
type OAuthState struct {
	CSRFToken string      `json:"csrf_token"`           // Random string for CSRF protection
	OverlayID string      `json:"overlay_id,omitempty"` // Target overlay for source addition
	UserID    string      `json:"user_id,omitempty"`    // Current user ID for account linking
	Action    OAuthAction `json:"action"`               // Action to take after callback
	Purpose   string      `json:"purpose,omitempty"`    // Sub-flow marker (e.g. moderation re-consent)
	// ReturnPath, when set, is where a moderation re-consent returns instead of the
	// overlay monitor. Validate accepts only the two known frontend paths.
	ReturnPath string `json:"return_path,omitempty"`
	// Origin is the allowlisted frontend origin (e.g. https://beta.allch.at) the
	// flow was started from. It selects the provider redirect_uri at authorize
	// and exchange time and the frontend the callback redirects back to. Set
	// server-side only; the callback byte-compares the query state against the
	// Redis copy, so a tampered origin never survives validation. Empty means
	// the canonical FRONTEND_URL (states written before this field existed).
	Origin string `json:"origin,omitempty"`
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

// NewModConsentState creates a state for a delegated moderator granting their own moderation
// scopes (ADR-0048).
//
// No overlay id: because Twitch's and Kick's moderation scopes are role-based rather than
// channel-scoped, one consent per platform serves every streamer who delegated that platform.
// Binding it to an overlay would imply a per-streamer consent that the platforms do not model.
func NewModConsentState(csrfToken, userID string) *OAuthState {
	return &OAuthState{
		CSRFToken: csrfToken,
		UserID:    userID,
		Action:    ActionModConsent,
	}
}

// IsModConsent reports whether this state is a delegated moderator's own consent flow.
func (s *OAuthState) IsModConsent() bool {
	return s.Action == ActionModConsent
}

// NewBotConnectState creates a state for granting all-chat the scopes of a BOT chat
// account (ADR-0081). purpose is PurposeBotConnectCustom (the streamer's own bot,
// gated premium) or PurposeBotConnectAllChat (the global All-Chat Bot, admin only).
// No overlay id for the same reason as mod-consent: the grant sits on the bot
// account, not on any overlay, and the callback must never reach addSourceToOverlay.
func NewBotConnectState(csrfToken, userID, purpose, returnPath string) *OAuthState {
	return &OAuthState{
		CSRFToken:  csrfToken,
		UserID:     userID,
		Action:     ActionBotConnect,
		Purpose:    purpose,
		ReturnPath: returnPath,
	}
}

// IsBotConnect reports whether this state is a bot-account consent flow (ADR-0081).
func (s *OAuthState) IsBotConnect() bool {
	return s.Action == ActionBotConnect
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
	if s.Action != ActionLogin && s.Action != ActionAddSource && s.Action != ActionModConsent && s.Action != ActionBotConnect {
		return fmt.Errorf("invalid action: %s", s.Action)
	}
	if s.Action == ActionAddSource && s.OverlayID == "" {
		return fmt.Errorf("overlay_id is required for add_source action")
	}
	if s.ReturnPath != "" && s.ReturnPath != ChatCommandsReturnPath && s.ReturnPath != ChatBotsAdminReturnPath {
		return fmt.Errorf("invalid return_path: %s", s.ReturnPath)
	}
	// A moderator's credential is keyed on them, so without a user id there is nobody to
	// attribute it to — and the callback must never fall back to creating an account here.
	if s.Action == ActionModConsent && s.UserID == "" {
		return fmt.Errorf("user_id is required for mod_consent action")
	}
	// Same for a bot consent: the row it lands in is attributed to this user (NULL owner
	// for the All-Chat Bot is derived by the callback from the purpose; a client can
	// never hand the state a user id it should not own).
	if s.Action == ActionBotConnect && s.UserID == "" {
		return fmt.Errorf("user_id is required for bot_connect action")
	}
	if s.Action == ActionBotConnect && s.Purpose != PurposeBotConnectCustom && s.Purpose != PurposeBotConnectAllChat {
		return fmt.Errorf("invalid purpose %q for bot_connect action", s.Purpose)
	}
	return nil
}
