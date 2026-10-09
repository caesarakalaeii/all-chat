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
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOverlay_Validate(t *testing.T) {
	tests := []struct {
		name    string
		overlay *Overlay
		wantErr bool
	}{
		{
			name: "valid overlay",
			overlay: &Overlay{
				ID:          uuid.New().String(),
				UserID:      uuid.New().String(),
				Name:        "My Overlay",
				Description: "Test description",
				IsActive:    true,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing user_id",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   "",
				Name:     "My Overlay",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "missing name",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   uuid.New().String(),
				Name:     "",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "name too short",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   uuid.New().String(),
				Name:     "",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "name too long (over 100 chars)",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   uuid.New().String(),
				Name:     "a very long name that exceeds the maximum allowed length of 100 characters and should fail validation check",
				IsActive: true,
			},
			wantErr: true,
		},
		{
			name: "description too long (over 500 chars)",
			overlay: &Overlay{
				ID:          uuid.New().String(),
				UserID:      uuid.New().String(),
				Name:        "Valid Name",
				Description: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum. Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam.",
				IsActive:    true,
			},
			wantErr: true,
		},
		{
			name: "valid overlay with empty description (optional)",
			overlay: &Overlay{
				ID:          uuid.New().String(),
				UserID:      uuid.New().String(),
				Name:        "My Overlay",
				Description: "",
				IsActive:    true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.overlay.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Overlay.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// overlay_type (ADR-0064): the four kinds Validate accepts, the default an
// absent kind resolves to, and what must be rejected.
func TestOverlay_ValidateOverlayType(t *testing.T) {
	// The smallest Overlay that passes Validate, so each case below varies only
	// the field under test.
	validOverlay := func() *Overlay {
		return &Overlay{
			UserID: uuid.New().String(),
			Name:   "My Overlay",
		}
	}

	supported := []string{OverlayTypeChat, OverlayTypeAlerts, OverlayTypeGoal, OverlayTypeList}

	for _, overlayType := range supported {
		t.Run("accepts "+overlayType, func(t *testing.T) {
			overlay := validOverlay()
			overlay.OverlayType = overlayType

			if err := overlay.Validate(); err != nil {
				t.Fatalf("supported kind %q rejected: %v", overlayType, err)
			}
			if overlay.OverlayType != overlayType {
				t.Errorf("a supported kind must survive validation unchanged, got %q", overlay.OverlayType)
			}
		})
	}

	t.Run("absent kind resolves to chat", func(t *testing.T) {
		overlay := validOverlay()

		if err := overlay.Validate(); err != nil {
			t.Fatalf("an overlay created before the column existed must still validate: %v", err)
		}
		if overlay.OverlayType != OverlayTypeChat {
			t.Errorf("absent kind resolved to %q, want %q", overlay.OverlayType, OverlayTypeChat)
		}
	})

	t.Run("rejects unknown kind", func(t *testing.T) {
		overlay := validOverlay()
		overlay.OverlayType = "webcam"

		err := overlay.Validate()
		if err == nil {
			t.Fatal("an unknown kind must be rejected")
		}
		if !strings.Contains(err.Error(), "overlay_type") || !strings.Contains(err.Error(), "webcam") {
			t.Errorf("error must name the field and the bad value, got: %v", err)
		}
	})

	t.Run("rejects case variant", func(t *testing.T) {
		overlay := validOverlay()
		overlay.OverlayType = "Chat"

		if err := overlay.Validate(); err == nil {
			t.Fatal("kinds are wire values, not free text: 'Chat' must not silently become a chat overlay")
		}
	})
}
