# Phase 14: Secret Rotation Infrastructure - Pattern Map

**Mapped:** 2026-04-27
**Files analyzed:** 18 new/modified files
**Analogs found:** 17 / 18

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `shared/encryption/versioned.go` (NEW) | encryption-primitive | transform | `shared/encryption/encryption.go` | exact |
| `shared/crypto/crypto.go` (DELETE/deprecate) | encryption-primitive | transform | `shared/encryption/encryption.go` | exact |
| `shared/auth/jwt.go` (MODIFY) | jwt-primitive | request-response | `shared/auth/jwt.go` itself (extend) | self |
| `services/auth-service/cmd/key-rotator/main.go` (NEW) | sweeper | batch | `services/auth-service/cmd/token-encryption-backfill/main.go` | exact |
| `services/auth-service/cmd/main.go` (MODIFY) | jwt-issuer, encryption-callsite | request-response | self | self |
| `services/auth-service/handlers/auth_handler.go` (MODIFY) | jwt-issuer | request-response | `services/auth-service/cmd/main.go` lines 102–142 | role-match |
| `services/auth-service/handlers/viewer_auth.go` (MODIFY) | jwt-issuer | request-response | `services/auth-service/cmd/main.go` lines 102–142 | role-match |
| `services/api-gateway/middleware/auth.go` (MODIFY) | jwt-validator | request-response | `services/api-gateway/middleware/auth.go` itself | self |
| `shared/middleware/service_auth.go` (MODIFY) | jwt-validator | request-response | `shared/middleware/service_auth.go` itself | self |
| `services/share-service/cmd/main.go` (MODIFY) | jwt-validator, jwt-issuer | request-response | `services/api-gateway/middleware/auth.go` | role-match |
| `services/overlay-manager/cmd/main.go` (MODIFY) | jwt-validator, encryption-callsite | request-response | `services/auth-service/cmd/main.go` lines 102–142 | role-match |
| `services/youtube-listener/oauth/store.go` (MODIFY) | encryption-callsite | CRUD | `services/youtube-listener/oauth/store.go` itself | self |
| `services/token-refresh-service/repository/token_repository.go` (MODIFY) | encryption-callsite | CRUD | `services/youtube-listener/oauth/store.go` | role-match |
| `services/twitch-eventsub-listener/channels/manager.go` (MODIFY) | encryption-callsite | CRUD | `services/youtube-listener/oauth/store.go` | role-match |
| `services/kick-listener/channels/manager.go` (MODIFY) | encryption-callsite | CRUD | `services/youtube-listener/oauth/store.go` | role-match |
| `services/overlay-manager/handlers/sources.go` (MODIFY) | encryption-callsite | CRUD | `services/youtube-listener/oauth/store.go` | role-match |
| `migrations/050_kick_token_encryption.sql` (NEW) | migration | batch | `migrations/006_youtube_token_encryption.sql` | exact |
| `migrations/051_tiktok_token_encryption.sql` (NEW) | migration | batch | `migrations/006_youtube_token_encryption.sql` | exact |
| `caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml` (MODIFY) | k8s-deployment | config | self | self |
| `docs/runbooks/secret-rotation.md` (NEW) | runbook-doc | — | `docs/migrations/2025-02-auth-token-encryption.md` | role-match |

---

## Pattern Assignments

### `shared/encryption/versioned.go` (NEW — encryption-primitive, transform)

**Analog:** `shared/encryption/encryption.go`

**Existing imports block** (lines 17–27):
```go
package encryption

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "fmt"
    "io"
)
```

**Existing `AESEncryptor` struct + constructor pattern** (lines 34–55) — `MultiKeyEncryptor` wraps this:
```go
type AESEncryptor struct {
    gcm       cipher.AEAD
    nonceSize int
}

func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
    // ... validate key length, NewCipher, NewGCM
    return &AESEncryptor{gcm: gcm, nonceSize: gcm.NonceSize()}, nil
}
```

**Existing encrypt/decrypt core pattern** (lines 76–113) — Phase 14 adds a 1-byte kid prefix around this:
```go
func (e *AESEncryptor) EncryptString(plaintext string) (string, error) {
    nonce := make([]byte, e.nonceSize)
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("generate nonce: %w", err)
    }
    ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *AESEncryptor) DecryptString(ciphertext string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    // ... split nonce / payload, gcm.Open
    return string(plaintext), nil
}
```

