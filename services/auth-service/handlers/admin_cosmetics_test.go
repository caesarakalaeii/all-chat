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

// Tests for admin cosmetics catalog endpoints (Plan 30-02, Task 1).
// Uses a mock cosmeticsCatalogDB interface to avoid real DB dependency.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockCosmeticsCatalogDB implements cosmeticsCatalogDB for unit tests.
type mockCosmeticsCatalogDB struct {
	// queryResult holds rows to return from Query calls.
	queryRows []CosmeticCatalogEntry
	queryErr  error

	// queryRowResult holds single-row data for QueryRow calls.
	queryRowEntry CosmeticCatalogEntry
	queryRowErr   error

	// execResult holds RowsAffected for Exec calls.
	execRowsAffected int64
	execErr          error

	// calls records method invocations for assertion.
	queryCalls    int
	queryRowCalls int
	execCalls     int
}

// mockRows wraps a slice of CosmeticCatalogEntry to satisfy pgx.Rows.
type mockRows struct {
	entries []CosmeticCatalogEntry
	index   int
	err     error
}

func (r *mockRows) Close()                                       {}
func (r *mockRows) Err() error                                   { return r.err }
func (r *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *mockRows) Next() bool {
	if r.err != nil {
		return false
	}
	return r.index < len(r.entries)
}
func (r *mockRows) Scan(dest ...interface{}) error {
	if r.index >= len(r.entries) {
		return pgx.ErrNoRows
	}
	e := r.entries[r.index]
	r.index++
	// dest order: id, name, image_url, is_premium, created_at
	if len(dest) >= 5 {
		*dest[0].(*uuid.UUID) = e.ID
		*dest[1].(*string) = e.Name
		*dest[2].(*string) = e.ImageURL
		*dest[3].(*bool) = e.IsPremium
		*dest[4].(*time.Time) = e.CreatedAt
	}
	return nil
}
func (r *mockRows) Values() ([]interface{}, error) { return nil, nil }
func (r *mockRows) RawValues() [][]byte             { return nil }
func (r *mockRows) Conn() *pgx.Conn                 { return nil }

// mockRow wraps a single CosmeticCatalogEntry to satisfy pgx.Row.
type mockRow struct {
	entry CosmeticCatalogEntry
	err   error
}

func (r *mockRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) >= 5 {
		*dest[0].(*uuid.UUID) = r.entry.ID
		*dest[1].(*string) = r.entry.Name
		*dest[2].(*string) = r.entry.ImageURL
		*dest[3].(*bool) = r.entry.IsPremium
		*dest[4].(*time.Time) = r.entry.CreatedAt
	}
	return nil
}

func (m *mockCosmeticsCatalogDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	m.queryCalls++
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return &mockRows{entries: m.queryRows}, nil
}

func (m *mockCosmeticsCatalogDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	m.queryRowCalls++
	return &mockRow{entry: m.queryRowEntry, err: m.queryRowErr}
}

func (m *mockCosmeticsCatalogDB) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	m.execCalls++
	if m.execErr != nil {
		return pgconn.CommandTag{}, m.execErr
	}
	return pgconn.NewCommandTag(fmt.Sprintf("DELETE %d", m.execRowsAffected)), nil
}

// setupAdminCosmeticsRouter creates a Gin router with AdminCosmeticsHandler and mock DB.
func setupAdminCosmeticsRouter(t *testing.T, mock *mockCosmeticsCatalogDB) (*gin.Engine, *AdminCosmeticsHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := newAdminCosmeticsHandlerWithDB(nil, mock)
	router := gin.New()
	admin := router.Group("/admin")
	admin.GET("/cosmetics/frames", handler.HandleListFrames)
	admin.POST("/cosmetics/frames", handler.HandleCreateFrame)
	admin.DELETE("/cosmetics/frames/:id", handler.HandleDeleteFrame)
	admin.GET("/cosmetics/flairs", handler.HandleListFlairs)
	admin.POST("/cosmetics/flairs", handler.HandleCreateFlair)
	admin.DELETE("/cosmetics/flairs/:id", handler.HandleDeleteFlair)
	return router, handler
}

// --- Frame tests ---

func TestAdminCosmeticsFrames_List_Empty(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{queryRows: []CosmeticCatalogEntry{}}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	req := httptest.NewRequest(http.MethodGet, "/admin/cosmetics/frames", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var result []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// Must be empty array, not null
	if result == nil {
		t.Error("expected empty array [], got null")
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestAdminCosmeticsFrames_List_WithEntries(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC()
	mock := &mockCosmeticsCatalogDB{
		queryRows: []CosmeticCatalogEntry{
			{ID: id, Name: "Gold Frame", ImageURL: "https://cdn.example.com/gold.png", IsPremium: true, CreatedAt: now},
		},
	}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	req := httptest.NewRequest(http.MethodGet, "/admin/cosmetics/frames", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var result []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0]["name"] != "Gold Frame" {
		t.Errorf("expected name=Gold Frame, got %v", result[0]["name"])
	}
}

func TestAdminCosmeticsFrames_Create_Valid(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC()
	mock := &mockCosmeticsCatalogDB{
		queryRowEntry: CosmeticCatalogEntry{ID: id, Name: "Silver Frame", ImageURL: "https://cdn.example.com/silver.png", IsPremium: false, CreatedAt: now},
	}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	body := `{"name":"Silver Frame","image_url":"https://cdn.example.com/silver.png","is_premium":false}`
	req := httptest.NewRequest(http.MethodPost, "/admin/cosmetics/frames", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["name"] != "Silver Frame" {
		t.Errorf("expected name=Silver Frame, got %v", result["name"])
	}
}

func TestAdminCosmeticsFrames_Create_MissingName(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	body := `{"image_url":"https://cdn.example.com/silver.png"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/cosmetics/frames", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminCosmeticsFrames_Create_MissingImageURL(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	body := `{"name":"Silver Frame"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/cosmetics/frames", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing image_url, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminCosmeticsFrames_Delete_Exists(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{execRowsAffected: 1}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/cosmetics/frames/"+id.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminCosmeticsFrames_Delete_NotFound(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{execRowsAffected: 0}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/cosmetics/frames/"+id.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}

// --- Flair tests (mirror of frame tests) ---

func TestAdminCosmeticsFlairs_List_Empty(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{queryRows: []CosmeticCatalogEntry{}}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	req := httptest.NewRequest(http.MethodGet, "/admin/cosmetics/flairs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var result []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result == nil {
		t.Error("expected empty array [], got null")
	}
}

func TestAdminCosmeticsFlairs_Create_Valid(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC()
	mock := &mockCosmeticsCatalogDB{
		queryRowEntry: CosmeticCatalogEntry{ID: id, Name: "Star Flair", ImageURL: "https://cdn.example.com/star.png", IsPremium: true, CreatedAt: now},
	}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	body := `{"name":"Star Flair","image_url":"https://cdn.example.com/star.png","is_premium":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/cosmetics/flairs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminCosmeticsFlairs_Create_MissingName(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	body := `{"image_url":"https://cdn.example.com/star.png"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/cosmetics/flairs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminCosmeticsFlairs_Delete_Exists(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{execRowsAffected: 1}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/cosmetics/flairs/"+id.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestAdminCosmeticsFlairs_Delete_NotFound(t *testing.T) {
	mock := &mockCosmeticsCatalogDB{execRowsAffected: 0}
	router, _ := setupAdminCosmeticsRouter(t, mock)

	id := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/admin/cosmetics/flairs/"+id.String(), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", w.Code, w.Body.String())
	}
}
