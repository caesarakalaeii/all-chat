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

package relay

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// relayConfig holds per-overlay relay routing information.
type relayConfig struct {
	OverlayID  string
	WebhookURL string
}

// pendingRelayConfig represents a source that has relay enabled and a channel
// ID but no webhook URL yet — it needs a webhook to be auto-provisioned.
type pendingRelayConfig struct {
	SourceID  string // overlay_chat_sources.id (needed for UPDATE WHERE)
	OverlayID string
	ChannelID string // relay_channel_id from JSONB config
}

// RepositoryInterface is the database contract for relay config queries.
type RepositoryInterface interface {
	GetRelayConfigs(ctx context.Context) ([]relayConfig, error)
	GetPendingRelayConfigs(ctx context.Context) ([]pendingRelayConfig, error)
	StoreWebhookURL(ctx context.Context, sourceID, webhookURL string) error
}

// querier is the part of *pgxpool.Pool the repository uses.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository queries PostgreSQL for relay-enabled discord sources.
//
// config.relay_webhook_url is a credential (anyone holding it can post into the channel), so it
// is stored encrypted with the shared token key (ADR-0091). A value containing "://" is a URL
// written before that; it still relays and is encrypted in place on first read, as is one under
// a retired key. Ciphertext is base64, which has no ':', so the two cannot be confused. Never
// log the URL.
type Repository struct {
	db     querier
	cipher *encryption.MultiKeyEncryptor // nil without TOKEN_ENCRYPTION_KEY_V1: URLs stay plaintext
	logger *zap.Logger
}

// NewRepository creates a new relay Repository. cipher may be nil, in which case webhook URLs
// are written in plaintext and encrypted rows cannot be read.
func NewRepository(db querier, cipher *encryption.MultiKeyEncryptor, logger *zap.Logger) *Repository {
	return &Repository{db: db, cipher: cipher, logger: logger}
}

// isDiscordWebhookURL reports whether raw is a Discord webhook URL. The config JSONB was once
// client-writable, so anything else is refused rather than posted to.
func isDiscordWebhookURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" &&
		(u.Host == "discord.com" || u.Host == "discordapp.com") &&
		strings.HasPrefix(u.Path, "/api/webhooks/")
}

// openWebhookURL returns the URL a stored relay_webhook_url holds, and whether the stored value
// should be rewritten: it is plaintext, or encrypted under a key other than the current one (the
// key-rotator does not sweep this JSONB key, so rotation happens here).
func (r *Repository) openWebhookURL(stored string) (string, bool, error) {
	if strings.Contains(stored, "://") {
		return stored, r.cipher != nil, nil
	}
	if r.cipher == nil {
		return "", false, errors.New("relay webhook URL is encrypted but TOKEN_ENCRYPTION_KEY_V1 is not set")
	}
	plain, kid, err := r.cipher.DecryptStringWithKid(stored)
	return plain, err == nil && kid != r.cipher.CurrentKid(), err
}

// sealWebhookURL returns the value to store for a webhook URL.
func (r *Repository) sealWebhookURL(webhookURL string) (string, error) {
	if r.cipher == nil {
		return webhookURL, nil
	}
	return r.cipher.EncryptString(webhookURL)
}

