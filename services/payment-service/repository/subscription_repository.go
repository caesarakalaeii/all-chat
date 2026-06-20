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

// Subscription is a row of premium_subscriptions — the source of truth for the
// subscription half of users.is_premium.
type Subscription struct {
	UserID           *string // nullable: a webhook may arrive before the user links Patreon
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
// OAuth callback. A nil incoming UserID never clears a previously-resolved user id
// (COALESCE), so a webhook for an as-yet-unlinked patron is preserved until linked.
func (r *SubscriptionRepository) Upsert(ctx context.Context, s Subscription) error {
	provider := s.Provider
	if provider == "" {
		provider = "patreon"
	}

	var rawArg any
	if len(s.Raw) > 0 {
		rawArg = string(s.Raw)
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO premium_subscriptions
		    (user_id, provider, provider_user_id, status, tier_id, cents, current_period_end, raw, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (provider, provider_user_id) DO UPDATE SET
		    user_id            = COALESCE(EXCLUDED.user_id, premium_subscriptions.user_id),
		    status             = EXCLUDED.status,
		    tier_id            = EXCLUDED.tier_id,
		    cents              = EXCLUDED.cents,
		    current_period_end = EXCLUDED.current_period_end,
		    raw                = COALESCE(EXCLUDED.raw, premium_subscriptions.raw),
		    updated_at         = NOW()`,
		s.UserID, provider, s.ProviderUserID, s.Status, nullIfEmpty(s.TierID), s.Cents, s.CurrentPeriodEnd, rawArg)
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	return nil
}

// GetByUserID returns the most recently updated subscription for a user.
func (r *SubscriptionRepository) GetByUserID(ctx context.Context, userID string) (*Subscription, bool, error) {
	var s Subscription
	var tierID *string
	err := r.db.QueryRow(ctx, `
		SELECT user_id, provider, provider_user_id, status, tier_id, cents, current_period_end
		FROM premium_subscriptions
		WHERE user_id = $1
		ORDER BY updated_at DESC
		LIMIT 1`, userID).Scan(&s.UserID, &s.Provider, &s.ProviderUserID, &s.Status, &tierID, &s.Cents, &s.CurrentPeriodEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("get subscription by user: %w", err)
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

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
