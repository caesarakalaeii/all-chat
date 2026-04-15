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

package sourcemanager

import "time"

// ActiveSource matches the Source Manager API payload for active sources.
type ActiveSource struct {
	ID           string    `json:"id"`
	OverlayID    string    `json:"overlay_id"`
	Platform     string    `json:"platform"`
	ChannelID    string    `json:"channel_id"`
	StreamID     string    `json:"stream_id"`
	StreamSelect string    `json:"stream_select"`
	StreamMatch  string    `json:"stream_match"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ClaimResponse models the JSON response returned for leadership claims.
type ClaimResponse struct {
	Acquired   bool   `json:"acquired"`
	InstanceID string `json:"instance_id"`
	Platform   string `json:"platform"`
	StreamID   string `json:"stream_id"`
}

// RenewResponse models the JSON response returned when renewing leadership.
type RenewResponse struct {
	Renewed    bool   `json:"renewed"`
	InstanceID string `json:"instance_id"`
	Platform   string `json:"platform"`
	StreamID   string `json:"stream_id"`
}

// PeerResponse models the JSON response returned when registering a peer.
type PeerResponse struct {
	PeerCount int    `json:"peer_count"`
	Platform  string `json:"platform"`
	CallerID  string `json:"caller_id"`
}
