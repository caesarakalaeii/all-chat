package listener

import (
	"math/rand"
	"time"
)

// JitteredBackoff returns a randomized duration using the full jitter algorithm.
// Formula: random_between(0, min(cap, base * 2^attempt))
// Base: 1s, Cap: 30s.
//
// This prevents thundering herd when many goroutines retry simultaneously.
// Use attempt=0 for the first retry, incrementing on each subsequent failure.
func JitteredBackoff(attempt int) time.Duration {
	const base = time.Second
	const cap = 30 * time.Second

	exp := base
	for i := 0; i < attempt; i++ {
		exp *= 2
		if exp > cap {
			exp = cap
			break
		}
	}

	if exp <= 0 {
		return 0
	}

	return time.Duration(rand.Int63n(int64(exp)))
}
