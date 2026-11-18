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
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Erasure of a person's data (ADR-0088). Account deletion (UserRepository.Delete) and
// viewer self-deletion (ViewerIdentityRepository.EraseViewer) both end in eraseViewers,
// so "the viewer data of these platform accounts" means the same on both paths.

// userPlatformIdentitiesSQL lists the platform accounts an All-Chat account has proven
// it controls: its login identities, the accounts it connected through OAuth, and the
// viewer sessions the viewer callback linked to it (viewer_sessions.user_id).
const userPlatformIdentitiesSQL = `
SELECT 'twitch', twitch_id FROM users WHERE id = $1 AND twitch_id IS NOT NULL
UNION SELECT 'kick', kick_id FROM users WHERE id = $1 AND kick_id IS NOT NULL
UNION SELECT 'youtube', google_id FROM users WHERE id = $1 AND google_id IS NOT NULL
UNION SELECT 'twitch', twitch_user_id FROM twitch_oauth_tokens WHERE user_id = $1
UNION SELECT 'kick', kick_user_id FROM kick_oauth_tokens WHERE user_id = $1 AND kick_user_id IS NOT NULL
UNION SELECT 'youtube', channel_id FROM youtube_oauth_tokens WHERE user_id = $1
UNION SELECT platform, platform_user_id FROM viewer_sessions WHERE user_id = $1`

// eraseViewers deletes every viewer that one of the given platform identities maps to,
// plus the viewers in viewerIDs, and every viewer session of those viewers or
// identities. The viewers row cascades to its platform identities, cosmetics, points
// balances and ledger, poll votes, prediction wagers and viewer Patreon token and
// subscription; a session cascades to its viewer_message_history. Open polls and
// predictions are tallied from the remaining rows, so other viewers stay consistent.
//
// It returns every erased platform identity, for dropping the
// viewer:identity:{platform}:{id} cache the message-processor reads.
func eraseViewers(ctx context.Context, tx pgx.Tx, viewerIDs []uuid.UUID, identities []LinkedPlatform) ([]LinkedPlatform, error) {
	platforms, userIDs := splitIdentities(identities)
	rows, err := tx.Query(ctx, `
		SELECT viewer_id FROM viewer_platform_identities
		WHERE (platform, platform_user_id) IN (SELECT * FROM unnest($1::text[], $2::text[]))
		UNION
		SELECT viewer_id FROM viewer_sessions
		WHERE viewer_id IS NOT NULL
		  AND (platform, platform_user_id) IN (SELECT * FROM unnest($1::text[], $2::text[]))`,
		platforms, userIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve viewers: %w", err)
	}
	found, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return nil, fmt.Errorf("resolve viewers: %w", err)
	}
	viewerIDs = append(viewerIDs, found...)

	// Every platform linked to those viewers, so their other sessions go too.
	rows, err = tx.Query(ctx,
		`SELECT platform, platform_user_id FROM viewer_platform_identities WHERE viewer_id = ANY($1)`, viewerIDs)
	if err != nil {
		return nil, fmt.Errorf("list viewer identities: %w", err)
	}
	linked, err := pgx.CollectRows(rows, pgx.RowToStructByPos[LinkedPlatform])
	if err != nil {
		return nil, fmt.Errorf("list viewer identities: %w", err)
	}
	identities = dedupeIdentities(append(identities, linked...))

	platforms, userIDs = splitIdentities(identities)
	if _, err := tx.Exec(ctx, `
		DELETE FROM viewer_sessions
		WHERE viewer_id = ANY($1)
		   OR (platform, platform_user_id) IN (SELECT * FROM unnest($2::text[], $3::text[]))`,
		viewerIDs, platforms, userIDs); err != nil {
		return nil, fmt.Errorf("delete viewer sessions: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM viewers WHERE id = ANY($1)`, viewerIDs); err != nil {
		return nil, fmt.Errorf("delete viewers: %w", err)
	}
	return identities, nil
}

func splitIdentities(ids []LinkedPlatform) (platforms, userIDs []string) {
	for _, id := range ids {
		platforms = append(platforms, id.Platform)
		userIDs = append(userIDs, id.PlatformUserID)
	}
	return platforms, userIDs
}

func dedupeIdentities(ids []LinkedPlatform) []LinkedPlatform {
	seen := make(map[LinkedPlatform]bool, len(ids))
	out := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// EraseViewer erases the signed-in viewer's own data (ADR-0088): the viewer the token
// names (uuid.Nil for a token issued before viewer identities existed) and any viewer
// the token's platform identity maps to. That includes a viewer engagement-service
// created without a login, for someone who only voted, bet or earned points by chat.
// It returns the erased platform identities.
func (r *ViewerIdentityRepository) EraseViewer(ctx context.Context, viewerID uuid.UUID, platform, platformUserID string) ([]LinkedPlatform, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin viewer erasure: %w", err)
	}
	defer tx.Rollback(ctx)

	var claimed []uuid.UUID
	if viewerID != uuid.Nil {
		claimed = append(claimed, viewerID)
	}
	erased, err := eraseViewers(ctx, tx, claimed, []LinkedPlatform{{Platform: platform, PlatformUserID: platformUserID}})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit viewer erasure: %w", err)
	}
	return erased, nil
}

// Delete erases a streamer account in one transaction (ADR-0088). Rows that reference
// the user or its overlays go through their foreign keys (CASCADE, or SET NULL where
// another person's record must survive, migration 119). On top of that it erases what
// has no foreign key to cascade through: the account's own viewer identity (every
// viewer of a platform account the user proved it controls), the alert history and
// YouTube quota bookkeeping of its overlays. The moderation audit log is kept
// (ADR-0017: it must survive account deletion for abuse forensics).
//
// It returns the erased viewer platform identities.
func (r *UserRepository) Delete(ctx context.Context, id string) ([]LinkedPlatform, error) {
	if id == "" {
		return nil, fmt.Errorf("user id is required")
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin account deletion: %w", err)
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, userPlatformIdentitiesSQL, id)
	if err != nil {
		return nil, fmt.Errorf("list account platform identities: %w", err)
	}
	identities, err := pgx.CollectRows(rows, pgx.RowToStructByPos[LinkedPlatform])
	if err != nil {
		return nil, fmt.Errorf("list account platform identities: %w", err)
	}
	erased, err := eraseViewers(ctx, tx, nil, identities)
	if err != nil {
		return nil, err
	}

	for _, stmt := range []string{
		// Kept on overlay deletion by design (ADR-0068); nobody can read it once the
		// account is gone.
		`DELETE FROM alert_events WHERE overlay_id IN (SELECT id FROM overlays WHERE user_id = $1)`,
		`DELETE FROM youtube_quota_audit_log WHERE overlay_id IN (SELECT id FROM overlays WHERE user_id = $1)`,
		`DELETE FROM youtube_channel_quota WHERE user_id = $1`,
	} {
		if _, err := tx.Exec(ctx, stmt, id); err != nil {
			return nil, fmt.Errorf("erase account history: %w", err)
		}
	}

	result, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("failed to delete user: %w", err)
	}
	if result.RowsAffected() == 0 {
		return nil, ErrUserNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit account deletion: %w", err)
	}
	return erased, nil
}
