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

package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupViewerRepoTestDB starts a PostgreSQL container with the schema the
// ViewerRepository list/activity queries touch: users, overlays, viewers,
// viewer_sessions (incl. the migration-040 user_id link) and
// viewer_message_history.
func setupViewerRepoTestDB(t *testing.T) (*pgxpool.Pool, func()) {
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
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	if err != nil {
		t.Fatalf("connection pool: %v", err)
	}

	schema := `
		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(50) NOT NULL
		);
		CREATE TABLE overlays (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID REFERENCES users(id) ON DELETE CASCADE
		);
		CREATE TABLE viewers (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			is_premium BOOLEAN NOT NULL DEFAULT FALSE,
			premium_admin_override_expires_at TIMESTAMP NULL
		);
		CREATE TABLE viewer_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			platform VARCHAR(50) NOT NULL,
			platform_user_id VARCHAR(100) NOT NULL,
			username VARCHAR(100) NOT NULL,
			display_name VARCHAR(200) NOT NULL,
			avatar_url TEXT,
			access_token TEXT NOT NULL DEFAULT '',
			refresh_token TEXT,
			token_expires_at TIMESTAMP NOT NULL DEFAULT NOW(),
			last_message_at TIMESTAMP,
			message_count_1min INTEGER DEFAULT 0,
			message_count_1hour INTEGER DEFAULT 0,
			rate_limit_reset_1min TIMESTAMP,
			rate_limit_reset_1hour TIMESTAMP,
			viewer_id UUID REFERENCES viewers(id) ON DELETE SET NULL,
			user_id UUID REFERENCES users(id) ON DELETE SET NULL,
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			banned_at TIMESTAMP,
			banned_reason TEXT,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);
		CREATE TABLE viewer_message_history (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			viewer_session_id UUID NOT NULL REFERENCES viewer_sessions(id) ON DELETE CASCADE,
			streamer_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			overlay_id UUID REFERENCES overlays(id) ON DELETE SET NULL,
			platform VARCHAR(50) NOT NULL,
			channel_id VARCHAR(100) NOT NULL,
			channel_name VARCHAR(100) NOT NULL,
			message_text TEXT NOT NULL,
			sent_at TIMESTAMP DEFAULT NOW(),
			success BOOLEAN DEFAULT TRUE,
			error_message TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);
	`
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

// insertViewer inserts a viewers row and returns its id.
func insertViewer(t *testing.T, pool *pgxpool.Pool, isPremium bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO viewers (is_premium) VALUES ($1) RETURNING id`, isPremium).Scan(&id)
	if err != nil {
		t.Fatalf("insert viewer: %v", err)
	}
	return id
}

// insertUserNamed inserts a users row with the given username and returns its id.
func insertUserNamed(t *testing.T, pool *pgxpool.Pool, username string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (username) VALUES ($1) RETURNING id`, username).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

type sessionSpec struct {
	platform       string
	platformUserID string
	username       string
	displayName    string
	viewerID       *uuid.UUID
	isBanned       bool
}

// insertSession inserts a viewer_sessions row and returns its id.
func insertSession(t *testing.T, pool *pgxpool.Pool, s sessionSpec) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(), `
		INSERT INTO viewer_sessions (platform, platform_user_id, username, display_name, viewer_id, is_banned)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		s.platform, s.platformUserID, s.username, s.displayName, s.viewerID, s.isBanned).Scan(&id)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return id
}

// insertMessage inserts a viewer_message_history row at a specific sent_at.
func insertMessage(t *testing.T, pool *pgxpool.Pool, sessionID, streamerID uuid.UUID, overlayID *uuid.UUID, platform, channelName string, sentAt time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO viewer_message_history
			(viewer_session_id, streamer_user_id, overlay_id, platform, channel_id, channel_name, message_text, sent_at)
		VALUES ($1, $2, $3, $4, 'chan-id', $5, 'hello', $6)`,
		sessionID, streamerID, overlayID, platform, channelName, sentAt)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}
}

