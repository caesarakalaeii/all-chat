---
phase: 14-secret-rotation-infrastructure
plan: 06
type: execute
wave: 2
depends_on:
  - "14-01"
  - "14-03"
files_modified:
  - services/auth-service/cmd/key-rotator/main.go
  - services/auth-service/cmd/key-rotator/main_test.go
  - services/auth-service/cmd/key-rotator/sweeper.go
  - services/auth-service/cmd/key-rotator/sweeper_test.go
autonomous: true
decisions_addressed:
  - D-03
  - D-06
must_haves:
  truths:
    - "key-rotator binary builds and runs to completion as a Job"
    - "Sweeper iterates users, viewer_sessions, youtube_oauth_tokens, overlay_tts_configs, kick_oauth_tokens (and skips tiktok_oauth_tokens v0 rows)"
    - "Sweeper is idempotent — second run re-encrypts 0 rows on rows already at CurrentKid()"
    - "--dry-run flag logs would-update counts without mutating the DB"
    - "Per-batch throttling (--batch-delay-ms) prevents DB saturation"
    - "Telemetry: rows_scanned, rows_re_encrypted, rows_skipped, errors per table — emitted as zap structured logs"
    - "overlay_tts_configs.encrypted_api_key (BYTEA) handled with binary scan, not TEXT scan (Pitfall 5)"
  artifacts:
    - path: "services/auth-service/cmd/key-rotator/main.go"
      provides: "main() — flag parsing, env loading, DB pool, runs Sweeper"
    - path: "services/auth-service/cmd/key-rotator/sweeper.go"
      provides: "Sweeper type with per-table sweep methods, encryptIfNotCurrentKid helper, SweeperMetrics struct"
    - path: "services/auth-service/cmd/key-rotator/sweeper_test.go"
      provides: "TestSweeper_Idempotent, TestSweeper_SkipsCurrentKid, TestSweeper_HandlesDecryptError, TestSweeper_DryRun, TestSweeper_TTSBytea, TestSweeper_Telemetry"
  key_links:
    - from: "services/auth-service/cmd/key-rotator/sweeper.go"
      to: "shared/encryption.MultiKeyEncryptor"
      via: "calls .CurrentKid(), .EncryptString(), .DecryptString() to migrate ciphertext to current kid"
      pattern: "encryptor.CurrentKid\\(\\)"
    - from: "services/auth-service/cmd/key-rotator/main.go"
      to: "services/auth-service/cmd/token-encryption-backfill/main.go"
      via: "structurally analogous — same pgxpool/cipher/dryRun/iterate-tables shape (replaces v0→v1; backfill was plaintext→v0)"
      pattern: "encryptIfNotCurrentKid"
---

<objective>
Build the K8s-Job-ready sweeper binary that re-encrypts existing ciphertext to the current `MultiKeyEncryptor.CurrentKid()`. This is the long-tail completion path for D-03 (lazy on next write + scheduled background sweeper) and D-06 (sweeper as its own lightweight process).

Purpose: Implements decisions D-03 (re-encryption strategy) and D-06 (sweeper as cmd binary).

