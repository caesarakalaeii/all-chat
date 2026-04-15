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

package metrics

import (
	"testing"
)

// Note: NewInnerTubeMetrics() uses promauto which registers metrics globally.
// We can only call it once per test run. These tests verify the structure
// and constants without creating multiple metric instances.

func TestInnerTubeMetrics_Registration(t *testing.T) {
	// This test verifies that NewInnerTubeMetrics() creates a valid metrics instance
	// We call it once and verify all fields are initialized
	m := NewInnerTubeMetrics()

	// Verify metrics are registered
	if m.Errors == nil {
		t.Fatal("Errors metric not initialized")
	}
	if m.Requests == nil {
		t.Fatal("Requests metric not initialized")
	}
	if m.MessagesPublished == nil {
		t.Fatal("MessagesPublished metric not initialized")
	}
	if m.RedisPublishAttempts == nil {
		t.Fatal("RedisPublishAttempts metric not initialized")
	}
	if m.RedisPublishSuccess == nil {
		t.Fatal("RedisPublishSuccess metric not initialized")
	}
	if m.RedisPublishLatency == nil {
		t.Fatal("RedisPublishLatency metric not initialized")
	}
	if m.Reconnections == nil {
		t.Fatal("Reconnections metric not initialized")
	}

	// Test that metric operations don't panic
	// Track different error types
	m.Errors.WithLabelValues(ServiceLabel, ErrorTypeNetwork).Inc()
	m.Errors.WithLabelValues(ServiceLabel, ErrorTypeHTTP).Inc()
	m.Errors.WithLabelValues(ServiceLabel, ErrorTypeParse).Inc()
	m.Errors.WithLabelValues(ServiceLabel, ErrorTypeRateLimit).Inc()
	m.Errors.WithLabelValues(ServiceLabel, ErrorTypeRedis).Inc()

	// Track messages for multiple channels
	m.MessagesPublished.WithLabelValues(ServiceLabel, "channel1").Inc()
	m.MessagesPublished.WithLabelValues(ServiceLabel, "channel2").Inc()
	m.MessagesPublished.WithLabelValues(ServiceLabel, "channel1").Inc()

	// Track Redis publish metrics
	m.RedisPublishAttempts.WithLabelValues(ServiceLabel).Inc()
	m.RedisPublishSuccess.WithLabelValues(ServiceLabel).Inc()
	m.RedisPublishLatency.WithLabelValues(ServiceLabel).Observe(0.005) // 5ms
	m.RedisPublishLatency.WithLabelValues(ServiceLabel).Observe(0.1)   // 100ms

	// Track reconnections with different reasons
	m.Reconnections.WithLabelValues(ServiceLabel, "channel1", ReconnectionReasonError).Inc()
	m.Reconnections.WithLabelValues(ServiceLabel, "channel1", ReconnectionReasonOffline).Inc()
	m.Reconnections.WithLabelValues(ServiceLabel, "channel2", ReconnectionReasonBackoff).Inc()
	m.Reconnections.WithLabelValues(ServiceLabel, "channel2", ReconnectionReasonRediscovery).Inc()

	// If we reach here, all metric operations succeeded
}

func TestErrorTypeConstants(t *testing.T) {
	// Verify error type constants are unique
	errorTypes := map[string]bool{
		ErrorTypeHTTP:      true,
		ErrorTypeParse:     true,
		ErrorTypeRateLimit: true,
		ErrorTypeRedis:     true,
		ErrorTypeNetwork:   true,
	}

	if len(errorTypes) != 5 {
		t.Errorf("Expected 5 unique error types, got %d", len(errorTypes))
	}

	// Verify no empty strings
	for errorType := range errorTypes {
		if errorType == "" {
			t.Error("Error type constant is empty string")
		}
	}
}

func TestReconnectionReasonConstants(t *testing.T) {
	// Verify reconnection reason constants are unique
	reasons := map[string]bool{
		ReconnectionReasonError:       true,
		ReconnectionReasonOffline:     true,
		ReconnectionReasonBackoff:     true,
		ReconnectionReasonRediscovery: true,
	}

	if len(reasons) != 4 {
		t.Errorf("Expected 4 unique reconnection reasons, got %d", len(reasons))
	}

	// Verify no empty strings
	for reason := range reasons {
		if reason == "" {
			t.Error("Reconnection reason constant is empty string")
		}
	}
}

func TestServiceLabel(t *testing.T) {
	// Verify service label matches expected canary service name
	expected := "youtube-listener-innertube-canary"
	if ServiceLabel != expected {
		t.Errorf("ServiceLabel = %q, want %q", ServiceLabel, expected)
	}
}
