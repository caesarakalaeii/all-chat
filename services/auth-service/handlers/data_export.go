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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// The DSGVO Art. 15/20 data export: everything All-Chat stores about a streamer account
// (GET /me/data-export) or a viewer identity (GET /viewer/data-export), as one JSON file.
//
// Each section is a query whose rows become a JSON array, so the export's shape is the column
// lists below. Credentials never leave the database: tokens, token hashes, API keys and signing
// secrets are simply not selected, and every table that holds one is listed column by column.
// Settings tables that hold none are exported whole-row (wholeRowExportTables), so a new setting
// reaches the export without anyone remembering to add it.
//
// A section that fails fails the whole export: a file that silently omits a section would tell
// the person we hold less than we do.

// exportFormatVersion changes whenever a section is added, removed or reshaped. Version 1 was the
// original profile, overlays, viewer sessions, Discord, messages and delegations file.
const exportFormatVersion = 2

// eventLogLimit caps each append-only log (alert events, moderation actions, point transactions,
// votes, messages) at its most recent rows, so one long-running channel cannot produce a file the
// 15 s request deadline cannot deliver.
const eventLogLimit = "10000"

// defaultMediaPublicURL is production's MEDIA_PUBLIC_URL (ADR-0071): MinIO serves path-style, so
// the base includes the bucket.
const defaultMediaPublicURL = "https://media.allch.at/allchat-media"

// wholeRowExportTables are exported with every column. TestWholeRowExportTablesHoldNoCredentials
// fails if one of them gains a credential column; list that table's columns explicitly instead.
var wholeRowExportTables = []string{
	"overlay_configs", "overlay_event_settings", "alert_profiles", "goal_configs", "list_configs",
	"credit_roll_configs", "points_earn_config", "stream_sessions",
	"chat_commands", "chat_timers", "chat_bot_settings",
}

// exportSection is one named part of an export. $1 is the subject: a user id for the streamer
// export, a viewer id for the viewer export.
type exportSection struct {
	key   string
	query string
	// one marks a section with at most one row, exported as an object (or null) instead of a list.
	one bool
}

const premiumSubscriptionColumns = `provider, provider_user_id, product, status, tier_id, cents,
	current_period_end, created_at, updated_at`

const viewerSessionColumns = `platform, platform_user_id, username, display_name, avatar_url,
	created_at, updated_at, last_message_at, is_banned, banned_at, banned_reason`

