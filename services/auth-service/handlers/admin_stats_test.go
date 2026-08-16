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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// setupStatsTestDB starts a PostgreSQL container with the minimal schema that
// GetDashboardStats reads: users (for totals + is_banned), overlays (for
// active-overlay and active-user counts) and overlay_chat_sources (for the
// per-platform breakdown).
func setupStatsTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("cannot start postgres testcontainer (docker unavailable?): %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	schema := `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			is_banned BOOLEAN NOT NULL DEFAULT FALSE
		);
		CREATE TABLE IF NOT EXISTS overlays (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT NOW(),
			last_connected_at TIMESTAMP NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS overlay_chat_sources (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
			platform VARCHAR(50) NOT NULL
		);
	`
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return pool, func() {
		pool.Close()
		container.Terminate(ctx)
	}
}

// insertUser creates a user and returns its id.
func insertUser(t *testing.T, pool *pgxpool.Pool, banned bool) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (is_banned) VALUES ($1) RETURNING id`, banned).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

// insertOverlay creates an overlay for userID whose last_connected_at is `ago`
// before now. created_at is set well before that, since activity requires a
// last_connected_at that was actually bumped after the row was created.
func insertOverlay(t *testing.T, pool *pgxpool.Pool, userID string, ago time.Duration) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO overlays (user_id, created_at, last_connected_at)
		 VALUES ($1, NOW() - INTERVAL '90 days', NOW() - $2::interval)`,
		userID, ago.String())
	if err != nil {
		t.Fatalf("insert overlay: %v", err)
	}
}

// insertNeverOpenedOverlay creates an overlay the way overlay-manager does, with
// no timestamps given, so created_at and last_connected_at both land on the
// statement's NOW() and the overlay reads as "created but never opened".
func insertNeverOpenedOverlay(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO overlays (user_id) VALUES ($1)`, userID); err != nil {
		t.Fatalf("insert never-opened overlay: %v", err)
	}
}

// TestGetDashboardStats_ActiveUsers verifies the rolling-window active-user
// counts: an active user is a distinct non-banned owner of an overlay that was
// genuinely connected inside the 24h / 7d / 30d window. Creating an overlay is
// not usage, so an overlay whose last_connected_at was never bumped past its
// created_at does not count.
func TestGetDashboardStats_ActiveUsers(t *testing.T) {
	pool, cleanup := setupStatsTestDB(t)
	defer cleanup()

	// u1: connected 1h ago         -> 24h, 7d, 30d
	u1 := insertUser(t, pool, false)
	insertOverlay(t, pool, u1, time.Hour)

	// u2: connected 3 days ago     -> 7d, 30d
	u2 := insertUser(t, pool, false)
	insertOverlay(t, pool, u2, 3*24*time.Hour)

	// u3: connected 20 days ago    -> 30d only
	u3 := insertUser(t, pool, false)
	insertOverlay(t, pool, u3, 20*24*time.Hour)

	// u4: connected 60 days ago    -> none
	u4 := insertUser(t, pool, false)
	insertOverlay(t, pool, u4, 60*24*time.Hour)

	// u5: banned, connected 1h ago -> excluded from every window
	u5 := insertUser(t, pool, true)
	insertOverlay(t, pool, u5, time.Hour)

	// u6: two overlays both recent -> counted once (DISTINCT user)
	u6 := insertUser(t, pool, false)
	insertOverlay(t, pool, u6, 2*time.Hour)
	insertOverlay(t, pool, u6, 5*time.Hour)

	// u7: signed up and created an overlay but never opened it -> not usage
	u7 := insertUser(t, pool, false)
	insertNeverOpenedOverlay(t, pool, u7)

	handler := &AdminHandler{db: pool, logger: zap.NewNop()}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/stats", nil)

	handler.GetDashboardStats(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stats struct {
		TotalUsers     int `json:"total_users"`
		BannedUsers    int `json:"banned_users"`
		ActiveUsers24h int `json:"active_users_24h"`
		ActiveUsers7d  int `json:"active_users_7d"`
		ActiveUsers30d int `json:"active_users_30d"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode response: %v (%s)", err, w.Body.String())
	}

	if stats.TotalUsers != 7 {
		t.Errorf("total_users = %d, want 7", stats.TotalUsers)
	}
	if stats.BannedUsers != 1 {
		t.Errorf("banned_users = %d, want 1", stats.BannedUsers)
	}
	if stats.ActiveUsers24h != 2 { // u1, u6
		t.Errorf("active_users_24h = %d, want 2", stats.ActiveUsers24h)
	}
	if stats.ActiveUsers7d != 3 { // u1, u2, u6
		t.Errorf("active_users_7d = %d, want 3", stats.ActiveUsers7d)
	}
	if stats.ActiveUsers30d != 4 { // u1, u2, u3, u6
		t.Errorf("active_users_30d = %d, want 4", stats.ActiveUsers30d)
	}
}
