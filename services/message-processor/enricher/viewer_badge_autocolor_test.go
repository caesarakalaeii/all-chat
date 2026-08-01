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

// ADR-0047: the auto-colour is published in its own AutoColor field and must
// never be written into Color. Color stays reserved for authoritative colours
// (viewer's manual choice, then platform-native) so the overlay can rank the
// streamer's "Username color" setting between the two.

// An unregistered viewer (no DB row) with no platform colour gets a per-platform
// auto-colour keyed by "platform:userID" — in AutoColor, leaving Color empty.
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
	if msg.User.AutoColor != want {
		t.Errorf("expected auto-color %q keyed by platform:userID, got %q", want, msg.User.AutoColor)
	}
	if msg.User.Color != "" {
		t.Errorf("Color must stay empty so the streamer setting can apply, got %q", msg.User.Color)
	}
}

// A present platform-native colour must never be overwritten by the auto-colour.
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

// The auto-colour is emitted even when a platform colour is present, so a later
// change of heart (or a client that ignores Color) still has a stable fallback.
func TestViewerBadgeEnricher_AutoColor_PopulatedAlongsidePlatformColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		return "", nil, pgxErrNoRows
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "twitch-color-user", "#1E90FF")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := AutoColor("twitch:twitch-color-user"); msg.User.AutoColor != want {
		t.Errorf("auto-color = %q, want %q", msg.User.AutoColor, want)
	}
}

// An All-Chat cosmetic colour is authoritative and lands in Color.
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

// The viewer's manual All-Chat colour outranks the platform-native colour.
func TestViewerBadgeEnricher_CosmeticColorOverridesPlatformColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		return "viewer-uuid", ptr("#abcdef"), nil
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "tw-both", "#1E90FF")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "#abcdef" {
		t.Errorf("manual All-Chat color must beat the platform color, got %q", msg.User.Color)
	}
}

// A gradient suppresses both colour fields: the gradient renders instead.
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
	if msg.User.AutoColor != "" {
		t.Errorf("auto-color must stay empty when a gradient is set, got %q", msg.User.AutoColor)
	}
}

// An empty user ID short-circuits before the auto-colour defer is registered.
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
	if msg.User.AutoColor != "" {
		t.Errorf("empty-user-ID message should not get an auto-color, got %q", msg.User.AutoColor)
	}
}

// A viewer linked across platforms (same viewer UUID) gets the SAME auto-colour
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
	if twitchMsg.User.AutoColor != want {
		t.Errorf("twitch auto-color = %q, want UUID-keyed %q", twitchMsg.User.AutoColor, want)
	}
	if twitchMsg.User.AutoColor != youtubeMsg.User.AutoColor {
		t.Errorf("linked viewer should share one auto-color across platforms; got twitch=%q youtube=%q",
			twitchMsg.User.AutoColor, youtubeMsg.User.AutoColor)
	}
}
