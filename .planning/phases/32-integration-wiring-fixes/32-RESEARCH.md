# Phase 32: Integration Wiring Fixes - Research

**Researched:** 2026-03-16
**Domain:** Go SQL, TypeScript WebSocket message handling, Gin HTTP routing
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| BADGE-02 | Premium users automatically receive a gem/star icon badge shown in overlays | SQL fix: LEFT JOIN viewers v ON v.id = vpi.viewer_id; read v.is_premium instead of u.is_premium |
| PREM-02 | Gradient name renders in overlay using CSS background-clip: text — no JavaScript required | ws.onmessage parse: JSON.parse on name_gradient string before calling buildGradientCSS |
| PREM-03 | Premium viewer can select an avatar frame (decorative PNG ring overlaid on their avatar) | 2 catalog route registrations missing from api-gateway publicAPI block |
| PREM-04 | Premium viewer can select an avatar flair (small corner icon pinned to bottom-right of avatar) | 2 catalog route registrations missing from api-gateway publicAPI block |
| PREM-05 | Frame and flair catalog is managed by admins (add/remove items, mark as premium-only) | 6 admin cosmetics routes missing from api-gateway protectedAPI block |
| WEB-03 | Premium users can browse and select avatar frame from the frame catalog | Same gateway gap as PREM-03 |
| WEB-04 | Premium users can browse and select avatar flair from the flair catalog | Same gateway gap as PREM-04 |
</phase_requirements>

---

## Summary

Phase 32 closes the three integration-level wiring gaps identified in the v1.4 milestone audit. All five preceding phases (27–31) passed their own verification suites; the breaks were discovered only through cross-phase integration checking. The fixes are surgical: one SQL query change, one TypeScript parse call in an existing handler, and eight route registrations in an existing Gin router file.

**Plan 32-01** corrects `viewer_badge_enricher.go`: the enricher's SQL query joins `viewer_sessions → users u` to derive `u.is_premium`, which is the *streamer* premium flag (from migration 030). The `viewers` table carries the actual viewer premium status (`viewers.is_premium`, migration 036) and is never joined. Adding `LEFT JOIN viewers v ON v.id = vpi.viewer_id` and scanning `COALESCE(v.is_premium, false)` in place of `COALESCE(u.is_premium, false)` fixes BADGE-02. The `fakeViewerDB` test double in `viewer_badge_enricher_test.go` already supports 7-column scans and existing `TestEnrich_PremiumBadge` will catch a regression.

**Plan 32-02** corrects the overlay WebSocket message handler in `frontend/src/app/overlay/[id]/page.tsx`. The Go message processor serializes `msg.User.NameGradient` as a raw JSON string (e.g. `"{\"type\":\"linear\",...}"`). The `UserInfo` TypeScript interface declares `name_gradient?: NameGradient` (a structured object). When the overlay calls `buildGradientCSS(message.user.name_gradient)` without first parsing, `g.colors.join()` throws a TypeError because `g` is a string, not a `NameGradient` object. The extension path (`ChatContainer.tsx`) correctly applies `JSON.parse` in a `useMemo`. The fix is to insert a parse guard in `ws.onmessage` at the same point where `sortMessageBadges` is already applied.

**Plan 32-03** adds 8 proxy routes to `services/api-gateway/cmd/main.go`. Auth-service registers `GET /viewer/catalog/frames`, `GET /viewer/catalog/flairs` (public, no auth) and `GET/POST/DELETE /admin/cosmetics/frames`, `GET/POST/DELETE /admin/cosmetics/flairs` (protected, admin JWT). The gateway's `publicAPI` and `protectedAPI` Gin groups proxy via `proxyHandler.ForwardRequest`. None of these 8 routes are currently forwarded; all return 404.

**Primary recommendation:** Implement all three fixes as three sequential plans. No new libraries, no schema changes, no new services — pure wiring closure.

---

## Standard Stack

### Core (already in use — no new dependencies)

| Library/Tool | Version | Purpose | Relevance |
|---|---|---|---|
| `pgx/v5` | v5 | PostgreSQL driver for Go | SQL query in viewer_badge_enricher.go |
| `go-redis/v9` | v9 | Redis client | Cache TTL invalidation after enricher SQL fix |
| `gin-gonic/gin` | v1 | HTTP router | Route registration in api-gateway |
| `gorilla/websocket` | v1 | WebSocket server in api-gateway | ws.onmessage is frontend TypeScript, not Go |
| TypeScript | 5+ | Frontend language | JSON.parse fix |
| Vitest | current | Frontend test runner | `npx vitest run` |
| Go test | stdlib | Go unit tests | `go test ./enricher/...` |

