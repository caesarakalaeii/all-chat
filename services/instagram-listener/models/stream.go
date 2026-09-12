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

import "time"

// IGStream represents the resolved live-media state for one Instagram source
// (i.e. one IG professional account). Mirrors facebook-listener's FBStream
// shape, minus the quota-related fields (ADR-0062: no quota subsystem for
// Instagram — IG rate limits are BUC-based like Facebook's).
type IGStream struct {
	StreamID      string    // Live media (broadcast) ID
	ChannelID     string    // IG user ID
	ChannelName   string    // IG @handle
	OverlayID     string    // overlay this source belongs to
	UserID        string    // All-Chat user who owns the credential (for the token store lookup)
	MediaProduct  string    // Graph media_product_type: "LIVE" while broadcasting
	LastCommentID string    // Polling cursor: id of the newest comment seen
	Interval      int64     // polling interval in ms (per-stream override)
	StartedAt     time.Time // when this stream was first resolved
	UpdatedAt     time.Time
}
