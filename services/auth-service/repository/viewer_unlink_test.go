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
	"errors"
	"testing"

	"github.com/caesar/all-chat/shared/premium"
	"go.uber.org/zap"
)

// Unlinking a platform from a viewer must take that platform's OAuth tokens with it:
// the session row stays (its ban flags and message history are not the viewer's to
// erase), but its tokens are wiped and it is marked expired, so the daily
// cleanup_expired_oauth_tokens() job removes the row a week later.
func TestUnlinkPlatform_WipesTheUnlinkedSessionsTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("needs PostgreSQL")
	}
	pool, cleanup := setupMigrationTestDB(t)
	defer cleanup()
	runMigrations(t, pool, loadUpMigrations(t))
	ctx := context.Background()
	repo := NewViewerIdentityRepository(pool, premium.NewRecomputer(pool, zap.NewNop()))

	viewerID, err := repo.GetOrCreateViewerByPlatform(ctx, "twitch", "tw-1")
	must(t, err)
	must(t, repo.LinkPlatformToViewer(ctx, viewerID, "youtube", "yt-1"))
	otherViewer, err := repo.GetOrCreateViewerByPlatform(ctx, "youtube", "yt-2")
	must(t, err)

	insertSession := func(platform, platformUserID, viewer string) {
		_, err := pool.Exec(ctx, `
			INSERT INTO viewer_sessions (platform, platform_user_id, username, display_name,
			                             access_token, refresh_token, token_expires_at, viewer_id)
			VALUES ($1, $2, $2, $2, $4, $5, NOW() + INTERVAL '1 hour', $3)`,
			platform, platformUserID, viewer, "enc-access-"+platformUserID, "enc-refresh-"+platformUserID)
		must(t, err)
	}
	insertSession("twitch", "tw-1", viewerID.String())
	insertSession("youtube", "yt-1", viewerID.String())
	insertSession("youtube", "yt-2", otherViewer.String())

	removed, err := repo.UnlinkPlatform(ctx, viewerID, "youtube")
	if err != nil {
		t.Fatalf("UnlinkPlatform: %v", err)
	}
	if len(removed) != 1 || removed[0].AccessToken != "enc-access-yt-1" || removed[0].RefreshToken != "enc-refresh-yt-1" {
		t.Errorf("removed = %+v, want the unlinked session's stored tokens (for revocation)", removed)
	}

	type session struct {
		access  string
		refresh *string
		viewer  *string
		expired bool
	}
	read := func(platformUserID string) session {
		var s session
		must(t, pool.QueryRow(ctx, `
			SELECT access_token, refresh_token, viewer_id::text, token_expires_at <= NOW()
			FROM viewer_sessions WHERE platform_user_id = $1`, platformUserID).
			Scan(&s.access, &s.refresh, &s.viewer, &s.expired))
		return s
	}

	unlinked := read("yt-1")
	if unlinked.access != "" || unlinked.refresh != nil {
		t.Errorf("unlinked session still holds tokens: access=%q refresh=%v", unlinked.access, unlinked.refresh)
	}
	if unlinked.viewer != nil {
		t.Errorf("unlinked session still points at the viewer: %v", *unlinked.viewer)
	}
	if !unlinked.expired {
		t.Error("unlinked session must be marked expired so the cleanup job removes it")
	}
	for _, id := range []string{"tw-1", "yt-2"} {
		if s := read(id); s.access != "enc-access-"+id || s.refresh == nil {
			t.Errorf("session %s lost its tokens: %+v", id, s)
		}
	}

	var identities int
	must(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM viewer_platform_identities WHERE viewer_id = $1`, viewerID).Scan(&identities))
	if identities != 1 {
		t.Errorf("identities left = %d, want 1 (twitch)", identities)
	}

	if _, err := repo.UnlinkPlatform(ctx, viewerID, "twitch"); !errors.Is(err, ErrLastPlatform) {
		t.Errorf("unlinking the last platform: err = %v, want ErrLastPlatform", err)
	}
}