### Supporting
No new libraries required. All three fixes use constructs already present in the codebase.

---

## Architecture Patterns

### Fix 1: Enricher SQL JOIN Pattern

The enricher query pattern follows the existing multi-LEFT-JOIN structure already used for `cosmetic_frames`, `cosmetic_flairs`, and `viewer_sessions`. Adding `LEFT JOIN viewers v ON v.id = vpi.viewer_id` follows the same pattern.

Current query structure:
```sql
SELECT vpi.viewer_id::text, vc.name_color, vc.name_gradient,
       COALESCE(cf.image_url, '') AS avatar_frame_url,
       COALESCE(cfl.image_url, '') AS avatar_flair_url,
       COALESCE(u.is_admin, false) AS is_admin,
       COALESCE(u.is_premium, false) AS is_premium   -- BUG: reads users.is_premium (streamer flag)
FROM viewer_platform_identities vpi
LEFT JOIN viewer_cosmetics vc ON vc.viewer_id = vpi.viewer_id
LEFT JOIN cosmetic_frames cf ON cf.id = vc.avatar_frame_id
LEFT JOIN cosmetic_flairs cfl ON cfl.id = vc.avatar_flair_id
LEFT JOIN LATERAL (SELECT user_id FROM viewer_sessions WHERE viewer_id = vpi.viewer_id LIMIT 1) vs ON true
LEFT JOIN users u ON u.id = vs.user_id
WHERE vpi.platform = $1 AND vpi.platform_user_id = $2
```

Fixed query adds `LEFT JOIN viewers v ON v.id = vpi.viewer_id` and changes the `is_premium` column reference. The `is_admin` still correctly reads from `u.is_admin` (admin status is a streamer/owner concept in the `users` table). The Scan call order must match the SELECT column order exactly — the 7th dest argument (`*bool` for `isPremium`) currently reads `u.is_premium`; after the fix it will read `v.is_premium` (no scan order change needed, just the SQL column source changes).

**Critical detail:** The Redis cache stores `IsPremium bool` in `viewerIdentityCache`. After the SQL fix the cached value will be correct for new lookups. Existing cache entries with wrong `IsPremium: false` will expire within `ViewerIdentityCacheTTL` (5 minutes) — no cache flush needed.

### Fix 2: ws.onmessage Parse Pattern

The overlay page already applies `sortMessageBadges(message)` in the `ws.onmessage` handler immediately after receiving a `chat_message` envelope (line 238). The gradient parse should follow the same pattern — applied inline, before `setMessages`.

Current flow (broken):
```typescript
// ws.onmessage, ~line 236
let message: ChatMessage = envelope.data;
message = await resolveTwitchBadgeIcons(message);
message = sortMessageBadges(message);
// name_gradient is still a raw JSON string here
setMessages(...)
```

