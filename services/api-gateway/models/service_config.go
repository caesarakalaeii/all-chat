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
	"fmt"
	"os"
)

// ServiceConfig holds the configuration for a backend service
type ServiceConfig struct {
	Name          string // Service name (e.g., "auth-service")
	BaseURL       string // Base URL (e.g., "http://auth-service:8081")
	HealthPath    string // Health check path (e.g., "/health/live")
	PathPrefix    string // Request path prefix to match (e.g., "/api/v1/auth")
	StripPrefix   bool   // Whether to strip the path prefix when forwarding
	RewritePrefix string // Optional prefix to prepend after stripping PathPrefix
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
		StripPrefix: true, // Strip /api/v1/auth, forward remaining path
	}

	// Overlay Manager
	overlayURL := getEnvOrDefault("OVERLAY_SERVICE_URL", "http://localhost:8082")
	registry.Services["overlay-manager"] = &ServiceConfig{
		Name:        "overlay-manager",
		BaseURL:     overlayURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/overlays",
		StripPrefix: true, // Strip /api/v1/overlays, forward remaining path
	}

	// YouTube resolver (overlay-manager)
	registry.Services["youtube-resolver"] = &ServiceConfig{
		Name:          "youtube-resolver",
		BaseURL:       overlayURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/youtube",
		StripPrefix:   true,
		RewritePrefix: "/youtube",
	}

	// Emote Service
	emoteURL := getEnvOrDefault("EMOTE_SERVICE_URL", "http://localhost:8083")
	registry.Services["emote-service"] = &ServiceConfig{
		Name:          "emote-service",
		BaseURL:       emoteURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/emotes",
		StripPrefix:   true,      // Strip /api/v1/emotes, forward remaining path
		RewritePrefix: "/emotes", // Emote service exposes routes under /emotes
	}

	// Admin routes - Auth Service (users) - most specific match first
	registry.Services["admin-users"] = &ServiceConfig{
		Name:          "admin-users",
		BaseURL:       authURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/users",
		StripPrefix:   true,           // Strip /api/v1/admin/users
		RewritePrefix: "/admin/users", // Rewrite to /admin/users/* for auth-service
	}

	// Admin routes - Overlay Manager (overlays)
	registry.Services["admin-overlays"] = &ServiceConfig{
		Name:          "admin-overlays",
		BaseURL:       overlayURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/overlays",
		StripPrefix:   true,              // Strip /api/v1/admin/overlays
		RewritePrefix: "/admin/overlays", // Rewrite to /admin/overlays/* for overlay-manager
	}

	// Admin routes - Overlay Manager (overlays for a specific user)
	// Uses a dedicated prefix instead of /api/v1/admin/users/:id/overlays because
	// the gateway matches by longest static prefix: with the variable :id ahead of
	// the /overlays suffix, that path would always match /api/v1/admin/users and be
	// proxied to auth-service (which has no such route -> 404).
	registry.Services["admin-user-overlays"] = &ServiceConfig{
		Name:          "admin-user-overlays",
		BaseURL:       overlayURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/user-overlays",
		StripPrefix:   true,                   // Strip /api/v1/admin/user-overlays
		RewritePrefix: "/admin/user-overlays", // Rewrite to /admin/user-overlays/:id for overlay-manager
	}

	// Admin routes - Overlay Manager (sources)
	registry.Services["admin-sources"] = &ServiceConfig{
		Name:          "admin-sources",
		BaseURL:       overlayURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/sources",
		StripPrefix:   true,             // Strip /api/v1/admin/sources
		RewritePrefix: "/admin/sources", // Rewrite to /admin/sources for overlay-manager
	}

	// Admin routes - Auth Service (stats dashboard)
	registry.Services["admin-stats"] = &ServiceConfig{
		Name:          "admin-stats",
		BaseURL:       authURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/stats",
		StripPrefix:   true,
		RewritePrefix: "/admin/stats",
	}

	// Admin routes - Auth Service (viewers)
	registry.Services["admin-viewers"] = &ServiceConfig{
		Name:          "admin-viewers",
		BaseURL:       authURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/viewers",
		StripPrefix:   true,
		RewritePrefix: "/admin/viewers",
	}

	// Admin routes - Auth Service (cosmetics)
	registry.Services["admin-cosmetics"] = &ServiceConfig{
		Name:          "admin-cosmetics",
		BaseURL:       authURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/cosmetics",
		StripPrefix:   true,
		RewritePrefix: "/admin/cosmetics",
	}

	// Share Service
	shareURL := getEnvOrDefault("SHARE_SERVICE_URL", "http://localhost:8090")
	registry.Services["share-service-shares"] = &ServiceConfig{
		Name:        "share-service-shares",
		BaseURL:     shareURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/shares",
		StripPrefix: false,
	}
	registry.Services["share-service-users"] = &ServiceConfig{
		Name:        "share-service-users",
		BaseURL:     shareURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/users",
		StripPrefix: false,
	}
	// Admin premium routes — separate prefix to avoid conflict with admin-users → auth-service
	registry.Services["share-service-admin-premium"] = &ServiceConfig{
		Name:        "share-service-admin-premium",
		BaseURL:     shareURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/admin/premium",
		StripPrefix: false,
	}
	// Admin feature gate routes → share-service
	registry.Services["share-service-admin-feature-gates"] = &ServiceConfig{
		Name:        "share-service-admin-feature-gates",
		BaseURL:     shareURL,
		HealthPath:  "/health/live",
		PathPrefix:  "/api/v1/admin/feature-gates",
		StripPrefix: false,
	}

	// Admin maintenance window routes → overlay-manager
	registry.Services["admin-maintenance"] = &ServiceConfig{
		Name:          "admin-maintenance",
		BaseURL:       overlayURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/admin/maintenance",
		StripPrefix:   true,
		RewritePrefix: "/admin/maintenance",
	}

	// User-facing upcoming maintenance route → overlay-manager
	registry.Services["maintenance-upcoming"] = &ServiceConfig{
		Name:          "maintenance-upcoming",
		BaseURL:       overlayURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/maintenance/upcoming",
		StripPrefix:   true,
		RewritePrefix: "/maintenance/upcoming",
	}

	// Public test-stream generator → message-processor.
	// /api/v1/test-stream/* is rewritten to the message-processor's
	// /public/test-stream/* endpoints, keeping message-processor internal while
	// exposing the trigger through the already-public gateway.
	messageProcessorURL := getEnvOrDefault("MESSAGE_PROCESSOR_URL", "http://localhost:8087")
	registry.Services["message-processor-test-stream"] = &ServiceConfig{
		Name:          "message-processor-test-stream",
		BaseURL:       messageProcessorURL,
		HealthPath:    "/health/live",
		PathPrefix:    "/api/v1/test-stream",
		StripPrefix:   true,
		RewritePrefix: "/public/test-stream",
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
