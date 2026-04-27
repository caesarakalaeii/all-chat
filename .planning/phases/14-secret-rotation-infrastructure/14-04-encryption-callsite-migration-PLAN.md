---
phase: 14-secret-rotation-infrastructure
plan: 04
type: execute
wave: 2
depends_on:
  - "14-01"
files_modified:
  - services/auth-service/cmd/main.go
  - services/auth-service/handlers/viewer_auth.go
  - services/overlay-manager/cmd/main.go
  - services/overlay-manager/models/tts_config.go
  - services/token-refresh-service/cmd/main.go
  - services/token-refresh-service/repository/token_repository.go
  - services/twitch-eventsub-listener/cmd/main.go
  - services/twitch-eventsub-listener/channels/manager.go
  - services/youtube-listener/cmd/main.go
  - services/youtube-listener/cmd/token_backfill/main.go
  - services/youtube-listener/oauth/store.go
autonomous: true
decisions_addressed:
  - D-02
  - D-04
  - D-05
must_haves:
  truths:
    - "Every Go service that imports shared/encryption holds a *MultiKeyEncryptor (not *AESEncryptor) at runtime"
    - "All encrypt-on-write paths produce versioned ciphertext with kid prefix == CurrentKid()"
    - "All decrypt-on-read paths transparently handle versioned and legacy (kid-less) ciphertext"
    - "auth-service viewer_auth handler still accepts a StringEncryptor interface; *MultiKeyEncryptor satisfies it via Encrypt/Decrypt aliases"
  artifacts:
    - path: "services/auth-service/cmd/main.go"
      provides: "tokenCipher constructed via encryption.NewMultiKeyEncryptorFromEnv()"
      contains: "NewMultiKeyEncryptorFromEnv"
    - path: "services/youtube-listener/oauth/store.go"
      provides: "PostgresTokenStore.enc field type changed to *encryption.MultiKeyEncryptor"
      contains: "*encryption.MultiKeyEncryptor"
    - path: "services/twitch-eventsub-listener/channels/manager.go"
      provides: "Manager.cipher field type changed to *encryption.MultiKeyEncryptor"
      contains: "*encryption.MultiKeyEncryptor"
    - path: "services/token-refresh-service/repository/token_repository.go"
      provides: "TokenRepository.cipher field type changed to *encryption.MultiKeyEncryptor"
      contains: "*encryption.MultiKeyEncryptor"
  key_links:
    - from: "services/auth-service/cmd/main.go"
      to: "services/auth-service/handlers/viewer_auth.go (StringEncryptor)"
      via: "interface satisfaction — *MultiKeyEncryptor provides Encrypt/Decrypt aliases"
      pattern: "Encrypt.*Decrypt"
    - from: "services/youtube-listener/cmd/main.go"
      to: "shared/encryption.NewMultiKeyEncryptorFromEnv (D-04 unified chain reads YOUTUBE_TOKEN_ENCRYPTION_KEY as legacy)"
      via: "env-driven constructor reads YOUTUBE_TOKEN_ENCRYPTION_KEY for legacy YouTube ciphertext decryption"
      pattern: "NewMultiKeyEncryptorFromEnv"
---

<objective>
Migrate every encryption call site that today holds a `*encryption.AESEncryptor` to hold a `*encryption.MultiKeyEncryptor`. The wire-format change is transparent at the call site (same `EncryptString`/`DecryptString`/`Encrypt`/`Decrypt` method names), so this plan is mostly type-substitution + constructor swap. The behavioral change: writes now produce versioned ciphertext with a `kid` prefix; reads transparently handle both formats.

Purpose: Implements decisions D-02 (multi-key chain in env), D-04 (unified TOKEN_ENCRYPTION_KEY + YOUTUBE_TOKEN_ENCRYPTION_KEY chain), D-05 (legacy backwards-compat via fallback).

Scope (per RESEARCH.md §3 call-site enumeration):
- auth-service (`cmd/main.go` constructor; `handlers/viewer_auth.go` interface field unchanged — `*MultiKeyEncryptor` satisfies it)
- overlay-manager (`cmd/main.go` constructor; `models/tts_config.go` interface field type)
- token-refresh-service (`cmd/main.go` constructor; `repository/token_repository.go` field type)
- twitch-eventsub-listener (`cmd/main.go` constructor; `channels/manager.go` field type)
- youtube-listener (`cmd/main.go` constructor; `cmd/token_backfill/main.go` constructor; `oauth/store.go` field type)

