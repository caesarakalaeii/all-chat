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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Delegation is one overlay a moderator holds a grant on, from the MODERATOR's side.
//
// It carries the owner's identity because everything the moderator is shown about entitlement is
// keyed on the owner, never on themselves (ADR-0048).
type Delegation struct {
	GrantID          string
	OverlayID        string
	OverlayName      string
	OwnerUserID      string
	OwnerDisplayName string
	Status           string
	Actions          []string
	AcceptedAt       *time.Time
	LastActionAt     *time.Time
	Platforms        []GrantLeg
}

// ListDelegationsFor returns the overlays this user may moderate for someone else.
//
// Suspended grants are included, not filtered out. A dormancy-suspended grant that simply vanished
// from the list would look like a revocation to the moderator and give them nothing to act on; it
// is listed with its status so the UI can say "ask the streamer to reactivate this". Only `active`
// authorizes anything — ResolveOverlayAccess is the authority there, and it ignores every other
// status.
//
// Pending invites cannot appear: an unredeemed grant has no moderator_user_id yet.
func (r *Repository) ListDelegationsFor(ctx context.Context, moderatorUserID string) ([]Delegation, error) {
	// A caller id that is not a UUID cannot match a grant. Normalise rather than letting the cast
	// fail and surface as a 500.
	if _, err := uuid.Parse(moderatorUserID); err != nil {
		return []Delegation{}, nil
	}

	// Deliberately not restricted to status = 'active', so this does not use the partial index
	// idx_overlay_moderators_by_mod. That index still serves the authorization path; here the
	// row count is bounded by the per-overlay cap times the handful of streamers one volunteer
	// works for, so the scan is not worth a second index.
	const query = `
		SELECT m.id::text,
		       m.overlay_id::text,
		       o.name,
		       o.user_id::text,
		       COALESCE(NULLIF(u.display_name, ''), u.username),
		       m.status,
		       m.actions,
		       m.accepted_at,
		       m.last_action_at
		FROM overlay_moderators m
		JOIN overlays o ON o.id = m.overlay_id
		JOIN users u    ON u.id = o.user_id
		WHERE m.moderator_user_id = $1
		  AND m.revoked_at IS NULL
		  AND m.status IN ('active', 'suspended')
		ORDER BY o.name, m.id`

	rows, err := r.db.Query(ctx, query, moderatorUserID)
	if err != nil {
		return nil, fmt.Errorf("list delegations: %w", err)
	}
	defer rows.Close()

	delegations := make([]Delegation, 0)
	index := map[string]int{}
	for rows.Next() {
		var d Delegation
		if err := rows.Scan(
			&d.GrantID, &d.OverlayID, &d.OverlayName, &d.OwnerUserID, &d.OwnerDisplayName,
			&d.Status, &d.Actions, &d.AcceptedAt, &d.LastActionAt,
		); err != nil {
			return nil, fmt.Errorf("scan delegation: %w", err)
		}
		index[d.GrantID] = len(delegations)
		delegations = append(delegations, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delegations: %w", err)
	}
	if len(delegations) == 0 {
		return delegations, nil
	}

	legRows, err := r.db.Query(ctx, `
		SELECT p.grant_id::text, p.platform, p.enabled, p.verification, p.verified_at
		FROM overlay_moderator_platforms p
		JOIN overlay_moderators m ON m.id = p.grant_id
		WHERE m.moderator_user_id = $1
		  AND m.revoked_at IS NULL
		  AND m.status IN ('active', 'suspended')
		ORDER BY p.platform`, moderatorUserID)
	if err != nil {
		return nil, fmt.Errorf("list delegation platforms: %w", err)
	}
	defer legRows.Close()

	for legRows.Next() {
		var grantID string
		var leg GrantLeg
		if err := legRows.Scan(&grantID, &leg.Platform, &leg.Enabled, &leg.Verification, &leg.VerifiedAt); err != nil {
			return nil, fmt.Errorf("scan delegation platform: %w", err)
		}
		if i, ok := index[grantID]; ok {
			delegations[i].Platforms = append(delegations[i].Platforms, leg)
		}
	}
	if err := legRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delegation platforms: %w", err)
	}
	return delegations, nil
}

// ModeratorGrantedScopes returns the scopes on the moderator's OWN credential for a platform, and
// whether such a credential exists at all.
//
// Only granted_scopes is read — never the token — because the capability answer needs no
// decryption and nothing here should be able to leak a credential. Reading from
// mod_oauth_credentials rather than the per-channel token tables is the whole point of that table
// (ADR-0048): the moderator's authority comes from their own account, never from the streamer's
// stored credential.
func (r *Repository) ModeratorGrantedScopes(ctx context.Context, userID, platform string) ([]string, bool, error) {
	if _, err := uuid.Parse(userID); err != nil {
		return nil, false, nil
	}
	const query = `
		SELECT granted_scopes
		FROM mod_oauth_credentials
		WHERE user_id = $1 AND platform = $2`

	var scopes []string
	err := r.db.QueryRow(ctx, query, userID, platform).Scan(&scopes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read moderator scopes: %w", err)
	}
	return scopes, true, nil
}
