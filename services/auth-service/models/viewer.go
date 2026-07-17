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
	"time"

	"github.com/google/uuid"
)

// ViewerSession represents a viewer's OAuth session for sending messages
type ViewerSession struct {
	ID                  uuid.UUID  `json:"id"`
	Platform            string     `json:"platform"`               // 'twitch', 'youtube', 'kick', 'tiktok'
	PlatformUserID      string     `json:"platform_user_id"`       // Platform-specific user ID
	Username            string     `json:"username"`               // Platform username
	DisplayName         string     `json:"display_name"`           // Display name
	AvatarURL           *string    `json:"avatar_url"`             // Profile picture URL
	AccessToken         string     `json:"-"`                      // Encrypted OAuth access token
	RefreshToken        *string    `json:"-"`                      // Encrypted OAuth refresh token
	TokenExpiresAt      time.Time  `json:"token_expires_at"`       // Token expiration
	LastMessageAt       *time.Time `json:"last_message_at"`        // Last message timestamp
	MessageCount1Min    int        `json:"message_count_1min"`     // Messages in last 1 minute
	MessageCount1Hour   int        `json:"message_count_1hour"`    // Messages in last 1 hour
	RateLimitReset1Min  *time.Time `json:"rate_limit_reset_1min"`  // 1-minute counter reset
	RateLimitReset1Hour *time.Time `json:"rate_limit_reset_1hour"` // 1-hour counter reset
	IsPremium           bool       `json:"is_premium"`             // Whether viewer has premium access
	PremiumExpiresAt    *time.Time `json:"premium_expires_at"`     // ADR-0027: time-limited admin premium deadline (NULL = permanent)
	ViewerID            *uuid.UUID `json:"viewer_id"`              // Linked viewer identity ID
	UserID              *string    `json:"user_id"`                // Linked streamer account (viewer_sessions.user_id, migration 040); NULL when the viewer has no streamer account
	IsBanned            bool       `json:"is_banned"`              // Whether viewer is banned
	BannedAt            *time.Time `json:"banned_at"`              // When viewer was banned
	BannedReason        *string    `json:"banned_reason"`          // Reason for ban
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// ViewerMessageLog represents a message sent by a viewer through All-Chat
type ViewerMessageLog struct {
	ID              uuid.UUID  `json:"id"`
	ViewerSessionID uuid.UUID  `json:"viewer_session_id"`
	StreamerUserID  uuid.UUID  `json:"streamer_user_id"`
	OverlayID       *uuid.UUID `json:"overlay_id"`    // May be null
	Platform        string     `json:"platform"`      // Platform message was sent to
	ChannelID       string     `json:"channel_id"`    // Target channel ID
	ChannelName     string     `json:"channel_name"`  // Target channel name
	MessageText     string     `json:"message_text"`  // Message content
	SentAt          time.Time  `json:"sent_at"`       // When message was sent
	Success         bool       `json:"success"`       // Whether send was successful
	ErrorMessage    *string    `json:"error_message"` // Error details if failed
	CreatedAt       time.Time  `json:"created_at"`
}

// ViewerAuthResponse is returned after successful viewer OAuth
type ViewerAuthResponse struct {
	Token      string     `json:"token"`       // JWT token for viewer session
	ExpiresIn  int        `json:"expires_in"`  // JWT expiration in seconds
	ViewerInfo ViewerInfo `json:"viewer_info"` // Viewer information
}

// ViewerInfo contains viewer profile information
type ViewerInfo struct {
	ID          uuid.UUID `json:"id"`           // ViewerSession ID
	Platform    string    `json:"platform"`     // Platform name
	Username    string    `json:"username"`     // Platform username
	DisplayName string    `json:"display_name"` // Display name
	AvatarURL   *string   `json:"avatar_url"`   // Profile picture URL
}

// ViewerJWTClaims represents JWT claims for viewer sessions
type ViewerJWTClaims struct {
	SessionID      uuid.UUID `json:"session_id"`
	Platform       string    `json:"platform"`
	PlatformUserID string    `json:"platform_user_id"`
	Username       string    `json:"username"`
	IsViewer       bool      `json:"is_viewer"` // Distinguishes viewer tokens from streamer tokens
	ExpiresAt      int64     `json:"exp"`
	IssuedAt       int64     `json:"iat"`
}
