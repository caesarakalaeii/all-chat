package models

import "time"

// MaintenanceWindow represents a scheduled maintenance/downtime window.
type MaintenanceWindow struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateMaintenanceRequest is the request body for creating a maintenance window.
type CreateMaintenanceRequest struct {
	Title       string    `json:"title"       binding:"required"`
	Description string    `json:"description"`
	StartsAt    time.Time `json:"starts_at"   binding:"required"`
	EndsAt      time.Time `json:"ends_at"     binding:"required"`
}
