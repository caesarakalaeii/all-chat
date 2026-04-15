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
	"context"
	"errors"
	"net/http"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/services/overlay-manager/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MaintenanceRepository defines the interface for maintenance window persistence.
type MaintenanceRepository interface {
	Create(ctx context.Context, req models.CreateMaintenanceRequest, createdBy string) (*models.MaintenanceWindow, error)
	ListAll(ctx context.Context) ([]models.MaintenanceWindow, error)
	ListUpcoming(ctx context.Context) ([]models.MaintenanceWindow, error)
	Delete(ctx context.Context, id string) error
}

// MaintenanceHandler handles maintenance window CRUD endpoints.
type MaintenanceHandler struct {
	maintenanceRepo MaintenanceRepository
	logger          *zap.Logger
}

// NewMaintenanceHandler creates a new MaintenanceHandler.
func NewMaintenanceHandler(repo MaintenanceRepository, logger *zap.Logger) *MaintenanceHandler {
	return &MaintenanceHandler{
		maintenanceRepo: repo,
		logger:          logger,
	}
}

// HandleCreateMaintenance creates a new maintenance window.
// POST /admin/maintenance
func (h *MaintenanceHandler) HandleCreateMaintenance(c *gin.Context) {
	var req models.CreateMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	if !req.StartsAt.Before(req.EndsAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "starts_at must be before ends_at"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	createdBy, ok := userID.(string)
	if !ok || createdBy == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	mw, err := h.maintenanceRepo.Create(c.Request.Context(), req, createdBy)
	if err != nil {
		h.logger.Error("Failed to create maintenance window", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create maintenance window"})
		return
	}

	h.logger.Info("Created maintenance window",
		zap.String("id", mw.ID),
		zap.String("title", mw.Title),
		zap.String("created_by", createdBy),
	)
	c.JSON(http.StatusCreated, mw)
}

// HandleListMaintenance returns all maintenance windows (admin).
// GET /admin/maintenance
func (h *MaintenanceHandler) HandleListMaintenance(c *gin.Context) {
	windows, err := h.maintenanceRepo.ListAll(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list maintenance windows", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list maintenance windows"})
		return
	}

	c.JSON(http.StatusOK, windows)
}

// HandleDeleteMaintenance deletes a maintenance window by ID.
// DELETE /admin/maintenance/:id
func (h *MaintenanceHandler) HandleDeleteMaintenance(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing maintenance window ID"})
		return
	}

	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid maintenance window ID: must be a valid UUID"})
		return
	}

	if err := h.maintenanceRepo.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrMaintenanceNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Maintenance window not found"})
			return
		}
		h.logger.Error("Failed to delete maintenance window", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete maintenance window"})
		return
	}

	h.logger.Info("Deleted maintenance window", zap.String("id", id))
	c.Status(http.StatusNoContent)
}

// HandleListUpcoming returns upcoming/active maintenance windows (non-admin users).
// GET /maintenance/upcoming
func (h *MaintenanceHandler) HandleListUpcoming(c *gin.Context) {
	windows, err := h.maintenanceRepo.ListUpcoming(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list upcoming maintenance windows", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list maintenance windows"})
		return
	}

	c.JSON(http.StatusOK, windows)
}
