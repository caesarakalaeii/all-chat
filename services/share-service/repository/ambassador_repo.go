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

	"github.com/caesar/all-chat/shared/premium"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Showcase is a streamer's ambassador state: whether they hold the role, plus the
// presentation metadata for their public homepage card. Tagline/SortOrder are
// admin-curated; FeaturedConsent is the streamer's own opt-in (ADR-0041).
type Showcase struct {
	IsAmbassador    bool
	Tagline         *string
	SortOrder       int
	FeaturedConsent bool
}

// PublicAmbassador is one card in the public "Featured Ambassadors" homepage list.
// It carries only non-sensitive, already-public profile fields.
type PublicAmbassador struct {
	Username    string
	DisplayName string
	AvatarURL   string
	Platform    string // auth_provider: "twitch" | "youtube" | "kick"
	Tagline     *string
}

// AmbassadorRepository owns the ambassador role write-path and the public showcase
// read-path (ADR-0041). Role changes fold into premium via the shared recomputer,
// mirroring the beta-tester path (repository.PremiumRepository.SetUserBetaTester).
type AmbassadorRepository struct {
	db         *pgxpool.Pool
	recomputer *premium.Recomputer
	logger     *zap.Logger
}

func NewAmbassadorRepository(db *pgxpool.Pool, recomputer *premium.Recomputer, logger *zap.Logger) *AmbassadorRepository {
	return &AmbassadorRepository{db: db, recomputer: recomputer, logger: logger}
}

// SetUserAmbassador records the admin's ambassador decision (ADR-0041) and
// re-derives users.is_premium via shared/premium (an ambassador is premium, and
// also passes early-access gates). It mirrors SetUserBetaTester, and additionally
// curates the public showcase card:
//
//   - On grant, it upserts the ambassador_showcase row with the admin-curated
//     tagline and sort_order. Both are optional: a nil value PRESERVES the existing
//     value (so re-granting or editing one field never wipes the other), and the
//     streamer's featured_consent is never touched here — it is their own opt-in.
//   - On revoke, the flag alone is cleared; the showcase row is left intact (the
//     public endpoint filters on is_ambassador, so a revoked streamer disappears
//     regardless, and a later re-grant restores their prior card + consent).
//
// is_premium is materialized: the flag write and the recompute are separate, and
// Recompute is convergent, so the pair mirrors SetUserBetaTester's ordering.
func (r *AmbassadorRepository) SetUserAmbassador(ctx context.Context, userID string, isAmbassador bool, tagline *string, sortOrder *int) error {
	result, err := r.db.Exec(ctx,
		"UPDATE users SET is_ambassador = $1 WHERE id = $2",
		isAmbassador, userID)
	if err != nil {
		r.logger.Error("Failed to update ambassador status",
			zap.String("user_id", userID),
			zap.Bool("is_ambassador", isAmbassador),
			zap.Error(err))
		return fmt.Errorf("failed to update ambassador status: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	if isAmbassador {
		// Upsert the admin-curated card metadata, preserving any field the admin did
		// not send and never overwriting the streamer's featured_consent opt-in.
		if _, err := r.db.Exec(ctx, `
			INSERT INTO ambassador_showcase (user_id, tagline, sort_order, updated_at)
			VALUES ($1, $2, COALESCE($3, 0), NOW())
			ON CONFLICT (user_id) DO UPDATE SET
			    tagline = COALESCE($2, ambassador_showcase.tagline),
			    sort_order = COALESCE($3, ambassador_showcase.sort_order),
			    updated_at = NOW()`,
			userID, tagline, sortOrder); err != nil {
			r.logger.Error("Failed to upsert ambassador showcase",
				zap.String("user_id", userID), zap.Error(err))
			return fmt.Errorf("failed to update ambassador showcase: %w", err)
		}
	}

	if _, err := r.recomputer.Recompute(ctx, userID); err != nil {
		r.logger.Error("Failed to recompute premium after ambassador change",
			zap.String("user_id", userID), zap.Error(err))
		return fmt.Errorf("failed to recompute premium status: %w", err)
	}

	r.logger.Info("Ambassador status updated",
		zap.String("user_id", userID),
		zap.Bool("is_ambassador", isAmbassador))
	return nil
}

// GetShowcase returns a user's ambassador state and showcase metadata. The LEFT
// JOIN means a user who holds the role but has no showcase row yet returns the
// zero-value card (no tagline, sort_order 0, consent false). Returns a
// "user not found" error when the user id does not exist.
func (r *AmbassadorRepository) GetShowcase(ctx context.Context, userID string) (*Showcase, error) {
	var s Showcase
	err := r.db.QueryRow(ctx, `
		SELECT u.is_ambassador,
		       sc.tagline,
		       COALESCE(sc.sort_order, 0),
		       COALESCE(sc.featured_consent, FALSE)
		FROM users u
		LEFT JOIN ambassador_showcase sc ON sc.user_id = u.id
		WHERE u.id = $1`, userID).
		Scan(&s.IsAmbassador, &s.Tagline, &s.SortOrder, &s.FeaturedConsent)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read ambassador showcase: %w", err)
	}
	return &s, nil
}

