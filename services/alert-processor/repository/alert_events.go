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
	"encoding/json"
	"fmt"

	"github.com/caesar/all-chat/services/alert-processor/models"
)

// InsertAlertEvent persists one routed alert. ON CONFLICT (id) DO NOTHING is
// the other half of the deterministic alert id (models.NewAlertID): the
// consumer group is at-least-once, so this exact row can be written twice after
// a redelivery, and the second write must be absorbed rather than appended.
func (r *Repository) InsertAlertEvent(ctx context.Context, alert *models.Alert) error {
	eventData, err := json.Marshal(alert.EventData)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO alert_events (id, overlay_id, platform, event_type, user_id, user_name, event_data, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO NOTHING
	`,
		alert.AlertID,
		alert.OverlayID,
		alert.Platform,
		alert.EventType,
		alert.User.ID,
		alert.User.Name,
		string(eventData),
		alert.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert alert event: %w", err)
	}
	return nil
}