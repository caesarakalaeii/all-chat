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

package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// One container per test binary: the full migration set runs once into a template database
// and every test clones it, so each test starts from a fresh, fully migrated schema.
var (
	pgOnce      sync.Once
	pgContainer testcontainers.Container
	pgBaseURL   string
	pgErr       error
	pgDBSeq     atomic.Int64

	quotaMetricsOnce sync.Once
	quotaMetrics     *metrics.ListenerMetrics
)

func TestMain(m *testing.M) {
	code := m.Run()
	if pgContainer != nil {
		_ = pgContainer.Terminate(context.Background())
	}
	os.Exit(code)
}

func listenerMetrics() *metrics.ListenerMetrics {
	quotaMetricsOnce.Do(func() {
		quotaMetrics = metrics.NewListenerMetrics("test", "test")
	})
	return quotaMetrics
}

type migrationFile struct {
	name string
	sql  string
}

// loadUpMigrations reads migrations/[0-9]*.sql from the repo root in lexicographic order,
// skipping *_down.sql — the same selection as scripts/run-migrations.sh.
func loadUpMigrations() ([]migrationFile, error) {
	dir := filepath.Join("..", "..", "..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
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
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}
		files = append(files, migrationFile{name: name, sql: string(content)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	if len(files) == 0 {
		return nil, fmt.Errorf("no migration files found in %s", dir)
	}
	return files, nil
}

func startMigratedTemplate() error {
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
		return fmt.Errorf("cannot start postgres testcontainer (docker unavailable?): %w", err)
	}
	pgContainer = container

	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return fmt.Errorf("container port: %w", err)
	}
	pgBaseURL = "postgres://testuser:testpass@" + host + ":" + port.Port() + "/"

	migrations, err := loadUpMigrations()
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, pgBaseURL+"testdb?sslmode=disable")
	if err != nil {
		return fmt.Errorf("connect template: %w", err)
	}
	// CREATE DATABASE ... TEMPLATE refuses while any session is connected to the template.
	defer pool.Close()
	for _, m := range migrations {
		if _, err := pool.Exec(ctx, m.sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.name, err)
		}
	}
	return nil
}

