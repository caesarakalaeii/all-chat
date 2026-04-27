---
phase: 14-secret-rotation-infrastructure
plan: 03
type: execute
wave: 1
depends_on: []
files_modified:
  - migrations/050_kick_token_encryption.sql
  - migrations/051_tiktok_token_encryption.sql
  - migrations/050_kick_token_encryption_down.sql
  - migrations/051_tiktok_token_encryption_down.sql
autonomous: true
decisions_addressed:
  - D-16
must_haves:
  truths:
    - "kick_oauth_tokens has an encryption_version SMALLINT column with default 0 (existing rows = plaintext)"
    - "tiktok_oauth_tokens has an encryption_version SMALLINT column with default 0 (existing rows = plaintext)"
    - "Both migrations are idempotent (use IF NOT EXISTS)"
    - "Down migrations exist and are reversible"
  artifacts:
    - path: "migrations/050_kick_token_encryption.sql"
      provides: "ADD COLUMN encryption_version to kick_oauth_tokens + index"
      contains: "ALTER TABLE kick_oauth_tokens"
    - path: "migrations/051_tiktok_token_encryption.sql"
      provides: "ADD COLUMN encryption_version to tiktok_oauth_tokens + index"
      contains: "ALTER TABLE tiktok_oauth_tokens"
    - path: "migrations/050_kick_token_encryption_down.sql"
      provides: "DROP COLUMN reversal"
    - path: "migrations/051_tiktok_token_encryption_down.sql"
      provides: "DROP COLUMN reversal"
  key_links:
    - from: "migrations/050_kick_token_encryption.sql"
      to: "migrations/006_youtube_token_encryption.sql"
      via: "exact pattern copy: ADD COLUMN encryption_version SMALLINT NOT NULL DEFAULT 0 + CREATE INDEX"
      pattern: "encryption_version SMALLINT NOT NULL DEFAULT 0"
---

<objective>
Add `encryption_version` columns to `kick_oauth_tokens` and `tiktok_oauth_tokens`, mirroring the precedent from `migrations/006_youtube_token_encryption.sql`. This is the schema half of D-16 (encryption gap-fill); the corresponding code changes (write paths encrypt under V1, read paths decrypt) ship in Plan 14-05 (kick) and are explicitly DEFERRED for tiktok (Node.js scope — see Open Question 1 below).

Purpose: Implements decision D-16 partial — schema-only. Code changes (D-17) are scoped to Plan 14-05.

Output:
- `migrations/050_kick_token_encryption.sql` and matching `_down.sql`.
- `migrations/051_tiktok_token_encryption.sql` and matching `_down.sql`.

**Open Question 1 disposition (per planning context):** The tiktok-listener is Node.js. No Go service reads or writes `tiktok_oauth_tokens`. Phase 14 ships the schema column for the TikTok table so future Node.js work (or a Go shim) can use it; the Node.js encryption code-side change is **deferred to a follow-up phase** with this rationale documented in the plan summary. The sweeper (Plan 14-06) will sweep tiktok_oauth_tokens conditional on `encryption_version=1` rows existing, so it remains forward-compatible without immediate Node.js changes.
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
@migrations/006_youtube_token_encryption.sql
@migrations/004_tiktok_support.sql
@migrations/005_kick_support.sql
</context>

<tasks>

