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

package sessions

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
