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
	"encoding/json"
	"fmt"

	"github.com/caesar/all-chat/shared/premium"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ViewerIdentityRepository handles database operations for viewer identity
// (cross-platform linking and cosmetics).
type ViewerIdentityRepository struct {
	db         *pgxpool.Pool
	recomputer *premium.Recomputer
}

// NewViewerIdentityRepository creates a new ViewerIdentityRepository. recomputer
// derives viewers.is_premium after a viewer↔streamer link change (ADR-0019).
func NewViewerIdentityRepository(db *pgxpool.Pool, recomputer *premium.Recomputer) *ViewerIdentityRepository {
	return &ViewerIdentityRepository{db: db, recomputer: recomputer}
}

// GetOrCreateViewerByPlatform looks up or creates a viewer record for the given
// (platform, platformUserID). Returns the durable viewer_id UUID.
//
// Flow:
//  1. SELECT viewer_id FROM viewer_platform_identities WHERE platform=$1 AND platform_user_id=$2
//  2. If found: return that viewer_id
//  3. If not found:
//     a. INSERT INTO viewers DEFAULT VALUES RETURNING id  → newViewerID
//     b. INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id)
//     c. INSERT INTO viewer_cosmetics (viewer_id) ON CONFLICT DO NOTHING
//     d. UPDATE viewer_sessions SET viewer_id=$1 WHERE platform=$2 AND platform_user_id=$3
//     e. Return newViewerID
func (r *ViewerIdentityRepository) GetOrCreateViewerByPlatform(ctx context.Context, platform, platformUserID string) (uuid.UUID, error) {
	// Step 1: try to find existing mapping
	var viewerID uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT viewer_id FROM viewer_platform_identities WHERE platform = $1 AND platform_user_id = $2`,
		platform, platformUserID,
	).Scan(&viewerID)

	if err == nil {
		// Found existing viewer
		return viewerID, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, fmt.Errorf("failed to lookup viewer_platform_identity: %w", err)
	}

	// Step 3a: create new viewer record
	var newViewerID uuid.UUID
	err = r.db.QueryRow(ctx,
		`INSERT INTO viewers DEFAULT VALUES RETURNING id`,
	).Scan(&newViewerID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert viewer: %w", err)
	}

	// Step 3b: create platform identity mapping
	_, err = r.db.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES ($1, $2, $3)`,
		newViewerID, platform, platformUserID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert viewer_platform_identity: %w", err)
	}

	// Step 3c: create cosmetics row (no-op if already exists)
	_, err = r.db.Exec(ctx,
		`INSERT INTO viewer_cosmetics (viewer_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		newViewerID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to insert viewer_cosmetics: %w", err)
	}

	// Step 3d: backfill viewer_id on existing session rows for this platform user
	_, err = r.db.Exec(ctx,
		`UPDATE viewer_sessions SET viewer_id = $1 WHERE platform = $2 AND platform_user_id = $3`,
		newViewerID, platform, platformUserID,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to update viewer_sessions viewer_id: %w", err)
	}

	return newViewerID, nil
}

// GetViewerCosmetics returns the name_color for a viewer, or nil if not set.
func (r *ViewerIdentityRepository) GetViewerCosmetics(ctx context.Context, viewerID uuid.UUID) (*string, error) {
	var nameColor *string
	err := r.db.QueryRow(ctx,
		`SELECT name_color FROM viewer_cosmetics WHERE viewer_id = $1`,
		viewerID,
	).Scan(&nameColor)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get viewer cosmetics: %w", err)
	}

	return nameColor, nil
}

// ViewerCosmetics holds all cosmetic settings for a viewer.
type ViewerCosmetics struct {
	NameColor     *string         `json:"name_color"`
	NameGradient  json.RawMessage `json:"name_gradient"`
	AvatarFrameID *uuid.UUID      `json:"avatar_frame_id"`
	AvatarFlairID *uuid.UUID      `json:"avatar_flair_id"`
}

// GetFullCosmetics returns all cosmetic settings for a viewer.
func (r *ViewerIdentityRepository) GetFullCosmetics(ctx context.Context, viewerID uuid.UUID) (*ViewerCosmetics, error) {
	var c ViewerCosmetics
	err := r.db.QueryRow(ctx,
		`SELECT name_color, name_gradient, avatar_frame_id, avatar_flair_id FROM viewer_cosmetics WHERE viewer_id = $1`,
		viewerID,
	).Scan(&c.NameColor, &c.NameGradient, &c.AvatarFrameID, &c.AvatarFlairID)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get viewer cosmetics: %w", err)
	}
	return &c, nil
}

// CosmeticsUpdate describes a partial update to a viewer's cosmetics. Each Set*
// flag selects whether that column group is written; groups whose flag is false are
// left exactly as they were (or NULL on first insert). name_color and name_gradient
// move together under SetName because they are mutually exclusive (setting one
// clears the other); avatar_frame_id and avatar_flair_id are independent and have
// their own flags, so changing one never disturbs the other.
//
// To CLEAR a set column, pass a nil pointer (→ SQL NULL) with the Set flag true.
// Never pass a pointer to uuid.Nil for an avatar column: it would be encoded as the
// literal '00000000-...' value and violate the avatar foreign keys.
type CosmeticsUpdate struct {
	SetName      bool
	NameColor    *string
	NameGradient []byte

	SetFrame      bool
	AvatarFrameID *uuid.UUID

	SetFlair      bool
	AvatarFlairID *uuid.UUID
}

// UpsertViewerCosmetics applies a partial cosmetics update and returns the full
// persisted row. It is a true per-column PATCH: only the column groups whose Set*
// flag is true are written, so e.g. changing a name color never clears a saved
// avatar frame, and setting a flair never clears the frame. The RETURNING clause
// yields the authoritative post-write state in the same round-trip, so callers
// never need a separate (racy, fallible) read-back to report what was stored.
// Callers MUST pass a nil value for any group whose Set flag is false (the handler
// does: absent request fields decode to nil). This keeps the INSERT branch's VALUES
// unwrapped so the JSONB/UUID columns get their parameters directly (wrapping a param
// in CASE would lose the column's type context and break inference); the per-group
// gating that preserves an existing row's untouched columns lives in the typed
// ON CONFLICT branch, where both CASE arms are already column-typed.
func (r *ViewerIdentityRepository) UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, u CosmeticsUpdate) (*ViewerCosmetics, error) {
	var c ViewerCosmetics
	err := r.db.QueryRow(ctx,
		`INSERT INTO viewer_cosmetics (viewer_id, name_color, name_gradient, avatar_frame_id, avatar_flair_id, updated_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())
		 ON CONFLICT (viewer_id) DO UPDATE SET
		     name_color      = CASE WHEN $6 THEN EXCLUDED.name_color      ELSE viewer_cosmetics.name_color      END,
		     name_gradient   = CASE WHEN $6 THEN EXCLUDED.name_gradient   ELSE viewer_cosmetics.name_gradient   END,
		     avatar_frame_id = CASE WHEN $7 THEN EXCLUDED.avatar_frame_id ELSE viewer_cosmetics.avatar_frame_id END,
		     avatar_flair_id = CASE WHEN $8 THEN EXCLUDED.avatar_flair_id ELSE viewer_cosmetics.avatar_flair_id END,
		     updated_at = NOW()
		 RETURNING name_color, name_gradient, avatar_frame_id, avatar_flair_id`,
		viewerID, u.NameColor, u.NameGradient, u.AvatarFrameID, u.AvatarFlairID,
		u.SetName, u.SetFrame, u.SetFlair,
	).Scan(&c.NameColor, &c.NameGradient, &c.AvatarFrameID, &c.AvatarFlairID)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert viewer cosmetics: %w", err)
	}
	return &c, nil
}

// LinkViewerToUser links a viewer session to a streamer user account and recomputes
// the viewer's premium badge. Called after OAuth when a linked streamer account is
// found. viewers.is_premium is derived by shared/premium.RecomputeViewer — whose
// "premium" half includes the linked-streamer inheritance term — so this no longer
// writes the flag directly; RecomputeViewer is the single writer (ADR-0019). This
// also lets a streamer lapse later revoke the inherited badge on the next recompute,
// which the old direct write never did.
func (r *ViewerIdentityRepository) LinkViewerToUser(ctx context.Context, platform, platformUserID, userID string) error {
	parsedID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	// Link the viewer session row to the user account.
	if _, err := r.db.Exec(ctx,
		`UPDATE viewer_sessions SET user_id = $1 WHERE platform = $2 AND platform_user_id = $3`,
		parsedID, platform, platformUserID,
	); err != nil {
		return fmt.Errorf("failed to link viewer session to user: %w", err)
	}

	// Recompute the durable viewer's premium badge now that the streamer link exists.
	// If the platform identity isn't mapped to a viewer yet, there is nothing to
	// recompute (cosmetics fail gracefully); the next viewer activity will converge.
	var viewerID uuid.UUID
	err = r.db.QueryRow(ctx,
		`SELECT viewer_id FROM viewer_platform_identities WHERE platform = $1 AND platform_user_id = $2 LIMIT 1`,
		platform, platformUserID,
	).Scan(&viewerID)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to resolve viewer for premium recompute: %w", err)
	}

	if _, err := r.recomputer.RecomputeViewer(ctx, viewerID.String()); err != nil {
		return fmt.Errorf("failed to recompute viewer premium: %w", err)
	}
	return nil
}

// LinkPlatformToViewer adds a new platform identity to an existing viewer record.
// It is idempotent: if (platform, platformUserID) already maps to viewerID the call
// succeeds without modifying anything.
//
// Returns ErrPlatformAlreadyLinked if (platform, platformUserID) already exists but
// belongs to a *different* viewer — the caller must handle this case.
func (r *ViewerIdentityRepository) LinkPlatformToViewer(ctx context.Context, viewerID uuid.UUID, platform, platformUserID string) error {
	// Check whether this platform identity already exists.
	var existingViewerID uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT viewer_id FROM viewer_platform_identities WHERE platform = $1 AND platform_user_id = $2`,
		platform, platformUserID,
	).Scan(&existingViewerID)

	if err == nil {
		// Row exists — idempotent if it already points to the right viewer.
		if existingViewerID == viewerID {
			return nil
		}
		// The platform belongs to a different viewer. In the connect flow the
		// user explicitly asked to link this platform to their current account,
		// so move it by updating the viewer_id.
		_, updateErr := r.db.Exec(ctx,
			`UPDATE viewer_platform_identities SET viewer_id = $1 WHERE platform = $2 AND platform_user_id = $3`,
			viewerID, platform, platformUserID,
		)
		if updateErr != nil {
			return fmt.Errorf("failed to re-link platform identity: %w", updateErr)
		}
		return nil
	}
	if err != pgx.ErrNoRows {
		return fmt.Errorf("failed to check existing platform identity: %w", err)
	}

	// Insert the new platform identity for the existing viewer.
	_, err = r.db.Exec(ctx,
		`INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES ($1, $2, $3)`,
		viewerID, platform, platformUserID,
	)
	if err != nil {
		return fmt.Errorf("failed to insert platform identity: %w", err)
	}

	// Backfill viewer_id on the viewer_sessions row for this platform user.
	_, err = r.db.Exec(ctx,
		`UPDATE viewer_sessions SET viewer_id = $1 WHERE platform = $2 AND platform_user_id = $3`,
		viewerID, platform, platformUserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update viewer_sessions viewer_id: %w", err)
	}

	return nil
}