Output: All five service binaries compile against `*MultiKeyEncryptor`. Existing tests still pass (call signatures are unchanged on the public methods).
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
@shared/encryption/versioned.go
@shared/encryption/encryption.go
@services/auth-service/cmd/main.go
@services/auth-service/handlers/viewer_auth.go
@services/overlay-manager/cmd/main.go
@services/overlay-manager/models/tts_config.go
@services/token-refresh-service/cmd/main.go
@services/token-refresh-service/repository/token_repository.go
@services/twitch-eventsub-listener/cmd/main.go
@services/twitch-eventsub-listener/channels/manager.go
@services/youtube-listener/cmd/main.go
@services/youtube-listener/cmd/token_backfill/main.go
@services/youtube-listener/oauth/store.go

<interfaces>
<!-- The MultiKeyEncryptor's public surface, identical to AESEncryptor for drop-in replacement. -->

From shared/encryption/versioned.go (Plan 14-01):
```go
type MultiKeyEncryptor struct { /* ... */ }
func NewMultiKeyEncryptorFromEnv() (*MultiKeyEncryptor, error)
func (m *MultiKeyEncryptor) EncryptString(plaintext string) (string, error)
func (m *MultiKeyEncryptor) DecryptString(ciphertext string) (string, error)
func (m *MultiKeyEncryptor) Encrypt(s string) (string, error)
func (m *MultiKeyEncryptor) Decrypt(s string) (string, error)
func (m *MultiKeyEncryptor) CurrentKid() KidByte
```

From services/auth-service/handlers/viewer_auth.go (interface preserved — no change needed):
```go
type StringEncryptor interface {
    Encrypt(plaintext string) (string, error)
    Decrypt(ciphertext string) (string, error)
}
// *MultiKeyEncryptor satisfies this via the Encrypt/Decrypt aliases.
```