// newMigratedDB returns a pool on a fresh clone of the fully migrated schema.
func newMigratedDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pgOnce.Do(func() { pgErr = startMigratedTemplate() })
	if pgErr != nil {
		t.Skipf("%v", pgErr)
	}
	ctx := context.Background()

	name := fmt.Sprintf("eligibility_%d", pgDBSeq.Add(1))
	admin, err := pgxpool.New(ctx, pgBaseURL+"postgres?sslmode=disable")
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name+" TEMPLATE testdb"); err != nil {
		t.Fatalf("clone template: %v", err)
	}

	pool, err := pgxpool.New(ctx, pgBaseURL+name+"?sslmode=disable")
	if err != nil {
		t.Fatalf("connect %s: %v", name, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type fixtures struct {
	t    *testing.T
	pool *pgxpool.Pool
	seq  int
}

func newFixtures(t *testing.T) *fixtures {
	return &fixtures{t: t, pool: newMigratedDB(t)}
}

func (f *fixtures) user(premium, banned bool) string {
	f.t.Helper()
	f.seq++
	var id string
	err := f.pool.QueryRow(context.Background(), `
		INSERT INTO users (twitch_id, auth_provider, username, display_name,
		                   access_token, refresh_token, token_expires_at, is_premium, is_banned)
		VALUES ($1, 'twitch', $2, $2, 'a', 'r', NOW() + INTERVAL '1 hour', $3, $4)
		RETURNING id`,
		fmt.Sprintf("tw-%d", f.seq), fmt.Sprintf("user%d", f.seq), premium, banned,
	).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert user: %v", err)
	}
	return id
}

func (f *fixtures) overlay(userID string, active bool) string {
	f.t.Helper()
	var id string
	err := f.pool.QueryRow(context.Background(),
		`INSERT INTO overlays (user_id, name, is_active) VALUES ($1, 'o', $2) RETURNING id`,
		userID, active,
	).Scan(&id)
	if err != nil {
		f.t.Fatalf("insert overlay: %v", err)
	}
	return id
}

func (f *fixtures) source(overlayID, channelID, config string, active bool) {
	f.t.Helper()
	_, err := f.pool.Exec(context.Background(), `
		INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name, config, is_active)
		VALUES ($1, 'youtube', $2, $2, $3::jsonb, $4)`,
		overlayID, channelID, config, active,
	)
	if err != nil {
		f.t.Fatalf("insert source: %v", err)
	}
}

func (f *fixtures) token(userID, channelID string) {
	f.t.Helper()
	_, err := f.pool.Exec(context.Background(), `
		INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry)
		VALUES ($1, $2, 'enc-a', 'enc-r', NOW() + INTERVAL '1 hour')`,
		userID, channelID,
	)
	if err != nil {
		f.t.Fatalf("insert token: %v", err)
	}
}

func (f *fixtures) eligible(gateFree bool) []*models.StreamSource {
	f.t.Helper()
	got, err := NewRepository(f.pool, zap.NewNop()).GetEligibleSources(context.Background(), gateFree)
	if err != nil {
		f.t.Fatalf("GetEligibleSources: %v", err)
	}
	return got
}

const optedIn = `{"official_api": true}`

func TestEligibleSources_OptedInPremiumIncluded(t *testing.T) {
	f := newFixtures(t)
	u := f.user(true, false)
	o := f.overlay(u, true)
	// ocs.is_active is written by the listeners themselves, so it must not gate eligibility:
	// a source innertube marked inactive is exactly the one official mode should pick up.
	f.source(o, "UCowned", optedIn, false)
	f.token(u, "UCowned")

	got := f.eligible(false)
	if len(got) != 1 {
		t.Fatalf("got %d eligible rows, want 1", len(got))
	}
	if got[0].OverlayID != o || got[0].ChannelID != "UCowned" || got[0].OwnerUserID != u {
		t.Fatalf("row = %+v, want overlay %s channel UCowned owner %s", *got[0], o, u)
	}
}

func TestEligibleSources_LapsedExcluded(t *testing.T) {
	f := newFixtures(t)
	u := f.user(false, false)
	f.source(f.overlay(u, true), "UClapsed", optedIn, true)
	f.token(u, "UClapsed")

	if got := f.eligible(false); len(got) != 0 {
		t.Fatalf("non-premium owner with the gate closed must be excluded, got %+v", *got[0])
	}
}

func TestEligibleSources_GateFreeNonPremiumIncluded(t *testing.T) {
	f := newFixtures(t)
	u := f.user(false, false)
	f.source(f.overlay(u, true), "UCfree", optedIn, true)
	f.token(u, "UCfree")

	got := f.eligible(true)
	if len(got) != 1 || got[0].OwnerUserID != u {
		t.Fatalf("gate open: want the non-premium owner's row, got %d rows", len(got))
	}
}

func TestEligibleSources_NoTokenExcluded(t *testing.T) {
	f := newFixtures(t)
	u := f.user(true, false)
	f.source(f.overlay(u, true), "UCnotoken", optedIn, true)
	f.token(u, "UCsomethingElse")

	if got := f.eligible(true); len(got) != 0 {
		t.Fatalf("owner without a token row for the channel must be excluded, got %+v", *got[0])
	}
}

func TestEligibleSources_BannedExcluded(t *testing.T) {
	f := newFixtures(t)
	u := f.user(true, true)
	f.source(f.overlay(u, true), "UCbanned", optedIn, true)
	f.token(u, "UCbanned")

	if got := f.eligible(true); len(got) != 0 {
		t.Fatalf("banned owner must be excluded, got %+v", *got[0])
	}
}

func TestEligibleSources_InactiveOverlayExcluded(t *testing.T) {
	f := newFixtures(t)
	u := f.user(true, false)
	f.source(f.overlay(u, false), "UCinactive", optedIn, true)
	f.token(u, "UCinactive")

	if got := f.eligible(true); len(got) != 0 {
		t.Fatalf("inactive overlay must be excluded, got %+v", *got[0])
	}
}

func TestEligibleSources_NotOptedInExcluded(t *testing.T) {
	f := newFixtures(t)
	u := f.user(true, false)
	o := f.overlay(u, true)
	f.source(o, "UCabsent", `{}`, true)
	f.source(o, "UCfalse", `{"official_api": false}`, true)
	f.source(o, "UCstring", `{"official_api": "yes"}`, true)
	f.token(u, "UCabsent")
	f.token(u, "UCfalse")
	f.token(u, "UCstring")

	if got := f.eligible(true); len(got) != 0 {
		t.Fatalf("sources without official_api=true must be excluded, got %+v", *got[0])
	}
}

func TestEligibleSources_OwnerIsOptingInUser(t *testing.T) {
	f := newFixtures(t)
	owner := f.user(false, false)
	other := f.user(true, false)
	ownerOverlay := f.overlay(owner, true)
	f.source(ownerOverlay, "UCshared", optedIn, true)
	f.token(owner, "UCshared")
	// A premium user with their own token row on the same channel but no opt-in must not
	// become the owner, nor make the non-premium opt-in eligible.
	f.source(f.overlay(other, true), "UCshared", `{}`, true)
	f.token(other, "UCshared")

	if got := f.eligible(false); len(got) != 0 {
		t.Fatalf("gate closed: the opting-in owner is not premium, got %+v", *got[0])
	}
	got := f.eligible(true)
	if len(got) != 1 {
		t.Fatalf("got %d rows, want only the opting-in owner's", len(got))
	}
	if got[0].OwnerUserID != owner || got[0].OverlayID != ownerOverlay {
		t.Fatalf("row = %+v, want owner %s overlay %s", *got[0], owner, ownerOverlay)
	}
}

func TestEligibleQuota_LiveBroadcastsCharged(t *testing.T) {
	pool := newMigratedDB(t)
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/liveBroadcasts") {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{{
				"id":      "vid12345678",
				"snippet": map[string]string{"channelId": "UCquota", "liveChatId": "chat-1", "title": "t"},
				"status":  map[string]string{"privacyStatus": "unlisted", "lifeCycleStatus": "live"},
			}},
		})
	}))
	defer srv.Close()

	service, err := youtube.NewService(ctx, option.WithEndpoint(srv.URL+"/"), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatalf("youtube.NewService: %v", err)
	}
	tracker := quota.NewTracker(pool, quota.DefaultDailyQuota, zap.NewNop(), listenerMetrics())
	client := api.NewClient(service, srv.Client(), tracker, zap.NewNop())

	usedToday := func() int {
		var used int
		today := time.Now().In(quota.YouTubePST).Format("2006-01-02")
		if err := pool.QueryRow(ctx,
			`SELECT COALESCE(SUM(units_used), 0) FROM youtube_quota_usage WHERE date = $1`, today,
		).Scan(&used); err != nil {
			t.Fatalf("read units_used: %v", err)
		}
		return used
	}

	before := usedToday()
	broadcasts, err := client.GetActiveBroadcasts(ctx, "UCquota", nil)
	if err != nil {
		t.Fatalf("GetActiveBroadcasts: %v", err)
	}
	if len(broadcasts) != 1 {
		t.Fatalf("got %d broadcasts, want 1", len(broadcasts))
	}
	if delta := usedToday() - before; delta != quota.QuotaCostLiveBroadcasts {
		t.Fatalf("units_used grew by %d, want QuotaCostLiveBroadcasts (%d)", delta, quota.QuotaCostLiveBroadcasts)
	}
}
