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

import "time"

// ServiceStatus represents the health status of a single service
type ServiceStatus struct {
	Status    string `json:"status"`     // "up", "down"
	LatencyMs int64  `json:"latency_ms"` // Response time in milliseconds
}

// HealthResponse represents the aggregated health status of all services
type HealthResponse struct {
	Status    string                   `json:"status"`    // "healthy", "degraded", "unhealthy"
	Services  map[string]ServiceStatus `json:"services"`  // Map of service name to status
	Timestamp time.Time                `json:"timestamp"` // Time of health check
}

// DetermineOverallStatus determines the overall health status based on individual service statuses
func (h *HealthResponse) DetermineOverallStatus() {
	upCount := 0
	totalCount := len(h.Services)

	// Handle edge case: no services configured
	if totalCount == 0 {
		h.Status = "unhealthy"
		return
	}

	for _, service := range h.Services {
		if service.Status == "up" {
			upCount++
		}
	}

	switch {
	case upCount == totalCount:
		h.Status = "healthy"
	case upCount > 0:
		h.Status = "degraded"
	default:
		h.Status = "unhealthy"
	}
}
