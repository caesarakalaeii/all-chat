# Phase 14: Secret Rotation Infrastructure - Context

**Gathered:** 2026-04-27
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a repeatable rotation capability for the platform's three primary secret classes following a leak. Phase 14 ships the *mechanism*; the actual rotation of the leaked keys is the first run of that mechanism (silent — no forced re-login, no big-bang re-encrypt).

**In scope:**
1. **TOKEN_ENCRYPTION_KEY rotation** — versioned-key infrastructure for AES-GCM master key used by `shared/encryption`, with a background sweeper to retire old keys. Affects `users.access_token/refresh_token`, `youtube_oauth_tokens.*`, `overlay_tts_configs.encrypted_api_key`. Unifies `TOKEN_ENCRYPTION_KEY` and `YOUTUBE_TOKEN_ENCRYPTION_KEY` into one chain during the rotation.
2. **JWT signing key rotation** — `kid` header + multi-key validation across User, Viewer, and Service JWTs. Two independent key chains (`JWT_SECRET_V<n>` for user-facing, `SERVICE_JWT_SECRET_V<n>` for service-to-service).
3. **DB password rotation** — CNPG-managed via operator (preferred); runbook+script fallback if research shows CNPG can't handle app-user rotation cleanly.
4. **Encryption gap-fill** — encrypt the existing plaintext `kick_oauth_tokens.access_token/refresh_token` and `tiktok_oauth_tokens.access_token/refresh_token` columns using the new versioned scheme.

**Out of scope (deferred to later phases):**
- Redis-backed JWT denylist for targeted revocation.
- Migration to RS256 + JWKS endpoint.
- Per-service DB users (architectural change, smaller blast radius).
- Encryption of OAuth client credentials and bot tokens at rest in DB (today they live only in K8s Secret).
- Reconciling the SOPS-vs-live K8s Secret drift (31 live keys vs 11 in SOPS) — operational hazard, not a Phase 14 design problem, but a *prerequisite* for any plan step that touches `allchat-secrets`.
- TTS overlay JWT rotation — Phase 13's per-overlay `tts_signing_secret` regeneration model remains untouched. Phase 14 does NOT change TTS token rotation.

</domain>

<decisions>
## Implementation Decisions

### TOKEN_ENCRYPTION_KEY rotation strategy

- **D-01:** Ciphertext format gains a 1-byte `kid` (key id) prefix: `[v(1B)] [nonce(12B)] [ciphertext] [tag(16B)]`. New writes always emit this format.
- **D-02:** Multiple master keys live concurrently in env: `TOKEN_ENCRYPTION_KEY_V1`, `TOKEN_ENCRYPTION_KEY_V2`, ... New writes use the latest; reads pick by kid byte. Implementation in `shared/encryption` (and `shared/crypto` if both still referenced — see code_context).
- **D-03:** Re-encryption strategy is **lazy on next write + scheduled background sweeper**. Reads decrypt with whichever key matches the kid; on the next natural write to a row (token refresh, profile update, ElevenLabs key update) the row flips to the latest kid. A separate sweeper command re-encrypts long-tail rows in batches so the old key can be retired on a deadline.
- **D-04:** **Unify into one master key chain.** Drop `YOUTUBE_TOKEN_ENCRYPTION_KEY`. One `TOKEN_ENCRYPTION_KEY_V<n>` chain serves `users` + `youtube_oauth_tokens` + `overlay_tts_configs` (and the new `kick_oauth_tokens` / `tiktok_oauth_tokens` encrypted columns). Migration path: when YouTube tokens are first touched after Phase 14 ships, they're decrypted with `YOUTUBE_TOKEN_ENCRYPTION_KEY` (treated as kid-less legacy of the YouTube chain) and re-encrypted under the unified chain. Sweeper finishes the long tail.
- **D-05:** **Backwards compat for legacy kid-less ciphertext is implicit-v0.** On decrypt, if the first byte is not a registered kid (or the blob is shorter than `1+nonce+tag`), treat the whole blob as legacy ciphertext and decrypt with the legacy `TOKEN_ENCRYPTION_KEY` env var (current value). After the sweeper completes, the legacy env var is dropped. *No upfront migration required* — the rollout doesn't have to rewrite the database before going live.
- **D-06:** Sweeper is its own lightweight process (likely `cmd/key-rotator` or similar in auth-service or a new tool). Runs to completion as a Job on demand or scheduled CronJob. Idempotent. Logs progress per table.

### JWT signing rotation strategy

