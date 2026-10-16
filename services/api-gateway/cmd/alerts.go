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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/shared/alerts"
)

// alertProfileLookup returns an overlay's saved profile for an event type;
// found is false when the overlay runs on the default.
type alertProfileLookup func(ctx context.Context, overlayID, eventType string) (p alerts.Profile, found bool, err error)

// buildAlertFrame wraps one overlay:{id}:alerts payload in a type "alert"
// WebSocket frame (ADR-0070): the alert envelope unchanged, plus "profile" —
// the overlay's saved or default profile for the event type with its text
// template already rendered. Event types without profiles are forwarded with
// no "profile" key. It returns a nil frame for a profile the streamer switched
// off. A failed lookup falls back to the default profile and is returned as
// the error alongside a usable frame: losing the alert is worse than showing
// the default text.
func buildAlertFrame(ctx context.Context, overlayID string, payload []byte, lookup alertProfileLookup, now time.Time) ([]byte, error) {
	var data map[string]json.RawMessage
	var alert alerts.Alert
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("decode alert: %w", err)
	}
	if err := json.Unmarshal(payload, &alert); err != nil {
		return nil, fmt.Errorf("decode alert: %w", err)
	}

	var lookupErr error
	if profile, known := alerts.DefaultProfile(alert.EventType); known {
		saved, found, err := lookup(ctx, overlayID, alert.EventType)
		if err != nil {
			lookupErr = fmt.Errorf("alert profile lookup: %w", err)
		} else if found {
			profile = saved
		}
		if !profile.Enabled {
			return nil, nil
		}
		resolved, err := json.Marshal(alerts.Resolve(profile, alert))
		if err != nil {
			return nil, err
		}
		data["profile"] = resolved
	}

	msg := models.WSMessage{Type: models.WSMessageTypeAlert, Data: data, Timestamp: now}
	frame, err := msg.ToJSON()
	if err != nil {
		return nil, err
	}
	return frame, lookupErr
}
