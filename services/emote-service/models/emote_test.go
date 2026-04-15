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

	"github.com/stretchr/testify/assert"
)

func TestEmote_Validate(t *testing.T) {
	tests := []struct {
		name    string
		emote   Emote
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid emote",
			emote: Emote{
				Code:     "Kappa",
				URL:      "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/1.0",
				Provider: "twitch",
				Channel:  "xqc",
			},
			wantErr: false,
		},
		{
			name: "valid 7tv emote",
			emote: Emote{
				Code:     "OMEGALUL",
				URL:      "https://cdn.7tv.app/emote/123/1x.webp",
				Provider: "7tv",
				Channel:  "shroud",
			},
			wantErr: false,
		},
		{
			name: "empty code",
			emote: Emote{
				Code:     "",
				URL:      "https://example.com/emote.png",
				Provider: "twitch",
				Channel:  "xqc",
			},
			wantErr: true,
			errMsg:  "code cannot be empty",
		},
		{
			name: "empty URL",
			emote: Emote{
				Code:     "Kappa",
				URL:      "",
				Provider: "twitch",
				Channel:  "xqc",
			},
			wantErr: true,
			errMsg:  "url cannot be empty",
		},
		{
			name: "invalid URL",
			emote: Emote{
				Code:     "Kappa",
				URL:      "not-a-url",
				Provider: "twitch",
				Channel:  "xqc",
			},
			wantErr: true,
			errMsg:  "invalid url format",
		},
		{
			name: "empty provider",
			emote: Emote{
				Code:     "Kappa",
				URL:      "https://example.com/emote.png",
				Provider: "",
				Channel:  "xqc",
			},
			wantErr: true,
			errMsg:  "provider cannot be empty",
		},
		{
			name: "invalid provider",
			emote: Emote{
				Code:     "Kappa",
				URL:      "https://example.com/emote.png",
				Provider: "invalid",
				Channel:  "xqc",
			},
			wantErr: true,
			errMsg:  "provider must be one of: twitch, 7tv, bttv, ffz",
		},
		{
			name: "empty channel",
			emote: Emote{
				Code:     "Kappa",
				URL:      "https://example.com/emote.png",
				Provider: "twitch",
				Channel:  "",
			},
			wantErr: true,
			errMsg:  "channel cannot be empty",
		},
		{
			name: "code too long",
			emote: Emote{
				Code:     strings.Repeat("a", 101),
				URL:      "https://example.com/emote.png",
				Provider: "twitch",
				Channel:  "xqc",
			},
			wantErr: true,
			errMsg:  "code cannot exceed 100 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.emote.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEmote_IsValidProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		want     bool
	}{
		{"twitch is valid", "twitch", true},
		{"7tv is valid", "7tv", true},
		{"bttv is valid", "bttv", true},
		{"ffz is valid", "ffz", true},
		{"invalid provider", "invalid", false},
		{"empty provider", "", false},
		{"case sensitive - lowercase twitch", "Twitch", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidProvider(tt.provider)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEmoteResponse_Validate(t *testing.T) {
	tests := []struct {
		name    string
		resp    EmoteResponse
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid response",
			resp: EmoteResponse{
				Channel: "xqc",
				Emotes: []Emote{
					{
						Code:     "Kappa",
						URL:      "https://example.com/kappa.png",
						Provider: "twitch",
						Channel:  "xqc",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty channel",
			resp: EmoteResponse{
				Channel: "",
				Emotes:  []Emote{},
			},
			wantErr: true,
			errMsg:  "channel cannot be empty",
		},
		{
			name: "invalid emote in list",
			resp: EmoteResponse{
				Channel: "xqc",
				Emotes: []Emote{
					{
						Code:     "",
						URL:      "https://example.com/kappa.png",
						Provider: "twitch",
						Channel:  "xqc",
					},
				},
			},
			wantErr: true,
			errMsg:  "code cannot be empty",
		},
		{
			name: "empty emotes list is valid",
			resp: EmoteResponse{
				Channel: "xqc",
				Emotes:  []Emote{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resp.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
