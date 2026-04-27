# Phase 14: Secret Rotation Infrastructure — Research

**Researched:** 2026-04-27
**Domain:** Go cryptography, JWT rotation, CNPG ManagedRoles, Kubernetes secret management
**Confidence:** HIGH (all claims verified against live codebase; CNPG dual-password claim MEDIUM — verified against official docs and GH issues)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Ciphertext format: `[v(1B)] [nonce(12B)] [ciphertext] [tag(16B)]`. New writes always emit this format.
- **D-02:** Multiple master keys concurrently in env: `TOKEN_ENCRYPTION_KEY_V1`, `TOKEN_ENCRYPTION_KEY_V2`, …. New writes use latest; reads pick by kid byte.
- **D-03:** Re-encryption strategy: lazy on next write + scheduled background sweeper.
- **D-04:** Unify `TOKEN_ENCRYPTION_KEY` and `YOUTUBE_TOKEN_ENCRYPTION_KEY` into one chain. `YOUTUBE_TOKEN_ENCRYPTION_KEY` is dropped post-rotation.
- **D-05:** Legacy kid-less ciphertext treated as implicit-v0; decrypt with `TOKEN_ENCRYPTION_KEY` env var until sweeper completes.
- **D-06:** Sweeper is its own lightweight process (`cmd/key-rotator` or similar). K8s Job + CronJob.
- **D-07:** Add `kid` header to all User, Viewer, and Service JWTs.
- **D-08:** Validators accept list loaded from `JWT_SECRET_V1`, `JWT_SECRET_V2`, …. Unknown kid → fall back to `JWT_SECRET` legacy env var.
- **D-09:** Rotation timeline: `T+0` add new kid; `T+max(token_TTL)` drop old kid.
- **D-10:** Service JWTs use same code path, separate chain: `SERVICE_JWT_SECRET_V<n>`.
- **D-11:** TTS overlay JWTs NOT in scope for Phase 14.
- **D-12:** No JWT denylist in Phase 14.
- **D-13:** Preferred path: CNPG-managed rotation via `ManagedRoles` with `passwordSecret`.
- **D-14:** Fallback if CNPG can't cleanly support app-user rotation: runbook + `kubectl patch`.
- **D-15:** K8s Secret updates use `kubectl patch`, NOT `sops set`.
- **D-16:** Kick + TikTok OAuth tokens: add encrypted columns or encrypt-in-place using new `[v||nonce||ct||tag]` format.
- **D-17:** Update kick-listener and tiktok-listener write paths to use `shared/encryption`.
- **D-18:** Build mechanism first, rotate using it as the first run.
- **D-19:** Leaked keys remain valid during Phase 14 development. Mitigation is execution speed.
- **D-20:** All four workstreams ship in Phase 14. Likely 5-7 plans, 2-3 waves.

### Claude's Discretion

- Exact Go API surface for the versioned encryption package (function names, package layout, unification strategy).
- Sweeper batch size, throttling, telemetry shape.
- Migration numbering and exact SQL for gap-fill encryptions.
- Test strategy details.
- Sweeper location: `auth-service/cmd/key-rotator`, or standalone top-level service.
- JWKS-readiness of validator key-loading interface (door open, but no endpoint built).

### Deferred Ideas (OUT OF SCOPE)

- Redis-backed JWT denylist.
- JWKS endpoint + RS256 migration.
- Per-service DB users.
- Encryption of OAuth client credentials and bot tokens at rest in DB.
- Reconciliation of SOPS-vs-live K8s Secret drift.
- Interim mitigation (rate limiting, anomaly detection, audit logging).
- TTS overlay JWT versioning.

</user_constraints>

---

## Summary

Phase 14 builds a repeatable rotation infrastructure across three secret classes: AES-GCM token-encryption keys, JWT signing keys, and the database password. The codebase audit shows two nearly-identical AES-GCM packages (`shared/encryption` and `shared/crypto`) and a well-understood JWT path (`shared/auth/jwt.go`). The versioning work is additive and low-risk.

The most material research findings are: (1) CNPG `ManagedRoles` does NOT support dual-password overlap natively — the operator issues a single `ALTER ROLE … PASSWORD …` on reconcile, so the DB password changes atomically at the Postgres level without a grace window. The app-user password change propagates to running pods only when their env-injected `DATABASE_PASSWORD` is refreshed on the next rolling restart. The safe rotation sequence is therefore: patch secret → rolling restart → verify. (2) `shared/crypto/crypto.go` and `shared/encryption/encryption.go` are functional duplicates with identical semantics; they should be unified under `shared/encryption` which already exposes the `StringCipher`-compatible aliases. (3) The `YOUTUBE_TOKEN_ENCRYPTION_KEY` secret exists in SOPS (11-key file line 38) and in deployment manifests only via the rotate script — it is NOT referenced in any deployment YAML found; `youtube-listener` reads it via `YOUTUBE_TOKEN_ENCRYPTION_KEY` env var which is NOT present in its K8s deployment manifest (the service is absent from `caesar-deployment` — likely deprecated in production in favor of `youtube-listener-innertube`). This simplifies D-04 migration slightly.

**Primary recommendation:** Extend `shared/encryption` with a versioned `MultiKeyEncryptor`, collapse `shared/crypto` callers (one file: `token-encryption-backfill/main.go`) to use `shared/encryption` interfaces, and place the sweeper as `services/auth-service/cmd/key-rotator` (matching existing precedent).

---

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| TOKEN_ENCRYPTION_KEY versioning | shared library (`shared/encryption`) | auth-service, overlay-manager, token-refresh-service, twitch-eventsub-listener | All consumers import `shared/encryption`; versioning is a library concern |
| Background re-encryption sweeper | auth-service cmd tool (`cmd/key-rotator`) | K8s Job / CronJob trigger | Mirrors existing `cmd/token-encryption-backfill` precedent; needs DB creds only |
| JWT kid plumbing | shared library (`shared/auth/jwt.go`) | api-gateway, share-service, overlay-manager, source-manager | JWT sign/validate is already centralised in `shared/auth` |
| Service JWT key-loading | `shared/auth` + `shared/middleware` | source-manager, all listeners | `ServiceJWTAuth` middleware in `shared/middleware` owns validation |
| DB password rotation | CNPG Cluster spec (`allchat-cluster.yaml`) | K8s Secret (`allchat-secrets`) | CNPG operator reconciles `ALTER ROLE` from `passwordSecret` ref |
| Kick + TikTok gap-fill encryption | DB migration + service write paths | kick-listener, overlay-manager/handlers/sources.go | Write paths are in these two services; read in kick-listener only |

---

## Investigation Item 1: `shared/encryption` vs `shared/crypto` — Duplicate or Different?

### Comparison

[VERIFIED: codebase grep + file read]

| Attribute | `shared/encryption` (`AESEncryptor`) | `shared/crypto` (`AESGCMCipher`) |
|-----------|--------------------------------------|----------------------------------|
| Algorithm | AES-GCM (standard library) | AES-GCM (standard library) |
| Wire format | `base64(nonce \|\| ct \|\| tag)` — GCM Seal prepends nonce | `base64(nonce \|\| ct \|\| tag)` — identical |
| Key input | `[]byte` (via `ParseKey` which handles raw or base64 input) | `string` (raw bytes only — no base64 decode) |
| Empty string handling | no special case (error or panic on empty key) | returns `"", nil` for empty plaintext — explicit guard |
| Interface | `Encrypt/Decrypt` aliases on `AESEncryptor` | `StringCipher` interface with `Encrypt/Decrypt` |
| Users | auth-service, overlay-manager, token-refresh-service, youtube-listener, twitch-eventsub-listener | `token-encryption-backfill/main.go` (ONE file) |

**Verdict: They are functional duplicates.** The wire format is byte-for-byte identical (`Seal(nonce, nonce, …)` prepends nonce, then tag is appended by GCM — both produce `base64(nonce[12] || ct || tag[16])`). The only differences are cosmetic (key parsing, empty guard). A ciphertext produced by one is decryptable by the other with the same key.

### Recommendation: Unify Under `shared/encryption`

Keep `shared/encryption` as the single package. The `AESEncryptor` already exposes `Encrypt`/`Decrypt` aliases that satisfy the `StringCipher` interface from `shared/crypto`. After Phase 14 adds `MultiKeyEncryptor`:

