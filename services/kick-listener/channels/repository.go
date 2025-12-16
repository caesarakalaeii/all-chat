package channels

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ActiveChannel represents an active Kick channel to monitor
type ActiveChannel struct {
	OverlayID   string
	ChannelSlug string
	ChatroomID  int
	IsActive    bool
}

// Repository handles database operations for channels
type queryExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Repository struct {
	db     queryExecutor
	logger *zap.Logger
}

// NewRepository creates a new channel repository
func NewRepository(db *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// GetActiveChannels retrieves all active Kick channels from the database
func (r *Repository) GetActiveChannels(ctx context.Context) ([]*ActiveChannel, error) {
	query := `
		SELECT
			ocs.overlay_id,
			ocs.channel_id as channel_slug,
			ocs.config->>'chatroom_id' as chatroom_id,
			ocs.is_active
		FROM overlay_chat_sources ocs
		WHERE ocs.platform = 'kick'
		  AND ocs.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active channels: %w", err)
	}
	defer rows.Close()

	var channels []*ActiveChannel
	for rows.Next() {
		var ch ActiveChannel
		var chatroomID sql.NullString

		err := rows.Scan(
			&ch.OverlayID,
			&ch.ChannelSlug,
			&chatroomID,
			&ch.IsActive,
		)
		if err != nil {
			r.logger.Error("Failed to scan channel row", zap.Error(err))
			continue
		}

		// If chatroom_id is set, parse the value
		if chatroomID.Valid && chatroomID.String != "" {
			parsedID, err := strconv.Atoi(chatroomID.String)
			if err != nil {
				r.logger.Warn("Invalid chatroom_id metadata",
					zap.String("overlay_id", ch.OverlayID),
					zap.String("channel_slug", ch.ChannelSlug),
					zap.String("raw_chatroom_id", chatroomID.String),
					zap.Error(err),
				)
			} else {
				ch.ChatroomID = parsedID
			}
		}

		channels = append(channels, &ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel rows: %w", err)
	}

	r.logger.Info("Retrieved active Kick channels",
		zap.Int("count", len(channels)),
	)

	return channels, nil
}

// UpdateChatroomID updates the chatroom ID for a channel in the database
func (r *Repository) UpdateChatroomID(ctx context.Context, overlayID, channelSlug string, chatroomID int) error {
	query := `
		UPDATE overlay_chat_sources
		SET config = jsonb_set(
			COALESCE(config, '{}'::jsonb),
			'{chatroom_id}',
			to_jsonb($3::int)
		)
		WHERE overlay_id = $1
		  AND channel_id = $2
		  AND platform = 'kick'
	`

	result, err := r.db.Exec(ctx, query, overlayID, channelSlug, chatroomID)
	if err != nil {
		return fmt.Errorf("failed to update chatroom ID: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no rows updated for overlay_id=%s, channel=%s", overlayID, channelSlug)
	}

	r.logger.Info("Updated chatroom ID",
		zap.String("overlay_id", overlayID),
		zap.String("channel_slug", channelSlug),
		zap.Int("chatroom_id", chatroomID),
	)

	return nil
}

// SetSourceActive updates the is_active flag for Kick sources with the given channel slug
func (r *Repository) SetSourceActive(ctx context.Context, channelSlug string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'kick'
		  AND channel_id = $2
	`

	result, err := r.db.Exec(ctx, query, isActive, channelSlug)
	if err != nil {
		r.logger.Error("Failed to update source status",
			zap.String("channel_slug", channelSlug),
			zap.Bool("is_active", isActive),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update source status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// Not an error - source may have been removed
		r.logger.Debug("No sources updated (may have been removed)",
			zap.String("channel_slug", channelSlug),
		)
		return nil
	}

	r.logger.Debug("Updated source status",
		zap.String("channel_slug", channelSlug),
		zap.Bool("is_active", isActive),
		zap.Int64("rows_affected", rowsAffected),
	)

	return nil
}