**What changes for Phase 14:** `versioned.go` introduces `MultiKeyEncryptor` that prepends a 1-byte kid to the encoded blob on encrypt and dispatches by kid byte on decrypt. The existing `AESEncryptor` remains unchanged as the per-key primitive. `ParseKey` (lines 57–74) is reused verbatim for loading each versioned key from env.

**New API surface to implement** (from RESEARCH.md §1 proposed API):
```go
// versioned.go — same package: encryption

type KidByte = byte

const LegacyKid KidByte = 0x00

type KeyEntry struct {
    Kid    KidByte
    Cipher *AESEncryptor
}

type MultiKeyEncryptor struct {
    latest *KeyEntry
    byKid  map[KidByte]*AESEncryptor
    legacy *AESEncryptor // nil when no legacy data exists (Kick/TikTok new columns)
}

// NewMultiKeyEncryptorFromEnv — reads TOKEN_ENCRYPTION_KEY_V1, _V2, …; TOKEN_ENCRYPTION_KEY as legacy
func NewMultiKeyEncryptorFromEnv() (*MultiKeyEncryptor, error)

// NewMultiKeyEncryptor — explicit entries, for tests
func NewMultiKeyEncryptor(entries []KeyEntry, legacyKey *AESEncryptor) (*MultiKeyEncryptor, error)

// EncryptString — base64([kid(1B)][nonce(12B)][ct][tag(16B)])
func (m *MultiKeyEncryptor) EncryptString(plaintext string) (string, error)

// DecryptString — try versioned path; fallback to legacy on AEAD auth error
func (m *MultiKeyEncryptor) DecryptString(ciphertext string) (string, error)

// Encrypt/Decrypt — StringCipher-compatible aliases
func (m *MultiKeyEncryptor) Encrypt(s string) (string, error)
func (m *MultiKeyEncryptor) Decrypt(s string) (string, error)

// CurrentKid — returns kid byte of the latest (write) key
func (m *MultiKeyEncryptor) CurrentKid() KidByte
```

**Error handling pattern** — copy from `encryption.go`:
```go
var (
    ErrEmptyKey        = errors.New("encryption key cannot be empty")
    ErrInvalidKeyBytes = errors.New("encryption key must be 16, 24, or 32 bytes")
)
// Wrap errors with fmt.Errorf("context: %w", err) — never swallow.
```

---

### `shared/crypto/crypto.go` (DELETE/deprecate)

**Analog:** `shared/encryption/encryption.go`

`shared/crypto` is a functional duplicate of `shared/encryption` (identical wire format; see RESEARCH.md §1). Its sole caller is `services/auth-service/cmd/token-encryption-backfill/main.go` (one file). Phase 14 migrates that one caller to `shared/encryption` and deletes `shared/crypto`.

**What changes:** Replace `crypto.NewAESGCMCipher` → `encryption.NewAESEncryptor` + `encryption.ParseKey`. Replace `crypto.StringCipher` interface references with `*encryption.MultiKeyEncryptor` (which satisfies the same `Encrypt/Decrypt` contract).

---

### `shared/auth/jwt.go` (MODIFY — jwt-primitive, request-response)

**Analog:** `shared/auth/jwt.go` itself (lines 1–239)

**Existing signing pattern** (lines 85–105) — to be wrapped by a `kid`-aware version:
```go
func GenerateJWT(userID, twitchID, username, secret string, isAdmin bool) (string, error) {
    claims := Claims{
        UserID: userID, TwitchID: twitchID, Username: username, Roles: roles,
        RegisteredClaims: jwt.RegisteredClaims{
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            Issuer:    "all-chat",
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(secret))
}
```

**Existing KeyFunc pattern** (lines 156–170) — to be extended with kid dispatch:
```go
func ValidateJWT(tokenString, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(secret), nil  // ← single secret; Phase 14 replaces with KeyChain.KeyFunc
    })
    if errors.Is(err, jwt.ErrTokenExpired) {
        return nil, ErrExpiredToken
    }
    // ...
}
```

