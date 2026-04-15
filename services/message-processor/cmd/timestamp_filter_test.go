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

package main

import (
	"testing"
	"time"
)

func TestMessageAgeCalculation(t *testing.T) {
	tests := []struct {
		name           string
		messageTime    time.Time
		cutoff         time.Duration
		shouldBeFiltered bool
	}{
		{
			name:           "Recent message (5 seconds old)",
			messageTime:    time.Now().Add(-5 * time.Second),
			cutoff:         60 * time.Second,
			shouldBeFiltered: false,
		},
		{
			name:           "Just under cutoff (59 seconds old)",
			messageTime:    time.Now().Add(-59 * time.Second),
			cutoff:         60 * time.Second,
			shouldBeFiltered: false,
		},
		{
			name:           "Old message (2 minutes old)",
			messageTime:    time.Now().Add(-2 * time.Minute),
			cutoff:         60 * time.Second,
			shouldBeFiltered: true,
		},
		{
			name:           "Very old message (1 hour old)",
			messageTime:    time.Now().Add(-1 * time.Hour),
			cutoff:         60 * time.Second,
			shouldBeFiltered: true,
		},
		{
			name:           "Custom cutoff (5 minutes)",
			messageTime:    time.Now().Add(-4 * time.Minute),
			cutoff:         5 * time.Minute,
			shouldBeFiltered: false,
		},
		{
			name:           "Custom cutoff exceeded",
			messageTime:    time.Now().Add(-6 * time.Minute),
			cutoff:         5 * time.Minute,
			shouldBeFiltered: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messageAge := time.Since(tt.messageTime)
			isFiltered := messageAge > tt.cutoff

			if isFiltered != tt.shouldBeFiltered {
				t.Errorf("Expected shouldBeFiltered=%v, got %v (messageAge=%v, cutoff=%v)",
					tt.shouldBeFiltered, isFiltered, messageAge, tt.cutoff)
			}
		})
	}
}

func TestEnvVarParsing(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "Default (60 seconds)",
			input:    "60",
			expected: 60,
		},
		{
			name:     "Custom (120 seconds)",
			input:    "120",
			expected: 120,
		},
		{
			name:     "Custom (300 seconds / 5 minutes)",
			input:    "300",
			expected: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse as we do in main.go
			if parsed, err := time.ParseDuration(tt.input + "s"); err == nil {
				got := int(parsed.Seconds())
				if got != tt.expected {
					t.Errorf("Expected %d seconds, got %d", tt.expected, got)
				}
			} else {
				t.Errorf("Failed to parse duration: %v", err)
			}
		})
	}
}
