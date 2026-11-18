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
	"errors"

	"github.com/caesar/all-chat/services/payment-service/metrics"
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
// product. The row is linked to the subject only while the subject has this
// Patreon account connected (see SubscriptionRepository.Upsert), so a stale
// subject is recomputed without premium from this row. Only the snapshot's typed
// fields are stored; the provider payload is
// not, because a webhook body carries the patron's email, name and address.
//
// At most one of userID / viewerID should be set. It is idempotent and convergent:
// repeated or out-of-order calls converge because the upsert is keyed and the
// recompute is a pure function of current rows.
func (s *Service) Apply(ctx context.Context, snap *patreon.MembershipSnapshot, userID, viewerID *string) (status string, isPremium bool, err error) {
	product := repository.ProductStreamer
	threshold := s.minCents
	if viewerID != nil && *viewerID != "" {
		product = repository.ProductViewer
		threshold = s.viewerMinCents
	}

	status = patreon.SubscriptionStatusFor(*snap, threshold)

	// A StatusNone we arrived at by discarding members is a read failure wearing the
	// same clothes as a non-patron. Say so loudly: revoking premium is the one thing
	// this path does that a user notices, and it must never happen quietly again.
	if status == patreon.StatusNone && snap.UnmatchedMembers > 0 {
		metrics.UnmatchedMembers.Add(float64(snap.UnmatchedMembers))
		s.logger.Warn("Patreon identity returned membership(s) we could not attribute to our campaign; treating as no membership",
			zap.String("patreon_user_id", snap.PatreonUserID),
			zap.Int("unmatched_members", snap.UnmatchedMembers))
	}

	if err = s.subs.Upsert(ctx, repository.Subscription{
		Product:          product,
		Provider:         "patreon",
		ProviderUserID:   snap.PatreonUserID,
		Status:           status,
		TierID:           snap.TierID,
		Cents:            snap.EntitledCents,
		CurrentPeriodEnd: snap.NextChargeDate,
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

// PruneDisconnected deletes the memberships still linked to a subject that no
// longer has that Patreon account connected and recomputes each such subject, so
// premium they kept is revoked. It returns how many rows it deleted. The reconcile
// pass runs it; disconnect itself already deletes the subject's rows.
func (s *Service) PruneDisconnected(ctx context.Context) (int, error) {
	subs, err := s.subs.DeleteDisconnected(ctx)
	if err != nil {
		return 0, err
	}
	// The rows are gone once deleted, so a later pass cannot retry a failed
	// recompute: keep going and report every failure rather than stopping early.
	var errs []error
	for _, sub := range subs {
		if sub.UserID != nil {
			_, err = s.recomputer.Recompute(ctx, *sub.UserID)
		} else {
			_, err = s.recomputer.RecomputeViewer(ctx, *sub.ViewerID)
		}
		errs = append(errs, err)
	}
	if len(subs) > 0 {
		s.logger.Info("Deleted memberships of disconnected Patreon accounts", zap.Int("count", len(subs)))
	}
	return len(subs), errors.Join(errs...)
}