**New additions to implement** (from RESEARCH.md §4 proposed API):
```go
// KeyChain holds multiple HS256 secrets indexed by string kid ("v1", "v2", …).
type KeyChain struct {
    byKid  map[string][]byte
    legacy []byte // JWT_SECRET (no version suffix) — fallback
}

// NewKeyChainFromEnv reads JWT_SECRET_V1, _V2, … until absent.
// prefix = "JWT_SECRET" for user/viewer; "SERVICE_JWT_SECRET" for service tokens.
func NewKeyChainFromEnv(prefix string) (*KeyChain, error)

// KeyFunc returns a jwt.Keyfunc selecting secret by token.Header["kid"].
// Falls back to legacy when kid is absent or unrecognised.
func (kc *KeyChain) KeyFunc(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    kid, hasKid := token.Header["kid"].(string)
    if !hasKid || kid == "" {
        if kc.legacy == nil {
            return nil, errors.New("no legacy key and token has no kid")
        }
        return kc.legacy, nil
    }
    key, ok := kc.byKid[kid]
    if !ok {
        if kc.legacy == nil {
            return nil, fmt.Errorf("unknown kid %q and no legacy key", kid)
        }
        return kc.legacy, nil
    }
    return key, nil
}
```

**New generate function with kid header** — copy existing `GenerateJWT` and add `token.Header["kid"] = kid` before `SignedString`:
```go
// GenerateJWTWithKid sets the kid header in addition to all existing claims.
// kid is a string like "v1", "v2" matching the KeyChain entry.
func GenerateJWTWithKid(kid, userID, twitchID, username, secret string, isAdmin bool) (string, error) {
    // ... same claims construction as GenerateJWT ...
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    token.Header["kid"] = kid
    return token.SignedString([]byte(secret))
}
// Replicate for GenerateViewerJWTWithKid, GenerateServiceJWTWithKid, GenerateImpersonationJWTWithKid
```

**What changes:** Add `KeyChain` type + `NewKeyChainFromEnv` + `KeyFunc`. Add `GenerateJWT*WithKid` variants. Existing `GenerateJWT`/`ValidateJWT` signatures remain for now (not broken until callers are updated to pass `KeyChain`).

---

### `services/auth-service/cmd/key-rotator/main.go` (NEW — sweeper, batch)

**Analog:** `services/auth-service/cmd/token-encryption-backfill/main.go` (lines 1–242, full file)

**Imports/structure pattern** (lines 19–29):
```go
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/caesar/all-chat/shared/encryption"  // change: was shared/crypto
    "github.com/jackc/pgx/v5/pgxpool"
)
```

**Runner struct pattern** (lines 31–35):
```go
type backfillRunner struct {
    pool   *pgxpool.Pool
    cipher crypto.StringCipher  // change: *encryption.MultiKeyEncryptor
    dryRun bool
}
```

**Flag + env bootstrap pattern** (lines 37–88):
```go
func main() {
    dryRun := flag.Bool("dry-run", false, "log rows that would be updated without mutating the database")
    // Phase 14 adds: batchSize, batchDelayMs, skipKick, skipTikTok, etc.
    flag.Parse()

    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" { log.Fatal("DATABASE_URL must be set") }

    // Phase 14: NewMultiKeyEncryptorFromEnv() instead of single-key cipher
    encryptor, err := encryption.NewMultiKeyEncryptorFromEnv()
    if err != nil { log.Fatalf("failed to build encryptor: %v", err) }

    pool, err := pgxpool.New(ctx, dbURL)
    if err != nil { log.Fatalf("failed to create database pool: %v", err) }
    defer pool.Close()
}
```

**Per-table sweep function pattern** (lines 90–155, `backfillUsers`):
```go
func (r *backfillRunner) sweepUsers(ctx context.Context) error {
    rows, err := r.pool.Query(ctx, `SELECT id, COALESCE(access_token,''), COALESCE(refresh_token,'') FROM users ORDER BY id`)
    // ... iterate, call encryptIfNotCurrentKid, batch UPDATE, log progress ...
}
```

**Idempotency helper** (lines 227–241, `encryptIfPlaintext`) — Phase 14 replaces with kid-aware version:
```go
// Phase 14 variant:
func encryptIfNotCurrentKid(encryptor *encryption.MultiKeyEncryptor, stored string) (string, bool, error) {
    if stored == "" { return "", false, nil }
    decoded, err := base64.StdEncoding.DecodeString(stored)
    if err == nil && len(decoded) >= 29 && decoded[0] == encryptor.CurrentKid() {
        return stored, false, nil // already on latest kid
    }
    // Re-encrypt (decrypt first to verify integrity, then re-encrypt)
    plaintext, err := encryptor.DecryptString(stored)
    if err != nil { return "", false, fmt.Errorf("decrypt before re-encrypt: %w", err) }
    reencrypted, err := encryptor.EncryptString(plaintext)
    if err != nil { return "", false, err }
    return reencrypted, true, nil
}
```

