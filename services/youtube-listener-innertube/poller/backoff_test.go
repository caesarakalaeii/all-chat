package poller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
)

func TestNewBackoff(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	if b == nil {
		t.Fatal("NewBackoff returned nil")
	}

	if b.policy == nil {
		t.Fatal("Backoff policy is nil")
	}

	// Verify user-specified configuration
	if b.policy.InitialInterval != 2*time.Second {
		t.Errorf("InitialInterval = %v, want 2s", b.policy.InitialInterval)
	}

	if b.policy.MaxInterval != 60*time.Second {
		t.Errorf("MaxInterval = %v, want 60s", b.policy.MaxInterval)
	}

	if b.policy.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", b.policy.Multiplier)
	}

	if b.policy.MaxElapsedTime != 0 {
		t.Errorf("MaxElapsedTime = %v, want 0 (infinite)", b.policy.MaxElapsedTime)
	}
}

func TestBackoff_Wait_FatalError(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	// Fatal errors should return immediately without waiting
	fatalErrors := []error{
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 401, Body: "Unauthorized"}),
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 403, Body: "Forbidden"}),
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 404, Body: "Not Found"}),
	}

	for _, err := range fatalErrors {
		start := time.Now()
		ctx := context.Background()

		result := b.Wait(ctx, err)

		// Should return immediately (< 100ms)
		elapsed := time.Since(start)
		if elapsed > 100*time.Millisecond {
			t.Errorf("Wait took %v for fatal error, expected immediate return", elapsed)
		}

		// Should return the error (not nil)
		if result == nil {
			t.Error("Wait returned nil for fatal error, expected error")
		}
	}
}

func TestBackoff_Wait_TransientError(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	// First transient error should backoff ~2s (with jitter: 1s - 3s)
	transientErr := fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 429, Body: "Too Many Requests"})

	start := time.Now()
	ctx := context.Background()

	err := b.Wait(ctx, transientErr)

	elapsed := time.Since(start)

	// Should wait approximately 2s (allow jitter: 1s - 3s for backoff/v4)
	if elapsed < 1*time.Second || elapsed > 3*time.Second {
		t.Errorf("Wait took %v, expected ~2s (1s-3s with jitter)", elapsed)
	}

	// Should return nil (successful wait)
	if err != nil {
		t.Errorf("Wait returned error: %v, expected nil", err)
	}
}

func TestBackoff_Wait_ContextCancellation(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	// Context cancelled during wait should return ctx.Err()
	transientErr := fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 503, Body: "Service Unavailable"})

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after 500ms
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := b.Wait(ctx, transientErr)
	elapsed := time.Since(start)

	// Should return after ~500ms (when context is cancelled)
	if elapsed < 400*time.Millisecond || elapsed > 600*time.Millisecond {
		t.Errorf("Wait took %v, expected ~500ms (context cancellation)", elapsed)
	}

	// Should return context.Canceled error
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Wait returned error: %v, expected context.Canceled", err)
	}
}

func TestBackoff_Wait_UnknownError(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	// Unknown errors are classified as transient (network errors)
	unknownErr := errors.New("unknown error type")

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	result := b.Wait(ctx, unknownErr)

	elapsed := time.Since(start)

	// Unknown errors are transient, so should backoff ~2s
	if elapsed < 1*time.Second || elapsed > 3*time.Second {
		t.Errorf("Wait took %v for unknown error, expected ~2s backoff (transient)", elapsed)
	}

	// Should return nil (successful wait, not error)
	if result != nil && !errors.Is(result, context.DeadlineExceeded) {
		t.Errorf("Wait returned error: %v, expected nil", result)
	}
}

