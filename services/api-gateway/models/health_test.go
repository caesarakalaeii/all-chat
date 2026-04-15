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

package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthResponse_DetermineOverallStatus(t *testing.T) {
	tests := []struct {
		name           string
		services       map[string]ServiceStatus
		expectedStatus string
	}{
		{
			name: "all services up - healthy",
			services: map[string]ServiceStatus{
				"auth-service":    {Status: "up", LatencyMs: 5},
				"overlay-manager": {Status: "up", LatencyMs: 8},
				"emote-service":   {Status: "up", LatencyMs: 3},
			},
			expectedStatus: "healthy",
		},
		{
			name: "some services down - degraded",
			services: map[string]ServiceStatus{
				"auth-service":    {Status: "up", LatencyMs: 5},
				"overlay-manager": {Status: "down", LatencyMs: 0},
				"emote-service":   {Status: "up", LatencyMs: 3},
			},
			expectedStatus: "degraded",
		},
		{
			name: "all services down - unhealthy",
			services: map[string]ServiceStatus{
				"auth-service":    {Status: "down", LatencyMs: 0},
				"overlay-manager": {Status: "down", LatencyMs: 0},
				"emote-service":   {Status: "down", LatencyMs: 0},
			},
			expectedStatus: "unhealthy",
		},
		{
			name: "single service up - degraded",
			services: map[string]ServiceStatus{
				"auth-service":    {Status: "up", LatencyMs: 5},
				"overlay-manager": {Status: "down", LatencyMs: 0},
				"emote-service":   {Status: "down", LatencyMs: 0},
			},
			expectedStatus: "degraded",
		},
		{
			name:           "no services - unhealthy",
			services:       map[string]ServiceStatus{},
			expectedStatus: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := &HealthResponse{
				Services:  tt.services,
				Timestamp: time.Now(),
			}

			health.DetermineOverallStatus()

			assert.Equal(t, tt.expectedStatus, health.Status)
		})
	}
}