The `UserInfo` TypeScript interface types `name_gradient` as `NameGradient | undefined`. The Go WebSocket payload contains the field as a JSON-serialized string (Go's `string` type encodes as a JSON string value, not a nested object). `JSON.parse` on a well-formed gradient JSON string returns a `NameGradient` object matching the interface.

Fix pattern (consistent with extension `ChatContainer.tsx` which also uses `JSON.parse`):
```typescript
if (msg.user?.name_gradient && typeof msg.user.name_gradient === 'string') {
  msg.user.name_gradient = JSON.parse(msg.user.name_gradient as unknown as string);
}
```

This guard should be applied to both the `chat_message` and `message_update` branches in `ws.onmessage` (both call `sortMessageBadges` and produce messages rendered with the gradient CSS path).

**TypeScript note:** `UserInfo.name_gradient` is typed as `NameGradient | undefined`. At runtime the incoming value is `string | undefined`. The `typeof` guard is required; the `as unknown as string` cast bypasses the type system for the parse — this is the correct approach matching the extension pattern.

### Fix 3: API Gateway Route Registration Pattern

The gateway uses two Gin groups: `publicAPI` (no auth) and `protectedAPI` (JWT middleware applied). All route registrations call `proxyHandler.ForwardRequest` — the proxy handler reads the request path and forwards to the appropriate service via the service registry. No handler-specific code is needed.

Auth-service route → Gateway route mapping:

| Auth-service path | Auth-service group | Gateway group | Gateway path |
|---|---|---|---|
| `GET /viewer/catalog/frames` | `viewerPublic` (no auth) | `publicAPI` | `GET /auth/viewer/catalog/frames` |
| `GET /viewer/catalog/flairs` | `viewerPublic` (no auth) | `publicAPI` | `GET /auth/viewer/catalog/flairs` |
| `GET /admin/cosmetics/frames` | `admin` (JWT + AdminOnly) | `protectedAPI` | `GET /admin/cosmetics/frames` |
| `POST /admin/cosmetics/frames` | `admin` (JWT + AdminOnly) | `protectedAPI` | `POST /admin/cosmetics/frames` |
| `DELETE /admin/cosmetics/frames/:id` | `admin` (JWT + AdminOnly) | `protectedAPI` | `DELETE /admin/cosmetics/frames/:id` |
| `GET /admin/cosmetics/flairs` | `admin` (JWT + AdminOnly) | `protectedAPI` | `GET /admin/cosmetics/flairs` |
| `POST /admin/cosmetics/flairs` | `admin` (JWT + AdminOnly) | `protectedAPI` | `POST /admin/cosmetics/flairs` |
| `DELETE /admin/cosmetics/flairs/:id` | `admin` (JWT + AdminOnly) | `protectedAPI` | `DELETE /admin/cosmetics/flairs/:id` |

The gateway's `protectedAPI` group uses `sharedmiddleware.JWTAuth(jwtSecret)` only — it does NOT apply an `AdminOnly()` check. Admin role enforcement happens at the auth-service level for admin routes (auth-service registers the routes under its own `admin` group with `middleware.AdminOnly()`). This is the existing pattern (see `GET /admin/users`, `POST /admin/premium/users/:id`).

The catalog routes (`GET /auth/viewer/catalog/frames`, `GET /auth/viewer/catalog/flairs`) are public (no auth) in both auth-service and the gateway — confirmed by auth-service registration under `viewerPublic` group and the audit description "public block — no auth."

**Insertion points in `services/api-gateway/cmd/main.go`:**
- Public block: after line 374 (after `publicAPI.GET("/overlays/public/:id/credit-roll", ...)`)
- Protected block: after line 453 (after `protectedAPI.GET("/admin/sources", ...)`)

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Proxy logic for new routes | Custom forwarding handler | `proxyHandler.ForwardRequest` (already exists) | Handles service registry lookup, header forwarding, error mapping |
| Type assertion for gradient parse | Custom deserialization | `JSON.parse` + `typeof` guard | Standard JS pattern; matches extension implementation |
| Cache invalidation after SQL fix | Active cache flush | Let TTL expire (5 min) | `ViewerIdentityCacheTTL` is short enough; forced flush adds complexity with no user-visible benefit |

---

## Common Pitfalls

### Pitfall 1: Scan Column Order Must Match SELECT Order
**What goes wrong:** The `row.Scan(...)` call in `viewer_badge_enricher.go` must receive destination pointers in the exact order the SELECT clause returns columns. The fix changes which table `is_premium` is read from but does NOT change the column position in the SELECT (it remains column 7). The Scan call order is unchanged. Mismatching column order causes silent wrong-column reads.
**How to avoid:** Verify SELECT column order after editing SQL; run `TestEnrich_PremiumBadge` to confirm the 7th scan dest receives the correct value.

### Pitfall 2: Gradient Parse Applied to Both Message Branches
**What goes wrong:** `ws.onmessage` has two branches that produce rendered messages: `chat_message` (line 235) and `message_update` (line 247). If the parse guard is only applied to one branch, gradient fails silently for TikTok like-aggregate updates.
**How to avoid:** Apply parse guard in both `chat_message` and `message_update` branches, immediately after `let message: ChatMessage = envelope.data`.

### Pitfall 3: TypeScript Type Narrowing for name_gradient
**What goes wrong:** The `UserInfo` interface types `name_gradient` as `NameGradient | undefined`. TypeScript will flag an assignment of `JSON.parse(...)` (which returns `unknown`) to `msg.user.name_gradient` without a cast. Using `as NameGradient` after `JSON.parse` is correct; using `as unknown as string` is needed only for the input to `JSON.parse`.
**How to avoid:** Follow the audit's recommended pattern exactly: `typeof` guard on the raw string, then `JSON.parse(msg.user.name_gradient as unknown as string) as NameGradient`.

### Pitfall 4: Catalog Routes Must Be Public (No Auth Middleware)
**What goes wrong:** Adding catalog routes to `protectedAPI` instead of `publicAPI` causes viewers who are not yet authenticated to see empty frame/flair grids (unauthorized). Auth-service explicitly registers them under `viewerPublic` (no JWT middleware).
**How to avoid:** Add `GET /auth/viewer/catalog/frames` and `GET /auth/viewer/catalog/flairs` to `publicAPI` only.

### Pitfall 5: Redis Cache Entry Still Wrong After SQL Fix
**What goes wrong:** Existing cache entries for premium viewers will still have `IsPremium: false` until TTL expires. Tests using miniredis are not affected (fresh cache per test). In production, premium viewer badge will appear within 5 minutes of deployment.
**Warning signs:** If a badge test checks live Redis state after the fix — this is not a code bug, just cache lag. Not a concern for unit tests.

---

## Code Examples

### Enricher SQL Fix
```go
// File: services/message-processor/enricher/viewer_badge_enricher.go
// Add after "LEFT JOIN users u ON u.id = vs.user_id":
LEFT JOIN viewers v ON v.id = vpi.viewer_id

// Change in SELECT:
COALESCE(v.is_premium, false) AS is_premium   -- was: COALESCE(u.is_premium, false)
```

### Overlay ws.onmessage Gradient Parse (chat_message branch)
```typescript
// File: frontend/src/app/overlay/[id]/page.tsx
// In the "chat_message" branch, after "let message: ChatMessage = envelope.data":
if (message.user?.name_gradient && typeof message.user.name_gradient === 'string') {
  message.user.name_gradient = JSON.parse(message.user.name_gradient as unknown as string) as NameGradient;
}
```

### API Gateway Route Additions (public block)
```go
// File: services/api-gateway/cmd/main.go
// In publicAPI block, after existing viewer OAuth routes:
publicAPI.GET("/auth/viewer/catalog/frames", proxyHandler.ForwardRequest)
publicAPI.GET("/auth/viewer/catalog/flairs", proxyHandler.ForwardRequest)
```

### API Gateway Route Additions (protected block)
```go
// File: services/api-gateway/cmd/main.go
// In protectedAPI block, after existing admin routes:
protectedAPI.GET("/admin/cosmetics/frames", proxyHandler.ForwardRequest)
protectedAPI.POST("/admin/cosmetics/frames", proxyHandler.ForwardRequest)
protectedAPI.DELETE("/admin/cosmetics/frames/:id", proxyHandler.ForwardRequest)
protectedAPI.GET("/admin/cosmetics/flairs", proxyHandler.ForwardRequest)
protectedAPI.POST("/admin/cosmetics/flairs", proxyHandler.ForwardRequest)
protectedAPI.DELETE("/admin/cosmetics/flairs/:id", proxyHandler.ForwardRequest)
```

---

## State of the Art

| Old Approach | Current Approach | Impact |
|---|---|---|
| `COALESCE(u.is_premium, false)` — reads streamer flag | `COALESCE(v.is_premium, false)` — reads viewer flag | Premium badge appears for viewers correctly |
| `buildGradientCSS(message.user.name_gradient)` — passes raw string | Parse guard in ws.onmessage, then call buildGradientCSS | Gradient renders without TypeError |
| 0 catalog/admin cosmetics routes in gateway | 8 routes registered | Catalog pages load; admin pages function |

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Go test framework | `go test` (stdlib) |
| Frontend test framework | Vitest |
| Go config file | `go.mod` per service |
| Frontend config file | `frontend/vitest.config.ts` |
| Go quick run (enricher) | `go test ./enricher/... -v` (from `services/message-processor/`) |
| Frontend quick run | `npx vitest run` (from `frontend/`) |
| Full Go suite | `go test ./...` (from `services/message-processor/`) |
| Full frontend suite | `npx vitest run` (from `frontend/`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BADGE-02 | Enricher injects premium badge when viewers.is_premium=true | unit | `go test ./enricher/... -run TestEnrich_PremiumBadge -v` | ✅ viewer_badge_enricher_test.go |
| BADGE-02 | Enricher does NOT inject premium badge when viewers.is_premium=false | unit | `go test ./enricher/... -run TestEnrich_AdminBadge -v` | ✅ (covers non-premium path) |
| PREM-02 | Parsed NameGradient object passed to buildGradientCSS — no TypeError | unit | `npx vitest run src/app/overlay/__tests__/gradient-render.test.tsx` | ✅ gradient-render.test.tsx (tests buildGradientCSS with object, not parse) |
| PREM-02 | ws.onmessage parse guard converts JSON string to NameGradient | unit | `npx vitest run` — new test needed | ❌ Wave 0 gap |
| PREM-03/04 | Catalog routes reachable (integration) | smoke/manual | `curl http://localhost:8080/api/v1/auth/viewer/catalog/frames` | manual |
| PREM-05 | Admin cosmetics routes reachable (integration) | smoke/manual | `curl -H "Authorization: Bearer ..." http://localhost:8080/api/v1/admin/cosmetics/frames` | manual |
| WEB-03 | Frame catalog page loads without 404 | manual | Browser check `/settings/viewer` premium section | manual |
| WEB-04 | Flair catalog page loads without 404 | manual | Browser check `/settings/viewer` premium section | manual |

### Sampling Rate
- **Per task commit:** `go test ./enricher/... -v` and `npx vitest run src/app/overlay/__tests__/gradient-render.test.tsx`
- **Per wave merge:** `go test ./... && cd frontend && npx vitest run`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `frontend/src/app/overlay/__tests__/ws-message-parse.test.ts` — covers PREM-02 ws.onmessage parse guard (tests that a raw JSON string on `name_gradient` is converted to a `NameGradient` object before `buildGradientCSS` is called)

*(All Go tests for BADGE-02 already exist in `viewer_badge_enricher_test.go`. The existing `TestEnrich_PremiumBadge` test already passes `isPremium: true` through `fakeViewerDB` and asserts the badge is injected — this test will verify the fix indirectly once the SQL is corrected. A new test specifically asserting `viewers.is_premium` is the source would require an integration DB; unit test coverage via fakeViewerDB is sufficient.)*

---

## Open Questions

1. **Admin route authorization at gateway vs auth-service**
   - What we know: gateway `protectedAPI` applies `JWTAuth` only; auth-service `admin` group applies `JWTAuth + AdminOnly`. Non-admin JWT holders who call `POST /admin/cosmetics/frames` via gateway will pass gateway auth but be rejected 403 by auth-service.
   - What's unclear: whether the planner should add a note to document this dual-layer enforcement pattern.
   - Recommendation: Document in plan comments; no code change needed — the pattern matches existing admin routes (`GET /admin/users`).

2. **`message_update` branch gradient parse**
   - What we know: `message_update` is used for TikTok like-aggregate updates. TikTok messages may have viewer-enriched gradients if the TikTok viewer is registered.
   - What's unclear: whether TikTok viewer identity lookup is active in the enricher for this path.
   - Recommendation: Apply the parse guard to `message_update` branch as well (zero cost, defensive).

---

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `services/message-processor/enricher/viewer_badge_enricher.go` — confirmed SQL bug at line 134
- Direct code inspection: `frontend/src/app/overlay/[id]/page.tsx` lines 152-250 — confirmed missing JSON.parse in ws.onmessage
- Direct code inspection: `services/api-gateway/cmd/main.go` lines 329-453 — confirmed 8 routes absent
- Direct code inspection: `services/auth-service/cmd/main.go` lines 289-334 — confirmed auth-service routes exist
- Direct code inspection: `services/message-processor/enricher/viewer_badge_enricher_test.go` — confirmed fakeViewerDB 7-return signature, TestEnrich_PremiumBadge exists
- `.planning/v1.4-MILESTONE-AUDIT.md` — authoritative gap description with file/line references and fix specifications

### Secondary (MEDIUM confidence)
- `frontend/src/lib/utils/gradient.ts` — confirmed `buildGradientCSS` expects `NameGradient` object, calls `g.colors.join()`
- `frontend/src/lib/types/message.ts` lines 70-89 — confirmed `NameGradient` interface and `UserInfo.name_gradient?: NameGradient`
- `frontend/src/app/overlay/__tests__/gradient-render.test.tsx` — existing test coverage for buildGradientCSS with object input

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all fixes use existing patterns and zero new dependencies
- Architecture: HIGH — all three insertion points identified with exact file/line references from audit + direct code inspection
- Pitfalls: HIGH — derived from direct code reading and audit's root-cause analysis

**Research date:** 2026-03-16
**Valid until:** Until Phase 32 plans are executed (stable codebase, no external API changes involved)