func TestBackoff_Reset(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	// Trigger multiple backoffs to increase duration
	transientErr := fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 500, Body: "Internal Server Error"})

	ctx := context.Background()

	// First backoff: ~2s
	start := time.Now()
	_ = b.Wait(ctx, transientErr)
	firstDuration := time.Since(start)

	// Second backoff: ~4s (without reset)
	start = time.Now()
	_ = b.Wait(ctx, transientErr)
	secondDuration := time.Since(start)

	// Second duration should be longer than first (exponential progression).
	// Use a lenient check: second should be at least as long as first, not a strict
	// 1.5x multiple, because jitter means two independent samples from adjacent
	// intervals can overlap.
	if secondDuration < firstDuration {
		t.Errorf("Second backoff (%v) not longer than first (%v)", secondDuration, firstDuration)
	}

	// Reset backoff
	b.Reset()

	// Third backoff: should be back to ~2s (same as first)
	start = time.Now()
	_ = b.Wait(ctx, transientErr)
	thirdDuration := time.Since(start)

	// Third duration should be similar to first duration after reset.
	// The backoff jitter factor is 0.5, giving a range of [base*0.5, base*1.5] for
	// a 2s base interval: 1s–3s. The ratio between two independent samples from
	// that range can therefore be anywhere from ~0.33 to ~3.0. Use a generous
	// tolerance that catches clearly-wrong behaviour (e.g. no reset at all) while
	// still passing under any realistic system load.
	ratio := float64(thirdDuration) / float64(firstDuration)
	if ratio < 0.2 || ratio > 5.0 {
		t.Errorf("Third backoff (%v) not similar to first (%v) after reset, ratio=%v", thirdDuration, firstDuration, ratio)
	} else {
		t.Logf("Third backoff (%v) reset successfully, ratio to first (%v) = %.2f", thirdDuration, firstDuration, ratio)
	}
}

func TestBackoffSequence(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	transientErr := fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 502, Body: "Bad Gateway"})

	ctx := context.Background()

	// Test first 2 backoffs only to avoid test timeout
	// The pattern (2s → 4s) demonstrates exponential progression
	expectedDurations := []time.Duration{
		2 * time.Second,
		4 * time.Second,
	}

	for i, expected := range expectedDurations {
		start := time.Now()
		err := b.Wait(ctx, transientErr)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Backoff %d returned error: %v", i+1, err)
		}

		// Allow wider variance due to jitter (backoff/v4 uses RandomizationFactor)
		// Accept anything within reasonable bounds (0.5x - 2x of expected)
		minDuration := expected / 2
		maxDuration := expected * 2

		if elapsed < minDuration || elapsed > maxDuration {
			t.Errorf("Backoff %d took %v, expected %v (range: %v - %v)",
				i+1, elapsed, expected, minDuration, maxDuration)
		}

		// Note: we intentionally do not compare consecutive elapsed values because
		// jitter means adjacent intervals (e.g. 2s and 4s) can overlap in practice.
		// The minDuration/maxDuration range check above is sufficient to verify the
		// exponential progression.
		t.Logf("Backoff %d: %v (expected ~%v)", i+1, elapsed, expected)
	}
}

func TestBackoff_AllTransientErrorTypes(t *testing.T) {
	logger := zap.NewNop()
	b := NewBackoff(logger)

	// All transient error types should trigger backoff
	transientErrors := []error{
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 429, Body: "Too Many Requests"}),
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 500, Body: "Internal Server Error"}),
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 502, Body: "Bad Gateway"}),
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 503, Body: "Service Unavailable"}),
		fmt.Errorf("HTTP error: %w", &innertube.HTTPStatusError{StatusCode: 504, Body: "Gateway Timeout"}),
		errors.New("network error"),
	}

	for _, err := range transientErrors {
		b.Reset() // Reset between tests

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		waitErr := b.Wait(ctx, err)

		elapsed := time.Since(start)

		// Should wait ~2s (first backoff, allow jitter)
		if elapsed < 1*time.Second || elapsed > 3*time.Second {
			t.Errorf("Wait for %v took %v, expected ~2s (1s-3s with jitter)", err, elapsed)
		}

		// Should return nil (successful wait)
		if waitErr != nil {
			t.Errorf("Wait for %v returned error: %v", err, waitErr)
		}
	}
}