<task type="auto">
  <name>Task 1: Create migration 050 (kick) and 051 (tiktok) plus down migrations</name>
  <files>migrations/050_kick_token_encryption.sql, migrations/051_tiktok_token_encryption.sql, migrations/050_kick_token_encryption_down.sql, migrations/051_tiktok_token_encryption_down.sql</files>
  <read_first>
    - migrations/006_youtube_token_encryption.sql (full file — exact pattern to copy)
    - migrations/004_tiktok_support.sql (full file — confirms tiktok_oauth_tokens schema)
    - migrations/005_kick_support.sql (full file — confirms kick_oauth_tokens schema)
    - migrations/049_overlay_tts_configs.sql (skim — exemplary use of `COMMENT ON COLUMN` to document encryption semantics)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §8 "Migration Strategy" (lines 687–743)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "migrations/050..." section (lines 466–502)
  </read_first>
  <action>
    Step 1 — Verify next migration number is 050:
    ```
    ls /home/moersener/Hobby/all-chat/migrations/ | grep -E '^[0-9]+_' | sort | tail -5
    ```
    Expected last: `049_overlay_tts_configs.sql`. If something has already claimed 050 or 051, stop and notify orchestrator.

    Step 2 — Create `migrations/050_kick_token_encryption.sql`. Copy the exact pattern from migration 006:
    ```sql
    -- All-Chat Migration 050: Add encryption metadata for Kick OAuth tokens
    -- Phase 14 (Secret Rotation Infrastructure) - D-16 schema half.
    -- Existing rows have encryption_version=0 (plaintext); new writes from Plan 14-05
    -- onward set encryption_version=1 with the versioned [kid||nonce||ct||tag] format
    -- from shared/encryption.MultiKeyEncryptor.
    BEGIN;

    ALTER TABLE kick_oauth_tokens
        ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;

    CREATE INDEX IF NOT EXISTS idx_kick_oauth_tokens_enc_version
        ON kick_oauth_tokens(encryption_version);

    COMMENT ON COLUMN kick_oauth_tokens.encryption_version IS
        '0 = legacy plaintext access_token/refresh_token (pre-Phase-14); 1+ = versioned ciphertext per shared/encryption.MultiKeyEncryptor wire format [kid(1B)||nonce(12B)||ct||tag(16B)]';

    COMMIT;
    ```

    Step 3 — Create `migrations/051_tiktok_token_encryption.sql` with the same shape, swapping `kick_oauth_tokens` → `tiktok_oauth_tokens` and `idx_kick_*` → `idx_tiktok_*`. Add a CRITICAL doc comment at the top:
    ```sql
    -- All-Chat Migration 051: Add encryption metadata for TikTok OAuth tokens
    -- Phase 14 (Secret Rotation Infrastructure) - D-16 schema half.
    --
    -- IMPORTANT: The tiktok-listener service is Node.js (services/tiktok-listener/),
    -- not Go. The encryption code-side change for D-17 (encrypt-on-write,
    -- decrypt-on-read) is deferred to a follow-up phase that adds a Node.js
    -- equivalent of shared/encryption.MultiKeyEncryptor (or routes token I/O
    -- through a Go gateway). This migration ships the schema column NOW so the
    -- sweeper (services/auth-service/cmd/key-rotator) and any future writer can
    -- distinguish v0 plaintext from v1+ ciphertext rows.
    BEGIN;

    ALTER TABLE tiktok_oauth_tokens
        ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;

    CREATE INDEX IF NOT EXISTS idx_tiktok_oauth_tokens_enc_version
        ON tiktok_oauth_tokens(encryption_version);

    COMMENT ON COLUMN tiktok_oauth_tokens.encryption_version IS
        '0 = plaintext (current state — Node.js listener has not migrated to versioned encryption); 1+ = versioned ciphertext per shared/encryption.MultiKeyEncryptor wire format. See Phase 14 plan 14-03.';

    COMMIT;
    ```

    Step 4 — Create down migrations `migrations/050_kick_token_encryption_down.sql`:
    ```sql
    -- Down migration for 050_kick_token_encryption.sql
    -- WARNING: Dropping the encryption_version column does NOT decrypt rows; any
    -- ciphertext stored in access_token/refresh_token will become unrecoverable
    -- because the decryption path (kick-listener) reads encryption_version to
    -- decide whether to decrypt. Run only if Phase 14 is being fully reverted.
    BEGIN;

    DROP INDEX IF EXISTS idx_kick_oauth_tokens_enc_version;
    ALTER TABLE kick_oauth_tokens DROP COLUMN IF EXISTS encryption_version;

    COMMIT;
    ```
    And `migrations/051_tiktok_token_encryption_down.sql` analogous.

    Step 5 — Verify SQL syntax by running through a docker-compose Postgres or `make migrate-up` if available. If `make migrate-up` is available in this repo, run it (do not commit DB state changes — this is a syntax verification only). Otherwise rely on a `psql --dry-run`-style parse check by piping into a docker postgres container:
    ```
    docker run --rm -i postgres:16 psql -U postgres -e -c '\set ON_ERROR_STOP on' < migrations/050_kick_token_encryption.sql 2>&1 | head -20
    ```
    NOTE: The above will FAIL because there's no `kick_oauth_tokens` table in a fresh container. The acceptance gate is the file existing with the exact ALTER/CREATE INDEX/COMMENT statements; the actual application is verified end-to-end in Plan 14-08 against a staging DB.

    Step 6 — Confirm the migration file naming matches the existing pattern. The repo uses `NNN_name.sql` (zero-padded 3 digits) per the `006_youtube_token_encryption.sql` precedent.
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && test -f migrations/050_kick_token_encryption.sql && test -f migrations/051_tiktok_token_encryption.sql && test -f migrations/050_kick_token_encryption_down.sql && test -f migrations/051_tiktok_token_encryption_down.sql && grep -q "ALTER TABLE kick_oauth_tokens" migrations/050_kick_token_encryption.sql && grep -q "ALTER TABLE tiktok_oauth_tokens" migrations/051_tiktok_token_encryption.sql && grep -q "encryption_version SMALLINT NOT NULL DEFAULT 0" migrations/050_kick_token_encryption.sql && grep -q "encryption_version SMALLINT NOT NULL DEFAULT 0" migrations/051_tiktok_token_encryption.sql</automated>
  </verify>
  <acceptance_criteria>
    - `test -f migrations/050_kick_token_encryption.sql && test -f migrations/050_kick_token_encryption_down.sql`
    - `test -f migrations/051_tiktok_token_encryption.sql && test -f migrations/051_tiktok_token_encryption_down.sql`
    - `grep -q "ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0" migrations/050_kick_token_encryption.sql`
    - `grep -q "ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0" migrations/051_tiktok_token_encryption.sql`
    - `grep -q "idx_kick_oauth_tokens_enc_version" migrations/050_kick_token_encryption.sql`
    - `grep -q "idx_tiktok_oauth_tokens_enc_version" migrations/051_tiktok_token_encryption.sql`
    - `grep -q "Node.js" migrations/051_tiktok_token_encryption.sql` (Node.js scope rationale documented in-file)
    - `grep -q "BEGIN;" migrations/050_kick_token_encryption.sql && grep -q "COMMIT;" migrations/050_kick_token_encryption.sql`
    - `grep -q "DROP COLUMN IF EXISTS encryption_version" migrations/050_kick_token_encryption_down.sql`
    - Re-running the up migration produces no error (proven by `IF NOT EXISTS` clauses; verified syntactically via grep)
  </acceptance_criteria>
  <done>Both migrations exist with idempotent ALTER + index + COMMENT. Down migrations exist with DROP COLUMN + index. Tiktok migration carries explicit Node.js scope rationale.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Migration runner → live DB | The init container running migrations has DDL privileges; ALTER TABLE on a production table |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-03-01 | Tampering | Re-running migration 050/051 corrupts column type or default | mitigate | `ADD COLUMN IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS` make both migrations idempotent — re-runs are no-ops |
