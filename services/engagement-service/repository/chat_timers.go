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

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrDuplicateTimerName is returned when the user already has a timer with the same
// name (case-insensitive).
var ErrDuplicateTimerName = errors.New("a chat timer with this name already exists")

const chatTimerCols = `id, user_id, name, message, interval_minutes, min_messages, enabled,
	last_fired_at, lines_at_last_fire, fire_count, created_at, updated_at`

// mapChatTimerErr turns the DB rejections the handler can name into sentinels; the
// handler's validation mirrors the CHECK constraints, so a well-validated request
// hits none of these.
func mapChatTimerErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicateTimerName
	}
	return fmt.Errorf("save chat timer: %w", err)
}

// scanChatTimer scans chatTimerCols, after any leading columns into lead.
func scanChatTimer(row pgx.Row, lead ...any) (models.ChatTimer, error) {
	var t models.ChatTimer
	err := row.Scan(append(lead, &t.ID, &t.UserID, &t.Name, &t.Message, &t.IntervalMinutes,
		&t.MinMessages, &t.Enabled, &t.LastFiredAt, &t.LinesAtLastFire, &t.FireCount,
		&t.CreatedAt, &t.UpdatedAt)...)
	return t, err
}

func collectChatTimers(rows pgx.Rows) ([]models.ChatTimer, error) {
	defer rows.Close()
	out := []models.ChatTimer{}
	for rows.Next() {
		t, err := scanChatTimer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chat timer: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListChatTimers returns every timer of a user, oldest first.
func (r *Repository) ListChatTimers(ctx context.Context, userID uuid.UUID) ([]models.ChatTimer, error) {
	rows, err := r.db.Query(ctx, `SELECT `+chatTimerCols+` FROM chat_timers
		WHERE user_id = $1 ORDER BY created_at, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list chat timers: %w", err)
	}
	return collectChatTimers(rows)
}

// CountChatTimers returns how many timers a user has (the per-user cap).
func (r *Repository) CountChatTimers(ctx context.Context, userID uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRow(ctx, `SELECT count(*) FROM chat_timers WHERE user_id = $1`, userID).Scan(&n)
	return n, err
}

// CreateChatTimer inserts t for t.UserID. last_fired_at starts the first period.
func (r *Repository) CreateChatTimer(ctx context.Context, t models.ChatTimer) (models.ChatTimer, error) {
	out, err := scanChatTimer(r.db.QueryRow(ctx, `INSERT INTO chat_timers
		(user_id, name, message, interval_minutes, min_messages, enabled, last_fired_at)
		VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $6 THEN now() ELSE NULL END)
		RETURNING `+chatTimerCols,
		t.UserID, t.Name, t.Message, t.IntervalMinutes, t.MinMessages, t.Enabled))
	return out, mapChatTimerErr(err)
}

// UpdateChatTimer replaces the editable fields of a timer the user owns. Re-enabling
// restarts the first period; disabling clears the lines baseline with it. ErrNotFound
// when the id does not exist or belongs to someone else.
func (r *Repository) UpdateChatTimer(ctx context.Context, t models.ChatTimer) (models.ChatTimer, error) {
	out, err := scanChatTimer(r.db.QueryRow(ctx, `UPDATE chat_timers SET
		name = $3, message = $4, interval_minutes = $5, min_messages = $6, enabled = $7,
		last_fired_at = CASE WHEN $7 THEN now() ELSE NULL END,
		lines_at_last_fire = 0, updated_at = now()
		WHERE id = $1 AND user_id = $2 RETURNING `+chatTimerCols,
		t.ID, t.UserID, t.Name, t.Message, t.IntervalMinutes, t.MinMessages, t.Enabled))
	return out, mapChatTimerErr(err)
}

// DeleteChatTimer removes a timer the user owns. ErrNotFound otherwise.
func (r *Repository) DeleteChatTimer(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM chat_timers WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete chat timer: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// EnabledChatTimers returns every enabled timer with its owner's premium flag, one
// query per scheduler tick (ADR-0081).
func (r *Repository) EnabledChatTimers(ctx context.Context) ([]models.EnabledChatTimer, error) {
	// The owner's premium comes from a scalar subquery so chatTimerCols stays
	// unqualified: a JOIN would make id and the timestamps ambiguous.
	rows, err := r.db.Query(ctx, `SELECT (SELECT u.is_premium FROM users u WHERE u.id = chat_timers.user_id), `+chatTimerCols+`
		FROM chat_timers WHERE enabled ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("enabled chat timers: %w", err)
	}
	defer rows.Close()
	var out []models.EnabledChatTimer
	for rows.Next() {
		var premium bool
		timer, err := scanChatTimer(rows, &premium)
		if err != nil {
			return nil, fmt.Errorf("scan enabled chat timer: %w", err)
		}
		out = append(out, models.EnabledChatTimer{Timer: timer, Premium: premium})
	}
	return out, rows.Err()
}

// ClaimChatTimer fires one timer exactly once out of N replicas racing the same tick:
// the WHERE sees only the last_fired_at the caller decided on, so the first claim wins
// and every other replica's UPDATE returns nothing.
func (r *Repository) ClaimChatTimer(ctx context.Context, t models.ChatTimer, lines int64) bool {
	tag, err := r.db.Exec(ctx, `UPDATE chat_timers SET
		last_fired_at = now(), lines_at_last_fire = $2, fire_count = fire_count + 1, updated_at = now()
		WHERE id = $1 AND last_fired_at IS NOT DISTINCT FROM $3`, t.ID, lines, t.LastFiredAt)
	if err != nil {
		return false
	}
	return tag.RowsAffected() == 1
}

// OwnerIsPremium reads users.is_premium for the timer scheduler: YouTube sends spend
// Data API quota, so only a premium owner's timer asks YouTube. ErrNotFound mirrors
// the other owner reads.
func (r *Repository) OwnerIsPremium(ctx context.Context, userID uuid.UUID) (bool, error) {
	var premium bool
	err := r.db.QueryRow(ctx, `SELECT is_premium FROM users WHERE id = $1`, userID).Scan(&premium)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	return premium, err
}

// Sender values of chat_bot_settings (AuthBot's migration 115; engagement only
// reads/writes the switch, auth-service resolves which account posts per send).
const (
	SenderAllChatBot = "bot"
	SenderSelf       = "self"
)

// GetChatBotSetting reads the sender a streamer chose for their command replies and
// timer posts. No row (never configured) also reads back as the All-Chat Bot.
func (r *Repository) GetChatBotSetting(ctx context.Context, userID uuid.UUID) (string, error) {
	var s string
	err := r.db.QueryRow(ctx, `SELECT sender FROM chat_bot_settings WHERE user_id = $1`, userID).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return SenderAllChatBot, nil
	}
	if err != nil {
		return SenderAllChatBot, fmt.Errorf("chat bot setting: %w", err)
	}
	return s, nil
}

// SetChatBotSetting upserts the sender one streamer uses.
func (r *Repository) SetChatBotSetting(ctx context.Context, userID uuid.UUID, sender string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO chat_bot_settings (user_id, sender)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET sender = $2, updated_at = now()`,
		userID, sender)
	return err
}
