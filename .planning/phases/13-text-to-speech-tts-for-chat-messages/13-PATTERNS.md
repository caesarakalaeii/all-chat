# Phase 13: Text-to-Speech (TTS) for chat messages - Pattern Map

**Mapped:** 2026-04-23
**Files analyzed:** 21 (13 create + 8 modify)
**Analogs found:** 21 / 21

All net-new files have at least one strong analog already in tree. This is a high-reuse phase; the planner should treat every "copy the pattern from X" instruction literally. No net-new net-engineering patterns are required.

---

## File Classification

### Files to Create

| New File | Role | Data Flow | Closest Analog | Match Quality |
|----------|------|-----------|----------------|---------------|
| `migrations/049_overlay_tts_configs.sql` | migration (DDL + idempotent INSERT) | batch | `migrations/048_impersonation_audit_log.sql` + `migrations/044_feature_gates.sql` | exact (two analogs cover both halves) |
| `services/overlay-manager/handlers/tts.go` | controller (Gin handlers — 6 new endpoints + JWT-middleware-backed proxy) | request-response + streaming proxy | `services/overlay-manager/handlers/config.go` + `services/api-gateway/handlers/proxy.go` + `services/share-service/handlers/admin_featuregates.go` | exact (ownership check from config.go, streaming proxy from proxy.go, narrow-DB-interface idiom from admin_featuregates.go) |
| `services/overlay-manager/repository/tts_config_repo.go` | repository (CRUD + RotateSecret) | CRUD | `services/overlay-manager/repository/config_repo.go` | exact (same pgxpool + pgx.Row scanner idiom) |
| `services/overlay-manager/tts/jwt.go` | utility (JWT sign + verify) | request-response | `shared/auth/jwt.go` | role-match (mirror `GenerateServiceJWT` / `ValidateServiceJWT` shape using a narrower claims struct) |
| `services/overlay-manager/models/tts_config.go` | model (struct + `EnsureDefaults`) | N/A | `services/overlay-manager/models/config.go` | exact |
| `services/overlay-manager/handlers/tts_test.go` | test (Gin + httptest + mocked ElevenLabs upstream) | test | `services/share-service/middleware/premium_test.go` | role-match (mocked DI + httptest pattern); no existing overlay-manager-handler test file has the exact streaming-mock shape |
| `services/overlay-manager/repository/tts_config_repo_test.go` | test (pgxpool roundtrip) | test | `services/share-service/featuregates/cache_test.go` (if present) OR `services/overlay-manager/repository/config_repo.go` roundtrip patterns | role-match (simple DB-backed repo test; encryption roundtrip stays inside `shared/encryption`) |
| `services/overlay-manager/tts/jwt_test.go` | test (sign/verify roundtrip + rotation) | test | `services/share-service/middleware/premium_test.go` | role-match (table-driven sign/verify pattern is well-established in the project) |
| `shared/featuregates/cache.go` | utility (in-memory cache + Pub/Sub + ticker) — MOVED from `services/share-service/featuregates/cache.go` | pub-sub + batch | source is the same file being moved | identity (byte-for-byte move; per ADR-0008 verified no `promauto` imports) |
| `shared/middleware/premium.go` | middleware (Gin) — MOVED from `services/share-service/middleware/premium.go` | request-response | source is the same file being moved | identity (byte-for-byte move; import-path updates are the only change) |
| `frontend/src/lib/utils/ttsPlayer.ts` | utility (client-side queue/sampling/cooldown/rate-limiter) | event-driven | `frontend/src/lib/utils/soundPlayer.ts` | role-match (shape mirror; TTS adds queue + priority logic) |
| `frontend/src/lib/utils/__tests__/ttsPlayer.test.ts` | test (vitest + fake timers + stubGlobal) | test | `frontend/src/lib/utils/__tests__/soundPlayer.test.ts` | exact (identical test scaffold — `vi.stubGlobal`, `vi.useFakeTimers`) |
| `frontend/src/components/appearance/TTSGroup.tsx` | component (settings group) | request-response (form) | `frontend/src/components/appearance/SoundGroup.tsx` + `frontend/src/components/appearance/FilterGroup.tsx` | exact (SoundGroup shape + FilterGroup chip-list + PremiumBadge decoration) |
| `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx` | test (RTL) | test | `frontend/src/components/appearance/__tests__/SoundGroup.test.tsx` | exact (same `renderX({overrides})` idiom) |
| `frontend/src/lib/api/tts.ts` OR inline into `overlays.ts` | utility (fetch wrappers for 7 endpoints) | request-response | `frontend/src/lib/api/overlays.ts` | exact (apiClient.get/post/put/delete wrapper functions) |
| `frontend/src/lib/hooks/useBrowserVoices.ts` | hook (subscribe to `speechSynthesis.onvoiceschanged`) | event-driven | no direct analog — general React hook pattern | role-match (pure React effect; use project hook conventions) |
| `docs/adr/0012-*.md` (OPTIONAL) | documentation | N/A | `docs/adr/0008-feature-gate-infrastructure.md` | role-match (ADR template) |

### Files to Modify

| Modified File | Role | Change Summary | Reference Existing Block |
|---------------|------|----------------|--------------------------|
| `services/overlay-manager/cmd/main.go` | service entry point | add featuregates.Cache init + RequirePremium middleware + AES encryptor + TTS handlers wiring | `services/share-service/cmd/main.go:102-107` (cache init) + `services/share-service/cmd/main.go:186-190` (premium routes) + `services/auth-service/cmd/main.go:134-139` (encryptor init) |
| `services/share-service/cmd/main.go` | service entry point | rewrite imports: `services/share-service/featuregates` → `shared/featuregates`; `services/share-service/middleware` → `shared/middleware` | `services/share-service/cmd/main.go:29, 32` (import block) |
| `services/share-service/handlers/admin_featuregates.go` | controller | rewrite import of `featuregates.PubSubChannel` → `shared/featuregates.PubSubChannel` (one line) | `services/share-service/handlers/admin_featuregates.go:23` |
| `frontend/src/lib/types/overlay.ts` | type definitions | extend `DisplaySettings` with 20 `tts_*` fields (D-24 verbatim) | `frontend/src/lib/types/overlay.ts:54-75` (existing sound + phase-9 fields follow the same shape) |
| `frontend/src/components/appearance/AppearancePanel.tsx` | mount point | add `CollapsibleSection id="tts"` after the `id="sounds"` block; extend `AppearancePanelProps` with TTS callbacks | `frontend/src/components/appearance/AppearancePanel.tsx:89-97` (the existing `sounds` block is the 1:1 template) |
| `frontend/src/app/overlay/[id]/page.tsx` | integration | add `ttsPlayerRef`/`ttsSettingsRef` + destroy effect (near line 139) + config-load branch (near line 232) + `ttsPlayerRef.current?.speak(message)` adjacent to line 414 | `frontend/src/app/overlay/[id]/page.tsx:117-123, 138-144, 232-256, 410-414` |
| `frontend/src/app/overlays/[id]/preview/embed/page.tsx` | integration (editor embed iframe) | add `TTS_SETTINGS_UPDATE` listener (near line 272) + config-load branch (near line 347) + `ttsPlayerRef.current?.speak(message)` adjacent to line 400 | `frontend/src/app/overlays/[id]/preview/embed/page.tsx:48-51, 272-288, 347-370, 397-400` |
| `frontend/src/app/overlays/[id]/page.tsx` | integration (editor) | add TTS config load (near line 1393) + Copy-OBS-URL button + Regenerate-URL button + save-key/test-key/remove-key flow state | `frontend/src/app/overlays/[id]/page.tsx:1388-1403` (sound-settings load pattern) |

