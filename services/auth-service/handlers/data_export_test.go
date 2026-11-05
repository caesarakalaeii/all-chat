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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// The privacy policy promises an Art. 15/20 export of everything All-Chat stores about a person.
// These tests run against the real schema (every migration applied), seed one row in each table
// the export covers, and read the export back over HTTP. Every credential column is seeded with a
// value containing "SECRET", so a single substring check proves none of them reaches the file.

const (
	exStreamer = "11111111-1111-1111-1111-111111111111"
	exOther    = "22222222-2222-2222-2222-222222222222"
	exMod      = "33333333-3333-3333-3333-333333333333"
	exMine     = "aaaaaaaa-1111-1111-1111-111111111111"
	exTheirs   = "aaaaaaaa-2222-2222-2222-222222222222"
	exViewer   = "bbbbbbbb-1111-1111-1111-111111111111"
	exSession  = "cccccccc-1111-1111-1111-111111111111"
	exMediaKey = exStreamer + "/dddddddd-1111-1111-1111-111111111111/clip #1.mp3"
)

// seedExportFixture writes a streamer with data in every table the streamer export covers, plus
// the viewer identity they signed in with, and a second streamer and a moderator as counterparts.
func seedExportFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	sql := strings.NewReplacer(
		":me", "'"+exStreamer+"'", ":other", "'"+exOther+"'", ":mod", "'"+exMod+"'",
		":mine", "'"+exMine+"'", ":theirs", "'"+exTheirs+"'",
		":viewer", "'"+exViewer+"'", ":session", "'"+exSession+"'", ":objkey", "'"+exMediaKey+"'",
	).Replace(`
INSERT INTO users (id, twitch_id, username, display_name, auth_provider, access_token, refresh_token,
                   token_expires_at, granted_scopes, is_beta_tester, is_ambassador, is_banned, banned_reason)
VALUES (:me, '1001', 'streamer_x', 'Streamer X', 'twitch', 'SECRET-login-access', 'SECRET-login-refresh',
        NOW() + INTERVAL '1 hour', '{user:read:chat}', TRUE, TRUE, TRUE, 'testing the ban reason'),
       (:other, '1002', 'other_streamer', 'Other Streamer', 'twitch', 'SECRET-a', 'SECRET-r', NOW(), '{}', FALSE, FALSE, FALSE, NULL),
       (:mod, '1003', 'sarah', 'Sarah', 'twitch', 'SECRET-a', 'SECRET-r', NOW(), '{}', FALSE, FALSE, FALSE, NULL);

INSERT INTO overlays (id, user_id, name) VALUES (:mine, :me, 'My Overlay'), (:theirs, :other, 'Their Overlay');
INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name) VALUES (:mine, 'twitch', '1001', 'streamer_x');
INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name, config)
VALUES (:mine, 'discord', '2001', 'general', '{"relay_enabled": true, "relay_channel_id": "2002", "relay_webhook_url": "SECRET-relay-webhook"}');
UPDATE overlay_configs SET custom_css = '.my-css {}' WHERE overlay_id = :mine;
UPDATE overlay_event_settings SET twitch_channel_points_name = 'Bananas' WHERE overlay_id = :mine;
UPDATE credit_roll_configs SET custom_css = '.my-roll {}' WHERE overlay_id = :mine;
INSERT INTO alert_profiles (overlay_id, event_type, text_template) VALUES (:mine, 'follow', 'thanks {user}');
INSERT INTO goal_configs (overlay_id, metric, platform, target, title) VALUES (:mine, 'followers', 'twitch', 100, 'Road to 100');
INSERT INTO list_configs (overlay_id, category, title) VALUES (:mine, 'top_chatters', 'Top chatters');
INSERT INTO overlay_tts_configs (overlay_id, encrypted_api_key, voice_id, tts_signing_secret)
VALUES (:mine, 'SECRET-tts-key'::bytea, 'voice-rachel', 'SECRET-tts-signing'::bytea);
INSERT INTO points_earn_config (overlay_id, points_name) VALUES (:mine, 'Banana Points');
INSERT INTO alert_events (id, overlay_id, platform, event_type, user_id, user_name, event_data, occurred_at)
VALUES (gen_random_uuid(), :mine, 'twitch', 'follow', '555', 'a_follower', '{}', NOW());
INSERT INTO stream_sessions (overlay_id, started_at) VALUES (:mine, NOW() - INTERVAL '2 hours');
WITH p AS (INSERT INTO polls (overlay_id, question) VALUES (:mine, 'Best emote?') RETURNING id)
INSERT INTO poll_options (poll_id, idx, label) SELECT id, 0, 'Kappa' FROM p;
WITH p AS (INSERT INTO predictions (overlay_id, title) VALUES (:mine, 'Will I win?') RETURNING id)
INSERT INTO prediction_outcomes (prediction_id, idx, label) SELECT id, 0, 'Yes' FROM p;

INSERT INTO chat_commands (user_id, kind, trigger, response) VALUES (:me, 'command', '!discord', 'join us');
INSERT INTO chat_timers (user_id, name, message, interval_minutes) VALUES (:me, 'socials', 'follow me', 15);
INSERT INTO chat_bot_settings (user_id, sender) VALUES (:me, 'bot');
INSERT INTO chat_bot_accounts (owner_user_id, platform, platform_user_id, login, display_name,
                               access_token_enc, refresh_token_enc, scopes)
VALUES (:me, 'twitch', '777', 'mybot', 'My Bot', 'SECRET-bot-access', 'SECRET-bot-refresh', '{user:bot}');
INSERT INTO media_objects (user_id, object_key, filename, content_type, size_bytes)
VALUES (:me, :objkey, 'clip #1.mp3', 'audio/mpeg', 1234);
INSERT INTO api_tokens (user_id, name, token_hash, scopes) VALUES (:me, 'Stream Deck', 'SECRET-pat-hash'::bytea, '{chat:write}');
INSERT INTO device_tokens (user_id, overlay_id, name, token_hash, scopes)
VALUES (:me, :mine, 'StreamController', 'SECRET-device-hash'::bytea, '{chat:write}');
INSERT INTO localization_locales (code, english_name, native_name, requested_by) VALUES ('tlh', 'Klingon', 'tlhIngan Hol', :me);
INSERT INTO localization_translations (locale, key, value, submitted_by) VALUES ('tlh', 'nav.home', 'juH', :me);
INSERT INTO premium_subscriptions (user_id, provider, provider_user_id, status, tier_id, cents, product)
VALUES (:me, 'patreon', 'p-1', 'active', 't-1', 500, 'streamer');
INSERT INTO ambassador_showcase (user_id, tagline, featured_consent) VALUES (:me, 'Multistreams to 3 platforms', TRUE);
INSERT INTO moderation_actions (overlay_id, actor_user_id, platform, channel_id, action, outcome, reason)
VALUES (:mine, :mod, 'twitch', '1001', 'timeout', 'success', 'on my overlay'),
       (:theirs, :me, 'twitch', '1002', 'delete', 'success', 'by me elsewhere'),
       (:theirs, :other, 'twitch', '1002', 'ban', 'success', 'not mine at all');
INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, status, expires_at)
VALUES (:me, :mine, :other, 'pending', NOW() + INTERVAL '1 day'),
       (:other, :theirs, :me, 'accepted', NOW() + INTERVAL '1 day');
INSERT INTO impersonation_audit_log (admin_user_id, admin_username, target_user_id, target_username)
VALUES (:other, 'other_streamer', :me, 'streamer_x');
INSERT INTO twitch_oauth_tokens (user_id, twitch_user_id, twitch_login, access_token, refresh_token, token_expires_at, granted_scopes)
VALUES (:me, '1001', 'streamer_x', 'SECRET-tw-access', 'SECRET-tw-refresh', NOW() + INTERVAL '1 hour', '{channel:bot}');
INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry)
VALUES (:me, 'UC-mine', 'SECRET-yt-access', 'SECRET-yt-refresh', NOW() + INTERVAL '1 hour');
INSERT INTO kick_oauth_tokens (user_id, channel_id, access_token, refresh_token, expiry)
VALUES (:me, 'kick-mine', 'SECRET-kick-access', 'SECRET-kick-refresh', NOW() + INTERVAL '1 hour');
INSERT INTO tiktok_oauth_tokens (user_id, open_id, access_token, refresh_token, expiry, scope)
VALUES (:me, 'open-mine', 'SECRET-tt-access', 'SECRET-tt-refresh', NOW() + INTERVAL '1 hour', 'user.info.basic,live.room.info');
INSERT INTO facebook_oauth_tokens (user_id, page_id, page_name, access_token) VALUES (:me, 'page-mine', 'My Page', 'SECRET-fb');
INSERT INTO instagram_oauth_tokens (user_id, ig_user_id, ig_username, access_token) VALUES (:me, 'ig-mine', 'my_ig', 'SECRET-ig');
INSERT INTO patreon_oauth_tokens (user_id, patreon_user_id, access_token, refresh_token, token_expires_at)
VALUES (:me, 'patreon-mine', 'SECRET-patreon-access', 'SECRET-patreon-refresh', NOW() + INTERVAL '1 hour');
INSERT INTO mod_oauth_credentials (user_id, platform, platform_user_id, platform_login, access_token, refresh_token)
VALUES (:me, 'kick', 'kick-mod-id', 'streamer_x', 'SECRET-mod-access', 'SECRET-mod-refresh');
INSERT INTO overlay_moderators (overlay_id, moderator_user_id, granted_by, status, invite_token_hash, accepted_at)
VALUES (:mine, :mod, :me, 'active', 'SECRET-invite'::bytea, NOW()),
       (:theirs, :me, :other, 'active', NULL, NOW());
INSERT INTO discord_identities (user_id, discord_user_id, discord_username) VALUES (:me, 'discord-mine', 'disc_user');
INSERT INTO discord_guilds (user_id, guild_id, guild_name) VALUES (:me, 'guild-mine', 'My Guild');

-- The viewer identity the streamer signed in with on someone else's channel.
INSERT INTO viewers (id) VALUES (:viewer);
INSERT INTO viewer_platform_identities (viewer_id, platform, platform_user_id) VALUES (:viewer, 'twitch', '1001');
INSERT INTO viewer_sessions (id, platform, platform_user_id, username, display_name, access_token, refresh_token,
                             token_expires_at, viewer_id, user_id)
VALUES (:session, 'twitch', '1001', 'streamer_x', 'Streamer X', 'SECRET-viewer-access', 'SECRET-viewer-refresh',
        NOW() + INTERVAL '1 hour', :viewer, :me);
INSERT INTO viewer_cosmetics (viewer_id, name_color) VALUES (:viewer, '#ff0000');
INSERT INTO viewer_points (viewer_id, overlay_id, balance) VALUES (:viewer, :theirs, 42);
INSERT INTO points_transactions (viewer_id, overlay_id, delta, reason, dedup_key) VALUES (:viewer, :theirs, 42, 'chat', 'd-1');
WITH p AS (INSERT INTO polls (overlay_id, question) VALUES (:theirs, 'Pizza or pasta?') RETURNING id),
     o AS (INSERT INTO poll_options (poll_id, idx, label) SELECT id, 0, 'Pizza' FROM p RETURNING id, poll_id)
INSERT INTO poll_votes (poll_id, viewer_id, option_id) SELECT poll_id, :viewer, id FROM o;
WITH p AS (INSERT INTO predictions (overlay_id, title) VALUES (:theirs, 'Will they win?') RETURNING id),
     o AS (INSERT INTO prediction_outcomes (prediction_id, idx, label) SELECT id, 0, 'No' FROM p RETURNING id, prediction_id)
INSERT INTO prediction_entries (prediction_id, viewer_id, outcome_id, amount) SELECT prediction_id, :viewer, id, 10 FROM o;
INSERT INTO viewer_message_history (viewer_session_id, streamer_user_id, platform, channel_id, channel_name, message_text)
VALUES (:session, :other, 'twitch', '1002', 'other_streamer', 'hello from me');
INSERT INTO premium_subscriptions (viewer_id, provider, provider_user_id, status, cents, product)
VALUES (:viewer, 'patreon', 'p-viewer', 'active', 300, 'viewer');
INSERT INTO patreon_oauth_tokens (viewer_id, patreon_user_id, access_token, refresh_token, token_expires_at)
VALUES (:viewer, 'patreon-viewer', 'SECRET-viewer-patreon-access', 'SECRET-viewer-patreon-refresh', NOW() + INTERVAL '1 hour');
`)
	if _, err := pool.Exec(context.Background(), sql); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestHandleDataExport_CoversEverythingStoredAboutTheStreamer(t *testing.T) {
	pool := exportTestDB(t)
	seedExportFixture(t, pool)
	t.Setenv("MEDIA_PUBLIC_URL", "https://media.example.com/allchat-media/")

	h := &AuthHandler{userRepo: repository.NewUserRepository(pool, nil), logger: zap.NewNop()}
	body := serveExport(t, "/me/data-export", h.HandleDataExport, map[string]any{"user_id": exStreamer})

	if strings.Contains(body, "SECRET") {
		t.Fatalf("the export contains a credential or third-party payload:\n%s", secretContext(body))
	}

	var export map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &export); err != nil {
		t.Fatalf("export is not a JSON object: %v", err)
	}
	if v := string(export["format_version"]); v != "2" {
		t.Errorf("format_version = %s, want 2", v)
	}

	user := object(t, export, "user")
	for field, want := range map[string]any{
		"username": "streamer_x", "is_beta_tester": true, "is_ambassador": true, "is_admin": false,
		"is_banned": true, "banned_reason": "testing the ban reason",
	} {
		if user[field] != want {
			t.Errorf("user.%s = %v, want %v", field, user[field], want)
		}
	}

	overlays := rows(t, export, "overlays")
	if len(overlays) != 1 || overlays[0]["name"] != "My Overlay" {
		t.Fatalf("overlays must list exactly the streamer's own overlay, got %v", overlays)
	}
	overlay := overlays[0]
	for path, want := range map[string]any{
		"config.custom_css":                         ".my-css {}",
		"event_settings.twitch_channel_points_name": "Bananas",
		"credit_roll.custom_css":                    ".my-roll {}",
		"goal.title":                                "Road to 100",
		"list.title":                                "Top chatters",
		"tts.voice_id":                              "voice-rachel",
		"points_config.points_name":                 "Banana Points",
	} {
		parts := strings.SplitN(path, ".", 2)
		section, _ := overlay[parts[0]].(map[string]any)
		if section[parts[1]] != want {
			t.Errorf("overlays[0].%s = %v, want %v", path, section[parts[1]], want)
		}
	}
	if !hasRow(asRows(overlay["alert_profiles"]), "text_template", "thanks {user}") {
		t.Errorf("overlays[0].alert_profiles missing the follow alert: %v", overlay["alert_profiles"])
	}
	if !hasRow(asRows(overlay["sources"]), "channel_name", "streamer_x") {
		t.Errorf("overlays[0].sources missing the twitch source: %v", overlay["sources"])
	}
	// The Discord source is exported with its relay settings, but not the relay webhook URL,
	// a credential (ADR-0091); the SECRET check above would catch it.
	if !hasRow(asRows(overlay["sources"]), "channel_name", "general") {
		t.Errorf("overlays[0].sources missing the discord source: %v", overlay["sources"])
	}

	for _, want := range []struct {
		section, field string
		value          any
	}{
		{"alert_events", "user_name", "a_follower"},
		{"stream_sessions", "state", "ACTIVE"},
		{"polls", "question", "Best emote?"},
		{"predictions", "title", "Will I win?"},
		{"chat_commands", "trigger", "!discord"},
		{"chat_timers", "name", "socials"},
		{"chat_bot_settings", "sender", "bot"},
		{"chat_bot_accounts", "login", "mybot"},
		{"media", "url", "https://media.example.com/allchat-media/" + exStreamer + "/dddddddd-1111-1111-1111-111111111111/clip%20%231.mp3"},
		{"api_tokens", "name", "Stream Deck"},
		{"devices", "name", "StreamController"},
		{"translations", "value", "juH"},
		{"requested_locales", "code", "tlh"},
		{"premium_subscriptions", "status", "active"},
		{"ambassador_showcase", "tagline", "Multistreams to 3 platforms"},
		{"moderation_actions", "reason", "on my overlay"},
		{"moderation_actions", "reason", "by me elsewhere"},
		{"share_requests", "direction", "sent"},
		{"share_requests", "direction", "received"},
		{"impersonations", "admin_username", "other_streamer"},
		{"moderation_delegations", "direction", "granted"},
		{"moderation_delegations", "direction", "received"},
		{"discord_accounts", "discord_username", "disc_user"},
		{"discord_guilds", "guild_name", "My Guild"},
		{"viewer_sessions", "username", "streamer_x"},
		{"messages", "message_text", "hello from me"},
	} {
		if !hasRow(rows(t, export, want.section), want.field, want.value) {
			t.Errorf("%s has no row with %s = %v: %s", want.section, want.field, want.value, export[want.section])
		}
	}
	if hasRow(rows(t, export, "moderation_actions"), "reason", "not mine at all") {
		t.Error("moderation_actions leaked an action by someone else on someone else's overlay")
	}

	var linked []string
	for _, r := range rows(t, export, "linked_accounts") {
		linked = append(linked, r["platform"].(string)+"/"+r["purpose"].(string))
		// The section mixes naive (UTC) and zoned columns; both must come out as UTC instants.
		if created, _ := r["created_at"].(string); !strings.HasSuffix(created, "+00:00") {
			t.Errorf("linked_accounts %s/%s created_at = %q, want a UTC timestamp", r["platform"], r["purpose"], created)
		}
	}
	sort.Strings(linked)
	want := []string{"facebook/linked", "instagram/linked", "kick/linked", "kick/moderation",
		"patreon/premium", "tiktok/linked", "twitch/linked", "twitch/sign_in", "youtube/linked"}
	if strings.Join(linked, ",") != strings.Join(want, ",") {
		t.Errorf("linked_accounts = %v, want %v", linked, want)
	}

	viewers := rows(t, export, "viewer_accounts")
	if len(viewers) != 1 {
		t.Fatalf("the streamer's own viewer identity must be exported, got %v", viewers)
	}
	assertViewerSections(t, viewers[0])
}