Existing callsite struct fields that get type-substituted:
```go
// services/youtube-listener/oauth/store.go
type PostgresTokenStore struct { db *pgxpool.Pool; enc *encryption.AESEncryptor; logger *zap.Logger }
//                                                            ^^^ change to *MultiKeyEncryptor

// services/token-refresh-service/repository/token_repository.go
type TokenRepository struct { /*...*/; cipher *encryption.AESEncryptor }
//                                              ^^^ change to *MultiKeyEncryptor

// services/twitch-eventsub-listener/channels/manager.go
type Manager struct { /*...*/; cipher *encryption.AESEncryptor }
//                                      ^^^ change to *MultiKeyEncryptor
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Migrate auth-service + token-refresh-service + twitch-eventsub-listener encryption call sites</name>
  <files>services/auth-service/cmd/main.go, services/token-refresh-service/cmd/main.go, services/token-refresh-service/repository/token_repository.go, services/twitch-eventsub-listener/cmd/main.go, services/twitch-eventsub-listener/channels/manager.go</files>
  <read_first>
    - shared/encryption/versioned.go (created in Plan 14-01)
    - services/auth-service/cmd/main.go lines 102–142 (current single-key bootstrap)
    - services/auth-service/handlers/viewer_auth.go lines 38–50 (StringEncryptor interface — unchanged)
    - services/token-refresh-service/cmd/main.go full file
    - services/token-refresh-service/repository/token_repository.go full file
    - services/twitch-eventsub-listener/cmd/main.go full file
    - services/twitch-eventsub-listener/channels/manager.go (lines 1–60 + the cipher field declaration)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §3 "Full Call-Site Enumeration" (lines 280–322) and Pitfall 1 "ENCRYPTION_KEY vs TOKEN_ENCRYPTION_KEY" (lines 902–907)
    - .planning/phases/14-secret-rotation-infrastructure/14-PATTERNS.md "services/auth-service/cmd/main.go" + "services/youtube-listener/oauth/store.go" sections (lines 319–342, 398–429)
  </read_first>
  <behavior>
    - Existing unit tests for each service still pass (call signatures of internal interfaces unchanged).
    - For services/twitch-eventsub-listener: compile-time check `var _ encryption.StringCipherLike = (*encryption.MultiKeyEncryptor)(nil)` may not be necessary, but a `go vet` clean run proves type compatibility.
    - The `tokenCipher` / `cipher` / `enc` field constructed in each service's `cmd/main.go` is non-nil at server start (verified by existing startup tests / smoke tests).
  </behavior>
  <action>
    Step 1 — auth-service `cmd/main.go` (lines 102, 104, 130–142):

    a) Delete the lines reading `tokenEncryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")` and the `if tokenEncryptionKey == ""` fatal check, AND the `parsedKey, err := encryption.ParseKey(...)` block, AND `tokenCipher, err := encryption.NewAESEncryptor(parsedKey)`.

    b) Replace with one line:
    ```go
    tokenCipher, err := encryption.NewMultiKeyEncryptorFromEnv()
    if err != nil {
        log.Fatal("failed to initialize token cipher (TOKEN_ENCRYPTION_KEY_V1 must be set; legacy TOKEN_ENCRYPTION_KEY optional)", zap.Error(err))
    }
    log.Info("token cipher initialized", zap.Uint8("current_kid", tokenCipher.CurrentKid()))
    ```

    c) Verify `tokenCipher` is passed to handlers as `StringEncryptor` (the existing interface in viewer_auth.go) — no further changes needed since `*MultiKeyEncryptor` satisfies the interface.

    d) NOTE: do NOT touch `jwtSecret` / `jwtExpiry` here. JWT keychain wiring is Plan 14-05's scope.

    Step 2 — token-refresh-service `cmd/main.go`:

    a) Find the line `cipher, err := encryption.NewAESEncryptor(parsedKey)` (or similar single-key construction reading `ENCRYPTION_KEY` env var).

    b) Replace the whole block with:
    ```go
    cipher, err := encryption.NewMultiKeyEncryptorFromEnv()
    if err != nil {
        log.Fatal("failed to initialize encryption", zap.Error(err))
    }
    ```

    c) IMPORTANT — Pitfall 1: this service today reads `ENCRYPTION_KEY`, not `TOKEN_ENCRYPTION_KEY`. The new `NewMultiKeyEncryptorFromEnv()` reads `TOKEN_ENCRYPTION_KEY_V1` and `TOKEN_ENCRYPTION_KEY` (legacy). Plan 14-07 fixes the deployment manifest to mount `token-encryption-key` as `TOKEN_ENCRYPTION_KEY` (instead of `ENCRYPTION_KEY`) and adds `TOKEN_ENCRYPTION_KEY_V1`. This task code-side already assumes the new env names. If the manifest is not yet updated when this task lands, the service WILL fail to start in production until Plan 14-07 ships — that is the intended sequencing (this is Wave 2; manifest changes are Wave 3).

    Step 3 — token-refresh-service `repository/token_repository.go`:

    a) Find the field `cipher *encryption.AESEncryptor`.

    b) Change to `cipher *encryption.MultiKeyEncryptor`.

    c) Find the constructor `NewTokenRepository(...)` — change the matching parameter type from `*encryption.AESEncryptor` to `*encryption.MultiKeyEncryptor`.

    d) Method bodies (`r.cipher.Encrypt(...)`, `r.cipher.Decrypt(...)`) are unchanged.

    Step 4 — twitch-eventsub-listener `cmd/main.go`:

    a) Apply the same pattern as token-refresh-service Step 2. The current code reads `ENCRYPTION_KEY` env var. Replace with `encryption.NewMultiKeyEncryptorFromEnv()`.

    b) Same Pitfall 1 caveat: deployment manifest fix is Plan 14-07.

    Step 5 — twitch-eventsub-listener `channels/manager.go`:

    a) Find the field `cipher *encryption.AESEncryptor` (line 25 per PATTERNS.md).

    b) Change to `cipher *encryption.MultiKeyEncryptor`.

    c) Update the `Manager` constructor parameter type accordingly.

    d) Method bodies that call `m.cipher.DecryptString(...)` are unchanged.

    Step 6 — Verify compilation across all three services:
    ```
    cd /home/moersener/Hobby/all-chat && go build ./services/auth-service/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/...
    ```

    Step 7 — Run service unit tests:
    ```
    cd /home/moersener/Hobby/all-chat && go test ./services/auth-service/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/... -count=1
    ```
    Existing tests should continue to pass because public method signatures (Encrypt/Decrypt/EncryptString/DecryptString) are unchanged. If a test constructs `*AESEncryptor` directly to inject as a dependency, update the test to use `encryption.NewMultiKeyEncryptor(...)` with a single-entry slice — pattern:
    ```go
    cipher, _ := encryption.NewAESEncryptor(testKey32)
    multiCipher, _ := encryption.NewMultiKeyEncryptor(
        []encryption.KeyEntry{{Kid: 0x01, Cipher: cipher}},
        nil,
    )
    repo := repository.NewTokenRepository(db, multiCipher)
    ```
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go build ./services/auth-service/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/... && go test ./services/auth-service/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `grep -q "encryption.NewMultiKeyEncryptorFromEnv" services/auth-service/cmd/main.go`
    - `! grep -q "encryption.NewAESEncryptor" services/auth-service/cmd/main.go` (legacy single-key constructor removed from cmd path)
    - `! grep -q "tokenEncryptionKey := os.Getenv" services/auth-service/cmd/main.go` (env var preflight removed — constructor handles it)
    - `grep -q "encryption.NewMultiKeyEncryptorFromEnv" services/token-refresh-service/cmd/main.go`
    - `grep -q "cipher \*encryption.MultiKeyEncryptor" services/token-refresh-service/repository/token_repository.go`
    - `grep -q "encryption.NewMultiKeyEncryptorFromEnv" services/twitch-eventsub-listener/cmd/main.go`
    - `grep -q "cipher \*encryption.MultiKeyEncryptor" services/twitch-eventsub-listener/channels/manager.go`
    - `cd /home/moersener/Hobby/all-chat && go build ./services/auth-service/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/...` exits 0
    - `cd /home/moersener/Hobby/all-chat && go test ./services/auth-service/... ./services/token-refresh-service/... ./services/twitch-eventsub-listener/... -count=1` exits 0
  </acceptance_criteria>
  <done>auth-service, token-refresh-service, twitch-eventsub-listener all build and test green against `*MultiKeyEncryptor`. The viewer_auth StringEncryptor interface remains unchanged. Existing tests pass without behavioral regressions.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Migrate overlay-manager and youtube-listener encryption call sites</name>
  <files>services/overlay-manager/cmd/main.go, services/overlay-manager/models/tts_config.go, services/youtube-listener/cmd/main.go, services/youtube-listener/cmd/token_backfill/main.go, services/youtube-listener/oauth/store.go</files>
  <read_first>
    - services/overlay-manager/cmd/main.go (full — current single-key construction)
    - services/overlay-manager/models/tts_config.go (full — interface field for ElevenLabs key encryption)
    - services/youtube-listener/cmd/main.go (full — reads YOUTUBE_TOKEN_ENCRYPTION_KEY)
    - services/youtube-listener/cmd/token_backfill/main.go (full — separate binary, also single-key today)
    - services/youtube-listener/oauth/store.go lines 38–80 + 144–170 (struct definition, constructor, encrypt/decrypt with encryption_version gate)
    - .planning/phases/14-secret-rotation-infrastructure/14-RESEARCH.md §1 "Recommendation" (D-04 unification — `YOUTUBE_TOKEN_ENCRYPTION_KEY` becomes legacy fallback in the unified chain)
  </read_first>
  <action>
    Step 1 — overlay-manager `cmd/main.go`:

    a) Find the existing single-key construction. Per RESEARCH.md §3 line 35: `parsedKey, err := encryption.ParseKey(tokenEncryptionKey)` followed by `NewAESEncryptor`. Replace with `encryption.NewMultiKeyEncryptorFromEnv()` exactly like auth-service Step 1.

    b) Verify the cipher is passed to handlers/repos via the `StringEncryptor`-like interface in `tts_config.go`.

    Step 2 — overlay-manager `models/tts_config.go`:

    a) Inspect the type for the encrypted-API-key field (today likely `*encryption.AESEncryptor` per RESEARCH.md). If it's an interface, no change needed — `*MultiKeyEncryptor` satisfies the same Encrypt/Decrypt contract. If it's a concrete type, change to `*encryption.MultiKeyEncryptor`.

    Step 3 — youtube-listener `cmd/main.go`:

    a) Replace `parsedKey/NewAESEncryptor` block with `encryption.NewMultiKeyEncryptorFromEnv()`.

    b) IMPORTANT (D-04): The constructor automatically picks up `YOUTUBE_TOKEN_ENCRYPTION_KEY` as a legacy fallback (per Plan 14-01 design). No code changes needed beyond the constructor swap — the unification is handled inside `NewMultiKeyEncryptorFromEnv`.

    c) Update the log message to indicate which keys are loaded:
    ```go
    log.Info("YouTube token cipher initialized — unified chain reads TOKEN_ENCRYPTION_KEY_V<n>; legacy fallback also reads YOUTUBE_TOKEN_ENCRYPTION_KEY",
        zap.Uint8("current_kid", cipher.CurrentKid()))
    ```

    Step 4 — youtube-listener `cmd/token_backfill/main.go`:

    a) This is the original Phase-13 backfill tool (separate from `services/auth-service/cmd/token-encryption-backfill`). It currently reads `YOUTUBE_TOKEN_ENCRYPTION_KEY` directly. Replace with `encryption.NewMultiKeyEncryptorFromEnv()` for consistency.

    b) NOTE: The new sweeper from Plan 14-06 supersedes this binary. Keep the binary in place for historical reproducibility but use the unified constructor.

    Step 5 — youtube-listener `oauth/store.go`:

    a) Change struct field `enc *encryption.AESEncryptor` → `enc *encryption.MultiKeyEncryptor` (line 40).

    b) Change `NewPostgresTokenStore(db *pgxpool.Pool, enc *encryption.AESEncryptor, logger *zap.Logger)` parameter type to `*encryption.MultiKeyEncryptor`.

    c) Method bodies (lines 55–73 encrypt-on-write; lines 144–170 decrypt-on-read with `if encryptionVersion >= 1`) are unchanged. The `encryption_version >= 1` gate semantics stay: row=0 is plaintext, row≥1 is encrypted (legacy or versioned — `MultiKeyEncryptor.DecryptString` auto-detects).

    Step 6 — Compile and test:
    ```
    cd /home/moersener/Hobby/all-chat && go build ./services/overlay-manager/... ./services/youtube-listener/... && go test ./services/overlay-manager/... ./services/youtube-listener/... -count=1
    ```

    Step 7 — Mock-update note: if `services/overlay-manager/repository/tts_config_repo_test.go` or similar test files construct `*encryption.AESEncryptor` directly for mock injection, they must be updated to use `encryption.NewMultiKeyEncryptor(...)` with a one-entry slice (same pattern as Task 1 Step 7).
  </action>
  <verify>
    <automated>cd /home/moersener/Hobby/all-chat && go build ./services/overlay-manager/... ./services/youtube-listener/... && go test ./services/overlay-manager/... ./services/youtube-listener/... -count=1</automated>
  </verify>
  <acceptance_criteria>
    - `grep -q "encryption.NewMultiKeyEncryptorFromEnv" services/overlay-manager/cmd/main.go`
    - `grep -q "encryption.NewMultiKeyEncryptorFromEnv" services/youtube-listener/cmd/main.go`
    - `grep -q "encryption.NewMultiKeyEncryptorFromEnv" services/youtube-listener/cmd/token_backfill/main.go`
    - `grep -q "enc \*encryption.MultiKeyEncryptor\|cipher \*encryption.MultiKeyEncryptor" services/youtube-listener/oauth/store.go`
    - `! grep -q "encryption.NewAESEncryptor" services/youtube-listener/cmd/main.go`
    - `cd /home/moersener/Hobby/all-chat && go build ./services/overlay-manager/... ./services/youtube-listener/...` exits 0
    - `cd /home/moersener/Hobby/all-chat && go test ./services/overlay-manager/... ./services/youtube-listener/... -count=1` exits 0
  </acceptance_criteria>
  <done>overlay-manager and youtube-listener build and test green against `*MultiKeyEncryptor`. The unified chain (D-04) — TOKEN_ENCRYPTION_KEY_V<n> latest, TOKEN_ENCRYPTION_KEY + YOUTUBE_TOKEN_ENCRYPTION_KEY legacy fallback — is implicit via the constructor.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| K8s Secret → process env (TOKEN_ENCRYPTION_KEY_V<n>, legacy TOKEN_ENCRYPTION_KEY, YOUTUBE_TOKEN_ENCRYPTION_KEY) | Each service constructs `*MultiKeyEncryptor` from env at start; missing env → fatal log + exit |
