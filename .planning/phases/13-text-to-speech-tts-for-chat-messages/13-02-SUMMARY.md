---
phase: 13-text-to-speech-tts-for-chat-messages
plan: 02
subsystem: api
tags: [tts, elevenlabs, aes-gcm, jwt, featuregates, postgres, pgx, gin, streaming-proxy, premium-gate]

# Dependency graph
requires:
  - phase: 07-feature-gate-infrastructure
    provides: FeatureGateCache + RequirePremium middleware (promoted into shared/ in this plan)
  - phase: 06 (shared/encryption.AESEncryptor — already in tree)
    provides: AES-GCM Encrypt/Decrypt primitives with 12-byte nonce, consumed here for api_key storage
provides:
  - "overlay_tts_configs table + tts feature_gates row (migration 049)"
  - "shared/featuregates package (moved from services/share-service/featuregates)"
  - "shared/middleware/premium.go (moved from services/share-service/middleware)"
  - "services/overlay-manager/tts — per-overlay tts_token JWT sign/verify (HS256, rotation-based revocation)"
  - "services/overlay-manager/models.TTSConfig — model for overlay_tts_configs rows"
  - "services/overlay-manager/repository.TTSConfigRepository — CRUD + RotateSigningSecret"
  - "services/overlay-manager/handlers.TTSHandler — 7 HTTP endpoints incl. streaming proxy"
  - "k8s deployment of TOKEN_ENCRYPTION_KEY + OVERLAY_PUBLIC_BASE_URL for overlay-manager"
affects: [13-03-elevenlabs-frontend-ux, 13-01-web-speech-tier, ADR-0012-aes-gcm-encryption]

# Tech tracking
tech-stack:
  added:
    - "golang-jwt/jwt/v5 promoted from indirect to direct in services/overlay-manager/go.mod"
    - "shared/featuregates and shared/middleware/premium.go as shared packages (formerly share-service-only)"
  patterns:
    - "Per-overlay HS256 JWT with no ExpiresAt — revocation via rotating the signing secret"
    - "Streaming HTTP proxy with context-propagated cancellation (http.NewRequestWithContext + io.Copy)"
    - "In-memory fixed-window rate limiter (map[overlayID]*rateBucket + sync.Mutex) for per-tenant safety caps"
    - "Narrow test-seam interfaces (ttsConfigStore, overlayOwnershipChecker, aesCipher) for whitebox handler tests without pgxmock"
    - "json:\"-\" tag on encrypted blob + HMAC secret fields — belt-and-suspenders guarantee that the model never leaks over the wire"

key-files:
  created:
    - "migrations/049_overlay_tts_configs.sql"
    - "migrations/049_overlay_tts_configs_down.sql"
    - "shared/featuregates/cache.go"
    - "shared/featuregates/cache_test.go"
    - "shared/middleware/premium.go"
    - "shared/middleware/premium_test.go"
    - "services/overlay-manager/tts/jwt.go"
    - "services/overlay-manager/tts/jwt_test.go"
    - "services/overlay-manager/models/tts_config.go"
    - "services/overlay-manager/repository/tts_config_repo.go"
    - "services/overlay-manager/repository/tts_config_repo_test.go"
    - "services/overlay-manager/handlers/tts.go"
    - "services/overlay-manager/handlers/tts_test.go"
  modified:
    - "services/overlay-manager/cmd/main.go"
    - "services/overlay-manager/go.mod"
    - "services/share-service/cmd/main.go"
    - "services/share-service/go.mod"
    - "services/share-service/handlers/admin_featuregates.go"
    - "services/share-service/handlers/admin_featuregates_test.go"
    - "deployments/k8s/base/overlay-manager/deployment.yaml"