Output:
- `services/auth-service/cmd/key-rotator/main.go` — entry point with flags `--dry-run`, `--batch-size`, `--batch-delay-ms`, `--skip-tiktok` (default true since v0 plaintext can't be migrated without Node.js), `--skip-table` (repeatable).
- `services/auth-service/cmd/key-rotator/sweeper.go` — Sweeper type with per-table methods.
- Unit tests covering idempotency, BYTEA handling, telemetry, dry-run, decrypt-error skip-and-log.

The sweeper does NOT touch deployment YAML — that's Plan 14-07. The sweeper does NOT need to be deployed to production during Phase 14 — the operator runs it as a Job during the rotation runbook.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.planning/phases/14-secret-rotation-infrastructure/14-CONTEXT.md
@.planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md
@.planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md
@.planning/phases/14-secret-rotation-infrastructure/14-01-SUMMARY.md
@.planning/phases/14-secret-rotation-infrastructure/14-03-SUMMARY.md
@shared/encryption/versioned.go
@services/auth-service/cmd/token-encryption-backfill/main.go

<interfaces>
<!-- The MultiKeyEncryptor methods the sweeper calls. -->

From shared/encryption/versioned.go (Plan 14-01):
```go
func (m *MultiKeyEncryptor) CurrentKid() KidByte
func (m *MultiKeyEncryptor) EncryptString(plaintext string) (string, error)
func (m *MultiKeyEncryptor) DecryptString(ciphertext string) (string, error)
func NewMultiKeyEncryptorFromEnv() (*MultiKeyEncryptor, error)
```

The token-encryption-backfill structural template:
```go
// services/auth-service/cmd/token-encryption-backfill/main.go (post-Plan-14-01 migration)
type backfillRunner struct {
    pool   *pgxpool.Pool
    cipher *encryption.AESEncryptor // <-- Plan 14-06 changes to *MultiKeyEncryptor
    dryRun bool
}
func (r *backfillRunner) backfillUsers(ctx context.Context) error { /* SELECT .. FOR UPDATE batch + UPDATE */ }
func (r *backfillRunner) encryptIfPlaintext(token string) (string, bool, error) { /* try Decrypt — if works, already encrypted; else encrypt */ }
```

Tables (per RESEARCH.md §6 sweep order):
- users (access_token TEXT, refresh_token TEXT)
- viewer_sessions (access_token TEXT, refresh_token TEXT)
- youtube_oauth_tokens (access_token TEXT, refresh_token TEXT, encryption_version SMALLINT)
- overlay_tts_configs (encrypted_api_key BYTEA)
- kick_oauth_tokens (access_token TEXT, refresh_token TEXT, encryption_version SMALLINT) — only if encryption_version >= 1
- tiktok_oauth_tokens (access_token TEXT, refresh_token TEXT, encryption_version SMALLINT) — SKIP (v0 plaintext, Node.js not migrated)
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Implement Sweeper type with per-table sweep methods + idempotency helper</name>
  <files>services/auth-service/cmd/key-rotator/sweeper.go, services/auth-service/cmd/key-rotator/sweeper_test.go</files>
  <read_first>
    - services/auth-service/cmd/token-encryption-backfill/main.go (post-Plan-14-01 — full file; the structural analog)
    - shared/encryption/versioned.go (Plan 14-01 output)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §6 "Sweeper Design" (lines 478–569) and Pitfall 5 "Sweeper Re-Encrypts TTS encrypted_api_key (BYTEA)" (lines 924–927)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "services/auth-service/cmd/key-rotator/main.go" section (lines 239–315)
    - .planning/phases/14-secret-rotation-infrastructure/14-VALIDATION.md table rows for D-03/D-06 and D-06 (lines 55–56)
    - migrations/006_youtube_token_encryption.sql, migrations/049_overlay_tts_configs.sql, migrations/050_kick_token_encryption.sql, migrations/051_tiktok_token_encryption.sql (column shapes)
  </read_first>
  <behavior>
    - TestSweeper_EncryptIfNotCurrentKid_AlreadyCurrent: blob with kid==CurrentKid() → returns (input, false, nil) without re-encrypting.
    - TestSweeper_EncryptIfNotCurrentKid_OldKid: blob with kid==0x01, CurrentKid()==0x02 → returns (newBlob, true, nil); decrypting newBlob yields the original plaintext; newBlob has kid prefix 0x02.
    - TestSweeper_EncryptIfNotCurrentKid_LegacyKidless: blob from legacy AESEncryptor (kid-less) → re-encrypted to current kid; returns (newBlob, true, nil).
    - TestSweeper_EncryptIfNotCurrentKid_DecryptFails: blob is garbage / valid-but-no-key-matches → returns ("", false, error); caller is responsible for skipping the row and incrementing error counter.
    - TestSweeper_SweepUsers_Idempotent: insert 3 users via mocked pool — 1 already-current, 1 old-kid, 1 legacy; sweep → exactly 2 UPDATEs issued; metrics.RowsReEncrypted["users"]==2; RowsSkipped["users"]==1.
    - TestSweeper_SweepUsers_DryRun: dryRun=true; sweep → 0 UPDATEs issued; metrics still report what WOULD have been re-encrypted.
    - TestSweeper_SweepTTSBytea: insert overlay_tts_configs row with encrypted_api_key as a BYTEA blob (legacy AESEncryptor output, base64-decoded into raw bytes); sweep using BYTEA-aware scan; row updated to versioned blob.
    - TestSweeper_HandlesDecryptError: insert a row whose access_token is corrupted (not valid base64); sweep → error logged via zap, row NOT touched, sweeper continues to next row.
    - TestSweeper_Telemetry: assert metrics.RowsScanned, RowsReEncrypted, RowsSkipped, Errors are populated correctly across 3 tables.
    - TestSweeper_SkipsTikTokV0: insert a tiktok_oauth_tokens row with encryption_version=0; sweep tiktok → row is NOT touched (skipped because v0 means Node.js plaintext); RowsSkipped["tiktok_oauth_tokens"]==1.
  </behavior>
  <action>
    Step 1 — Create `services/auth-service/cmd/key-rotator/sweeper.go`:

    ```go
    // Copyright header (AGPL, copy from existing files)
    package main

    import (
        "context"
        "encoding/base64"
        "errors"
        "fmt"
        "time"

        "github.com/caesar/all-chat/shared/encryption"
        "github.com/jackc/pgx/v5"
        "github.com/jackc/pgx/v5/pgxpool"
        "go.uber.org/zap"
    )

    // SweeperMetrics tracks per-table results during a sweep run.
    type SweeperMetrics struct {
        RowsScanned     map[string]int64
        RowsReEncrypted map[string]int64
        RowsSkipped     map[string]int64
        Errors          map[string]int64
    }

    func NewSweeperMetrics() *SweeperMetrics {
        return &SweeperMetrics{
            RowsScanned:     make(map[string]int64),
            RowsReEncrypted: make(map[string]int64),
            RowsSkipped:     make(map[string]int64),
            Errors:          make(map[string]int64),
        }
    }

    // Sweeper re-encrypts ciphertext columns to the current MultiKeyEncryptor kid.
    type Sweeper struct {
        pool         *pgxpool.Pool
        encryptor    *encryption.MultiKeyEncryptor
        dryRun       bool
        batchSize    int
        batchDelay   time.Duration
        logger       *zap.Logger
        metrics      *SweeperMetrics
        skipTables   map[string]bool
        skipTikTokV0 bool // default true — tiktok_oauth_tokens v0 rows are Node.js plaintext, can't be migrated yet
    }

    func NewSweeper(pool *pgxpool.Pool, encryptor *encryption.MultiKeyEncryptor, logger *zap.Logger, opts ...SweeperOption) *Sweeper {
        s := &Sweeper{
            pool:         pool,
            encryptor:    encryptor,
            logger:       logger,
            batchSize:    100,
            batchDelay:   50 * time.Millisecond,
            metrics:      NewSweeperMetrics(),
            skipTables:   map[string]bool{},
            skipTikTokV0: true,
        }
        for _, opt := range opts { opt(s) }
        return s
    }

    type SweeperOption func(*Sweeper)
    func WithDryRun(v bool) SweeperOption     { return func(s *Sweeper) { s.dryRun = v } }
    func WithBatchSize(n int) SweeperOption   { return func(s *Sweeper) { s.batchSize = n } }
    func WithBatchDelay(d time.Duration) SweeperOption { return func(s *Sweeper) { s.batchDelay = d } }
    func WithSkipTable(name string) SweeperOption     { return func(s *Sweeper) { s.skipTables[name] = true } }

    // SweepAll runs all per-table sweeps in order. Errors per row are logged and
    // counted; a fatal sweep error (DB unavailable) returns immediately.
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
                s.logger.Info("skipping table", zap.String("table", sw.name))
                continue
            }
            s.logger.Info("sweeping table", zap.String("table", sw.name), zap.Bool("dry_run", s.dryRun))
            if err := sw.fn(ctx); err != nil {
                s.logger.Error("table sweep aborted", zap.String("table", sw.name), zap.Error(err))
                return fmt.Errorf("sweep %s: %w", sw.name, err)
            }
            s.logger.Info("table sweep complete",
                zap.String("table", sw.name),
                zap.Int64("scanned", s.metrics.RowsScanned[sw.name]),
                zap.Int64("re_encrypted", s.metrics.RowsReEncrypted[sw.name]),
                zap.Int64("skipped", s.metrics.RowsSkipped[sw.name]),
                zap.Int64("errors", s.metrics.Errors[sw.name]),
            )
        }
        return nil
    }

    // encryptIfNotCurrentKid is the per-row idempotency helper. Returns:
    //   (newBlob, true, nil)  — re-encrypted; caller should write back
    //   (input, false, nil)   — already on current kid OR empty; caller skips
    //   ("", false, err)      — decryption failed; caller logs and continues
    func (s *Sweeper) encryptIfNotCurrentKid(stored string) (string, bool, error) {
        if stored == "" { return "", false, nil }
        decoded, err := base64.StdEncoding.DecodeString(stored)
        if err == nil && len(decoded) >= 1+12+16 && decoded[0] == s.encryptor.CurrentKid() {
            return stored, false, nil // already on current kid
        }
        plaintext, err := s.encryptor.DecryptString(stored)
        if err != nil {
            return "", false, fmt.Errorf("decrypt: %w", err)
        }
        reencrypted, err := s.encryptor.EncryptString(plaintext)
        if err != nil {
            return "", false, fmt.Errorf("re-encrypt: %w", err)
        }
        return reencrypted, true, nil
    }

    // sweepUsers iterates users.access_token and users.refresh_token in batches.
    func (s *Sweeper) sweepUsers(ctx context.Context) error {
        const sel = `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,'') FROM users ORDER BY id`
        rows, err := s.pool.Query(ctx, sel)
        if err != nil { return err }
        defer rows.Close()
        var batch []userUpdate
        for rows.Next() {
            var id, at, rt string
            if err := rows.Scan(&id, &at, &rt); err != nil { return err }
            s.metrics.RowsScanned["users"]++
            newAt, atChanged, err := s.encryptIfNotCurrentKid(at)
            if err != nil {
                s.logger.Warn("user access_token sweep error", zap.String("user_id", id), zap.Error(err))
                s.metrics.Errors["users"]++
                continue
            }
            newRt, rtChanged, err := s.encryptIfNotCurrentKid(rt)
            if err != nil {
                s.logger.Warn("user refresh_token sweep error", zap.String("user_id", id), zap.Error(err))
                s.metrics.Errors["users"]++
                continue
            }
            if !atChanged && !rtChanged {
                s.metrics.RowsSkipped["users"]++
                continue
            }
            batch = append(batch, userUpdate{id, newAt, newRt})
            if len(batch) >= s.batchSize {
                if err := s.flushUsersBatch(ctx, batch); err != nil { return err }
                batch = batch[:0]
                time.Sleep(s.batchDelay)
            }
        }
        if err := rows.Err(); err != nil { return err }
        if len(batch) > 0 {
            if err := s.flushUsersBatch(ctx, batch); err != nil { return err }
        }
        return nil
    }

    type userUpdate struct{ ID, AccessToken, RefreshToken string }

    func (s *Sweeper) flushUsersBatch(ctx context.Context, batch []userUpdate) error {
        if s.dryRun {
            s.metrics.RowsReEncrypted["users"] += int64(len(batch))
            return nil
        }
        // Use pgx.Batch for transactional updates
        b := &pgx.Batch{}
        for _, u := range batch {
            b.Queue(`UPDATE users SET access_token=$1, refresh_token=$2, updated_at=NOW() WHERE id=$3`, u.AccessToken, u.RefreshToken, u.ID)
        }
        br := s.pool.SendBatch(ctx, b)
        defer br.Close()
        for range batch {
            if _, err := br.Exec(); err != nil {
                s.metrics.Errors["users"]++
                s.logger.Error("user batch update failed", zap.Error(err))
                return err
            }
            s.metrics.RowsReEncrypted["users"]++
        }
        return nil
    }

    // sweepViewerSessions, sweepYouTubeOAuthTokens, sweepKickOAuthTokens — same shape as sweepUsers.
    // Difference: youtube_oauth_tokens and kick_oauth_tokens additionally read encryption_version
    // and SKIP rows where encryption_version=0 (plaintext) for kick (will be encrypted on next live write).
    // For youtube: encryption_version=0 means plaintext from before token-encryption-backfill ran.
    // We re-encrypt those too, setting encryption_version=1.

    func (s *Sweeper) sweepKickOAuthTokens(ctx context.Context) error {
        const sel = `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,''), encryption_version FROM kick_oauth_tokens ORDER BY id`
        // ... same iteration; if encryption_version == 0, leave alone (plaintext — overlay-manager will encrypt on next write per Plan 14-05)
        // ... if encryption_version >= 1, run encryptIfNotCurrentKid and bump encryption_version stays at 1+ (always 1 since we're using current kid)
        // Implementation detail: we want sweeper to migrate v0→v1 OR not? Per CONTEXT D-16: lazy + sweeper. So YES, sweeper SHOULD encrypt v0 rows in kick_oauth_tokens for completeness.
        // BUT a v0 row in kick_oauth_tokens means a row written before Plan 14-05 deployed; encrypt it now.
        // SO: if encryption_version == 0, encrypt the plaintext access_token/refresh_token directly (no decrypt step needed) and set encryption_version=1.
        // ... full implementation in code
        return nil // placeholder — fill out per behaviors above
    }

    func (s *Sweeper) sweepTikTokOAuthTokens(ctx context.Context) error {
        // Per Plan 14-03 deferral note: tiktok_oauth_tokens v0 rows are Node.js plaintext;
        // we cannot encrypt them safely without a Node.js change. SKIP all v0 rows.
        // Only sweep v1+ rows (which won't exist until a future Node.js phase ships).
        const sel = `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,''), encryption_version FROM tiktok_oauth_tokens WHERE encryption_version >= 1 ORDER BY id`
        // ... same iteration as sweepUsers, but bounded to v1+ rows
        return nil // placeholder
    }

    // sweepOverlayTTSConfigs handles the BYTEA encrypted_api_key column (Pitfall 5).
    func (s *Sweeper) sweepOverlayTTSConfigs(ctx context.Context) error {
        const sel = `SELECT id, encrypted_api_key FROM overlay_tts_configs WHERE encrypted_api_key IS NOT NULL ORDER BY id`
        rows, err := s.pool.Query(ctx, sel)
        if err != nil { return err }
        defer rows.Close()
        type row struct { ID string; Bytes []byte }
        var batch []row
        for rows.Next() {
            var id string
            var b []byte
            if err := rows.Scan(&id, &b); err != nil { return err }
            s.metrics.RowsScanned["overlay_tts_configs"]++
            // The BYTEA blob holds the base64-encoded ciphertext (legacy AESEncryptor.EncryptString output is a base64 string;
            // Phase 13 stored it as-is into BYTEA). Decode to string for encryptIfNotCurrentKid:
            stored := string(b)
            newStored, changed, err := s.encryptIfNotCurrentKid(stored)
            if err != nil {
                s.logger.Warn("tts encrypted_api_key sweep error", zap.String("config_id", id), zap.Error(err))
                s.metrics.Errors["overlay_tts_configs"]++
                continue
            }
            if !changed {
                s.metrics.RowsSkipped["overlay_tts_configs"]++
                continue
            }
            batch = append(batch, row{id, []byte(newStored)})
            if len(batch) >= s.batchSize {
                if err := s.flushTTSBatch(ctx, batch); err != nil { return err }
                batch = batch[:0]
                time.Sleep(s.batchDelay)
            }
        }
        if len(batch) > 0 {
            if err := s.flushTTSBatch(ctx, batch); err != nil { return err }
        }
        return rows.Err()
    }

    func (s *Sweeper) flushTTSBatch(ctx context.Context, batch []row) error { /* analogous to flushUsersBatch using BYTEA UPDATE */ }
    ```

    Step 2 — Implement the remaining `sweepViewerSessions`, `sweepYouTubeOAuthTokens`, `sweepKickOAuthTokens`, `sweepTikTokOAuthTokens` following the same pattern as `sweepUsers`. For the `*_oauth_tokens` tables, additionally:
    - Read `encryption_version`.
    - For kick: encrypt v0 rows (plaintext) directly (skip the Decrypt step) and set `encryption_version=1`.
    - For youtube: re-encrypt v0 (plaintext) AND v1 (legacy ciphertext from YOUTUBE_TOKEN_ENCRYPTION_KEY) → set `encryption_version=1` (or `2` if you want to track multiple versions; D-04 doesn't require a version bump beyond `>=1`).
    - For tiktok: SQL filter `WHERE encryption_version >= 1` ensures v0 rows are skipped per the deferral.

    Step 3 — Create `services/auth-service/cmd/key-rotator/sweeper_test.go` with all 9 listed tests. Use `pgxmock` (already used in the codebase per Plan 07's miniredis precedent — verify via grep) OR use the real test DB if `make test-integration` is the standard. If pgxmock is unavailable, use a real Postgres docker-compose (see `make docker-up` from Quick Start). Tests MUST be deterministic.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go build ./services/auth-service/cmd/key-rotator/... && go test ./services/auth-service/cmd/key-rotator/... -count=1 -run 'TestSweeper'</automated>
  </verify>
  <acceptance_criteria>
    - `test -f services/auth-service/cmd/key-rotator/sweeper.go && test -f services/auth-service/cmd/key-rotator/sweeper_test.go`
    - `grep -q "type Sweeper struct\|type SweeperMetrics struct" services/auth-service/cmd/key-rotator/sweeper.go`
    - `grep -q "encryptIfNotCurrentKid" services/auth-service/cmd/key-rotator/sweeper.go`
    - `grep -q "func (s \*Sweeper) sweepUsers\|sweepViewerSessions\|sweepYouTubeOAuthTokens\|sweepOverlayTTSConfigs\|sweepKickOAuthTokens\|sweepTikTokOAuthTokens" services/auth-service/cmd/key-rotator/sweeper.go` (all six methods present)
    - `grep -q "encryption_version >= 1" services/auth-service/cmd/key-rotator/sweeper.go` (tiktok skip and youtube/kick gate)
    - `grep -q "BYTEA\|encrypted_api_key" services/auth-service/cmd/key-rotator/sweeper.go` (Pitfall 5 BYTEA path)
    - `grep -q "TestSweeper_Idempotent\|TestSweeper_DryRun\|TestSweeper_TTSBytea\|TestSweeper_SkipsTikTok\|TestSweeper_HandlesDecrypt" services/auth-service/cmd/key-rotator/sweeper_test.go`
    - `cd /home/moersener/Hobby/all-chat && go build ./services/auth-service/cmd/key-rotator/...` exits 0
    - `cd /home/moersener/Hobby/all-chat && go test ./services/auth-service/cmd/key-rotator/... -count=1` exits 0
  </acceptance_criteria>
  <done>Sweeper type and tests exist and pass. All 6 tables have sweep methods. BYTEA Pitfall 5 mitigation in place. TikTok v0 skip in place. Idempotency proven (TestSweeper_SkipsCurrentKid).</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Create main.go entry point with flags, env loading, structured-log summary</name>
  <files>services/auth-service/cmd/key-rotator/main.go, services/auth-service/cmd/key-rotator/main_test.go</files>
  <read_first>
    - services/auth-service/cmd/token-encryption-backfill/main.go (post-Plan-14-01) — full file, the structural template
    - services/auth-service/cmd/key-rotator/sweeper.go (Task 1 output)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §6 "Deployment Shape" (lines 528–569)
  </read_first>
  <behavior>
    - Test 1 (TestMain_FlagsParse): synthetic argv with `--dry-run --batch-size=50 --batch-delay-ms=10 --skip-table=tiktok_oauth_tokens`; assert flags parsed correctly.
    - Test 2 (TestMain_RequiresDatabaseURL): env DATABASE_URL unset → main() exits non-zero with "DATABASE_URL must be set" message in zap log.
    - Test 3 (TestMain_RequiresEncryptionKey): env DATABASE_URL set but no TOKEN_ENCRYPTION_KEY_V<n> → main() exits non-zero.

    These tests use `os/exec` to run the binary as a subprocess (Go convention for testing main). Alternative: extract `run(ctx, opts) error` and test that.
  </behavior>
  <action>
    Step 1 — Create `services/auth-service/cmd/key-rotator/main.go`:

    ```go
    // AGPL header
    package main

    import (
        "context"
        "flag"
        "fmt"
        "os"
        "os/signal"
        "syscall"
        "time"

        "github.com/caesar/all-chat/shared/encryption"
        "github.com/jackc/pgx/v5/pgxpool"
        "go.uber.org/zap"
    )

    type repeatedString []string
    func (r *repeatedString) String() string { return fmt.Sprint(*r) }
    func (r *repeatedString) Set(v string) error { *r = append(*r, v); return nil }

    func main() {
        dryRun := flag.Bool("dry-run", false, "log rows that would be updated without mutating the database")
        batchSize := flag.Int("batch-size", 100, "rows per UPDATE batch")
        batchDelayMs := flag.Int("batch-delay-ms", 50, "milliseconds between batches to throttle DB load")
        var skipTables repeatedString
        flag.Var(&skipTables, "skip-table", "table to skip (repeatable)")
        flag.Parse()

        logger, _ := zap.NewProduction()
        defer logger.Sync()

        dbURL := os.Getenv("DATABASE_URL")
        if dbURL == "" { logger.Fatal("DATABASE_URL must be set") }

        encryptor, err := encryption.NewMultiKeyEncryptorFromEnv()
        if err != nil { logger.Fatal("encryption init failed", zap.Error(err)) }

        ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
        defer cancel()

        pool, err := pgxpool.New(ctx, dbURL)
        if err != nil { logger.Fatal("db pool init failed", zap.Error(err)) }
        defer pool.Close()

        opts := []SweeperOption{
            WithDryRun(*dryRun),
            WithBatchSize(*batchSize),
            WithBatchDelay(time.Duration(*batchDelayMs) * time.Millisecond),
        }
        for _, t := range skipTables {
            opts = append(opts, WithSkipTable(t))
        }

        sweeper := NewSweeper(pool, encryptor, logger, opts...)
        logger.Info("starting key-rotator sweep",
            zap.Bool("dry_run", *dryRun),
            zap.Int("batch_size", *batchSize),
            zap.Duration("batch_delay", time.Duration(*batchDelayMs)*time.Millisecond),
            zap.Strings("skip_tables", skipTables),
            zap.Uint8("current_kid", encryptor.CurrentKid()),
        )

        if err := sweeper.SweepAll(ctx); err != nil {
            logger.Fatal("sweep failed", zap.Error(err))
        }

        logger.Info("sweep complete",
            zap.Any("scanned", sweeper.metrics.RowsScanned),
            zap.Any("re_encrypted", sweeper.metrics.RowsReEncrypted),
            zap.Any("skipped", sweeper.metrics.RowsSkipped),
            zap.Any("errors", sweeper.metrics.Errors),
        )
    }
    ```

    Step 2 — To make the binary testable, refactor: extract a `run(ctx context.Context, args []string, env map[string]string, stdout, stderr io.Writer) int` (returning an exit code) so tests can call `run(...)` instead of forking subprocesses. Wrap `main` around `run`:
    ```go
    func main() {
        os.Exit(run(context.Background(), os.Args[1:], envMap(), os.Stdout, os.Stderr))
    }
    ```

    Step 3 — Create `services/auth-service/cmd/key-rotator/main_test.go` with tests TestMain_FlagsParse, TestMain_RequiresDatabaseURL, TestMain_RequiresEncryptionKey using the `run(...)` extraction.

    Step 4 — Verify the binary builds:
    ```
    cd /home/moersener/Hobby/all-chat/services/auth-service && go build -o /tmp/key-rotator ./cmd/key-rotator/...
    /tmp/key-rotator --help 2>&1 | head -20
    ```
    Expected: usage message listing --dry-run, --batch-size, --batch-delay-ms, --skip-table.

    Step 5 — Smoke test the dry-run mode against a local docker-compose Postgres (if available). This is optional in CI; the unit tests cover the contract.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go build ./services/auth-service/cmd/key-rotator/... && go test ./services/auth-service/cmd/key-rotator/... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `test -f services/auth-service/cmd/key-rotator/main.go && test -f services/auth-service/cmd/key-rotator/main_test.go`
    - `grep -q "func main()" services/auth-service/cmd/key-rotator/main.go`
    - `grep -q "dry-run\|batch-size\|batch-delay-ms\|skip-table" services/auth-service/cmd/key-rotator/main.go` (all four flags present)
    - `grep -q "DATABASE_URL must be set\|encryption init failed" services/auth-service/cmd/key-rotator/main.go`
    - `grep -q "NewMultiKeyEncryptorFromEnv\|SweepAll" services/auth-service/cmd/key-rotator/main.go`
    - `grep -q "TestMain_FlagsParse\|TestMain_RequiresDatabaseURL" services/auth-service/cmd/key-rotator/main_test.go`
    - `cd /home/moersener/Hobby/all-chat && go build -o /tmp/key-rotator ./services/auth-service/cmd/key-rotator/...` exits 0
    - `cd /home/moersener/Hobby/all-chat && go test ./services/auth-service/cmd/key-rotator/... -count=1` exits 0
  </acceptance_criteria>
  <done>key-rotator binary builds. main_test.go covers flag parsing, env preflight, and the run() contract. Help output lists all four flags.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| K8s Secret → Job process env | Sweeper Job mounts `database-password`, `token-encryption-key`, `token-encryption-key-v1`, `youtube-token-encryption-key` (legacy) — full DB read+write privileges |
| DB ciphertext → process plaintext (briefly) | Each row is decrypted then re-encrypted in process memory; plaintext lives in memory < 1ms per row |
| process re-encrypted blob → DB | UPDATE round-trip writes versioned ciphertext to the same column |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-06-01 | Tampering | Sweeper corrupts a row by writing un-decryptable ciphertext (e.g., write fails after re-encrypt success in memory) | mitigate | UPDATE happens inside a `pgx.Batch`; partial-batch failure rolls back the SendBatch transaction; the source columns are not touched until the new ciphertext is verified-decryptable. encryptIfNotCurrentKid VERIFIES decryption succeeds before re-encrypting (T6) |
| T-14-06-02 | Tampering | Concurrent sweeper run (two Jobs in parallel) writing conflicting versions | mitigate | Each row's UPDATE only proceeds if its current value differs from CurrentKid. If two sweepers race, the second sees CurrentKid match and skips. Row-level row LOCK is overkill; idempotency is sufficient. Documented in Task 1 design. |
| T-14-06-03 | Information Disclosure | Sweeper logs leak plaintext token values | mitigate | All logger calls use only IDs (`zap.String("user_id", id)`) and counts; never the plaintext. Code review acceptance criterion checks `! grep -q "zap.*plaintext\|zap.*access_token\|zap.*token_value"` |
| T-14-06-04 | Information Disclosure | Sweeper logs leak the plaintext from a decrypt error path | mitigate | encryptIfNotCurrentKid wraps decrypt error as `fmt.Errorf("decrypt: %w", err)` — `%w` preserves the error chain (which contains library-level error names like "cipher: message authentication failed") without including ciphertext or plaintext payloads. Test TestSweeper_HandlesDecryptError asserts the row-id-only log line. |
| T-14-06-05 | Denial of Service | Unthrottled sweep saturating DB during business hours | mitigate | --batch-size + --batch-delay-ms; default 100 rows / 50ms = 2000 rows/sec ceiling, well under primary's capacity. CronJob schedule is Sunday 03:00 UTC per RESEARCH.md §6 |
| T-14-06-06 | Denial of Service | Sweeper hung indefinitely on a slow query | mitigate | Context propagation from `signal.NotifyContext` (SIGINT/SIGTERM) cancels in-flight queries. Job spec sets activeDeadlineSeconds=3600 in Plan 14-07 |
| T-14-06-07 | Tampering | Sweeper accidentally migrates tiktok_oauth_tokens v0 plaintext rows by mis-applying the kick code path | mitigate | sweepTikTokOAuthTokens uses SQL filter `WHERE encryption_version >= 1`, NOT the kick "encrypt v0 plaintext" path; TestSweeper_SkipsTikTokV0 asserts v0 rows are untouched |
| T-14-06-08 | Repudiation | Sweeper "completed" but operator can't audit which rows were updated when | mitigate | Per-batch flush logs a structured event with `{table, batch_size, dry_run}`; final `sweep complete` log includes per-table SweeperMetrics counts; CronJob retains last 3 successful + last 1 failed Job per Plan 14-07 |
</threat_model>

<verification>
- `go test ./services/auth-service/cmd/key-rotator/... -count=1 -race` — all 9 sweeper tests + 3 main tests green.
- `go build -o /tmp/key-rotator ./services/auth-service/cmd/key-rotator/...` — binary builds.
- `/tmp/key-rotator --help` — usage shows all four flags.
- BYTEA path proven by TestSweeper_TTSBytea.
- TikTok v0 skip proven by TestSweeper_SkipsTikTokV0.
- Idempotency proven by TestSweeper_Idempotent.
</verification>

<success_criteria>
- `services/auth-service/cmd/key-rotator/` directory contains main.go + sweeper.go + tests.
- Binary builds and passes all unit tests.
- All 6 sweep methods present; TikTok skip and BYTEA Pitfall 5 mitigations applied.
- --dry-run, --batch-size, --batch-delay-ms, --skip-table flags wired.
- Telemetry: structured zap logs report per-table counts at end of sweep.
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-06-SUMMARY.md` documenting:
- Sweeper file structure and which tables are swept.
- TikTok v0 skip rationale (Node.js scope deferral) — ensures sweeper doesn't accidentally encrypt plaintext.
- BYTEA pattern for overlay_tts_configs.encrypted_api_key (Pitfall 5).
- Concrete K8s Job/CronJob YAML deferred to Plan 14-07.
- Note for Plan 14-07: the K8s Job needs env vars TOKEN_ENCRYPTION_KEY (legacy), TOKEN_ENCRYPTION_KEY_V1, YOUTUBE_TOKEN_ENCRYPTION_KEY (legacy), DATABASE_URL.
- Note for Plan 14-08: production sweep is in two stages — first `--dry-run` to size the change, then live sweep.
</output>
