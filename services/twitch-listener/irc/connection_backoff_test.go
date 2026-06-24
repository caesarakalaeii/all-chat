// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package irc

import (
	"testing"
	"time"
)

// TestNextBackoff_Attempt0_BelowOneSecond verifies the first retry uses the
// base delay (1s full jitter → result in [0, 1s)).
func TestNextBackoff_Attempt0_BelowOneSecond(t *testing.T) {
	for i := 0; i < 200; i++ {
		d := nextBackoff(0)
		if d < 0 {
			t.Fatalf("nextBackoff(0) returned negative: %v", d)
		}
		if d >= time.Second {
			t.Fatalf("nextBackoff(0) = %v, want < 1s (base=1s, full jitter)", d)
		}
	}
}

// TestNextBackoff_NeverExceedsCap verifies the 30s cap is never breached,
// regardless of how large attempt grows.
func TestNextBackoff_NeverExceedsCap(t *testing.T) {
	const cap30 = 30 * time.Second
	for _, attempt := range []int{0, 1, 2, 3, 4, 5, 10, 50, 100, 1000} {
		for i := 0; i < 200; i++ {
			d := nextBackoff(attempt)
			if d < 0 {
				t.Fatalf("nextBackoff(%d) returned negative: %v", attempt, d)
			}
			if d >= cap30 {
				t.Fatalf("nextBackoff(%d) = %v, want < 30s (cap)", attempt, d)
			}
		}
	}
}

// TestNextBackoff_EscalatesUpToCap verifies that the backoff escalates across
// consecutive failures: the maximum possible delay at a higher attempt must
// be at least as large as the maximum at a lower attempt, up to the 30s cap.
// This is the core invariant the M7 fix restores — before the fix,
// JitteredBackoff was always called with attempt=0 so delays never grew.
//
// Because JitteredBackoff uses full jitter (random in [0, upperBound)), we
// sample many values and assert the observed maxima are monotonically
// non-decreasing up to the cap.
func TestNextBackoff_EscalatesUpToCap(t *testing.T) {
	const samples = 500
	const cap30 = 30 * time.Second

	// Upper bounds per attempt (math: min(cap, base * 2^attempt)):
	//   0 → 1s, 1 → 2s, 2 → 4s, 3 → 8s, 4 → 16s, 5 → 30s(cap), 6+ → 30s(cap)
	upperBounds := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		cap30,
		cap30,
	}

	var prevMax time.Duration
	for attempt, ub := range upperBounds {
		var maxObserved time.Duration
		for i := 0; i < samples; i++ {
			d := nextBackoff(attempt)
			if d > maxObserved {
				maxObserved = d
			}
		}
		// The observed maximum should approach the theoretical upper bound.
		// With 500 samples the expected max is ~99.8% of the bound (order
		// statistics of uniform), so assert > 80% to avoid flakiness.
		threshold := ub * 4 / 5
		if maxObserved < threshold && ub > 0 {
			t.Errorf("nextBackoff(%d): max observed %v, expected > %v (80%% of upper bound %v)",
				attempt, maxObserved, threshold, ub)
		}
		// Monotonic non-decreasing — but ONLY before the cap is reached.
		// Once the cap applies (attempt >= 5), both levels draw from the same
		// [0, 30s) range so their observed maxima are statistically
		// indistinguishable and must not be compared.
		if attempt > 0 && ub < cap30 && maxObserved < prevMax {
			t.Errorf("nextBackoff(%d) max %v < nextBackoff(%d) max %v — backoff not escalating",
				attempt-1, prevMax, attempt, maxObserved)
		}
		if ub < cap30 {
			prevMax = maxObserved
		}
	}

	// At the cap (attempt 5+), the observed max should be very close to 30s.
	var capMax time.Duration
	for i := 0; i < samples; i++ {
		if d := nextBackoff(10); d > capMax {
			capMax = d
		}
	}
	if capMax < 24*time.Second {
		t.Errorf("nextBackoff at cap: max observed %v, expected close to 30s", capMax)
	}
}

// TestNextBackoff_NeverNegative verifies no attempt value produces a negative
// duration (defensive — JitteredBackoff guards against this).
func TestNextBackoff_NeverNegative(t *testing.T) {
	for attempt := 0; attempt <= 100; attempt++ {
		for i := 0; i < 20; i++ {
			if d := nextBackoff(attempt); d < 0 {
				t.Fatalf("nextBackoff(%d) returned negative: %v", attempt, d)
			}
		}
	}
}
