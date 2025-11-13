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
