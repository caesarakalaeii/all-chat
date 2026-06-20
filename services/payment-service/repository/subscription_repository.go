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
// (product='viewer'). At most one of UserID / ViewerID is set (ADR-0019).
type Subscription struct {
	UserID           *string // streamer subject; nullable (webhook may precede the link)
	ViewerID         *string // viewer subject; nullable
	Product          string  // "streamer" | "viewer" (defaults to streamer when empty)
	Provider         string
	ProviderUserID   string
	Status           string
	TierID           string
	Cents            int
	CurrentPeriodEnd *time.Time
	Raw              []byte // original payload as JSON (audit/debug); nil to leave unset
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
// OAuth callback. A nil incoming UserID/ViewerID never clears a previously-resolved
// subject (COALESCE), so a webhook for an as-yet-unlinked patron is preserved until
// linked.
func (r *SubscriptionRepository) Upsert(ctx context.Context, s Subscription) error {
	provider := s.Provider
	if provider == "" {
		provider = "patreon"
	}
	product := s.Product
	if product == "" {
		product = ProductStreamer
	}

	var rawArg any
	if len(s.Raw) > 0 {
		rawArg = string(s.Raw)
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO premium_subscriptions
		    (user_id, viewer_id, product, provider, provider_user_id, status, tier_id, cents, current_period_end, raw, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (provider, provider_user_id) DO UPDATE SET
		    user_id            = COALESCE(EXCLUDED.user_id, premium_subscriptions.user_id),
		    viewer_id          = COALESCE(EXCLUDED.viewer_id, premium_subscriptions.viewer_id),
		    product            = EXCLUDED.product,
		    status             = EXCLUDED.status,
		    tier_id            = EXCLUDED.tier_id,
		    cents              = EXCLUDED.cents,
		    current_period_end = EXCLUDED.current_period_end,
		    raw                = COALESCE(EXCLUDED.raw, premium_subscriptions.raw),
		    updated_at         = NOW()`,
		s.UserID, s.ViewerID, product, provider, s.ProviderUserID, s.Status, nullIfEmpty(s.TierID), s.Cents, s.CurrentPeriodEnd, rawArg)
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

// MarkFormerByUserID flags all of a user's subscriptions as former (on disconnect).
func (r *SubscriptionRepository) MarkFormerByUserID(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE premium_subscriptions SET status = 'former', updated_at = NOW() WHERE user_id = $1", userID)
	if err != nil {
		return fmt.Errorf("mark subscriptions former: %w", err)
	}
	return nil
}

// MarkFormerByViewerID flags all of a viewer's subscriptions as former (on disconnect).
func (r *SubscriptionRepository) MarkFormerByViewerID(ctx context.Context, viewerID string) error {
	_, err := r.db.Exec(ctx,
		"UPDATE premium_subscriptions SET status = 'former', updated_at = NOW() WHERE viewer_id = $1", viewerID)
	if err != nil {
		return fmt.Errorf("mark viewer subscriptions former: %w", err)
	}
	return nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