// GetRelayConfigs returns all (overlay_id, webhook_url) pairs for active,
// relay-enabled discord sources that have a webhook URL configured, decrypted. A row that
// cannot be read is skipped (and logged by source id) so it cannot stop the other relays.
func (r *Repository) GetRelayConfigs(ctx context.Context) ([]relayConfig, error) {
	query := `
		SELECT ocs.id, ocs.overlay_id,
		       ocs.config->>'relay_webhook_url' AS webhook_url
		FROM overlay_chat_sources ocs
		JOIN overlays o ON o.id = ocs.overlay_id
		WHERE ocs.platform = 'discord'
		  AND (ocs.config->>'relay_enabled')::boolean = true
		  AND ocs.config->>'relay_webhook_url' <> ''
		  AND o.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query relay configs: %w", err)
	}
	defer rows.Close()

	type storedRow struct{ sourceID, overlayID, stored string }
	var stored []storedRow
	for rows.Next() {
		var row storedRow
		if err := rows.Scan(&row.sourceID, &row.overlayID, &row.stored); err != nil {
			return nil, fmt.Errorf("failed to scan relay config row: %w", err)
		}
		stored = append(stored, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating relay config rows: %w", err)
	}
	rows.Close() // release the connection before the re-encrypt writes below

	var configs []relayConfig
	for _, row := range stored {
		webhookURL, rewrite, err := r.openWebhookURL(row.stored)
		if err != nil {
			r.logger.Error("Skipping relay source: cannot decrypt its webhook URL",
				zap.String("source_id", row.sourceID), zap.Error(err))
			continue
		}
		if !isDiscordWebhookURL(webhookURL) {
			r.logger.Error("Skipping relay source: stored webhook URL is not a Discord webhook",
				zap.String("source_id", row.sourceID))
			continue
		}
		if rewrite {
			r.reencryptWebhookURL(ctx, row.sourceID, row.stored, webhookURL)
		}
		configs = append(configs, relayConfig{OverlayID: row.overlayID, WebhookURL: webhookURL})
	}

	return configs, nil
}

// reencryptWebhookURL rewrites a stored relay_webhook_url under the current key. The WHERE
// clause only replaces the exact value read, so a webhook provisioned meanwhile is never
// overwritten. A failure is logged and retried on the next sync; the relay works either way.
func (r *Repository) reencryptWebhookURL(ctx context.Context, sourceID, stored, webhookURL string) {
	sealed, err := r.sealWebhookURL(webhookURL)
	if err == nil {
		_, err = r.db.Exec(ctx, `
			UPDATE overlay_chat_sources
			SET config = jsonb_set(config, '{relay_webhook_url}', to_jsonb($2::text))
			WHERE id = $1 AND config->>'relay_webhook_url' = $3
		`, sourceID, sealed, stored)
	}
	if err != nil {
		r.logger.Warn("Failed to re-encrypt a relay webhook URL; will retry",
			zap.String("source_id", sourceID), zap.Error(err))
	}
}

// GetPendingRelayConfigs returns sources that have relay_enabled=true and a
// relay_channel_id but no relay_webhook_url — these need webhook provisioning.
func (r *Repository) GetPendingRelayConfigs(ctx context.Context) ([]pendingRelayConfig, error) {
	query := `
		SELECT ocs.id, ocs.overlay_id,
		       ocs.config->>'relay_channel_id' AS channel_id
		FROM overlay_chat_sources ocs
		JOIN overlays o ON o.id = ocs.overlay_id
		WHERE ocs.platform = 'discord'
		  AND (ocs.config->>'relay_enabled')::boolean = true
		  AND ocs.config->>'relay_channel_id' IS NOT NULL
		  AND (ocs.config->>'relay_webhook_url' IS NULL OR ocs.config->>'relay_webhook_url' = '')
		  AND o.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending relay configs: %w", err)
	}
	defer rows.Close()

	var configs []pendingRelayConfig
	for rows.Next() {
		var cfg pendingRelayConfig
		if err := rows.Scan(&cfg.SourceID, &cfg.OverlayID, &cfg.ChannelID); err != nil {
			return nil, fmt.Errorf("failed to scan pending relay config row: %w", err)
		}
		configs = append(configs, cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending relay config rows: %w", err)
	}

	return configs, nil
}

// StoreWebhookURL persists a provisioned webhook URL, encrypted, into the source's JSONB
// config and sends a NOTIFY so the relay manager picks up the change immediately.
func (r *Repository) StoreWebhookURL(ctx context.Context, sourceID, webhookURL string) error {
	sealed, err := r.sealWebhookURL(webhookURL)
	if err != nil {
		return fmt.Errorf("failed to encrypt webhook URL for source %s: %w", sourceID, err)
	}
	query := `
		UPDATE overlay_chat_sources
		SET config = config || jsonb_build_object('relay_webhook_url', $2::text),
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := r.db.Exec(ctx, query, sourceID, sealed); err != nil {
		return fmt.Errorf("failed to store webhook URL for source %s: %w", sourceID, err)
	}

	if _, err := r.db.Exec(ctx, "SELECT pg_notify('chat_source_changes', $1)", sourceID); err != nil {
		return fmt.Errorf("failed to notify chat_source_changes: %w", err)
	}

	return nil
}
