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

// Package zombie provides received-vs-published drift detection for Twitch listener pods.
//
// A zombie pod is one that appears alive (IRC connected, process running) but has
// stopped delivering messages to Redis. This is detected by comparing two atomic
// counters: messagesReceived (incremented on each IRC PRIVMSG) and messagesPublished
// (incremented after each successful ring buffer enqueue).
//
// If messages are received from IRC but publishing stalls for stallWindow (default 5
// minutes), IsZombie() returns true and the liveness probe should fail — causing
// Kubernetes to restart the pod.
//
// False positive avoidance (D-10): when both counters are zero (offline streamer),
// IsZombie() returns false. Source-manager keeps sources "active" even when streamers
// are offline, so a pod with no messages flowing must not be treated as a zombie.
package zombie

import (
	"sync/atomic"
	"time"
)

// Detector tracks message received-vs-published drift.
// All methods are safe to call from multiple goroutines.
type Detector struct {
	messagesReceived  atomic.Int64
	messagesPublished atomic.Int64

	stallWindow     time.Duration
	lastSnapshot    snapshot
	snapshotTakenAt time.Time
}

// snapshot captures counter values at a point in time.
type snapshot struct {
	received  int64
	published int64
}

// NewDetector creates a zombie detector with the given stall window.
// Recommended default: 5 minutes.
func NewDetector(stallWindow time.Duration) *Detector {
	return &Detector{
		stallWindow:     stallWindow,
		snapshotTakenAt: time.Now(),
	}
}

// RecordReceived increments the received counter. Call this in the IRC PRIVMSG callback.
func (d *Detector) RecordReceived() { d.messagesReceived.Add(1) }

// RecordPublished increments the published counter. Call this after a successful ring
// buffer enqueue (not after XADD — the ring buffer accept is the relevant signal).
func (d *Detector) RecordPublished() { d.messagesPublished.Add(1) }

// IsZombie returns true if messages are being received from IRC but publishing has
// stalled for at least stallWindow/2. Returns false when both counters are zero (offline
// channel — D-10 false positive avoidance).
//
// Snapshot rotation: a new snapshot is taken every stallWindow/2. The evaluation
// compares current counters against the last snapshot. If not enough time has passed
// since the last snapshot (< stallWindow/2), we return false — not enough data to
// determine a stall. Once stallWindow/2 has elapsed, we evaluate the delta then rotate.
func (d *Detector) IsZombie() bool {
	now := time.Now()
	currentReceived := d.messagesReceived.Load()
	currentPublished := d.messagesPublished.Load()

	elapsed := now.Sub(d.snapshotTakenAt)

	// Not enough time has passed since the last snapshot — cannot evaluate a stall yet.
	if elapsed < d.stallWindow/2 {
		return false
	}

	// Evaluate drift against the snapshot taken stallWindow/2 ago.
	// Both zero = offline streamer (D-10): no drift, no zombie.
	if currentReceived == 0 && d.lastSnapshot.received == 0 {
		// Rotate snapshot before returning so we're ready for the next window.
		d.lastSnapshot = snapshot{received: currentReceived, published: currentPublished}
		d.snapshotTakenAt = now
		return false
	}

	// Compute deltas since the last snapshot.
	receivedDelta := currentReceived - d.lastSnapshot.received
	publishedDelta := currentPublished - d.lastSnapshot.published

	// Rotate snapshot for the next evaluation window.
	d.lastSnapshot = snapshot{received: currentReceived, published: currentPublished}
	d.snapshotTakenAt = now

	// Zombie condition: messages received but publishing has stalled.
	return receivedDelta > 0 && publishedDelta == 0
}