| Plaintext OAuth tokens → DB ciphertext | Encrypt-on-write paths now produce versioned ciphertext with kid prefix |
| DB ciphertext → plaintext OAuth tokens | Decrypt-on-read paths transparently handle versioned + legacy; AEAD auth failure on versioned path falls back to legacy |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-14-04-01 | Tampering | Mismatched env var names — Plan 14-07 hasn't shipped, code reads `TOKEN_ENCRYPTION_KEY_V1`, deployment still mounts `ENCRYPTION_KEY` | accept | Documented sequencing: Wave 2 (this plan) ships code; Wave 3 (Plan 14-07) ships deployment manifest. Production deployment of services rebuilt after Wave 2 will fail until Wave 3 ships. CI gates this via build, not deployment. The team must merge 14-07 before redeploying any of these services to production. |
| T-14-04-02 | Information Disclosure | Constructor log message printing env var values | mitigate | The log message in each service prints `tokenCipher.CurrentKid()` (a single byte) and the env var NAMES, never values |
| T-14-04-03 | Tampering | Test fixtures bypassing the new constructor and re-using `*AESEncryptor` directly, masking integration regressions | mitigate | Tests that construct cipher directly are updated to use `encryption.NewMultiKeyEncryptor(entries, legacyKeys)` with explicit kid; Task 1 Step 7 + Task 2 Step 7 pattern |
| T-14-04-04 | Repudiation | Existing encrypted rows fail to decrypt because the legacy fallback wasn't wired | mitigate | `NewMultiKeyEncryptorFromEnv` reads BOTH `TOKEN_ENCRYPTION_KEY` and `YOUTUBE_TOKEN_ENCRYPTION_KEY` as legacy fallbacks (proven by TestNewMultiKeyEncryptorFromEnv_YouTubeLegacy in Plan 14-01); youtube-listener migration in this plan does NOT need to special-case YouTube — the unified chain handles it |
| T-14-04-05 | Spoofing | Forged ciphertext crafted to trigger the wrong fallback path | mitigate | AEAD authentication is verified by every key in the chain; only a key that matches the actual ciphertext can succeed. False-positive kid byte falls through to legacy keys per Plan 14-01 design |
</threat_model>

