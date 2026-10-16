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

package webhooks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Twitch sources store the channel login in whatever case the streamer typed,
// never the numeric broadcaster id the handler used to match on, so the
// cross-platform event never reached any overlay.
func TestHandleStreamOnline_PublishesToOverlaysMatchedByLogin(t *testing.T) {
	_, rc, h := newStatusTestHandler(t)
	h.db = startStreamOnlineTestDB(t)
	ctx := context.Background()

	if _, err := h.db.Exec(ctx, `INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, is_active) VALUES
		('11111111-1111-1111-1111-111111111111', 'twitch', 'Lejoe_TTV', true),
		('22222222-2222-2222-2222-222222222222', 'twitch', 'someone_else', true)`); err != nil {
		t.Fatal(err)
	}

	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	pubsub := rc.PSubscribe(subCtx, "platform:event:*")
	defer pubsub.Close()
	if _, err := pubsub.Receive(subCtx); err != nil {
		t.Fatal(err)
	}

	event, _ := json.Marshal(map[string]string{
		"broadcaster_user_id":    "123456",
		"broadcaster_user_login": "lejoe_ttv",
		"broadcaster_user_name":  "Lejoe_TTV",
	})
	if err := h.handleStreamOnline(ctx, event); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-pubsub.Channel():
		if msg.Channel != "platform:event:11111111-1111-1111-1111-111111111111" {
			t.Fatalf("published to %s, want lejoe's overlay", msg.Channel)
		}
	case <-subCtx.Done():
		t.Fatal("stream.online published to no overlay")
	}
}

func startStreamOnlineTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env:          map[string]string{"POSTGRES_USER": "u", "POSTGRES_PASSWORD": "p", "POSTGRES_DB": "d"},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	pool, err := pgxpool.New(ctx, "postgres://u:p@"+host+":"+port.Port()+"/d?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `CREATE TABLE overlay_chat_sources (
		overlay_id UUID NOT NULL,
		platform VARCHAR(50) NOT NULL,
		channel_id VARCHAR(100) NOT NULL,
		is_active BOOLEAN NOT NULL DEFAULT true)`); err != nil {
		t.Fatal(err)
	}
	return pool
}
