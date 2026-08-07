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
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Grant-lifecycle failures a handler must distinguish. Everything else is an internal error.
var (
	// ErrModeratorCapReached: the overlay already carries ModeratorsPerOverlayCap live grants.
	// Refused at invite time only; existing grants are never affected by the cap changing.
	ErrModeratorCapReached = errors.New("moderator cap reached for this overlay")
	// ErrGrantNotFound: no live grant with that id on THIS overlay.
	ErrGrantNotFound = errors.New("grant not found")
	// ErrInviteNotFound: the secret matches no outstanding invite. Covers unknown, already
	// redeemed and revoked alike — all three are equally dead, and the holder learns nothing
	// useful from the difference.
	ErrInviteNotFound = errors.New("invite not found")
	// ErrInviteExpired: the invite existed but its 7 days elapsed. Reported distinctly because
	// whoever holds the secret already knows the invite was real, and "expired, ask again" is a
	// far better answer than "not found".
	ErrInviteExpired = errors.New("invite expired")
	// ErrAlreadyModerator: this account already holds a live grant on the overlay.
	ErrAlreadyModerator = errors.New("already a moderator on this overlay")
	// ErrOwnerCannotAccept: the overlay owner tried to redeem their own invite. Their access
	// comes from ownership; a self-grant would only add a confusing second row.
	ErrOwnerCannotAccept = errors.New("the overlay owner cannot accept a delegation")
	// ErrInviteBoundToOtherAccount: the invite names a specific platform account and the
	// redeeming user is not it. The returned InviteDetails carry the expectation so the response
	// can say who the invite is for.
	ErrInviteBoundToOtherAccount = errors.New("invite is bound to another account")
)

// GrantLeg is one platform's enablement on a grant.
type GrantLeg struct {
	Platform string
	Enabled  bool
	// Verification is the last known platform moderator status. TELEMETRY ONLY: never read when
	// authorizing (migration 080's column comment says the same), because caching a denial would
	// make All-Chat the stale authority the design exists to avoid.
	Verification string
	VerifiedAt   *time.Time
}

// Grant is one delegation row. It deliberately carries no invite digest: nothing outside this
// file needs it, so nothing outside this file can leak it.
type Grant struct {
	ID              string
	OverlayID       string
	ModeratorUserID string // empty while the invite is unredeemed
	Status          string
	Actions         []string
	InviteeLabel    string
	// ModeratorDisplayName is captured at accept time so an action stays attributable after the
	// moderator's account is deleted.
	ModeratorDisplayName   string
	ExpectedPlatform       string
	ExpectedPlatformUserID string
	CreatedAt              time.Time
	AcceptedAt             *time.Time
	InviteExpiresAt        *time.Time
	SuspendedAt            *time.Time
	LastActionAt           *time.Time
	Platforms              []GrantLeg
}

// InviteDetails is a grant plus the context an invite holder needs before agreeing to moderate:
// which overlay, and whose.
type InviteDetails struct {
	Grant
	OverlayName      string
	OwnerDisplayName string
}

// InviteParams is everything CreateInvite needs. TokenHash is a digest — the plaintext secret
// never reaches the repository.
type InviteParams struct {
	OverlayID              string
	GrantedBy              string
	Actions                []string
	Platforms              []string
	InviteeLabel           string
	ExpectedPlatform       string
	ExpectedPlatformUserID string
	TokenHash              []byte
	ExpiresAt              time.Time
	// BypassCap skips the per-overlay moderator cap. Admin-only.
	BypassCap bool
}

