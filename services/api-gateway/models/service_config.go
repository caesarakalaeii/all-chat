package models

import (
	"fmt"
	"os"
)

// ServiceConfig holds the configuration for a backend service
type ServiceConfig struct {
	Name        string // Service name (e.g., "auth-service")
	BaseURL     string // Base URL (e.g., "http://auth-service:8081")
	HealthPath  string // Health check path (e.g., "/health/live")
	PathPrefix  string // Request path prefix to match (e.g., "/api/v1/auth")
	StripPrefix bool   // Whether to strip the path prefix when forwarding
}

// ServiceRegistry holds all configured backend services
type ServiceRegistry struct {
	Services map[string]*ServiceConfig
}

// NewServiceRegistry creates and initializes a service registry from environment variables
func NewServiceRegistry() (*ServiceRegistry, error) {
	registry := &ServiceRegistry{
		Services: make(map[string]*ServiceConfig),
	}

	// Auth Service
	authURL := getEnvOrDefault("AUTH_SERVICE_URL", "http://localhost:8081")
	registry.Services["auth-service"] = &ServiceConfig{
		Name:        "auth-service",
		BaseURL:     authURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/auth",
		StripPrefix: false,
	}

	// Overlay Manager
	overlayURL := getEnvOrDefault("OVERLAY_SERVICE_URL", "http://localhost:8082")
	registry.Services["overlay-manager"] = &ServiceConfig{
		Name:        "overlay-manager",
		BaseURL:     overlayURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/overlays",
		StripPrefix: false,
	}

	// Emote Service
	emoteURL := getEnvOrDefault("EMOTE_SERVICE_URL", "http://localhost:8083")
	registry.Services["emote-service"] = &ServiceConfig{
		Name:        "emote-service",
		BaseURL:     emoteURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/emotes",
		StripPrefix: false,
	}

	// Validate all service URLs are set
	for name, service := range registry.Services {
		if service.BaseURL == "" {
			return nil, fmt.Errorf("service URL not configured for %s", name)
		}
	}

	return registry, nil
}

// GetServiceForPath finds the service that should handle the given request path
func (sr *ServiceRegistry) GetServiceForPath(path string) *ServiceConfig {
	// Try to match each service's path prefix
	// We need to match the longest prefix first to handle overlapping paths
	var matched *ServiceConfig
	longestMatch := 0

	for _, service := range sr.Services {
		if len(service.PathPrefix) > longestMatch && matchesPrefix(path, service.PathPrefix) {
			matched = service
			longestMatch = len(service.PathPrefix)
		}
	}

	return matched
}

// matchesPrefix checks if the path starts with the given prefix
func matchesPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