// SetFeaturedConsent records the streamer's own opt-in to appear on the public
// homepage (ADR-0041). It upserts only the consent flag, leaving the admin-curated
// tagline/sort_order untouched. Callers must gate this on the user actually holding
// the ambassador role; a stray consent row on a non-ambassador is harmless because
// the public read still requires users.is_ambassador.
func (r *AmbassadorRepository) SetFeaturedConsent(ctx context.Context, userID string, consent bool) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO ambassador_showcase (user_id, featured_consent, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
		    featured_consent = $2,
		    updated_at = NOW()`,
		userID, consent); err != nil {
		return fmt.Errorf("failed to set ambassador consent: %w", err)
	}
	r.logger.Info("Ambassador featured consent updated",
		zap.String("user_id", userID), zap.Bool("featured_consent", consent))
	return nil
}

// ListPublic returns the ambassadors who both hold the role AND have opted in
// (featured_consent), ordered by the admin-curated sort_order then display name.
// Banned accounts are excluded defensively. It selects only already-public profile
// fields for the marketing card.
func (r *AmbassadorRepository) ListPublic(ctx context.Context) ([]PublicAmbassador, error) {
	// Exclude banned accounts by BOTH ban paths: the account-level users.is_banned
	// AND an active platform-ID ban (banned_platform_ids), which is the authoritative
	// ban signal the rest of the app uses (auth-service GetAllUsers) and which does
	// NOT set users.is_banned. A public marketing card must never feature either.
	rows, err := r.db.Query(ctx, `
		SELECT u.username, u.display_name, u.profile_image_url, u.auth_provider, sc.tagline
		FROM ambassador_showcase sc
		JOIN users u ON u.id = sc.user_id
		WHERE u.is_ambassador = TRUE
		  AND sc.featured_consent = TRUE
		  AND u.is_banned = FALSE
		  AND NOT EXISTS (
		      SELECT 1 FROM banned_platform_ids b
		      WHERE b.is_active = TRUE AND (
		          (b.platform = 'twitch'  AND b.platform_id = u.twitch_id) OR
		          (b.platform = 'youtube' AND b.platform_id = u.google_id) OR
		          (b.platform = 'kick'    AND b.platform_id = u.kick_id)
		      )
		  )
		ORDER BY sc.sort_order ASC, u.display_name ASC`)
	if err != nil {
		return nil, fmt.Errorf("failed to list ambassadors: %w", err)
	}
	defer rows.Close()

	out := []PublicAmbassador{}
	for rows.Next() {
		var a PublicAmbassador
		// profile_image_url is nullable; scan through a pointer.
		var avatar *string
		if err := rows.Scan(&a.Username, &a.DisplayName, &avatar, &a.Platform, &a.Tagline); err != nil {
			return nil, fmt.Errorf("failed to scan ambassador: %w", err)
		}
		if avatar != nil {
			a.AvatarURL = *avatar
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ambassadors: %w", err)
	}
	return out, nil
}
