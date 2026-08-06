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
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/invites"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newModerator inserts a user who can redeem an invite, and returns their id.
func newModerator(t *testing.T, r *Repository, displayName string) string {
	t.Helper()
	id := uuid.New().String()
	_, err := r.db.Exec(context.Background(),
		`INSERT INTO users (id, is_premium, display_name) VALUES ($1, false, $2)`, id, displayName)
	require.NoError(t, err)
	return id
}

// inviteFor mints a secret and stores its hash, returning the secret the way a streamer would
// receive it — the repository never sees the plaintext.
func inviteFor(t *testing.T, r *Repository, p InviteParams) (Grant, string) {
	t.Helper()
	secret, err := invites.NewSecret()
	require.NoError(t, err)
	if p.OverlayID == "" {
		p.OverlayID = overlayID
	}
	if p.GrantedBy == "" {
		p.GrantedBy = ownerID
	}
	if p.Actions == nil {
		p.Actions = []string{"delete", "timeout"}
	}
	p.TokenHash = invites.Hash(secret)
	if p.ExpiresAt.IsZero() {
		p.ExpiresAt = time.Now().Add(invites.TTL)
	}
	grant, err := r.CreateInvite(context.Background(), p)
	require.NoError(t, err)
	return grant, secret
}

func TestCreateInvite_StoresAPendingGrantWithNoAccountBound(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	grant, secret := inviteFor(t, repo, InviteParams{
		Actions:      []string{"delete", "timeout", "ban"},
		Platforms:    []string{"twitch"},
		InviteeLabel: "Sarah",
	})

	assert.NotEmpty(t, grant.ID)
	assert.Equal(t, "pending", grant.Status)
	assert.Empty(t, grant.ModeratorUserID, "an invite is bound to nobody until it is redeemed")
	assert.Equal(t, []string{"delete", "timeout", "ban"}, grant.Actions)
	assert.Equal(t, "Sarah", grant.InviteeLabel)
	require.NotNil(t, grant.InviteExpiresAt)

	// The enabled leg is stored; every other platform stays absent, which is disablement.
	require.Len(t, grant.Platforms, 1)
	assert.Equal(t, "twitch", grant.Platforms[0].Platform)
	assert.True(t, grant.Platforms[0].Enabled)
	assert.Equal(t, "unverified", grant.Platforms[0].Verification)

	// Only the digest is persisted; the plaintext secret exists nowhere in the database.
	var stored []byte
	require.NoError(t, repo.db.QueryRow(ctx,
		`SELECT invite_token_hash FROM overlay_moderators WHERE id = $1`, grant.ID).Scan(&stored))
	assert.Equal(t, invites.Hash(secret), stored)
	assert.NotContains(t, string(stored), secret)
}

// A pending invite still occupies a seat: otherwise minting invites would be an unbounded way
// around the cap.
func TestCreateInvite_CapCountsPendingInvitesAndIsRefusedOnTheEleventh(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		inviteFor(t, repo, InviteParams{})
	}

	secret, err := invites.NewSecret()
	require.NoError(t, err)
	_, err = repo.CreateInvite(ctx, InviteParams{
		OverlayID: overlayID, GrantedBy: ownerID, Actions: []string{"delete"},
		TokenHash: invites.Hash(secret), ExpiresAt: time.Now().Add(invites.TTL),
	})
	assert.ErrorIs(t, err, ErrModeratorCapReached)

	t.Run("an admin override still gets through", func(t *testing.T) {
		override, err := invites.NewSecret()
		require.NoError(t, err)
		grant, err := repo.CreateInvite(ctx, InviteParams{
			OverlayID: overlayID, GrantedBy: ownerID, Actions: []string{"delete"},
			TokenHash: invites.Hash(override), ExpiresAt: time.Now().Add(invites.TTL),
			BypassCap: true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, grant.ID)
	})
}