// CreateInvite mints a pending grant for an unknown recipient.
//
// The whole thing runs in one transaction that first takes a row lock on the overlay, so the cap
// is a real limit rather than a race: two tabs or a double-click cannot both observe nine grants
// and both insert. The lock also proves the overlay exists before anything else happens.
func (r *Repository) CreateInvite(ctx context.Context, p InviteParams) (Grant, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Grant{}, fmt.Errorf("begin invite tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var lockedOverlay string
	err = tx.QueryRow(ctx, `SELECT id::text FROM overlays WHERE id = $1 FOR UPDATE`, p.OverlayID).Scan(&lockedOverlay)
	if errors.Is(err, pgx.ErrNoRows) {
		return Grant{}, ErrOverlayNotFound
	}
	if err != nil {
		return Grant{}, fmt.Errorf("lock overlay for invite: %w", err)
	}

	if !p.BypassCap {
		var live int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM overlay_moderators
			WHERE overlay_id = $1 AND revoked_at IS NULL AND status <> 'revoked'`,
			p.OverlayID).Scan(&live); err != nil {
			return Grant{}, fmt.Errorf("count live grants: %w", err)
		}
		if live >= models.ModeratorsPerOverlayCap {
			return Grant{}, ErrModeratorCapReached
		}
	}

	var grant Grant
	err = tx.QueryRow(ctx, `
		INSERT INTO overlay_moderators
			(overlay_id, granted_by, status, actions, invite_token_hash, invite_expires_at,
			 invitee_label, expected_platform, expected_platform_user_id)
		VALUES ($1, $2, 'pending', $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''))
		RETURNING id::text, created_at`,
		p.OverlayID, p.GrantedBy, p.Actions, p.TokenHash, p.ExpiresAt.UTC(),
		p.InviteeLabel, p.ExpectedPlatform, p.ExpectedPlatformUserID,
	).Scan(&grant.ID, &grant.CreatedAt)
	if err != nil {
		return Grant{}, fmt.Errorf("insert invite: %w", err)
	}

	for _, platform := range p.Platforms {
		if _, err := tx.Exec(ctx, `
			INSERT INTO overlay_moderator_platforms (grant_id, platform, enabled)
			VALUES ($1, $2, TRUE)
			ON CONFLICT (grant_id, platform) DO UPDATE SET enabled = TRUE`,
			grant.ID, platform); err != nil {
			return Grant{}, fmt.Errorf("enable platform leg %q: %w", platform, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Grant{}, fmt.Errorf("commit invite: %w", err)
	}

	grant.OverlayID = p.OverlayID
	grant.Status = models.GrantStatusPending
	grant.Actions = p.Actions
	grant.InviteeLabel = p.InviteeLabel
	grant.ExpectedPlatform = p.ExpectedPlatform
	grant.ExpectedPlatformUserID = p.ExpectedPlatformUserID
	expires := p.ExpiresAt
	grant.InviteExpiresAt = &expires
	for _, platform := range p.Platforms {
		grant.Platforms = append(grant.Platforms, GrantLeg{
			Platform: platform, Enabled: true, Verification: "unverified",
		})
	}
	return grant, nil
}

// grantColumns is the projection shared by every grant read. invite_token_hash is absent on
// purpose: a digest that never leaves this file cannot be logged or serialized by accident.
const grantColumns = `
	m.id::text,
	m.overlay_id::text,
	COALESCE(m.moderator_user_id::text, ''),
	m.status,
	m.actions,
	COALESCE(m.invitee_label, ''),
	COALESCE(m.moderator_display_name, ''),
	COALESCE(m.expected_platform, ''),
	COALESCE(m.expected_platform_user_id, ''),
	m.created_at,
	m.accepted_at,
	m.invite_expires_at,
	m.suspended_at,
	m.last_action_at`

func scanGrant(row pgx.Row) (Grant, error) {
	var g Grant
	err := row.Scan(
		&g.ID, &g.OverlayID, &g.ModeratorUserID, &g.Status, &g.Actions,
		&g.InviteeLabel, &g.ModeratorDisplayName, &g.ExpectedPlatform, &g.ExpectedPlatformUserID,
		&g.CreatedAt, &g.AcceptedAt, &g.InviteExpiresAt, &g.SuspendedAt, &g.LastActionAt,
	)
	return g, err
}

// ListGrants returns the overlay's live delegations — pending invites included, revoked rows
// excluded. Revoked rows stay in the table as history; they are not roster.
func (r *Repository) ListGrants(ctx context.Context, overlayID string) ([]Grant, error) {
	if _, err := uuid.Parse(overlayID); err != nil {
		return nil, ErrOverlayNotFound
	}

	rows, err := r.db.Query(ctx, `
		SELECT`+grantColumns+`
		FROM overlay_moderators m
		WHERE m.overlay_id = $1 AND m.revoked_at IS NULL AND m.status <> 'revoked'
		ORDER BY m.created_at`, overlayID)
	if err != nil {
		return nil, fmt.Errorf("list grants: %w", err)
	}
	defer rows.Close()

	grants := make([]Grant, 0)
	index := map[string]int{}
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			return nil, fmt.Errorf("scan grant: %w", err)
		}
		index[g.ID] = len(grants)
		grants = append(grants, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grants: %w", err)
	}
	if len(grants) == 0 {
		return grants, nil
	}

	legRows, err := r.db.Query(ctx, `
		SELECT p.grant_id::text, p.platform, p.enabled, p.verification, p.verified_at
		FROM overlay_moderator_platforms p
		JOIN overlay_moderators m ON m.id = p.grant_id
		WHERE m.overlay_id = $1 AND m.revoked_at IS NULL AND m.status <> 'revoked'
		ORDER BY p.platform`, overlayID)
	if err != nil {
		return nil, fmt.Errorf("list grant platforms: %w", err)
	}
	defer legRows.Close()

	for legRows.Next() {
		var grantID string
		var leg GrantLeg
		if err := legRows.Scan(&grantID, &leg.Platform, &leg.Enabled, &leg.Verification, &leg.VerifiedAt); err != nil {
			return nil, fmt.Errorf("scan grant platform: %w", err)
		}
		if i, ok := index[grantID]; ok {
			grants[i].Platforms = append(grants[i].Platforms, leg)
		}
	}
	if err := legRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate grant platforms: %w", err)
	}
	return grants, nil
}

// legsFor loads one grant's platform legs.
func (r *Repository) legsFor(ctx context.Context, q pgx.Tx, grantID string) ([]GrantLeg, error) {
	rows, err := q.Query(ctx, `
		SELECT platform, enabled, verification, verified_at
		FROM overlay_moderator_platforms
		WHERE grant_id = $1
		ORDER BY platform`, grantID)
	if err != nil {
		return nil, fmt.Errorf("load grant platforms: %w", err)
	}
	defer rows.Close()

	var legs []GrantLeg
	for rows.Next() {
		var leg GrantLeg
		if err := rows.Scan(&leg.Platform, &leg.Enabled, &leg.Verification, &leg.VerifiedAt); err != nil {
			return nil, fmt.Errorf("scan grant platform: %w", err)
		}
		legs = append(legs, leg)
	}
	return legs, rows.Err()
}

// UpdateGrant narrows or widens a live grant. actions == nil leaves the action set alone; legs ==
// nil leaves platform enablement alone, and a partial map changes only the platforms it names, so
// one toggle in the UI does not have to restate the rest.
//
// The overlay id scopes the write as well as the authorization: a grant id belonging to another
// overlay must be invisible here, or an owner could edit somebody else's team by guessing an id.
func (r *Repository) UpdateGrant(ctx context.Context, overlayID, grantID string, actions []string, legs map[string]bool) (Grant, error) {
	if _, err := uuid.Parse(grantID); err != nil {
		return Grant{}, ErrGrantNotFound
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Grant{}, fmt.Errorf("begin grant update: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var live string
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM overlay_moderators
		WHERE id = $1 AND overlay_id = $2 AND revoked_at IS NULL AND status <> 'revoked'
		FOR UPDATE`, grantID, overlayID).Scan(&live)
	if errors.Is(err, pgx.ErrNoRows) {
		return Grant{}, ErrGrantNotFound
	}
	if err != nil {
		return Grant{}, fmt.Errorf("lock grant: %w", err)
	}

	if actions != nil {
		if _, err := tx.Exec(ctx,
			`UPDATE overlay_moderators SET actions = $2 WHERE id = $1`, grantID, actions); err != nil {
			return Grant{}, fmt.Errorf("update grant actions: %w", err)
		}
	}
	for platform, enabled := range legs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO overlay_moderator_platforms (grant_id, platform, enabled)
			VALUES ($1, $2, $3)
			ON CONFLICT (grant_id, platform) DO UPDATE SET enabled = EXCLUDED.enabled`,
			grantID, platform, enabled); err != nil {
			return Grant{}, fmt.Errorf("set platform leg %q: %w", platform, err)
		}
	}

	updated, err := scanGrant(tx.QueryRow(ctx,
		`SELECT`+grantColumns+` FROM overlay_moderators m WHERE m.id = $1`, grantID))
	if err != nil {
		return Grant{}, fmt.Errorf("reload grant: %w", err)
	}
	updated.Platforms, err = r.legsFor(ctx, tx, grantID)
	if err != nil {
		return Grant{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Grant{}, fmt.Errorf("commit grant update: %w", err)
	}
	return updated, nil
}

// revokeSQL is shared by single and bulk revocation.
//
// Clearing invite_token_hash is the load-bearing part: a revoked invite whose secret is still in
// someone's inbox must be dead, not merely marked. Nulling the digest also frees the partial
// unique index for a future invite.
const revokeSQL = `
	UPDATE overlay_moderators
	SET status = 'revoked', revoked_at = NOW(), revoked_by = $1,
	    invite_token_hash = NULL, invite_expires_at = NULL
	WHERE overlay_id = $2 AND revoked_at IS NULL AND status <> 'revoked'`

// RevokeGrant revokes one grant. It reports false when there was nothing live to revoke — an
// unknown id, another overlay's grant, or a second click — which is not an error.
func (r *Repository) RevokeGrant(ctx context.Context, overlayID, grantID, revokedBy string) (bool, error) {
	if _, err := uuid.Parse(grantID); err != nil {
		return false, nil
	}
	tag, err := r.db.Exec(ctx, revokeSQL+` AND id = $3`, revokedBy, overlayID, grantID)
	if err != nil {
		return false, fmt.Errorf("revoke grant: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// RevokeAllGrants is the kill switch: every live delegation on the overlay goes, unredeemed
// invites included. Returns how many were revoked.
func (r *Repository) RevokeAllGrants(ctx context.Context, overlayID, revokedBy string) (int, error) {
	if _, err := uuid.Parse(overlayID); err != nil {
		return 0, ErrOverlayNotFound
	}
	tag, err := r.db.Exec(ctx, revokeSQL, revokedBy, overlayID)
	if err != nil {
		return 0, fmt.Errorf("revoke all grants: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// inviteLookupSQL finds an outstanding invite by digest, together with the overlay and owner an
// invite holder is entitled to see before agreeing to anything.
const inviteLookupSQL = `
	SELECT` + grantColumns + `,
	       o.name, u.display_name, o.user_id::text
	FROM overlay_moderators m
	JOIN overlays o ON o.id = m.overlay_id
	JOIN users u ON u.id = o.user_id
	WHERE m.invite_token_hash = $1 AND m.revoked_at IS NULL AND m.status = 'pending'`

// scanInvite reads inviteLookupSQL's projection.
func scanInvite(row pgx.Row) (InviteDetails, string, error) {
	var d InviteDetails
	var ownerUserID string
	err := row.Scan(
		&d.ID, &d.OverlayID, &d.ModeratorUserID, &d.Status, &d.Actions,
		&d.InviteeLabel, &d.ModeratorDisplayName, &d.ExpectedPlatform, &d.ExpectedPlatformUserID,
		&d.CreatedAt, &d.AcceptedAt, &d.InviteExpiresAt, &d.SuspendedAt, &d.LastActionAt,
		&d.OverlayName, &d.OwnerDisplayName, &ownerUserID,
	)
	return d, ownerUserID, err
}

// expired reports whether an invite's window has closed. A NULL expiry is treated as expired
// rather than eternal: an invite with no deadline is a permanent unattended key.
//
// invite_expires_at is a naive TIMESTAMP, which pgx reads back as UTC. That is only comparable to
// a real instant because CreateInvite writes the deadline in UTC — writing a local wall clock
// would come back offset by the writer's zone, and an already-expired invite would read as valid.
func expired(d InviteDetails) bool {
	return d.InviteExpiresAt == nil || d.InviteExpiresAt.Before(time.Now().UTC())
}

// PreviewInvite reads an outstanding invite without redeeming it, so the invitee can see who is
// asking and what for. It performs no writes at all.
func (r *Repository) PreviewInvite(ctx context.Context, tokenHash []byte) (InviteDetails, error) {
	details, _, err := scanInvite(r.db.QueryRow(ctx, inviteLookupSQL, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return InviteDetails{}, ErrInviteNotFound
	}
	if err != nil {
		return InviteDetails{}, fmt.Errorf("preview invite: %w", err)
	}
	if expired(details) {
		return InviteDetails{}, ErrInviteExpired
	}

	legs, err := r.db.Query(ctx, `
		SELECT platform, enabled, verification, verified_at
		FROM overlay_moderator_platforms WHERE grant_id = $1 ORDER BY platform`, details.ID)
	if err != nil {
		return InviteDetails{}, fmt.Errorf("preview invite platforms: %w", err)
	}
	defer legs.Close()
	for legs.Next() {
		var leg GrantLeg
		if err := legs.Scan(&leg.Platform, &leg.Enabled, &leg.Verification, &leg.VerifiedAt); err != nil {
			return InviteDetails{}, fmt.Errorf("scan invite platform: %w", err)
		}
		details.Platforms = append(details.Platforms, leg)
	}
	if err := legs.Err(); err != nil {
		return InviteDetails{}, fmt.Errorf("iterate invite platforms: %w", err)
	}
	return details, nil
}

// AcceptInvite redeems an invite for userID: it binds the grant to that account, activates it, and
// burns the secret.
//
// Burning means nulling the digest, so the row can never be redeemed again and a database leak
// yields no usable invite. The moderator's display name is read from their own user row rather
// than taken from a claim, so the owner's roster shows what All-Chat actually knows about them.
func (r *Repository) AcceptInvite(ctx context.Context, tokenHash []byte, userID string) (InviteDetails, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return InviteDetails{}, ErrInviteNotFound
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return InviteDetails{}, fmt.Errorf("begin accept tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	details, ownerUserID, err := scanInvite(tx.QueryRow(ctx, inviteLookupSQL+` FOR UPDATE OF m`, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return InviteDetails{}, ErrInviteNotFound
	}
	if err != nil {
		return InviteDetails{}, fmt.Errorf("load invite: %w", err)
	}
	if expired(details) {
		return InviteDetails{}, ErrInviteExpired
	}
	if ownerUserID == userID {
		return InviteDetails{}, ErrOwnerCannotAccept
	}

	// A pre-bound invite names one platform account. Fail closed: if we cannot prove the
	// redeeming user is that account, the invite is not theirs — the details go back with the
	// error so the response can say which account it expects.
	if details.ExpectedPlatform != "" {
		matches, err := identityMatches(ctx, tx, userID, details.ExpectedPlatform, details.ExpectedPlatformUserID)
		if err != nil {
			return InviteDetails{}, err
		}
		if !matches {
			return details, ErrInviteBoundToOtherAccount
		}
	}

	var alreadyModerates bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM overlay_moderators
			WHERE overlay_id = $1 AND moderator_user_id = $2 AND revoked_at IS NULL
		)`, details.OverlayID, userID).Scan(&alreadyModerates); err != nil {
		return InviteDetails{}, fmt.Errorf("check existing grant: %w", err)
	}
	if alreadyModerates {
		return InviteDetails{}, ErrAlreadyModerator
	}

	err = tx.QueryRow(ctx, `
		UPDATE overlay_moderators m
		SET moderator_user_id = u.id,
		    moderator_display_name = LEFT(u.display_name, 120),
		    status = 'active',
		    accepted_at = NOW(),
		    invite_token_hash = NULL,
		    invite_expires_at = NULL
		FROM users u
		WHERE m.id = $1 AND u.id = $2
		RETURNING m.moderator_user_id::text, m.moderator_display_name, m.accepted_at`,
		details.ID, userID,
	).Scan(&details.ModeratorUserID, &details.ModeratorDisplayName, &details.AcceptedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// The join found no user row, so the account vanished between authentication and here.
		return InviteDetails{}, ErrInviteNotFound
	}
	// The partial unique index is the last line of defence against two concurrent accepts.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return InviteDetails{}, ErrAlreadyModerator
	}
	if err != nil {
		return InviteDetails{}, fmt.Errorf("accept invite: %w", err)
	}

	details.Status = models.GrantStatusActive
	details.InviteExpiresAt = nil
	details.Platforms, err = r.legsFor(ctx, tx, details.ID)
	if err != nil {
		return InviteDetails{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return InviteDetails{}, fmt.Errorf("commit accept: %w", err)
	}
	return details, nil
}