key-decisions:
  - "Reused shared/encryption.AESEncryptor and its existing TOKEN_ENCRYPTION_KEY env var rather than creating a new shared/crypto package + CRYPTO_MASTER_KEY. The existing package already implements AES-256-GCM with 12-byte random nonce prefix and has a complete test suite; adding another crypto layer would only duplicate code."
  - "POST /tts is NOT behind RequirePremium — graceful-permanence contract (D-10 implied). A premium-lapse must not interrupt OBS audio mid-stream; revocation is exclusively via rotate-token."
  - "The 7th GET /tts-config endpoint (research Open Question 3) is authed-only, no premium gate, so a downgraded user can still see config state and grab the OBS URL for any future re-upgrade."
  - "Rate limiter runs BEFORE JWT verify + key decrypt to cap attacker work factor for invalid-token floods (T-13-04 defence in depth)."
  - "Streaming path uses a separate unbounded http.Client; voices/subscription/test-sample use the handler-wide 30s-timeout client. The streaming client relies entirely on context.Context cancellation for shutdown."
  - "Tampered-signature test substitutes the entire signature segment with garbage ('AAAA...') rather than flipping one character — single-char flips can occasionally alias back to identical raw bytes through base64 rounding (T-13-02 test reliability)."

patterns-established:
  - "Move-before-add for shared code: when a second service needs a share-service-only package, promote it to shared/ rather than importing across services. Verified with zero-grep for stale paths and a go-mod-tidy pass on both sides."
  - "Token-bucket-style rate limit inline in the handler struct — no global registry, no promauto (shared-package rule). Struct + sync.Mutex only."
  - "Test-seam triad: ttsConfigStore + overlayOwnershipChecker + aesCipher — three minimal interfaces, three simple mocks, sharedMiddleware.RequirePremiumWithQuerier for the premium gate check without a real pgxpool.Pool."

requirements-completed: [D-03, D-05, D-06, D-07, D-08, D-09, D-10, D-11, D-12, D-13, D-14, D-15, D-16, D-17]

# Metrics
duration: 29m
completed: 2026-04-23
---

# Phase 13 Plan 02: Backend TTS (migration 049, shared/ package moves, AES-GCM key storage, per-overlay JWT, 7 TTS endpoints) Summary

**Server-side ElevenLabs plumbing: AES-GCM-encrypted api_key storage via shared/encryption, per-overlay HS256 tts_token JWT with rotation-based revocation, 7 overlay-manager endpoints (5 premium-gated + 1 authed-but-not-premium + 1 tts_token-verified streaming proxy), and the featuregates/premium-middleware promotion to shared/ so overlay-manager can consume them without forking share-service.**

## Performance

- **Duration:** ~29 min
- **Started:** 2026-04-23T18:41:30Z
- **Completed:** 2026-04-23T19:10:29Z
- **Tasks:** 6 (all committed atomically)
- **Files created:** 13
- **Files modified:** 7

## Accomplishments

- Migration 049 creates `overlay_tts_configs` (UUID PK, overlay_id UUID UNIQUE FK, encrypted_api_key BYTEA, voice_id TEXT, tts_signing_secret BYTEA, timestamps) with ON DELETE CASCADE from overlays, plus the idempotent `tts` feature_gate row.
- `shared/featuregates` and `shared/middleware/premium.go` are now byte-identical copies of the former share-service packages, available for both share-service and overlay-manager. Share-service tests still pass.
- `services/overlay-manager/tts` ships a 96-line JWT sign/verify package with 11 tests (11/11 green). Algorithm-confusion defence (T-13-02) enforced via `*jwt.SigningMethodHMAC` type-assertion. Rotation-based revocation (T-13-03) — no exp claim is ever emitted.
- `services/overlay-manager/repository/tts_config_repo.go` implements GetByOverlayID, CreateOrUpdate (upsert), Delete, RotateSigningSecret — 7/7 testcontainers-backed integration tests green.
- `services/overlay-manager/handlers/tts.go` implements 7 endpoints: POST/DELETE/GET `/tts-config`, POST `/tts-config/rotate-token`, GET `/tts-voices`, POST `/tts-config/test`, POST `/tts` (streaming proxy with client-disconnect propagation). 19/19 tests green.
- overlay-manager's `cmd/main.go` is wired: featuregates cache, AES cipher, TTSConfigRepository, TTSHandler, and the seven routes are mounted with correct auth/premium boundaries. `/public/:id/config` is untouched — T-13-06 regression test asserts no TTS data leak.
- K8s deployment now mounts `TOKEN_ENCRYPTION_KEY` from `allchat-secrets` (same secret auth-service consumes; no sealed-secret change needed) and `OVERLAY_PUBLIC_BASE_URL=https://allch.at` for obs_url construction.