var streamerExportSections = []exportSection{
	{key: "user", one: true, query: `
		SELECT id, username, display_name, profile_image_url, auth_provider, twitch_id, google_id, kick_id,
		       is_premium, premium_admin_override, premium_admin_override_expires_at,
		       is_beta_tester, is_ambassador, is_admin, is_banned, banned_at, banned_reason,
		       onboarding_completed_at, tos_accepted, tos_accepted_at, created_at, updated_at
		FROM users WHERE id = $1`},

	// Which platform accounts are connected and what they may do; never the tokens themselves.
	{key: "linked_accounts", query: `
		SELECT auth_provider AS platform, 'sign_in' AS purpose,
		       COALESCE(twitch_id, google_id, kick_id) AS account_id, username AS account_name,
		       granted_scopes AS scopes, token_expires_at AS expires_at, created_at, updated_at
		FROM users WHERE id = $1
		UNION ALL SELECT 'twitch', 'linked', twitch_user_id, twitch_login, granted_scopes, token_expires_at, created_at, updated_at
		FROM twitch_oauth_tokens WHERE user_id = $1
		UNION ALL SELECT 'youtube', 'linked', channel_id, NULL, granted_scopes, expiry, created_at, updated_at
		FROM youtube_oauth_tokens WHERE user_id = $1
		UNION ALL SELECT 'kick', 'linked', COALESCE(kick_user_id, channel_id), channel_id, granted_scopes, expiry, created_at, updated_at
		FROM kick_oauth_tokens WHERE user_id = $1
		UNION ALL SELECT 'tiktok', 'linked', open_id, NULL, string_to_array(scope, ','), expiry, created_at, updated_at
		FROM tiktok_oauth_tokens WHERE user_id = $1
		UNION ALL SELECT 'facebook', 'linked', page_id, page_name, granted_scopes, NULL, created_at, updated_at
		FROM facebook_oauth_tokens WHERE user_id = $1
		UNION ALL SELECT 'instagram', 'linked', ig_user_id, ig_username, granted_scopes, expires_at, created_at, updated_at
		FROM instagram_oauth_tokens WHERE user_id = $1
		UNION ALL SELECT 'patreon', 'premium', patreon_user_id, NULL, granted_scopes, token_expires_at, created_at, updated_at
		FROM patreon_oauth_tokens WHERE user_id = $1
		UNION ALL SELECT platform, 'moderation', platform_user_id, platform_login, granted_scopes, token_expires_at, created_at, updated_at
		FROM mod_oauth_credentials WHERE user_id = $1
		ORDER BY 1, 2`},

	// Each overlay with its sources and every per-overlay setting. The TTS config is listed
	// column by column: the row also holds the ElevenLabs API key and the signing secret. A
	// source's config holds the Discord relay webhook URL, a credential (ADR-0091).
	{key: "overlays", query: `
		SELECT o.id, o.name, o.description, o.overlay_type, o.is_active, o.is_public_for_viewers,
		       o.created_at, o.updated_at, o.last_connected_at,
		       (SELECT COALESCE(json_agg(s ORDER BY s.platform, s.channel_name), '[]'::json) FROM (
		            SELECT platform, channel_id, channel_name, channel_handle, is_active,
		                   config - 'relay_webhook_url' AS config, created_at
		            FROM overlay_chat_sources WHERE overlay_id = o.id) s) AS sources,
		       (SELECT to_json(x) FROM overlay_configs x WHERE x.overlay_id = o.id) AS config,
		       (SELECT to_json(x) FROM overlay_event_settings x WHERE x.overlay_id = o.id) AS event_settings,
		       (SELECT COALESCE(json_agg(x ORDER BY x.event_type), '[]'::json)
		          FROM alert_profiles x WHERE x.overlay_id = o.id) AS alert_profiles,
		       (SELECT to_json(x) FROM goal_configs x WHERE x.overlay_id = o.id) AS goal,
		       (SELECT to_json(x) FROM list_configs x WHERE x.overlay_id = o.id) AS list,
		       (SELECT to_json(x) FROM credit_roll_configs x WHERE x.overlay_id = o.id) AS credit_roll,
		       (SELECT to_json(x) FROM points_earn_config x WHERE x.overlay_id = o.id) AS points_config,
		       (SELECT json_build_object('voice_id', x.voice_id, 'created_at', x.created_at, 'updated_at', x.updated_at)
		          FROM overlay_tts_configs x WHERE x.overlay_id = o.id) AS tts
		FROM overlays o WHERE o.user_id = $1 ORDER BY o.created_at`},

	{key: "alert_events", query: `
		SELECT e.overlay_id, e.platform, e.event_type, e.user_id AS platform_user_id, e.user_name,
		       e.event_data, e.occurred_at
		FROM alert_events e JOIN overlays o ON o.id = e.overlay_id
		WHERE o.user_id = $1 ORDER BY e.occurred_at DESC LIMIT ` + eventLogLimit},
	{key: "stream_sessions", query: `
		SELECT s.* FROM stream_sessions s JOIN overlays o ON o.id = s.overlay_id
		WHERE o.user_id = $1 ORDER BY s.started_at DESC`},
	{key: "polls", query: `
		SELECT p.overlay_id, p.source, p.question, p.state, p.allow_change, p.created_at, p.ends_at, p.closed_at,
		       (SELECT json_agg(po.label ORDER BY po.idx) FROM poll_options po WHERE po.poll_id = p.id) AS options
		FROM polls p JOIN overlays o ON o.id = p.overlay_id
		WHERE o.user_id = $1 ORDER BY p.created_at DESC`},
	{key: "predictions", query: `
		SELECT p.overlay_id, p.source, p.title, p.state, p.created_at, p.locked_at, p.resolved_at,
		       (SELECT json_agg(po.label ORDER BY po.idx) FROM prediction_outcomes po WHERE po.prediction_id = p.id) AS outcomes,
		       (SELECT po.label FROM prediction_outcomes po WHERE po.id = p.winning_outcome_id) AS winning_outcome
		FROM predictions p JOIN overlays o ON o.id = p.overlay_id
		WHERE o.user_id = $1 ORDER BY p.created_at DESC`},

	{key: "chat_commands", query: `SELECT * FROM chat_commands WHERE user_id = $1 ORDER BY trigger`},
	{key: "chat_timers", query: `SELECT * FROM chat_timers WHERE user_id = $1 ORDER BY name`},
	{key: "chat_bot_settings", query: `SELECT * FROM chat_bot_settings WHERE user_id = $1`},
	{key: "chat_bot_accounts", query: `
		SELECT platform, login, display_name, created_at, updated_at
		FROM chat_bot_accounts WHERE owner_user_id = $1 ORDER BY platform`},

	{key: "api_tokens", query: `
		SELECT name, scopes, created_at, last_used_at, expires_at, revoked_at
		FROM api_tokens WHERE user_id = $1 ORDER BY created_at`},
	{key: "devices", query: `
		SELECT name, overlay_id, scopes, created_at, last_used_at, expires_at, revoked_at
		FROM device_tokens WHERE user_id = $1 ORDER BY created_at`},

	{key: "translations", query: `
		SELECT locale, key, value, status, review_note, updated_at
		FROM localization_translations WHERE submitted_by = $1 ORDER BY locale, key`},
	{key: "requested_locales", query: `
		SELECT code, english_name, native_name, status, created_at, updated_at
		FROM localization_locales WHERE requested_by = $1 ORDER BY code`},

	{key: "premium_subscriptions", query: `
		SELECT ` + premiumSubscriptionColumns + ` FROM premium_subscriptions WHERE user_id = $1 ORDER BY created_at`},
	{key: "ambassador_showcase", query: `
		SELECT tagline, sort_order, featured_consent, created_at, updated_at
		FROM ambassador_showcase WHERE user_id = $1`},

	// Actions the user took anywhere, and every action taken on their overlays.
	{key: "moderation_actions", query: `
		SELECT m.overlay_id, m.platform, m.channel_id, m.action, m.target_user_id, m.target_message_id,
		       m.reason, m.outcome, m.actor_role, m.actor_user_id = $1 AS performed_by_you,
		       m.impersonated_by IS NOT NULL AS during_admin_impersonation, m.created_at
		FROM moderation_actions m
		WHERE m.actor_user_id = $1 OR m.on_behalf_of_user_id = $1
		   OR m.overlay_id IN (SELECT id FROM overlays WHERE user_id = $1)
		ORDER BY m.created_at DESC LIMIT ` + eventLogLimit},

	// Delegated-moderation grants in both directions (ADR-0048). Revoked grants are retained as
	// history, so they are exported too.
	{key: "moderation_delegations", query: `
		SELECT 'granted' AS direction, m.overlay_id, o.name AS overlay_name, m.status, m.actions,
		       -- The name captured at accept time, else whoever the account says it is now, else
		       -- the label the streamer typed on an invite nobody has redeemed.
		       COALESCE(NULLIF(m.moderator_display_name, ''), NULLIF(mod_user.display_name, ''),
		                m.invitee_label, '') AS counterpart,
		       m.created_at, m.accepted_at, m.revoked_at
		FROM overlay_moderators m
		JOIN overlays o ON o.id = m.overlay_id
		LEFT JOIN users mod_user ON mod_user.id = m.moderator_user_id
		WHERE o.user_id = $1
		UNION ALL
		SELECT 'received', m.overlay_id, o.name, m.status, m.actions, COALESCE(owner.display_name, ''),
		       m.created_at, m.accepted_at, m.revoked_at
		FROM overlay_moderators m
		JOIN overlays o ON o.id = m.overlay_id
		JOIN users owner ON owner.id = o.user_id
		WHERE m.moderator_user_id = $1
		ORDER BY 7`},

	{key: "share_requests", query: `
		SELECT CASE WHEN r.sender_user_id = $1 THEN 'sent' ELSE 'received' END AS direction,
		       COALESCE(u.display_name, '') AS counterpart, r.sender_overlay_id, r.recipient_overlay_id,
		       r.status, r.expiry_option, r.created_at, r.responded_at, r.expires_at, r.share_expires_at
		FROM share_requests r
		LEFT JOIN users u ON u.id = CASE WHEN r.sender_user_id = $1 THEN r.recipient_user_id ELSE r.sender_user_id END
		WHERE r.sender_user_id = $1 OR r.recipient_user_id = $1
		ORDER BY r.created_at`},
	{key: "impersonations", query: `
		SELECT admin_username, created_at FROM impersonation_audit_log
		WHERE target_user_id = $1 ORDER BY created_at`},

	{key: "discord_accounts", query: `
		SELECT discord_user_id, discord_username, linked_at, updated_at FROM discord_identities WHERE user_id = $1`},
	{key: "discord_guilds", query: `
		SELECT guild_id, guild_name, guild_icon, connected_at FROM discord_guilds
		WHERE user_id = $1 ORDER BY connected_at`},

	// Viewer sessions this account signed in with. The viewer identities behind them, with their
	// points, votes and cosmetics, are in viewer_accounts.
	{key: "viewer_sessions", query: `
		SELECT ` + viewerSessionColumns + ` FROM viewer_sessions WHERE user_id = $1 ORDER BY created_at`},
	// Messages sent through All-Chat by this account, and by viewers in this account's channels.
	// Kept for one hour (migration 046).
	{key: "messages", query: `
		SELECT vmh.platform, vmh.channel_name, vmh.message_text, vmh.sent_at, vmh.success,
		       vs.user_id IS NOT DISTINCT FROM $1 AS sent_by_you
		FROM viewer_message_history vmh
		JOIN viewer_sessions vs ON vs.id = vmh.viewer_session_id
		WHERE vmh.streamer_user_id = $1 OR vs.user_id = $1
		ORDER BY vmh.sent_at DESC LIMIT ` + eventLogLimit},
}

