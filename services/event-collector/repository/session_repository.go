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

// SessionRepository handles database operations for stream sessions
type SessionRepository struct {
	db *pgxpool.Pool
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

// CreateSession creates a new stream session
func (r *SessionRepository) CreateSession(ctx context.Context, session *models.StreamSession) error {
	platformInfoJSON, err := json.Marshal(session.PlatformInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal platform info: %w", err)
	}

	statsJSON, err := json.Marshal(session.Stats)
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}

	query := `
		INSERT INTO stream_sessions (
			id, user_id, title, description, started_at, ended_at,
			platform_info, status, stats, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	_, err = r.db.Exec(ctx, query,
		session.ID,
		session.UserID,
		session.Title,
		session.Description,
		session.StartedAt,
		session.EndedAt,
		platformInfoJSON,
		session.Status,
		statsJSON,
		session.CreatedAt,
		session.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert session: %w", err)
	}

	return nil
}

// GetActiveSession retrieves the current active session for a user
func (r *SessionRepository) GetActiveSession(ctx context.Context, userID uuid.UUID) (*models.StreamSession, error) {
	query := `
		SELECT
			id, user_id, title, description, started_at, ended_at,
			platform_info, status, stats, created_at, updated_at
		FROM stream_sessions
		WHERE user_id = $1 AND status = $2
		ORDER BY started_at DESC
		LIMIT 1
	`

	var session models.StreamSession
	var platformInfoJSON, statsJSON []byte
	var title, description sql.NullString
	var endedAt sql.NullTime

	err := r.db.QueryRow(ctx, query, userID, models.SessionStatusLive).Scan(
		&session.ID,
		&session.UserID,
		&title,
		&description,
		&session.StartedAt,
		&endedAt,
		&platformInfoJSON,
		&session.Status,
		&statsJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query active session: %w", err)
	}

	if title.Valid {
		session.Title = &title.String
	}
	if description.Valid {
		session.Description = &description.String
	}
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}

	if err := json.Unmarshal(platformInfoJSON, &session.PlatformInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal platform info: %w", err)
	}

	if err := json.Unmarshal(statsJSON, &session.Stats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
	}

	return &session, nil
}

// GetMostRecentSession retrieves the most recent session for a user (regardless of status)
func (r *SessionRepository) GetMostRecentSession(ctx context.Context, userID uuid.UUID) (*models.StreamSession, error) {
	query := `
		SELECT
			id, user_id, title, description, started_at, ended_at,
			platform_info, status, stats, created_at, updated_at
		FROM stream_sessions
		WHERE user_id = $1
		ORDER BY started_at DESC
		LIMIT 1
	`

	var session models.StreamSession
	var platformInfoJSON, statsJSON []byte
	var title, description sql.NullString
	var endedAt sql.NullTime

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&session.ID,
		&session.UserID,
		&title,
		&description,
		&session.StartedAt,
		&endedAt,
		&platformInfoJSON,
		&session.Status,
		&statsJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query recent session: %w", err)
	}

	if title.Valid {
		session.Title = &title.String
	}
	if description.Valid {
		session.Description = &description.String
	}
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}

	if err := json.Unmarshal(platformInfoJSON, &session.PlatformInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal platform info: %w", err)
	}

	if err := json.Unmarshal(statsJSON, &session.Stats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
	}

	return &session, nil
}

// GetSessionByID retrieves a session by ID
func (r *SessionRepository) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*models.StreamSession, error) {
	query := `
		SELECT
			id, user_id, title, description, started_at, ended_at,
			platform_info, status, stats, created_at, updated_at
		FROM stream_sessions
		WHERE id = $1
	`

	var session models.StreamSession
	var platformInfoJSON, statsJSON []byte
	var title, description sql.NullString
	var endedAt sql.NullTime

	err := r.db.QueryRow(ctx, query, sessionID).Scan(
		&session.ID,
		&session.UserID,
		&title,
		&description,
		&session.StartedAt,
		&endedAt,
		&platformInfoJSON,
		&session.Status,
		&statsJSON,
		&session.CreatedAt,
		&session.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	if title.Valid {
		session.Title = &title.String
	}
	if description.Valid {
		session.Description = &description.String
	}
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}

	if err := json.Unmarshal(platformInfoJSON, &session.PlatformInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal platform info: %w", err)
	}

	if err := json.Unmarshal(statsJSON, &session.Stats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
	}

	return &session, nil
}

// UpdateSessionStatus updates the session status
func (r *SessionRepository) UpdateSessionStatus(ctx context.Context, sessionID uuid.UUID, status string) error {
	query := `
		UPDATE stream_sessions
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(ctx, query, status, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}

	return nil
}

// EndSession marks a session as ended
func (r *SessionRepository) EndSession(ctx context.Context, sessionID uuid.UUID) error {
	query := `
		UPDATE stream_sessions
		SET status = $1, ended_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`

	_, err := r.db.Exec(ctx, query, models.SessionStatusEnded, sessionID)
	if err != nil {
		return fmt.Errorf("failed to end session: %w", err)
	}

	return nil
}

// GetSessionsByUser retrieves all sessions for a user
func (r *SessionRepository) GetSessionsByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.StreamSession, error) {
	query := `
		SELECT
			id, user_id, title, description, started_at, ended_at,
			platform_info, status, stats, created_at, updated_at
		FROM stream_sessions
		WHERE user_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []models.StreamSession
	for rows.Next() {
		var session models.StreamSession
		var platformInfoJSON, statsJSON []byte
		var title, description sql.NullString
		var endedAt sql.NullTime

		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&title,
			&description,
			&session.StartedAt,
			&endedAt,
			&platformInfoJSON,
			&session.Status,
			&statsJSON,
			&session.CreatedAt,
			&session.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if title.Valid {
			session.Title = &title.String
		}
		if description.Valid {
			session.Description = &description.String
		}
		if endedAt.Valid {
			session.EndedAt = &endedAt.Time
		}

		if err := json.Unmarshal(platformInfoJSON, &session.PlatformInfo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal platform info: %w", err)
		}

		if err := json.Unmarshal(statsJSON, &session.Stats); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}
