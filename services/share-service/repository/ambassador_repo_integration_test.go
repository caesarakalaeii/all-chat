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

	"github.com/caesar/all-chat/shared/premium"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// TestAmbassadorRepository_Flow exercises the real ambassador SQL end to end against
// the full migration set: grant curates the card and force-grants premium; the
// public list is gated on consent AND the role AND not-banned; a nil tagline/order
// on re-grant preserves the existing values and never touches consent; revoke hides
// the card and drops premium; re-grant restores the streamer's prior consent.
func TestAmbassadorRepository_Flow(t *testing.T) {
	pool, cleanup := setupAmbassadorTestDB(t)
	defer cleanup()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name, profile_image_url,
		                   access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ('700700', 'twitch', 'amb_flow', 'Ambassador Flow', 'https://cdn/avatar.png',
		        'a', 'r', NOW() + INTERVAL '4 hours', ARRAY[]::text[])
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	repo := NewAmbassadorRepository(pool, premium.NewRecomputer(pool, nil), zap.NewNop())

	tagline := "Multistreams to 3 platforms"
	sortOrder := 5

	// Grant with a curated card. Consent defaults false, so still not public.
	if err := repo.SetUserAmbassador(ctx, id, true, &tagline, &sortOrder); err != nil {
		t.Fatalf("grant: %v", err)
	}
	assertPremiumCol(t, pool, id, true) // ambassador is premium (recompute)

	sc, err := repo.GetShowcase(ctx, id)
	if err != nil {
		t.Fatalf("GetShowcase: %v", err)
	}
	if !sc.IsAmbassador || sc.Tagline == nil || *sc.Tagline != tagline || sc.SortOrder != 5 || sc.FeaturedConsent {
		t.Fatalf("showcase after grant = %+v, want ambassador+tagline+order5+consent=false", sc)
	}

	if list, _ := repo.ListPublic(ctx); len(list) != 0 {
		t.Fatalf("ListPublic before consent = %d, want 0 (opt-in)", len(list))
	}

	// Streamer opts in => now public.
	if err := repo.SetFeaturedConsent(ctx, id, true); err != nil {
		t.Fatalf("consent: %v", err)
	}
	list, err := repo.ListPublic(ctx)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListPublic after consent = %d, want 1", len(list))
	}
	got := list[0]
	if got.Username != "amb_flow" || got.DisplayName != "Ambassador Flow" ||
		got.AvatarURL != "https://cdn/avatar.png" || got.Platform != "twitch" ||
		got.Tagline == nil || *got.Tagline != tagline {
		t.Fatalf("public card = %+v, want the seeded profile + tagline", got)
	}

	// Re-grant with nil card fields must PRESERVE tagline/order and NOT touch consent.
	if err := repo.SetUserAmbassador(ctx, id, true, nil, nil); err != nil {
		t.Fatalf("re-grant: %v", err)
	}
	sc, _ = repo.GetShowcase(ctx, id)
	if sc.Tagline == nil || *sc.Tagline != tagline || sc.SortOrder != 5 || !sc.FeaturedConsent {
		t.Fatalf("re-grant clobbered card/consent: %+v", sc)
	}

	// A banned ambassador is excluded from the public list (defensive), via BOTH ban
	// paths: the account-level flag AND an active platform-ID ban that does not set it.
	if _, err := pool.Exec(ctx, `UPDATE users SET is_banned = TRUE WHERE id = $1`, id); err != nil {
		t.Fatalf("ban: %v", err)
	}
	if list, _ := repo.ListPublic(ctx); len(list) != 0 {
		t.Fatalf("ListPublic with account-banned ambassador = %d, want 0", len(list))
	}
	pool.Exec(ctx, `UPDATE users SET is_banned = FALSE WHERE id = $1`, id)

	// Platform-ID ban (banned_platform_ids) WITHOUT users.is_banned — the path the
	// review flagged. The public card must still be excluded.
	if _, err := pool.Exec(ctx, `
		INSERT INTO banned_platform_ids (platform, platform_id, banned_by, reason)
		VALUES ('twitch', '700700', $1, 'test platform ban')`, id); err != nil {
		t.Fatalf("platform ban: %v", err)
	}
	if list, _ := repo.ListPublic(ctx); len(list) != 0 {
		t.Fatalf("ListPublic with platform-banned ambassador = %d, want 0", len(list))
	}
	if _, err := pool.Exec(ctx, `UPDATE banned_platform_ids SET is_active = FALSE WHERE platform_id = '700700'`); err != nil {
		t.Fatalf("unban platform: %v", err)
	}

	// Revoke hides the card and drops premium; consent row is preserved.
	if err := repo.SetUserAmbassador(ctx, id, false, nil, nil); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	assertPremiumCol(t, pool, id, false)
	if list, _ := repo.ListPublic(ctx); len(list) != 0 {
		t.Fatalf("ListPublic after revoke = %d, want 0", len(list))
	}
	sc, _ = repo.GetShowcase(ctx, id)
	if sc.IsAmbassador {
		t.Fatalf("still ambassador after revoke")
	}
	if !sc.FeaturedConsent {
		t.Fatalf("revoke should preserve prior consent for a later re-grant")
	}

	// Re-grant restores the public card using the preserved consent.
	if err := repo.SetUserAmbassador(ctx, id, true, nil, nil); err != nil {
		t.Fatalf("second grant: %v", err)
	}
	if list, _ := repo.ListPublic(ctx); len(list) != 1 {
		t.Fatalf("ListPublic after re-grant = %d, want 1 (consent preserved)", len(list))
	}

	// GetShowcase on a non-ambassador returns is_ambassador=false, not an error.
	var otherID string
	pool.QueryRow(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name, access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ('700701','twitch','plain','Plain','a','r', NOW() + INTERVAL '4 hours', ARRAY[]::text[]) RETURNING id`).Scan(&otherID)
	if sc, err := repo.GetShowcase(ctx, otherID); err != nil || sc.IsAmbassador {
		t.Fatalf("GetShowcase(non-ambassador) = (%+v, %v), want is_ambassador=false, nil", sc, err)
	}

	// Unknown user id is a not-found error, not a panic.
	if _, err := repo.GetShowcase(ctx, "00000000-0000-0000-0000-000000000000"); err == nil {
		t.Fatalf("GetShowcase(unknown) should error")
	}
}

func assertPremiumCol(t *testing.T, pool *pgxpool.Pool, id string, want bool) {
	t.Helper()
	var got bool
	if err := pool.QueryRow(context.Background(), `SELECT is_premium FROM users WHERE id = $1`, id).Scan(&got); err != nil {
		t.Fatalf("read is_premium: %v", err)
	}
	if got != want {
		t.Errorf("is_premium = %v, want %v", got, want)
	}
}

// setupAmbassadorTestDB starts a fresh postgres and applies the real up-migrations,
// so the repository runs against the exact production schema (users + ambassador_showcase).
func setupAmbassadorTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
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
			WithOccurrence(2).WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Fatalf("start container: %v", err)
	}
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	if err != nil {
		t.Fatalf("pool: %v", err)
	}

	dir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".sql") || strings.HasSuffix(n, "_down.sql") {
			continue
		}
		if n[0] >= '0' && n[0] <= '9' {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		sqlBytes, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("migration %s: %v", n, err)
		}
	}

	return pool, func() {
		pool.Close()
		container.Terminate(ctx)
	}
}