// A viewer who never created a streamer account has their own export, keyed on the viewer
// identity in their viewer session token.
func TestHandleViewerDataExport_CoversEverythingStoredAboutTheViewer(t *testing.T) {
	pool := exportTestDB(t)
	seedExportFixture(t, pool)

	h := &ViewerAuthHandler{userRepo: repository.NewUserRepository(pool, nil), logger: zap.NewNop()}
	body := serveExport(t, "/viewer/data-export", h.HandleViewerDataExport,
		map[string]any{"viewer_id": exViewer, "session_id": exSession, "is_viewer": true})

	if strings.Contains(body, "SECRET") {
		t.Fatalf("the viewer export contains a credential:\n%s", secretContext(body))
	}
	var export map[string]any
	if err := json.Unmarshal([]byte(body), &export); err != nil {
		t.Fatalf("export is not a JSON object: %v", err)
	}
	if export["format_version"] != float64(2) {
		t.Errorf("format_version = %v, want 2", export["format_version"])
	}
	assertViewerSections(t, export)
}

func assertViewerSections(t *testing.T, viewer map[string]any) {
	t.Helper()
	profile, _ := viewer["profile"].(map[string]any)
	if profile["id"] != exViewer {
		t.Errorf("profile = %v, want viewer %s", viewer["profile"], exViewer)
	}
	if _, ok := profile["tos_accepted"]; !ok {
		t.Errorf("profile = %v, want the viewer's terms acceptance", profile)
	}
	if cosmetics, _ := viewer["cosmetics"].(map[string]any); cosmetics["name_color"] != "#ff0000" {
		t.Errorf("cosmetics = %v", viewer["cosmetics"])
	}
	for _, want := range []struct {
		section, field string
		value          any
	}{
		{"platform_identities", "platform_user_id", "1001"},
		{"sessions", "username", "streamer_x"},
		{"points", "balance", float64(42)},
		{"points_transactions", "reason", "chat"},
		{"poll_votes", "option", "Pizza"},
		{"prediction_entries", "outcome", "No"},
		{"messages", "message_text", "hello from me"},
		{"premium_subscriptions", "provider_user_id", "p-viewer"},
		{"linked_patreon_accounts", "patreon_user_id", "patreon-viewer"},
	} {
		if !hasRow(asRows(viewer[want.section]), want.field, want.value) {
			t.Errorf("viewer %s has no row with %s = %v: %v", want.section, want.field, want.value, viewer[want.section])
		}
	}
}

