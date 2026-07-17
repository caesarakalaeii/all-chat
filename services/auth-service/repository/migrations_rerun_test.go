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

package repository

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestMigrationsSurviveRerun guards against semantically destructive re-runs.
//
// The production migration runner (scripts/run-migrations.sh) executes EVERY
// migration file on EVERY pod start — it does not track which migrations have
// already been applied. A migration can therefore be syntactically re-runnable
// (no errors under ON_ERROR_STOP=1) while still destroying data on each
// deploy. That happened with 009_invalidate_old_twitch_tokens.sql: its
// one-time "expire all valid Twitch tokens to force re-auth" UPDATE ran again
// on every deploy, knocking every EventSub-partitioned channel back to the
// IRC listener until token-refresh-service re-validated the tokens.
//
// This test applies the full migration set, creates live user state, applies
// the full set AGAIN (= a pod restart), and asserts the state survives.
func TestMigrationsSurviveRerun(t *testing.T) {
	pool, cleanup := setupMigrationTestDB(t)
	defer cleanup()
	ctx := context.Background()

	migrations := loadUpMigrations(t)
	runMigrations(t, pool, migrations)

	// Live state a deploy must not destroy: a Twitch streamer with a valid
	// token and the EventSub chat scopes granted.
	_, err := pool.Exec(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name,
		                   access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ('424242', 'twitch', 'rerun_canary', 'Rerun Canary',
		        'access-token', 'refresh-token', NOW() + INTERVAL '4 hours',
		        ARRAY['user:read:chat','user:bot','channel:bot'])
	`)
	if err != nil {
		t.Fatalf("failed to insert canary user: %v", err)
	}

	// Give the canary an overlay. The 077 onboarding backfill marks
	// overlay-owning users as onboarded, but ONLY in the run that first adds
	// the column — a re-run (= every pod restart) must not re-fire it and
	// silently "complete" onboarding for users who restarted it from Settings.
	_, err = pool.Exec(ctx, `
		INSERT INTO overlays (user_id, name)
		SELECT id, 'canary overlay' FROM users WHERE username = 'rerun_canary'
	`)
	if err != nil {
		t.Fatalf("failed to insert canary overlay: %v", err)
	}

	// Second pass = pod restart.
	runMigrations(t, pool, migrations)

	var tokenStillValid bool
	var scopes []string
	err = pool.QueryRow(ctx, `
		SELECT token_expires_at > NOW(), granted_scopes
		FROM users WHERE username = 'rerun_canary'
	`).Scan(&tokenStillValid, &scopes)
	if err != nil {
		t.Fatalf("canary user vanished after migration re-run: %v", err)
	}

	if !tokenStillValid {
		t.Errorf("re-running migrations expired the canary's valid Twitch token; a deploy would demote every EventSub channel to IRC")
	}
	hasChat := false
	for _, s := range scopes {
		if s == "user:read:chat" {
			hasChat = true
		}
	}
	if !hasChat {
		t.Errorf("re-running migrations destroyed granted_scopes: %v", scopes)
	}

	// 077 backfill must not re-fire: the canary owns an overlay but was
	// created AFTER the column-add run, so its onboarding state must still be
	// NULL after the re-run.
	var onboardingCompleted *time.Time
	err = pool.QueryRow(ctx, `
		SELECT onboarding_completed_at FROM users WHERE username = 'rerun_canary'
	`).Scan(&onboardingCompleted)
	if err != nil {
		t.Fatalf("failed to read canary onboarding state: %v", err)
	}
	if onboardingCompleted != nil {
		t.Errorf("re-running migrations re-fired the 077 onboarding backfill (onboarding_completed_at = %v); a deploy would silently complete onboarding for users who restarted it", onboardingCompleted)
	}
}

// loadUpMigrations reads migrations/[0-9]*.sql from the repo root in
// lexicographic order, skipping *_down.sql — the same selection as
// scripts/run-migrations.sh.
func loadUpMigrations(t *testing.T) []migrationFile {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read migrations dir %s: %v", dir, err)
	}

	var files []migrationFile
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, "_down.sql") {
			continue
		}
		if name[0] < '0' || name[0] > '9' {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", name, err)
		}
		files = append(files, migrationFile{name: name, sql: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	if len(files) == 0 {
		t.Fatal("no migration files found — wrong path?")
	}
	return files
}

type migrationFile struct {
	name string
	sql  string
}

func runMigrations(t *testing.T, pool *pgxpool.Pool, migrations []migrationFile) {
	t.Helper()
	ctx := context.Background()
	for _, m := range migrations {
		if _, err := pool.Exec(ctx, m.sql); err != nil {
			t.Fatalf("migration %s failed: %v", m.name, err)
		}
	}
}

// setupMigrationTestDB starts a PostgreSQL container WITHOUT any pre-created
// schema (unlike setupTestDB) so the real migration set owns the database,
// exactly like a fresh production cluster.
func setupMigrationTestDB(t *testing.T) (*pgxpool.Pool, func()) {
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
		t.Fatalf("Failed to start container: %v", err)
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

	return pool, func() {
		pool.Close()
		container.Terminate(ctx)
	}
}