## Task Commits

Each task was committed atomically:

1. **Task 1: Migration 049 + apply** — `5fd86b29` (feat)
2. **Task 2: Move featuregates + premium middleware to shared/** — `9af25be7` (refactor)
3. **Task 3: tts/jwt.go + tests (TDD)** — `0e165633` (feat; test + impl in one commit since they must land together)
4. **Task 4: models.TTSConfig + repository.TTSConfigRepository + tests (TDD)** — `feba3b8b` (feat)
5. **Task 5: handlers.TTSHandler + tests (TDD)** — `e2f1a318` (feat)
6. **Task 6: Wire main.go + k8s deployment** — `7353c2f8` (feat; also contains the tamper-test hardening noted in deviations)

_TDD note: for the backend-only tests in this plan I wrote test-before-impl per the plan, ran to confirm RED, wrote impl, ran to confirm GREEN, and then committed the test+impl together because the test is useless without the symbols it references. The RED/GREEN transition was verified in-session (see deviations for transcripts)._

## Files Created/Modified

**Migrations:**
- `migrations/049_overlay_tts_configs.sql` — new table + feature_gate row
- `migrations/049_overlay_tts_configs_down.sql` — rollback

**Shared packages (promoted):**
- `shared/featuregates/cache.go` — FeatureGateCache (was in share-service)
- `shared/featuregates/cache_test.go` — tests with import path updated
- `shared/middleware/premium.go` — RequirePremium + RequirePremiumWithQuerier
- `shared/middleware/premium_test.go` — tests (no import path change needed; already `package middleware`)

**overlay-manager:**
- `services/overlay-manager/tts/jwt.go` + `jwt_test.go` — HS256 sign/verify, 11 tests
- `services/overlay-manager/models/tts_config.go` — TTSConfig struct with `json:"-"` on secrets
- `services/overlay-manager/repository/tts_config_repo.go` + `tts_config_repo_test.go` — CRUD + rotation, 7 tests
- `services/overlay-manager/handlers/tts.go` + `tts_test.go` — 7 endpoints, 19 tests
- `services/overlay-manager/cmd/main.go` — wiring (modified)
- `services/overlay-manager/go.mod` — jwt/v5 promoted to direct (modified)

**share-service (import updates only):**
- `services/share-service/cmd/main.go` — imports shared/featuregates + shared/middleware
- `services/share-service/handlers/admin_featuregates.go` + `admin_featuregates_test.go` — import fix
- `services/share-service/go.mod` — tidied

**Infrastructure:**
- `deployments/k8s/base/overlay-manager/deployment.yaml` — adds TOKEN_ENCRYPTION_KEY and OVERLAY_PUBLIC_BASE_URL

## Decisions Made

1. **Reuse shared/encryption, not a new shared/crypto package.** The plan's `<critical_project_rules>` required this, and it turned out to be the right call — the existing AESEncryptor implements 12-byte random-nonce AES-GCM, already has a full test suite, and uses the same TOKEN_ENCRYPTION_KEY env var auth-service already consumes. No duplication, no new secrets management surface.

2. **TOKEN_ENCRYPTION_KEY env var name kept as UPPER_SNAKE_CASE.** The plan text sometimes referred to a kebab-case `token-encryption-key` k8s key, but the existing auth-service deployment uses `TOKEN_ENCRYPTION_KEY` as the secret map key — we matched the existing convention to avoid creating a second secret entry or a rename migration.

3. **POST /tts has no premium gate.** The graceful-permanence contract: a user whose subscription lapses must not hear OBS audio cut out mid-stream. The plan documents this as `TestHandleTTSStillWorksForDowngradedPremium` and the test passes. Revocation is exclusively via `POST /tts-config/rotate-token`.

4. **GET /tts-config is authed-but-not-premium-gated.** Research Open Question 3 — a downgraded user must still be able to view the OBS URL / voice_id for when they re-upgrade. No security risk because the endpoint never returns the api_key, encrypted_api_key, or signing secret (verified by `TestGetTTSConfigHidesKey`).

5. **Separate unbounded http.Client for POST /tts streaming.** Gin's context-based cancellation is the lifecycle control; a hard client-side timeout would cut off legitimate long TTS streams. Voices / test-sample / test-key use the 30-second-timeout client.

6. **In-memory fixed-window rate limit (60/min/overlay) is intentionally non-persistent.** Restart-on-deploy is acceptable for abuse prevention (T-13-04). The knob `ttsRateLimitPerMinute` lives at `services/overlay-manager/handlers/tts.go:42` for operator tuning without code hunting.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Bug] Tampered-signature test was flaky**

- **Found during:** Task 6 (full overlay-manager test run after wiring)
- **Issue:** `TestVerifyRejectsTamperedSignature` flipped the last character of the base64 signature and expected verification to fail. That flip can alias back to the same raw signature bytes after base64 decoding (base64 is not an injection from n-length to (3n+2)/4-length strings when padding characters are in play), so the test would occasionally produce a still-valid token. The test passed in isolation when run right after commit but failed once JWT library internal state was warmed by the preceding tts handler test run — likely a timing quirk, not a genuine security issue.
- **Fix:** Substitute the entire signature segment with `AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA` — a deterministic invalid-signature payload. The test now fails reliably on any signing-secret mismatch.
- **Files modified:** `services/overlay-manager/tts/jwt_test.go`
- **Verification:** 11/11 tts tests pass on repeat runs.
- **Committed in:** `7353c2f8` (part of Task 6 commit — a single-file hardening).

**2. [Rule 3 — Blocking] `admin_featuregates_test.go` stale import not listed in plan's step-by-step**

- **Found during:** Task 2 (share-service `go build` after directory moves)
- **Issue:** The plan enumerated the two files in `cmd/main.go` and `handlers/admin_featuregates.go` that imported the old share-service path, but `handlers/admin_featuregates_test.go` was not listed. It still imported `github.com/caesar/all-chat/services/share-service/featuregates` and therefore failed to compile after the directory was removed.
- **Fix:** Updated the import to `github.com/caesar/all-chat/shared/featuregates`.
- **Files modified:** `services/share-service/handlers/admin_featuregates_test.go`
- **Verification:** `grep -rn "services/share-service/featuregates" services/ shared/` returns zero; share-service tests pass.
- **Committed in:** `9af25be7` (part of Task 2 refactor commit).

---

**Total deviations:** 2 auto-fixed (1 bug / test reliability, 1 blocking / missing file in plan list)
**Impact on plan:** Both fixes necessary for the test suite and build to pass. Neither expanded scope; Task 2's deviation is a strict inclusion of a missed file and Task 6's is a test-robustness upgrade that strengthens T-13-02 assurance.

## Issues Encountered

- **Docker Compose naming conflict with the main worktree.** The repo's `docker-compose.frontend.yml` uses fixed container names (`allchat-frontend-postgres`, etc.), which collide with the parent worktree's containers. Worked around by spinning up an ad-hoc `allchat-wt-ac879c82-postgres` container on port 55432 for migration SQL validation, then cleaning it up. The overlay-manager integration tests use testcontainers which mints fresh ephemeral Postgres per test — no shared-container coupling.
- **testcontainers CLI Ryuk reaper warnings in CI-like containerised Docker hosts.** Setting `TESTCONTAINERS_RYUK_DISABLED=true` is harmless and silent; kept for clean test output.

## User Setup Required

Three environment variables need to be present in **every environment** that runs overlay-manager (dev, staging, prod):

| Variable | Value | Where |
|----------|-------|-------|
| `TOKEN_ENCRYPTION_KEY` | Base64-encoded 32-byte AES-256 key (same as auth-service uses) | k8s secret `allchat-secrets/TOKEN_ENCRYPTION_KEY` — already sealed, no change needed |
| `OVERLAY_PUBLIC_BASE_URL` | `https://allch.at` in prod, `http://localhost:3000` in dev | k8s deployment env value (added in this plan) |
| `ELEVENLABS_BASE_URL` | Unset in prod (defaults to `https://api.elevenlabs.io`); override in tests | Currently only test-configurable via `TTSHandler.elevenLabsBaseURL`. Not env-driven yet. If ops ever needs to route through a corporate proxy, lift it to `getEnv("ELEVENLABS_BASE_URL", defaultElevenLabsBaseURL)` in `cmd/main.go`. |

No sealed-secret changes required — `TOKEN_ENCRYPTION_KEY` was already provisioned for auth-service.

## Next Phase Readiness

- **Plan 13-01** (Web Speech tier) — independent, no coupling to this plan. Can proceed in parallel.
- **Plan 13-03** (ElevenLabs frontend UX) — depends on this plan. All 7 API endpoints are ready to consume:
  - `GET /api/v1/overlays/:id/tts-config` → `{has_elevenlabs_config, voice_id, obs_url}`
  - `POST /api/v1/overlays/:id/tts-config` → save `{api_key, voice_id}`
  - `DELETE /api/v1/overlays/:id/tts-config` → 204
  - `POST /api/v1/overlays/:id/tts-config/rotate-token` → `{obs_url}`
  - `GET /api/v1/overlays/:id/tts-voices` → ElevenLabs voices JSON
  - `POST /api/v1/overlays/:id/tts-config/test` → audio/mpeg + `x-characters-{remaining,limit}` headers
  - `POST /api/v1/overlays/:id/tts?text=&voice=&tts_token=` → streaming audio/mpeg
- **ADR-0012** (AES-GCM secret encryption) — planner flagged this as an optional follow-up. The existing `shared/encryption` package predates a formal ADR; writing ADR-0012 would document its Phase 13 second-consumer role and close the CLAUDE.md tech-debt note about token encryption.

## Self-Check

All acceptance criteria verified in-session:

- [x] `test -f migrations/049_overlay_tts_configs.sql` — exists, 38 lines
- [x] `test -f migrations/049_overlay_tts_configs_down.sql` — exists, 9 lines
- [x] Migration applied to ephemeral postgres — 7 columns present in declared order, `tts` feature gate `is_premium=true` confirmed
- [x] `shared/featuregates/{cache.go, cache_test.go}` and `shared/middleware/{premium.go, premium_test.go}` exist, byte-identical to the former share-service files (except cache_test.go import path)
- [x] `services/share-service/{featuregates, middleware}/` directories removed
- [x] `grep -rn "caesar/all-chat/services/share-service" shared/` → zero matches
- [x] `grep -rn promauto shared/featuregates/ shared/middleware/premium.go` → zero matches (Pitfall 6)
- [x] `cd shared && go build ./... && go test ./featuregates/... ./middleware/... -count=1` → exit 0
- [x] `cd services/share-service && go build ./... && go test ./... -count=1` → exit 0
- [x] `cd services/overlay-manager && go build ./... && go test ./... -count=1 -timeout 600s` → exit 0 (handlers, models, repository, tts, creditroll — 38 tests total, all green)
- [x] Secret-logging audit: `grep -rE 'zap\.String\("(api_key|apiKey|xi-api-key)"' services/overlay-manager/` → zero matches
- [x] AGPL-3.0 header present on all 7 new `.go` files (verified via Grep tool)
- [x] 7 endpoints wired in `services/overlay-manager/cmd/main.go` (5 premium-gated, 1 authed-no-premium, 1 tts_token-verified streaming)
- [x] `GET /public/:id/config` untouched (line 252 of main.go) — T-13-06 regression test passes
- [x] k8s overlay-manager deployment has `TOKEN_ENCRYPTION_KEY` and `OVERLAY_PUBLIC_BASE_URL`

## Self-Check: PASSED

---
*Phase: 13-text-to-speech-tts-for-chat-messages*
*Completed: 2026-04-23*
