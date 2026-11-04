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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Product values stored in premium_subscriptions.product (ADR-0019).
const (
	ProductStreamer = "streamer"
	ProductViewer   = "viewer"
)

// Subscription is a row of premium_subscriptions — the source of truth for the
// subscription half of users.is_premium (product='streamer') or viewers.is_premium
// (product='viewer'). At most one of UserID / ViewerID is set (ADR-0019); both are
// read-only here, because Upsert takes the subject from the Patreon connection.
type Subscription struct {
	UserID           *string // streamer subject; nil while the patron is not connected
	ViewerID         *string // viewer subject; nil while the patron is not connected
	Product          string  // "streamer" | "viewer" (defaults to streamer when empty)
	Provider         string
	ProviderUserID   string
	Status           string
	TierID           string
	Cents            int
	CurrentPeriodEnd *time.Time
}

// SubscriptionRepository persists Patreon subscription state.
type SubscriptionRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewSubscriptionRepository builds a SubscriptionRepository.
func NewSubscriptionRepository(db *pgxpool.Pool, logger *zap.Logger) *SubscriptionRepository {
	return &SubscriptionRepository{db: db, logger: logger}
}

// Upsert writes a subscription keyed on (provider, provider_user_id). It is the
// single idempotent write path shared by the webhook, the reconcile job, and the
// OAuth callback.
//
// The row's subject (user_id / viewer_id) is not taken from s: the statement reads
// it from the Patreon connection (patreon_oauth_tokens), so a membership is linked
// to an all-chat account exactly while that account has the Patreon account
// connected. A webhook for a patron with no connection, or a reconcile pass applying
// a token that was disconnected after the pass read it, stores the row unlinked;
// only connecting via OAuth links it again.
func (r *SubscriptionRepository) Upsert(ctx context.Context, s Subscription) error {
	provider := s.Provider
	if provider == "" {
		provider = "patreon"
	}
	product := s.Product
	if product == "" {
		product = ProductStreamer
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO premium_subscriptions
		    (user_id, viewer_id, product, provider, provider_user_id, status, tier_id, cents, current_period_end, updated_at)
		VALUES (
		    (SELECT user_id FROM patreon_oauth_tokens WHERE patreon_user_id = $3),
		    (SELECT viewer_id FROM patreon_oauth_tokens WHERE patreon_user_id = $3),
		    $1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (provider, provider_user_id) DO UPDATE SET
		    user_id            = EXCLUDED.user_id,
		    viewer_id          = EXCLUDED.viewer_id,
		    product            = EXCLUDED.product,
		    status             = EXCLUDED.status,
		    tier_id            = EXCLUDED.tier_id,
		    cents              = EXCLUDED.cents,
		    current_period_end = EXCLUDED.current_period_end,
		    updated_at         = NOW()`,
		product, provider, s.ProviderUserID, s.Status, nullIfEmpty(s.TierID), s.Cents, s.CurrentPeriodEnd)
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	return nil
}

// GetByUserID returns the most recently updated streamer-product subscription for a user.
func (r *SubscriptionRepository) GetByUserID(ctx context.Context, userID string) (*Subscription, bool, error) {
	return r.getLatest(ctx, "user_id = $1 AND product = '"+ProductStreamer+"'", userID)
}

// GetByViewerID returns the most recently updated viewer-product subscription for a viewer.
func (r *SubscriptionRepository) GetByViewerID(ctx context.Context, viewerID string) (*Subscription, bool, error) {
	return r.getLatest(ctx, "viewer_id = $1 AND product = '"+ProductViewer+"'", viewerID)
}

func (r *SubscriptionRepository) getLatest(ctx context.Context, where, arg string) (*Subscription, bool, error) {
	var s Subscription
	var tierID *string
	err := r.db.QueryRow(ctx, `
		SELECT user_id, viewer_id, product, provider, provider_user_id, status, tier_id, cents, current_period_end
		FROM premium_subscriptions
		WHERE `+where+`
		ORDER BY updated_at DESC
		LIMIT 1`, arg).Scan(&s.UserID, &s.ViewerID, &s.Product, &s.Provider, &s.ProviderUserID, &s.Status, &tierID, &s.Cents, &s.CurrentPeriodEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get subscription: %w", err)
	}
	if tierID != nil {
		s.TierID = *tierID
	}
	return &s, true, nil
}

// DeleteByUserID deletes a user's subscriptions (on disconnect), so the membership
// is no longer tied to the account.
func (r *SubscriptionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM premium_subscriptions WHERE user_id = $1", userID); err != nil {
		return fmt.Errorf("delete subscriptions: %w", err)
	}
	return nil
}

// DeleteByViewerID deletes a viewer's subscriptions (on disconnect).
func (r *SubscriptionRepository) DeleteByViewerID(ctx context.Context, viewerID string) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM premium_subscriptions WHERE viewer_id = $1", viewerID); err != nil {
		return fmt.Errorf("delete viewer subscriptions: %w", err)
	}
	return nil
}

// DeleteDisconnected deletes every subscription linked to a subject that no longer
// has that Patreon account connected, and returns those rows' subjects so their
// premium can be recomputed. Such rows were left by disconnects from before
// disconnecting deleted them, and by switching a subject to another Patreon
// account. Unlinked rows are left alone.
func (r *SubscriptionRepository) DeleteDisconnected(ctx context.Context) ([]Subscription, error) {
	rows, err := r.db.Query(ctx, `
		DELETE FROM premium_subscriptions s
		WHERE (s.user_id IS NOT NULL OR s.viewer_id IS NOT NULL)
		  AND NOT EXISTS (
		      SELECT 1 FROM patreon_oauth_tokens t
		      WHERE t.patreon_user_id = s.provider_user_id
		        AND t.user_id IS NOT DISTINCT FROM s.user_id
		        AND t.viewer_id IS NOT DISTINCT FROM s.viewer_id)
		RETURNING s.user_id, s.viewer_id`)
	if err != nil {
		return nil, fmt.Errorf("delete disconnected subscriptions: %w", err)
	}
	subs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Subscription, error) {
		var s Subscription
		err := row.Scan(&s.UserID, &s.ViewerID)
		return s, err
	})
	if err != nil {
		return nil, fmt.Errorf("delete disconnected subscriptions: %w", err)
	}
	return subs, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