<verification>
- All five service binaries compile after the migration.
- All existing service unit tests pass without behavioral changes (public method names preserved).
- `grep` counts in acceptance_criteria prove the substitution.
- Wave 3 (Plan 14-07) is the gate before production deploy — documented in T-14-04-01.
</verification>

<success_criteria>
- Every service that previously held `*encryption.AESEncryptor` now holds `*encryption.MultiKeyEncryptor`.
- New writes produce versioned ciphertext (kid byte present); reads transparently handle both versioned and legacy.
- D-04 unification is implicit via `NewMultiKeyEncryptorFromEnv()` reading `YOUTUBE_TOKEN_ENCRYPTION_KEY` as legacy.
- Tests pass; build is green.
</success_criteria>

<output>
After completion, create `.planning/phases/14-secret-rotation-infrastructure/14-04-SUMMARY.md` documenting:
- Final list of services migrated and their cipher field type changes.
- Confirmation that StringEncryptor interface in viewer_auth.go is unchanged (interface satisfied by *MultiKeyEncryptor via Encrypt/Decrypt aliases).
- Pitfall 1 acknowledgment: token-refresh-service and twitch-eventsub-listener code now reads `TOKEN_ENCRYPTION_KEY_V1` while their deployment manifests still mount `ENCRYPTION_KEY` — flag for Plan 14-07 to reconcile.
- Note for Plan 14-05: kick-listener and overlay-manager handlers/sources.go encryption gap-fill (D-16/D-17 code-side) is the NEXT step in Wave 2.
- Note for Plan 14-06: the new sweeper supersedes `services/youtube-listener/cmd/token_backfill/main.go`; the old binary remains compiled but is not part of the rotation runbook.
</output>