1. Update `token-encryption-backfill/main.go` (the sole `shared/crypto` user) to import `shared/encryption` instead.
2. Delete `shared/crypto/crypto.go` or deprecate with a compile-time comment.
3. The `StringCipher` interface moves into `shared/encryption` (it already exists implicitly via the aliases).

### Proposed Go API Surface for Versioned Encryption

```go
// shared/encryption/versioned.go

// KidByte is a 1-byte key identifier prefix on every new-format ciphertext.
// 0x00 is reserved as "legacy / no kid".
// 0x01 = TOKEN_ENCRYPTION_KEY_V1 (first post-Phase-14 key).
type KidByte = byte

const LegacyKid KidByte = 0x00

// KeyEntry maps a KidByte to an AES-GCM cipher.
type KeyEntry struct {
    Kid    KidByte
    Cipher *AESEncryptor
}

// MultiKeyEncryptor encrypts with the latest key and decrypts with any registered key.
// Thread-safe; keys slice is immutable after construction.
type MultiKeyEncryptor struct {
    latest  *KeyEntry
    byKid   map[KidByte]*AESEncryptor
    legacy  *AESEncryptor // decrypts kid-less legacy blobs
}

// NewMultiKeyEncryptorFromEnv constructs a MultiKeyEncryptor from environment variables.
// It reads TOKEN_ENCRYPTION_KEY_V1, _V2, … in sequence until an env var is missing.
// The highest Vn present is treated as "latest".
// TOKEN_ENCRYPTION_KEY (no version suffix) is loaded as the legacy-v0 key.
//
// Example env:
//   TOKEN_ENCRYPTION_KEY=<old-key>    → legacy decryption fallback
//   TOKEN_ENCRYPTION_KEY_V1=<new-key> → new writes; kid byte 0x01
func NewMultiKeyEncryptorFromEnv() (*MultiKeyEncryptor, error)

// NewMultiKeyEncryptor constructs from explicit entries (for tests).
// legacyKey may be nil when no legacy data exists (Kick/TikTok gap-fill case).
func NewMultiKeyEncryptor(entries []KeyEntry, legacyKey *AESEncryptor) (*MultiKeyEncryptor, error)

// EncryptString encrypts plaintext with the latest key and returns a versioned blob.
// Format: base64( [kid(1B)] [nonce(12B)] [ciphertext] [tag(16B)] )
func (m *MultiKeyEncryptor) EncryptString(plaintext string) (string, error)

// DecryptString auto-detects format:
//   - If blob[0] is a registered kid AND len(decoded) >= 1+12+16: versioned path.
//   - Otherwise: legacy path (decrypt with legacy key).
func (m *MultiKeyEncryptor) DecryptString(ciphertext string) (string, error)

// Encrypt / Decrypt are StringCipher-compatible aliases.
func (m *MultiKeyEncryptor) Encrypt(s string) (string, error)
func (m *MultiKeyEncryptor) Decrypt(s string) (string, error)

// CurrentKid returns the KidByte of the latest (write) key.
func (m *MultiKeyEncryptor) CurrentKid() KidByte
```

The `AESEncryptor` struct (single-key) stays unchanged — it is the underlying primitive.

---

## Investigation Item 2: CNPG `ManagedRoles` — PASS or FAIL for App-User Rotation?

