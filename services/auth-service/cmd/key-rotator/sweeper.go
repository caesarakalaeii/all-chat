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
	"encoding/base64"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// SweeperMetrics tracks per-table results during a sweep run.
// D-06: telemetry requirement — each table sweep emits rows_scanned,
// rows_re_encrypted, rows_skipped, errors, and current_kid via zap.
type SweeperMetrics struct {
	RowsScanned     map[string]int64
	RowsReEncrypted map[string]int64
	RowsSkipped     map[string]int64
	Errors          map[string]int64
}

// NewSweeperMetrics initialises all maps.
func NewSweeperMetrics() *SweeperMetrics {
	return &SweeperMetrics{
		RowsScanned:     make(map[string]int64),
		RowsReEncrypted: make(map[string]int64),
		RowsSkipped:     make(map[string]int64),
		Errors:          make(map[string]int64),
	}
}

// Sweeper re-encrypts ciphertext columns to the current MultiKeyEncryptor kid.
// D-03: idempotent — rows already at CurrentKid() are skipped on re-run.
type Sweeper struct {
	pool       *pgxpool.Pool
	encryptor  *encryption.MultiKeyEncryptor
	dryRun     bool
	batchSize  int
	batchDelay time.Duration
	logger     *zap.Logger
	metrics    *SweeperMetrics
	skipTables map[string]bool
}

// SweeperOption is a functional option for NewSweeper.
type SweeperOption func(*Sweeper)

// WithDryRun enables dry-run mode: rows are not mutated but counts are still recorded.
func WithDryRun(v bool) SweeperOption { return func(s *Sweeper) { s.dryRun = v } }

// WithBatchSize sets the number of rows per UPDATE batch.
func WithBatchSize(n int) SweeperOption { return func(s *Sweeper) { s.batchSize = n } }

// WithBatchDelay sets the sleep duration between batches to throttle DB load (D-05).
func WithBatchDelay(d time.Duration) SweeperOption { return func(s *Sweeper) { s.batchDelay = d } }

// WithSkipTable adds a table to the skip list.
func WithSkipTable(name string) SweeperOption {
	return func(s *Sweeper) { s.skipTables[name] = true }
}

// NewSweeper constructs a Sweeper with sensible defaults (batch 100, 50ms delay).
func NewSweeper(pool *pgxpool.Pool, encryptor *encryption.MultiKeyEncryptor, logger *zap.Logger, opts ...SweeperOption) *Sweeper {
	s := &Sweeper{
		pool:       pool,
		encryptor:  encryptor,
		logger:     logger,
		batchSize:  100,
		batchDelay: 50 * time.Millisecond,
		metrics:    NewSweeperMetrics(),
		skipTables: make(map[string]bool),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SweepAll runs all per-table sweeps in order.
// Per-row errors are logged and counted; a DB-level error aborts and returns immediately.
func (s *Sweeper) SweepAll(ctx context.Context) error {
	sweeps := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"users", s.sweepUsers},
		{"viewer_sessions", s.sweepViewerSessions},
		{"youtube_oauth_tokens", s.sweepYouTubeOAuthTokens},
		{"overlay_tts_configs", s.sweepOverlayTTSConfigs},
		{"kick_oauth_tokens", s.sweepKickOAuthTokens},
		{"tiktok_oauth_tokens", s.sweepTikTokOAuthTokens},
	}
	for _, sw := range sweeps {
		if s.skipTables[sw.name] {
			s.logger.Info("skipping table (--skip-table)", zap.String("table", sw.name))
			continue
		}
		s.logger.Info("sweeping table",
			zap.String("table", sw.name),
			zap.Bool("dry_run", s.dryRun),
			zap.Uint8("current_kid", s.encryptor.CurrentKid()),
		)
		if err := sw.fn(ctx); err != nil {
			s.logger.Error("table sweep aborted", zap.String("table", sw.name), zap.Error(err))
			return fmt.Errorf("sweep %s: %w", sw.name, err)
		}
		s.logger.Info("table sweep complete",
			zap.String("table", sw.name),
			zap.Int64("rows_scanned", s.metrics.RowsScanned[sw.name]),
			zap.Int64("rows_re_encrypted", s.metrics.RowsReEncrypted[sw.name]),
			zap.Int64("rows_skipped", s.metrics.RowsSkipped[sw.name]),
			zap.Int64("errors", s.metrics.Errors[sw.name]),
			zap.Uint8("current_kid", s.encryptor.CurrentKid()),
		)
	}
	return nil
}