// A user with nothing but an account still gets every section, as an empty list, so the file
// states "you have none of these" instead of leaving the reader to guess.
func TestHandleDataExport_EmptySectionsAreEmptyLists(t *testing.T) {
	pool := exportTestDB(t)
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO users (id, twitch_id, username, display_name, auth_provider, access_token, refresh_token, token_expires_at)
		VALUES ('`+exStreamer+`', '1001', 'lonely', 'Lonely', 'twitch', 'a', 'r', NOW())`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	h := &AuthHandler{userRepo: repository.NewUserRepository(pool, nil), logger: zap.NewNop()}
	body := serveExport(t, "/me/data-export", h.HandleDataExport, map[string]any{"user_id": exStreamer})

	var export map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &export); err != nil {
		t.Fatalf("export is not a JSON object: %v", err)
	}
	for _, section := range []string{"overlays", "alert_events", "chat_commands", "media", "viewer_accounts", "moderation_delegations"} {
		if string(export[section]) != "[]" {
			t.Errorf("%s = %s, want []", section, export[section])
		}
	}
	// Only the sign-in grant: it lives on the users row itself.
	if got := rows(t, export, "linked_accounts"); len(got) != 1 || got[0]["purpose"] != "sign_in" {
		t.Errorf("linked_accounts = %v, want just the sign-in grant", got)
	}
}

// A delegated-moderation grant is personal data about both parties (ADR-0048), and it is not
// derivable from any other section: the streamer's overlays say nothing about who may moderate
// them, and a moderator's own overlays say nothing about channels delegated to them. A revoked
// grant is retained as history, so an export that hid it would misreport what All-Chat holds.
func TestHandleDataExport_DelegationsBothDirectionsIncludingRevoked(t *testing.T) {
	pool := exportTestDB(t)
	seedExportFixture(t, pool)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE overlay_moderators SET status = 'revoked', revoked_at = NOW() WHERE overlay_id = $1`, exMine); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	h := &AuthHandler{userRepo: repository.NewUserRepository(pool, nil), logger: zap.NewNop()}
	var export map[string]json.RawMessage
	if err := json.Unmarshal([]byte(serveExport(t, "/me/data-export", h.HandleDataExport, map[string]any{"user_id": exStreamer})), &export); err != nil {
		t.Fatal(err)
	}
	byDirection := map[string]map[string]any{}
	for _, d := range rows(t, export, "moderation_delegations") {
		byDirection[d["direction"].(string)] = d
	}
	granted, received := byDirection["granted"], byDirection["received"]
	if granted["overlay_name"] != "My Overlay" || granted["counterpart"] != "Sarah" || granted["revoked_at"] == nil {
		t.Errorf("granted grant misreported (must name the moderator and carry its revocation): %v", granted)
	}
	if actions, _ := granted["actions"].([]any); len(actions) != 2 {
		t.Errorf("the delegated actions are part of the record: %v", granted["actions"])
	}
	if received["overlay_name"] != "Their Overlay" || received["counterpart"] != "Other Streamer" {
		t.Errorf("received grant misreported: %v", received)
	}
}

