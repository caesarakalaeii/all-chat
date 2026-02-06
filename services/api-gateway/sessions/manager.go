package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// SessionKeyPrefix is the Redis key prefix for active sessions
	SessionKeyPrefix = "session:active:"

	// SessionTTL is how long to keep session data in Redis
	SessionTTL = 24 * time.Hour
)

// parseSessionTime safely parses RFC3339 time with validation
func parseSessionTime(timeStr string, fieldName string) (time.Time, error) {
	if timeStr == "" {
		return time.Time{}, fmt.Errorf("%s is empty", fieldName)
	}

	parsed, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse %s: %w", fieldName, err)
	}

	if parsed.IsZero() {
		return time.Time{}, fmt.Errorf("%s is zero value", fieldName)
	}

	return parsed, nil
}

// validateStartedAt checks if time is valid for duration calculation
func validateStartedAt(t time.Time) error {
	if t.IsZero() {
		return fmt.Errorf("started_at is zero value")
	}

	// Sanity check: reject times before 2020 or in future
	minTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTime := time.Now().UTC().Add(1 * time.Hour)

	if t.Before(minTime) {
		return fmt.Errorf("started_at before 2020: %s", t.Format(time.RFC3339))
	}

	if t.After(maxTime) {
		return fmt.Errorf("started_at in future: %s", t.Format(time.RFC3339))
	}

	return nil
}

// SessionInfo represents the active session information
type SessionInfo struct {
	SessionID   string    `json:"session_id"`
	StartedAt   time.Time `json:"started_at"`
	State       string    `json:"state"` // ACTIVE, ENDING, COMPLETED
	EventCount  int       `json:"event_count"`
	LastEventAt time.Time `json:"last_event_at,omitempty"`
}

// SessionManager manages stream session lifecycle
type SessionManager struct {
	redis       *redis.Client
	db          *pgxpool.Pool
	logger      *zap.Logger
	gracePeriod time.Duration
}

// NewSessionManager creates a new session manager
func NewSessionManager(redis *redis.Client, db *pgxpool.Pool, logger *zap.Logger, gracePeriod time.Duration) *SessionManager {
	return &SessionManager{
		redis:       redis,
		db:          db,
		logger:      logger,
		gracePeriod: gracePeriod,
	}
}

// EnsureSession creates a session if none exists for this overlay
func (sm *SessionManager) EnsureSession(ctx context.Context, overlayID string) error {
	key := SessionKeyPrefix + overlayID

	// Check if session already exists
	exists, err := sm.redis.Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to check session existence: %w", err)
	}

	if exists > 0 {
		// Session exists, check if it's in ENDING state and revert to ACTIVE
		state, err := sm.redis.HGet(ctx, key, "state").Result()
		if err == nil && state == "ENDING" {
			// Cancel grace period by reverting to ACTIVE
			return sm.CancelGracePeriod(ctx, overlayID)
		}
		return nil // Session already active
	}

	// Create new session
	sessionID := uuid.New().String()
	startedAt := time.Now().UTC()

	session := SessionInfo{
		SessionID: sessionID,
		StartedAt: startedAt,
		State:     "ACTIVE",
		EventCount: 0,
	}

	sessionJSON, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Store session in Redis as hash
	pipe := sm.redis.Pipeline()
	pipe.HSet(ctx, key, "session_id", sessionID)
	pipe.HSet(ctx, key, "started_at", startedAt.Format(time.RFC3339))
	pipe.HSet(ctx, key, "state", "ACTIVE")
	pipe.HSet(ctx, key, "event_count", 0)
	pipe.Expire(ctx, key, SessionTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to create session in Redis: %w", err)
	}

	// Create session record in database
	query := `
		INSERT INTO stream_sessions (id, overlay_id, started_at, state)
		VALUES ($1, $2, $3, $4)
	`
	if _, err := sm.db.Exec(ctx, query, sessionID, overlayID, startedAt, "ACTIVE"); err != nil {
		sm.logger.Error("Failed to create session in database",
			zap.String("session_id", sessionID),
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		// Continue even if DB insert fails - Redis is source of truth
	}

	sm.logger.Info("Created new session",
		zap.String("session_id", sessionID),
		zap.String("overlay_id", overlayID),
		zap.String("session_json", string(sessionJSON)),
	)

	return nil
}