| T-14-03-02 | Information Disclosure | Migration leaks plaintext via comment or logging | accept | `COMMENT ON COLUMN` text contains no secret material; only schema documentation |
| T-14-03-03 | Repudiation | Down migration runs accidentally and drops `encryption_version`, leaving ciphertext rows undecodable | mitigate | Down migration includes WARNING comment explaining the consequence; in production, down migrations are gated by the deployment runbook (Plan 14-07) |
| T-14-03-04 | Denial of Service | ALTER TABLE acquiring AccessExclusiveLock on a hot table during migration | accept | `ADD COLUMN ... NOT NULL DEFAULT 0` in PostgreSQL 11+ is metadata-only (no full table rewrite); kick_oauth_tokens has < 10k rows in production based on user count |
</threat_model>

<verification>
- All four files exist with the required SQL statements.
- Migrations are idempotent by use of `IF NOT EXISTS`.
- Down migrations exist and are syntactically valid.
- `grep -q "Node.js" migrations/051_tiktok_token_encryption.sql` confirms the explicit Node.js scope-deferral note is present.
</verification>

<success_criteria>
- Two new up migrations and two new down migrations committed.
- Idempotent (re-runnable safely).
- Tiktok migration documents the Node.js code-side deferral.
- Numbering is sequential (050 follows 049_overlay_tts_configs.sql; 051 follows 050).
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-03-SUMMARY.md` documenting:
- Migration files created with exact paths.
- Confirmation that Node.js (tiktok) encryption code-side work is deferred (D-17 partial).
- Confirmation that re-runs are idempotent.
- Note for Plan 14-05: kick-listener and overlay-manager handlers must read `encryption_version` and gate decrypt on `>= 1`.
- Note for Plan 14-06 (sweeper): tiktok_oauth_tokens scan should skip rows with `encryption_version=0` for now (Node.js hasn't migrated); only sweep `encryption_version >= 1` rows for re-encryption to current kid.
</output>