// Erasure needs no delegation-specific code: overlay_moderators cascades from BOTH overlays and
// users, so deleting the account takes every row on either side with it. If that ever stops being
// true, a deleted volunteer would keep a live grant on someone's channel.
func TestDeletingAnAccountCascadesItsDelegations(t *testing.T) {
	pool := exportTestDB(t)
	seedExportFixture(t, pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, exMod); err != nil {
		t.Fatalf("delete moderator: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM overlay_moderators WHERE overlay_id = $1`, exMine).Scan(&remaining); err != nil {
		t.Fatalf("count: %v", err)
	}
	if remaining != 0 {
		t.Errorf("deleting the moderator's account must remove their grant, %d left", remaining)
	}
}

// Settings tables are exported whole-row so a new setting reaches the export without anyone
// remembering to list it. That is only safe while they hold no credentials: if one gains a token,
// key, hash or secret column, list its columns explicitly in the export query instead.
func TestWholeRowExportTablesHoldNoCredentials(t *testing.T) {
	pool := exportTestDB(t)
	credential := regexp.MustCompile(`token$|^token|secret|password|hash|api_key|_enc$|encrypted`)
	for _, table := range wholeRowExportTables {
		cols, err := pool.Query(context.Background(),
			`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = $1`, table)
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		n := 0
		for cols.Next() {
			var name, dataType string
			if err := cols.Scan(&name, &dataType); err != nil {
				t.Fatal(err)
			}
			n++
			if credential.MatchString(name) || dataType == "bytea" {
				t.Errorf("%s.%s looks like a credential but %s is exported whole-row", table, name, table)
			}
		}
		cols.Close()
		if n == 0 {
			t.Errorf("%s does not exist; drop it from wholeRowExportTables", table)
		}
	}
}

// A viewer export is keyed on the viewer identity in a viewer token. A streamer session carries
// none, so it must be refused before any read (userRepo is nil: reaching the DB would panic).
func TestHandleViewerDataExport_RefusesNonViewerSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h := &ViewerAuthHandler{logger: zap.NewNop()}
	router.GET("/viewer/data-export", injectCtx(map[string]string{"user_id": testUserID}), h.HandleViewerDataExport)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/viewer/data-export", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
}

