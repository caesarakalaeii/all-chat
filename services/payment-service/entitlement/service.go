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

// Package entitlement applies a Patreon membership snapshot to our state: it maps
// the snapshot to a subscription status, persists it, and recomputes
// users.is_premium. It is the single convergent write path shared by the webhook
// handler, the reconcile job, and the OAuth callback.
package entitlement

import (
	"context"

	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"github.com/caesar/all-chat/shared/premium"
	"go.uber.org/zap"
)

// Service applies membership snapshots to subscription + premium state.
type Service struct {
	subs       *repository.SubscriptionRepository
	recomputer *premium.Recomputer
	minCents   int
	logger     *zap.Logger
}

// NewService builds an entitlement Service. minCents is the qualifying tier
// threshold (PATREON_MIN_TIER_CENTS).
func NewService(subs *repository.SubscriptionRepository, recomputer *premium.Recomputer, minCents int, logger *zap.Logger) *Service {
	return &Service{subs: subs, recomputer: recomputer, minCents: minCents, logger: logger}
}

// Apply maps the snapshot to a subscription status, upserts the subscription row,
// and — when the all-chat user is known — recomputes users.is_premium. raw is the
// original provider payload for audit (nil when not available, e.g. reconcile).
// It is idempotent and convergent: repeated or out-of-order calls converge to the
// same state because the upsert is keyed and the recompute is a pure function of
// current rows.
func (s *Service) Apply(ctx context.Context, snap *patreon.MembershipSnapshot, userID *string, raw []byte) (status string, isPremium bool, err error) {
	status = patreon.SubscriptionStatusFor(*snap, s.minCents)

	if err = s.subs.Upsert(ctx, repository.Subscription{
		UserID:           userID,
		Provider:         "patreon",
		ProviderUserID:   snap.PatreonUserID,
		Status:           status,
		TierID:           snap.TierID,
		Cents:            snap.EntitledCents,
		CurrentPeriodEnd: snap.NextChargeDate,
		Raw:              raw,
	}); err != nil {
		return status, false, err
	}

	if userID != nil && *userID != "" {
		isPremium, err = s.recomputer.Recompute(ctx, *userID)
		if err != nil {
			return status, false, err
		}
		s.logger.Info("Applied Patreon membership",
			zap.String("user_id", *userID),
			zap.String("patreon_user_id", snap.PatreonUserID),
			zap.String("status", status),
			zap.Bool("is_premium", isPremium))
	} else {
		s.logger.Info("Stored Patreon membership for unlinked patron",
			zap.String("patreon_user_id", snap.PatreonUserID),
			zap.String("status", status))
	}

	return status, isPremium, nil
}
