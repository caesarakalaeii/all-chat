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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Roles a caller can hold on an overlay's moderation write-path.
const (
	RoleOwner     = "owner"
	RoleModerator = "moderator"
	RoleNone      = "none"
)

// ErrOverlayNotFound reports that the overlay does not exist, or that the id was not even a
// UUID. Callers MUST respond exactly as they do for RoleNone: a distinguishable 404 would turn
// the moderation endpoints into an overlay-existence oracle for any holder of any valid token.
var ErrOverlayNotFound = errors.New("overlay not found")

// OverlayAccess is everything the moderation write-path needs to decide a request, resolved in
// one round trip.
//
// Owner and caller are answered together on purpose: authorization keys on the CALLER's role,
// while the premium gate keys on the OWNER's entitlement — a delegated moderator moderates on a
// premium streamer's overlay for free, and must never be shown an upgrade prompt for a plan they
// cannot buy (ADR-0048).
type OverlayAccess struct {
	OwnerUserID    string
	OwnerIsPremium bool
	Role           string
	// GrantID and Actions are populated for RoleModerator only.
	GrantID string
	Actions []string
}

// IsOwner reports whether the caller owns the overlay.
func (a OverlayAccess) IsOwner() bool { return a.Role == RoleOwner }

// Authorized reports whether the caller holds any moderation role at all.
func (a OverlayAccess) Authorized() bool { return a.Role == RoleOwner || a.Role == RoleModerator }

// MayPerform reports whether the caller's role permits this action. An owner may perform
// anything the platform supports; a delegated moderator is limited to the actions the owner
// granted, enforced here rather than only hidden in the UI.
func (a OverlayAccess) MayPerform(action string) bool {
	switch a.Role {
	case RoleOwner:
		return true
	case RoleModerator:
		for _, granted := range a.Actions {
			if granted == action {
				return true
			}
		}
	}
	return false
}

// ResolveOverlayAccess resolves the caller's role on an overlay together with the owner's
// identity and entitlement.
//
// Never cached. A revocation must take effect within one request, so the grant is read live on
// every action; only the feature-gate flags come from an in-memory cache.
func (r *Repository) ResolveOverlayAccess(ctx context.Context, overlayID, callerID string) (OverlayAccess, error) {
	// Validate before querying: overlays.id is a UUID column, so a malformed path parameter
	// would otherwise come back as a Postgres cast error and surface as a 500 — turning bad
	// input into an error-rate spike.
	if _, err := uuid.Parse(overlayID); err != nil {
		return OverlayAccess{}, ErrOverlayNotFound
	}

	// A caller id that is not a UUID cannot match anything. Normalise it to a value that
	// compares cleanly rather than letting the cast fail.
	if _, err := uuid.Parse(callerID); err != nil {
		callerID = uuid.Nil.String()
	}

	const query = `
		SELECT o.user_id::text,
		       u.is_premium,
		       CASE WHEN o.user_id::text = $2 THEN 'owner'
		            WHEN m.id IS NOT NULL     THEN 'moderator'
		            ELSE 'none' END,
		       COALESCE(m.id::text, ''),
		       COALESCE(m.actions, '{}')
		FROM overlays o
		JOIN users u ON u.id = o.user_id
		LEFT JOIN overlay_moderators m
		       ON m.overlay_id = o.id
		      AND m.moderator_user_id::text = $2
		      AND m.status = 'active'
		      AND m.revoked_at IS NULL
		WHERE o.id = $1`

	var access OverlayAccess
	err := r.db.QueryRow(ctx, query, overlayID, callerID).Scan(
		&access.OwnerUserID,
		&access.OwnerIsPremium,
		&access.Role,
		&access.GrantID,
		&access.Actions,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OverlayAccess{}, ErrOverlayNotFound
	}
	if err != nil {
		return OverlayAccess{}, fmt.Errorf("resolve overlay access: %w", err)
	}

	return access, nil
}
