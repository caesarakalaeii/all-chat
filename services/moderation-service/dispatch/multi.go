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

package dispatch

import (
	"context"

	"github.com/caesar/all-chat/services/moderation-service/models"
)

// PlatformDispatcher performs the real moderation call for a single platform.
// *Twitch, *Discord, *Kick, and *YouTube each satisfy it.
type PlatformDispatcher interface {
	Dispatch(ctx context.Context, userID string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error)
}

// Multi routes a platform-agnostic moderation command to the dispatcher registered for
// its platform, keeping the HTTP handler platform-agnostic: adding a platform means
// registering one more entry here, not touching the handler. A platform with no
// registered dispatcher reports DispatchDryRun, so the handler still emits the
// reflect-back event — the safe pre-client behaviour — when a platform's credentials
// are not configured for this deployment.
type Multi struct {
	byPlatform map[string]PlatformDispatcher
}

// NewMulti wires a router over the given per-platform dispatchers (keyed by platform).
func NewMulti(byPlatform map[string]PlatformDispatcher) *Multi {
	return &Multi{byPlatform: byPlatform}
}

// Dispatch routes by req.Platform.
func (m *Multi) Dispatch(ctx context.Context, userID string, action models.Action, req models.DispatchRequest) (models.DispatchResult, error) {
	if d, ok := m.byPlatform[req.Platform]; ok && d != nil {
		return d.Dispatch(ctx, userID, action, req)
	}
	return models.DispatchResult{Outcome: models.DispatchDryRun}, nil
}