// encryptIfNotCurrentKid is the per-row idempotency helper (D-03).
//
// Returns:
//
//	(newBlob, true, nil)   — re-encrypted to current kid; caller should write back.
//	(input, false, nil)    — already at current kid (or empty); caller skips.
//	("", false, err)       — decryption failed; caller logs and skips row.
func (s *Sweeper) encryptIfNotCurrentKid(stored string) (string, bool, error) {
	if stored == "" {
		return "", false, nil
	}
	// Fast-path: check if the stored blob is already at the current kid.
	// minVersionedBlobLen = 1(kid) + 12(nonce) + 16(tag) = 29 bytes raw → decoded len >= 29.
	decoded, err := base64.StdEncoding.DecodeString(stored)
	if err == nil && len(decoded) >= 29 && decoded[0] == s.encryptor.CurrentKid() {
		return stored, false, nil // already on current kid
	}
	// Decrypt using any key in the chain (versioned or legacy).
	plaintext, err := s.encryptor.DecryptString(stored)
	if err != nil {
		return "", false, fmt.Errorf("decrypt: %w", err)
	}
	// Re-encrypt under the current kid.
	reencrypted, err := s.encryptor.EncryptString(plaintext)
	if err != nil {
		return "", false, fmt.Errorf("re-encrypt: %w", err)
	}
	return reencrypted, true, nil
}

// --- users ---

type userUpdate struct {
	ID          string
	AccessToken string
	RefreshToken string
}

