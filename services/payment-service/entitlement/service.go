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
// the snapshot to a subscription status, persists it, and recomputes the relevant
// is_premium column. It is the single convergent write path shared by the webhook
// handler, the reconcile job, and the OAuth callback — for both the streamer
// (users.is_premium) and viewer (viewers.is_premium) products (ADR-0018, ADR-0019).
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
	subs           *repository.SubscriptionRepository
	recomputer     *premium.Recomputer
	minCents       int // streamer qualifying threshold (PATREON_MIN_TIER_CENTS)
	viewerMinCents int // viewer qualifying threshold (PATREON_VIEWER_MIN_TIER_CENTS)
	logger         *zap.Logger
}

// NewService builds an entitlement Service. minCents / viewerMinCents are the
// qualifying tier thresholds for the streamer / viewer products respectively.
func NewService(subs *repository.SubscriptionRepository, recomputer *premium.Recomputer, minCents, viewerMinCents int, logger *zap.Logger) *Service {
	return &Service{subs: subs, recomputer: recomputer, minCents: minCents, viewerMinCents: viewerMinCents, logger: logger}
}

// Apply maps the snapshot to a subscription status for the subject's product,
// upserts the subscription row, and — when the subject is known — recomputes the
// relevant is_premium column. The product is the viewer product iff viewerID is
// set, otherwise the streamer product; the qualifying threshold follows the
// product. raw is the original provider payload for audit (nil when not available,
// e.g. reconcile / OAuth callback).
//
// At most one of userID / viewerID should be set. It is idempotent and convergent:
// repeated or out-of-order calls converge because the upsert is keyed and the
// recompute is a pure function of current rows.
func (s *Service) Apply(ctx context.Context, snap *patreon.MembershipSnapshot, userID, viewerID *string, raw []byte) (status string, isPremium bool, err error) {
	product := repository.ProductStreamer
	threshold := s.minCents
	if viewerID != nil && *viewerID != "" {
		product = repository.ProductViewer
		threshold = s.viewerMinCents
	}

	status = patreon.SubscriptionStatusFor(*snap, threshold)

	if err = s.subs.Upsert(ctx, repository.Subscription{
		UserID:           userID,
		ViewerID:         viewerID,
		Product:          product,
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

	switch {
	case userID != nil && *userID != "":
		isPremium, err = s.recomputer.Recompute(ctx, *userID)
		if err != nil {
			return status, false, err
		}
		s.logger.Info("Applied Patreon membership (streamer)",
			zap.String("user_id", *userID),
			zap.String("patreon_user_id", snap.PatreonUserID),
			zap.String("status", status),
			zap.Bool("is_premium", isPremium))
	case viewerID != nil && *viewerID != "":
		isPremium, err = s.recomputer.RecomputeViewer(ctx, *viewerID)
		if err != nil {
			return status, false, err
		}
		s.logger.Info("Applied Patreon membership (viewer)",
			zap.String("viewer_id", *viewerID),
			zap.String("patreon_user_id", snap.PatreonUserID),
			zap.String("status", status),
			zap.Bool("is_premium", isPremium))
	default:
		s.logger.Info("Stored Patreon membership for unlinked patron",
			zap.String("patreon_user_id", snap.PatreonUserID),
			zap.String("status", status))
	}

	return status, isPremium, nil
}