func TestListAll_QueryMatchesUsernameDisplayNamePlatformUserID(t *testing.T) {
	pool, cleanup := setupViewerRepoTestDB(t)
	defer cleanup()
	repo := repository.NewViewerRepository(pool, nil, nil)
	ctx := context.Background()

	insertSession(t, pool, sessionSpec{platform: "twitch", platformUserID: "111", username: "AliceStream", displayName: "Alice"})
	insertSession(t, pool, sessionSpec{platform: "kick", platformUserID: "222", username: "bob", displayName: "BobbyDisplay"})
	insertSession(t, pool, sessionSpec{platform: "youtube", platformUserID: "UC_charlie_999", username: "carol", displayName: "Carol"})
	insertSession(t, pool, sessionSpec{platform: "twitch", platformUserID: "333", username: "dave", displayName: "Dave"})

	// Match on username (case-insensitive substring).
	sessions, total, err := repo.ListAll(ctx, repository.ViewerListFilter{Query: "alice"}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll query username: %v", err)
	}
	if total != 1 || len(sessions) != 1 || sessions[0].Username != "AliceStream" {
		t.Fatalf("query 'alice' -> total=%d len=%d, want the AliceStream row", total, len(sessions))
	}

	// Match on display_name.
	sessions, total, err = repo.ListAll(ctx, repository.ViewerListFilter{Query: "bobbydisplay"}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll query display_name: %v", err)
	}
	if total != 1 || len(sessions) != 1 || sessions[0].Username != "bob" {
		t.Fatalf("query 'bobbydisplay' -> total=%d len=%d, want the bob row", total, len(sessions))
	}

	// Match on platform_user_id.
	sessions, total, err = repo.ListAll(ctx, repository.ViewerListFilter{Query: "charlie"}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll query platform_user_id: %v", err)
	}
	if total != 1 || len(sessions) != 1 || sessions[0].Username != "carol" {
		t.Fatalf("query 'charlie' -> total=%d len=%d, want the carol row", total, len(sessions))
	}
}

func TestListAll_BannedPremiumPlatformFilters(t *testing.T) {
	pool, cleanup := setupViewerRepoTestDB(t)
	defer cleanup()
	repo := repository.NewViewerRepository(pool, nil, nil)
	ctx := context.Background()

	premiumViewer := insertViewer(t, pool, true)
	freeViewer := insertViewer(t, pool, false)

	insertSession(t, pool, sessionSpec{platform: "twitch", platformUserID: "1", username: "banned_premium", displayName: "d", viewerID: &premiumViewer, isBanned: true})
	insertSession(t, pool, sessionSpec{platform: "twitch", platformUserID: "2", username: "active_premium", displayName: "d", viewerID: &premiumViewer, isBanned: false})
	insertSession(t, pool, sessionSpec{platform: "kick", platformUserID: "3", username: "active_free", displayName: "d", viewerID: &freeViewer, isBanned: false})
	insertSession(t, pool, sessionSpec{platform: "kick", platformUserID: "4", username: "banned_noviewer", displayName: "d", isBanned: true})

	banned := true
	_, total, err := repo.ListAll(ctx, repository.ViewerListFilter{IsBanned: &banned}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll is_banned: %v", err)
	}
	if total != 2 { // banned_premium, banned_noviewer
		t.Errorf("is_banned=true total = %d, want 2", total)
	}

	premium := true
	_, total, err = repo.ListAll(ctx, repository.ViewerListFilter{IsPremium: &premium}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll is_premium: %v", err)
	}
	if total != 2 { // banned_premium, active_premium
		t.Errorf("is_premium=true total = %d, want 2", total)
	}

	notPremium := false
	_, total, err = repo.ListAll(ctx, repository.ViewerListFilter{IsPremium: &notPremium}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll is_premium=false: %v", err)
	}
	if total != 2 { // active_free, banned_noviewer (NULL viewer -> COALESCE false)
		t.Errorf("is_premium=false total = %d, want 2", total)
	}

	_, total, err = repo.ListAll(ctx, repository.ViewerListFilter{Platform: "kick"}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll platform: %v", err)
	}
	if total != 2 { // active_free, banned_noviewer
		t.Errorf("platform=kick total = %d, want 2", total)
	}

	// Combined: banned AND premium -> only banned_premium.
	sessions, total, err := repo.ListAll(ctx, repository.ViewerListFilter{IsBanned: &banned, IsPremium: &premium}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll combined: %v", err)
	}
	if total != 1 || len(sessions) != 1 || sessions[0].Username != "banned_premium" {
		t.Fatalf("banned+premium -> total=%d len=%d, want the banned_premium row", total, len(sessions))
	}
}

func TestListAll_TotalReflectsFullMatchNotPage(t *testing.T) {
	pool, cleanup := setupViewerRepoTestDB(t)
	defer cleanup()
	repo := repository.NewViewerRepository(pool, nil, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		insertSession(t, pool, sessionSpec{
			platform:       "twitch",
			platformUserID: uuid.NewString(),
			username:       "match_user",
			displayName:    "d",
		})
	}
	// A non-matching row that must be excluded from total.
	insertSession(t, pool, sessionSpec{platform: "twitch", platformUserID: "x", username: "other", displayName: "d"})

	sessions, total, err := repo.ListAll(ctx, repository.ViewerListFilter{Query: "match_user"}, 2, 0)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(sessions) != 2 {
		t.Errorf("page len = %d, want 2 (limit)", len(sessions))
	}
	if total != 5 {
		t.Errorf("total = %d, want 5 (full match count, not the page)", total)
	}
}

