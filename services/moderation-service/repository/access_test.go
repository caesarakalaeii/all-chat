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
