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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "Channels I moderate" (ADR-0048). GET /api/v1/overlays is owner-filtered, so this query is the
// only thing standing between an accepted moderator and an unreachable grant.

// grantWithLegs creates a grant and enables the named platform legs on it.
func grantWithLegs(t *testing.T, r *Repository, modID, status string, actions, platforms []string) string {
	t.Helper()
	ctx := context.Background()
	var grantID string
	err := r.db.QueryRow(ctx,
		`INSERT INTO overlay_moderators (overlay_id, moderator_user_id, granted_by, status, actions, accepted_at)
		 VALUES ($1, $2, $3, $4, $5, NOW()) RETURNING id::text`,
		overlayID, modID, ownerID, status, actions).Scan(&grantID)
	require.NoError(t, err)
	for _, p := range platforms {
		_, err := r.db.Exec(ctx,
			`INSERT INTO overlay_moderator_platforms (grant_id, platform, enabled) VALUES ($1, $2, TRUE)`,
			grantID, p)
		require.NoError(t, err)
	}
	return grantID
}

func TestListDelegationsFor(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	modID := newModerator(t, repo, "A Moderator")
	grantID := grantWithLegs(t, repo, modID, "active", []string{"delete", "timeout"}, []string{"twitch"})

	delegations, err := repo.ListDelegationsFor(ctx, modID)
	require.NoError(t, err)
	require.Len(t, delegations, 1)

	d := delegations[0]
	assert.Equal(t, grantID, d.GrantID)
	assert.Equal(t, overlayID, d.OverlayID, "the overlay id is the whole point of this listing")
	assert.Equal(t, "My Overlay", d.OverlayName)
	assert.Equal(t, ownerID, d.OwnerUserID)
	assert.Equal(t, "The Streamer", d.OwnerDisplayName)
	assert.Equal(t, []string{"delete", "timeout"}, d.Actions)
	assert.NotNil(t, d.AcceptedAt)
	require.Len(t, d.Platforms, 1)
	assert.Equal(t, "twitch", d.Platforms[0].Platform)
	assert.True(t, d.Platforms[0].Enabled)
}

// The listing is scoped to the caller. A moderator on one overlay must not see another's.
func TestListDelegationsFor_ScopedToTheModerator(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	mine := newModerator(t, repo, "A Moderator")
	theirs := newModerator(t, repo, "A Moderator")
	grantWithLegs(t, repo, mine, "active", []string{"delete"}, []string{"twitch"})

	delegations, err := repo.ListDelegationsFor(ctx, theirs)
	require.NoError(t, err)
	assert.Empty(t, delegations, "someone else's grant is not mine to see")
}

// The owner is not their own moderator: ownership is not a grant, and self-listing would put the
// streamer's own overlays in a list that means "channels other people handed me".
func TestListDelegationsFor_OwnerSeesNothing(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	delegations, err := repo.ListDelegationsFor(context.Background(), ownerID)
	require.NoError(t, err)
	assert.Empty(t, delegations)
}

// Which lifecycle states reach the moderator's list. Suspended is included deliberately: a channel
// that silently vanished would be indistinguishable from a revocation, leaving the moderator with
// nothing to ask the streamer about.
func TestListDelegationsFor_LifecycleStates(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	cases := []struct {
		name    string
		status  string
		revoked bool
		listed  bool
	}{
		{"active is listed", "active", false, true},
		{"suspended is listed, so the moderator can ask for reactivation", "suspended", false, true},
		{"revoked is not listed", "revoked", true, false},
		{"a revoked_at stamp hides an otherwise active grant", "active", true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modID := newModerator(t, repo, "A Moderator")
			grantWithLegs(t, repo, modID, tc.status, []string{"delete"}, []string{"twitch"})
			if tc.revoked {
				_, err := repo.db.Exec(ctx,
					`UPDATE overlay_moderators SET revoked_at = NOW() WHERE moderator_user_id = $1`, modID)
				require.NoError(t, err)
			}

			delegations, err := repo.ListDelegationsFor(ctx, modID)
			require.NoError(t, err)
			assert.Equal(t, tc.listed, len(delegations) == 1)
		})
	}
}

// A pending invite has no moderator bound to it yet, so it cannot appear in anyone's list — the
// invite secret is the only thing that reaches it.
func TestListDelegationsFor_PendingInviteIsUnreachable(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	_, err := repo.db.Exec(ctx,
		`INSERT INTO overlay_moderators (overlay_id, granted_by, status) VALUES ($1, $2, 'pending')`,
		overlayID, ownerID)
	require.NoError(t, err)

	modID := newModerator(t, repo, "A Moderator")
	delegations, err := repo.ListDelegationsFor(ctx, modID)
	require.NoError(t, err)
	assert.Empty(t, delegations)
}

// A disabled leg is still reported, with enabled=false: the moderator's page explains which
// platforms the streamer has turned on, and an absent row would be indistinguishable from a
// platform the overlay does not carry.
func TestListDelegationsFor_ReportsDisabledLegs(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	modID := newModerator(t, repo, "A Moderator")
	grantID := grantWithLegs(t, repo, modID, "active", []string{"delete"}, []string{"twitch"})
	_, err := repo.db.Exec(ctx,
		`INSERT INTO overlay_moderator_platforms (grant_id, platform, enabled) VALUES ($1, 'kick', FALSE)`,
		grantID)
	require.NoError(t, err)

	delegations, err := repo.ListDelegationsFor(ctx, modID)
	require.NoError(t, err)
	require.Len(t, delegations, 1)

	legs := map[string]bool{}
	for _, leg := range delegations[0].Platforms {
		legs[leg.Platform] = leg.Enabled
	}
	assert.Equal(t, map[string]bool{"twitch": true, "kick": false}, legs)
}

// A malformed caller id must not reach Postgres and come back as a 500.
func TestListDelegationsFor_MalformedUserID(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()

	delegations, err := repo.ListDelegationsFor(context.Background(), "not-a-uuid")
	require.NoError(t, err)
	assert.Empty(t, delegations)
}

// The moderator's own credential is what authorizes them, so the capability answer reads its
// scopes — and only its scopes, never a token.
func TestModeratorGrantedScopes(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	modID := newModerator(t, repo, "A Moderator")
	_, err := repo.db.Exec(ctx, `
		INSERT INTO mod_oauth_credentials (user_id, platform, platform_user_id, access_token, granted_scopes)
		VALUES ($1, 'twitch', '4242', 'encrypted', $2)`,
		modID, []string{"moderator:manage:chat_messages"})
	require.NoError(t, err)

	scopes, ok, err := repo.ModeratorGrantedScopes(ctx, modID, "twitch")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []string{"moderator:manage:chat_messages"}, scopes)

	t.Run("a platform they never consented to reports no credential", func(t *testing.T) {
		_, ok, err := repo.ModeratorGrantedScopes(ctx, modID, "kick")
		require.NoError(t, err)
		assert.False(t, ok, "absence must be distinguishable from an empty scope list")
	})

	t.Run("another user's credential is not visible", func(t *testing.T) {
		_, ok, err := repo.ModeratorGrantedScopes(ctx, newModerator(t, repo, "A Moderator"), "twitch")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("malformed user id does not surface a database error", func(t *testing.T) {
		_, ok, err := repo.ModeratorGrantedScopes(ctx, "not-a-uuid", "twitch")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
