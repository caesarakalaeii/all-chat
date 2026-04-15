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
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// UserSearchRepository handles user search operations
type UserSearchRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewUserSearchRepository creates a new user search repository
func NewUserSearchRepository(db *pgxpool.Pool, logger *zap.Logger) *UserSearchRepository {
	return &UserSearchRepository{db: db, logger: logger}
}

// UserSearchResult represents a user search result
type UserSearchResult struct {
	ID              string `json:"id"`
	Username        string `json:"username"`
	DisplayName     string `json:"display_name"`
	ProfileImageURL string `json:"profile_image_url"`
}

// SearchUsersByPlatform searches for users by platform and username query
// Uses LOWER() to leverage the functional index from migration 028
func (r *UserSearchRepository) SearchUsersByPlatform(
	ctx context.Context,
	platform string, // "twitch", "youtube", "kick", "tiktok"
	query string,
	limit int,
) ([]UserSearchResult, error) {
	// Validate inputs
	if query == "" {
		return []UserSearchResult{}, nil
	}

	// ILIKE pattern for partial matching (case-insensitive)
	likePattern := "%" + query + "%"

	// Platform-specific filtering
	platformCondition := ""
	switch platform {
	case "twitch":
		platformCondition = "AND twitch_id IS NOT NULL"
	case "youtube":
		platformCondition = "AND google_id IS NOT NULL"
	case "kick":
		platformCondition = "AND kick_id IS NOT NULL"
	case "tiktok":
		platformCondition = "AND tiktok_id IS NOT NULL"
	default:
		return nil, fmt.Errorf("invalid platform: %s", platform)
	}

	// Use LOWER() to leverage functional index from migration 028
	sqlQuery := fmt.Sprintf(`
		SELECT id, username, display_name, profile_image_url
		FROM users
		WHERE LOWER(username) LIKE LOWER($1)
		%s
		ORDER BY username
		LIMIT $2
	`, platformCondition)

	rows, err := r.db.Query(ctx, sqlQuery, likePattern, limit)
	if err != nil {
		r.logger.Error("User search query failed",
			zap.String("platform", platform),
			zap.String("query", query),
			zap.Error(err))
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []UserSearchResult
	for rows.Next() {
		var user UserSearchResult
		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &user.ProfileImageURL); err != nil {
			r.logger.Error("Failed to scan user row", zap.Error(err))
			continue
		}
		results = append(results, user)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating user rows", zap.Error(err))
		return nil, fmt.Errorf("failed to iterate results: %w", err)
	}

	r.logger.Debug("User search completed",
		zap.String("platform", platform),
		zap.String("query", query),
		zap.Int("result_count", len(results)))

	return results, nil
}
