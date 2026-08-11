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
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// DataExportResponse is the DSGVO Art. 20 data-portability envelope.
// It returns all personal data we store for the requesting user in a
// machine-readable JSON format.
type DataExportResponse struct {
	ExportedAt string              `json:"exported_at"`
	User       DataExportUser      `json:"user"`
	Overlays   []DataExportOverlay `json:"overlays"`
	Viewers    []DataExportViewer  `json:"viewers,omitempty"`
	Guilds     []DataExportGuild   `json:"guilds,omitempty"`
	Messages   []DataExportMessage `json:"messages,omitempty"`
	// Delegations covers both directions of a moderation grant (ADR-0048): people this user
	// delegated moderation to, and overlays delegated to this user. Both are personal data about
	// the exporting user, and neither is derivable from the sections above.
	Delegations []DataExportDelegation `json:"moderation_delegations,omitempty"`
	// DiscordAccount is the linked Discord identity (migration 083). A third-party account
	// identifier is personal data in its own right and is not derivable from Guilds, which records
	// servers rather than people.
	DiscordAccount *DataExportDiscordAccount `json:"discord_account,omitempty"`
}

// DataExportDiscordAccount is the Discord user account linked to this All-Chat account.
type DataExportDiscordAccount struct {
	DiscordUserID   string    `json:"discord_user_id"`
	DiscordUsername string    `json:"discord_username,omitempty"`
	LinkedAt        time.Time `json:"linked_at"`
}

