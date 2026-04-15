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

package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	registry *models.ServiceRegistry
	client   *http.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(registry *models.ServiceRegistry) *HealthHandler {
	return &HealthHandler{
		registry: registry,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// CheckHealth checks the health of all backend services and returns aggregated status
func (h *HealthHandler) CheckHealth(c *gin.Context) {
	response := &models.HealthResponse{
		Services:  make(map[string]models.ServiceStatus),
		Timestamp: time.Now(),
	}

	// Check each service in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, service := range h.registry.Services {
		wg.Add(1)
		go func(svc *models.ServiceConfig) {
			defer wg.Done()

			status := h.checkService(svc)

			mu.Lock()
			response.Services[svc.Name] = status
			mu.Unlock()
		}(service)
	}

	wg.Wait()

	// Determine overall status based on individual service statuses
	response.DetermineOverallStatus()

	c.JSON(http.StatusOK, response)
}

// checkService checks the health of a single service
func (h *HealthHandler) checkService(service *models.ServiceConfig) models.ServiceStatus {
	start := time.Now()

	url := service.BaseURL + service.HealthPath
	resp, err := h.client.Get(url)

	latency := time.Since(start)

	if err != nil {
		return models.ServiceStatus{
			Status:    "down",
			LatencyMs: latency.Milliseconds(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return models.ServiceStatus{
			Status:    "up",
			LatencyMs: latency.Milliseconds(),
		}
	}

	return models.ServiceStatus{
		Status:    "down",
		LatencyMs: latency.Milliseconds(),
	}
}
