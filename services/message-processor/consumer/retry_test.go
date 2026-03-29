package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryOp_SucceedsOnFirstCall(t *testing.T) {
	calls := 0
	err := retryOp(context.Background(), func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
}

func TestRetryOp_SucceedsAfterTwoFailures(t *testing.T) {
	calls := 0
	err := retryOp(context.Background(), func() error {
		calls++
		if calls < 3 {
			return errors.New("transient error")
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRetryOp_ReturnsLastErrorAfterThreeFailures(t *testing.T) {
	calls := 0
	sentinel := errors.New("permanent error")
	err := retryOp(context.Background(), func() error {
		calls++
		return sentinel
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, sentinel)
	assert.Equal(t, 3, calls)
}

func TestRetryOp_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	// Cancel context after first failure so second attempt sees cancelled context
	err := retryOp(ctx, func() error {
		calls++
		cancel() // cancel after first call
		return errors.New("error")
	})

	// Should return context error (not the transient error)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	// Only called once since context was cancelled before sleep could complete
	assert.Equal(t, 1, calls)
}

func TestRetryOp_DelaysAreReasonable(t *testing.T) {
	// Verify the function completes in finite time (with very short real delays for testing)
	// Just ensure it doesn't hang
	start := time.Now()
	calls := 0
	err := retryOp(context.Background(), func() error {
		calls++
		return errors.New("always fails")
	})
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, 3, calls)
	// Should be at least 600ms (100ms + 500ms delays) but not too long
	assert.GreaterOrEqual(t, elapsed, 600*time.Millisecond)
	assert.Less(t, elapsed, 10*time.Second)
}
