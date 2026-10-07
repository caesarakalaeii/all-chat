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

// Package processor turns one event-typed raw stream message into the alert
// pipeline: normalize, route to alert-capable overlays, persist, publish, and
// feed 'list' overlay leaderboards.
package processor

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/alert-processor/leaderboard"
	"github.com/caesar/all-chat/services/alert-processor/metrics"
	"github.com/caesar/all-chat/services/alert-processor/models"
	"github.com/caesar/all-chat/services/alert-processor/normalizer"
	"github.com/caesar/all-chat/services/alert-processor/publisher"
	"github.com/caesar/all-chat/services/alert-processor/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"go.uber.org/zap"
)

// Store is the repository slice the pipeline needs — a seam so the fan-out
// and failure contracts are testable without Postgres.
type Store interface {
	FindAlertOverlays(ctx context.Context, platform, channelID string) ([]repository.AlertOverlay, error)
	InsertAlertEvent(ctx context.Context, alert *models.Alert) error
}

// Processor runs the alert pipeline for one raw message at a time.
type Processor struct {
	store       Store
	publisher   *publisher.Publisher
	leaderboard *leaderboard.Client
	log         *zap.Logger
}

// New creates a Processor.
func New(store Store, pub *publisher.Publisher, boards *leaderboard.Client, log *zap.Logger) *Processor {
	return &Processor{store: store, publisher: pub, leaderboard: boards, log: log}
}

// Handle routes one event-typed message to its alert overlays.
//
// Error contract: any error means the consumer must leave the stream entry
// unacked so it is redelivered (at-least-once). Normalization failures are the
// one exception — they are deterministic for a given message, so retrying can
// never succeed and would wedge the consumer group; those are logged and
// dropped with a nil error.
//
// Per-overlay ordering is persist → publish → leaderboard on purpose: the
// persist is idempotent (deterministic alert id + ON CONFLICT), so a
// redelivery after a later failure cannot double-write history, and the
// leaderboard write is claimed once per alert id, so a redelivery cannot
// double-count a board that already scored. A publish before persist would
// animate overlays on an event history does not record. The remaining
// duplicate window is the usual at-least-once one — a redelivered alert can
// reach subscribers twice — which the spec accepts.
func (p *Processor) Handle(ctx context.Context, raw *mpmodels.RawChatMessage) error {
	info, user, err := normalizer.Normalize(raw)
	if err != nil {
		p.log.Warn("Dropping event the normalizers cannot decode",
			zap.String("platform", raw.Platform),
			zap.String("message_id", raw.MessageID),
			zap.String("event_type", raw.EventType),
			zap.Error(err),
		)
		return nil
	}

	overlays, err := p.store.FindAlertOverlays(ctx, raw.Platform, raw.ChannelID)
	if err != nil {
		return fmt.Errorf("route %s/%s event: %w", raw.Platform, raw.EventType, err)
	}

	for _, overlay := range overlays {
		alert := &models.Alert{
			AlertID:    models.NewAlertID(raw.Platform, raw.ChannelID, raw.MessageID, overlay.OverlayID),
			OverlayID:  overlay.OverlayID,
			Platform:   raw.Platform,
			EventType:  raw.EventType,
			EventData:  info,
			User:       models.AlertUserFrom(user),
			OccurredAt: raw.Timestamp,
		}
		metrics.RecordRouted(raw.Platform, raw.EventType)

		if err := p.store.InsertAlertEvent(ctx, alert); err != nil {
			return fmt.Errorf("persist alert %s: %w", alert.AlertID, err)
		}
		metrics.RecordPersisted(raw.Platform, raw.EventType)

		if err := p.publisher.Publish(ctx, alert); err != nil {
			metrics.RecordPublishError(raw.Platform, raw.EventType)
			return fmt.Errorf("publish alert %s: %w", alert.AlertID, err)
		}

		if overlay.OverlayType == "list" {
			if err := p.recordLeaderboardAmount(ctx, alert); err != nil {
				return err
			}
		}
	}

	return nil
}

// recordLeaderboardAmount scores amount-bearing events on the 'list'
// overlay's board. Leaderboards are list-only: 'alerts' and 'goal' overlays
// render events but do not rank their senders.
func (p *Processor) recordLeaderboardAmount(ctx context.Context, alert *models.Alert) error {
	category, ok := leaderboard.Category(alert.EventType)
	if !ok {
		return nil
	}
	info := alert.EventData
	if info == nil || info.Value == nil || info.Value.Amount <= 0 {
		return nil // not an amount-bearing delivery (e.g. a 0-gift mystery gift)
	}
	if alert.User.ID == "" {
		return nil // unattributable spend cannot rank a user
	}

	member := leaderboard.Member(alert.Platform, alert.User.ID)
	if err := p.leaderboard.RecordAmount(ctx, alert.OverlayID, category, member, info.Value.Amount, alert.AlertID); err != nil {
		return fmt.Errorf("record leaderboard amount for overlay %s: %w", alert.OverlayID, err)
	}
	return nil
}
