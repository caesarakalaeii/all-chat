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
	"testing"

	"github.com/google/uuid"
)

func TestChatSource_Validate(t *testing.T) {
	tests := []struct {
		name    string
		source  *ChatSource
		wantErr bool
	}{
		{
			name: "valid twitch source",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "twitch",
				ChannelID:   "shroud",
				ChannelName: "shroud",
				IsActive:    true,
			},
			wantErr: false,
		},
		{
			name: "valid youtube source",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "youtube",
				ChannelID:   "UCxyz123",
				ChannelName: "Example Channel",
				IsActive:    true,
			},
			wantErr: false,
		},
		{
			name: "missing overlay_id",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   "",
				Platform:    "twitch",
				ChannelID:   "shroud",
				ChannelName: "shroud",
			},
			wantErr: true,
		},
		{
			name: "missing platform",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "",
				ChannelID:   "shroud",
				ChannelName: "shroud",
			},
			wantErr: true,
		},
		{
			name: "invalid platform",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "invalid_platform",
				ChannelID:   "shroud",
				ChannelName: "shroud",
			},
			wantErr: true,
		},
		{
			name: "missing channel_id",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "twitch",
				ChannelID:   "",
				ChannelName: "shroud",
			},
			wantErr: true,
		},
		{
			name: "missing channel_name",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "twitch",
				ChannelID:   "shroud",
				ChannelName: "",
			},
			wantErr: true,
		},
		{
			name: "valid kick source",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "kick",
				ChannelID:   "xqc",
				ChannelName: "xQc",
				IsActive:    true,
			},
			wantErr: false,
		},
		{
			name: "valid tiktok source",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "tiktok",
				ChannelID:   "@user123",
				ChannelName: "User 123",
				IsActive:    true,
			},
			wantErr: false,
		},
		{
			name: "valid shared_overlay source",
			source: &ChatSource{
				ID:          uuid.New().String(),
				OverlayID:   uuid.New().String(),
				Platform:    "shared_overlay",
				ChannelID:   uuid.New().String(), // sender_overlay_id UUID
				ChannelName: "xqc's overlay",
				IsActive:    true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ChatSource.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChatSource_IsValidPlatform(t *testing.T) {
	tests := []struct {
		platform string
		want     bool
	}{
		{"twitch", true},
		{"youtube", true},
		{"kick", true},
		{"tiktok", true},
		{"shared_overlay", true},
		{"discord", true}, // Phase 27: Discord Listener added
		{"", false},
		{"TWITCH", false}, // case-sensitive
		{"twitter", false},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			source := &ChatSource{Platform: tt.platform}
			if got := source.IsValidPlatform(); got != tt.want {
				t.Errorf("IsValidPlatform() = %v, want %v", got, tt.want)
			}
		})
	}
}