func TestCreateInvite_RevokedGrantsFreeTheirSeat(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	var first Grant
	for i := 0; i < 10; i++ {
		g, _ := inviteFor(t, repo, InviteParams{})
		if i == 0 {
			first = g
		}
	}
	revoked, err := repo.RevokeGrant(ctx, overlayID, first.ID, ownerID)
	require.NoError(t, err)
	require.True(t, revoked)

	_, err = repo.CreateInvite(ctx, InviteParams{
		OverlayID: overlayID, GrantedBy: ownerID, Actions: []string{"delete"},
		TokenHash: invites.Hash("free-seat"), ExpiresAt: time.Now().Add(invites.TTL),
	})
	assert.NoError(t, err, "revoking a moderator must make room for another")
}

// The cap is only a control if it holds when the streamer double-clicks, or two browser tabs race.
// Counting without serialising on the overlay would let both transactions see nine and insert.
func TestCreateInvite_CapHoldsUnderConcurrentInvites(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	const attempts = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
		capErrors int
	)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			secret, err := invites.NewSecret()
			if err != nil {
				return
			}
			_, err = repo.CreateInvite(ctx, InviteParams{
				OverlayID: overlayID, GrantedBy: ownerID, Actions: []string{"delete"},
				TokenHash: invites.Hash(secret), ExpiresAt: time.Now().Add(invites.TTL),
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				succeeded++
			case err == ErrModeratorCapReached:
				capErrors++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 10, succeeded, "exactly the cap may be created, however many callers race")
	assert.Equal(t, attempts-10, capErrors)

	var live int
	require.NoError(t, repo.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM overlay_moderators WHERE overlay_id = $1 AND revoked_at IS NULL`,
		overlayID).Scan(&live))
	assert.Equal(t, 10, live)
}

func TestAcceptInvite_BindsTheGrantAndBurnsTheSecret(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	grant, secret := inviteFor(t, repo, InviteParams{Platforms: []string{"twitch", "kick"}})
	modID := newModerator(t, repo, "Sarah Example")

	accepted, err := repo.AcceptInvite(ctx, invites.Hash(secret), modID)
	require.NoError(t, err)
	assert.Equal(t, grant.ID, accepted.ID)
	assert.Equal(t, "active", accepted.Status)
	assert.Equal(t, modID, accepted.ModeratorUserID)
	assert.Equal(t, "Sarah Example", accepted.ModeratorDisplayName,
		"the name is captured now so the owner's log can still name them after the account is gone")
	assert.Equal(t, "My Overlay", accepted.OverlayName)
	assert.Equal(t, "The Streamer", accepted.OwnerDisplayName)
	assert.Len(t, accepted.Platforms, 2)
	require.NotNil(t, accepted.AcceptedAt)

	t.Run("the same secret cannot be redeemed twice", func(t *testing.T) {
		other := newModerator(t, repo, "Someone Else")
		_, err := repo.AcceptInvite(ctx, invites.Hash(secret), other)
		assert.ErrorIs(t, err, ErrInviteNotFound)
	})

	t.Run("the hash is cleared, so a database leak yields no live invite", func(t *testing.T) {
		var hash []byte
		require.NoError(t, repo.db.QueryRow(ctx,
			`SELECT invite_token_hash FROM overlay_moderators WHERE id = $1`, grant.ID).Scan(&hash))
		assert.Nil(t, hash)
	})
}

func TestAcceptInvite_Refusals(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("an unknown secret", func(t *testing.T) {
		_, err := repo.AcceptInvite(ctx, invites.Hash("never-issued"), newModerator(t, repo, "X"))
		assert.ErrorIs(t, err, ErrInviteNotFound)
	})

	t.Run("an expired invite reports expiry, which the holder of the secret may know", func(t *testing.T) {
		_, secret := inviteFor(t, repo, InviteParams{ExpiresAt: time.Now().Add(-time.Minute)})
		_, err := repo.AcceptInvite(ctx, invites.Hash(secret), newModerator(t, repo, "Late"))
		assert.ErrorIs(t, err, ErrInviteExpired)
	})

	t.Run("the overlay owner cannot delegate to themselves", func(t *testing.T) {
		_, secret := inviteFor(t, repo, InviteParams{})
		_, err := repo.AcceptInvite(ctx, invites.Hash(secret), ownerID)
		assert.ErrorIs(t, err, ErrOwnerCannotAccept)
	})

	t.Run("someone who already moderates this overlay", func(t *testing.T) {
		_, first := inviteFor(t, repo, InviteParams{})
		modID := newModerator(t, repo, "Twice")
		_, err := repo.AcceptInvite(ctx, invites.Hash(first), modID)
		require.NoError(t, err)

		_, second := inviteFor(t, repo, InviteParams{})
		_, err = repo.AcceptInvite(ctx, invites.Hash(second), modID)
		assert.ErrorIs(t, err, ErrAlreadyModerator)
	})

	// A revoked invite must be dead even though the secret is still in someone's inbox.
	t.Run("a revoked invite", func(t *testing.T) {
		grant, secret := inviteFor(t, repo, InviteParams{})
		_, err := repo.RevokeGrant(ctx, overlayID, grant.ID, ownerID)
		require.NoError(t, err)
		_, err = repo.AcceptInvite(ctx, invites.Hash(secret), newModerator(t, repo, "Too late"))
		assert.ErrorIs(t, err, ErrInviteNotFound)
	})
}

// A pre-bound invite is a promise that only one account can redeem it. If acceptance ignored the
// binding, the "pick from your Twitch moderators" flow would hand the channel to whoever opened
// the link first.
func TestAcceptInvite_PreBinding(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	bound := InviteParams{ExpectedPlatform: "twitch", ExpectedPlatformUserID: "77777", InviteeLabel: "@sarah"}

	t.Run("the account named in the invite is accepted via their All-Chat identity", func(t *testing.T) {
		_, secret := inviteFor(t, repo, bound)
		modID := newModerator(t, repo, "Sarah")
		_, err := repo.db.Exec(ctx, `UPDATE users SET twitch_id = '77777' WHERE id = $1`, modID)
		require.NoError(t, err)

		accepted, err := repo.AcceptInvite(ctx, invites.Hash(secret), modID)
		require.NoError(t, err)
		assert.Equal(t, "active", accepted.Status)
	})

	t.Run("a linked Twitch credential also proves the identity", func(t *testing.T) {
		_, secret := inviteFor(t, repo, bound)
		modID := newModerator(t, repo, "Sarah linked")
		_, err := repo.db.Exec(ctx,
			`INSERT INTO twitch_oauth_tokens (user_id, twitch_user_id, twitch_login) VALUES ($1, '77777', 'sarah')`,
			modID)
		require.NoError(t, err)

		_, err = repo.AcceptInvite(ctx, invites.Hash(secret), modID)
		assert.NoError(t, err)
	})

	t.Run("a moderator credential from an earlier consent also proves it", func(t *testing.T) {
		_, secret := inviteFor(t, repo, bound)
		modID := newModerator(t, repo, "Sarah consented")
		_, err := repo.db.Exec(ctx,
			`INSERT INTO mod_oauth_credentials (user_id, platform, platform_user_id, access_token)
			 VALUES ($1, 'twitch', '77777', 'enc')`, modID)
		require.NoError(t, err)

		_, err = repo.AcceptInvite(ctx, invites.Hash(secret), modID)
		assert.NoError(t, err)
	})

	t.Run("the wrong account is refused, and told who the invite is for", func(t *testing.T) {
		_, secret := inviteFor(t, repo, bound)
		bob := newModerator(t, repo, "Bob")
		_, err := repo.db.Exec(ctx, `UPDATE users SET twitch_id = '12345' WHERE id = $1`, bob)
		require.NoError(t, err)

		details, err := repo.AcceptInvite(ctx, invites.Hash(secret), bob)
		assert.ErrorIs(t, err, ErrInviteBoundToOtherAccount)
		assert.Equal(t, "@sarah", details.InviteeLabel,
			"the refusal must carry enough to say which account the invite expects")
		assert.Equal(t, "twitch", details.ExpectedPlatform)
	})

	// Fail closed: an account with no Twitch identity at all cannot satisfy a Twitch binding.
	t.Run("an account with no identity on that platform is refused", func(t *testing.T) {
		_, secret := inviteFor(t, repo, bound)
		_, err := repo.AcceptInvite(ctx, invites.Hash(secret), newModerator(t, repo, "No twitch"))
		assert.ErrorIs(t, err, ErrInviteBoundToOtherAccount)
	})
}

func TestListGrants(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	pending, _ := inviteFor(t, repo, InviteParams{InviteeLabel: "Outstanding"})
	activeGrant, secret := inviteFor(t, repo, InviteParams{Platforms: []string{"twitch"}})
	modID := newModerator(t, repo, "Active Mod")
	_, err := repo.AcceptInvite(ctx, invites.Hash(secret), modID)
	require.NoError(t, err)
	gone, _ := inviteFor(t, repo, InviteParams{})
	_, err = repo.RevokeGrant(ctx, overlayID, gone.ID, ownerID)
	require.NoError(t, err)

	grants, err := repo.ListGrants(ctx, overlayID)
	require.NoError(t, err)

	byID := map[string]Grant{}
	for _, g := range grants {
		byID[g.ID] = g
	}
	assert.Len(t, grants, 2, "revoked grants are history, not roster")
	assert.NotContains(t, byID, gone.ID)

	assert.Equal(t, "pending", byID[pending.ID].Status)
	assert.Equal(t, "Outstanding", byID[pending.ID].InviteeLabel)
	require.NotNil(t, byID[pending.ID].InviteExpiresAt, "an outstanding invite still has a deadline")

	live := byID[activeGrant.ID]
	assert.Equal(t, "active", live.Status)
	assert.Equal(t, "Active Mod", live.ModeratorDisplayName)
	assert.Equal(t, modID, live.ModeratorUserID)
	require.Len(t, live.Platforms, 1)
	assert.Equal(t, "twitch", live.Platforms[0].Platform)

	t.Run("another overlay's grants are not listed", func(t *testing.T) {
		otherOverlay := uuid.New().String()
		_, err := repo.db.Exec(ctx, `INSERT INTO overlays (id, user_id, name) VALUES ($1, $2, 'Other')`,
			otherOverlay, ownerID)
		require.NoError(t, err)
		inviteFor(t, repo, InviteParams{OverlayID: otherOverlay})

		grants, err := repo.ListGrants(ctx, overlayID)
		require.NoError(t, err)
		assert.Len(t, grants, 2)
	})
}

func TestUpdateGrant(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	grant, _ := inviteFor(t, repo, InviteParams{Actions: []string{"delete"}, Platforms: []string{"twitch"}})

	t.Run("actions are replaced, not merged", func(t *testing.T) {
		updated, err := repo.UpdateGrant(ctx, overlayID, grant.ID, []string{"timeout"}, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"timeout"}, updated.Actions,
			"narrowing a grant must actually remove what was taken away")
	})

	t.Run("a leg can be enabled and disabled", func(t *testing.T) {
		updated, err := repo.UpdateGrant(ctx, overlayID, grant.ID, nil, map[string]bool{"kick": true})
		require.NoError(t, err)
		legs := legsByPlatform(updated)
		assert.True(t, legs["kick"].Enabled)
		assert.True(t, legs["twitch"].Enabled, "an unmentioned platform is left alone")

		updated, err = repo.UpdateGrant(ctx, overlayID, grant.ID, nil, map[string]bool{"twitch": false})
		require.NoError(t, err)
		legs = legsByPlatform(updated)
		assert.False(t, legs["twitch"].Enabled)
		assert.True(t, legs["kick"].Enabled)
	})

	t.Run("nil arguments leave the grant untouched", func(t *testing.T) {
		before, err := repo.UpdateGrant(ctx, overlayID, grant.ID, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"timeout"}, before.Actions)
	})

	// The overlay id in the path is part of the authorization decision, so it must also scope the
	// write: otherwise an owner could edit a grant on somebody else's overlay by guessing its id.
	t.Run("a grant on another overlay is invisible", func(t *testing.T) {
		otherOverlay := uuid.New().String()
		_, err := repo.db.Exec(ctx, `INSERT INTO overlays (id, user_id, name) VALUES ($1, $2, 'Other')`,
			otherOverlay, ownerID)
		require.NoError(t, err)
		foreign, _ := inviteFor(t, repo, InviteParams{OverlayID: otherOverlay})

		_, err = repo.UpdateGrant(ctx, overlayID, foreign.ID, []string{"ban"}, nil)
		assert.ErrorIs(t, err, ErrGrantNotFound)
	})

	t.Run("an unknown or malformed grant id", func(t *testing.T) {
		_, err := repo.UpdateGrant(ctx, overlayID, uuid.New().String(), []string{"ban"}, nil)
		assert.ErrorIs(t, err, ErrGrantNotFound)
		_, err = repo.UpdateGrant(ctx, overlayID, "not-a-uuid", []string{"ban"}, nil)
		assert.ErrorIs(t, err, ErrGrantNotFound, "a bad id must not surface as a 500")
	})

	t.Run("a revoked grant cannot be edited back to life", func(t *testing.T) {
		revoked, _ := inviteFor(t, repo, InviteParams{})
		_, err := repo.RevokeGrant(ctx, overlayID, revoked.ID, ownerID)
		require.NoError(t, err)
		_, err = repo.UpdateGrant(ctx, overlayID, revoked.ID, []string{"ban"}, nil)
		assert.ErrorIs(t, err, ErrGrantNotFound)
	})
}

func TestRevokeGrant(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	grant, secret := inviteFor(t, repo, InviteParams{})

	revoked, err := repo.RevokeGrant(ctx, overlayID, grant.ID, adminActor)
	require.NoError(t, err)
	assert.True(t, revoked)

	var status string
	var revokedBy *string
	var hash []byte
	require.NoError(t, repo.db.QueryRow(ctx,
		`SELECT status, revoked_by::text, invite_token_hash FROM overlay_moderators WHERE id = $1`,
		grant.ID).Scan(&status, &revokedBy, &hash))
	assert.Equal(t, "revoked", status)
	require.NotNil(t, revokedBy)
	assert.Equal(t, adminActor, *revokedBy, "who revoked is part of the trail")
	assert.Nil(t, hash, "revoking an outstanding invite must kill the secret already sent out")

	_, err = repo.AcceptInvite(ctx, invites.Hash(secret), newModerator(t, repo, "Denied"))
	assert.ErrorIs(t, err, ErrInviteNotFound)

	t.Run("revoking twice is not an error but changes nothing", func(t *testing.T) {
		again, err := repo.RevokeGrant(ctx, overlayID, grant.ID, ownerID)
		require.NoError(t, err)
		assert.False(t, again)
	})

	t.Run("a grant on another overlay is untouched", func(t *testing.T) {
		otherOverlay := uuid.New().String()
		_, err := repo.db.Exec(ctx, `INSERT INTO overlays (id, user_id, name) VALUES ($1, $2, 'Other')`,
			otherOverlay, ownerID)
		require.NoError(t, err)
		foreign, _ := inviteFor(t, repo, InviteParams{OverlayID: otherOverlay})

		ok, err := repo.RevokeGrant(ctx, overlayID, foreign.ID, ownerID)
		require.NoError(t, err)
		assert.False(t, ok)

		var stillLive string
		require.NoError(t, repo.db.QueryRow(ctx,
			`SELECT status FROM overlay_moderators WHERE id = $1`, foreign.ID).Scan(&stillLive))
		assert.Equal(t, "pending", stillLive)
	})

	t.Run("a malformed grant id is a miss, not a database error", func(t *testing.T) {
		ok, err := repo.RevokeGrant(ctx, overlayID, "not-a-uuid", ownerID)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

// The kill switch: one call, everyone off, including invites nobody has redeemed yet.
func TestRevokeAllGrants(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	_, outstanding := inviteFor(t, repo, InviteParams{})
	_, toAccept := inviteFor(t, repo, InviteParams{})
	modID := newModerator(t, repo, "Working Mod")
	_, err := repo.AcceptInvite(ctx, invites.Hash(toAccept), modID)
	require.NoError(t, err)

	otherOverlay := uuid.New().String()
	_, err = repo.db.Exec(ctx, `INSERT INTO overlays (id, user_id, name) VALUES ($1, $2, 'Other')`,
		otherOverlay, ownerID)
	require.NoError(t, err)
	untouched, _ := inviteFor(t, repo, InviteParams{OverlayID: otherOverlay})

	count, err := repo.RevokeAllGrants(ctx, overlayID, ownerID)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	grants, err := repo.ListGrants(ctx, overlayID)
	require.NoError(t, err)
	assert.Empty(t, grants)

	_, err = repo.AcceptInvite(ctx, invites.Hash(outstanding), newModerator(t, repo, "Nope"))
	assert.ErrorIs(t, err, ErrInviteNotFound, "the kill switch must also kill unredeemed invites")

	access, err := repo.ResolveOverlayAccess(ctx, overlayID, modID)
	require.NoError(t, err)
	assert.Equal(t, RoleNone, access.Role, "revocation takes effect on the very next request")

	var foreignStatus string
	require.NoError(t, repo.db.QueryRow(ctx,
		`SELECT status FROM overlay_moderators WHERE id = $1`, untouched.ID).Scan(&foreignStatus))
	assert.Equal(t, "pending", foreignStatus, "another overlay's team must survive")

	t.Run("a second sweep reports nothing left", func(t *testing.T) {
		count, err := repo.RevokeAllGrants(ctx, overlayID, ownerID)
		require.NoError(t, err)
		assert.Zero(t, count)
	})
}

func TestPreviewInvite(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	_, secret := inviteFor(t, repo, InviteParams{
		Actions:      []string{"delete", "ban"},
		Platforms:    []string{"twitch"},
		InviteeLabel: "@sarah",
	})

	preview, err := repo.PreviewInvite(ctx, invites.Hash(secret))
	require.NoError(t, err)
	assert.Equal(t, "My Overlay", preview.OverlayName)
	assert.Equal(t, "The Streamer", preview.OwnerDisplayName,
		"an invite must say who is asking before anyone agrees to work for them")
	assert.Equal(t, []string{"delete", "ban"}, preview.Actions)
	require.Len(t, preview.Platforms, 1)
	assert.Equal(t, "@sarah", preview.InviteeLabel)

	t.Run("previewing does not redeem", func(t *testing.T) {
		_, err := repo.PreviewInvite(ctx, invites.Hash(secret))
		assert.NoError(t, err)
		modID := newModerator(t, repo, "Sarah")
		_, err = repo.AcceptInvite(ctx, invites.Hash(secret), modID)
		assert.NoError(t, err)
	})

	t.Run("an unknown secret", func(t *testing.T) {
		_, err := repo.PreviewInvite(ctx, invites.Hash("nope"))
		assert.ErrorIs(t, err, ErrInviteNotFound)
	})

	t.Run("an expired invite", func(t *testing.T) {
		_, expired := inviteFor(t, repo, InviteParams{ExpiresAt: time.Now().Add(-time.Hour)})
		_, err := repo.PreviewInvite(ctx, invites.Hash(expired))
		assert.ErrorIs(t, err, ErrInviteExpired)
	})
}

// last_action_at drives the 90-day dormancy suspension. Stamping it from the first delegated
// action onwards is what keeps a later dormancy job from reading a working mod team as idle.
func TestTouchGrantActivity(t *testing.T) {
	repo, cleanup := setupTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	grant, secret := inviteFor(t, repo, InviteParams{})
	modID := newModerator(t, repo, "Busy")
	_, err := repo.AcceptInvite(ctx, invites.Hash(secret), modID)
	require.NoError(t, err)

	require.NoError(t, repo.TouchGrantActivity(ctx, grant.ID))

	var lastAction *time.Time
	require.NoError(t, repo.db.QueryRow(ctx,
		`SELECT last_action_at FROM overlay_moderators WHERE id = $1`, grant.ID).Scan(&lastAction))
	require.NotNil(t, lastAction)
	assert.WithinDuration(t, time.Now(), *lastAction, time.Minute)

	t.Run("an unknown or malformed grant id is not an error", func(t *testing.T) {
		assert.NoError(t, repo.TouchGrantActivity(ctx, uuid.New().String()))
		assert.NoError(t, repo.TouchGrantActivity(ctx, ""),
			"an owner action carries no grant id and must not log a spurious failure")
	})
}

func legsByPlatform(g Grant) map[string]GrantLeg {
	out := make(map[string]GrantLeg, len(g.Platforms))
	for _, leg := range g.Platforms {
		out[leg.Platform] = leg
	}
	return out
}
