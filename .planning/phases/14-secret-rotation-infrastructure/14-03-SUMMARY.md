---
phase: 14-secret-rotation-infrastructure
plan: "03"
subsystem: database
tags: [postgresql, migrations, encryption, aes-gcm, kick, tiktok]

requires:
  - phase: 14-01
    provides: shared/encryption MultiKeyEncryptor (versioned AES-GCM with kid prefix)
  - phase: 14-02
    provides: shared/auth KeyChain for kid-aware JWT signing/validation

provides:
  - encryption_version SMALLINT NOT NULL DEFAULT 0 column on kick_oauth_tokens
  - encryption_version SMALLINT NOT NULL DEFAULT 0 column on tiktok_oauth_tokens
  - idx_kick_oauth_tokens_enc_version index for sweeper efficiency
  - idx_tiktok_oauth_tokens_enc_version index for sweeper efficiency
  - COMMENT ON COLUMN documenting wire format for both tables
  - Idempotent up migrations (IF NOT EXISTS)
  - Reversible down migrations with ciphertext-unrecoverability safety warning

affects:
  - 14-05
  - 14-06
  - kick-listener
  - overlay-manager
  - auth-service/key-rotator

tech-stack:
  added: []
  patterns:
    - "ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0 — idempotent column addition pattern (mirrors migration 006 for youtube)"
    - "COMMENT ON COLUMN to document encryption wire format and scope-deferral rationale in schema"
    - "Down migration WARNING comment pattern — documents ciphertext-loss risk before dropping column"

key-files:
  created:
    - migrations/050_kick_token_encryption.sql
    - migrations/050_kick_token_encryption_down.sql
    - migrations/051_tiktok_token_encryption.sql
    - migrations/051_tiktok_token_encryption_down.sql
  modified: []

key-decisions:
  - "D-16 schema half: encryption_version SMALLINT NOT NULL DEFAULT 0 (not INT per critical constraints) mirrors 006_youtube_token_encryption.sql exactly"
  - "Node.js tiktok-listener encryption code-side change is deferred — migration 051 ships schema-only; sweeper (14-06) must skip encryption_version=0 tiktok rows"
  - "No BYTEA column-type change in this plan — access_token/refresh_token remain TEXT; Plan 14-05 (kick code) handles encryption at the application layer, not via SQL type change (matching youtube pattern)"
  - "Down migrations are conservative — DROP COLUMN IF EXISTS only, no data manipulation; includes explicit safety check guidance for operators"

patterns-established:
  - "Column-addition-only migration: no rewrite of existing TEXT column data — application layer decides encrypt/decrypt per encryption_version value"
  - "COMMENT ON COLUMN as in-schema documentation for encryption wire format, Phase tracking, and Node.js scope-deferral rationale"

requirements-completed: []

duration: 11min
completed: "2026-04-27"
---

# Phase 14 Plan 03: Kick + TikTok Encryption Migrations Summary

**Schema foundation for kick + tiktok token encryption: migrations 050 and 051 add `encryption_version SMALLINT NOT NULL DEFAULT 0` columns with indexes, enabling Plan 14-05 read/write encryption gates and Plan 14-06 sweeper targeting**

## Performance

- **Duration:** 11 min
- **Started:** 2026-04-27T13:36:49Z
- **Completed:** 2026-04-27T13:48:00Z
- **Tasks:** 1 (single atomic task, 4 files)
- **Files modified:** 4 (all new)

## Accomplishments

- Created `migrations/050_kick_token_encryption.sql` — idempotent `ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0` + index + `COMMENT ON COLUMN` for `kick_oauth_tokens`, wrapped in `BEGIN`/`COMMIT` transaction.
- Created `migrations/051_tiktok_token_encryption.sql` — same shape for `tiktok_oauth_tokens`. Carries mandatory header comment explaining Node.js scope-deferral (tiktok-listener is Node.js, not Go; code-side encryption deferred to a follow-up phase; sweeper must skip `encryption_version=0` tiktok rows).
- Created `migrations/050_kick_token_encryption_down.sql` and `migrations/051_tiktok_token_encryption_down.sql` — reversible `DROP INDEX IF EXISTS` + `DROP COLUMN IF EXISTS`, each with a safety WARNING comment guiding operators to verify no v1 rows exist before running.

## Task Commits

1. **Task 1: migrations 050 + 051 (up + down)** - `7c15932a` (feat)