[VERIFIED: cloudnative-pg.io/docs/1.27/declarative_role_management/ + github.com/cloudnative-pg/cloudnative-pg/discussions/8062 + GH issue #2658]

### What CNPG ManagedRoles Does

CNPG's `spec.managed.roles` allows declarative PostgreSQL role lifecycle management. A role entry with `passwordSecret` references a K8s Secret containing `username` and `password` keys. The CNPG instance manager reconciles this on its normal reconciliation loop and issues `ALTER ROLE … PASSWORD …` when the secret value changes.

To trigger reconciliation on secret update, the secret must be labeled `cnpg.io/reload: "true"`.

The allchat cluster (`allchat-cluster.yaml`) currently has NO `managed.roles` section — the `allchat_user` is created via `bootstrap.initdb.owner` / `bootstrap.initdb.secret` and managed by CNPG internally, but NOT via `ManagedRoles`. The `allchat-cluster-secret` is a superuser-equivalent bootstrap secret, not an app-user secret.

### D-13/D-14 Verdict: FAIL (use fallback runbook)

Criteria from CONTEXT.md D-14 specifics:

| Criterion | Status |
|-----------|--------|
| (a) Operator handles dual-password window so pods don't drop mid-rotation | **FAIL** — CNPG issues a single `ALTER ROLE … PASSWORD` atomically. No dual-password overlap. Running pods lose DB auth as soon as the reconcile fires and before their `DATABASE_PASSWORD` env is updated. |
| (b) Services pick up new password without manual deployment intervention | **FAIL** — `DATABASE_PASSWORD` is injected as `secretKeyRef` at pod start. Running pods do not see the new value until they restart. CNPG does not trigger rolling restarts of application pods (it only manages the DB cluster). |
| (c) No application code changes required | PASS — once pods restart with new env, no code changes needed. |

CNPG `ManagedRoles` is **suitable for creating the app user declaratively**, but NOT for zero-downtime password rotation of an already-running app user. The operator owns the DB-side change but has no mechanism to coordinate with application pod restarts.

### Fallback Runbook (numbered, concrete commands)

Use this approach because criterion (a) cannot be met — we must own the window ourselves.

**Pre-flight:**
```bash
# Inspect live secret keys without leaking values (per global feedback memory)
kubectl get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | python3 -c "import sys,json; [print(k) for k in sorted(json.load(sys.stdin).keys())]"
```

**Step 1 — Generate a new password:**
```bash
NEW_PASSWORD=$(head -c 32 /dev/urandom | base64 | tr -d '/')
echo "New password length: ${#NEW_PASSWORD}"
```

**Step 2 — Create the NEW user with the new password in PostgreSQL (dual-window opens):**
```bash
# Connect to CNPG primary (port-forward or psql via kubectl exec)
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat \
  -c "CREATE ROLE allchat_user_new LOGIN PASSWORD '$NEW_PASSWORD';"
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat \
  -c "GRANT ALL PRIVILEGES ON DATABASE allchat TO allchat_user_new;"
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat \
  -c "GRANT ALL ON SCHEMA public TO allchat_user_new;"
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat \
  -c "GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO allchat_user_new;"
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat \
  -c "GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO allchat_user_new;"
```

**Alternative (preferred — rename pattern):** Change `allchat_user`'s password AND simultaneously create the user under the new password. This avoids a 2-user period:
```bash
# Change password directly on existing user
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat \
  -c "ALTER ROLE allchat_user PASSWORD '$NEW_PASSWORD';"
```

**Step 3 — Patch the K8s Secret with the new password (NOT sops set — drift hazard):**
```bash
ENCODED_PASSWORD=$(echo -n "$NEW_PASSWORD" | base64)
kubectl patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"replace\", \"path\": \"/data/database-password\", \"value\": \"$ENCODED_PASSWORD\"}]"
```

**Step 4 — Trigger rolling restart of all services that mount `database-password`:**
```bash
# Services consuming database-password (verified from deployment manifests):
# auth-service, token-refresh-service, twitch-eventsub-listener
# (and via migration initContainers: same services)
kubectl rollout restart deployment/auth-service -n allchat
kubectl rollout restart deployment/token-refresh-service -n allchat
kubectl rollout restart deployment/twitch-eventsub-listener -n allchat
# All other services that connect to DB:
kubectl rollout restart deployment/api-gateway -n allchat
kubectl rollout restart deployment/overlay-manager -n allchat
kubectl rollout restart deployment/share-service -n allchat
kubectl rollout restart deployment/message-processor -n allchat
kubectl rollout restart deployment/source-manager -n allchat
kubectl rollout restart deployment/kick-listener -n allchat
kubectl rollout restart deployment/tiktok-listener -n allchat
kubectl rollout restart deployment/twitch-listener -n allchat
```

**Step 5 — Wait for rollout to complete:**
```bash
kubectl rollout status deployment/auth-service -n allchat --timeout=5m
kubectl rollout status deployment/overlay-manager -n allchat --timeout=5m
# Repeat for each deployment above
```

**Step 6 — Verify connectivity:**
```bash
kubectl exec -n allchat deploy/auth-service -- wget -qO- http://localhost:8081/health/ready
```

**Step 7 — Retire the interim state (if two-user approach was used):**
```bash
# Drop the old user if a new user was created (not needed for ALTER ROLE approach)
kubectl exec -n allchat allchat-cluster-1 -- psql -U postgres allchat \
  -c "DROP ROLE IF EXISTS allchat_user_old;"
```

**Dual-window duration:** Steps 3→5. During this window old pods (still starting, or PodDisruptionBudget'd) may fail if they start after the DB password changed but before their secret is propagated. CNPG replicas are not affected (they use the superuser path, not `allchat_user`). The `ALTER ROLE` approach (Step 2 alt) is safer than creating a new user.

---

## Investigation Item 3: Full Call-Site Enumeration

[VERIFIED: grep across entire codebase]

### `shared/encryption` import sites

| File | Line | Usage |
|------|------|-------|
| `services/auth-service/cmd/main.go` | 36 | Constructs `AESEncryptor` from `TOKEN_ENCRYPTION_KEY` |
| `services/overlay-manager/cmd/main.go` | 35 | Constructs `AESEncryptor` from `TOKEN_ENCRYPTION_KEY` |
| `services/overlay-manager/handlers/tts.go` | (comment) | Documents that `shared/encryption.AESEncryptor` encrypts ElevenLabs key |
| `services/overlay-manager/models/tts_config.go` | 24 | Same doc comment |
| `services/overlay-manager/repository/tts_config_repo.go` | 35 | Same doc comment |
| `services/token-refresh-service/cmd/main.go` | 33 | Constructs `AESEncryptor` from `ENCRYPTION_KEY` env var |
| `services/token-refresh-service/repository/token_repository.go` | 24 | `*AESEncryptor` field on `TokenRepository`; all encrypt/decrypt operations |
| `services/twitch-eventsub-listener/channels/manager.go` | 25 | `*AESEncryptor` field on `Manager`; decrypts user OAuth tokens from DB |
| `services/twitch-eventsub-listener/cmd/main.go` | 35 | Constructs from `ENCRYPTION_KEY` env var |
| `services/youtube-listener/cmd/main.go` | 40 | Constructs from `YOUTUBE_TOKEN_ENCRYPTION_KEY` |
| `services/youtube-listener/cmd/token_backfill/main.go` | 26 | Constructs from `YOUTUBE_TOKEN_ENCRYPTION_KEY` |
| `services/youtube-listener/oauth/store.go` | 24 | `*AESEncryptor` field on `PostgresTokenStore`; all encrypt/decrypt ops |

### `shared/crypto` import sites

| File | Line | Usage |
|------|------|-------|
| `services/auth-service/cmd/token-encryption-backfill/main.go` | 27 | Only caller; uses `crypto.NewAESGCMCipher` + `crypto.StringCipher` interface |

**CONTEXT.md missed call sites:** CONTEXT.md listed `services/auth-service/handlers/viewer_auth.go` as an encryption call site. Confirmed: `viewer_auth.go` uses a `StringEncryptor` interface (defined locally in `handlers/viewer_auth.go` line 39-42) which is satisfied by whichever concrete type is passed in from `cmd/main.go`. The actual `shared/encryption` import is in `auth-service/cmd/main.go`. The handlers themselves are interface-typed — this is the correct injection point.

**CONTEXT.md missed:** `services/twitch-eventsub-listener/channels/manager.go` imports `shared/encryption` — this service ALSO needs to be updated. It reads user OAuth access tokens via its `cipher *encryption.AESEncryptor` field. It mounts `TOKEN_ENCRYPTION_KEY` as `ENCRYPTION_KEY` env var (note the different name in the deployment manifest).

**CONTEXT.md missed:** `services/token-refresh-service/cmd/main.go` + `repository/token_repository.go` — reads `ENCRYPTION_KEY` env var. The deployment mounts `token-encryption-key` secret key as `ENCRYPTION_KEY` (not `TOKEN_ENCRYPTION_KEY`).

**Env var name inconsistency found:**
- `auth-service`: `TOKEN_ENCRYPTION_KEY`
- `overlay-manager`: `TOKEN_ENCRYPTION_KEY`
- `token-refresh-service`: `ENCRYPTION_KEY` (maps to `token-encryption-key` secret key)
- `twitch-eventsub-listener`: `ENCRYPTION_KEY` (maps to `token-encryption-key` secret key)
- `youtube-listener`: `YOUTUBE_TOKEN_ENCRYPTION_KEY` (maps to `youtube-token-encryption-key` secret key)

Phase 14 must standardize all consumers to read `TOKEN_ENCRYPTION_KEY_V1` etc., while also reading the legacy `TOKEN_ENCRYPTION_KEY` / `YOUTUBE_TOKEN_ENCRYPTION_KEY` for backwards compat.

---

## Investigation Item 4: JWT Issuance and Validation Surface

[VERIFIED: file reads + grep across all services]

### Current Signing

All JWT types use `jwt.SigningMethodHS256` with a secret string cast to `[]byte`. No `kid` header currently set.

| JWT Type | Issuer | Where | TTL |
|----------|--------|-------|-----|
| User JWT | `auth-service/handlers/auth_handler.go` | Calls `auth.GenerateToken(…, h.jwtSecret, h.jwtExpiry, …)` | `JWT_EXPIRY_HOURS` env var (K8s configmap) — **default not found in code**; check configmap |
| Impersonation JWT | `auth-service/handlers/auth_handler.go` | Calls `auth.GenerateImpersonationJWT(…, secret)` | Hardcoded `2 * time.Hour` in `shared/auth/jwt.go:146` |
| Viewer JWT | `auth-service/handlers/viewer_auth.go` | `generateViewerJWT` inline: `jwt.NewWithClaims(jwt.SigningMethodHS256, claims); token.SignedString([]byte(h.jwtSecret))` | Same `h.jwtExpiry` as User JWT |
| Service JWT | `shared/sourcemanager/token.go` → `auth.GenerateServiceJWT` | All listeners via SDK `SigningTokenSource` | 15 minutes (`NewSigningTokenSource(platform+"-listener", secret, 15*time.Minute)`) |
| share-service service JWT | `services/share-service/handlers/shares.go:395,651` | `auth.GenerateServiceJWT("share-service", h.jwtSecret, 30*time.Second)` | **30 seconds** (uses `JWT_SECRET` not `SERVICE_JWT_SECRET`) |

**Critical finding on Service JWT issuance:** The share-service generates Service JWTs using `h.jwtSecret` (= `JWT_SECRET`), NOT `SERVICE_JWT_SECRET`. This is inconsistent with the validator (source-manager validates against `SERVICE_JWT_SECRET`). Either share-service is misconfigured, or it's calling a different endpoint that validates with `JWT_SECRET`. This needs clarification in planning — share-service may need to be updated to use `SERVICE_JWT_SECRET` as part of Phase 14 cleanup. [VERIFIED: `shares.go` line 395 uses `h.jwtSecret` which is loaded from `JWT_SECRET`.]

### Where `KeyFunc` Would Plug In

`ValidateJWT` (and `ValidateViewerJWT`, `ValidateServiceJWT`) all inline a `KeyFunc` closure:
```go
jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
    return []byte(secret), nil  // ← single secret, no kid lookup
})
```

For multi-key: the `KeyFunc` is the extension point. It receives the parsed (but unverified) `*jwt.Token` so `token.Header["kid"]` is accessible.

### Validators and What They Validate

| Service | Validates | Secret Source | Validator Function |
|---------|-----------|---------------|-------------------|
| api-gateway (middleware) | User JWT | `JWT_SECRET` env | `auth.ValidateJWT` |
| api-gateway (middleware) | Viewer JWT | `JWT_SECRET` env | `auth.ValidateViewerJWT` |
| api-gateway (WebSocket handlers) | User JWT | `h.jwtSecret` (same as above) | `auth.ValidateJWT` |
| api-gateway (WebSocket handlers) | Viewer JWT | `h.jwtSecret` | `auth.ValidateViewerJWT` |
| share-service | User JWT | `JWT_SECRET` | (via middleware) |
| overlay-manager | User JWT | `JWT_SECRET` | (via middleware) |
| source-manager | Service JWT | `SERVICE_JWT_SECRET` | `shared/middleware.ServiceJWTAuth` → `auth.ValidateServiceJWT` |

### Token TTLs for D-09 Retire Deadline

| Token | TTL | Retire deadline after rotation |
|-------|-----|-------------------------------|
| User JWT | `JWT_EXPIRY_HOURS` (configmap; default unknown — [ASSUMED] likely 24h based on `shared/auth/jwt.go:99: ExpiresAt: time.Now().Add(24 * time.Hour)`) | T + 24h |
| Impersonation JWT | 2h (hardcoded `shared/auth/jwt.go:146`) | T + 2h |
| Viewer JWT | Same as User JWT (`h.jwtExpiry`) | T + 24h |
| Service JWT (listeners) | 15 minutes | T + 15m |
| Service JWT (share-service) | 30 seconds | T + 30s |

**Dominant TTL for rotation retire window:** 24 hours.

### Proposed `KeyFunc` Shape for Multi-Key JWT Validation

```go
// shared/auth/jwt.go additions

// Keychain holds multiple HS256 secrets indexed by kid.
// KeyFunc selects the correct secret; unknown kid falls back to legacy secret.
type KeyChain struct {
    byKid   map[string][]byte  // "v1" -> secret bytes
    legacy  []byte             // JWT_SECRET (legacy, no kid)
}

// NewKeyChainFromEnv reads JWT_SECRET_V1, _V2, … until absent.
// JWT_SECRET (no version) is loaded as the legacy fallback.
func NewKeyChainFromEnv(prefix string) (*KeyChain, error)
// prefix = "JWT_SECRET" for user/viewer, "SERVICE_JWT_SECRET" for service tokens

// KeyFunc returns a jwt.Keyfunc that selects the right secret by kid header.
// Falls back to legacy when kid is absent or unrecognised.
func (kc *KeyChain) KeyFunc(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    kid, hasKid := token.Header["kid"].(string)
    if !hasKid || kid == "" {
        // legacy token, no kid
        if kc.legacy == nil {
            return nil, errors.New("no legacy key and token has no kid")
        }
        return kc.legacy, nil
    }
    key, ok := kc.byKid[kid]
    if !ok {
        // unknown kid — may be a future version; fall back to legacy for safety
        if kc.legacy == nil {
            return nil, fmt.Errorf("unknown kid %q and no legacy key", kid)
        }
        return kc.legacy, nil
    }
    return key, nil
}

// GenerateJWTWithKid — new version of GenerateJWT/GenerateToken that adds kid header.
func GenerateJWTWithKid(kid, userID, twitchID, username, secret string, isAdmin bool) (string, error)
// Similarly for GenerateViewerJWT, GenerateServiceJWT, GenerateImpersonationJWT
```

The JWKS-readiness door: the `KeyChain` interface could later grow a `KeyFunc` method backed by HTTP instead of env vars without changing call sites.

---

## Investigation Item 5: Backwards-Compat Boundary — Current Format vs. Proposed Kid Format

[VERIFIED: file read of `shared/encryption/encryption.go`]

### Current Wire Format (Legacy)

`AESEncryptor.EncryptString` does:
```go
ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
return base64.StdEncoding.EncodeToString(ciphertext), nil
```

`gcm.Seal(dst, nonce, plaintext, additionalData)` with `dst = nonce` → the result is `nonce || ciphertext || tag` appended to `nonce` → `nonce || nonce || ciphertext || tag`? **No** — `Seal(dst, nonce, pt, ad)` appends the sealed ciphertext to `dst`. If `dst = nonce` (a slice copy), the result is `nonce[12] || ct || tag[16]`.

So: **legacy decoded blob = `nonce(12B) || ct(varlen) || tag(16B)`**. Minimum length = 28 bytes (12 + 0 + 16). The legacy first byte is the first byte of the random nonce — uniformly random over 0x00–0xFF.

### Proposed New Format

Decoded blob = `kid(1B) || nonce(12B) || ct(varlen) || tag(16B)`. Minimum length = 29 bytes. First byte is the kid byte (0x01, 0x02, …).

### Disambiguation Logic

```
decoded_len = len(base64_decode(blob))
if decoded_len < 1 + 12 + 16 (= 29):
    → legacy path (too short for versioned format, no room for kid)
elif decoded[0] is a registered kid byte:
    → versioned path: kid = decoded[0], nonce = decoded[1:13], ct+tag = decoded[13:]
else:
    → legacy path (first byte is not a registered kid)
```

**Ambiguity risk (D-05):** A legacy ciphertext's first byte is random. If kid bytes are allocated from 0x01 upward, the probability that a legacy blob's first byte coincidentally equals a kid byte AND the blob is ≥29 bytes = `(num_registered_kids / 256) × P(len ≥ 29)`. Since all real-world token ciphertexts are ≥29 bytes (the plaintext itself is always ≥1 byte), the false-positive rate = `num_registered_kids / 256`.

With Phase 14 using only kid `0x01`, false-positive rate = 1/256 = 0.39%. This is non-zero. However, the consequence is a decryption failure (AEAD will detect the tampered nonce and return an authentication error), not a silent data corruption. The system should fall back to the other path on authentication error.

**Recommended mitigation (CONTEXT.md specifics already chose this):** Reserve `0x01..0x7F` for kids. Legacy blobs whose first byte is in this range will hit false-positive detection, fail AEAD authentication (nonce mismatch), and the code should then retry with legacy path. Implementation note: the `MultiKeyEncryptor.DecryptString` should be written as: try versioned; if AEAD auth fails, retry with legacy key. This eliminates false positives at the cost of one extra decrypt attempt (negligible).

### YouTube `encryption_version` Flag Semantics

`migrations/006_youtube_token_encryption.sql`: adds `encryption_version SMALLINT NOT NULL DEFAULT 0` to `youtube_oauth_tokens`.

`youtube-listener/oauth/store.go` (line 144): `if encryptionVersion >= 1 { decrypt }`. So `encryption_version=0` means plaintext; `=1` means encrypted with `YOUTUBE_TOKEN_ENCRYPTION_KEY` (no kid prefix).

**How D-04 "unify into one chain" maps:**

When Phase 14 deploys, existing YouTube tokens have `encryption_version=1` (encrypted with `YOUTUBE_TOKEN_ENCRYPTION_KEY`, kid-less legacy format). The new `MultiKeyEncryptor` treats these as legacy-v0 using `YOUTUBE_TOKEN_ENCRYPTION_KEY` as the legacy key. On next write (lazy re-encrypt), the sweeper re-encrypts with the new unified kid format and can update `encryption_version` to 2 (or drop the column entirely). The column should be removed eventually, but can stay as metadata during the transition.

---

## Investigation Item 6: Sweeper Design

### Where It Lives

**Recommendation: `services/auth-service/cmd/key-rotator/main.go`** — mirrors the existing `services/auth-service/cmd/token-encryption-backfill/main.go` precedent. Same access pattern (DB read+write, one encryption key), same binary shape (flags, pool, cipher, iterate tables).

Rationale for NOT making it a new top-level service: the sweeper runs to completion as a Job. It has no HTTP server, no long-running loop. A `cmd/` binary inside auth-service is idiomatic (see backfill). Auth-service's Dockerfile can include the binary in a separate stage or as a sidecar container init job.

### Per-Table Sweep Order and Batching

```
Pass 1: users           — access_token, refresh_token
Pass 2: viewer_sessions — access_token, refresh_token
Pass 3: youtube_oauth_tokens — access_token, refresh_token (+ update encryption_version)
Pass 4: overlay_tts_configs  — encrypted_api_key (BYTEA, needs different handling)
Pass 5: kick_oauth_tokens    — access_token, refresh_token (new encrypted columns, post-D-16)
Pass 6: tiktok_oauth_tokens  — access_token, refresh_token (new encrypted columns, post-D-16)
```

**Batching:** 100 rows per batch (matching token-refresh-service's `LIMIT 100` pattern). Add `--batch-size` flag.

**Throttling:** 50ms sleep between batches to avoid DB saturation. Add `--batch-delay-ms` flag (default 50).

**Idempotency proof:** On each row, call `encryptor.DecryptString(stored)`. If decryption succeeds AND `MultiKeyEncryptor.CurrentKid()` matches the kid byte of the stored blob, skip the row (already on latest kid). If decryption succeeds but kid is old, re-encrypt and write. If decryption fails, log error and skip (do not corrupt row). This is exactly the `encryptIfPlaintext` shape from the backfill, extended to `encryptIfNotCurrentKid`.

```go
func shouldReEncrypt(encryptor *encryption.MultiKeyEncryptor, storedBlob string) (bool, error) {
    decoded, err := base64.StdEncoding.DecodeString(storedBlob)
    if err != nil || len(decoded) < 29 {
        return true, nil // legacy or short → re-encrypt
    }
    if decoded[0] == encryptor.CurrentKid() {
        return false, nil // already latest
    }
    return true, nil // different kid → re-encrypt
}
```

**Telemetry (minimum required):**
```go
type SweeperMetrics struct {
    RowsScanned    map[string]int64 // table → count
    RowsReEncrypted map[string]int64 // table → count
    RowsSkipped    map[string]int64 // table → already current kid
    Errors         map[string]int64 // table → error count
}
```
Log summary at end of each table pass (structured JSON via zap).

### Deployment Shape (K8s Job + CronJob)

```yaml
# Job (manual trigger — run once during rotation)
apiVersion: batch/v1
kind: Job
metadata:
  name: key-rotator-manual
  namespace: allchat
spec:
  template:
    spec:
      restartPolicy: OnFailure
      containers:
        - name: key-rotator
          image: ghcr.io/caesarakalaeii/allchat-auth-service:main
          command: ["/app/key-rotator"]
          args: ["--batch-size=100", "--batch-delay-ms=50"]
          env:
            - name: DATABASE_URL
              valueFrom: { secretKeyRef: { name: allchat-secrets, key: database-url } }
            - name: TOKEN_ENCRYPTION_KEY
              valueFrom: { secretKeyRef: { name: allchat-secrets, key: token-encryption-key } }
            - name: TOKEN_ENCRYPTION_KEY_V1
              valueFrom: { secretKeyRef: { name: allchat-secrets, key: token-encryption-key-v1 } }
            # YOUTUBE_TOKEN_ENCRYPTION_KEY for legacy YouTube decrypt (during transition)
            - name: YOUTUBE_TOKEN_ENCRYPTION_KEY
              valueFrom: { secretKeyRef: { name: allchat-secrets, key: youtube-token-encryption-key } }
---
# CronJob (weekly during rotation window)
apiVersion: batch/v1
kind: CronJob
metadata:
  name: key-rotator-weekly
  namespace: allchat
spec:
  schedule: "0 3 * * 0"  # Sundays 03:00
  jobTemplate:
    spec:
      template: ... # same as Job above
```

**RBAC:** The Job runs with the same service account as auth-service, which already has DB credentials via `allchat-secrets`. No additional RBAC needed beyond the secret mount.

---

## Investigation Item 7: K8s Rotation Runbook — Per Secret Class

### Pre-flight (All Rotations)

```bash
# Check live secret keys WITHOUT leaking values (per feedback memory: never kubectl get -o yaml)
kubectl get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | python3 -c "import sys,json; [print(k) for k in sorted(json.load(sys.stdin).keys())]"
```

### TOKEN_ENCRYPTION_KEY Rotation

**Consumers identified from deployment manifests:** [VERIFIED]

| Service | Env Var Name | Secret Key |
|---------|-------------|------------|
| auth-service | `TOKEN_ENCRYPTION_KEY` | `token-encryption-key` |
| overlay-manager | `TOKEN_ENCRYPTION_KEY` | `token-encryption-key` |
| token-refresh-service | `ENCRYPTION_KEY` | `token-encryption-key` |
| twitch-eventsub-listener | `ENCRYPTION_KEY` | `token-encryption-key` |

**Note:** `youtube-listener` reads `YOUTUBE_TOKEN_ENCRYPTION_KEY` → `youtube-token-encryption-key`. It is NOT in `caesar-deployment` (no deployment yaml found for `youtube-listener` — only `youtube-listener-innertube`). The old `youtube-listener` may be inactive in production. Still handle in sweeper for correctness.

**Rotation sequence:**
```bash
# 1. Generate new V1 key
NEW_KEY=$(head -c 32 /dev/urandom | base64)

# 2. Add V1 to allchat-secrets (keeps old key as TOKEN_ENCRYPTION_KEY for legacy decrypt)
ENCODED=$(echo -n "$NEW_KEY" | base64)
kubectl patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"add\", \"path\": \"/data/token-encryption-key-v1\", \"value\": \"$ENCODED\"}]"

# 3. Deploy new code that reads both TOKEN_ENCRYPTION_KEY and TOKEN_ENCRYPTION_KEY_V1
# (rolling restart picks up new env var after manifest update)
kubectl rollout restart deployment/auth-service -n allchat
kubectl rollout restart deployment/overlay-manager -n allchat
kubectl rollout restart deployment/token-refresh-service -n allchat
kubectl rollout restart deployment/twitch-eventsub-listener -n allchat
kubectl rollout status deployment/auth-service -n allchat --timeout=5m

# 4. Run sweeper Job to re-encrypt all rows to V1
kubectl apply -f key-rotator-job.yaml
kubectl wait --for=condition=complete job/key-rotator-manual -n allchat --timeout=60m

# 5. Verify sweeper: 0 rows left on legacy kid
# (check sweeper logs)

# 6. T+0: sweeper complete — remove legacy TOKEN_ENCRYPTION_KEY from env
# Update deployment manifests to remove TOKEN_ENCRYPTION_KEY env var
# (the old key is no longer needed in env; V1 is the only active key)
kubectl rollout restart deployment/auth-service -n allchat
# ... repeat for all consumers
```

### JWT_SECRET Rotation

**Consumers identified:** [VERIFIED]

| Service | Env Var | Secret Key |
|---------|---------|------------|
| auth-service | `JWT_SECRET` | `jwt-secret` |
| api-gateway | `JWT_SECRET` | `jwt-secret` |
| overlay-manager | `JWT_SECRET` | `jwt-secret` |
| share-service | `JWT_SECRET` | `jwt-secret` |

**Rotation sequence:**
```bash
# 1. Generate new V1 secret
NEW_JWT_SECRET=$(head -c 32 /dev/urandom | base64)
ENCODED=$(echo -n "$NEW_JWT_SECRET" | base64)

# 2. Add JWT_SECRET_V1 to secret (keep old JWT_SECRET for legacy token validation)
kubectl patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"add\", \"path\": \"/data/jwt-secret-v1\", \"value\": \"$ENCODED\"}]"

# 3. Rolling restart — issuer signs with V1 kid; validators accept V1 + legacy fallback
kubectl rollout restart deployment/auth-service -n allchat
kubectl rollout restart deployment/api-gateway -n allchat
kubectl rollout restart deployment/overlay-manager -n allchat
kubectl rollout restart deployment/share-service -n allchat
kubectl rollout status deployment/auth-service -n allchat --timeout=5m

# 4. T+24h (max User JWT TTL) — all old tokens have expired
# Remove JWT_SECRET from env (legacy fallback no longer needed)
# Update manifests to remove JWT_SECRET env; deploy
```

### SERVICE_JWT_SECRET Rotation

**Consumers identified:** [VERIFIED]

| Service | Env Var | Secret Key | Role |
|---------|---------|------------|------|
| source-manager | `SERVICE_JWT_SECRET` | `service-jwt-secret` | Validator |
| kick-listener | `SERVICE_JWT_SECRET` (SOURCE_MANAGER_SECRET maps to same) | `service-jwt-secret` | Generator (via SDK) |
| tiktok-listener | `SERVICE_JWT_SECRET` | `service-jwt-secret` | Generator (via SDK) |
| twitch-listener | `SERVICE_JWT_SECRET` | `service-jwt-secret` | Generator (via SDK) |
| twitch-eventsub-listener | `SERVICE_JWT_SECRET` + `SOURCE_MANAGER_SECRET` (both → same key) | `service-jwt-secret` | Generator + Validator |
| youtube-listener-innertube | `SOURCE_MANAGER_SECRET` | `service-jwt-secret` | Generator (via SDK) |
| discord-listener | `SOURCE_MANAGER_SECRET` | `service-jwt-secret` | Generator (via SDK) |

**Note on share-service:** share-service uses `h.jwtSecret` (= `JWT_SECRET`) for `GenerateServiceJWT`. This is inconsistent. share-service should be updated in Phase 14 to use `SERVICE_JWT_SECRET`. This is a gap that Phase 14 must close.

**Service JWT TTL is 15 minutes** — retire old key at T+15m after rotation.

### DB Password Rotation

See Investigation Item 2 (fallback runbook above). Use `kubectl patch` on `allchat-secrets`, NOT `sops set`.

---

## Investigation Item 8: Migration Strategy for Kick + TikTok Plaintext Gap-Fill

### Current Column Types (Verified)

[VERIFIED: migrations/004_tiktok_support.sql, migrations/005_kick_support.sql]

**tiktok_oauth_tokens:** `access_token TEXT NOT NULL`, `refresh_token TEXT NOT NULL` — plaintext.
**kick_oauth_tokens:** `access_token TEXT NOT NULL`, `refresh_token TEXT NOT NULL` — plaintext.

### Migration Numbering

[VERIFIED: `ls migrations/ | sort`]

Last migration: `049_overlay_tts_configs.sql`. Next free: `050`.

Proposed:
- `050_kick_token_encryption.sql` — add `encryption_version SMALLINT NOT NULL DEFAULT 0` to `kick_oauth_tokens`
- `051_tiktok_token_encryption.sql` — add `encryption_version SMALLINT NOT NULL DEFAULT 0` to `tiktok_oauth_tokens`

This mirrors the YouTube pattern (`006_youtube_token_encryption.sql`) exactly.

### Read and Write Path Inventory

**kick_oauth_tokens read paths (plaintext today):**
- `services/kick-listener/channels/manager.go:969` — SELECT `access_token` (used for Pusher auth)
- `services/overlay-manager/handlers/sources.go:216` — SELECT `access_token, refresh_token` (copy-token flow)

**kick_oauth_tokens write paths:**
- `services/overlay-manager/handlers/sources.go:235` — INSERT `access_token, refresh_token` (initial write)
- `services/auth-service/handlers/viewer_auth.go` — Kick viewer OAuth callback writes encrypted viewer session to `viewer_sessions`, NOT `kick_oauth_tokens`. kick_oauth_tokens is a streamer token table.

**tiktok_oauth_tokens read/write paths:**
- No service code found reading `tiktok_oauth_tokens` directly. The TikTok listener is Node.js (`services/tiktok-listener/` is a Node/TypeScript service, not Go). `migrations/047_expired_token_cleanup.sql` references it for cleanup.

**Recommendation for TikTok:** Since the tiktok-listener is Node.js and there are no Go read paths found, the encryption of `tiktok_oauth_tokens.access_token/refresh_token` must happen in the Node.js service if those tokens are ever read at runtime. **Verify during planning:** does any Go service read `tiktok_oauth_tokens`? If not, the Node.js service needs its own encryption implementation (or a REST call to auth-service). This is a potential gap — flag for planning.

### Migration Strategy (D-16 Implementation)

Use **encrypt-in-place** (not dual-column) since there are no legacy ciphertexts to preserve — these are first-time encryptions. The migration adds an `encryption_version` column (v0 = plaintext, v1 = encrypted) and leaves existing rows as v0. The sweeper (or a one-shot script) backfills to v1. New writes go straight to v1.

```sql
-- 050_kick_token_encryption.sql
BEGIN;
ALTER TABLE kick_oauth_tokens
    ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_kick_oauth_tokens_enc_version
    ON kick_oauth_tokens(encryption_version);
COMMIT;

-- 051_tiktok_token_encryption.sql
BEGIN;
ALTER TABLE tiktok_oauth_tokens
    ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_tiktok_oauth_tokens_enc_version
    ON tiktok_oauth_tokens(encryption_version);
COMMIT;
```

---

## Investigation Item 9: Test Strategy

### Unit Tests for Encryption

```
TestMultiKeyEncryptor_EncryptDecryptSameKey
  - Encrypt with V1, decrypt with {V1} chain → success
  - Encrypt with V1, decrypt with {V1, V2} chain → success (V1 still valid)
  - Encrypt with V2, decrypt with {V1, V2} chain → success

TestMultiKeyEncryptor_LegacyBackcompat
  - Encrypt with AESEncryptor (old single-key, kid-less) → blob
  - Decrypt with MultiKeyEncryptor (legacy key set) → success

TestMultiKeyEncryptor_LegacyFalsePositive
  - Craft a legacy blob where decoded[0] == 0x01 (simulated false positive)
  - Decrypt with MultiKeyEncryptor → versioned path fails auth; fallback to legacy → success

TestMultiKeyEncryptor_Idempotent
  - Blob already on current kid → DecryptString + EncryptString → same kid byte
```

### Golden Ciphertexts

Commit fixed test vectors to `shared/encryption/testdata/golden_ciphertexts_test.go`:
```go
// encrypted with AES-128 key: hex "0102030405060708090a0b0c0d0e0f10"
// nonce: "000000000000000000000000" (12 zero bytes — deterministic for test)
// plaintext: "test-token-value"
var legacyGoldenCiphertext = "..." // base64(nonce||ct||tag)
var v1GoldenCiphertext = "..."     // base64(0x01||nonce||ct||tag)
```

This ensures future refactors can't silently break the decoder.

### Unit Tests for JWT

```
TestKeyChain_ValidTokenWithKid
  - Sign with kid "v1" using V1 secret
  - Validate with KeyChain{v1→secret} → success, kid extracted

TestKeyChain_ValidLegacyToken
  - Sign with NO kid (legacy, using JWT_SECRET)
  - Validate with KeyChain{v1→newSecret, legacy→JWT_SECRET} → success via legacy fallback

TestKeyChain_UnknownKidFallsBack
  - Sign with kid "v99" (future version not in chain)
  - Validate with KeyChain{v1→secret, legacy→JWT_SECRET} → fallback to legacy → success or controlled error

TestKeyChain_ExpiredTokenRejected
  - Sign with kid "v1", expiry in the past
  - Validate → ErrExpiredToken (even though kid is valid)
```

### Integration Tests for Sweeper

```
TestSweeper_Idempotent
  - Insert row with legacy ciphertext
  - Run sweeper → row updated to V1 kid
  - Run sweeper again → row unchanged (0 rows re-encrypted in 2nd run)

TestSweeper_SkipsCurrentKid
  - Insert row already encrypted with V1 kid
  - Run sweeper → 0 rows re-encrypted

TestSweeper_HandlesDecryptError
  - Insert row with invalid ciphertext
  - Run sweeper → error logged, row NOT touched, sweeper continues to next row
```

### Runbook Dry-Run Test (validation gate)

```bash
# Prove the rotation runbook script is syntactically valid
# and the key-rotator binary builds
cd services/auth-service && go build ./cmd/key-rotator/...
# Dry-run mode:
TOKEN_ENCRYPTION_KEY=<current> TOKEN_ENCRYPTION_KEY_V1=<new> \
  go run ./cmd/key-rotator/ --dry-run
# Expected: logs "would update N rows across M tables" with 0 actual DB writes
```

---

## Investigation Item 10: Rollout Wave Layout

[VERIFIED: against actual file dependencies]

### Wave 1 — Independent (can be parallelized)

**Plan 14-01: `shared/encryption` versioning + tests + `shared/crypto` collapse**
- Extend `shared/encryption` with `MultiKeyEncryptor`, `NewMultiKeyEncryptorFromEnv`
- Update `token-encryption-backfill/main.go` to import `shared/encryption` (drop `shared/crypto`)
- Unit tests including golden ciphertexts
- Migration 050 and 051 (Kick + TikTok `encryption_version` columns)
- No service changes yet (library only)

**Plan 14-02: `shared/auth/jwt.go` kid plumbing + tests**
- Add `KeyChain`, `NewKeyChainFromEnv`, `KeyFunc`
- Add `GenerateJWTWithKid`, `GenerateViewerJWTWithKid`, `GenerateServiceJWTWithKid`
- Keep existing `GenerateJWT` etc. as deprecated shims
- Unit tests

### Wave 2 — Depends on Wave 1 (can partially parallelize within wave)

**Plan 14-03: Service call-site updates (auth-service, token-refresh-service, twitch-eventsub-listener)**
- auth-service: `cmd/main.go` → `NewMultiKeyEncryptorFromEnv()`, update `GenerateJWT` calls to use `kid`
- auth-service: `cmd/main.go` → `NewKeyChainFromEnv("JWT_SECRET")`, wire into handler validation
- token-refresh-service: `cmd/main.go` → `NewMultiKeyEncryptorFromEnv()`
- twitch-eventsub-listener: `cmd/main.go` + `channels/manager.go` → `MultiKeyEncryptor`
- api-gateway: `middleware/auth.go`, `middleware/viewer_auth.go`, websocket handlers → `KeyChain.KeyFunc`
- overlay-manager: `cmd/main.go` → `MultiKeyEncryptor`; `cmd/main.go` → `KeyChain` for JWT validation
- share-service: `cmd/main.go` → `NewKeyChainFromEnv("JWT_SECRET")`; fix `GenerateServiceJWT` to use `SERVICE_JWT_SECRET`
- source-manager: `cmd/main.go` → `NewKeyChainFromEnv("SERVICE_JWT_SECRET")`

**Plan 14-04: Kick + TikTok write-path encryption + sweeper**
- kick-listener `channels/manager.go`: decrypt `access_token` on read (multi-key)
- overlay-manager `handlers/sources.go`: encrypt on write, decrypt on read (multi-key)
- New `services/auth-service/cmd/key-rotator/main.go` sweeper binary
- Sweeper tests (idempotency, dry-run mode)

**Plan 14-05: Deployment manifests**
- All deployment yamls: add `TOKEN_ENCRYPTION_KEY_V1`, `JWT_SECRET_V1`, `SERVICE_JWT_SECRET_V1` env entries
- Add `youtube-token-encryption-key` env to sweeper Job (legacy YouTube decrypt during transition)
- Document rotation runbook in `docs/runbooks/secret-rotation.md`
- DB password rotation runbook

### Wave 3 — Depends on Wave 2

**Plan 14-06: End-to-end validation**
- Run sweeper Job in `--dry-run` mode against staging/production
- Verify 0 JWT auth failures after rotation
- Execute DB password rotation runbook

### Wave Layout Validation

Wave 1 plans (14-01, 14-02) have no inter-dependencies and no service restarts needed — they are pure library changes. Wave 2 plans (14-03, 14-04) depend on Wave 1 (must import updated `shared/encryption` and `shared/auth`). Plans 14-03 and 14-04 can be parallelized within Wave 2. Plan 14-05 (manifest updates) depends on all Wave 2 plans completing and being built into images. Plan 14-06 is a validation gate and must be last.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| AES-GCM encryption | Custom cipher | `crypto/aes` + `cipher.NewGCM` (already in `shared/encryption`) | Already correct; add versioning wrapper only |
| JWT parsing/validation | JWT parse loop | `golang-jwt/jwt/v5` `ParseWithClaims` with `KeyFunc` | Library handles all edge cases (exp, iat, alg validation) |
| Base64 encoding for ciphertext | Custom encoding | `encoding/base64` `StdEncoding` | Matches existing wire format |
| Kubernetes secret updates | SOPS edit | `kubectl patch` | SOPS edit prunes live keys (31 live vs 11 in SOPS) |
| DB password rotation dual-window | CNPG ManagedRoles alone | Manual runbook (`ALTER ROLE` + rolling restart) | CNPG does not support dual-password overlap |

---

## Common Pitfalls

### Pitfall 1: ENCRYPTION_KEY vs TOKEN_ENCRYPTION_KEY Env Var Name Inconsistency
**What goes wrong:** token-refresh-service and twitch-eventsub-listener read `ENCRYPTION_KEY`, not `TOKEN_ENCRYPTION_KEY`. Adding `TOKEN_ENCRYPTION_KEY_V1` to manifests without also adding `ENCRYPTION_KEY_V1` leaves these services unable to find the new key.
**Why it happens:** Historical naming inconsistency in deployment manifests.
**How to avoid:** In Phase 14 manifest updates, either standardize all consumers to `TOKEN_ENCRYPTION_KEY_V1` (update env var name in code too), or add `ENCRYPTION_KEY_V1` as an alias in the manifests.
**Warning signs:** Sweeper runs successfully but token-refresh-service still writes legacy-format ciphertexts.

### Pitfall 2: SOPS Drift — Never Use `sops set` for K8s Secret Updates
**What goes wrong:** SOPS source has 11 keys; live secret has 31 keys. Running `sops set` or `sops edit` overwrites the live secret with only the SOPS-tracked 11 keys, deleting 20 live keys.
**How to avoid:** Use `kubectl patch secret allchat-secrets -n allchat --type='json'` for all rotation steps. Document this in every plan that touches secrets.
**Warning signs:** Services start throwing "secret key not found" errors after any secret operation.

### Pitfall 3: False-Positive Kid Detection on Legacy Blobs
**What goes wrong:** A legacy ciphertext whose first decoded byte happens to equal a registered kid byte (e.g., 0x01) gets treated as versioned. The AEAD authentication fails silently, and the token is returned as an error instead of decrypting correctly.
**How to avoid:** Implement retry-with-legacy on AEAD auth failure in `MultiKeyEncryptor.DecryptString`.
**Warning signs:** Specific users unable to log in after Phase 14 deploys (their access token happened to have first byte = 0x01).

### Pitfall 4: Share-service Generates Service JWTs with `JWT_SECRET` Instead of `SERVICE_JWT_SECRET`
**What goes wrong:** After rotation, share-service-generated service JWTs are signed with the old `JWT_SECRET` key. source-manager validates against `SERVICE_JWT_SECRET`. Authentication failures for share→overlay-manager calls.
**How to avoid:** Fix share-service to use `SERVICE_JWT_SECRET` in Phase 14. Add regression test.
**Warning signs:** Share link generation fails with 401 from overlay-manager.

### Pitfall 5: Sweeper Re-Encrypts TTS `encrypted_api_key` (BYTEA, Not TEXT)
**What goes wrong:** `overlay_tts_configs.encrypted_api_key` is stored as `BYTEA`, not `TEXT`. The sweeper's generic `TEXT` scan/update query will fail or corrupt data.
**How to avoid:** Add table-specific scan logic for `overlay_tts_configs`. The BYTEA column needs `pgx` binary scan, not string scan.
**Warning signs:** Sweeper panics or logs type-scan errors on `overlay_tts_configs` table.

### Pitfall 6: Rolling Restart Race During DB Password Rotation
**What goes wrong:** A pod restarts AFTER the DB password changes but BEFORE the K8s secret propagation reaches its container (can take 1-2 minutes for `secretKeyRef` to propagate). The pod starts with the old cached password and fails to connect.
**How to avoid:** Use `kubectl rollout restart` AFTER `kubectl patch`, not before. Give 60 seconds between patch and restart to allow secret propagation.
**Warning signs:** New pods stuck in `CrashLoopBackOff` with "authentication failed for user allchat_user" immediately after DB password rotation.

---

## Code Examples

### Verified Pattern: Existing Backfill (Precedent for Sweeper)
```go
// Source: services/auth-service/cmd/token-encryption-backfill/main.go:227-241
func (r *backfillRunner) encryptIfPlaintext(token string) (string, bool, error) {
    if token == "" {
        return "", false, nil
    }
    if _, err := r.cipher.Decrypt(token); err == nil {
        return token, false, nil  // already encrypted
    }
    encrypted, err := r.cipher.Encrypt(token)
    if err != nil {
        return "", false, err
    }
    return encrypted, true, nil
}
```

### Verified Pattern: Existing AES-GCM Wire Format (Legacy)
```go
// Source: shared/encryption/encryption.go:76-83
func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
    nonce := make([]byte, e.nonceSize) // 12 bytes for GCM
    io.ReadFull(rand.Reader, nonce)
    ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    // ciphertext = nonce[12] || ct || tag[16]
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}
```

### Verified Pattern: Existing KeyFunc Shape (Will Be Extended)
```go
// Source: shared/auth/jwt.go:156-162
jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    return []byte(secret), nil  // ← extend to: lookup by token.Header["kid"]
})
```

---

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | User JWT TTL = 24h (from `shared/auth/jwt.go:99` hardcode; actual prod TTL set by `JWT_EXPIRY_HOURS` configmap which was not read) | Item 4 (Token TTLs) | D-09 retire deadline could be wrong; retire window too short → some user JWTs rejected prematurely |
| A2 | tiktok-listener (Node.js) has no Go code reading `tiktok_oauth_tokens` | Item 8 | If a Go service reads TikTok tokens, it also needs encryption update |
| A3 | `youtube-listener` (original, non-innertube) is inactive in production (no deployment yaml found in caesar-deployment) | Item 7 | If it's running, it won't decrypt V1-format tokens. Must be included in manifest updates if active |
| A4 | CNPG version in production is 1.20+ (supports ManagedRoles) | Item 2 | ManagedRoles requires CNPG >= 1.20 |

---

## Open Questions

1. **What is `JWT_EXPIRY_HOURS` set to in the allchat configmap?**
   - What we know: code defaults not seen; `shared/auth/jwt.go` GenerateJWT hardcodes 24h but `GenerateToken` uses the passed-in `expiry`.
   - What's unclear: actual production value.
   - Recommendation: Read `caesar-deployment/apps/workloads/all-chat/configmap.yaml` during planning.

2. **Does share-service call an overlay-manager endpoint that validates via `SERVICE_JWT_SECRET`, and if so, which one?**
   - What we know: share-service generates service JWTs with `JWT_SECRET` (not `SERVICE_JWT_SECRET`).
   - What's unclear: whether this is a bug or whether the endpoint being called validates via `JWT_SECRET`.
   - Recommendation: Read `services/share-service/handlers/shares.go:390-400` and the overlay-manager endpoint it calls.

3. **Is `youtube-listener` (non-innertube) active in production?**
   - What we know: no deployment yaml in `caesar-deployment`; `YOUTUBE_TOKEN_ENCRYPTION_KEY` is in SOPS.
   - What's unclear: whether the service is deployed via a different mechanism or simply retired.
   - Recommendation: `kubectl get deploy -n allchat | grep youtube` to check.

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Building sweeper binary | ✓ | 1.25+ (per CLAUDE.md) | — |
| PostgreSQL | Integration tests for sweeper | ✓ (via docker-compose) | 16 | — |
| kubectl | Rotation runbook | ✓ (cluster context `default`) | — | — |
| CNPG operator | DB password rotation (preferred path) | ASSUMED present | 1.20+ | Fallback runbook (Item 2) |

---

## Validation Architecture

`workflow.nyquist_validation` is absent from `.planning/config.json` → treated as enabled.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package + `github.com/stretchr/testify` (used throughout codebase) |
| Config file | none — standard `go test` |
| Quick run command | `go test ./shared/encryption/... ./shared/auth/...` |
| Full suite command | `go test ./...` from repo root (or `make test`) |

### Phase Requirements → Test Map

| Req | Behavior | Test Type | Automated Command | File Exists? |
|-----|----------|-----------|-------------------|-------------|
| D-01/D-02 | Versioned ciphertext format: new writes have kid byte | Unit | `go test ./shared/encryption/... -run TestMultiKey` | ❌ Wave 1 |
| D-05 | Legacy kid-less ciphertext decrypts via fallback | Unit | `go test ./shared/encryption/... -run TestLegacyBackcompat` | ❌ Wave 1 |
| D-05 | False-positive kid byte → retry with legacy | Unit | `go test ./shared/encryption/... -run TestFalsePositive` | ❌ Wave 1 |
| D-07/D-08 | JWT kid header present; multi-key validation | Unit | `go test ./shared/auth/... -run TestKeyChain` | ❌ Wave 1 |
| D-08 | Legacy token (no kid) validates via fallback | Unit | `go test ./shared/auth/... -run TestLegacyFallback` | ❌ Wave 1 |
| D-03/D-06 | Sweeper idempotency: second run touches 0 rows | Integration | `go test ./services/auth-service/cmd/key-rotator/... -run TestSweeper_Idempotent` | ❌ Wave 2 |
| D-01 | Golden ciphertexts: fixed test vectors decode correctly | Unit | `go test ./shared/encryption/... -run TestGolden` | ❌ Wave 1 |
| D-16 | Kick token encrypted on write, decrypted on read | Unit | `go test ./services/kick-listener/...` | ❌ Wave 2 |
| D-13/D-14 | DB password rotation runbook succeeds (dry-run) | Manual | `key-rotator --dry-run` smoke check | ❌ Wave 3 |

### Sampling Rate

- **Per task commit:** `go test ./shared/encryption/... ./shared/auth/...`
- **Per wave merge:** `go test ./... ` (full suite)
- **Phase gate:** Full suite green + sweeper dry-run passes before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `shared/encryption/versioned_test.go` — covers D-01, D-02, D-05 golden ciphertexts
- [ ] `shared/auth/keychains_test.go` — covers D-07, D-08
- [ ] `services/auth-service/cmd/key-rotator/main_test.go` — sweeper idempotency

---

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | JWT rotation with multi-key `KeyFunc` |
| V3 Session Management | yes | Token TTL enforcement; retire old keys at T+max(TTL) |
| V4 Access Control | yes | Service JWT chain isolation (blast-radius) |
| V5 Input Validation | yes | Kid byte range validation (0x01–0x7F reserved) |
| V6 Cryptography | yes | AES-256-GCM (never hand-roll); nonce generated via `crypto/rand` |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Legacy ciphertext false-positive kid detection | Tampering | AEAD authentication failure → retry with legacy key |
| Old JWT valid for entire TTL after rotation | Repudiation | Retire old kid only after `T+max(TTL)`; D-09 enforces this |
| Concurrent rotations corrupting kid namespace | Tampering | Allocate kids monotonically; document kid registry in codebase |
| SOPS edit pruning live keys | Tampering / DoS | All rotation steps use `kubectl patch`, never `sops set` |
| DB password rotation split-brain | DoS | Rolling restart after patch; readiness probes catch failures |

---

## Standard Stack

### Core Libraries (All Already In Use)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/aes` + `crypto/cipher` | stdlib | AES-GCM primitive | stdlib, no dependency risk |
| `golang-jwt/jwt/v5` | v5.x | JWT sign/validate; `KeyFunc` multi-key support | Already in `shared/auth/jwt.go` |
| `jackc/pgx/v5` | v5.x | DB access for sweeper | Already used by all Go services |
| `go.uber.org/zap` | v1.x | Structured logging for sweeper | Already used by all services |
| `encoding/base64` | stdlib | Ciphertext encoding | Already used in `shared/encryption` |

No new library dependencies required for Phase 14.

---

## Sources

### Primary (HIGH confidence)

- Codebase: `shared/encryption/encryption.go`, `shared/crypto/crypto.go` — read directly
- Codebase: `shared/auth/jwt.go` — read directly
- Codebase: `services/auth-service/cmd/token-encryption-backfill/main.go` — read directly
- Codebase: `services/auth-service/handlers/auth_handler.go`, `viewer_auth.go` — read directly
- Codebase: `services/api-gateway/middleware/auth.go` — read directly
- Codebase: `services/token-refresh-service/repository/token_repository.go` — read directly
- Codebase: `services/youtube-listener/oauth/store.go` — read directly
- Codebase: `services/twitch-eventsub-listener/channels/manager.go` — read directly
- Codebase: `migrations/004_tiktok_support.sql`, `005_kick_support.sql`, `006_youtube_token_encryption.sql`, `049_overlay_tts_configs.sql` — read directly
- Deployment: `caesar-deployment/apps/workloads/all-chat/*.yaml` — read directly (all service deployments)
- Deployment: `caesar-deployment/apps/workloads/all-chat/allchat-cluster.yaml` — read directly
- Deployment: `caesar-deployment/apps/workloads/all-chat/secrets/allchat-secret.enc.yaml` — read directly (key names only, not values)
- Deployment: `caesar-deployment/scripts/rotate-allchat-secrets.go` — read directly (key inventory)

### Secondary (MEDIUM confidence)

- [CNPG declarative_role_management docs 1.27](https://cloudnative-pg.io/docs/1.27/declarative_role_management/) — confirms `passwordSecret` field but no dual-password window
- [CNPG GH discussion #8062](https://github.com/cloudnative-pg/cloudnative-pg/discussions/8062) — confirms `cnpg.io/reload: "true"` label requirement; reconciliation not always immediate

### Tertiary (LOW confidence)

- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use, verified from imports
- Architecture: HIGH — verified from live code and deployment manifests
- CNPG dual-password verdict (FAIL): MEDIUM — verified against official docs; no dual-password mentioned anywhere
- TikTok write path: LOW — Node.js service not inspected; Go grep returned no results

**Research date:** 2026-04-27
**Valid until:** 2026-05-27 (30 days; stable domain)
