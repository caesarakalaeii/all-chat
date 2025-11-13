package quota

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

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
			tracker := NewTracker(nil, tt.dailyLimit, logger)
			assert.NotNil(t, tracker)
			assert.Equal(t, tt.expected, tracker.dailyLimit)
		})
	}
}

func TestGetUsageToday(t *testing.T) {
	logger := zap.NewNop()
	tracker := NewTracker(nil, 10000, logger)

	tracker.mu.Lock()
	tracker.usageToday = 5000
	tracker.mu.Unlock()

	usage := tracker.GetUsageToday()
	assert.Equal(t, 5000, usage)
}

func TestGetRemainingQuota(t *testing.T) {
	logger := zap.NewNop()
	tracker := NewTracker(nil, 10000, logger)

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
			tracker.mu.Unlock()

			remaining := tracker.GetRemainingQuota()
			assert.Equal(t, tt.expected, remaining)
		})
	}
}

func TestCanMakeRequest(t *testing.T) {
	logger := zap.NewNop()
	tracker := NewTracker(nil, 10000, logger)

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
			tracker.mu.Unlock()

			can := tracker.CanMakeRequest(tt.units)
			assert.Equal(t, tt.expected, can)
		})
	}
}

func TestGetUsagePercentage(t *testing.T) {
	logger := zap.NewNop()
	tracker := NewTracker(nil, 10000, logger)

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
			tracker.mu.Unlock()

			percentage := tracker.GetUsagePercentage()
			assert.Equal(t, tt.expected, percentage)
		})
	}
}

func TestQuotaCosts(t *testing.T) {
	// Verify quota cost constants
	assert.Equal(t, 5, QuotaCostLiveChatMessages)
	assert.Equal(t, 1, QuotaCostVideos)
	assert.Equal(t, 100, QuotaCostSearch)
}