func serveExport(t *testing.T, path string, handler gin.HandlerFunc, ctxValues map[string]any) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET(path, func(c *gin.Context) {
		for k, v := range ctxValues {
			c.Set(k, v)
		}
	}, handler)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", path, w.Code, w.Body.String())
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment;") {
		t.Errorf("Content-Disposition = %q, want an attachment", cd)
	}
	return w.Body.String()
}

func object(t *testing.T, export map[string]json.RawMessage, key string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(export[key], &out); err != nil || out == nil {
		t.Fatalf("%s is not an object: %s", key, export[key])
	}
	return out
}

func rows(t *testing.T, export map[string]json.RawMessage, key string) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(export[key], &out); err != nil || out == nil {
		t.Fatalf("%s is not a list: %s", key, export[key])
	}
	return out
}

func asRows(v any) []map[string]any {
	list, _ := v.([]any)
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func hasRow(rows []map[string]any, field string, want any) bool {
	for _, r := range rows {
		if r[field] == want {
			return true
		}
	}
	return false
}

func secretContext(body string) string {
	i := strings.Index(body, "SECRET")
	return body[max(0, i-200):min(len(body), i+60)]
}

// exportPostgres is the one testcontainer every export test creates its database in. It is
// started on first use and removed by the testcontainers reaper when the test binary exits.
var exportPostgres struct {
	once sync.Once
	dsn  string
	err  error
}

// exportTestDB returns a pool on a fresh database carrying the full production schema: every
// up-migration in order, the selection scripts/run-migrations.sh makes. The database is created
// for this test and dropped after it, on a shared testcontainer, or on the server
// TEST_DATABASE_URL names so the tests also run where Docker does not (any existing database
// on that server is left alone).
func exportTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		exportPostgres.once.Do(func() {
			container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Image:        "postgres:16-alpine",
					ExposedPorts: []string{"5432/tcp"},
					Env:          map[string]string{"POSTGRES_USER": "testuser", "POSTGRES_PASSWORD": "testpass", "POSTGRES_DB": "testdb"},
					WaitingFor: wait.ForLog("database system is ready to accept connections").
						WithOccurrence(2).WithStartupTimeout(60 * time.Second),
				},
				Started: true,
			})
			if err != nil {
				exportPostgres.err = err
				return
			}
			endpoint, err := container.PortEndpoint(ctx, "5432/tcp", "")
			exportPostgres.dsn, exportPostgres.err = "postgres://testuser:testpass@"+endpoint+"/testdb?sslmode=disable", err
		})
		if exportPostgres.err != nil {
			t.Skipf("cannot start postgres testcontainer (docker unavailable? set TEST_DATABASE_URL): %v", exportPostgres.err)
		}
		dsn = exportPostgres.dsn
	}

	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	name := "export_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	// A server zone other than UTC, so an export that renders timestamps in the session's zone
	// shows up in the assertions instead of hiding behind a UTC default.
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	if _, err := admin.Exec(ctx, "ALTER DATABASE "+name+" SET timezone TO 'America/New_York'"); err != nil {
		t.Fatalf("set database timezone: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)")
		_ = admin.Close(ctx)
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	t.Cleanup(pool.Close)

	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "[0-9]*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	sort.Strings(files)
	for _, f := range files {
		if strings.HasSuffix(f, "_down.sql") {
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(f), err)
		}
	}
	return pool
}
