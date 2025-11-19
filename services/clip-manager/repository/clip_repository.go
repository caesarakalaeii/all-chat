package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/caesar/all-chat/services/clip-manager/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ClipRepository handles database operations for clips
type ClipRepository struct {
	db *pgxpool.Pool
}

// NewClipRepository creates a new clip repository
func NewClipRepository(db *pgxpool.Pool) *ClipRepository {
	return &ClipRepository{db: db}
}

// CreateClip inserts a new clip
func (r *ClipRepository) CreateClip(ctx context.Context, clip *models.Clip) error {
	query := `
		INSERT INTO clips (
			id, user_id, stream_session_id, platform, platform_clip_id,
			clip_url, embed_url, thumbnail_url, title, duration_seconds,
			view_count, created_at_platform, is_user_provided, user_notes,
			rank_score, fetched_at, last_updated
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		)
		ON CONFLICT (platform, platform_clip_id)
		DO UPDATE SET
			view_count = EXCLUDED.view_count,
			rank_score = EXCLUDED.rank_score,
			last_updated = EXCLUDED.last_updated
	`

	_, err := r.db.Exec(ctx, query,
		clip.ID,
		clip.UserID,
		clip.StreamSessionID,
		clip.Platform,
		clip.PlatformClipID,
		clip.ClipURL,
		clip.EmbedURL,
		clip.ThumbnailURL,
		clip.Title,
		clip.DurationSeconds,
		clip.ViewCount,
		clip.CreatedAtPlatform,
		clip.IsUserProvided,
		clip.UserNotes,
		clip.RankScore,
		clip.FetchedAt,
		clip.LastUpdated,
	)

	if err != nil {
		return fmt.Errorf("failed to insert clip: %w", err)
	}

	return nil
}

// GetClipsByUser retrieves all clips for a user
func (r *ClipRepository) GetClipsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.Clip, error) {
	query := `
		SELECT
			id, user_id, stream_session_id, platform, platform_clip_id,
			clip_url, embed_url, thumbnail_url, title, duration_seconds,
			view_count, created_at_platform, is_user_provided, user_notes,
			rank_score, fetched_at, last_updated
		FROM clips
		WHERE user_id = $1
		ORDER BY rank_score DESC NULLS LAST, view_count DESC
		LIMIT $2
	`

	return r.queryClips(ctx, query, userID, limit)
}

// GetTopClips retrieves the highest-ranked clips for a user
func (r *ClipRepository) GetTopClips(ctx context.Context, userID uuid.UUID, limit int) ([]models.Clip, error) {
	query := `
		SELECT
			id, user_id, stream_session_id, platform, platform_clip_id,
			clip_url, embed_url, thumbnail_url, title, duration_seconds,
			view_count, created_at_platform, is_user_provided, user_notes,
			rank_score, fetched_at, last_updated
		FROM clips
		WHERE user_id = $1 AND rank_score IS NOT NULL
		ORDER BY rank_score DESC
		LIMIT $2
	`

	return r.queryClips(ctx, query, userID, limit)
}

// GetClipsBySession retrieves clips for a specific stream session
func (r *ClipRepository) GetClipsBySession(ctx context.Context, sessionID uuid.UUID) ([]models.Clip, error) {
	query := `
		SELECT
			id, user_id, stream_session_id, platform, platform_clip_id,
			clip_url, embed_url, thumbnail_url, title, duration_seconds,
			view_count, created_at_platform, is_user_provided, user_notes,
			rank_score, fetched_at, last_updated
		FROM clips
		WHERE stream_session_id = $1
		ORDER BY rank_score DESC NULLS LAST
	`

	return r.queryClips(ctx, query, sessionID)
}

// queryClips is a helper function to query clips
func (r *ClipRepository) queryClips(ctx context.Context, query string, args ...interface{}) ([]models.Clip, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query clips: %w", err)
	}
	defer rows.Close()

	var clips []models.Clip
	for rows.Next() {
		var clip models.Clip
		var platformClipID, embedURL, thumbnailURL, title sql.NullString
		var durationSeconds sql.NullInt32
		var createdAtPlatform sql.NullTime
		var userNotes sql.NullString
		var rankScore sql.NullFloat64

		// Use sql.Null types for nullable UUID
		var streamSessionUUID sql.NullString

		err := rows.Scan(
			&clip.ID,
			&clip.UserID,
			&streamSessionUUID,
			&clip.Platform,
			&platformClipID,
			&clip.ClipURL,
			&embedURL,
			&thumbnailURL,
			&title,
			&durationSeconds,
			&clip.ViewCount,
			&createdAtPlatform,
			&clip.IsUserProvided,
			&userNotes,
			&rankScore,
			&clip.FetchedAt,
			&clip.LastUpdated,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan clip: %w", err)
		}

		// Handle nullable fields
		if streamSessionUUID.Valid {
			sessionID, _ := uuid.Parse(streamSessionUUID.String)
			clip.StreamSessionID = &sessionID
		}
		if platformClipID.Valid {
			clip.PlatformClipID = &platformClipID.String
		}
		if embedURL.Valid {
			clip.EmbedURL = &embedURL.String
		}
		if thumbnailURL.Valid {
			clip.ThumbnailURL = &thumbnailURL.String
		}
		if title.Valid {
			clip.Title = &title.String
		}
		if durationSeconds.Valid {
			duration := int(durationSeconds.Int32)
			clip.DurationSeconds = &duration
		}
		if createdAtPlatform.Valid {
			clip.CreatedAtPlatform = &createdAtPlatform.Time
		}
		if userNotes.Valid {
			clip.UserNotes = &userNotes.String
		}
		if rankScore.Valid {
			clip.RankScore = &rankScore.Float64
		}

		clips = append(clips, clip)
	}

	return clips, nil
}

// DeleteClip removes a clip
func (r *ClipRepository) DeleteClip(ctx context.Context, clipID uuid.UUID) error {
	query := `DELETE FROM clips WHERE id = $1`

	result, err := r.db.Exec(ctx, query, clipID)
	if err != nil {
		return fmt.Errorf("failed to delete clip: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("clip not found")
	}

	return nil
}