---

## Pattern Assignments

### Backend

#### `migrations/049_overlay_tts_configs.sql` (migration, batch)

**Analogs:**
1. `migrations/048_impersonation_audit_log.sql` — current-era migration template (DDL + index + comment)
2. `migrations/044_feature_gates.sql` — idempotent feature_gates INSERT with `ON CONFLICT DO NOTHING`

**Template pattern from `migrations/048_impersonation_audit_log.sql:1-22`:**
```sql
-- All-Chat Impersonation Audit Log
-- Migration: 048
-- Description: ...

CREATE TABLE IF NOT EXISTS impersonation_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ...
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_impersonation_audit_admin ON impersonation_audit_log(admin_user_id);

COMMENT ON TABLE impersonation_audit_log IS
    'DSGVO Art.5(2) accountability: ...';
```

**Idempotent-insert pattern from `migrations/044_feature_gates.sql:20-22`:**
```sql
INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('sharing', TRUE, 'Overlay share requests — ...')
ON CONFLICT (feature_key) DO NOTHING;
```

**Preserve:**
- Commented migration header (`-- All-Chat ...`, `-- Migration: 049`, `-- Description: ...`)
- `CREATE TABLE IF NOT EXISTS`
- `UUID PRIMARY KEY DEFAULT gen_random_uuid()`
- `ON DELETE CASCADE` on the `overlay_id` FK (matches cascade semantics of `overlay_configs`)
- `TIMESTAMP NOT NULL DEFAULT NOW()` for created_at / updated_at
- Index on the FK column (`CREATE INDEX idx_overlay_tts_configs_overlay ON overlay_tts_configs(overlay_id);`)
- `COMMENT ON TABLE` and `COMMENT ON COLUMN` for the encrypted fields (security note: "AES-GCM ciphertext, 12-byte nonce prefix, see shared/encryption")
- `ON CONFLICT (feature_key) DO NOTHING` on the `feature_gates` INSERT (Pitfall 5)
- SQL files do NOT need the AGPL C-style comment header; `--` SQL comments are fine (observe 048/044 style — neither carries an AGPL preamble)

#### `services/overlay-manager/handlers/tts.go` (controller, request-response + streaming)

**Analogs:**
1. `services/overlay-manager/handlers/config.go` — owner-check + `GetByIDAndUserID` pattern
2. `services/api-gateway/handlers/proxy.go` — `io.Copy(c.Writer, body)` streaming with `http.NewRequestWithContext`
3. `services/share-service/handlers/admin_featuregates.go` — narrow-interface DB injection (testable without pgxmock)

**Owner-check + unauthorized-handling pattern from `services/overlay-manager/handlers/config.go:47-64`:**
```go
func (h *ConfigHandler) HandleGetConfig(c *gin.Context) {
    userID, exists := c.Get("user_id")
    if !exists {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }

    overlayID := c.Param("id")
    if overlayID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
        return
    }

    if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
        return
    }

    config, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
    ...
}
```

**Streaming-proxy pattern from `services/api-gateway/handlers/proxy.go:97-134`:**
```go
backendReq, err := http.NewRequestWithContext(
    c.Request.Context(),           // propagate client disconnect (Pitfall 7)
    c.Request.Method,
    backendURL,
    c.Request.Body,
)
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{
        "error": "failed to create backend request",
    })
    return
}

copyHeaders(backendReq.Header, c.Request.Header)

backendResp, err := p.client.Do(backendReq)
if err != nil {
    c.JSON(http.StatusBadGateway, gin.H{
        "error": "backend service unavailable: " + err.Error(),
    })
    return
}
defer backendResp.Body.Close()

copyHeaders(c.Writer.Header(), backendResp.Header)
c.Status(backendResp.StatusCode)

_, err = io.Copy(c.Writer, backendResp.Body)
if err != nil {
    c.Error(err)   // logged, not written to response (headers already sent)
}
```

**Narrow-DB-interface + mock-friendly constructor from `services/share-service/handlers/admin_featuregates.go:37-106`:**
```go
// featureGateDB is a narrow interface over pgxpool.Pool for the feature gate handler.
// This allows mock injection in tests without pgxmock.
type featureGateDB interface {
    QueryFeatureGates(ctx context.Context) ([]FeatureGateResponse, error)
    UpdateFeatureGate(ctx context.Context, key string, isPremium bool) (int64, error)
}

type pgxFeatureGateDB struct {
    pool *pgxpool.Pool
}
// ... Query/Update impl ...

type AdminFeatureGatesHandler struct {
    db     featureGateDB
    redis  featureGateRedis
    logger *zap.Logger
}

func NewAdminFeatureGatesHandler(db *pgxpool.Pool, rc *redis.Client, logger *zap.Logger) *AdminFeatureGatesHandler {
    return &AdminFeatureGatesHandler{
        db:     &pgxFeatureGateDB{pool: db},
        redis:  &redisFeatureGateClient{client: rc},
        logger: logger,
    }
}
```

**Preserve:**
- AGPL-3.0 license header (every new `.go` file — verified on all existing files via grep)
- `c.Request.Context()` passed to the upstream ElevenLabs HTTP request (Pitfall 7)
- `defer resp.Body.Close()` after a non-error upstream response
- `c.Error(err)` (NOT `c.JSON(500, ...)`) after headers are already written during streaming
- No CORS headers in the handler (Pitfall 8 — api-gateway handles CORS)
- No logging of the ElevenLabs API key or `xi-api-key` header value (V8 Data Protection threat model item)
- `owner-scope` check on all 5 user-auth endpoints via `h.overlays.GetByIDAndUserID(...)`
- The 6th endpoint (`POST /tts`) does NOT use owner-check — it uses JWT-secret verification against the per-overlay `tts_signing_secret` loaded from the `overlay_tts_configs` row

#### `services/overlay-manager/repository/tts_config_repo.go` (repository, CRUD)

**Analog:** `services/overlay-manager/repository/config_repo.go`

**Complete CRUD idiom from `services/overlay-manager/repository/config_repo.go:36-115`:**
```go
type OverlayConfigRepository struct {
    pool *pgxpool.Pool
}

func NewOverlayConfigRepository(connString string) (*OverlayConfigRepository, error) {
    pool, err := pgxpool.New(context.Background(), connString)
    if err != nil {
        return nil, fmt.Errorf("failed to create connection pool: %w", err)
    }

    if err := pool.Ping(context.Background()); err != nil {
        pool.Close()
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    return &OverlayConfigRepository{pool: pool}, nil
}

func (r *OverlayConfigRepository) GetByOverlayID(ctx context.Context, overlayID string) (*models.OverlayConfig, error) {
    query := `
        SELECT id, overlay_id, display_settings, ...
        FROM overlay_configs
        WHERE overlay_id = $1
    `
    row := r.pool.QueryRow(ctx, query, overlayID)
    return scanOverlayConfig(row)
}
```

**Row-scanner helper pattern (`services/overlay-manager/repository/config_repo.go:117-153`):**
```go
func scanOverlayConfig(row pgx.Row) (*models.OverlayConfig, error) {
    config := &models.OverlayConfig{}
    var displaySettingsJSON, filterSettingsJSON, visualSettingsJSON []byte

    err := row.Scan(&config.ID, &config.OverlayID, &displaySettingsJSON, ...)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, fmt.Errorf("overlay config not found")
        }
        return nil, fmt.Errorf("failed to scan overlay config: %w", err)
    }
    ...
}
```

