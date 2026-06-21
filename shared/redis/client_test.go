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

package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// fastRetryOpts returns retry options with millisecond backoffs so tests don't sleep.
func fastRetryOpts(maxAttempts int) RetryOptions {
	return RetryOptions{
		MaxAttempts:    maxAttempts,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
	}
}

func TestNewClientWithRetry_SucceedsImmediately(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	retries := 0
	client, err := NewClientWithRetry(context.Background(), mr.Addr(), "", false, fastRetryOpts(3),
		func(attempt int, err error, backoff time.Duration) { retries++ })
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	defer client.Close()

	if retries != 0 {
		t.Errorf("expected no retries on immediate success, got %d", retries)
	}
}

func TestNewClientWithRetry_RetriesThenFails(t *testing.T) {
	retries := 0
	// 127.0.0.1:1 refuses connections immediately, so each attempt fails fast.
	client, err := NewClientWithRetry(context.Background(), "127.0.0.1:1", "", false, fastRetryOpts(3),
		func(attempt int, err error, backoff time.Duration) { retries++ })
	if err == nil {
		client.Close()
		t.Fatal("expected error after exhausting attempts, got nil")
	}
	// With MaxAttempts=3, onRetry fires after attempts 1 and 2; attempt 3 returns the error.
	if retries != 2 {
		t.Errorf("expected 2 retry callbacks before giving up, got %d", retries)
	}
}

func TestNewClientWithRetry_AbortsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	retries := 0
	// MaxAttempts=0 means retry forever; only the cancelled context stops it.
	done := make(chan struct{})
	go func() {
		defer close(done)
		client, err := NewClientWithRetry(ctx, "127.0.0.1:1", "", false, fastRetryOpts(0),
			func(attempt int, err error, backoff time.Duration) { retries++ })
		if err == nil {
			client.Close()
			t.Error("expected error when context is cancelled, got nil")
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("NewClientWithRetry did not abort on context cancellation")
	}
}
