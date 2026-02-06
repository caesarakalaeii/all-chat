package creditroll

import (
	"testing"
	"time"
)

func TestParseSessionTime(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		fieldName string
		wantError bool
	}{
		{
			name:      "valid RFC3339 time",
			timeStr:   "2026-02-07T10:00:00Z",
			fieldName: "started_at",
			wantError: false,
		},
		{
			name:      "empty string",
			timeStr:   "",
			fieldName: "started_at",
			wantError: true,
		},
		{
			name:      "invalid format",
			timeStr:   "not-a-time",
			fieldName: "started_at",
			wantError: true,
		},
		{
			name:      "zero time",
			timeStr:   "0001-01-01T00:00:00Z",
			fieldName: "started_at",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSessionTime(tt.timeStr, tt.fieldName)
			if tt.wantError {
				if err == nil {
					t.Errorf("parseSessionTime() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("parseSessionTime() unexpected error: %v", err)
				}
				if result.IsZero() {
					t.Errorf("parseSessionTime() returned zero time for valid input")
				}
			}
		})
	}
}

func TestValidateStartedAt(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		time      time.Time
		wantError bool
	}{
		{
			name:      "valid current time",
			time:      now,
			wantError: false,
		},
		{
			name:      "valid time 1 hour ago",
			time:      now.Add(-1 * time.Hour),
			wantError: false,
		},
		{
			name:      "zero time",
			time:      time.Time{},
			wantError: true,
		},
		{
			name:      "time before 2020",
			time:      time.Date(2019, 12, 31, 23, 59, 59, 0, time.UTC),
			wantError: true,
		},
		{
			name:      "time in future (2 hours)",
			time:      now.Add(2 * time.Hour),
			wantError: true,
		},
		{
			name:      "time at year 0001 (common parse error)",
			time:      time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStartedAt(tt.time)
			if tt.wantError {
				if err == nil {
					t.Errorf("validateStartedAt() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("validateStartedAt() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCalculateSessionDuration(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		startedAt time.Time
		wantMin   int
		wantMax   int
	}{
		{
			name:      "zero time returns 0",
			startedAt: time.Time{},
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "1 hour ago",
			startedAt: now.Add(-1 * time.Hour),
			wantMin:   3500,  // Allow some variance
			wantMax:   3700,
		},
		{
			name:      "5 minutes ago",
			startedAt: now.Add(-5 * time.Minute),
			wantMin:   290,
			wantMax:   310,
		},
		{
			name:      "future time returns 0",
			startedAt: now.Add(1 * time.Hour),
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "40 days ago (capped at 30 days)",
			startedAt: now.Add(-40 * 24 * time.Hour),
			wantMin:   30 * 24 * 60 * 60,      // Exactly 30 days
			wantMax:   30 * 24 * 60 * 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSessionDuration(tt.startedAt)
			if result < tt.wantMin || result > tt.wantMax {
				t.Errorf("calculateSessionDuration() = %d, want between %d and %d", result, tt.wantMin, tt.wantMax)
			}
		})
	}
}