**What changes:** Replace `crypto.StringCipher` → `*encryption.MultiKeyEncryptor`. Add tables: `viewer_sessions`, `overlay_tts_configs` (BYTEA), `kick_oauth_tokens`, `tiktok_oauth_tokens`. Add `--batch-size` + `--batch-delay-ms` flags. Add `SweeperMetrics` struct for structured log summary per table. Add per-batch `time.Sleep` for throttling.

---

### `services/auth-service/cmd/main.go` (MODIFY — jwt-issuer, encryption-callsite)

**Analog:** self (lines 102–142)

**Existing encryption key loading pattern** (lines 104, 130–142):
```go
tokenEncryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
if tokenEncryptionKey == "" {
    log.Fatal("TOKEN_ENCRYPTION_KEY must be set and must be 16, 24, or 32 bytes")
}
parsedKey, err := encryption.ParseKey(tokenEncryptionKey)
if err != nil { log.Fatal("failed to parse TOKEN_ENCRYPTION_KEY", zap.Error(err)) }
tokenCipher, err := encryption.NewAESEncryptor(parsedKey)
if err != nil { log.Fatal("failed to initialize token cipher", zap.Error(err)) }
```

**Existing JWT secret loading pattern** (lines 102–103, 127–128):
```go
jwtSecret := os.Getenv("JWT_SECRET")
if jwtSecret == "" { log.Fatal("JWT_SECRET must be set") }
```

**What changes:** Replace `encryption.NewAESEncryptor(parsedKey)` → `encryption.NewMultiKeyEncryptorFromEnv()`. Replace `jwtSecret string` loading → `auth.NewKeyChainFromEnv("JWT_SECRET")`. Pass `*KeyChain` into handlers instead of raw secret string. Add `TOKEN_ENCRYPTION_KEY_V1` (etc.) and `JWT_SECRET_V1` (etc.) to the startup validation log messages.

---

### `services/api-gateway/middleware/auth.go` (MODIFY — jwt-validator)

**Analog:** `services/api-gateway/middleware/auth.go` (lines 1–68, full file)

**Existing middleware function signature** (lines 28–68):
```go
func JWTAuth(jwtSecret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // ... Bearer token extraction ...
        claims, err := auth.ValidateJWT(token, jwtSecret)
        if err != nil {
            c.JSON(401, gin.H{"error": "invalid or expired token"})
            c.Abort()
            return
        }
        c.Set("user_id", claims.UserID)
        c.Set("username", claims.Username)
        c.Set("twitch_id", claims.TwitchID)
        c.Next()
    }
}
```

**What changes:** Change signature from `JWTAuth(jwtSecret string)` → `JWTAuth(kc *auth.KeyChain)`. Replace `auth.ValidateJWT(token, jwtSecret)` → `auth.ValidateJWTWithKeyChain(token, kc)` (new function on jwt.go that calls `jwt.ParseWithClaims` with `kc.KeyFunc`). The Bearer extraction and claims-to-context wiring stay identical.

---

### `shared/middleware/service_auth.go` (MODIFY — jwt-validator)

**Analog:** `shared/middleware/service_auth.go` (lines 1–80, full file)

**Existing validator function** (lines 30–80):
```go
func ServiceJWTAuth(secret string, allowedServices ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := auth.ValidateServiceJWT(tokenString, secret)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired service token"})
            c.Abort()
            return
        }
        // allowedServices check ...
        c.Set("service_name", claims.ServiceName)
        c.Next()
    }
}
```

**What changes:** Change `secret string` → `kc *auth.KeyChain`. Replace `auth.ValidateServiceJWT(tokenString, secret)` → `auth.ValidateServiceJWTWithKeyChain(tokenString, kc)`. All other logic unchanged.

---

### `services/youtube-listener/oauth/store.go` (MODIFY — encryption-callsite, CRUD)

**Analog:** `services/youtube-listener/oauth/store.go` (lines 38–80)

**Existing struct and constructor — field type changes** (lines 38–51):
```go
type PostgresTokenStore struct {
    db     *pgxpool.Pool
    enc    *encryption.AESEncryptor  // change → *encryption.MultiKeyEncryptor
    logger *zap.Logger
}

func NewPostgresTokenStore(db *pgxpool.Pool, enc *encryption.AESEncryptor, logger *zap.Logger) *PostgresTokenStore {
    // change enc param → *encryption.MultiKeyEncryptor
}
```