// LinkedPlatform represents a single platform linked to a viewer.
type LinkedPlatform struct {
	Platform       string
	PlatformUserID string
}

// GetLinkedPlatforms returns all platforms currently linked to a viewer.
func (r *ViewerIdentityRepository) GetLinkedPlatforms(ctx context.Context, viewerID uuid.UUID) ([]LinkedPlatform, error) {
	rows, err := r.db.Query(ctx,
		`SELECT platform, platform_user_id FROM viewer_platform_identities WHERE viewer_id = $1 ORDER BY created_at`,
		viewerID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query linked platforms: %w", err)
	}
	defer rows.Close()

	var platforms []LinkedPlatform
	for rows.Next() {
		var lp LinkedPlatform
		if err := rows.Scan(&lp.Platform, &lp.PlatformUserID); err != nil {
			return nil, fmt.Errorf("failed to scan linked platform: %w", err)
		}
		platforms = append(platforms, lp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate linked platforms: %w", err)
	}
	return platforms, nil
}

// UnlinkPlatform removes a platform identity from a viewer.
// Returns ErrLastPlatform if the viewer only has one platform linked (must retain at least one).
// Returns ErrNotFound if the platform is not linked to this viewer.
func (r *ViewerIdentityRepository) UnlinkPlatform(ctx context.Context, viewerID uuid.UUID, platform string) error {
	// Count how many platforms the viewer currently has.
	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM viewer_platform_identities WHERE viewer_id = $1`,
		viewerID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count linked platforms: %w", err)
	}
	if count <= 1 {
		return ErrLastPlatform
	}

	// Look up the platform_user_id before deleting so we can clear viewer_sessions.
	var platformUserID string
	err = r.db.QueryRow(ctx,
		`SELECT platform_user_id FROM viewer_platform_identities WHERE viewer_id = $1 AND platform = $2`,
		viewerID, platform,
	).Scan(&platformUserID)
	if err != nil {
		return ErrNotFound
	}

	// Delete the platform identity row.
	tag, err := r.db.Exec(ctx,
		`DELETE FROM viewer_platform_identities WHERE viewer_id = $1 AND platform = $2`,
		viewerID, platform,
	)
	if err != nil {
		return fmt.Errorf("failed to delete platform identity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	// Clear viewer_id on sessions that belonged to the unlinked platform identity
	// so they are no longer associated with the viewer.
	_, _ = r.db.Exec(ctx,
		`UPDATE viewer_sessions SET viewer_id = NULL WHERE platform = $1 AND platform_user_id = $2 AND viewer_id = $3`,
		platform, platformUserID, viewerID,
	)

	return nil
}

// MigratePlatformUserID updates the platform_user_id in viewer_platform_identities for a viewer
// previously identified by (platform, oldPlatformUserID) to newPlatformUserID.  This is used to
// migrate existing YouTube viewers from the Google account ID to the YouTube channel ID.
func (r *ViewerIdentityRepository) MigratePlatformUserID(ctx context.Context, platform, oldPlatformUserID, newPlatformUserID string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE viewer_platform_identities SET platform_user_id = $3 WHERE platform = $1 AND platform_user_id = $2`,
		platform, oldPlatformUserID, newPlatformUserID,
	)
	if err != nil {
		return fmt.Errorf("failed to migrate viewer_platform_identities platform_user_id: %w", err)
	}
	return nil
}

// GetViewerIsPremium returns the is_premium flag for a viewer.
func (r *ViewerIdentityRepository) GetViewerIsPremium(ctx context.Context, viewerID uuid.UUID) (bool, error) {
	var isPremium bool
	err := r.db.QueryRow(ctx,
		`SELECT is_premium FROM viewers WHERE id = $1`,
		viewerID,
	).Scan(&isPremium)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to get viewer is_premium: %w", err)
	}
	return isPremium, nil
}