**Plan metadata:** (docs commit below)

## Files Created/Modified

- `migrations/050_kick_token_encryption.sql` — adds `encryption_version` column + index + column comment to `kick_oauth_tokens`
- `migrations/050_kick_token_encryption_down.sql` — reversal: drops index + column with operator safety guidance
- `migrations/051_tiktok_token_encryption.sql` — adds `encryption_version` column + index + column comment to `tiktok_oauth_tokens`; documents Node.js scope-deferral and sweeper skip requirement
- `migrations/051_tiktok_token_encryption_down.sql` — reversal: drops index + column with operator safety guidance

## Decisions Made

**No BYTEA column-type change in this plan.** The `access_token` and `refresh_token` columns remain `TEXT` in both tables. This matches the YouTube precedent (`006_youtube_token_encryption.sql`): the migration adds only the `encryption_version` sentinel; the application layer (Plan 14-05) handles encryption/decryption. The TEXT columns store base64-encoded ciphertext when `encryption_version >= 1`, which is valid for TEXT columns.

**SMALLINT not INT for encryption_version.** The plan frontmatter specifies `SMALLINT NOT NULL DEFAULT 0` (matching migration 006). This is semantically appropriate — values 0/1/2 never approach INT range, and SMALLINT saves 2 bytes per row vs INT.

**Node.js TikTok deferral is explicit in two places.** The header comment of `051_tiktok_token_encryption.sql` documents the deferral. The `COMMENT ON COLUMN` text also references it. This ensures the decision is discoverable at both the file level and via `\d+ tiktok_oauth_tokens` in psql, without any future agent accidentally "fixing" the missing encryption code.

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None. Migration numbering confirmed as 050/051 (last existing: `049_overlay_tts_configs.sql`). All four acceptance criteria checks passed.

## Static Verification

Applied migrations via `make migrate-up` was NOT run (local dev DB availability unknown). Instead, static verification was performed:

- All four acceptance criteria grep checks passed (see task verification).
- Migration SQL structure is syntactically correct (verified by pattern comparison with `006_youtube_token_encryption.sql` and `049_overlay_tts_configs.sql`).
- `ADD COLUMN IF NOT EXISTS` in PostgreSQL 11+ for `SMALLINT NOT NULL DEFAULT 0` is a metadata-only operation — no full table rewrite, no `AccessExclusiveLock` beyond the brief DDL lock (per T-14-03-04 threat model: kick_oauth_tokens has < 10k rows).
- Full end-to-end application against a staging DB is gated to Plan 14-08 per the task spec.

## Notes for Downstream Plans

**Plan 14-05 (kick-listener encryption code):**
- Read path: SELECT must include `encryption_version` alongside `access_token`/`refresh_token`. Gate `DecryptString` on `encryption_version >= 1`.
- Write path: always write `encryption_version = 1` and the MultiKeyEncryptor-encrypted blob.
- overlay-manager/handlers/sources.go has both a read path (lines 214-249) and a write path that need the same treatment.
- kick-listener/channels/manager.go line 969 reads `access_token` only — must be extended to also read `encryption_version`.

**Plan 14-06 (sweeper / key-rotator):**
- For `kick_oauth_tokens`: sweep all rows, re-encrypt `encryption_version >= 1` rows to current kid. Backfill `encryption_version = 0` rows if the encryption code-side (14-05) has shipped.
- For `tiktok_oauth_tokens`: SKIP rows with `encryption_version = 0` — the Node.js tiktok-listener cannot yet produce v1 rows. Only re-encrypt any `encryption_version >= 1` rows that may have been written by a future Node.js implementation.

## Threat Surface Scan

No new network endpoints, auth paths, or trust boundaries introduced. This plan is DDL-only (schema changes). The `COMMENT ON COLUMN` text contains no secret material (T-14-03-02 disposition: accept). Down migrations carry explicit operator warnings preventing accidental data loss (T-14-03-03 disposition: mitigate via documentation).

## Self-Check: PASSED

Files confirmed present:
- `migrations/050_kick_token_encryption.sql` — FOUND
- `migrations/050_kick_token_encryption_down.sql` — FOUND
- `migrations/051_tiktok_token_encryption.sql` — FOUND
- `migrations/051_tiktok_token_encryption_down.sql` — FOUND

Commit `7c15932a` confirmed in git log.

---
*Phase: 14-secret-rotation-infrastructure*
*Completed: 2026-04-27*