// identityMatches reports whether userID demonstrably owns platformUserID on platform.
//
// Only Twitch is answerable today (models.PreBindablePlatform gates what may be stored), and it is
// answered from three places, any of which is proof: the account's own Twitch identity, a linked
// Twitch credential (ADR-0016), or a moderator credential from an earlier consent. Anything else
// is a no, never an error — an unverifiable binding must not become an accidental allow.
func identityMatches(ctx context.Context, tx pgx.Tx, userID, platform, platformUserID string) (bool, error) {
	if platform != "twitch" || platformUserID == "" {
		return false, nil
	}
	const query = `
		SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND twitch_id = $2)
		    OR EXISTS(SELECT 1 FROM twitch_oauth_tokens WHERE user_id = $1 AND twitch_user_id = $2)
		    OR EXISTS(SELECT 1 FROM mod_oauth_credentials
		              WHERE user_id = $1 AND platform = 'twitch' AND platform_user_id = $2)`
	var matches bool
	if err := tx.QueryRow(ctx, query, userID, platformUserID).Scan(&matches); err != nil {
		return false, fmt.Errorf("verify invite binding: %w", err)
	}
	return matches, nil
}

// TouchGrantActivity stamps a grant's last successful action.
//
// This drives the 90-day dormancy suspension, and it is written from the first delegated action
// onwards so a later dormancy job cannot mistake a working mod team for an idle one. An empty
// grant id (an owner acting on their own overlay) is a no-op, not a failure.
func (r *Repository) TouchGrantActivity(ctx context.Context, grantID string) error {
	if _, err := uuid.Parse(grantID); err != nil {
		return nil
	}
	if _, err := r.db.Exec(ctx,
		`UPDATE overlay_moderators SET last_action_at = NOW() WHERE id = $1`, grantID); err != nil {
		return fmt.Errorf("touch grant activity: %w", err)
	}
	return nil
}
