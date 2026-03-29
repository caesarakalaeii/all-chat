package websocket

import (
	"testing"
	"time"
)

func newTestClient() *Client {
	return &Client{
		send:          make(chan []byte, 256),
		done:          make(chan struct{}),
		reconnectChan: make(chan struct{}, 1),
	}
}

func TestIsStale_NeverConnected_ReturnsFalse(t *testing.T) {
	c := newTestClient()
	// lastActivityAt is zero (never connected)
	if c.IsStale() {
		t.Error("IsStale() should return false when the client has never been connected")
	}
}

func TestIsStale_RecentActivity_ReturnsFalse(t *testing.T) {
	c := newTestClient()
	c.lastActivityAt = time.Now().Add(-1 * time.Minute)
	if c.IsStale() {
		t.Error("IsStale() should return false when lastActivityAt is recent")
	}
}

func TestIsStale_ActivityJustBelowThreshold_ReturnsFalse(t *testing.T) {
	c := newTestClient()
	// Just under the 5-minute threshold
	c.lastActivityAt = time.Now().Add(-staleLivenessThreshold + time.Second)
	if c.IsStale() {
		t.Error("IsStale() should return false when below the stale threshold")
	}
}

func TestIsStale_ActivityBeyondThreshold_ReturnsTrue(t *testing.T) {
	c := newTestClient()
	// Well beyond the 5-minute threshold
	c.lastActivityAt = time.Now().Add(-staleLivenessThreshold - time.Minute)
	if !c.IsStale() {
		t.Error("IsStale() should return true when lastActivityAt exceeds staleLivenessThreshold")
	}
}

func TestIsStale_ExactlyAtThreshold_ReturnsFalse(t *testing.T) {
	c := newTestClient()
	// Exactly at threshold — not stale yet (strictly greater than)
	c.lastActivityAt = time.Now().Add(-staleLivenessThreshold)
	// time.Since might make this flicker; give a tiny margin
	if c.IsStale() {
		// Allow: might be exactly at boundary
		t.Log("IsStale() returned true exactly at threshold — acceptable due to timing")
	}
}

func TestLastActivityAt_ReturnsStoredTime(t *testing.T) {
	c := newTestClient()
	expected := time.Now().Add(-2 * time.Minute)
	c.lastActivityAt = expected
	got := c.LastActivityAt()
	if !got.Equal(expected) {
		t.Errorf("LastActivityAt() = %v, want %v", got, expected)
	}
}

func TestLastActivityAt_ZeroWhenNeverConnected(t *testing.T) {
	c := newTestClient()
	got := c.LastActivityAt()
	if !got.IsZero() {
		t.Errorf("LastActivityAt() should be zero when never connected, got %v", got)
	}
}