// DataExportDelegation is one delegated-moderation grant, from whichever side the exporting user
// sits on.
type DataExportDelegation struct {
	// Direction is "granted" when the user delegated moderation on their own overlay, and
	// "received" when someone delegated it to them.
	Direction   string     `json:"direction"`
	OverlayID   string     `json:"overlay_id"`
	OverlayName string     `json:"overlay_name"`
	Status      string     `json:"status"`
	Actions     []string   `json:"actions"`
	Counterpart string     `json:"counterpart,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	AcceptedAt  *time.Time `json:"accepted_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

// DataExportUser contains the user profile (tokens excluded).
type DataExportUser struct {
	ID              string    `json:"id"`
	Username        string    `json:"username"`
	DisplayName     string    `json:"display_name"`
	ProfileImageURL string    `json:"profile_image_url"`
	AuthProvider    string    `json:"auth_provider"`
	TwitchID        *string   `json:"twitch_id,omitempty"`
	GoogleID        *string   `json:"google_id,omitempty"`
	KickID          *string   `json:"kick_id,omitempty"`
	IsPremium       bool      `json:"is_premium"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DataExportOverlay is a single overlay with its sources.
type DataExportOverlay struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	IsActive  bool               `json:"is_active"`
	CreatedAt time.Time          `json:"created_at"`
	Sources   []DataExportSource `json:"sources"`
}

// DataExportSource is a connected chat source.
type DataExportSource struct {
	Platform    string `json:"platform"`
	ChannelID   string `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	IsActive    bool   `json:"is_active"`
}

// DataExportViewer is a viewer session (tokens excluded).
type DataExportViewer struct {
	Platform    string    `json:"platform"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// DataExportGuild is a connected Discord server.
type DataExportGuild struct {
	GuildID     string    `json:"guild_id"`
	GuildName   string    `json:"guild_name"`
	ConnectedAt time.Time `json:"connected_at"`
}

// DataExportMessage is a message sent through All-Chat (retained <=1h).
type DataExportMessage struct {
	Platform    string    `json:"platform"`
	ChannelName string    `json:"channel_name"`
	MessageText string    `json:"message_text"`
	SentAt      time.Time `json:"sent_at"`
	Success     bool      `json:"success"`
}

// HandleDataExport returns all personal data stored for the authenticated user.
// DSGVO Art. 20 — right to data portability.
func (h *AuthHandler) HandleDataExport(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	uid := userID.(string)
	ctx := c.Request.Context()

	user, err := h.userRepo.GetByID(ctx, uid)
	if err != nil {
		h.logger.Error("Data export: user not found", zap.String("user_id", uid), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	export := DataExportResponse{
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		User: DataExportUser{
			ID:              user.ID,
			Username:        user.Username,
			DisplayName:     user.DisplayName,
			ProfileImageURL: user.ProfileImageURL,
			AuthProvider:    user.AuthProvider,
			TwitchID:        user.TwitchID,
			GoogleID:        user.GoogleID,
			KickID:          user.KickID,
			IsPremium:       user.IsPremium,
			CreatedAt:       user.CreatedAt,
			UpdatedAt:       user.UpdatedAt,
		},
	}

	// Fetch related data — all queries are best-effort; partial export is
	// better than no export.
	db := h.userRepo.DB()
	export.Overlays = fetchOverlays(ctx, db, uid, h.logger)
	export.Viewers = fetchViewerSessions(ctx, db, uid, h.logger)
	export.Guilds = fetchGuilds(ctx, db, uid, h.logger)
	export.DiscordAccount = fetchDiscordAccount(ctx, db, uid, h.logger)
	export.Messages = fetchMessages(ctx, db, uid, h.logger)
	export.Delegations = fetchDelegations(ctx, db, uid, h.logger)

	c.Header("Content-Disposition", "attachment; filename=allchat-data-export.json")
	c.JSON(http.StatusOK, export)
}

func fetchOverlays(ctx context.Context, db *pgxpool.Pool, userID string, log *zap.Logger) []DataExportOverlay {
	rows, err := db.Query(ctx, `
		SELECT o.id, o.name, o.is_active, o.created_at,
		       COALESCE(s.platform, ''), COALESCE(s.channel_id, ''),
		       COALESCE(s.channel_name, ''), COALESCE(s.is_active, false)
		FROM overlays o
		LEFT JOIN overlay_chat_sources s ON s.overlay_id = o.id
		WHERE o.user_id = $1
		ORDER BY o.created_at, s.platform`, userID)
	if err != nil {
		log.Warn("Data export: failed to fetch overlays", zap.Error(err))
		return nil
	}
	defer rows.Close()

	byID := map[string]*DataExportOverlay{}
	var ordered []string
	for rows.Next() {
		var oid, name, platform, chID, chName string
		var active, srcActive bool
		var createdAt time.Time
		if err := rows.Scan(&oid, &name, &active, &createdAt, &platform, &chID, &chName, &srcActive); err != nil {
			continue
		}
		if _, ok := byID[oid]; !ok {
			byID[oid] = &DataExportOverlay{ID: oid, Name: name, IsActive: active, CreatedAt: createdAt}
			ordered = append(ordered, oid)
		}
		if platform != "" {
			byID[oid].Sources = append(byID[oid].Sources, DataExportSource{
				Platform: platform, ChannelID: chID, ChannelName: chName, IsActive: srcActive,
			})
		}
	}
	result := make([]DataExportOverlay, 0, len(ordered))
	for _, id := range ordered {
		result = append(result, *byID[id])
	}
	return result
}

func fetchViewerSessions(ctx context.Context, db *pgxpool.Pool, userID string, log *zap.Logger) []DataExportViewer {
	rows, err := db.Query(ctx, `
		SELECT platform, username, display_name, created_at
		FROM viewer_sessions
		WHERE user_id = $1
		ORDER BY created_at`, userID)
	if err != nil {
		log.Warn("Data export: failed to fetch viewer sessions", zap.Error(err))
		return nil
	}
	defer rows.Close()

	var out []DataExportViewer
	for rows.Next() {
		var v DataExportViewer
		if err := rows.Scan(&v.Platform, &v.Username, &v.DisplayName, &v.CreatedAt); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out
}

func fetchGuilds(ctx context.Context, db *pgxpool.Pool, userID string, log *zap.Logger) []DataExportGuild {
	rows, err := db.Query(ctx, `
		SELECT guild_id, guild_name, connected_at
		FROM discord_guilds
		WHERE user_id = $1
		ORDER BY connected_at`, userID)
	if err != nil {
		log.Warn("Data export: failed to fetch guilds", zap.Error(err))
		return nil
	}
	defer rows.Close()

	var out []DataExportGuild
	for rows.Next() {
		var g DataExportGuild
		if err := rows.Scan(&g.GuildID, &g.GuildName, &g.ConnectedAt); err != nil {
			continue
		}
		out = append(out, g)
	}
	return out
}

// fetchDiscordAccount returns the linked Discord identity, or nil when the user has not linked
// one. Absence is the common case and is not an error.
func fetchDiscordAccount(ctx context.Context, db *pgxpool.Pool, userID string, log *zap.Logger) *DataExportDiscordAccount {
	var acc DataExportDiscordAccount
	var username *string
	err := db.QueryRow(ctx, `
		SELECT discord_user_id, discord_username, linked_at
		FROM discord_identities
		WHERE user_id = $1`, userID).Scan(&acc.DiscordUserID, &username, &acc.LinkedAt)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Warn("Data export: failed to fetch discord account", zap.Error(err))
		}
		return nil
	}
	if username != nil {
		acc.DiscordUsername = *username
	}
	return &acc
}

// fetchDelegations returns every delegated-moderation grant the user is a party to, in both
// directions (ADR-0048).
//
// Revoked grants are included deliberately: they are retained as history, so a portability export
// that omitted them would misreport what All-Chat still holds. Erasure needs no counterpart here —
// overlay_moderators cascades from both overlays and users, so deleting the account removes every
// row where this user is the moderator, and every row on their own overlays.
func fetchDelegations(ctx context.Context, db *pgxpool.Pool, userID string, log *zap.Logger) []DataExportDelegation {
	rows, err := db.Query(ctx, `
		SELECT 'granted', m.overlay_id::text, o.name, m.status, m.actions,
		       -- The name captured at accept time, else whoever the account says it is now, else
		       -- the label the streamer typed on an invite nobody has redeemed.
		       COALESCE(NULLIF(m.moderator_display_name, ''), NULLIF(mod_user.display_name, ''),
		                m.invitee_label, ''),
		       m.created_at, m.accepted_at, m.revoked_at
		FROM overlay_moderators m
		JOIN overlays o ON o.id = m.overlay_id
		LEFT JOIN users mod_user ON mod_user.id = m.moderator_user_id
		WHERE o.user_id = $1
		UNION ALL
		SELECT 'received', m.overlay_id::text, o.name, m.status, m.actions,
		       COALESCE(owner.display_name, ''),
		       m.created_at, m.accepted_at, m.revoked_at
		FROM overlay_moderators m
		JOIN overlays o ON o.id = m.overlay_id
		JOIN users owner ON owner.id = o.user_id
		WHERE m.moderator_user_id = $1
		ORDER BY 7`, userID)
	if err != nil {
		// The table may not exist yet on a deployment that has not run migration 080. A partial
		// export is better than none.
		log.Warn("Data export: failed to fetch moderation delegations", zap.Error(err))
		return nil
	}
	defer rows.Close()

	var out []DataExportDelegation
	for rows.Next() {
		var d DataExportDelegation
		if err := rows.Scan(&d.Direction, &d.OverlayID, &d.OverlayName, &d.Status, &d.Actions,
			&d.Counterpart, &d.CreatedAt, &d.AcceptedAt, &d.RevokedAt); err != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}

func fetchMessages(ctx context.Context, db *pgxpool.Pool, userID string, log *zap.Logger) []DataExportMessage {
	rows, err := db.Query(ctx, `
		SELECT vmh.platform, vmh.channel_name, vmh.message_text, vmh.sent_at, vmh.success
		FROM viewer_message_history vmh
		JOIN viewer_sessions vs ON vs.id = vmh.viewer_session_id
		WHERE vmh.streamer_user_id = $1 OR vs.user_id = $1
		ORDER BY vmh.sent_at DESC
		LIMIT 1000`, userID)
	if err != nil {
		log.Warn("Data export: failed to fetch messages", zap.Error(err))
		return nil
	}
	defer rows.Close()

	var out []DataExportMessage
	for rows.Next() {
		var m DataExportMessage
		if err := rows.Scan(&m.Platform, &m.ChannelName, &m.MessageText, &m.SentAt, &m.Success); err != nil {
			continue
		}
		out = append(out, m)
	}
	return out
}
