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

func (r *recordingDispatcher) Dispatch(_ context.Context, _ models.Actor, _ models.Action, _ models.DispatchRequest) (models.DispatchResult, error) {
	r.calls++
	return models.DispatchResult{Outcome: models.DispatchPerformed, PlatformStatus: r.platform}, nil
}

func TestMulti_RoutesByPlatform(t *testing.T) {
	twitch := &recordingDispatcher{platform: "twitch"}
	discord := &recordingDispatcher{platform: "discord"}
	m := NewMulti(map[string]PlatformDispatcher{"twitch": twitch, "discord": discord})

	res, err := m.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "discord"})
	require.NoError(t, err)
	assert.Equal(t, "discord", res.PlatformStatus, "request routed to the discord dispatcher")
	assert.Equal(t, 1, discord.calls)
	assert.Zero(t, twitch.calls, "the twitch dispatcher is untouched for a discord request")
}

func TestMulti_UnregisteredPlatformIsDryRun(t *testing.T) {
	twitch := &recordingDispatcher{platform: "twitch"}
	m := NewMulti(map[string]PlatformDispatcher{"twitch": twitch})

	res, err := m.Dispatch(context.Background(), owner("u1"), models.ActionBan, models.DispatchRequest{Platform: "kick"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchDryRun, res.Outcome, "an unconfigured platform falls back to dry-run reflect-back")
	assert.Zero(t, twitch.calls)
}

// owner builds an Actor for a streamer moderating their own overlay — the role every test in
// this package used implicitly before delegation existed.
func owner(userID string) models.Actor {
	return models.Actor{UserID: userID, Role: models.RoleOwner, OwnerUserID: userID}
}

// moderator builds an Actor for a delegated moderator acting on ownerID's overlay.
func moderator(userID, ownerID string) models.Actor {
	return models.Actor{
		UserID: userID, Role: models.RoleModerator, OwnerUserID: ownerID, GrantID: "grant-1",
	}
}

// Delegation is gated per platform (ADR-0048 Phase 2), and each unbuilt leg must REFUSE rather
// than fall through. The reasons differ by platform but the requirement does not.
func TestDelegatedActionsAreRefusedOnUnbuiltLegs(t *testing.T) {
	t.Run("unregistered platform is not a dry run for a moderator", func(t *testing.T) {
		// For an owner, dry-run means "reflect back, nothing happened upstream". For a moderator
		// it would report success for an action nobody performed on a platform with no delegated
		// path at all.
		m := NewMulti(map[string]PlatformDispatcher{})

		res, err := m.Dispatch(context.Background(), moderator("mod", "own"), models.ActionDelete,
			models.DispatchRequest{Platform: "tiktok"})

		require.NoError(t, err)
		assert.Equal(t, models.DispatchDelegationUnsupported, res.Outcome)
	})

	t.Run("an owner still gets the dry run", func(t *testing.T) {
		m := NewMulti(map[string]PlatformDispatcher{})

		res, err := m.Dispatch(context.Background(), owner("u1"), models.ActionDelete,
			models.DispatchRequest{Platform: "tiktok"})

		require.NoError(t, err)
		assert.Equal(t, models.DispatchDryRun, res.Outcome)
	})
}