func (s *Sweeper) sweepUsers(ctx context.Context) error {
	const query = `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,'') FROM users ORDER BY id`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var batch []userUpdate
	for rows.Next() {
		var id, at, rt string
		if err := rows.Scan(&id, &at, &rt); err != nil {
			return fmt.Errorf("scan users: %w", err)
		}
		s.metrics.RowsScanned["users"]++

		newAt, atChanged, err := s.encryptIfNotCurrentKid(at)
		if err != nil {
			s.logger.Warn("user access_token re-encrypt error",
				zap.String("user_id", id),
				zap.Error(err),
			)
			s.metrics.Errors["users"]++
			continue
		}
		newRt, rtChanged, err := s.encryptIfNotCurrentKid(rt)
		if err != nil {
			s.logger.Warn("user refresh_token re-encrypt error",
				zap.String("user_id", id),
				zap.Error(err),
			)
			s.metrics.Errors["users"]++
			continue
		}
		if !atChanged && !rtChanged {
			s.metrics.RowsSkipped["users"]++
			continue
		}
		batch = append(batch, userUpdate{id, newAt, newRt})
		if len(batch) >= s.batchSize {
			if err := s.flushUsersBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
			if s.batchDelay > 0 {
				time.Sleep(s.batchDelay)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate users: %w", err)
	}
	if len(batch) > 0 {
		return s.flushUsersBatch(ctx, batch)
	}
	return nil
}

func (s *Sweeper) flushUsersBatch(ctx context.Context, batch []userUpdate) error {
	if s.dryRun {
		s.metrics.RowsReEncrypted["users"] += int64(len(batch))
		return nil
	}
	b := &pgx.Batch{}
	for _, u := range batch {
		b.Queue(
			`UPDATE users SET access_token=$1, refresh_token=$2, updated_at=NOW() WHERE id=$3`,
			u.AccessToken, u.RefreshToken, u.ID,
		)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range batch {
		if _, err := br.Exec(); err != nil {
			s.metrics.Errors["users"]++
			s.logger.Error("users batch update failed", zap.Error(err))
			return fmt.Errorf("flush users batch: %w", err)
		}
		s.metrics.RowsReEncrypted["users"]++
	}
	return nil
}

// --- viewer_sessions ---

type viewerSessionUpdate struct {
	ID           string
	AccessToken  string
	RefreshToken string
}

func (s *Sweeper) sweepViewerSessions(ctx context.Context) error {
	const query = `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,'') FROM viewer_sessions ORDER BY id`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query viewer_sessions: %w", err)
	}
	defer rows.Close()

	var batch []viewerSessionUpdate
	for rows.Next() {
		var id, at, rt string
		if err := rows.Scan(&id, &at, &rt); err != nil {
			return fmt.Errorf("scan viewer_sessions: %w", err)
		}
		s.metrics.RowsScanned["viewer_sessions"]++

		newAt, atChanged, err := s.encryptIfNotCurrentKid(at)
		if err != nil {
			s.logger.Warn("viewer_session access_token re-encrypt error",
				zap.String("session_id", id),
				zap.Error(err),
			)
			s.metrics.Errors["viewer_sessions"]++
			continue
		}
		newRt, rtChanged, err := s.encryptIfNotCurrentKid(rt)
		if err != nil {
			s.logger.Warn("viewer_session refresh_token re-encrypt error",
				zap.String("session_id", id),
				zap.Error(err),
			)
			s.metrics.Errors["viewer_sessions"]++
			continue
		}
		if !atChanged && !rtChanged {
			s.metrics.RowsSkipped["viewer_sessions"]++
			continue
		}
		batch = append(batch, viewerSessionUpdate{id, newAt, newRt})
		if len(batch) >= s.batchSize {
			if err := s.flushViewerSessionsBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
			if s.batchDelay > 0 {
				time.Sleep(s.batchDelay)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate viewer_sessions: %w", err)
	}
	if len(batch) > 0 {
		return s.flushViewerSessionsBatch(ctx, batch)
	}
	return nil
}

func (s *Sweeper) flushViewerSessionsBatch(ctx context.Context, batch []viewerSessionUpdate) error {
	if s.dryRun {
		s.metrics.RowsReEncrypted["viewer_sessions"] += int64(len(batch))
		return nil
	}
	b := &pgx.Batch{}
	for _, v := range batch {
		b.Queue(
			`UPDATE viewer_sessions SET access_token=$1, refresh_token=$2, updated_at=NOW() WHERE id=$3`,
			v.AccessToken, v.RefreshToken, v.ID,
		)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range batch {
		if _, err := br.Exec(); err != nil {
			s.metrics.Errors["viewer_sessions"]++
			s.logger.Error("viewer_sessions batch update failed", zap.Error(err))
			return fmt.Errorf("flush viewer_sessions batch: %w", err)
		}
		s.metrics.RowsReEncrypted["viewer_sessions"]++
	}
	return nil
}

// --- youtube_oauth_tokens ---

type youtubeTokenUpdate struct {
	UserID       string
	ChannelID    string
	AccessToken  string
	RefreshToken string
}

func (s *Sweeper) sweepYouTubeOAuthTokens(ctx context.Context) error {
	// Select all rows (including v0 plaintext from before backfill); encryptIfNotCurrentKid
	// handles both plaintext and versioned ciphertext.
	const query = `SELECT user_id, channel_id, COALESCE(access_token,''), COALESCE(refresh_token,''), encryption_version FROM youtube_oauth_tokens ORDER BY user_id, channel_id`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query youtube_oauth_tokens: %w", err)
	}
	defer rows.Close()

	var batch []youtubeTokenUpdate
	for rows.Next() {
		var userID, channelID, at, rt string
		var encVer int
		if err := rows.Scan(&userID, &channelID, &at, &rt, &encVer); err != nil {
			return fmt.Errorf("scan youtube_oauth_tokens: %w", err)
		}
		s.metrics.RowsScanned["youtube_oauth_tokens"]++

		newAt, atChanged, err := s.encryptIfNotCurrentKid(at)
		if err != nil {
			s.logger.Warn("youtube access_token re-encrypt error",
				zap.String("user_id", userID),
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			s.metrics.Errors["youtube_oauth_tokens"]++
			continue
		}
		newRt, rtChanged, err := s.encryptIfNotCurrentKid(rt)
		if err != nil {
			s.logger.Warn("youtube refresh_token re-encrypt error",
				zap.String("user_id", userID),
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			s.metrics.Errors["youtube_oauth_tokens"]++
			continue
		}
		if !atChanged && !rtChanged {
			s.metrics.RowsSkipped["youtube_oauth_tokens"]++
			continue
		}
		batch = append(batch, youtubeTokenUpdate{userID, channelID, newAt, newRt})
		if len(batch) >= s.batchSize {
			if err := s.flushYouTubeBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
			if s.batchDelay > 0 {
				time.Sleep(s.batchDelay)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate youtube_oauth_tokens: %w", err)
	}
	if len(batch) > 0 {
		return s.flushYouTubeBatch(ctx, batch)
	}
	return nil
}

func (s *Sweeper) flushYouTubeBatch(ctx context.Context, batch []youtubeTokenUpdate) error {
	if s.dryRun {
		s.metrics.RowsReEncrypted["youtube_oauth_tokens"] += int64(len(batch))
		return nil
	}
	b := &pgx.Batch{}
	for _, yt := range batch {
		b.Queue(
			`UPDATE youtube_oauth_tokens SET access_token=$1, refresh_token=$2, encryption_version=1, updated_at=NOW() WHERE user_id=$3 AND channel_id=$4`,
			yt.AccessToken, yt.RefreshToken, yt.UserID, yt.ChannelID,
		)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range batch {
		if _, err := br.Exec(); err != nil {
			s.metrics.Errors["youtube_oauth_tokens"]++
			s.logger.Error("youtube_oauth_tokens batch update failed", zap.Error(err))
			return fmt.Errorf("flush youtube batch: %w", err)
		}
		s.metrics.RowsReEncrypted["youtube_oauth_tokens"]++
	}
	return nil
}

// --- overlay_tts_configs (BYTEA — Pitfall 5) ---

// sweepOverlayTTSConfigs handles the BYTEA encrypted_api_key column.
//
// Pitfall 5: encrypted_api_key is stored as BYTEA. Phase 13 stored the raw bytes of the
// base64-encoded ciphertext string directly into BYTEA (i.e., []byte(base64string)).
// The sweeper reads the BYTEA column as []byte, converts to string for encryptIfNotCurrentKid,
// then writes the re-encrypted string back as []byte into BYTEA.
// Do NOT base64-decode the BYTEA value again — it is already a base64 string stored as bytes.
func (s *Sweeper) sweepOverlayTTSConfigs(ctx context.Context) error {
	const query = `SELECT id, encrypted_api_key FROM overlay_tts_configs WHERE encrypted_api_key IS NOT NULL ORDER BY id`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query overlay_tts_configs: %w", err)
	}
	defer rows.Close()

	var batch []ttsBatchItem

	for rows.Next() {
		var id string
		var b []byte
		if err := rows.Scan(&id, &b); err != nil {
			return fmt.Errorf("scan overlay_tts_configs: %w", err)
		}
		s.metrics.RowsScanned["overlay_tts_configs"]++

		// The BYTEA blob holds the base64-encoded ciphertext as a raw byte slice.
		// Convert to string for encryptIfNotCurrentKid.
		stored := string(b)
		newStored, changed, err := s.encryptIfNotCurrentKid(stored)
		if err != nil {
			s.logger.Warn("overlay_tts_configs encrypted_api_key re-encrypt error",
				zap.String("config_id", id),
				zap.Error(err),
			)
			s.metrics.Errors["overlay_tts_configs"]++
			continue
		}
		if !changed {
			s.metrics.RowsSkipped["overlay_tts_configs"]++
			continue
		}
		batch = append(batch, ttsBatchItem{id, []byte(newStored)})
		if len(batch) >= s.batchSize {
			if err := s.flushTTSBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
			if s.batchDelay > 0 {
				time.Sleep(s.batchDelay)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate overlay_tts_configs: %w", err)
	}
	if len(batch) > 0 {
		return s.flushTTSBatch(ctx, batch)
	}
	return nil
}

type ttsBatchItem struct {
	ID    string
	Bytes []byte
}

func (s *Sweeper) flushTTSBatch(ctx context.Context, batch []ttsBatchItem) error {
	if s.dryRun {
		s.metrics.RowsReEncrypted["overlay_tts_configs"] += int64(len(batch))
		return nil
	}
	b := &pgx.Batch{}
	for _, tts := range batch {
		b.Queue(
			`UPDATE overlay_tts_configs SET encrypted_api_key=$1, updated_at=NOW() WHERE id=$2`,
			tts.Bytes, tts.ID,
		)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range batch {
		if _, err := br.Exec(); err != nil {
			s.metrics.Errors["overlay_tts_configs"]++
			s.logger.Error("overlay_tts_configs batch update failed", zap.Error(err))
			return fmt.Errorf("flush tts batch: %w", err)
		}
		s.metrics.RowsReEncrypted["overlay_tts_configs"]++
	}
	return nil
}

// --- kick_oauth_tokens ---

// sweepKickOAuthTokens handles kick_oauth_tokens with two sub-policies:
//
// encryption_version == 0: plaintext written before Plan 14-05 deployed.
// Sweeper ENCRYPTS DIRECTLY (no Decrypt step) with the current kid,
// then updates encryption_version=1.
//
// encryption_version >= 1: versioned ciphertext from Plan 14-05 onward.
// Run encryptIfNotCurrentKid; the helper auto-detects kid and re-encrypts if stale.
type kickTokenUpdate struct {
	ID            string
	AccessToken   string
	RefreshToken  string
	SetEncVersion int
}

func (s *Sweeper) sweepKickOAuthTokens(ctx context.Context) error {
	const query = `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,''), encryption_version FROM kick_oauth_tokens ORDER BY id`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query kick_oauth_tokens: %w", err)
	}
	defer rows.Close()

	var batch []kickTokenUpdate
	for rows.Next() {
		var id, at, rt string
		var encVer int
		if err := rows.Scan(&id, &at, &rt, &encVer); err != nil {
			return fmt.Errorf("scan kick_oauth_tokens: %w", err)
		}
		s.metrics.RowsScanned["kick_oauth_tokens"]++

		if encVer == 0 {
			// v0: plaintext — encrypt directly without a Decrypt step.
			newAt, err := s.encryptor.EncryptString(at)
			if err != nil {
				s.logger.Warn("kick v0 access_token encrypt error",
					zap.String("kick_token_id", id),
					zap.Error(err),
				)
				s.metrics.Errors["kick_oauth_tokens"]++
				continue
			}
			newRt, err := s.encryptor.EncryptString(rt)
			if err != nil {
				s.logger.Warn("kick v0 refresh_token encrypt error",
					zap.String("kick_token_id", id),
					zap.Error(err),
				)
				s.metrics.Errors["kick_oauth_tokens"]++
				continue
			}
			batch = append(batch, kickTokenUpdate{id, newAt, newRt, 1})
		} else {
			// v1+: versioned ciphertext — standard re-encryption check.
			newAt, atChanged, err := s.encryptIfNotCurrentKid(at)
			if err != nil {
				s.logger.Warn("kick access_token re-encrypt error",
					zap.String("kick_token_id", id),
					zap.Error(err),
				)
				s.metrics.Errors["kick_oauth_tokens"]++
				continue
			}
			newRt, rtChanged, err := s.encryptIfNotCurrentKid(rt)
			if err != nil {
				s.logger.Warn("kick refresh_token re-encrypt error",
					zap.String("kick_token_id", id),
					zap.Error(err),
				)
				s.metrics.Errors["kick_oauth_tokens"]++
				continue
			}
			if !atChanged && !rtChanged {
				s.metrics.RowsSkipped["kick_oauth_tokens"]++
				continue
			}
			batch = append(batch, kickTokenUpdate{id, newAt, newRt, encVer})
		}

		if len(batch) >= s.batchSize {
			if err := s.flushKickBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
			if s.batchDelay > 0 {
				time.Sleep(s.batchDelay)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate kick_oauth_tokens: %w", err)
	}
	if len(batch) > 0 {
		return s.flushKickBatch(ctx, batch)
	}
	return nil
}

func (s *Sweeper) flushKickBatch(ctx context.Context, batch []kickTokenUpdate) error {
	if s.dryRun {
		s.metrics.RowsReEncrypted["kick_oauth_tokens"] += int64(len(batch))
		return nil
	}
	b := &pgx.Batch{}
	for _, k := range batch {
		b.Queue(
			`UPDATE kick_oauth_tokens SET access_token=$1, refresh_token=$2, encryption_version=$3, updated_at=NOW() WHERE id=$4`,
			k.AccessToken, k.RefreshToken, k.SetEncVersion, k.ID,
		)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range batch {
		if _, err := br.Exec(); err != nil {
			s.metrics.Errors["kick_oauth_tokens"]++
			s.logger.Error("kick_oauth_tokens batch update failed", zap.Error(err))
			return fmt.Errorf("flush kick batch: %w", err)
		}
		s.metrics.RowsReEncrypted["kick_oauth_tokens"]++
	}
	return nil
}

// --- tiktok_oauth_tokens ---

// tiktokTokenUpdate holds a pending re-encryption for a tiktok_oauth_tokens row.
type tiktokTokenUpdate struct {
	ID           string
	AccessToken  string
	RefreshToken string
}

// sweepTikTokOAuthTokens sweeps tiktok_oauth_tokens.
//
// Per Plan 14-03 deferral (D-17): tiktok_oauth_tokens v0 rows are Node.js plaintext;
// the Node.js tiktok-listener has NOT been migrated to versioned encryption in Phase 14.
// Encrypting these rows would break the running tiktok-listener.
//
// Policy: SQL filter `WHERE encryption_version >= 1` ensures v0 rows are NEVER touched
// by this sweeper. Only rows written by a future Node.js migration (v1+) are re-encrypted.
//
// T-14-06-07: TestSweeper_SkipsTikTokV0 asserts v0 rows are untouched.
func (s *Sweeper) sweepTikTokOAuthTokens(ctx context.Context) error {
	// encryption_version >= 1 only — v0 plaintext rows are skipped by this SQL filter.
	const query = `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,'') FROM tiktok_oauth_tokens WHERE encryption_version >= 1 ORDER BY id`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query tiktok_oauth_tokens: %w", err)
	}
	defer rows.Close()

	var batch []tiktokTokenUpdate
	for rows.Next() {
		var id, at, rt string
		if err := rows.Scan(&id, &at, &rt); err != nil {
			return fmt.Errorf("scan tiktok_oauth_tokens: %w", err)
		}
		s.metrics.RowsScanned["tiktok_oauth_tokens"]++

		newAt, atChanged, err := s.encryptIfNotCurrentKid(at)
		if err != nil {
			s.logger.Warn("tiktok access_token re-encrypt error",
				zap.String("tiktok_token_id", id),
				zap.Error(err),
			)
			s.metrics.Errors["tiktok_oauth_tokens"]++
			continue
		}
		newRt, rtChanged, err := s.encryptIfNotCurrentKid(rt)
		if err != nil {
			s.logger.Warn("tiktok refresh_token re-encrypt error",
				zap.String("tiktok_token_id", id),
				zap.Error(err),
			)
			s.metrics.Errors["tiktok_oauth_tokens"]++
			continue
		}
		if !atChanged && !rtChanged {
			s.metrics.RowsSkipped["tiktok_oauth_tokens"]++
			continue
		}
		batch = append(batch, tiktokTokenUpdate{id, newAt, newRt})
		if len(batch) >= s.batchSize {
			if err := s.flushTikTokBatch(ctx, batch); err != nil {
				return err
			}
			batch = batch[:0]
			if s.batchDelay > 0 {
				time.Sleep(s.batchDelay)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tiktok_oauth_tokens: %w", err)
	}
	if len(batch) > 0 {
		return s.flushTikTokBatch(ctx, batch)
	}
	return nil
}

func (s *Sweeper) flushTikTokBatch(ctx context.Context, batch []tiktokTokenUpdate) error {
	if s.dryRun {
		s.metrics.RowsReEncrypted["tiktok_oauth_tokens"] += int64(len(batch))
		return nil
	}
	b := &pgx.Batch{}
	for _, tt := range batch {
		b.Queue(
			`UPDATE tiktok_oauth_tokens SET access_token=$1, refresh_token=$2, encryption_version=1, updated_at=NOW() WHERE id=$3`,
			tt.AccessToken, tt.RefreshToken, tt.ID,
		)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range batch {
		if _, err := br.Exec(); err != nil {
			s.metrics.Errors["tiktok_oauth_tokens"]++
			s.logger.Error("tiktok_oauth_tokens batch update failed", zap.Error(err))
			return fmt.Errorf("flush tiktok batch: %w", err)
		}
		s.metrics.RowsReEncrypted["tiktok_oauth_tokens"]++
	}
	return nil
}