**Preserve:**
- AGPL-3.0 header
- Repository takes `connString` (not pool) in the constructor; creates pool internally via `pgxpool.New`
- `Ping` on construction; `pool.Close()` on ping failure
- `pgx.ErrNoRows` mapped to a domain-meaningful error (`"tts config not found"`)
- `[]byte` scan targets for BYTEA columns (NOT `string`) — important for `encrypted_api_key` and `tts_signing_secret`
- Parameterized queries only (`$1`, `$2`, ...); never string-concat user input

**Extension needed (not in analog):** add a `RotateSigningSecret(ctx, overlayID) ([]byte, error)` method that generates a new 32-byte random secret and writes it atomically. This is plain SQL `UPDATE overlay_tts_configs SET tts_signing_secret = $1, updated_at = NOW() WHERE overlay_id = $2 RETURNING tts_signing_secret`.

#### `services/overlay-manager/tts/jwt.go` (utility, request-response)

**Analog:** `shared/auth/jwt.go` — existing `ServiceClaims` + `GenerateServiceJWT` + `ValidateServiceJWT` shape.

**Verify idiom from `shared/auth/jwt.go:155-176`:**
```go
func ValidateJWT(tokenString, secret string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method (algorithm-confusion mitigation)
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(secret), nil
    })

    if err != nil {
        if errors.Is(err, jwt.ErrTokenExpired) {
            return nil, ErrExpiredToken
        }
        return nil, ErrInvalidToken
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, ErrInvalidToken
}
```

**Sign idiom from `shared/auth/jwt.go:202-216`:**
```go
claims := ServiceClaims{
    ServiceName: serviceName,
    RegisteredClaims: jwt.RegisteredClaims{
        Subject:   serviceName,
        Issuer:    "all-chat-services",
        Audience:  []string{"internal"},
        IssuedAt:  jwt.NewNumericDate(time.Now()),
        ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
    },
}
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
return token.SignedString([]byte(secret))
```

**Preserve:**
- AGPL-3.0 header
- `*jwt.SigningMethodHMAC` type assertion in the parse callback (Pitfall on algorithm-confusion; threat model V2 Authentication item)
- `jwt.NewNumericDate(time.Now())` for `IssuedAt`
- Subject/Scope/Issuer claims verified after `parsed.Valid` check (not before — empty claims can still parse)
- HS256 signing method (matches all-chat convention; verified at `shared/auth/jwt.go:103, 127, 150, 214`)
- Signing-secret parameter typed as `[]byte` (not `string`) — the per-overlay secret is 32 random bytes, base64 is only for transit/storage

**Deviation from analog:** `TTSClaims` has NO `ExpiresAt` field (D-08 explicit — rotation-based revocation via `tts_signing_secret` change). Document this clearly in the code comment.

#### `services/overlay-manager/models/tts_config.go` (model)

**Analog:** `services/overlay-manager/models/config.go`

**Complete struct pattern from `services/overlay-manager/models/config.go:22-47`:**
```go
package models

import "time"

type OverlayConfig struct {
    ID              string         `json:"id"`
    OverlayID       string         `json:"overlay_id"`
    DisplaySettings map[string]any `json:"display_settings"`
    ...
    CreatedAt       time.Time      `json:"created_at"`
    UpdatedAt       time.Time      `json:"updated_at"`
}

func (c *OverlayConfig) EnsureMaps() {
    if c.DisplaySettings == nil {
        c.DisplaySettings = map[string]any{}
    }
    ...
}
```

**Preserve:**
- AGPL-3.0 header
- `time.Time` for timestamps
- `json:"..."` tags for all exported fields
- `EncryptedAPIKey` typed as `[]byte` (BYTEA column — do NOT use `string`)
- `SigningSecret` typed as `[]byte`
- `EnsureDefaults`-style nil-safety if any sub-maps are present (likely not needed — the table is flat)

#### `services/overlay-manager/handlers/tts_test.go` (test)

**Analog:** `services/share-service/middleware/premium_test.go` (DI + httptest pattern)

**Mock + httptest idiom from `services/share-service/middleware/premium_test.go:29-89`:**
```go
type mockGateChecker struct {
    isPremiumResult bool
}
func (m *mockGateChecker) IsPremium(_ string) bool {
    return m.isPremiumResult
}

func newTestRouter(userID string, handler gin.HandlerFunc) *gin.Engine {
    gin.SetMode(gin.TestMode)
    router := gin.New()
    router.GET("/test", func(c *gin.Context) {
        if userID != "" {
            c.Set("user_id", userID)
        }
        c.Next()
    }, handler, func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    })
    return router
}

// ...
w := httptest.NewRecorder()
req := httptest.NewRequest(http.MethodGet, "/test", nil)
router.ServeHTTP(w, req)
assert.Equal(t, http.StatusOK, w.Code)
```

**For upstream ElevenLabs mocking:** spin up an `httptest.NewServer(...)` with handlers returning `200 + audio/mpeg` or `401 / 429 / 500`; configure the handler under test to point at `server.URL`. Pattern not currently in codebase but stdlib-standard.

**Preserve:**
- AGPL-3.0 header
- `gin.SetMode(gin.TestMode)` at the top of each test or in TestMain
- `github.com/stretchr/testify/assert` for assertions (project standard — `services/overlay-manager/go.mod:12`)
- Narrow-interface mocks for the repository and the feature-gate cache (use `featuregates.NewFeatureGateCacheWithGates(map[string]bool{"tts": true})` for premium tests)
- For the streaming `POST /tts` tests: create an `httptest.NewServer` that writes `audio/mpeg` bytes; verify the test client receives them and Content-Type is `audio/mpeg`
- For client-disconnect propagation test (Pitfall 7): cancel `c.Request.Context()` mid-stream and assert upstream server saw connection close (can be observed via a signal channel in the mock server)

#### `services/overlay-manager/repository/tts_config_repo_test.go` (test)

**Analog:** None perfect — use `services/overlay-manager/repository/config_repo.go` as the structural reference plus a small `testcontainers-go` or dockerized PostgreSQL spin-up. Or, if the repo tests already use a shared fixture, mirror it. Research flagged this test file as "ERROR Wave 0" — the simplest viable path is a tabletop test that uses `pgxmock` to stub the pool.

**Preserve:**
- AGPL-3.0 header
- Roundtrip: Create → Get → assert ciphertext is opaque bytes (NOT the plaintext API key) → Decrypt via `shared/encryption` → assert matches input
- `RotateSigningSecret` test: pre-seed a row, call rotate, assert new secret differs from old and is exactly 32 bytes
- Do NOT duplicate AES-GCM correctness tests — those live in `shared/encryption/encryption_test.go` (verified: already passing with a 32-byte base64 key)

#### `services/overlay-manager/tts/jwt_test.go` (test)

**Analog:** The `ValidateJWT` / `GenerateJWT` roundtrip pattern in `shared/auth/jwt.go` (tests elsewhere in that package — verify the suite, follow the same shape).

**Preserve:**
- AGPL-3.0 header
- Table-driven test covering: (a) valid token → pass, (b) tampered signature → fail, (c) wrong overlay_id in sub → fail, (d) wrong scope → fail, (e) different signing-secret → fail
- Rotation test: sign with secret A → verify succeeds; rotate to secret B → verify against secret A fails, verify against secret B succeeds
- "no exp" assertion: sign with a fixed IAT → call Validate 100 years later → still succeeds (D-08 no-expiry by design)

### Shared Module Moves

#### `shared/featuregates/cache.go` (MOVED from `services/share-service/featuregates/cache.go`)

**Source file to move:** `services/share-service/featuregates/cache.go` (298 lines, verified)

