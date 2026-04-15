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
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/caesar/all-chat/shared/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
)

type backfillRunner struct {
	pool   *pgxpool.Pool
	cipher crypto.StringCipher
	dryRun bool
}

func main() {
	dryRun := flag.Bool("dry-run", false, "log rows that would be updated without mutating the database")
	skipUsers := flag.Bool("skip-users", false, "skip processing the users table")
	skipYouTube := flag.Bool("skip-youtube", false, "skip processing the youtube_oauth_tokens table")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	encryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("TOKEN_ENCRYPTION_KEY must be set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to create database pool: %v", err)
	}
	defer pool.Close()

	cipher, err := crypto.NewAESGCMCipher(encryptionKey)
	if err != nil {
		log.Fatalf("failed to build cipher: %v", err)
	}

	runner := &backfillRunner{
		pool:   pool,
		cipher: cipher,
		dryRun: *dryRun,
	}

	if !*skipUsers {
		if err := runner.backfillUsers(ctx); err != nil {
			log.Fatalf("users backfill failed: %v", err)
		}
	} else {
		log.Println("Skipping users table")
	}

	if !*skipYouTube {
		if err := runner.backfillYouTube(ctx); err != nil {
			log.Fatalf("youtube_oauth_tokens backfill failed: %v", err)
		}
	} else {
		log.Println("Skipping youtube_oauth_tokens table")
	}

	log.Println("Backfill completed successfully")
}

func (r *backfillRunner) backfillUsers(ctx context.Context) error {
	log.Println("Scanning users table for plaintext tokens")
	rows, err := r.pool.Query(ctx, `
                SELECT id, COALESCE(access_token, ''), COALESCE(refresh_token, '')
                FROM users
                ORDER BY id
        `)
	if err != nil {
		return fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var processed, updated int

	for rows.Next() {
		var (
			id           string
			accessToken  string
			refreshToken string
		)
		if err := rows.Scan(&id, &accessToken, &refreshToken); err != nil {
			return fmt.Errorf("scan user row: %w", err)
		}

		changed := false
		encryptedAccess, accessChanged, err := r.encryptIfPlaintext(accessToken)
		if err != nil {
			return fmt.Errorf("user %s access token: %w", id, err)
		}
		if accessChanged {
			changed = true
		}

		encryptedRefresh, refreshChanged, err := r.encryptIfPlaintext(refreshToken)
		if err != nil {
			return fmt.Errorf("user %s refresh token: %w", id, err)
		}
		if refreshChanged {
			changed = true
		}

		if changed {
			updated++
			if r.dryRun {
				log.Printf("[dry-run] users: would update user_id=%s", id)
				continue
			}

			if _, err := r.pool.Exec(ctx, `
                                UPDATE users
                                SET access_token = $2, refresh_token = $3, updated_at = $4
                                WHERE id = $1
                        `, id, encryptedAccess, encryptedRefresh, time.Now()); err != nil {
				return fmt.Errorf("update user %s: %w", id, err)
			}
		}

		processed++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate users: %w", err)
	}

	log.Printf("users backfill complete: processed=%d updated=%d", processed, updated)
	return nil
}

func (r *backfillRunner) backfillYouTube(ctx context.Context) error {
	log.Println("Scanning youtube_oauth_tokens table for plaintext tokens")
	rows, err := r.pool.Query(ctx, `
                SELECT user_id, channel_id, COALESCE(access_token, ''), COALESCE(refresh_token, '')
                FROM youtube_oauth_tokens
                ORDER BY user_id, channel_id
        `)
	if err != nil {
		return fmt.Errorf("query youtube_oauth_tokens: %w", err)
	}
	defer rows.Close()

	var processed, updated int

	for rows.Next() {
		var (
			userID       string
			channelID    string
			accessToken  string
			refreshToken string
		)
		if err := rows.Scan(&userID, &channelID, &accessToken, &refreshToken); err != nil {
			return fmt.Errorf("scan youtube row: %w", err)
		}

		changed := false
		encryptedAccess, accessChanged, err := r.encryptIfPlaintext(accessToken)
		if err != nil {
			return fmt.Errorf("youtube (%s,%s) access token: %w", userID, channelID, err)
		}
		if accessChanged {
			changed = true
		}

		encryptedRefresh, refreshChanged, err := r.encryptIfPlaintext(refreshToken)
		if err != nil {
			return fmt.Errorf("youtube (%s,%s) refresh token: %w", userID, channelID, err)
		}
		if refreshChanged {
			changed = true
		}

		if changed {
			updated++
			if r.dryRun {
				log.Printf("[dry-run] youtube_oauth_tokens: would update user_id=%s channel_id=%s", userID, channelID)
				continue
			}

			if _, err := r.pool.Exec(ctx, `
                                UPDATE youtube_oauth_tokens
                                SET access_token = $3, refresh_token = $4, updated_at = $5
                                WHERE user_id = $1 AND channel_id = $2
                        `, userID, channelID, encryptedAccess, encryptedRefresh, time.Now()); err != nil {
				return fmt.Errorf("update youtube tokens (%s,%s): %w", userID, channelID, err)
			}
		}

		processed++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate youtube_oauth_tokens: %w", err)
	}

	log.Printf("youtube_oauth_tokens backfill complete: processed=%d updated=%d", processed, updated)
	return nil
}

func (r *backfillRunner) encryptIfPlaintext(token string) (string, bool, error) {
	if token == "" {
		return "", false, nil
	}

	if _, err := r.cipher.Decrypt(token); err == nil {
		return token, false, nil
	}

	encrypted, err := r.cipher.Encrypt(token)
	if err != nil {
		return "", false, err
	}
	return encrypted, true, nil
}
