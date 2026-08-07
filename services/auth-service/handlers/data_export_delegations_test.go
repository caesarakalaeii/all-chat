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
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// A delegated-moderation grant is personal data about both parties (ADR-0048), and it is not
// derivable from any other section of the export: the streamer's overlays list says nothing about
// who may moderate them, and a moderator's own overlays say nothing about channels delegated to
// them. So both directions have to appear.
func TestFetchDelegations_ReportsBothDirections(t *testing.T) {
	pool, cleanup := setupDelegationExportDB(t)
	defer cleanup()
	ctx := context.Background()

	const (
		streamer = "11111111-1111-1111-1111-111111111111"
		mod      = "22222222-2222-2222-2222-222222222222"
		other    = "33333333-3333-3333-3333-333333333333"
		mine     = "aaaaaaaa-1111-1111-1111-111111111111"
		theirs   = "aaaaaaaa-2222-2222-2222-222222222222"
	)
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, display_name) VALUES
			($1, 'The Streamer'), ($2, 'Sarah'), ($3, 'Another Streamer')`, streamer, mod, other)
	if err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO overlays (id, user_id, name) VALUES ($1, $2, 'My Overlay'), ($3, $4, 'Their Overlay')`,
		mine, streamer, theirs, other); err != nil {
		t.Fatalf("seed overlays: %v", err)
	}
	// The streamer delegated to Sarah on their own overlay, and Another Streamer delegated to the
	// streamer on theirs.
	if _, err := pool.Exec(ctx, `
		INSERT INTO overlay_moderators
			(overlay_id, moderator_user_id, granted_by, status, actions, accepted_at, created_at)
		VALUES
			($1, $2, $3, 'active', '{delete,timeout}', NOW(), NOW() - INTERVAL '2 days'),
			($4, $3, $5, 'active', '{delete}', NOW(), NOW() - INTERVAL '1 day')`,
		mine, mod, streamer, theirs, other); err != nil {
		t.Fatalf("seed grants: %v", err)
	}

	got := fetchDelegations(ctx, pool, streamer, zap.NewNop())
	if len(got) != 2 {
		t.Fatalf("expected both directions, got %d: %+v", len(got), got)
	}

	byDirection := map[string]DataExportDelegation{}
	for _, d := range got {
		byDirection[d.Direction] = d
	}
	granted, ok := byDirection["granted"]
	if !ok {
		t.Fatal("the moderators a streamer delegated to must appear in their export")
	}
	if granted.OverlayName != "My Overlay" || granted.Counterpart != "Sarah" {
		t.Errorf("granted grant misreported: %+v", granted)
	}
	if len(granted.Actions) != 2 {
		t.Errorf("the delegated actions are part of the record: %+v", granted.Actions)
	}

	received, ok := byDirection["received"]
	if !ok {
		t.Fatal("overlays delegated TO the user must appear too — nothing else in the export shows them")
	}
	if received.OverlayName != "Their Overlay" || received.Counterpart != "Another Streamer" {
		t.Errorf("received grant misreported: %+v", received)
	}

	// A revoked grant is retained as history, so an export that hid it would misreport what
	// All-Chat still holds about the person.
	t.Run("revoked grants are still reported", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `
			UPDATE overlay_moderators SET status = 'revoked', revoked_at = NOW()
			WHERE overlay_id = $1`, mine); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		got := fetchDelegations(ctx, pool, streamer, zap.NewNop())
		if len(got) != 2 {
			t.Fatalf("expected 2 grants after revocation, got %d", len(got))
		}
		for _, d := range got {
			if d.Direction == "granted" && d.RevokedAt == nil {
				t.Error("a revoked grant must carry its revocation date")
			}
		}
	})

	t.Run("a user with no delegations gets no section", func(t *testing.T) {
		if got := fetchDelegations(ctx, pool, mod, zap.NewNop()); len(got) != 1 {
			t.Fatalf("Sarah has exactly one received grant, got %d", len(got))
		}
		lonely := "44444444-4444-4444-4444-444444444444"
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, display_name) VALUES ($1, 'Nobody')`, lonely); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		if got := fetchDelegations(ctx, pool, lonely, zap.NewNop()); len(got) != 0 {
			t.Fatalf("expected no delegations, got %+v", got)
		}
	})
}

// Erasure needs no delegation-specific code: overlay_moderators cascades from BOTH overlays and
// users, so deleting the account takes every row on either side with it. If that ever stops being
// true, a deleted volunteer would keep a live grant on someone's channel.
func TestDeletingAnAccountCascadesItsDelegations(t *testing.T) {
	pool, cleanup := setupDelegationExportDB(t)
	defer cleanup()
	ctx := context.Background()

	const (
		streamer = "11111111-1111-1111-1111-111111111111"
		mod      = "22222222-2222-2222-2222-222222222222"
		overlay  = "aaaaaaaa-1111-1111-1111-111111111111"
	)
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, display_name) VALUES ($1, 'Streamer'), ($2, 'Sarah')`, streamer, mod); err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO overlays (id, user_id, name) VALUES ($1, $2, 'My Overlay')`, overlay, streamer); err != nil {
		t.Fatalf("seed overlay: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO overlay_moderators (overlay_id, moderator_user_id, granted_by, status)
		VALUES ($1, $2, $3, 'active')`, overlay, mod, streamer); err != nil {
		t.Fatalf("seed grant: %v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, mod); err != nil {
		t.Fatalf("delete moderator: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM overlay_moderators`).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 0 {
		t.Errorf("deleting the moderator's account must remove their grant, %d left", remaining)
	}
}

// setupDelegationExportDB starts a throwaway Postgres carrying the columns the delegation export
// reads, with migration 080's cascade behaviour intact.
func setupDelegationExportDB(t *testing.T) (*pgxpool.Pool, func()) {
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
		ContainerRequest: req, Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
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
		t.Fatalf("pool: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		CREATE TABLE users (
			id UUID PRIMARY KEY,
			display_name VARCHAR(100) NOT NULL DEFAULT ''
		);
		CREATE TABLE overlays (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(100) NOT NULL DEFAULT ''
		);
		CREATE TABLE overlay_moderators (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			overlay_id UUID NOT NULL REFERENCES overlays(id) ON DELETE CASCADE,
			moderator_user_id UUID REFERENCES users(id) ON DELETE CASCADE,
			granted_by UUID NOT NULL,
			status VARCHAR(16) NOT NULL DEFAULT 'pending',
			actions TEXT[] NOT NULL DEFAULT '{delete,timeout}',
			invitee_label VARCHAR(120),
			moderator_display_name VARCHAR(120),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			accepted_at TIMESTAMP,
			revoked_at TIMESTAMP
		);`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}