**Move procedure:**
1. Create directory `shared/featuregates/`
2. Copy the file byte-for-byte
3. Update the package docstring (line 17-22) if it references the share-service path
4. In `services/share-service/cmd/main.go:29`, change `"github.com/caesar/all-chat/services/share-service/featuregates"` → `"github.com/caesar/all-chat/shared/featuregates"`
5. In `services/share-service/handlers/admin_featuregates.go:23`, same import change
6. In `services/share-service/cmd/main.go:186-190`, `featuregates.GateSharing` now comes from the shared package (unchanged call site)
7. In `services/overlay-manager/cmd/main.go` (NEW), add `"github.com/caesar/all-chat/shared/featuregates"` to the import block
8. Move `cache_test.go` alongside (keep the tests in the same directory)
9. Delete `services/share-service/featuregates/` entirely

**Preserve (critical from Pitfall 6):**
- `grep -n promauto services/share-service/featuregates/cache.go` returns zero (researcher verified). Do NOT add any Prometheus metrics during the move.
- The package-level constants `PubSubChannel`, `GateSharing`, `GateStreamSelection` — these are imported elsewhere; renaming or removing will break callers.
- `NewFeatureGateCacheWithGates` (test helper) — used by middleware tests
- `NewFeatureGateCacheForTest` / `NewFeatureGateCacheForTestWithInterval` — test-only constructors
- The `// ADR-0008: Feature Gate Infrastructure` comment at line 21

#### `shared/middleware/premium.go` (MOVED from `services/share-service/middleware/premium.go`)

**Source file to move:** `services/share-service/middleware/premium.go` (125 lines, verified)

**Move procedure:**
1. Copy the file to `shared/middleware/premium.go`
2. Update the package docstring and the `package middleware` declaration (unchanged — directory already uses `package middleware`; check for name conflicts with existing `shared/middleware/auth.go`, `cors.go` — all are `package middleware`; verified clean)
3. In `services/share-service/cmd/main.go:32`, change `localMiddleware "github.com/caesar/all-chat/services/share-service/middleware"` → `"github.com/caesar/all-chat/shared/middleware"` (and drop the alias if no longer necessary) OR keep the alias if `shared/middleware` is also imported (it already is, at line 36 — so alias stays as `localMiddleware` or rename pattern is chosen)
4. Move `premium_test.go` alongside
5. Delete `services/share-service/middleware/` (after verifying no other files there — only `premium.go` + `premium_test.go`, verified)

**Preserve (critical):**
- AGPL-3.0 header
- `GateChecker` interface (used by `featuregates.FeatureGateCache.IsPremium` — cross-module contract)
- `RequirePremiumWithQuerier` (test-only) signature
- The three-step flow: authN check → gate check → premium check
- The 401/403 JSON shape: `{"error": "authentication required"}` / `{"error": "Premium feature required", "message": "...", "upgrade_url": "/upgrade"}`
- Zap logger parameter as `*zap.Logger` (matches all other middleware in `shared/middleware/`)

### Frontend

#### `frontend/src/lib/utils/ttsPlayer.ts` (utility, event-driven)

**Analog:** `frontend/src/lib/utils/soundPlayer.ts`

**Complete shape from `frontend/src/lib/utils/soundPlayer.ts:35-122`:**
```typescript
export interface SoundSettings {
  enabled: boolean
  preset: string
  volume: number
  cooldownMs: number
  customUrl?: string
}

export interface SoundPlayer {
  play(): void
  updateSettings(settings: SoundSettings): void
  destroy(): void
}

export function createSoundPlayer(initialSettings: SoundSettings): SoundPlayer {
  let settings: SoundSettings = { ...initialSettings }

  // Pre-create pool of HTMLAudioElement instances
  const pool: HTMLAudioElement[] = Array.from({ length: POOL_SIZE }, () => {
    const el = new Audio()
    el.volume = settings.volume
    return el
  })

  function play(): void {
    if (!settings.enabled) return
    const now = Date.now()
    if (now - lastPlayedAt < settings.cooldownMs) return
    ...
  }

  function updateSettings(newSettings: SoundSettings): void {
    settings = { ...newSettings }
    pool.forEach(el => { el.volume = settings.volume })
  }

  function destroy(): void {
    pool.forEach(el => { el.pause(); el.src = '' })
  }

  return { play, updateSettings, destroy }
}
```

**Preserve:**
- AGPL-3.0 JSDoc header
- `export interface XxxSettings { ... }`, `export interface XxxPlayer { ... }`, `export function createXxxPlayer(initialSettings): XxxPlayer { ... }` exported-surface shape
- Closure-over-state (no class) — `let settings: XxxSettings = { ...initialSettings }`
- `updateSettings` takes a full settings object (not a patch)
- `destroy()` clears all timers/refs
- No `any` types — use proper `TTSSettings`, `ChatMessage`, `EventType`, `SpeechSynthesisUtterance`, `Blob`, `HTMLAudioElement`

**Extensions beyond analog (from RESEARCH Example 4):**
- Add `speak(message: ChatMessage): void` in lieu of `play()` (takes the message to extract user/event/platform/text)
- Add internal `queue: ChatMessage[]`, `cooldowns: Map<string, number>`, `bucketTokens: number`, `bucketLastRefill: number`, `sessionFallback: boolean`, `speaking: boolean`
- Add `speakBrowser(text)` + `speakElevenLabs(text)` async dispatchers
- Add optional `onFallback?: () => void` constructor callback (D-38 — the parent fires the toast)
- Use the `PRIORITY_EVENTS` Set from RESEARCH Example 4 line 771-775 verbatim (includes 11 event types)

#### `frontend/src/lib/utils/__tests__/ttsPlayer.test.ts` (test)

**Analog:** `frontend/src/lib/utils/__tests__/soundPlayer.test.ts`

**Vi-stubGlobal + fakeTimers scaffold from `frontend/src/lib/utils/__tests__/soundPlayer.test.ts:22-62`:**
```typescript
const mockPlay = vi.fn().mockResolvedValue(undefined)
const mockAudioInstances: Array<{ src: string; volume: number; ... }> = []

class MockAudio {
  src = ''
  volume = 1
  play = mockPlay
  onerror: (() => void) | null = null

  constructor() {
    mockAudioInstances.push(this as unknown as (typeof mockAudioInstances)[0])
  }
}

vi.stubGlobal('Audio', MockAudio)

describe('createSoundPlayer', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockPlay.mockClear()
    mockAudioInstances.length = 0
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('...', () => { /* ... */ })
})
```

**Preserve:**
- AGPL-3.0 JSDoc header
- `vi.stubGlobal('speechSynthesis', mockSynth)` for the Web Speech branch (class-based mock that records calls to `speak` / `cancel` / `getVoices`)
- `vi.stubGlobal('fetch', mockFetch)` for the ElevenLabs branch (returns `{ok: true, blob: () => new Blob(['...'])}` by default)
- `vi.stubGlobal('Audio', MockAudio)` for the ElevenLabs playback path (same pattern as soundPlayer)
- `vi.useFakeTimers()` in beforeEach, `vi.useRealTimers()` in afterEach
- Tests organized by decision: describe `D-33 queue overflow`, describe `D-35 cooldown`, describe `D-36 token bucket`, describe `D-37 staleness`, describe `D-38 session fallback`, describe `formatContent D-25..D-30`

**Coverage targets (from RESEARCH validation architecture):** D-03, D-05-only-applicable-client-side, D-19, D-25, D-26, D-29, D-30, D-31, D-32, D-33, D-35, D-36, D-37, D-38, D-42. See RESEARCH "Phase Requirements → Test Map" for exact `-t` names.

#### `frontend/src/components/appearance/TTSGroup.tsx` (component, form)

**Primary analog:** `frontend/src/components/appearance/SoundGroup.tsx`
**Secondary analog (chip list):** `frontend/src/components/appearance/FilterGroup.tsx`

