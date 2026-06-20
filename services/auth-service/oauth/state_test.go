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
	"testing"
)

func TestNewLoginState(t *testing.T) {
	state := NewLoginState("test-csrf-token")

	if state.CSRFToken != "test-csrf-token" {
		t.Errorf("Expected CSRFToken to be 'test-csrf-token', got '%s'", state.CSRFToken)
	}
	if state.Action != ActionLogin {
		t.Errorf("Expected Action to be ActionLogin, got '%s'", state.Action)
	}
	if state.OverlayID != "" {
		t.Errorf("Expected OverlayID to be empty, got '%s'", state.OverlayID)
	}
}

func TestNewAddSourceState(t *testing.T) {
	state := NewAddSourceState("test-csrf-token", "overlay-123", "user-456")

	if state.CSRFToken != "test-csrf-token" {
		t.Errorf("Expected CSRFToken to be 'test-csrf-token', got '%s'", state.CSRFToken)
	}
	if state.Action != ActionAddSource {
		t.Errorf("Expected Action to be ActionAddSource, got '%s'", state.Action)
	}
	if state.OverlayID != "overlay-123" {
		t.Errorf("Expected OverlayID to be 'overlay-123', got '%s'", state.OverlayID)
	}
	if state.UserID != "user-456" {
		t.Errorf("Expected UserID to be 'user-456', got '%s'", state.UserID)
	}
}

func TestNewModerationState(t *testing.T) {
	state := NewModerationState("test-csrf-token", "overlay-123", "user-456")

	// Moderation re-consent reuses the add-source action so the callback runs the
	// same token/scope persistence; only the Purpose marker differs (ADR-0017).
	if state.Action != ActionAddSource {
		t.Errorf("Expected Action to be ActionAddSource (reuses add-source persistence), got '%s'", state.Action)
	}
	if !state.IsAddSource() {
		t.Error("Expected IsAddSource() to be true for a moderation state")
	}
	if !state.IsModeration() {
		t.Error("Expected IsModeration() to be true for a moderation state")
	}
	if state.OverlayID != "overlay-123" {
		t.Errorf("Expected OverlayID to be 'overlay-123', got '%s'", state.OverlayID)
	}
	if state.UserID != "user-456" {
		t.Errorf("Expected UserID to be 'user-456', got '%s'", state.UserID)
	}
	if err := state.Validate(); err != nil {
		t.Errorf("Expected moderation state to validate, got %v", err)
	}
}

func TestOAuthState_IsModeration(t *testing.T) {
	if NewAddSourceState("csrf", "overlay-123", "user-456").IsModeration() {
		t.Error("Expected a plain add-source state to NOT be a moderation state")
	}
	if NewLoginState("csrf").IsModeration() {
		t.Error("Expected a login state to NOT be a moderation state")
	}
}

func TestModerationState_RoundTripPreservesPurpose(t *testing.T) {
	encoded, err := NewModerationState("csrf", "overlay-123", "user-456").Encode()
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := DecodeOAuthState(encoded)
	if err != nil {
		t.Fatalf("DecodeOAuthState() error = %v", err)
	}
	if !decoded.IsModeration() {
		t.Errorf("Expected decoded state to remain a moderation state, got purpose %q", decoded.Purpose)
	}
}

func TestOAuthState_Encode(t *testing.T) {
	tests := []struct {
		name    string
		state   *OAuthState
		wantErr bool
	}{
		{
			name: "login state",
			state: &OAuthState{
				CSRFToken: "test-token",
				Action:    ActionLogin,
			},
			wantErr: false,
		},
		{
			name: "add source state",
			state: &OAuthState{
				CSRFToken: "test-token",
				OverlayID: "overlay-123",
				UserID:    "user-456",
				Action:    ActionAddSource,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := tt.state.Encode()
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && encoded == "" {
				t.Error("Encode() returned empty string")
			}
		})
	}
}