var viewerExportSections = []exportSection{
	{key: "profile", one: true, query: `
		SELECT id, is_premium, premium_admin_override, premium_admin_override_expires_at,
		       tos_accepted, tos_accepted_at, created_at
		FROM viewers WHERE id = $1`},
	{key: "platform_identities", query: `
		SELECT platform, platform_user_id, created_at FROM viewer_platform_identities
		WHERE viewer_id = $1 ORDER BY created_at`},
	{key: "sessions", query: `
		SELECT ` + viewerSessionColumns + ` FROM viewer_sessions WHERE viewer_id = $1 ORDER BY created_at`},
	{key: "cosmetics", one: true, query: `
		SELECT c.name_color, c.name_gradient, fr.name AS avatar_frame, fl.name AS avatar_flair, c.updated_at
		FROM viewer_cosmetics c
		LEFT JOIN cosmetic_frames fr ON fr.id = c.avatar_frame_id
		LEFT JOIN cosmetic_flairs fl ON fl.id = c.avatar_flair_id
		WHERE c.viewer_id = $1`},
	{key: "points", query: `
		SELECT p.overlay_id, u.username AS streamer, p.balance, p.updated_at
		FROM viewer_points p JOIN overlays o ON o.id = p.overlay_id JOIN users u ON u.id = o.user_id
		WHERE p.viewer_id = $1 ORDER BY u.username`},
	{key: "points_transactions", query: `
		SELECT overlay_id, delta, reason, ref_type, created_at FROM points_transactions
		WHERE viewer_id = $1 ORDER BY created_at DESC LIMIT ` + eventLogLimit},
	{key: "poll_votes", query: `
		SELECT p.overlay_id, p.question, po.label AS option, v.platform, v.created_at
		FROM poll_votes v JOIN polls p ON p.id = v.poll_id JOIN poll_options po ON po.id = v.option_id
		WHERE v.viewer_id = $1 ORDER BY v.created_at DESC LIMIT ` + eventLogLimit},
	{key: "prediction_entries", query: `
		SELECT p.overlay_id, p.title, po.label AS outcome, e.amount, e.platform, e.created_at
		FROM prediction_entries e
		JOIN predictions p ON p.id = e.prediction_id JOIN prediction_outcomes po ON po.id = e.outcome_id
		WHERE e.viewer_id = $1 ORDER BY e.created_at DESC LIMIT ` + eventLogLimit},
	// Kept for one hour (migration 046).
	{key: "messages", query: `
		SELECT m.platform, m.channel_name, m.message_text, m.sent_at, m.success
		FROM viewer_message_history m JOIN viewer_sessions s ON s.id = m.viewer_session_id
		WHERE s.viewer_id = $1 ORDER BY m.sent_at DESC LIMIT ` + eventLogLimit},
	{key: "premium_subscriptions", query: `
		SELECT ` + premiumSubscriptionColumns + ` FROM premium_subscriptions WHERE viewer_id = $1 ORDER BY created_at`},
	{key: "linked_patreon_accounts", query: `
		SELECT patreon_user_id, granted_scopes, token_expires_at, created_at, updated_at
		FROM patreon_oauth_tokens WHERE viewer_id = $1`},
}

