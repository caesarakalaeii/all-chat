package listener

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJitteredBackoff_Attempt0(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := JitteredBackoff(0)
		assert.GreaterOrEqual(t, d, time.Duration(0), "JitteredBackoff(0) must not be negative")
		assert.Less(t, d, time.Second, "JitteredBackoff(0) must be < 1s (base=1s, full jitter)")
	}
}

func TestJitteredBackoff_Attempt1(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := JitteredBackoff(1)
		assert.GreaterOrEqual(t, d, time.Duration(0), "JitteredBackoff(1) must not be negative")
		assert.Less(t, d, 2*time.Second, "JitteredBackoff(1) must be < 2s (2^1 * base=2s, full jitter)")
	}
}

func TestJitteredBackoff_Attempt5_CapReached(t *testing.T) {
	// 2^5 * 1s = 32s > cap(30s) so result must be in [0, 30s)
	for i := 0; i < 100; i++ {
		d := JitteredBackoff(5)
		assert.GreaterOrEqual(t, d, time.Duration(0), "JitteredBackoff(5) must not be negative")
		assert.Less(t, d, 30*time.Second, "JitteredBackoff(5) must be < 30s (cap)")
	}
}

func TestJitteredBackoff_Attempt100_NeverExceedsCap(t *testing.T) {
	for i := 0; i < 100; i++ {
		d := JitteredBackoff(100)
		assert.GreaterOrEqual(t, d, time.Duration(0), "JitteredBackoff(100) must not be negative")
		assert.Less(t, d, 30*time.Second, "JitteredBackoff(100) must be < 30s (cap)")
	}
}

func TestJitteredBackoff_NeverNegative(t *testing.T) {
	for attempt := 0; attempt <= 50; attempt++ {
		for i := 0; i < 20; i++ {
			d := JitteredBackoff(attempt)
			assert.GreaterOrEqual(t, d, time.Duration(0),
				"JitteredBackoff(%d) returned negative duration: %v", attempt, d)
		}
	}
}
