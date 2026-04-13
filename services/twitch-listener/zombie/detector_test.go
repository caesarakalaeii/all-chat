package zombie

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDetector_BothZero_NotZombie verifies that when both received and published
// are zero (offline channel), IsZombie returns false (D-10 false positive avoidance).
func TestDetector_BothZero_NotZombie(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	// Fast-forward the snapshot to simulate a full window having elapsed.
	d.snapshotTakenAt = time.Now().Add(-6 * time.Minute)

	assert.False(t, d.IsZombie(), "both received=0 and published=0 must not trigger zombie (offline channel)")
}

// TestDetector_Healthy_AdvancingPublished_NotZombie verifies that when received > 0
// and published is also advancing, IsZombie returns false.
func TestDetector_Healthy_AdvancingPublished_NotZombie(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	d.snapshotTakenAt = time.Now().Add(-6 * time.Minute)
	d.lastSnapshot = snapshot{received: 10, published: 10}

	d.RecordReceived()
	d.RecordReceived()
	d.RecordPublished()
	d.RecordPublished()

	assert.False(t, d.IsZombie(), "advancing published counter means healthy, not zombie")
}

// TestDetector_StallDetected_IsZombie verifies that when received > 0 but published
// has not advanced since the last snapshot, IsZombie returns true.
func TestDetector_StallDetected_IsZombie(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	// Simulate: snapshot was taken 3 minutes ago with published=5
	d.snapshotTakenAt = time.Now().Add(-3 * time.Minute)
	d.lastSnapshot = snapshot{received: 5, published: 5}

	// New messages received since snapshot but published hasn't advanced.
	d.messagesReceived.Store(10) // delta = 5 since snapshot
	d.messagesPublished.Store(5) // delta = 0 since snapshot

	assert.True(t, d.IsZombie(), "received delta > 0 with published delta == 0 must trigger zombie")
}

// TestDetector_NormalLag_NotZombie verifies that when published < received (normal lag)
// but published IS advancing, IsZombie returns false. Only a stall triggers zombie.
func TestDetector_NormalLag_NotZombie(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	d.snapshotTakenAt = time.Now().Add(-3 * time.Minute)
	d.lastSnapshot = snapshot{received: 100, published: 80}

	// More messages received and published since snapshot (both advancing).
	d.messagesReceived.Store(200) // delta = 100
	d.messagesPublished.Store(90) // delta = 10 — lagging but not stalled

	assert.False(t, d.IsZombie(), "published advancing (delta>0) with lag is healthy, not zombie")
}

// TestDetector_RecordReceived_Increments verifies RecordReceived increments the atomic counter.
func TestDetector_RecordReceived_Increments(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	assert.Equal(t, int64(0), d.messagesReceived.Load())

	d.RecordReceived()
	d.RecordReceived()
	d.RecordReceived()

	assert.Equal(t, int64(3), d.messagesReceived.Load())
}

// TestDetector_RecordPublished_Increments verifies RecordPublished increments the atomic counter.
func TestDetector_RecordPublished_Increments(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	assert.Equal(t, int64(0), d.messagesPublished.Load())

	d.RecordPublished()
	d.RecordPublished()

	assert.Equal(t, int64(2), d.messagesPublished.Load())
}

// TestDetector_SnapshotRotation verifies that IsZombie rotates the snapshot when
// stallWindow/2 has elapsed since the last snapshot.
func TestDetector_SnapshotRotation(t *testing.T) {
	stallWindow := 10 * time.Second // short for testing
	d := NewDetector(stallWindow)

	// Set an old snapshot time so rotation is triggered.
	d.snapshotTakenAt = time.Now().Add(-6 * time.Second) // > stallWindow/2 (5s)
	d.lastSnapshot = snapshot{received: 5, published: 5}
	d.messagesReceived.Store(10)
	d.messagesPublished.Store(10)

	// IsZombie should rotate the snapshot and return false (published is advancing).
	result := d.IsZombie()
	assert.False(t, result)

	// After rotation, snapshotTakenAt should be approximately now.
	assert.WithinDuration(t, time.Now(), d.snapshotTakenAt, time.Second,
		"snapshot should be rotated to approximately now")
	// The new snapshot should reflect current counter values.
	assert.Equal(t, int64(10), d.lastSnapshot.received)
	assert.Equal(t, int64(10), d.lastSnapshot.published)
}

// TestDetector_NotEnoughTimeElapsed_NotZombie verifies that IsZombie returns false
// when the snapshot was taken very recently (not enough time for a stall window).
func TestDetector_NotEnoughTimeElapsed_NotZombie(t *testing.T) {
	d := NewDetector(5 * time.Minute)
	// snapshotTakenAt is recent — not enough time to evaluate a stall.
	d.snapshotTakenAt = time.Now().Add(-1 * time.Minute)
	d.lastSnapshot = snapshot{received: 0, published: 0}

	d.messagesReceived.Store(100)
	d.messagesPublished.Store(0)

	// Even though published delta is 0, we haven't waited long enough.
	assert.False(t, d.IsZombie(), "must not fire zombie before stallWindow/2 elapses")
}