- **D-07:** Add `kid` header to all User, Viewer, and Service JWTs issued by `shared/auth`. auth-service signs with the current kid.
- **D-08:** Validators (api-gateway, share-service, overlay-manager) accept a list of kids loaded from env: `JWT_SECRET_V1`, `JWT_SECRET_V2`, ... Use `golang-jwt/v5`'s `KeyFunc` to select the secret per kid. Unknown/missing kid → fall back to the legacy `JWT_SECRET` env var (same backcompat shape as TOKEN encryption).
- **D-09:** Rotation timeline: `T+0` add `JWT_SECRET_V<new>`, both issuer and validators see it; `T+0` issuer flips to signing with new kid; `T+max(token_TTL)` (24h+ for User JWTs) drop the old kid from validator's accept list.
- **D-10:** **Service JWTs use the same code path but a separate key chain.** `SERVICE_JWT_SECRET_V<n>` is independent from `JWT_SECRET_V<n>`. One mechanism in `shared/auth`, two key chains — preserves blast-radius isolation.
- **D-11:** **TTS overlay JWTs are NOT in scope for Phase 14 rotation.** They're signed with the per-overlay `tts_signing_secret` (Phase 13 design, not the global `JWT_SECRET`), and rotation is already designed (regenerate the DB column → user copies a fresh OBS link). Phase 14 does not touch this path.
- **D-12:** **No JWT denylist in Phase 14.** Rotate + wait `max(token_TTL)` is the sole revocation mechanism. Worst-case attacker validity window after a rotate = current User JWT TTL (24h). Acceptable. Denylist deferred (see deferred_ideas).

### DB password rotation mechanism

- **D-13:** **Preferred path: CNPG-managed rotation via operator** — define the app user (`allchat_user`) as a `ManagedRoles` entry in the `Cluster` spec referencing a `passwordSecret`; rotation = update the secret + let CNPG reconcile.
- **D-14:** **Conditional fallback:** if research-phase finds CNPG `ManagedRoles` doesn't cleanly support app-user rotation with a dual-password window (so pods don't drop mid-rotation), fall back to a manual-but-safe runbook + helper script that:
  1. Creates a new app user with the new password (so old + new are valid simultaneously).
  2. Updates `allchat-secrets:database-password` via `kubectl patch` (NOT SOPS edit — sidesteps the drift hazard).
  3. Triggers a rolling restart of all services that mount the secret.
  4. Drops the old user once all pods are confirmed on the new credentials.
- **D-15:** Either way: the K8s Secret update path uses `kubectl patch` — **NOT** `sops set` — to avoid the live-vs-SOPS drift pruning risk (see canonical_refs and the `project_secrets_drift` memory). Document explicitly that `database-password` is in the "manually-managed, drift accepted" bucket.

### Encryption gap-fill (Kick + TikTok plaintext OAuth tokens)

