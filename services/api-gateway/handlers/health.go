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
