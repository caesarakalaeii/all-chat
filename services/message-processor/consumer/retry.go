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

package consumer

import (
	"context"
	"time"
)

// retryDelays defines the wait durations between retry attempts (per D-04).
var retryDelays = []time.Duration{
	100 * time.Millisecond,
	500 * time.Millisecond,
	2 * time.Second,
}

// retryOp calls fn up to 3 times with exponential backoff delays (100ms, 500ms, 2s).
// Returns nil on first success. Returns the last error if all attempts fail.
// Respects ctx cancellation — if ctx is cancelled before a sleep completes, returns ctx.Err().
func retryOp(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// If this was the last attempt, don't sleep
		if attempt == 2 {
			break
		}

		// Wait with context cancellation support
		delay := retryDelays[attempt]
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// continue to next attempt
		}
	}
	return lastErr
}