**Existing encrypt-on-write pattern** (lines 55–73):
```go
encAccess, err := s.enc.EncryptString(token.AccessToken)
// ... same for refresh token ...
```

**Existing decrypt-on-read with encryption_version gate** (lines 144–170):
```go
if encryptionVersion >= 1 {
    decryptedAccess, decryptErr := s.enc.DecryptString(storedAccess)
    // ...
}
```

**What changes:** Field type `*encryption.AESEncryptor` → `*encryption.MultiKeyEncryptor`. The `enc.EncryptString` / `enc.DecryptString` call sites are unchanged (same method names on `MultiKeyEncryptor`). The `encryption_version >= 1` gate becomes `>= 1` for legacy encrypted and all new versioned blobs; gate remains because `=0` still means plaintext during the transition.

---

### `services/kick-listener/channels/manager.go` (MODIFY — encryption-callsite, CRUD)

**Analog:** `services/youtube-listener/oauth/store.go` (lines 38–80)

**Existing plaintext read** (lines 966–978):
```go
var accessToken string
query := `SELECT access_token FROM kick_oauth_tokens WHERE channel_id = $1 AND expiry > NOW() ORDER BY expiry DESC LIMIT 1`
err := pool.QueryRow(m.ctx, query, channelSlug).Scan(&accessToken)
```

**What changes:** After adding `encryption_version` column (migration 050), this read must also select `encryption_version` and call `encryptor.DecryptString(accessToken)` when `encryptionVersion >= 1`. The manager struct needs an `encryptor *encryption.MultiKeyEncryptor` field injected from `cmd/main.go`.

---

### `services/overlay-manager/handlers/sources.go` (MODIFY — encryption-callsite, CRUD)

**Analog:** `services/youtube-listener/oauth/store.go` (lines 38–80)

**Existing plaintext Kick token read/write** (lines 214–249):
```go
// READ (plaintext today):
err := h.db.QueryRow(ctx, query, adminUserID).Scan(&existingToken.AccessToken, ...)

// WRITE (plaintext today):
_, err = h.db.Exec(ctx, insertQuery, adminUserID, channelID,
    existingToken.AccessToken, existingToken.RefreshToken, ...)
```

**What changes:** Wrap reads with `encryptor.DecryptString` when `encryption_version >= 1`. Wrap writes with `encryptor.EncryptString` and set `encryption_version = 1`. The handler struct needs an `encryptor *encryption.MultiKeyEncryptor` field.

---

### `migrations/050_kick_token_encryption.sql` and `051_tiktok_token_encryption.sql` (NEW — migration)

**Analog:** `migrations/006_youtube_token_encryption.sql` (lines 1–11, full file)

**Exact pattern to copy:**
```sql
-- 006_youtube_token_encryption.sql
BEGIN;
ALTER TABLE youtube_oauth_tokens
    ADD COLUMN IF NOT EXISTS encryption_version SMALLINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_youtube_oauth_tokens_encryption_version
    ON youtube_oauth_tokens(encryption_version);
COMMIT;
```

**Phase 14 migrations** (adapt table name and index name):
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

**What changes:** Table name only. `DEFAULT 0` semantics: existing rows (plaintext) get `encryption_version=0`; new writes after Phase 14 ships use `1`.

Also add `COMMENT ON COLUMN` matching the style of `migrations/049_overlay_tts_configs.sql` (lines 26–29) to document the encryption scheme.

---

### K8s deployment files (MODIFY — k8s-deployment, config)

**Analog:** `caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml` (lines 56–178)

**Existing `secretKeyRef` per env var pattern** (lines 169–178):
```yaml
- name: JWT_SECRET
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: jwt-secret
- name: TOKEN_ENCRYPTION_KEY
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: token-encryption-key
```

**Phase 14 additions — copy this pattern verbatim per new key** (no `envFrom`, each key is explicit):
```yaml
- name: TOKEN_ENCRYPTION_KEY_V1
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: token-encryption-key-v1
- name: JWT_SECRET_V1
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: jwt-secret-v1
- name: SERVICE_JWT_SECRET_V1
  valueFrom:
    secretKeyRef:
      name: allchat-secrets
      key: service-jwt-secret-v1
```

**Services that need these additions:**

