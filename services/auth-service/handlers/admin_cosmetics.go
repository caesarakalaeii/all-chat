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
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// CosmeticCatalogEntry is the JSON shape for a frame or flair catalog item.
type CosmeticCatalogEntry struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	ImageURL  string    `json:"image_url"`
	IsPremium bool      `json:"is_premium"`
	CreatedAt time.Time `json:"created_at"`
}

// createCosmeticRequest is the POST body for adding a catalog entry.
type createCosmeticRequest struct {
	Name      string `json:"name"`
	ImageURL  string `json:"image_url"`
	IsPremium bool   `json:"is_premium"`
}

// cosmeticsCatalogDB is a minimal interface over pgxpool.Pool for unit testing.
type cosmeticsCatalogDB interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// pgxPoolAdapter wraps *pgxpool.Pool to satisfy cosmeticsCatalogDB.
// pgxpool.Pool already implements the interface but we need the conversion for type safety.
type pgxPoolAdapter struct {
	pool *pgxpool.Pool
}

func (a *pgxPoolAdapter) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return a.pool.Query(ctx, sql, args...)
}

func (a *pgxPoolAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return a.pool.QueryRow(ctx, sql, args...)
}

func (a *pgxPoolAdapter) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return a.pool.Exec(ctx, sql, args...)
}

// AdminCosmeticsHandler handles admin CRUD for cosmetic catalog tables.
type AdminCosmeticsHandler struct {
	log *zap.Logger
	db  cosmeticsCatalogDB
}

// allowedCosmeticTables is the allow-list of table names that can be used in
// SQL queries (audit L18). Any other table name is rejected to prevent SQL
// injection via string-concatenated table identifiers.
var allowedCosmeticTables = map[string]bool{
	"cosmetic_frames": true,
	"cosmetic_flairs": true,
}

// NewAdminCosmeticsHandler creates a new AdminCosmeticsHandler backed by a pgxpool.Pool.
func NewAdminCosmeticsHandler(log *zap.Logger, pool *pgxpool.Pool) *AdminCosmeticsHandler {
	return &AdminCosmeticsHandler{
		log: log.Named("admin-cosmetics"),
		db:  &pgxPoolAdapter{pool: pool},
	}
}

// newAdminCosmeticsHandlerWithDB creates a handler with a custom cosmeticsCatalogDB (for testing).
func newAdminCosmeticsHandlerWithDB(log *zap.Logger, db cosmeticsCatalogDB) *AdminCosmeticsHandler {
	var l *zap.Logger
	if log != nil {
		l = log.Named("admin-cosmetics")
	} else {
		l, _ = zap.NewDevelopment()
	}
	return &AdminCosmeticsHandler{log: l, db: db}
}

// listCatalogEntries queries a catalog table and returns all entries ordered by created_at ASC.
func (h *AdminCosmeticsHandler) listCatalogEntries(c *gin.Context, table string) {
	if !allowedCosmeticTables[table] {
		h.log.Warn("Rejected unknown cosmetic table name", zap.String("table", table))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table"})
		return
	}
	sql := `SELECT id, name, image_url, is_premium, created_at FROM ` + table + ` ORDER BY created_at ASC`
	rows, err := h.db.Query(c.Request.Context(), sql)
	if err != nil {
		h.log.Error("Failed to list catalog entries", zap.String("table", table), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list entries"})
		return
	}
	defer rows.Close()

	entries := make([]CosmeticCatalogEntry, 0)
	for rows.Next() {
		var e CosmeticCatalogEntry
		if err := rows.Scan(&e.ID, &e.Name, &e.ImageURL, &e.IsPremium, &e.CreatedAt); err != nil {
			h.log.Error("Failed to scan catalog entry", zap.String("table", table), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read entries"})
			return
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		h.log.Error("Row iteration error", zap.String("table", table), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read entries"})
		return
	}

	c.JSON(http.StatusOK, entries)
}

// createCatalogEntry inserts a new catalog entry into the given table.
func (h *AdminCosmeticsHandler) createCatalogEntry(c *gin.Context, table string) {
	if !allowedCosmeticTables[table] {
		h.log.Warn("Rejected unknown cosmetic table name", zap.String("table", table))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table"})
		return
	}
	var req createCosmeticRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.ImageURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_url is required"})
		return
	}

	sql := `INSERT INTO ` + table + ` (name, image_url, is_premium) VALUES ($1, $2, $3) RETURNING id, name, image_url, is_premium, created_at`
	row := h.db.QueryRow(c.Request.Context(), sql, req.Name, req.ImageURL, req.IsPremium)

	var entry CosmeticCatalogEntry
	if err := row.Scan(&entry.ID, &entry.Name, &entry.ImageURL, &entry.IsPremium, &entry.CreatedAt); err != nil {
		h.log.Error("Failed to create catalog entry", zap.String("table", table), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create entry"})
		return
	}

	c.JSON(http.StatusCreated, entry)
}

// deleteCatalogEntry removes a catalog entry by UUID from the given table.
func (h *AdminCosmeticsHandler) deleteCatalogEntry(c *gin.Context, table string) {
	if !allowedCosmeticTables[table] {
		h.log.Warn("Rejected unknown cosmetic table name", zap.String("table", table))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid table"})
		return
	}
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	sql := `DELETE FROM ` + table + ` WHERE id = $1`
	tag, err := h.db.Exec(c.Request.Context(), sql, id)
	if err != nil {
		h.log.Error("Failed to delete catalog entry", zap.String("table", table), zap.String("id", idStr), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete entry"})
		return
	}

	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleListFrames returns all frame catalog entries ordered by created_at ASC.
func (h *AdminCosmeticsHandler) HandleListFrames(c *gin.Context) {
	h.listCatalogEntries(c, "cosmetic_frames")
}

// HandleCreateFrame creates a new frame catalog entry.
func (h *AdminCosmeticsHandler) HandleCreateFrame(c *gin.Context) {
	h.createCatalogEntry(c, "cosmetic_frames")
}

// HandleDeleteFrame deletes a frame catalog entry by UUID.
func (h *AdminCosmeticsHandler) HandleDeleteFrame(c *gin.Context) {
	h.deleteCatalogEntry(c, "cosmetic_frames")
}

// HandleListFlairs returns all flair catalog entries ordered by created_at ASC.
func (h *AdminCosmeticsHandler) HandleListFlairs(c *gin.Context) {
	h.listCatalogEntries(c, "cosmetic_flairs")
}

// HandleCreateFlair creates a new flair catalog entry.
func (h *AdminCosmeticsHandler) HandleCreateFlair(c *gin.Context) {
	h.createCatalogEntry(c, "cosmetic_flairs")
}

// HandleDeleteFlair deletes a flair catalog entry by UUID.
func (h *AdminCosmeticsHandler) HandleDeleteFlair(c *gin.Context) {
	h.deleteCatalogEntry(c, "cosmetic_flairs")
}