func TestDecodeOAuthState(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    *OAuthState
		wantErr bool
	}{
		{
			name:    "valid login state",
			encoded: `{"csrf_token":"test-token","action":"login"}`,
			want: &OAuthState{
				CSRFToken: "test-token",
				Action:    ActionLogin,
			},
			wantErr: false,
		},
		{
			name:    "valid add source state",
			encoded: `{"csrf_token":"test-token","overlay_id":"overlay-123","user_id":"user-456","action":"add_source"}`,
			want: &OAuthState{
				CSRFToken: "test-token",
				OverlayID: "overlay-123",
				UserID:    "user-456",
				Action:    ActionAddSource,
			},
			wantErr: false,
		},
		{
			name:    "invalid json",
			encoded: `{invalid}`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeOAuthState(tt.encoded)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeOAuthState() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.CSRFToken != tt.want.CSRFToken {
					t.Errorf("CSRFToken = %v, want %v", got.CSRFToken, tt.want.CSRFToken)
				}
				if got.Action != tt.want.Action {
					t.Errorf("Action = %v, want %v", got.Action, tt.want.Action)
				}
				if got.OverlayID != tt.want.OverlayID {
					t.Errorf("OverlayID = %v, want %v", got.OverlayID, tt.want.OverlayID)
				}
			}
		})
	}
}

func TestOAuthState_EncodeAndDecode(t *testing.T) {
	original := &OAuthState{
		CSRFToken: "test-csrf-token",
		OverlayID: "overlay-123",
		UserID:    "user-456",
		Action:    ActionAddSource,
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode() failed: %v", err)
	}

	decoded, err := DecodeOAuthState(encoded)
	if err != nil {
		t.Fatalf("DecodeOAuthState() failed: %v", err)
	}

	if decoded.CSRFToken != original.CSRFToken {
		t.Errorf("CSRFToken = %v, want %v", decoded.CSRFToken, original.CSRFToken)
	}
	if decoded.OverlayID != original.OverlayID {
		t.Errorf("OverlayID = %v, want %v", decoded.OverlayID, original.OverlayID)
	}
	if decoded.UserID != original.UserID {
		t.Errorf("UserID = %v, want %v", decoded.UserID, original.UserID)
	}
	if decoded.Action != original.Action {
		t.Errorf("Action = %v, want %v", decoded.Action, original.Action)
	}
}

func TestOAuthState_IsAddSource(t *testing.T) {
	tests := []struct {
		name  string
		state *OAuthState
		want  bool
	}{
		{
			name:  "add source action",
			state: &OAuthState{Action: ActionAddSource},
			want:  true,
		},
		{
			name:  "login action",
			state: &OAuthState{Action: ActionLogin},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsAddSource(); got != tt.want {
				t.Errorf("IsAddSource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOAuthState_IsLogin(t *testing.T) {
	tests := []struct {
		name  string
		state *OAuthState
		want  bool
	}{
		{
			name:  "login action",
			state: &OAuthState{Action: ActionLogin},
			want:  true,
		},
		{
			name:  "add source action",
			state: &OAuthState{Action: ActionAddSource},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsLogin(); got != tt.want {
				t.Errorf("IsLogin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOAuthState_Validate(t *testing.T) {
	tests := []struct {
		name    string
		state   *OAuthState
		wantErr bool
	}{
		{
			name: "valid login state",
			state: &OAuthState{
				CSRFToken: "test-token",
				Action:    ActionLogin,
			},
			wantErr: false,
		},
		{
			name: "valid add source state",
			state: &OAuthState{
				CSRFToken: "test-token",
				OverlayID: "overlay-123",
				UserID:    "user-456",
				Action:    ActionAddSource,
			},
			wantErr: false,
		},
		{
			name: "missing csrf token",
			state: &OAuthState{
				Action: ActionLogin,
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			state: &OAuthState{
				CSRFToken: "test-token",
				Action:    "invalid",
			},
			wantErr: true,
		},
		{
			name: "add source without overlay_id",
			state: &OAuthState{
				CSRFToken: "test-token",
				Action:    ActionAddSource,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.state.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