// HandleDataExport returns everything stored about the authenticated streamer, including the
// viewer identities they signed in with. DSGVO Art. 15 and 20.
func (h *AuthHandler) HandleDataExport(c *gin.Context) {
	uid := c.GetString("user_id")
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// The export is a full PII dump. A personal access token's scopes cover chat and engagement
	// writes, never bulk disclosure of the account, so this surface is session-only.
	if !RefuseAPIToken(c, "Exporting your data requires a signed-in session, not a personal access token.") {
		return
	}

	ctx := c.Request.Context()
	export := newExport()
	err := readExport(ctx, h.userRepo.DB(), func(tx pgx.Tx) error {
		if err := exportSections(ctx, tx, streamerExportSections, uid, export); err != nil {
			return err
		}
		var err error
		if export["media"], err = exportMedia(ctx, tx, uid); err != nil {
			return err
		}
		export["viewer_accounts"], err = exportViewerAccounts(ctx, tx, uid)
		return err
	})
	if err != nil {
		h.logger.Error("Data export failed", zap.String("user_id", uid), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not build your data export. Please try again."})
		return
	}
	if export["user"].(json.RawMessage) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	serveExportFile(c, export)
}

// HandleViewerDataExport returns everything stored about the authenticated viewer identity: the
// same viewer sections a streamer's export lists under viewer_accounts. DSGVO Art. 15 and 20.
func (h *ViewerAuthHandler) HandleViewerDataExport(c *gin.Context) {
	viewerID := c.GetString("viewer_id")
	if viewerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	if _, err := uuid.Parse(viewerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid viewer ID"})
		return
	}

	ctx := c.Request.Context()
	export := newExport()
	if err := readExport(ctx, h.userRepo.DB(), func(tx pgx.Tx) error {
		return exportSections(ctx, tx, viewerExportSections, viewerID, export)
	}); err != nil {
		h.logger.Error("Viewer data export failed", zap.String("viewer_id", viewerID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not build your data export. Please try again."})
		return
	}
	if export["profile"].(json.RawMessage) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Viewer not found"})
		return
	}
	serveExportFile(c, export)
}

func newExport() map[string]any {
	return map[string]any{
		"format_version": exportFormatVersion,
		"exported_at":    time.Now().UTC().Format(time.RFC3339),
	}
}

func serveExportFile(c *gin.Context, export map[string]any) {
	c.Header("Content-Disposition", "attachment; filename=allchat-data-export.json")
	c.JSON(http.StatusOK, export)
}

// readExport runs read in one read-only snapshot, so every section describes the same moment and
// every timestamp is rendered in UTC (the naive TIMESTAMP columns are written as UTC).
func readExport(ctx context.Context, db *pgxpool.Pool, read func(pgx.Tx) error) error {
	return pgx.BeginTxFunc(ctx, db, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly},
		func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SET LOCAL TIME ZONE 'UTC'`); err != nil {
				return fmt.Errorf("export snapshot: %w", err)
			}
			return read(tx)
		})
}

// exportSections runs each section's query for subject id and stores its rows under the
// section's key.
func exportSections(ctx context.Context, tx pgx.Tx, sections []exportSection, id string, into map[string]any) error {
	for _, s := range sections {
		wrapped := `SELECT COALESCE(json_agg(t), '[]'::json) FROM (` + s.query + `) t`
		if s.one {
			wrapped = `SELECT (SELECT to_json(t) FROM (` + s.query + `) t)`
		}
		var rows json.RawMessage
		if err := tx.QueryRow(ctx, wrapped, id).Scan(&rows); err != nil {
			return fmt.Errorf("export section %s: %w", s.key, err)
		}
		into[s.key] = rows
	}
	return nil
}

// exportViewerAccounts exports every viewer identity whose sessions are linked to the user.
func exportViewerAccounts(ctx context.Context, tx pgx.Tx, userID string) ([]map[string]any, error) {
	rows, err := tx.Query(ctx, `
		SELECT DISTINCT viewer_id::text FROM viewer_sessions WHERE user_id = $1 AND viewer_id IS NOT NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("export viewer accounts: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("export viewer accounts: %w", err)
	}
	accounts := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		account := map[string]any{}
		if err := exportSections(ctx, tx, viewerExportSections, id, account); err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

type exportMediaObject struct {
	Filename    string    `json:"filename" db:"filename"`
	ContentType string    `json:"content_type" db:"content_type"`
	SizeBytes   int64     `json:"size_bytes" db:"size_bytes"`
	ObjectKey   string    `json:"object_key" db:"object_key"`
	URL         string    `json:"url" db:"-"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// exportMedia lists the user's uploaded files with the public URL each is served from.
func exportMedia(ctx context.Context, tx pgx.Tx, userID string) ([]exportMediaObject, error) {
	rows, err := tx.Query(ctx, `
		SELECT filename, content_type, size_bytes, object_key, created_at
		FROM media_objects WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("export media: %w", err)
	}
	media, err := pgx.CollectRows(rows, pgx.RowToStructByName[exportMediaObject])
	if err != nil {
		return nil, fmt.Errorf("export media: %w", err)
	}
	for i := range media {
		media[i].URL = mediaPublicURL(media[i].ObjectKey)
	}
	return media, nil
}

// mediaPublicURL builds an object's public URL the way media-service builds public_url: the
// MEDIA_PUBLIC_URL base plus the key with each segment escaped, since a raw "#" or "?" in a
// filename would otherwise cut the URL short.
func mediaPublicURL(objectKey string) string {
	base := os.Getenv("MEDIA_PUBLIC_URL")
	if base == "" {
		base = defaultMediaPublicURL
	}
	segments := strings.Split(objectKey, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.Join(segments, "/")
}
