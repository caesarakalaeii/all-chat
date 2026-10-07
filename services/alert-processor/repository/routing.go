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

package repository

import (
	"context"
	"fmt"
)

// FindAlertOverlays returns the alert-capable overlays subscribed to
// platform/channelID. It mirrors the message-processor's overlay_router UNION
// (direct overlay_chat_sources plus the accepted share_requests fan-out) with
// one addition and one deliberate non-change:
//
//   - both branches filter o.overlay_type IN ('alerts','goal','list'), so chat
//     overlays on the same source never receive alert events;
//   - the message-processor's router is NOT touched — it keeps serving the chat
//     pipeline byte-identically (issue #951 out of scope), which is why this
//     query lives here rather than being reused from there.
func (r *Repository) FindAlertOverlays(ctx context.Context, platform, channelID string) ([]AlertOverlay, error) {
	query := `
		-- Direct platform sources (alert overlays that have this platform/channel directly)
		SELECT DISTINCT o.id, o.overlay_type
		FROM overlays o
		JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
		WHERE o.is_active = true
		  AND ocs.is_active = true
		  AND ocs.platform = $1
		  AND ocs.channel_id = $2
		  AND o.overlay_type IN ('alerts','goal','list')

		UNION

		-- Shared overlay fan-out: recipient alert overlays that subscribed to sender overlay
		SELECT DISTINCT o.id, o.overlay_type
		FROM overlays o
		JOIN overlay_chat_sources ocs
		    ON o.id = ocs.overlay_id
		    AND ocs.platform = 'shared_overlay'
		    AND ocs.is_active = true
		JOIN share_requests sr
		    ON sr.sender_overlay_id = ocs.channel_id::uuid
		    AND sr.status = 'accepted'
		WHERE o.is_active = true
		  AND o.overlay_type IN ('alerts','goal','list')
		  AND sr.sender_overlay_id IN (
		      SELECT o2.id
		      FROM overlays o2
		      JOIN overlay_chat_sources ocs2 ON o2.id = ocs2.overlay_id
		      WHERE o2.is_active = true
		        AND ocs2.is_active = true
		        AND ocs2.platform = $1
		        AND ocs2.channel_id = $2
		  )
	`

	rows, err := r.db.Query(ctx, query, platform, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query alert overlays: %w", err)
	}
	defer rows.Close()

	var overlays []AlertOverlay
	for rows.Next() {
		var target AlertOverlay
		if err := rows.Scan(&target.OverlayID, &target.OverlayType); err != nil {
			return nil, fmt.Errorf("failed to scan alert overlay row: %w", err)
		}
		overlays = append(overlays, target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating alert overlay rows: %w", err)
	}

	return overlays, nil
}
