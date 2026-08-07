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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ResolveOverlayAccess replaces VerifyOverlayOwnership on the action path (ADR-0048). It answers
// three questions in one round trip — who owns the overlay, whether THAT owner is entitled, and
// what role the caller holds — because the premium gate must key on the owner while the
// authorization keys on the caller.

func grant(t *testing.T, r *Repository, modID, status string, actions []string, revoked bool) {
	t.Helper()
	revokedAt := "NULL"
	if revoked {
		revokedAt = "NOW()"
	}
	_, err := r.db.Exec(context.Background(),
		`INSERT INTO overlay_moderators (overlay_id, moderator_user_id, granted_by, status, actions, revoked_at)
		 VALUES ($1, $2, $3, $4, $5, `+revokedAt+`)`,
		overlayID, modID, ownerID, status, actions)
	require.NoError(t, err)
}

func TestResolveOverlayAccess(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	modID := uuid.New().String()
	_, err := repo.db.Exec(ctx, `INSERT INTO users (id, is_premium) VALUES ($1, false)`, modID)
	require.NoError(t, err)

	t.Run("owner gets role owner and their own entitlement", func(t *testing.T) {
		access, err := repo.ResolveOverlayAccess(ctx, overlayID, ownerID)
		require.NoError(t, err)
		assert.Equal(t, RoleOwner, access.Role)
		assert.Equal(t, ownerID, access.OwnerUserID)
		assert.True(t, access.OwnerIsPremium)
	})

	t.Run("stranger gets role none", func(t *testing.T) {
		access, err := repo.ResolveOverlayAccess(ctx, overlayID, strangerID)
		require.NoError(t, err)
		assert.Equal(t, RoleNone, access.Role)
	})

	t.Run("active grant makes the caller a moderator and reports the OWNER's premium", func(t *testing.T) {
		grant(t, repo, modID, "active", []string{"delete", "timeout"}, false)

		access, err := repo.ResolveOverlayAccess(ctx, overlayID, modID)
		require.NoError(t, err)
		assert.Equal(t, RoleModerator, access.Role)
		assert.Equal(t, ownerID, access.OwnerUserID)
		assert.True(t, access.OwnerIsPremium,
			"the gate must key on the owner's entitlement, not the moderator's")
		assert.Equal(t, []string{"delete", "timeout"}, access.Actions)
		assert.NotEmpty(t, access.GrantID)
	})

	t.Run("a non-premium moderator is still a moderator", func(t *testing.T) {
		access, err := repo.ResolveOverlayAccess(ctx, overlayID, modID)
		require.NoError(t, err)
		// The moderator's own is_premium is false; it must not appear anywhere in the answer.
		assert.True(t, access.OwnerIsPremium)
		assert.Equal(t, RoleModerator, access.Role)
	})
}

// The enabled platform legs travel with the role, because the action path needs both in the same
// round trip: a grant's action set and its platform set are two separate grants of authority, and
// checking only the first would let a Twitch-only moderator act on every source the overlay has.
func TestResolveOverlayAccess_PlatformLegs(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("only enabled legs are reported", func(t *testing.T) {
		modID := uuid.New().String()
		_, err := repo.db.Exec(ctx, `INSERT INTO users (id, is_premium) VALUES ($1, false)`, modID)
		require.NoError(t, err)
		grant(t, repo, modID, "active", []string{"delete"}, false)

		var grantID string
		require.NoError(t, repo.db.QueryRow(ctx,
			`SELECT id::text FROM overlay_moderators WHERE moderator_user_id = $1`, modID).Scan(&grantID))
		_, err = repo.db.Exec(ctx, `
			INSERT INTO overlay_moderator_platforms (grant_id, platform, enabled) VALUES
			($1, 'twitch', TRUE), ($1, 'kick', FALSE)`, grantID)
		require.NoError(t, err)

		access, err := repo.ResolveOverlayAccess(ctx, overlayID, modID)
		require.NoError(t, err)
		assert.Equal(t, []string{"twitch"}, access.Platforms,
			"a disabled leg is not a delegated platform")
		assert.True(t, access.MayUsePlatform("twitch"))
		assert.False(t, access.MayUsePlatform("kick"))
	})

	t.Run("a grant with no legs delegates no platform", func(t *testing.T) {
		modID := uuid.New().String()
		_, err := repo.db.Exec(ctx, `INSERT INTO users (id, is_premium) VALUES ($1, false)`, modID)
		require.NoError(t, err)
		grant(t, repo, modID, "active", []string{"delete"}, false)

		access, err := repo.ResolveOverlayAccess(ctx, overlayID, modID)
		require.NoError(t, err)
		assert.Empty(t, access.Platforms, "absence is disablement, not a default of everything")
		assert.False(t, access.MayUsePlatform("twitch"))
	})

	t.Run("an owner is never narrowed by legs", func(t *testing.T) {
		access, err := repo.ResolveOverlayAccess(ctx, overlayID, ownerID)
		require.NoError(t, err)
		assert.True(t, access.MayUsePlatform("twitch"))
		assert.True(t, access.MayUsePlatform("discord"))
	})
}

func TestResolveOverlayAccess_GrantLifecycle(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	cases := []struct {
		name    string
		status  string
		revoked bool
		want    string
	}{
		{"pending invite does not authorize", "pending", false, RoleNone},
		{"suspended grant does not authorize", "suspended", false, RoleNone},
		{"revoked grant does not authorize", "revoked", true, RoleNone},
		{"a revoked_at stamp beats an active status", "active", true, RoleNone},
		{"active grant authorizes", "active", false, RoleModerator},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			modID := uuid.New().String()
			_, err := repo.db.Exec(ctx, `INSERT INTO users (id, is_premium) VALUES ($1, false)`, modID)
			require.NoError(t, err)
			grant(t, repo, modID, tc.status, []string{"delete"}, tc.revoked)

			access, err := repo.ResolveOverlayAccess(ctx, overlayID, modID)
			require.NoError(t, err)
			assert.Equal(t, tc.want, access.Role)
		})
	}
}

// A missing or malformed overlay id must be reported as a distinct sentinel so the handler can
// answer with the SAME 403 it gives a stranger. Mapping it to 404 would turn the endpoint into
// an overlay-existence oracle for anyone holding any valid token.
func TestResolveOverlayAccess_UnknownOverlayIsIndistinguishable(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("nonexistent overlay", func(t *testing.T) {
		_, err := repo.ResolveOverlayAccess(ctx, uuid.New().String(), ownerID)
		assert.ErrorIs(t, err, ErrOverlayNotFound)
	})

	t.Run("malformed overlay id does not surface a database error", func(t *testing.T) {
		_, err := repo.ResolveOverlayAccess(ctx, "not-a-uuid", ownerID)
		assert.ErrorIs(t, err, ErrOverlayNotFound,
			"a bad id must not reach Postgres and come back as a 500")
	})

	t.Run("malformed caller id does not surface a database error", func(t *testing.T) {
		access, err := repo.ResolveOverlayAccess(ctx, overlayID, "not-a-uuid")
		require.NoError(t, err)
		assert.Equal(t, RoleNone, access.Role)
	})
}
