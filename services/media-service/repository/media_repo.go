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

// Package repository persists media_objects rows (migration 099).
package repository

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/media-service/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// MediaRepository is the Postgres implementation of handlers.MediaRegistry.
type MediaRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewMediaRepository creates the media_objects repository.
func NewMediaRepository(db *pgxpool.Pool, logger *zap.Logger) *MediaRepository {
	return &MediaRepository{db: db, logger: logger}
}

// Create inserts a registry row. Called at presign time, before the client
// performs the PUT — see migration 099 for why the orphan this can leave
// behind is expected and harmless.
func (r *MediaRepository) Create(ctx context.Context, obj *models.MediaObject) error {
	query := `
		INSERT INTO media_objects (user_id, object_key, filename, content_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	if err := r.db.QueryRow(ctx, query,
		obj.UserID,
		obj.ObjectKey,
		obj.Filename,
		obj.ContentType,
		obj.SizeBytes,
	).Scan(&obj.ID, &obj.CreatedAt); err != nil {
		return fmt.Errorf("insert media_objects: %w", err)
	}
	return nil
}

// CountByUser returns how many media objects the user has registered, for the
// presign-time quota check.
func (r *MediaRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM media_objects WHERE user_id = $1`, userID,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("count media_objects: %w", err)
	}
	return count, nil
}

// ListByUser returns the user's registered media, newest first.
func (r *MediaRepository) ListByUser(ctx context.Context, userID string) ([]models.MediaObject, error) {
	query := `
		SELECT id, user_id, object_key, filename, content_type, size_bytes, created_at
		FROM media_objects
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list media_objects: %w", err)
	}
	defer rows.Close()

	var media []models.MediaObject
	for rows.Next() {
		var obj models.MediaObject
		if err := rows.Scan(&obj.ID, &obj.UserID, &obj.ObjectKey, &obj.Filename, &obj.ContentType, &obj.SizeBytes, &obj.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan media_objects row: %w", err)
		}
		media = append(media, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media_objects rows: %w", err)
	}
	return media, nil
}

// DeleteByOwner deletes the row only when it belongs to userID, and reports
// whether such a row existed. The owner check is in the WHERE clause, not in
// application code, so it cannot race with a concurrent insert of the same
// key by a different user.
func (r *MediaRepository) DeleteByOwner(ctx context.Context, userID, objectKey string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM media_objects WHERE user_id = $1 AND object_key = $2`, userID, objectKey)
	if err != nil {
		return false, fmt.Errorf("delete media_objects: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