**Complete group shape from `frontend/src/components/appearance/SoundGroup.tsx:31-131`:**
```tsx
'use client'

// AGPL header ...

import React from 'react'
import { ToggleSwitch } from './ToggleSwitch'
import { SliderControl } from './SliderControl'
import { PremiumBadge } from '@/components/PremiumBadge'
import type { DisplaySettings } from '@/lib/types/overlay'

export interface SoundGroupProps {
  displaySettings: Partial<DisplaySettings>
  onChange: (patch: Partial<DisplaySettings>) => void
  isPremium: boolean
  onPreview?: () => void
}

export function SoundGroup({ displaySettings, onChange, isPremium, onPreview }: SoundGroupProps): React.ReactElement {
  const enabled = displaySettings.notification_sound_enabled ?? false
  ...

  return (
    <div className="space-y-4">
      <ToggleSwitch
        label="Enable notification sounds"
        checked={enabled}
        onChange={(checked) => onChange({ notification_sound_enabled: checked })}
      />

      {enabled && (
        <>
          {/* ... form controls ... */}

          <div>
            <div className="mb-1 flex items-center gap-2">
              {!isPremium && <PremiumBadge />}
              <p className="text-sm text-text-sub">
                Custom sound URL
                {!isPremium && (
                  <span className="ml-1 text-xs text-text-dim">
                    — Upload your own notification sound (Premium)
                  </span>
                )}
              </p>
            </div>
            <input
              type="url"
              ...
              disabled={!isPremium}
              className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-sm text-text placeholder:text-text-dim disabled:cursor-not-allowed disabled:opacity-50"
            />
          </div>
        </>
      )}
    </div>
  )
}
```

**Chip-list pattern from `frontend/src/components/appearance/FilterGroup.tsx:38-78` — adapt for `tts_enabled_platforms`:**
```tsx
function TagInput({ tags, onAdd, onRemove, placeholder }) {
  return (
    <div className="flex flex-wrap gap-1 rounded-lg border border-border bg-surface p-2">
      {tags.map(tag => (
        <span key={tag} className="flex items-center gap-1 rounded bg-surface-alt px-2 py-0.5 text-xs text-text">
          {tag}
          <button type="button" onClick={() => onRemove(tag)} aria-label={`Remove ${tag}`}>
            <X className="h-3 w-3" />
          </button>
        </span>
      ))}
    </div>
  )
}
```

**Preserve (SoundGroup):**
- `'use client'` directive at top
- AGPL-3.0 JSDoc header
- `export interface XxxGroupProps { displaySettings, onChange, isPremium, onPreview? }`
- `<div className="space-y-4">` root
- `ToggleSwitch` as the master enable control
- `{enabled && (<>...</>)}` conditional rendering (collapse when disabled)
- `SliderControl` for numeric settings (volume, rate, pitch, sample_rate)
- `PremiumBadge` inline for premium-only rows (non-blocking, informational)
- `disabled={!isPremium}` + `disabled:cursor-not-allowed disabled:opacity-50` classes for premium-gated inputs
- `text-sm text-text-sub` for row labels; `text-xs text-text-dim` for helpers

**Preserve (FilterGroup chip-list):** the `PlatformChipRow` implementation for `tts_enabled_platforms` mirrors the `TagInput` chip style (but with pre-defined platform chips, not free-text tags).