| Deployment YAML | Env vars to add |
|---|---|
| `auth-service-deployment.yaml` | `TOKEN_ENCRYPTION_KEY_V1`, `JWT_SECRET_V1` |
| `api-gateway-deployment.yaml` | `JWT_SECRET_V1` |
| `overlay-manager-deployment.yaml` | `TOKEN_ENCRYPTION_KEY_V1`, `JWT_SECRET_V1` |
| `share-service-deployment.yaml` | `JWT_SECRET_V1`, `SERVICE_JWT_SECRET_V1` (fix existing gap) |
| `token-refresh-service-deployment.yaml` | `TOKEN_ENCRYPTION_KEY_V1` (currently mounted as `ENCRYPTION_KEY` → standardize) |
| `twitch-eventsub-listener-deployment.yaml` | `TOKEN_ENCRYPTION_KEY_V1` (currently as `ENCRYPTION_KEY`) |
| `source-manager-deployment.yaml` | `SERVICE_JWT_SECRET_V1` |
| All listener deployments | `SERVICE_JWT_SECRET_V1` |

**What changes:** N+1 `secretKeyRef` blocks per consumer. The old `TOKEN_ENCRYPTION_KEY` / `JWT_SECRET` / `SERVICE_JWT_SECRET` entries stay in place (legacy fallback) until the sweeper completes and the retire window expires.

---

### `docs/runbooks/secret-rotation.md` (NEW — runbook-doc)

**Analog:** `docs/migrations/2025-02-auth-token-encryption.md` (lines 1–46, full file)

**Existing runbook structure to copy:**
```markdown
# [Title]

## Prerequisites
[numbered list]

## Deployment Flow
[numbered steps]

## Tool Usage
[command examples with flags table]

## Rollback Plan
[numbered steps: detect → restore → redeploy → validate]
```

**What changes:** Phase 14 runbook covers three secret classes (TOKEN_ENCRYPTION_KEY, JWT_SECRET, SERVICE_JWT_SECRET, DB password) each as a named section. The `kubectl patch` commands from RESEARCH.md §7 are the concrete step bodies. Add a pre-flight checklist referencing the SOPS drift hazard (`project_secrets_drift.md` memory: never use `sops set` while drift exists; use `kubectl patch`).

---

## Shared Patterns

### Encryption Constructor Call — `NewMultiKeyEncryptorFromEnv()`
**Source:** `services/auth-service/cmd/main.go` lines 104–142 (current single-key pattern to replace)
**Apply to:** `services/auth-service/cmd/main.go`, `services/overlay-manager/cmd/main.go`, `services/token-refresh-service/cmd/main.go`, `services/twitch-eventsub-listener/cmd/main.go`, `services/kick-listener/cmd/main.go`, `services/auth-service/cmd/key-rotator/main.go`

Current pattern (lines 130–142):
```go
tokenEncryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")
if tokenEncryptionKey == "" {
    log.Fatal("TOKEN_ENCRYPTION_KEY must be set and must be 16, 24, or 32 bytes")
}
parsedKey, err := encryption.ParseKey(tokenEncryptionKey)
tokenCipher, err := encryption.NewAESEncryptor(parsedKey)
```

Phase 14 replacement (one line, validation inside the constructor):
```go
tokenCipher, err := encryption.NewMultiKeyEncryptorFromEnv()
if err != nil {
    log.Fatal("failed to initialize token cipher", zap.Error(err))
}
```

### JWT KeyChain Constructor — `auth.NewKeyChainFromEnv(prefix)`
**Source:** `shared/auth/jwt.go` (new function to add)
**Apply to:** `services/auth-service/cmd/main.go`, `services/api-gateway` (middleware wiring), `services/share-service/cmd/main.go`, `services/overlay-manager/cmd/main.go`, `services/source-manager/cmd/main.go`

Wiring pattern (replaces raw `os.Getenv("JWT_SECRET")`):
```go
userKeyChain, err := auth.NewKeyChainFromEnv("JWT_SECRET")
if err != nil { log.Fatal("JWT key chain init failed", zap.Error(err)) }

serviceKeyChain, err := auth.NewKeyChainFromEnv("SERVICE_JWT_SECRET")
if err != nil { log.Fatal("service JWT key chain init failed", zap.Error(err)) }
```

