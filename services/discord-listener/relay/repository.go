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

// RepositoryInterface is the database contract for relay config queries.
type RepositoryInterface interface {
	GetRelayConfigs(ctx context.Context) ([]relayConfig, error)
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
