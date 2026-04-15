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

package poller

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
)

// Backoff manages exponential backoff for transient errors.
// Configuration:
//   - Initial interval: 2 seconds
//   - Multiplier: 2.0 (doubles each retry: 2s → 4s → 8s → 16s → 32s → 60s)
//   - Max interval: 60 seconds (capped)
//   - Max elapsed time: 0 (infinite retries, never stop)
//   - Jitter: enabled by default in backoff/v4
//
// Retry strategy:
//   - Transient errors (429, 5xx, network): wait with exponential backoff
//   - Fatal errors (401, 403, 404): return immediately without waiting
//   - Successful operation: reset backoff to initial interval (2s)
type Backoff struct {
	policy *backoff.ExponentialBackOff
	logger *zap.Logger
}

// NewBackoff creates a backoff state machine with user-specified parameters.
// Parameters are fixed (not configurable) per user decision for PoC phase:
//   - InitialInterval = 2s (start conservatively)
//   - MaxInterval = 60s (cap to prevent excessive delays)
//   - Multiplier = 2.0 (standard exponential backoff)
//   - MaxElapsedTime = 0 (infinite retries, never give up)
func NewBackoff(logger *zap.Logger) *Backoff {
	policy := backoff.NewExponentialBackOff()

	// User-specified configuration (fixed for PoC)
	policy.InitialInterval = 2 * time.Second
	policy.MaxInterval = 60 * time.Second
	policy.Multiplier = 2.0
	policy.MaxElapsedTime = 0 // Infinite retries

	// Jitter is enabled by default in backoff/v4
	// RandomizationFactor defaults to 0.5 (adds ±25% jitter)

	policy.Reset()

	return &Backoff{
		policy: policy,
		logger: logger,
	}
}

// Wait blocks until the backoff duration completes or context is cancelled.
// Logic:
//   - Fatal errors: return immediately without waiting
//   - Transient errors: wait for backoff duration, then return nil
//   - Context cancellation: return ctx.Err() immediately
//
// Backoff progression (with jitter):
//   - 1st retry: ~2s (1.5s - 2.5s with jitter)
//   - 2nd retry: ~4s (3s - 5s)
//   - 3rd retry: ~8s (6s - 10s)
//   - 4th retry: ~16s (12s - 20s)
//   - 5th retry: ~32s (24s - 40s)
//   - 6th+ retry: ~60s (45s - 75s, capped at MaxInterval)
func (b *Backoff) Wait(ctx context.Context, err error) error {
	// Fatal errors: return immediately, no backoff
	if innertube.IsFatalError(err) {
		b.logger.Error("Fatal error encountered, no backoff",
			zap.Error(err),
			zap.String("error_type", "fatal"))
		return err
	}

	// Transient errors: apply exponential backoff
	if innertube.IsTransientError(err) {
		duration := b.policy.NextBackOff()

		b.logger.Warn("Transient error, backing off",
			zap.Error(err),
			zap.Duration("backoff_duration", duration),
			zap.String("error_type", "transient"))

		// Sleep with context cancellation support
		select {
		case <-time.After(duration):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Unknown error type (should not happen): log and return immediately
	b.logger.Warn("Unknown error type, no backoff applied",
		zap.Error(err))
	return err
}

// Reset resets the backoff to the initial interval (2s).
// Called after a successful operation to clear accumulated backoff state.
func (b *Backoff) Reset() {
	b.policy.Reset()
	b.logger.Debug("Backoff reset after successful operation")
}
