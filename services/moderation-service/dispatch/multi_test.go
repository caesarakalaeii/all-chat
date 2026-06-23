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
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingDispatcher struct {
	platform string
	calls    int
}

func (r *recordingDispatcher) Dispatch(_ context.Context, _ string, _ models.Action, _ models.DispatchRequest) (models.DispatchResult, error) {
	r.calls++
	return models.DispatchResult{Outcome: models.DispatchPerformed, PlatformStatus: r.platform}, nil
}

func TestMulti_RoutesByPlatform(t *testing.T) {
	twitch := &recordingDispatcher{platform: "twitch"}
	discord := &recordingDispatcher{platform: "discord"}
	m := NewMulti(map[string]PlatformDispatcher{"twitch": twitch, "discord": discord})

	res, err := m.Dispatch(context.Background(), "u1", models.ActionDelete, models.DispatchRequest{Platform: "discord"})
	require.NoError(t, err)
	assert.Equal(t, "discord", res.PlatformStatus, "request routed to the discord dispatcher")
	assert.Equal(t, 1, discord.calls)
	assert.Zero(t, twitch.calls, "the twitch dispatcher is untouched for a discord request")
}

func TestMulti_UnregisteredPlatformIsDryRun(t *testing.T) {
	twitch := &recordingDispatcher{platform: "twitch"}
	m := NewMulti(map[string]PlatformDispatcher{"twitch": twitch})

	res, err := m.Dispatch(context.Background(), "u1", models.ActionBan, models.DispatchRequest{Platform: "kick"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchDryRun, res.Outcome, "an unconfigured platform falls back to dry-run reflect-back")
	assert.Zero(t, twitch.calls)
}
