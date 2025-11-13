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
