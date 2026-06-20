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

// Package reconcile periodically re-queries Patreon for every connected user to
// refresh tokens and catch membership changes that arrived via a missed webhook
// (e.g. a cancellation Patreon never delivered). It is the convergence backstop
// for the webhook path. Run as a SINGLE replica.
package reconcile

import (
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/payment-service/entitlement"
	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"go.uber.org/zap"
)

// Manager runs the periodic reconciliation loop.
type Manager struct {
	oauth         *patreon.OAuth
	tokenRepo     *repository.TokenRepository
	entitlement   *entitlement.Service
	campaignID    string
	interval      time.Duration
	refreshBuffer time.Duration
	batchSize     int
	logger        *zap.Logger

	mu        sync.Mutex
	isRunning bool
}

// NewManager builds a reconcile Manager.
func NewManager(oauth *patreon.OAuth, tokenRepo *repository.TokenRepository, ent *entitlement.Service, campaignID string, interval, refreshBuffer time.Duration, batchSize int, logger *zap.Logger) *Manager {
	return &Manager{
		oauth:         oauth,
		tokenRepo:     tokenRepo,
		entitlement:   ent,
		campaignID:    campaignID,
		interval:      interval,
		refreshBuffer: refreshBuffer,
		batchSize:     batchSize,
		logger:        logger,
	}
}

// Start runs the loop until ctx is cancelled, processing immediately on startup
// and then every interval.
func (m *Manager) Start(ctx context.Context) error {
	m.logger.Info("Starting Patreon reconcile manager",
		zap.Duration("interval", m.interval),
		zap.Int("batch_size", m.batchSize))

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	if err := m.ProcessBatch(ctx); err != nil {
		m.logger.Error("Initial reconcile batch failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Patreon reconcile manager stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := m.ProcessBatch(ctx); err != nil {
				m.logger.Error("Reconcile batch failed", zap.Error(err))
			}
		}
	}
}

// ProcessBatch refreshes tokens nearing expiry and re-queries membership for each
// connected user, applying the result (which revokes premium for lapsed patrons).
func (m *Manager) ProcessBatch(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		m.logger.Warn("Reconcile already in progress, skipping")
		return nil
	}
	m.isRunning = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.isRunning = false
		m.mu.Unlock()
	}()

	tokens, err := m.tokenRepo.ListAll(ctx, m.batchSize)
	if err != nil {
		return err
	}

	var reconciled, failed int
	for _, t := range tokens {
		accessToken := t.AccessToken

		// Refresh proactively if the access token is at/near expiry.
		if time.Until(t.ExpiresAt) <= m.refreshBuffer {
			newTok, refreshErr := m.oauth.RefreshToken(ctx, t.RefreshToken)
			if refreshErr != nil {
				m.logger.Warn("Failed to refresh Patreon token; skipping subject this pass",
					subjectLabel(t), zap.Error(refreshErr))
				failed++
				continue
			}
			if updErr := m.tokenRepo.UpdateTokens(ctx, t.PatreonUserID, newTok); updErr != nil {
				m.logger.Error("Failed to persist refreshed Patreon token",
					subjectLabel(t), zap.Error(updErr))
			}
			accessToken = newTok.AccessToken
		}

		snap, qErr := m.oauth.GetIdentityWithMembership(ctx, accessToken, m.campaignID)
		if qErr != nil {
			m.logger.Warn("Failed to re-query Patreon membership",
				subjectLabel(t), zap.Error(qErr))
			failed++
			continue
		}

		if _, _, applyErr := m.entitlement.Apply(ctx, snap, t.UserID, t.ViewerID, nil); applyErr != nil {
			m.logger.Error("Failed to apply reconciled membership",
				subjectLabel(t), zap.Error(applyErr))
			failed++
			continue
		}
		reconciled++
	}

	m.logger.Info("Reconcile batch complete",
		zap.Int("reconciled", reconciled),
		zap.Int("failed", failed),
		zap.Int("total", len(tokens)))
	return nil
}

// subjectLabel returns a log field identifying the connection's subject.
func subjectLabel(t repository.PatreonToken) zap.Field {
	switch {
	case t.ViewerID != nil && *t.ViewerID != "":
		return zap.String("viewer_id", *t.ViewerID)
	case t.UserID != nil && *t.UserID != "":
		return zap.String("user_id", *t.UserID)
	default:
		return zap.String("subject", "unknown")
	}
}
