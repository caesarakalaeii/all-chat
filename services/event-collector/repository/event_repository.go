package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/caesar/all-chat/services/event-collector/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventRepository handles database operations for stream events
type EventRepository struct {
	db *pgxpool.Pool
}

// NewEventRepository creates a new event repository
func NewEventRepository(db *pgxpool.Pool) *EventRepository {
	return &EventRepository{db: db}
}

// CreateEvent inserts a new stream event
func (r *EventRepository) CreateEvent(ctx context.Context, event *models.StreamEvent) error {
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO stream_events (
			id, stream_session_id, user_id, platform, event_type, event_subtype,
			platform_user_id, platform_username, display_name, avatar_url,
			metadata, occurred_at, created_at, is_test, is_backfilled
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
		)
	`

	_, err = r.db.Exec(ctx, query,
		event.ID,
		event.StreamSessionID,
		event.UserID,
		event.Platform,
		event.EventType,
		event.EventSubtype,
		event.PlatformUser.ID,
		event.PlatformUser.Username,
		event.PlatformUser.DisplayName,
		event.PlatformUser.AvatarURL,
		metadataJSON,
		event.OccurredAt,
		event.CreatedAt,
		event.IsTest,
		event.IsBackfilled,
	)

	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	return nil
}

// GetEventsBySession retrieves all events for a stream session
func (r *EventRepository) GetEventsBySession(ctx context.Context, sessionID uuid.UUID) ([]models.StreamEvent, error) {
	query := `
		SELECT
			id, stream_session_id, user_id, platform, event_type, event_subtype,
			platform_user_id, platform_username, display_name, avatar_url,
			metadata, occurred_at, created_at, is_test, is_backfilled
		FROM stream_events
		WHERE stream_session_id = $1
		ORDER BY occurred_at ASC
	`

	rows, err := r.db.Query(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []models.StreamEvent
	for rows.Next() {
		var event models.StreamEvent
		var metadataJSON []byte
		var avatarURL sql.NullString
		var eventSubtype sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.StreamSessionID,
			&event.UserID,
			&event.Platform,
			&event.EventType,
			&eventSubtype,
			&event.PlatformUser.ID,
			&event.PlatformUser.Username,
			&event.PlatformUser.DisplayName,
			&avatarURL,
			&metadataJSON,
			&event.OccurredAt,
			&event.CreatedAt,
			&event.IsTest,
			&event.IsBackfilled,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if eventSubtype.Valid {
			event.EventSubtype = &eventSubtype.String
		}

		if avatarURL.Valid {
			event.PlatformUser.AvatarURL = &avatarURL.String
		}

		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

// GetEventsBySessionAndType retrieves events filtered by type
func (r *EventRepository) GetEventsBySessionAndType(ctx context.Context, sessionID uuid.UUID, eventType string) ([]models.StreamEvent, error) {
	query := `
		SELECT
			id, stream_session_id, user_id, platform, event_type, event_subtype,
			platform_user_id, platform_username, display_name, avatar_url,
			metadata, occurred_at, created_at, is_test, is_backfilled
		FROM stream_events
		WHERE stream_session_id = $1 AND event_type = $2
		ORDER BY occurred_at ASC
	`

	rows, err := r.db.Query(ctx, query, sessionID, eventType)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []models.StreamEvent
	for rows.Next() {
		var event models.StreamEvent
		var metadataJSON []byte
		var avatarURL sql.NullString
		var eventSubtype sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.StreamSessionID,
			&event.UserID,
			&event.Platform,
			&event.EventType,
			&eventSubtype,
			&event.PlatformUser.ID,
			&event.PlatformUser.Username,
			&event.PlatformUser.DisplayName,
			&avatarURL,
			&metadataJSON,
			&event.OccurredAt,
			&event.CreatedAt,
			&event.IsTest,
			&event.IsBackfilled,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if eventSubtype.Valid {
			event.EventSubtype = &eventSubtype.String
		}

		if avatarURL.Valid {
			event.PlatformUser.AvatarURL = &avatarURL.String
		}

		if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}

		events = append(events, event)
	}

	return events, nil
}

// UpdateSessionStats updates the session statistics
func (r *EventRepository) UpdateSessionStats(ctx context.Context, sessionID uuid.UUID, eventType string, amount int) error {
	// This will be called to increment counters in the session stats JSONB
	var query string

	switch eventType {
	case models.EventTypeFollow:
		query = `
			UPDATE stream_sessions
			SET stats = jsonb_set(
				stats,
				'{followers}',
				(COALESCE((stats->>'followers')::int, 0) + 1)::text::jsonb
			),
			stats = jsonb_set(
				stats,
				'{total_events}',
				(COALESCE((stats->>'total_events')::int, 0) + 1)::text::jsonb
			)
			WHERE id = $1
		`
	case models.EventTypeSub, models.EventTypeGiftSub:
		query = `
			UPDATE stream_sessions
			SET stats = jsonb_set(
				stats,
				'{subscribers}',
				(COALESCE((stats->>'subscribers')::int, 0) + 1)::text::jsonb
			),
			stats = jsonb_set(
				stats,
				'{total_events}',
				(COALESCE((stats->>'total_events')::int, 0) + 1)::text::jsonb
			)
			WHERE id = $1
		`
	case models.EventTypeBits:
		query = `
			UPDATE stream_sessions
			SET stats = jsonb_set(
				stats,
				'{bits_total}',
				(COALESCE((stats->>'bits_total')::int, 0) + $2)::text::jsonb
			),
			stats = jsonb_set(
				stats,
				'{total_events}',
				(COALESCE((stats->>'total_events')::int, 0) + 1)::text::jsonb
			)
			WHERE id = $1
		`
	case models.EventTypeSuperChat:
		query = `
			UPDATE stream_sessions
			SET stats = jsonb_set(
				stats,
				'{super_chat_total}',
				(COALESCE((stats->>'super_chat_total')::int, 0) + $2)::text::jsonb
			),
			stats = jsonb_set(
				stats,
				'{total_events}',
				(COALESCE((stats->>'total_events')::int, 0) + 1)::text::jsonb
			)
			WHERE id = $1
		`
	default:
		// For other event types, just increment total_events
		query = `
			UPDATE stream_sessions
			SET stats = jsonb_set(
				stats,
				'{total_events}',
				(COALESCE((stats->>'total_events')::int, 0) + 1)::text::jsonb
			)
			WHERE id = $1
		`
	}

	_, err := r.db.Exec(ctx, query, sessionID, amount)
	if err != nil {
		return fmt.Errorf("failed to update session stats: %w", err)
	}

	return nil
}