func TestListAll_PopulatesUserID(t *testing.T) {
	pool, cleanup := setupViewerRepoTestDB(t)
	defer cleanup()
	repo := repository.NewViewerRepository(pool, nil, nil)
	ctx := context.Background()

	streamerID := insertUserNamed(t, pool, "linked_streamer")
	_, err := pool.Exec(ctx, `
		INSERT INTO viewer_sessions (platform, platform_user_id, username, display_name, user_id)
		VALUES ('twitch', 'linked', 'linkeduser', 'd', $1)`, streamerID)
	if err != nil {
		t.Fatalf("insert linked session: %v", err)
	}

	sessions, _, err := repo.ListAll(ctx, repository.ViewerListFilter{Query: "linkeduser"}, 50, 0)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	if sessions[0].UserID == nil || *sessions[0].UserID != streamerID.String() {
		t.Errorf("UserID = %v, want %s", sessions[0].UserID, streamerID.String())
	}
}

func TestGetViewerActivity_AggregatesAndReturnsStreamers(t *testing.T) {
	pool, cleanup := setupViewerRepoTestDB(t)
	defer cleanup()
	repo := repository.NewViewerRepository(pool, nil, nil)
	ctx := context.Background()

	session := insertSession(t, pool, sessionSpec{platform: "twitch", platformUserID: "v1", username: "viewer", displayName: "d"})
	streamerA := insertUserNamed(t, pool, "streamerAlpha")
	streamerB := insertUserNamed(t, pool, "streamerBeta")

	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	// streamerA: 3 messages (latest most recent), streamerB: 1 message (older).
	insertMessage(t, pool, session, streamerA, nil, "twitch", "alpha_chan", base.Add(10*time.Minute))
	insertMessage(t, pool, session, streamerA, nil, "twitch", "alpha_chan", base.Add(20*time.Minute))
	insertMessage(t, pool, session, streamerA, nil, "twitch", "alpha_chan", base.Add(30*time.Minute))
	insertMessage(t, pool, session, streamerB, nil, "kick", "beta_chan", base.Add(5*time.Minute))

	activity, err := repo.GetViewerActivity(ctx, session)
	if err != nil {
		t.Fatalf("GetViewerActivity: %v", err)
	}

	if activity.TotalMessages != 4 {
		t.Errorf("TotalMessages = %d, want 4", activity.TotalMessages)
	}
	if activity.LastSentAt == nil || !activity.LastSentAt.Equal(base.Add(30*time.Minute)) {
		t.Errorf("LastSentAt = %v, want %v", activity.LastSentAt, base.Add(30*time.Minute))
	}
	if len(activity.Streamers) != 2 {
		t.Fatalf("Streamers len = %d, want 2", len(activity.Streamers))
	}
	// Ordered by MAX(sent_at) DESC: streamerA first.
	if activity.Streamers[0].StreamerUsername != "streamerAlpha" {
		t.Errorf("Streamers[0].StreamerUsername = %q, want streamerAlpha", activity.Streamers[0].StreamerUsername)
	}
	if activity.Streamers[0].StreamerUserID != streamerA.String() {
		t.Errorf("Streamers[0].StreamerUserID = %q, want %s", activity.Streamers[0].StreamerUserID, streamerA.String())
	}
	if activity.Streamers[0].MessageCount != 3 {
		t.Errorf("Streamers[0].MessageCount = %d, want 3", activity.Streamers[0].MessageCount)
	}
	if activity.Streamers[0].ChannelName != "alpha_chan" {
		t.Errorf("Streamers[0].ChannelName = %q, want alpha_chan", activity.Streamers[0].ChannelName)
	}
	if activity.Streamers[1].StreamerUsername != "streamerBeta" || activity.Streamers[1].MessageCount != 1 {
		t.Errorf("Streamers[1] = %+v, want streamerBeta count 1", activity.Streamers[1])
	}
}

func TestGetViewerActivity_EmptyForUnknownSession(t *testing.T) {
	pool, cleanup := setupViewerRepoTestDB(t)
	defer cleanup()
	repo := repository.NewViewerRepository(pool, nil, nil)
	ctx := context.Background()

	activity, err := repo.GetViewerActivity(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetViewerActivity: %v", err)
	}
	if activity.TotalMessages != 0 {
		t.Errorf("TotalMessages = %d, want 0", activity.TotalMessages)
	}
	if activity.LastSentAt != nil {
		t.Errorf("LastSentAt = %v, want nil", activity.LastSentAt)
	}
	if len(activity.Streamers) != 0 {
		t.Errorf("Streamers len = %d, want 0", len(activity.Streamers))
	}
}
