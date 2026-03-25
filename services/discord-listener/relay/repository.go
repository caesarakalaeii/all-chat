package relay

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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

// Repository queries PostgreSQL for relay-enabled discord sources.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new relay Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetRelayConfigs returns all (overlay_id, webhook_url) pairs for active,
// relay-enabled discord sources that have a webhook URL configured.
func (r *Repository) GetRelayConfigs(ctx context.Context) ([]relayConfig, error) {
	query := `
		SELECT ocs.overlay_id,
		       ocs.config->>'relay_webhook_url' AS webhook_url
		FROM overlay_chat_sources ocs
		JOIN overlays o ON o.id = ocs.overlay_id
		WHERE ocs.platform = 'discord'
		  AND (ocs.config->>'relay_enabled')::boolean = true
		  AND ocs.config->>'relay_webhook_url' IS NOT NULL
		  AND o.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query relay configs: %w", err)
	}
	defer rows.Close()

	var configs []relayConfig
	for rows.Next() {
		var cfg relayConfig
		if err := rows.Scan(&cfg.OverlayID, &cfg.WebhookURL); err != nil {
			return nil, fmt.Errorf("failed to scan relay config row: %w", err)
		}
		configs = append(configs, cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating relay config rows: %w", err)
	}

	return configs, nil
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

// StoreWebhookURL persists a provisioned webhook URL into the source's JSONB
// config and sends a NOTIFY so the relay manager picks up the change immediately.
func (r *Repository) StoreWebhookURL(ctx context.Context, sourceID, webhookURL string) error {
	query := `
		UPDATE overlay_chat_sources
		SET config = config || jsonb_build_object('relay_webhook_url', $2::text),
		    updated_at = NOW()
		WHERE id = $1
	`
	if _, err := r.db.Exec(ctx, query, sourceID, webhookURL); err != nil {
		return fmt.Errorf("failed to store webhook URL for source %s: %w", sourceID, err)
	}

	if _, err := r.db.Exec(ctx, "SELECT pg_notify('chat_source_changes', $1)", sourceID); err != nil {
		return fmt.Errorf("failed to notify chat_source_changes: %w", err)
	}

	return nil
}