**Preserve (UI-SPEC contract):**
- Sub-section header style: `border-t border-border pt-4 mt-4` + `text-xs font-semibold uppercase tracking-wide text-text-dim` (verbatim from UI-SPEC "Tailwind / Styling Rules" § 650-653)
- First sub-section (Voice) has NO `border-t pt-4 mt-4` (see UI-SPEC line 653: "first header omits")
- Password input: add `font-mono` class to `className` (the one deviation from SoundGroup's custom URL input — UI-SPEC line 660)
- Read-only OBS URL input: `select-all` CSS + `readOnly` attribute
- Premium overlay wrap: `<div class="relative">` + `<div class="absolute inset-0 flex items-center justify-center bg-surface/80">` (UI-SPEC line 661)

#### `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx` (test)

**Analog:** `frontend/src/components/appearance/__tests__/SoundGroup.test.tsx`

**Scaffold from `frontend/src/components/appearance/__tests__/SoundGroup.test.tsx:19-42`:**
```tsx
// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest'
import React from 'react'
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { SoundGroup } from '../SoundGroup'
import type { SoundGroupProps } from '../SoundGroup'
import type { DisplaySettings } from '@/lib/types/overlay'

afterEach(() => { cleanup() })

function renderSoundGroup(overrides: Partial<SoundGroupProps> = {}) {
  const defaultProps: SoundGroupProps = {
    displaySettings: { notification_sound_enabled: true, ... },
    onChange: vi.fn(),
    isPremium: false,
    ...overrides,
  }
  return { ...render(<SoundGroup {...defaultProps} />), onChange: defaultProps.onChange }
}
```

**Preserve:**
- `// @vitest-environment jsdom` directive (line 19)
- AGPL-3.0 JSDoc header
- `afterEach(() => { cleanup() })`
- `renderXGroup(overrides)` helper returning `{ ...render(...), onChange }` for handler assertions
- `screen.getByRole('switch')`, `screen.getByLabelText(...)`, `fireEvent.click/change`
- Covers UI-SPEC flows: master toggle, provider radio, voice picker, chip toggle, premium gating, Test voice button speaking/idle states, inline confirm for Remove key

#### `frontend/src/lib/api/tts.ts` (or inline into `overlays.ts`) (utility, request-response)

**Analog:** `frontend/src/lib/api/overlays.ts:39-95`

**Wrapper idiom from `frontend/src/lib/api/overlays.ts:86-95`:**
```typescript
async getConfig(id: string): Promise<OverlayConfig> {
  return apiClient.get<OverlayConfig>(`/api/v1/overlays/${id}/config`)
},

async updateConfig(id: string, config: Partial<OverlayConfig>): Promise<OverlayConfig> {
  return apiClient.put<OverlayConfig>(`/api/v1/overlays/${id}/config`, config)
},
```

**7 endpoints to wrap (per RESEARCH Open Question 3):**
1. `saveTTSKey(id, apiKey, voiceId)` → `apiClient.post(/api/v1/overlays/:id/tts-config, {api_key, voice_id})`
2. `deleteTTSKey(id)` → `apiClient.delete(/api/v1/overlays/:id/tts-config)`
3. `rotateTTSToken(id)` → `apiClient.post(/api/v1/overlays/:id/tts-config/rotate-token, {})`
4. `getTTSVoices(id)` → `apiClient.get(/api/v1/overlays/:id/tts-voices)`
5. `testTTSKey(id)` → special: returns a `Blob` (audio/mpeg) plus `x-characters-remaining` / `x-characters-limit` headers — may need a direct `fetch` call rather than `apiClient` if the wrapper only returns JSON. Check `frontend/src/lib/api/client.ts` capabilities.
6. `getTTSConfig(id)` → `apiClient.get(/api/v1/overlays/:id/tts-config)` returns `{has_elevenlabs_config, voice_id?, obs_url?}`
7. (Note: `POST /api/v1/overlays/:id/tts` is called directly by `ttsPlayer.ts` via `fetch`, not through the api client — because it returns a streaming `audio/mpeg` blob)

**Preserve:**
- AGPL-3.0 JSDoc header
- Named imports from `'./client'` (apiClient)
- Type-safe wrappers; no `any` types
- Per-method JSDoc comment

#### `frontend/src/lib/hooks/useBrowserVoices.ts` (hook)

**Analog:** No direct analog; general React hook pattern. Use the shape in RESEARCH Example 5 verbatim:

```typescript
export function useBrowserVoices(): SpeechSynthesisVoice[] {
  const [voices, setVoices] = useState<SpeechSynthesisVoice[]>([])

  useEffect(() => {
    if (typeof window === 'undefined' || !window.speechSynthesis) return

    const update = () => setVoices(window.speechSynthesis.getVoices())

    update()
    window.speechSynthesis.addEventListener('voiceschanged', update)
    return () => window.speechSynthesis.removeEventListener('voiceschanged', update)
  }, [])

  return voices
}
```

**Preserve:**
- AGPL-3.0 JSDoc header
- `'use client'` directive if the hook is imported by a client component (TTSGroup is `'use client'`)
- SSR guard: `typeof window === 'undefined'`
- Cleanup function returned from useEffect (removes the listener)
- Handles Pitfall 1 (Chromium empty-on-first-call) via `voiceschanged` event

### Integration Sites

#### `services/overlay-manager/cmd/main.go` (modify)

**Reference blocks to splice in — source: `services/share-service/cmd/main.go`:**

**Imports to add (near line 33-39):**
```go
"github.com/caesar/all-chat/shared/encryption"
"github.com/caesar/all-chat/shared/featuregates"    // moved from share-service
// middleware.RequirePremium is now available from shared/middleware; no alias needed
```

**Cache init (after Redis connect, near current line 122) — pattern from `services/share-service/cmd/main.go:102-107`:**
```go
gateCache := featuregates.NewFeatureGateCache(dbPool, redisClient, log)
if err := gateCache.Start(context.Background()); err != nil {
    log.Fatal("Failed to start feature gate cache", zap.Error(err))
}
```

**AES encryptor init — pattern from `services/auth-service/cmd/main.go:132-141`:**
```go
tokenEncryptionKey := getEnv("TOKEN_ENCRYPTION_KEY", "")
if tokenEncryptionKey == "" {
    log.Fatal("TOKEN_ENCRYPTION_KEY environment variable required")
}
parsedKey, err := encryption.ParseKey(tokenEncryptionKey)
if err != nil {
    log.Fatal("failed to parse TOKEN_ENCRYPTION_KEY", zap.Error(err))
}
tokenCipher, err := encryption.NewAESEncryptor(parsedKey)
if err != nil {
    log.Fatal("failed to initialize token cipher", zap.Error(err))
}
```

**Routes — pattern from `services/share-service/cmd/main.go:185-194`:**
```go
// 5 premium-gated endpoints (POST/DELETE/rotate-token/voices/test)
ttsPremiumRoutes := protected.Group("")
ttsPremiumRoutes.Use(middleware.RequirePremium(dbPool, gateCache, "tts", log))
{
    ttsPremiumRoutes.POST("/:id/tts-config", ttsHandler.HandleSaveTTSConfig)
    ttsPremiumRoutes.DELETE("/:id/tts-config", ttsHandler.HandleDeleteTTSConfig)
    ttsPremiumRoutes.POST("/:id/tts-config/rotate-token", ttsHandler.HandleRotateToken)
    ttsPremiumRoutes.GET("/:id/tts-voices", ttsHandler.HandleGetVoices)
    ttsPremiumRoutes.POST("/:id/tts-config/test", ttsHandler.HandleTestKey)
}

// 7th endpoint: read-only config (auth only, no premium gate — graceful premium-loss)
protected.GET("/:id/tts-config", ttsHandler.HandleGetTTSConfig)

// 6th endpoint: streaming proxy with tts_token JWT auth (NOT user JWT)
// Mounted outside `protected` group — verify JWT inside the handler
router.POST("/:id/tts", ttsHandler.HandleTTS)  // or under a public group
```

**Preserve:**
- AGPL-3.0 header (already present — do not remove on edit)
- Graceful shutdown hook (lines 299-313 — do NOT alter)
- Existing `httpMetricsMiddleware` wiring (lines 128-198 — do NOT break metrics label consistency)
- Existing Zap logger patterns (`log.Fatal`, `log.Info`, `zap.String`, `zap.Error`)
- The `protected := router.Group("/")` + `protected.Use(middleware.JWTAuth(config.JWTSecret))` structure (line 223-225) — the 5 premium endpoints go inside this group
- The public `router.GET("/public/:id/config", ...)` route (line 217) is UNCHANGED — it must never return the encrypted key (threat model: public-config leak)

#### `frontend/src/components/appearance/AppearancePanel.tsx` (modify)

**Current mount block to duplicate (line 89-97):**
```tsx
{displaySettings && onSoundChange && (
  <CollapsibleSection id="sounds" title="Notification Sounds">
    <SoundGroup
      displaySettings={displaySettings}
      onChange={onSoundChange}
      isPremium={isPremium ?? false}
    />
  </CollapsibleSection>
)}
```

**Add IMMEDIATELY AFTER the `sounds` block:**
```tsx
{displaySettings && onTTSChange && (
  <CollapsibleSection id="tts" title="Text-to-Speech">
    <TTSGroup
      displaySettings={displaySettings}
      onChange={onTTSChange}
      isPremium={isPremium ?? false}
      overlayId={overlayId}
      hasElevenLabsConfig={hasElevenLabsConfig ?? false}
      obsUrl={obsUrl}
      onPreview={onTTSPreview}
      onPreviewStop={onTTSPreviewStop}
      onSaveKey={onSaveTTSKey!}
      onTestKey={onTestTTSKey!}
      onRotateToken={onRotateTTSToken!}
      onRemoveKey={onRemoveTTSKey!}
      onFetchVoices={onFetchTTSVoices!}
    />
  </CollapsibleSection>
)}
```

**Extend `AppearancePanelProps` with the new optional callbacks (D-11..D-16).**

#### `frontend/src/app/overlay/[id]/page.tsx` (modify)

**Four insertion points — mirror the Phase 12 sound-player touches:**

1. **Ref declaration (after line 123):**
   ```tsx
   const ttsPlayerRef = useRef<TTSPlayer | null>(null)
   const ttsSettingsRef = useRef<TTSSettings>({ /* safe defaults from D-20 */ })
   ```

2. **Destroy effect (mirror lines 138-144):**
   ```tsx
   useEffect(() => {
     return () => {
       ttsPlayerRef.current?.destroy()
       ttsPlayerRef.current = null
     }
   }, [])
   ```

3. **Config load (inside the existing `loadConfig` async fn, after line 256):**
   ```tsx
   // Phase 13: Load TTS settings from display_settings
   const tts_enabled = display.tts_enabled === true
   const tts_provider = display.tts_provider === 'elevenlabs' ? 'elevenlabs' : 'browser'
   // ... projection of all 20 tts_* fields ...
   const newTTSSettings: TTSSettings = { /* ... */ }
   ttsSettingsRef.current = newTTSSettings
   if (ttsPlayerRef.current) {
     ttsPlayerRef.current.updateSettings(newTTSSettings)
   } else {
     ttsPlayerRef.current = createTTSPlayer(newTTSSettings, handleTTSFallback)
   }
   ```

4. **Playback hook (adjacent to line 414):**
   ```tsx
   // Phase 12: play notification sound for messages that pass the filter (D-05)
   soundPlayerRef.current?.play()
   // Phase 13: speak the message via TTS (D-41, D-42 — independent of sound)
   ttsPlayerRef.current?.speak(message)
   ```

**Preserve:**
- The `shouldFilterMessage(message, filterSettingsRef.current)` return BEFORE the sound/TTS calls (filtered messages → neither fires — D-42)
- `ttsSettingsRef.current` is updated in tandem with the player, same pattern as `soundSettingsRef`
- `handleTTSFallback` is a locally-scoped closure that fires ONE `react-hot-toast` (D-38)

#### `frontend/src/app/overlays/[id]/preview/embed/page.tsx` (modify)

**Three insertion points — mirror `SOUND_SETTINGS_UPDATE` touches (lines 48-51, 272-288, 347-370, 397-400):**

1. **Imports (add after line 51):**
   ```tsx
   import { createTTSPlayer } from '@/lib/utils/ttsPlayer'
   import type { TTSPlayer, TTSSettings } from '@/lib/utils/ttsPlayer'
   ```

2. **Refs in component body:** mirror `soundPlayerRef` / `soundSettingsRef`

3. **`TTS_SETTINGS_UPDATE` listener (splice IMMEDIATELY AFTER line 288 `SOUND_SETTINGS_UPDATE` block):**
   ```tsx
   if (event.data?.type === 'TTS_SETTINGS_UPDATE') {
     const s = event.data.ttsSettings as Partial<DisplaySettings>
     const newSettings: TTSSettings = { /* project s.tts_* onto TTSSettings */ }
     ttsSettingsRef.current = newSettings
     if (ttsPlayerRef.current) {
       ttsPlayerRef.current.updateSettings(newSettings)
     } else {
       ttsPlayerRef.current = createTTSPlayer(newSettings, handleTTSFallback)
     }
     return
   }
   ```

4. **Config load (splice IMMEDIATELY AFTER line 370):** mirror the `newSoundSettings` construction with `newTTSSettings`

5. **Playback hook (adjacent to line 400):** add `ttsPlayerRef.current?.speak(message)`

#### `frontend/src/app/overlays/[id]/page.tsx` (modify)

**Config-load extension (after line 1403):**
```tsx
// Phase 13: Load TTS settings from display_settings
const ttsLoaded: Partial<DisplaySettings> = {}
if (typeof d.tts_enabled === 'boolean') ttsLoaded.tts_enabled = d.tts_enabled
// ... all 20 fields ...
setTTSSettings(ttsLoaded)

// Phase 13: Load TTS ElevenLabs config metadata (separate endpoint)
try {
  const cfg = await overlaysApi.getTTSConfig(id)
  setHasElevenLabsConfig(cfg.has_elevenlabs_config)
  setObsUrl(cfg.obs_url ?? null)
} catch { /* non-fatal */ }
```

**Save handlers (new):** `handleSaveTTSKey`, `handleRotateTTSToken`, `handleTestTTSKey`, `handleRemoveTTSKey`, `handleFetchTTSVoices` — each wraps the corresponding `overlaysApi.*` call with try/catch and uses `toastManager.add` (already imported at line 1436) for success/error copy verbatim from UI-SPEC line 163-178.

**Copy OBS URL button:** `navigator.clipboard.writeText(obsUrl)` + `toastManager.add({title: 'OBS URL copied.', type: 'success'})` (UI-SPEC line 176)

**Regenerate URL button:** open an `@base-ui/react/alert-dialog` (UI-SPEC line 637 — already used elsewhere in the project; grep `AlertDialog` or `alert-dialog` for analogs)

---

## Shared Patterns

### AGPL-3.0 License Header (ALL new source files)

**Source:** commit `b499543a` (verified); every existing `.go`, `.ts`, `.tsx` file carries this header.

**Go style (`// ` comments):**
```go
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
```

**TypeScript / TSX style (`/** */` JSDoc):**
```typescript
/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */
```

**SQL migration headers:** use plain `--` comments (see `migrations/048_*.sql:1-5`); no AGPL boilerplate is the existing convention.

**Apply to:** Every new file except `migrations/049_overlay_tts_configs.sql`.

### Gin Handler Owner-Check + Unauthorized

**Source:** `services/overlay-manager/handlers/config.go:47-64`

```go
userID, exists := c.Get("user_id")
if !exists {
    c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
    return
}

if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
    c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
    return
}
```

**Apply to:** `HandleSaveTTSConfig`, `HandleDeleteTTSConfig`, `HandleRotateToken`, `HandleGetVoices`, `HandleTestKey`, `HandleGetTTSConfig` (all 5 user-JWT endpoints + the 7th GET).

**Deliberately NOT applied to:** `HandleTTS` (the proxy) — that endpoint uses `tts_token` JWT verification against the per-overlay signing secret stored in `overlay_tts_configs.tts_signing_secret`, not the user JWT.

### AES-GCM Encrypt/Decrypt

**Source:** `shared/encryption/encryption.go` (VERIFIED production-deployed in `auth-service`)

**Service-wiring pattern from `services/auth-service/cmd/main.go:132-141`:**
```go
tokenEncryptionKey := getEnv("TOKEN_ENCRYPTION_KEY", "")
parsedKey, err := encryption.ParseKey(tokenEncryptionKey)
tokenCipher, err := encryption.NewAESEncryptor(parsedKey)
// pass tokenCipher into the TTS handler constructor
```

**Per-request encrypt pattern:**
```go
encrypted, err := tokenCipher.EncryptString(userElevenLabsKey)
// store []byte(encrypted) into overlay_tts_configs.encrypted_api_key BYTEA column
```

**Per-request decrypt pattern:**
```go
plaintext, err := tokenCipher.DecryptString(string(cfg.EncryptedAPIKey))
// use plaintext as xi-api-key header; never log
```

**Apply to:** `services/overlay-manager/handlers/tts.go`. Env var is `TOKEN_ENCRYPTION_KEY` (NOT `CRYPTO_MASTER_KEY` as D-07 originally said — researcher overrode per "Claude's Discretion"; see RESEARCH "Runtime State Inventory" § A7).

**Deployment:** Plan 02 must add `TOKEN_ENCRYPTION_KEY` to `deployments/k8s/base/overlay-manager/deployment.yaml` (mirror `deployments/k8s/base/auth-service/deployment.yaml:107`). Flag this to the planner — it's an infra-change that often gets forgotten.

### JWT Algorithm-Confusion Guard

**Source:** `shared/auth/jwt.go:156-161`

```go
token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
    if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
        return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
    }
    return []byte(secret), nil
})
```

**Apply to:** `services/overlay-manager/tts/jwt.go` `VerifyOverlayToken` function.

### Zap Logger — Never Log Secrets

**Source:** pattern observed across all services (no `zap.String("apiKey", ...)` greppable in tree)

**Rule:** `logger.Error("TTS upstream failed", zap.Error(err))` is OK. `logger.Error("TTS decrypt failed", zap.String("cipher", string(cfg.EncryptedAPIKey)))` is NOT OK. `logger.Info("saved TTS config", zap.String("api_key", apiKey))` is NOT OK. Err on the side of logging only IDs and generic error types. The V8 threat-model item "ElevenLabs key theft via logs" is the reason — one stray log line defeats the encryption-at-rest story.

**Apply to:** All new `.go` files.

### Frontend Utility Pattern

**Source:** `frontend/src/lib/utils/soundPlayer.ts`

- Pure utility; no React imports
- `export function createXxxPlayer(initialSettings): XxxPlayer`
- Returns `{ play/speak, updateSettings, destroy }` — three exported methods
- Closure-captured mutable state (`let settings = { ...initialSettings }`)
- `destroy()` nulls refs, clears timers, stops in-flight audio

**Apply to:** `frontend/src/lib/utils/ttsPlayer.ts`.

### Frontend Group-Component Pattern

**Source:** `frontend/src/components/appearance/SoundGroup.tsx`

- `'use client'` directive
- `export interface XxxGroupProps { displaySettings, onChange, isPremium, onPreview? }`
- Root `<div className="space-y-4">`
- Master `<ToggleSwitch>` + `{enabled && (<>...</>)}`
- `SliderControl` / `ToggleSwitch` / native `<select>` / native `<input>` for form controls
- `<PremiumBadge />` inline decoration for premium rows
- `disabled={!isPremium}` on premium-gated inputs

**Apply to:** `frontend/src/components/appearance/TTSGroup.tsx`.

### Live-Preview postMessage Pattern

**Source:** `frontend/src/app/overlays/[id]/preview/embed/page.tsx:272-288` (`SOUND_SETTINGS_UPDATE` idiom)

Producer (editor page):
```ts
previewIframeRef.current?.contentWindow?.postMessage(
  { type: 'TTS_SETTINGS_UPDATE', ttsSettings: newTTSSettings },
  '*',
)
```

Consumer (embed page):
```ts
if (event.data?.type === 'TTS_SETTINGS_UPDATE') {
  const s = event.data.ttsSettings as Partial<DisplaySettings>
  const newSettings: TTSSettings = { /* ... */ }
  ttsSettingsRef.current = newSettings
  if (ttsPlayerRef.current) {
    ttsPlayerRef.current.updateSettings(newSettings)
  } else {
    ttsPlayerRef.current = createTTSPlayer(newSettings, handleTTSFallback)
  }
  return
}
```

**Apply to:** `frontend/src/app/overlays/[id]/page.tsx` (producer) + `frontend/src/app/overlays/[id]/preview/embed/page.tsx` (consumer). No debouncing per D-22.

### Vitest Scaffold

**Source:** `frontend/src/lib/utils/__tests__/soundPlayer.test.ts:22-62` + `frontend/src/components/appearance/__tests__/SoundGroup.test.tsx:19-42`

Utility test:
```ts
vi.stubGlobal('Audio', MockAudio)
describe('createTTSPlayer', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    // clear mocks
  })
  afterEach(() => { vi.useRealTimers() })
})
```

Component test:
```tsx
// @vitest-environment jsdom
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
afterEach(() => { cleanup() })

function renderTTSGroup(overrides: Partial<TTSGroupProps> = {}) {
  const defaultProps: TTSGroupProps = { /* ... */ }
  return { ...render(<TTSGroup {...defaultProps} />), onChange: defaultProps.onChange }
}
```

**Apply to:** `frontend/src/lib/utils/__tests__/ttsPlayer.test.ts` + `frontend/src/components/appearance/__tests__/TTSGroup.test.tsx`.

### Go Handler Test Scaffold

**Source:** `services/share-service/middleware/premium_test.go:29-89`

- `gin.SetMode(gin.TestMode)` once per test file
- Narrow-interface mocks for DB/cache/upstream HTTP
- `httptest.NewRecorder()` + `httptest.NewRequest(...)` + `router.ServeHTTP(w, req)`
- `github.com/stretchr/testify/assert` for assertions
- `httptest.NewServer` for upstream ElevenLabs mocking (stdlib standard, not currently greppable in tree but widely used)

**Apply to:** `services/overlay-manager/handlers/tts_test.go` + `services/overlay-manager/tts/jwt_test.go` + `services/overlay-manager/repository/tts_config_repo_test.go`.

---

## No Analog Found

None. Every file in this phase has a strong analog already in tree. The closest "partial match" is:

| File | Role | Data Flow | Reason | Fallback |
|------|------|-----------|--------|----------|
| `frontend/src/lib/hooks/useBrowserVoices.ts` | hook | event-driven | No existing `.ts` files under `frontend/src/lib/hooks/` — hooks are typically inlined into components. | Use RESEARCH Example 5 verbatim; it is a textbook `useEffect` + `addEventListener` + cleanup pattern. |
| `services/overlay-manager/repository/tts_config_repo_test.go` | test | test | No existing overlay-manager repository test file uses an encryption-roundtrip assertion. | Either (a) use `pgxmock` for an in-memory mock, or (b) rely on `shared/encryption/encryption_test.go` for the roundtrip and keep this repo-level test focused on SQL correctness (column scanning, WHERE clause). |

These gaps are minor and do not block planning.

---

## Critical Recap: Overrides the Planner MUST Respect

1. **Use `shared/encryption`, NOT a new `shared/crypto`.** D-07 says "new `shared/crypto` package" but research proved this is factually redundant: `shared/encryption.AESEncryptor` is production-deployed in auth-service with the exact spec. RESEARCH Line 9, 151, 172, 419, 604, 963-964 all cite this. `shared/crypto/crypto.go` also exists but is not actively imported anywhere — flag it for removal in a follow-up quick task, but do NOT add a third package.

2. **Use env var `TOKEN_ENCRYPTION_KEY`, NOT `CRYPTO_MASTER_KEY`.** D-07 says `CRYPTO_MASTER_KEY`; research overrides per Claude's Discretion (RESEARCH Line 440). Reuse the existing auth-service env var for consistency — it's already wired in secrets management.

3. **MOVE, don't duplicate.** `shared/featuregates/cache.go` and `shared/middleware/premium.go` are byte-for-byte moves from `services/share-service/`. ADR-0008 explicitly blessed this move once a second service needs the packages. Overlay-manager is that second service.

4. **6-endpoint scope grows to 7.** RESEARCH Open Question 3 confirms: `GET /api/v1/overlays/:id/tts-config` is needed so the frontend knows whether a key is saved (`{has_elevenlabs_config, voice_id?, obs_url?}`). D-11..D-16 give only 6; the planner adds the 7th. Auth-required but NOT behind `RequirePremium` (read-only metadata is accessible to downgraded users for graceful premium loss).

5. **AGPL header on every new file.** Commit `b499543a` established this; plan-checker will reject PRs without it. The sole exception is `.sql` migration files, which use plain `--` comments per the 048/044 convention.

6. **`promauto` blind spot.** Before moving `shared/featuregates/cache.go`, verify `grep -n promauto cache.go` returns zero (research confirmed it does today). If the move ever adds Prometheus metrics, use `prometheus.Registerer` injection (per Phase 08 STATE.md guidance) — NOT `promauto.NewCounter(...)`.

7. **Client-disconnect propagation.** In `HandleTTS` (the streaming proxy), always pass `c.Request.Context()` into `http.NewRequestWithContext(...)` — Pitfall 7. Without this, user closes OBS → overlay-manager keeps hammering ElevenLabs → user's quota drains.

---

## Metadata

**Analog search scope:**
- `services/overlay-manager/**`, `services/share-service/**`, `services/auth-service/**`
- `shared/encryption/**`, `shared/auth/**`, `shared/middleware/**`, `shared/crypto/**`
- `migrations/044*.sql`, `migrations/048*.sql`
- `frontend/src/lib/utils/**`, `frontend/src/lib/api/**`, `frontend/src/lib/types/overlay.ts`
- `frontend/src/components/appearance/**` (all 10 `*Group.tsx` files inspected)
- `frontend/src/app/overlay/[id]/page.tsx`, `frontend/src/app/overlays/[id]/page.tsx`, `frontend/src/app/overlays/[id]/preview/embed/page.tsx`

**Files scanned:** ~30 files read directly; ~40 files located via Glob/Grep.

**Pattern extraction date:** 2026-04-23

**Research overrides acknowledged:**
- Reuse `shared/encryption` instead of building `shared/crypto` (RESEARCH § Scope Area 1 + Open Question 1)
- Move `featuregates` and `middleware.RequirePremium` to `shared/` (RESEARCH § "Recommended Project Structure")
- Add a 7th endpoint `GET /tts-config` (RESEARCH § Open Question 3)
- Use env var `TOKEN_ENCRYPTION_KEY` (RESEARCH § Runtime State Inventory A7)

## PATTERN MAPPING COMPLETE