### JWT Middleware Signature Change
**Source:** `services/api-gateway/middleware/auth.go` line 28 (current), `shared/middleware/service_auth.go` line 30 (current)
**Apply to:** All middleware wiring in `cmd/main.go` for api-gateway, overlay-manager, share-service, source-manager

Current call sites (string-based):
```go
protected.Use(middleware.JWTAuth(jwtSecret))
router.Use(middleware.ServiceJWTAuth(serviceJWTSecret, "kick-listener"))
```

Phase 14 call sites (KeyChain-based):
```go
protected.Use(middleware.JWTAuth(userKeyChain))
router.Use(middleware.ServiceJWTAuth(serviceKeyChain, "kick-listener"))
```

### `secretKeyRef` per-env-var Pattern (no `envFrom`)
**Source:** `caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml` lines 169–178
**Apply to:** Every deployment YAML that gains `TOKEN_ENCRYPTION_KEY_V1`, `JWT_SECRET_V1`, `SERVICE_JWT_SECRET_V1`

Each new secret key = one new YAML block. Never use `envFrom` for `allchat-secrets` (existing pattern is per-var, planner must not introduce `envFrom`).

### `kubectl patch` for Live Secret Edits
**Source:** RESEARCH.md §2 fallback runbook + RESEARCH.md §7 rotation runbook
**Apply to:** All runbook steps in `docs/runbooks/secret-rotation.md`

```bash
ENCODED=$(echo -n "$NEW_VALUE" | base64)
kubectl patch secret allchat-secrets -n allchat \
  --type='json' \
  -p="[{\"op\": \"add\", \"path\": \"/data/new-key-name\", \"value\": \"$ENCODED\"}]"
```

Never `sops set` while `allchat-secrets` is OutOfSync (31 live vs 11 SOPS keys). Always verify current live keys first:
```bash
kubectl get secret allchat-secrets -n allchat \
  -o jsonpath='{.data}' | python3 -c "import sys,json; [print(k) for k in sorted(json.load(sys.stdin).keys())]"
```

---

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `services/auth-service/cmd/key-rotator/key-rotator-job.yaml` | k8s-deployment | batch (Job/CronJob) | No K8s Job YAML for any existing cmd tool in the repo; planner must derive from RESEARCH.md §6 YAML skeleton |

---

## Critical Implementation Notes for Planner

1. **Disambiguation in `MultiKeyEncryptor.DecryptString`:** Try versioned path first; if AEAD authentication fails, fall back to legacy key. This eliminates false-positives where a legacy blob's first byte coincidentally equals a registered kid byte (probability 1/256 per kid).

2. **TikTok gap-fill is Go-only:** The `tiktok_oauth_tokens` table has no Go read paths found in the codebase. If the Node.js tiktok-listener reads these tokens, Phase 14 migration adds the column but the encryption call-site must be in the Node.js service. Planner must flag this for a separate sub-plan.

3. **share-service Service JWT inconsistency (RESEARCH.md §4):** `share-service` currently generates Service JWTs using `JWT_SECRET` (not `SERVICE_JWT_SECRET`). Phase 14 must fix this: `share-service` should use `SERVICE_JWT_SECRET_V<n>` for its Service JWT issuance. This is a bug fix bundled into Phase 14.

4. **Env var name standardization:** `token-refresh-service` and `twitch-eventsub-listener` mount `token-encryption-key` as `ENCRYPTION_KEY` (not `TOKEN_ENCRYPTION_KEY`). `NewMultiKeyEncryptorFromEnv()` should accept both names, OR deployment manifests are updated to use `TOKEN_ENCRYPTION_KEY`. Planner must decide; the cleanest path is updating the deployment manifests and having `NewMultiKeyEncryptorFromEnv` read `TOKEN_ENCRYPTION_KEY` only.

5. **`overlay_tts_configs.encrypted_api_key` is `BYTEA`, not `TEXT`:** The sweeper's sweep for this table needs a different read/write path than the TEXT columns — decode the BYTEA blob to string, decrypt, re-encrypt, write back as BYTEA.

---

## Metadata

**Analog search scope:** `shared/`, `services/auth-service/`, `services/api-gateway/`, `services/overlay-manager/`, `services/youtube-listener/`, `services/kick-listener/`, `services/token-refresh-service/`, `services/twitch-eventsub-listener/`, `shared/middleware/`, `migrations/`, `caesar-deployment/apps/workloads/all-chat/`, `docs/migrations/`
**Files read:** 16
**Pattern extraction date:** 2026-04-27
