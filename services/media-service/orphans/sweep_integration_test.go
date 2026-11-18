//go:build integration

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

package orphans

import (
	"context"
	"os"
	"testing"

	"github.com/caesar/all-chat/services/media-service/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Against a real Postgres with the migrations applied (CI's integration job; locally
// TEST_DATABASE_URL=... go test -tags=integration ./orphans/): deleting an account
// cascades its media_objects rows, and the next sweep removes exactly its files.
// The bucket is the in-memory fake; no MinIO server is involved.
func TestSweep_RemovesDeletedAccountsFiles(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://allchat:allchat_dev_password@localhost:5432/allchat"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("requires DB: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("requires DB: %v", err)
	}

	seed := func() (uuid.UUID, string) {
		var id uuid.UUID
		name := "media-sweep-" + uuid.NewString()[:8]
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO users (twitch_id, username, display_name, access_token, refresh_token, token_expires_at)
			 VALUES ($1, $1, $1, 'x', 'x', NOW() + INTERVAL '1 day') RETURNING id`, name).Scan(&id))
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		key := id.String() + "/" + uuid.NewString() + "/alert.mp3"
		_, err := pool.Exec(ctx,
			`INSERT INTO media_objects (user_id, object_key, filename, content_type, size_bytes)
			 VALUES ($1, $2, 'alert.mp3', 'audio/mpeg', 1)`, id, key)
		require.NoError(t, err)
		return id, key
	}
	deleted, deletedKey := seed()
	_, keptKey := seed()

	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, deleted)
	require.NoError(t, err)

	store := &fakeStore{keys: []string{deletedKey, keptKey}}
	removed, err := Sweep(ctx, store, repository.NewMediaRepository(pool), zap.NewNop())

	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.Equal(t, []string{deletedKey}, store.removed)
}
