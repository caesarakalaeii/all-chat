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

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	log := logger.NewLogger("youtube-token-backfill", getEnvOrDefault("LOG_LEVEL", "info"))
	defer log.Sync()

	// Use unified multi-key encryptor (D-04). Reads TOKEN_ENCRYPTION_KEY_V1 plus
	// YOUTUBE_TOKEN_ENCRYPTION_KEY as legacy fallback.
	// NOTE: The new sweeper from Plan 14-06 supersedes this binary. This binary
	// remains compiled for historical reproducibility but is not in the rotation runbook.
	encryptor, err := encryption.NewMultiKeyEncryptorFromEnv()
	if err != nil {
		log.Fatal("Failed to initialize encryptor (TOKEN_ENCRYPTION_KEY_V1 must be set)", zap.Error(err))
	}

	// DATABASE_PASSWORD is required when DATABASE_URL is not set.
	if os.Getenv("DATABASE_URL") == "" && getEnvOrDefault("DATABASE_PASSWORD", "") == "" {
		log.Fatal("DATABASE_PASSWORD must be set (or set DATABASE_URL)")
	}

	connString := buildConnString()

	pool, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	ctx := context.Background()

	total, err := backfillTokens(ctx, pool, encryptor, log)
	if err != nil {
		log.Fatal("Failed to backfill tokens", zap.Error(err))
	}

	log.Info("Token backfill complete", zap.Int("rows_encrypted", total))
}

func backfillTokens(ctx context.Context, pool *pgxpool.Pool, encryptor *encryption.MultiKeyEncryptor, log *zap.Logger) (int, error) {
	rows, err := pool.Query(ctx, `
        SELECT user_id, channel_id, access_token, refresh_token
        FROM youtube_oauth_tokens
        WHERE encryption_version = 0
        ORDER BY updated_at ASC
    `)
	if err != nil {
		return 0, fmt.Errorf("query tokens: %w", err)
	}
	defer rows.Close()

	processed := 0

	batch := &pgx.Batch{}

	for rows.Next() {
		var userID, channelID, accessToken, refreshToken string
		if err := rows.Scan(&userID, &channelID, &accessToken, &refreshToken); err != nil {
			return processed, fmt.Errorf("scan token: %w", err)
		}

		encryptedAccess, err := encryptor.EncryptString(accessToken)
		if err != nil {
			return processed, fmt.Errorf("encrypt access token: %w", err)
		}

		encryptedRefresh, err := encryptor.EncryptString(refreshToken)
		if err != nil {
			return processed, fmt.Errorf("encrypt refresh token: %w", err)
		}

		batch.Queue(`
            UPDATE youtube_oauth_tokens
            SET access_token = $1,
                refresh_token = $2,
                encryption_version = 1,
                updated_at = $3
            WHERE user_id = $4 AND channel_id = $5
        `, encryptedAccess, encryptedRefresh, time.Now().UTC(), userID, channelID)

		processed++
	}

	if err := rows.Err(); err != nil {
		return processed, fmt.Errorf("iterate tokens: %w", err)
	}

	if processed == 0 {
		return 0, nil
	}

	br := pool.SendBatch(ctx, batch)
	if err := br.Close(); err != nil {
		return processed, fmt.Errorf("apply batch: %w", err)
	}

	log.Info("Updated legacy tokens", zap.Int("rows", processed))
	return processed, nil
}

func buildConnString() string {
	if conn := os.Getenv("DATABASE_URL"); conn != "" {
		return conn
	}

	host := getEnvOrDefault("DATABASE_HOST", "localhost")
	port := getEnvOrDefault("DATABASE_PORT", "5432")
	user := getEnvOrDefault("DATABASE_USER", "allchat")
	password := getEnvOrDefault("DATABASE_PASSWORD", "")
	dbName := getEnvOrDefault("DATABASE_NAME", "allchat")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, dbName)
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
