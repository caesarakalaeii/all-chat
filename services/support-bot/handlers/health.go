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

// Package handlers serves the support-bot liveness/readiness probes.
package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type statusResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// HealthHandler serves liveness/readiness probes. The bot exposes no other HTTP API.
type HealthHandler struct {
	db *pgxpool.Pool
}

// NewHealthHandler builds a health handler. db may be nil (readiness then only checks
// the process is up).
func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{db: db}
}

// Live always returns 200 — the process is up.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, statusResponse{Status: "alive"})
}

// Ready returns 200 only when the memory database is reachable.
func (h *HealthHandler) Ready(c *gin.Context) {
	if h.db == nil {
		c.JSON(http.StatusOK, statusResponse{Status: "ready"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, statusResponse{Status: "unavailable", Reason: "database"})
		return
	}
	c.JSON(http.StatusOK, statusResponse{Status: "ready"})
}
