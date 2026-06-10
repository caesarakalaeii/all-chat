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

package enricher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

// An unregistered viewer (no DB row) with no platform color gets a per-platform
// auto-color keyed by "platform:userID".
func TestViewerBadgeEnricher_AutoColor_UnregisteredGetsPlatformKeyedColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		return "", nil, pgxErrNoRows
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("youtube", "yt-anon-1", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := AutoColor("youtube:yt-anon-1")
	if msg.User.Color != want {
		t.Errorf("expected auto-color %q keyed by platform:userID, got %q", want, msg.User.Color)
	}
}

// A present platform-native color must never be overwritten by the auto-color.
func TestViewerBadgeEnricher_AutoColor_DoesNotOverridePlatformColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		return "", nil, pgxErrNoRows
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "twitch-color-user", "#1E90FF")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "#1E90FF" {
		t.Errorf("platform color should be preserved, got %q", msg.User.Color)
	}
}

// An All-Chat cosmetic color takes precedence over the auto-color.
func TestViewerBadgeEnricher_AutoColor_DoesNotOverrideCosmeticColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		return "viewer-uuid", ptr("#abcdef"), nil
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("tiktok", "tt-cosmetic-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "#abcdef" {
		t.Errorf("cosmetic color should win, got %q", msg.User.Color)
	}
}

// A gradient suppresses the auto-color: color stays empty so the gradient renders.
func TestViewerBadgeEnricher_AutoColor_DoesNotOverrideGradient(t *testing.T) {
	mr := miniredis.RunT(t)
	gradientJSON := []byte(`{"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}`)
	db := &fakeViewerDB{
		queryFn: func(_, _ string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "viewer-grad", nil, gradientJSON, "", "", false, false, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "autocolor-grad-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "" {
		t.Errorf("color must stay empty when a gradient is set, got %q", msg.User.Color)
	}
}

// An empty user ID short-circuits before the auto-color defer is registered,
// leaving the (empty) color untouched.
func TestViewerBadgeEnricher_AutoColor_EmptyUserIDSkipped(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		t.Error("DB should not be queried for empty user ID")
		return "", nil, nil
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "" {
		t.Errorf("empty-user-ID message should not be auto-colored, got %q", msg.User.Color)
	}
}

// A viewer linked across platforms (same viewer UUID) gets the SAME auto-color
// on every platform, keyed by the UUID rather than the per-platform identity.
func TestViewerBadgeEnricher_AutoColor_CrossPlatformConsistency(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		t.Error("DB should not be called on cache hit")
		return "", nil, nil
	})
	e := newTestEnricher(t, mr, db)

	const sharedUUID = "uuid-cross-platform"
	seed := func(platform, userID string) {
		val, _ := json.Marshal(viewerIdentityCache{ViewerID: sharedUUID, NameColor: nil})
		mr.Set("viewer:identity:"+platform+":"+userID, string(val))
	}
	seed("twitch", "tw-1")
	seed("youtube", "yt-2")

	twitchMsg := makeMsg("twitch", "tw-1", "")
	youtubeMsg := makeMsg("youtube", "yt-2", "")
	if err := e.Enrich(context.Background(), twitchMsg); err != nil {
		t.Fatalf("twitch enrich error: %v", err)
	}
	if err := e.Enrich(context.Background(), youtubeMsg); err != nil {
		t.Fatalf("youtube enrich error: %v", err)
	}

	want := AutoColor(sharedUUID)
	if twitchMsg.User.Color != want {
		t.Errorf("twitch color = %q, want UUID-keyed %q", twitchMsg.User.Color, want)
	}
	if twitchMsg.User.Color != youtubeMsg.User.Color {
		t.Errorf("linked viewer should share one color across platforms; got twitch=%q youtube=%q",
			twitchMsg.User.Color, youtubeMsg.User.Color)
	}
}
