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

package quota

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

var (
	testMetrics     *metrics.ListenerMetrics
	testMetricsOnce sync.Once
)

// getTestMetrics returns a singleton metrics instance for tests to avoid duplicate registration
func getTestMetrics() *metrics.ListenerMetrics {
	testMetricsOnce.Do(func() {
		testMetrics = metrics.NewListenerMetrics("test", "test")
	})
	return testMetrics
}

// MockDB is a mock database for testing
type MockDB struct {
	mock.Mock
}

func (m *MockDB) Exec(ctx context.Context, sql string, args ...interface{}) (interface{}, error) {
	callArgs := m.Called(ctx, sql, args)
	return callArgs.Get(0), callArgs.Error(1)
}

func (m *MockDB) QueryRow(ctx context.Context, sql string, args ...interface{}) *MockRow {
	callArgs := m.Called(ctx, sql, args)
	return callArgs.Get(0).(*MockRow)
}

type MockRow struct {
	value int
	err   error
}

func (r *MockRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		*dest[0].(*int) = r.value
	}
	return nil
}

func TestNewTracker(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()

	tests := []struct {
		name       string
		dailyLimit int
		expected   int
	}{
		{"with valid limit", 10000, 10000},
		{"with zero limit", 0, DefaultDailyQuota},
		{"with negative limit", -100, DefaultDailyQuota},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewTracker(nil, tt.dailyLimit, logger, m)
			assert.NotNil(t, tracker)
			assert.Equal(t, tt.expected, tracker.dailyLimit)
		})
	}
}

func TestGetUsageToday(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()
	tracker := NewTracker(nil, 10000, logger, m)

	tracker.mu.Lock()
	tracker.usageToday = 5000
	tracker.currentDate = time.Now().In(YouTubePST).Format("2006-01-02")
	tracker.mu.Unlock()

	usage := tracker.GetUsageToday()
	assert.Equal(t, 5000, usage)
}

func TestGetRemainingQuota(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()
	tracker := NewTracker(nil, 10000, logger, m)

	tests := []struct {
		name     string
		used     int
		expected int
	}{
		{"half used", 5000, 5000},
		{"fully used", 10000, 0},
		{"over quota", 12000, 0},
		{"nothing used", 0, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker.mu.Lock()
			tracker.usageToday = tt.used
			tracker.currentDate = time.Now().In(YouTubePST).Format("2006-01-02")
			tracker.mu.Unlock()

			remaining := tracker.GetRemainingQuota()
			assert.Equal(t, tt.expected, remaining)
		})
	}
}

func TestCanMakeRequest(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()
	tracker := NewTracker(nil, 10000, logger, m)

	tests := []struct {
		name     string
		used     int
		units    int
		expected bool
	}{
		{"can make request", 5000, 100, true},
		{"exactly at limit", 9995, 5, true},
		{"would exceed limit", 9995, 10, false},
		{"already over limit", 10001, 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker.mu.Lock()
			tracker.usageToday = tt.used
			tracker.currentDate = time.Now().In(YouTubePST).Format("2006-01-02")
			tracker.mu.Unlock()

			can := tracker.CanMakeRequest(tt.units)
			assert.Equal(t, tt.expected, can)
		})
	}
}

func TestGetUsagePercentage(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()
	tracker := NewTracker(nil, 10000, logger, m)

	tests := []struct {
		name     string
		used     int
		expected float64
	}{
		{"zero percent", 0, 0.0},
		{"fifty percent", 5000, 50.0},
		{"eighty percent", 8000, 80.0},
		{"ninety percent", 9000, 90.0},
		{"hundred percent", 10000, 100.0},
		{"over hundred", 15000, 150.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker.mu.Lock()
			tracker.usageToday = tt.used
			tracker.currentDate = time.Now().In(YouTubePST).Format("2006-01-02")
			tracker.mu.Unlock()

			percentage := tracker.GetUsagePercentage()
			assert.Equal(t, tt.expected, percentage)
		})
	}
}

func TestDateRollover(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()
	tracker := NewTracker(nil, 10000, logger, m)

	// Set up tracker with yesterday's date and usage
	yesterday := time.Now().In(YouTubePST).AddDate(0, 0, -1).Format("2006-01-02")
	tracker.mu.Lock()
	tracker.currentDate = yesterday
	tracker.usageToday = 5000
	tracker.mu.Unlock()

	// Call GetUsageToday which should trigger date rollover
	usage := tracker.GetUsageToday()

	// Usage should be reset to 0 after date rollover
	assert.Equal(t, 0, usage)

	// Current date should be updated to today
	tracker.mu.RLock()
	today := time.Now().In(YouTubePST).Format("2006-01-02")
	assert.Equal(t, today, tracker.currentDate)
	tracker.mu.RUnlock()
}

func TestDateRolloverConcurrency(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()
	tracker := NewTracker(nil, 10000, logger, m)

	// Set up tracker with yesterday's date and usage
	yesterday := time.Now().In(YouTubePST).AddDate(0, 0, -1).Format("2006-01-02")
	tracker.mu.Lock()
	tracker.currentDate = yesterday
	tracker.usageToday = 5000
	tracker.mu.Unlock()

	// Simulate multiple goroutines calling getter methods concurrently
	// This tests that the double-check locking in checkDateRollover works correctly
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			tracker.GetUsageToday()
			tracker.GetRemainingQuota()
			tracker.GetUsagePercentage()
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// All methods should have triggered rollover, usage should be 0
	usage := tracker.GetUsageToday()
	assert.Equal(t, 0, usage)

	// Current date should be today
	tracker.mu.RLock()
	today := time.Now().In(YouTubePST).Format("2006-01-02")
	assert.Equal(t, today, tracker.currentDate)
	tracker.mu.RUnlock()
}

func TestStopTracker(t *testing.T) {
	logger := zap.NewNop()
	m := getTestMetrics()
	tracker := NewTracker(nil, 10000, logger, m)

	// Start the periodic check
	go tracker.periodicDateCheck()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop should complete without hanging
	done := make(chan bool)
	go func() {
		tracker.Stop()
		done <- true
	}()

	select {
	case <-done:
		// Success - Stop() completed
	case <-time.After(1 * time.Second):
		t.Fatal("Stop() did not complete within 1 second")
	}
}

func TestQuotaCosts(t *testing.T) {
	// Verify quota cost constants
	assert.Equal(t, 5, QuotaCostLiveChatMessages)
	assert.Equal(t, 1, QuotaCostVideos)
	assert.Equal(t, 100, QuotaCostSearch)
}