// StartGracePeriod transitions session to ENDING state
func (sm *SessionManager) StartGracePeriod(ctx context.Context, overlayID string) error {
	key := SessionKeyPrefix + overlayID

	// Update state to ENDING
	if err := sm.redis.HSet(ctx, key, "state", "ENDING").Err(); err != nil {
		return fmt.Errorf("failed to update session state to ENDING: %w", err)
	}

	sm.logger.Info("Session entering grace period",
		zap.String("overlay_id", overlayID),
		zap.Duration("grace_period", sm.gracePeriod),
	)

	return nil
}

// CancelGracePeriod transitions session back to ACTIVE (reconnect)
func (sm *SessionManager) CancelGracePeriod(ctx context.Context, overlayID string) error {
	key := SessionKeyPrefix + overlayID

	// Update state back to ACTIVE
	if err := sm.redis.HSet(ctx, key, "state", "ACTIVE").Err(); err != nil {
		return fmt.Errorf("failed to update session state to ACTIVE: %w", err)
	}

	sm.logger.Info("Session grace period cancelled (reconnected)",
		zap.String("overlay_id", overlayID),
	)

	return nil
}

// EndSession transitions to COMPLETED and archives to database
func (sm *SessionManager) EndSession(ctx context.Context, overlayID string) error {
	key := SessionKeyPrefix + overlayID

	// Get session info before deleting
	sessionID, err := sm.redis.HGet(ctx, key, "session_id").Result()
	if err != nil {
		if err == redis.Nil {
			// Session already ended or doesn't exist
			return nil
		}
		return fmt.Errorf("failed to get session_id: %w", err)
	}

	startedAtStr, err := sm.redis.HGet(ctx, key, "started_at").Result()
	if err != nil {
		return fmt.Errorf("failed to get started_at: %w", err)
	}

	eventCountStr, err := sm.redis.HGet(ctx, key, "event_count").Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get event_count: %w", err)
	}

	var eventCount int
	if eventCountStr != "" {
		fmt.Sscanf(eventCountStr, "%d", &eventCount)
	}

	startedAt, err := parseSessionTime(startedAtStr, "started_at")
	if err != nil {
		sm.logger.Error("Failed to parse started_at, using fallback",
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
		// Fallback: use minimal duration for DB record
		startedAt = time.Now().UTC().Add(-1 * time.Minute)
	}
	endedAt := time.Now().UTC()
	duration := endedAt.Sub(startedAt)

	// Update session in database
	query := `
		UPDATE stream_sessions
		SET state = $1, ended_at = $2, total_events = $3, updated_at = NOW()
		WHERE id = $4
	`
	if _, err := sm.db.Exec(ctx, query, "COMPLETED", endedAt, eventCount, sessionID); err != nil {
		sm.logger.Error("Failed to update session in database",
			zap.String("session_id", sessionID),
			zap.Error(err),
		)
		// Continue even if DB update fails
	}

	// Delete session from Redis
	if err := sm.redis.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session from Redis: %w", err)
	}

	sm.logger.Info("Session ended",
		zap.String("session_id", sessionID),
		zap.String("overlay_id", overlayID),
		zap.Duration("duration", duration),
		zap.Int("event_count", eventCount),
	)

	return nil
}

// GetActiveSession retrieves current session for overlay
func (sm *SessionManager) GetActiveSession(ctx context.Context, overlayID string) (*SessionInfo, error) {
	key := SessionKeyPrefix + overlayID

	// Get all session fields
	result, err := sm.redis.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no active session found")
	}

	session := &SessionInfo{
		SessionID: result["session_id"],
		State:     result["state"],
	}

	if startedAtStr, ok := result["started_at"]; ok {
		startedAt, err := parseSessionTime(startedAtStr, "started_at")
		if err != nil {
			sm.logger.Error("Invalid started_at in session",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			return nil, fmt.Errorf("invalid started_at: %w", err)
		}

		if err := validateStartedAt(startedAt); err != nil {
			return nil, fmt.Errorf("started_at validation failed: %w", err)
		}

		session.StartedAt = startedAt
	}

	if eventCountStr, ok := result["event_count"]; ok {
		fmt.Sscanf(eventCountStr, "%d", &session.EventCount)
	}

	if lastEventAtStr, ok := result["last_event_at"]; ok && lastEventAtStr != "" {
		lastEventAt, _ := time.Parse(time.RFC3339, lastEventAtStr)
		session.LastEventAt = lastEventAt
	}

	return session, nil
}