- **D-16:** Migrations to add encrypted columns or encrypt-in-place existing columns of `kick_oauth_tokens.access_token/refresh_token` and `tiktok_oauth_tokens.access_token/refresh_token`. Use the new versioned `[v||nonce||ct||tag]` format from day one (no legacy ciphertext to worry about for these tables — they're being encrypted for the first time).
- **D-17:** Update kick-listener and tiktok-listener (and whichever services write these rows) to use `shared/encryption` on the write path. Decrypt on the read path.

### Rotation timing — when do the leaked keys actually rotate?

- **D-18:** **Build the mechanism first, rotate using it.** Phase 14 ships the versioned-key infrastructure with both the legacy and the new keys already loaded. The first rotation IS the new tooling's first run — silent for users (kid+multi-key absorbs JWT changes), no big-bang re-encrypt (lazy + sweeper absorbs ciphertext changes), no service restart for the encryption key (env var swap on next deploy).
- **D-19:** Acceptance: leaked keys remain valid for the duration of Phase 14 work. Mitigation is execution speed, not architecture. (No interim rate-limiting / anomaly detection added — out of scope for Phase 14.)

### Phase shape

- **D-20:** **All four workstreams ship in Phase 14** as a single coherent phase. Likely 5-7 plans across 2-3 waves: shared-encryption versioning + sweeper, JWT kid plumbing, DB password rotation, and gap-fill.

### Claude's Discretion

- Exact Go API surface for the versioned encryption package (function names, package layout, whether `shared/encryption` and `shared/crypto` get unified).
- Sweeper batch size, throttling, telemetry shape.
- Migration numbering and exact SQL for the gap-fill encryptions.
- Test strategy details (table-driven decrypt-old/decrypt-new, golden ciphertexts, etc.).
- Whether the sweeper lives as a new top-level service, a `cmd/` under `auth-service`, or a `shared/cmd/key-rotator`.
- JWKS endpoint shape — explicitly NOT building one in Phase 14, but if the planner sees a clean way to make the validator key-loading shape JWKS-ready (e.g., an interface that today reads env vars but could later read HTTP), accept that.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Encryption primitives (existing, will be extended)
- `shared/encryption/encryption.go` — current AES-GCM impl; no kid; opaque `base64(nonce || ct || tag)` format. The package to add versioning to.
- `shared/crypto/crypto.go` — second AES-GCM implementation. Researcher must determine whether this duplicates `shared/encryption` and whether they should be unified or kept separate.

### Encryption usage (call sites that must be updated)
- `services/auth-service/cmd/token-encryption-backfill/main.go` — the existing backfill tool; the *new* sweeper has the same shape but for re-encrypting under a new kid.
- `services/auth-service/handlers/auth_handler.go` — Twitch OAuth token write/read.
- `services/auth-service/handlers/viewer_auth.go` — Viewer JWT issuance.
- `services/youtube-listener/oauth/store.go` — YouTube OAuth tokens (uses `YOUTUBE_TOKEN_ENCRYPTION_KEY`; must migrate to unified chain).
- `services/overlay-manager/models/tts_config.go` — ElevenLabs API key encryption (Phase 13 addition).
- `services/token-refresh-service/` — reads/refreshes encrypted tokens.

### JWT (existing, will be extended)
- `shared/auth/jwt.go` — HS256 signing/validation. Add `kid` header + multi-key `KeyFunc`.
- `services/api-gateway/middleware/auth.go` — JWT validation in the gateway.
- `services/share-service/cmd/main.go` — JWT validation in share-service.
- `services/overlay-manager/cmd/main.go` — JWT validation in overlay-manager (for non-TTS endpoints).
- `services/overlay-manager/tts/jwt.go` — TTS-specific JWT (per-overlay signing). **Out of scope for Phase 14** but referenced for boundary clarity.

### Migrations (schema we must extend)
- `migrations/001_initial_schema.sql` — `users.access_token`, `users.refresh_token` (Twitch, encrypted).
- `migrations/004_tiktok_support.sql` — `tiktok_oauth_tokens` (PLAINTEXT — gap-fill target).
- `migrations/005_kick_support.sql` — `kick_oauth_tokens` (PLAINTEXT — gap-fill target).
- `migrations/006_youtube_token_encryption.sql` — `youtube_oauth_tokens` + `encryption_version` flag (the existing kid-less precedent).
- `migrations/049_overlay_tts_configs.sql` — Phase 13: encrypted ElevenLabs key + plaintext per-overlay TTS signing secret.

### K8s deployment (rotation operations target)
- `caesar-deployment/apps/workloads/all-chat/auth-service-deployment.yaml` — example of `secretKeyRef` mounting pattern; every `database-password`/`jwt-secret`/`token-encryption-key` consumer follows this.
- `caesar-deployment/apps/workloads/all-chat/secrets/allchat-secret.enc.yaml` — SOPS source. **DO NOT** modify blind during rotation; live drift means SOPS edits can prune live keys.
- `caesar-deployment/apps/workloads/all-chat/allchat-cluster.yaml` — CNPG `Cluster` spec. The `ManagedRoles` extension lives here.
- `caesar-deployment/apps/workloads/all-chat/allchat-cluster-secret.yaml` — CNPG superuser secret (separate from app-user `database-password`).

### Design history (read for precedent + landmines)
- `docs/migrations/2025-02-auth-token-encryption.md` — original AES-GCM rollout + backfill strategy. The new sweeper is the descendant of this tool; the rollback plan documented here is the model for Phase 14.
- `.planning/phases/13-text-to-speech-tts-for-chat-messages/` — Phase 13 design decisions for per-overlay encryption + TTS JWT rotation. Establishes the "rotation by regeneration" pattern for ephemeral per-tenant secrets.

### Operational hazard (must read before any K8s Secret edit)
- `~/.claude/projects/-home-moersener-Hobby-all-chat/memory/project_secrets_drift.md` — `allchat-secrets` is OutOfSync (31 live keys vs 11 in SOPS source). Any rotation step that updates the K8s Secret MUST use `kubectl patch`, NOT `sops set`, until the drift is reconciled. Verify drift status before each Secret edit.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`shared/encryption` AES-GCM package**: The primitive is correct; we just lack versioning around it. Phase 14 work is mostly *additive* — wrap the existing functions with kid-aware encode/decode.
- **`golang-jwt/jwt/v5` already in use**: `KeyFunc` is the natural extension point for multi-key validation; no library swap needed.
- **`cmd/token-encryption-backfill` precedent**: The backfill tool's shape (read all rows, decrypt, re-encrypt, write back, idempotent) is the pattern for the new sweeper. Different trigger (rotate-on-deadline vs. one-time backfill), same internals.
- **CNPG `Cluster` spec pattern**: Already in use for the cluster-level secret; extending to `ManagedRoles` for app-user is incremental.

### Established Patterns
- **`secretKeyRef` per env var (no `envFrom`)**: Each secret is individually wired into deployments. Adding `TOKEN_ENCRYPTION_KEY_V2` etc. = N+1 env entries per consumer. Mechanical but verbose.
- **Backwards-compat by env-var coexistence**: Phase 13 introduced `TOKEN_ENCRYPTION_KEY` alongside the older plaintext path with the same coexistence shape. Phase 14 follows the same playbook: legacy `TOKEN_ENCRYPTION_KEY` env stays valid until the sweeper finishes.
- **Idempotent backfills**: 2025-02 token-encryption-backfill is decrypt-then-encrypt-aware. The new sweeper inherits this property — running it twice on the same row is safe.
- **`kubectl patch` for live Secret edits when SOPS drift is acknowledged**: Memory `project_secrets_drift.md` documents this pattern. Phase 14's rotation runbook uses it.

### Integration Points
- Every service that imports `shared/encryption` needs to be re-built/re-deployed when the package gains the kid format — no dynamic loading.
- The sweeper needs DB credentials with read+write to all tables containing encrypted columns. Either runs as a Kubernetes Job with the same secret mounts as auth-service, or uses a dedicated read+write user.
- CNPG `Cluster` spec changes are reconciled by ArgoCD — coordination with the SOPS drift hazard matters here too.

### Discovery during research
The investigation surfaced that `shared/crypto/crypto.go` and `shared/encryption/encryption.go` *both exist* with similar AES-GCM implementations. **Researcher must establish whether they're duplicates, parallel-evolved, or intentionally separate**, and decide whether Phase 14 collapses them or extends both. This is a small refactor opportunity worth confirming up front.

</code_context>

<specifics>
## Specific Ideas

- **Kid byte allocation:** Reserve `0x00` for "no kid / legacy" (never used in new ciphertext). Allocate sequentially: `0x01` = unified TOKEN_ENCRYPTION_KEY_V1 (the first post-Phase-14 key). Rolling back into the kid-less interpretation if `blob[0]` is `0x00` is the same as treating it as legacy. Document this clearly.
- **JWT `kid` value:** Use string-typed kid like `"v1"`, `"v2"` rather than integers (matches JWKS conventions and is human-readable in JWT debug tools).
- **Sweeper deployment shape:** Prefer a Kubernetes `Job` triggered manually + a `CronJob` running it weekly during Phase 14 rollout. Avoid making it a long-running service.
- **CNPG fallback signal:** "Cleanly support" for D-13/D-14 means: (a) operator handles dual-password window so pods don't drop mid-rotation, (b) services pick up the new password without manual deployment intervention beyond what `secretKeyRef` already provides, (c) no application code changes required to support rotation. If any of these fail, fall back to runbook+script.

</specifics>

<deferred>
## Deferred Ideas

- **Redis-backed JWT denylist** — for targeted/immediate revocation beyond rotation+TTL-wait. If/when we need to invalidate a specific token before its TTL expires (e.g., compromised admin account), build this. Adds Redis hit per JWT validation.
- **JWKS endpoint + RS256 migration** — if/when distributed asymmetric auth becomes valuable (e.g., third-party services validating our JWTs without sharing a secret). Phase 14's `KeyFunc`-by-kid abstraction keeps this option open.
- **Per-service DB users (least privilege)** — separate DB users per service (allchat_auth, allchat_overlay_mgr, ...) with table-scoped grants. Big refactor, smaller blast radius. Its own milestone.
- **Encrypt OAuth client credentials and bot tokens at rest in DB** — currently they live only in K8s Secret, never in PostgreSQL. If we ever store them in DB, they should use the versioned encryption.
- **Reconcile SOPS-vs-live K8s Secret drift** — 31 live keys vs 11 in SOPS source. NOT Phase 14's design problem but is a *prerequisite* for any rotation step that touches `allchat-secrets`. Worth a dedicated cleanup phase.
- **Interim mitigation during the leak window** (rate limiting, anomaly detection on token usage, audit logging) — out of scope for Phase 14. Rely on execution speed.
- **TTS overlay JWT versioning** — Phase 13's regenerate-the-secret pattern is unchanged. If we ever need finer rotation, build it; the per-overlay-secret design stays.

</deferred>

---

*Phase: 14-secret-rotation-infrastructure*
*Context gathered: 2026-04-27*
