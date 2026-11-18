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

// Unlinking a platform from a viewer must take that platform's session with it, OAuth
// tokens included. Nothing would remove the row later: the retention sweeper keeps
// viewer sessions (ADR-0089), and a detached row is no longer reachable by viewer
// erasure (ADR-0088), which follows the viewer and its linked identities. Signing out
// deletes the row the same way.
func TestUnlinkPlatform_DeletesTheUnlinkedSession(t *testing.T) {
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

	sessions := func(platformUserID string) (n int, access string) {
		must(t, pool.QueryRow(ctx,
			`SELECT COUNT(*), COALESCE(MAX(access_token), '') FROM viewer_sessions WHERE platform_user_id = $1`,
			platformUserID).Scan(&n, &access))
		return
	}
	if n, _ := sessions("yt-1"); n != 0 {
		t.Errorf("unlinked session rows left = %d, want 0", n)
	}
	for _, id := range []string{"tw-1", "yt-2"} {
		if n, access := sessions(id); n != 1 || access != "enc-access-"+id {
			t.Errorf("session %s = %d rows, access %q: other sessions must be untouched", id, n, access)
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
