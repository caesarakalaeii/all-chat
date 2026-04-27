# Phase 14: Secret Rotation Infrastructure - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-27
**Phase:** 14-secret-rotation-infrastructure
**Areas discussed:** TOKEN_ENCRYPTION_KEY rotation, JWT signing rotation, Phase scope + emergency timing, DB password rotation

---

## TOKEN_ENCRYPTION_KEY rotation strategy

### Q1: Core mechanism for rotating the AES-GCM master key

| Option | Description | Selected |
|--------|-------------|----------|
| Versioned keys + kid prefix in ciphertext | Format `[v(1B)][nonce][ct][tag]`. Multiple master keys live concurrently as `TOKEN_ENCRYPTION_KEY_V<n>`. New writes use latest; reads pick by kid. Rotation = add new kid + lazy re-encrypt. | ✓ |
| DEK-KEK envelope (per-row data keys) | Each row gets a random DEK; DEK is wrapped with a master KEK and stored alongside the ciphertext. Rotation = re-wrap all DEKs (cheap). Adds a `wrapped_dek` BYTEA column to every encrypted table. | |
| Big-bang re-encrypt (no versioning) | Same as the original 2025-02 backfill. Maintenance window, decrypt-all + re-encrypt-all + swap key in K8s. | |

**User's choice:** Versioned keys + kid prefix.
**Notes:** Modest code change, no big-bang migration, clean precedent (Fernet, AWS KMS data keys, libsodium SecretBox versioning). Old ciphertext stays valid throughout the rotation window.

### Q2: How do existing rows get re-encrypted from V_old to V_new?

| Option | Description | Selected |
|--------|-------------|----------|
| Lazy on next write | Read with whichever key matches the kid; on next write re-encrypt with latest. No background job. Risk: rarely-touched rows stay on old key indefinitely. | |
| Lazy + scheduled background sweeper | Same as lazy + a sweeper that scans rows on the old kid and re-encrypts in batches. Ensures old key can be retired on a deadline. | ✓ |
| Eager batch re-encrypt only (no lazy) | Run rotation tool, all rows flip in one batch, drop old key. Same as today's backfill model. | |

**User's choice:** Lazy + scheduled background sweeper.
**Notes:** Gives us the "silent rotation" property AND a deadline for retiring the old key.

### Q3: Two separate master keys (TOKEN_ENCRYPTION_KEY + YOUTUBE_TOKEN_ENCRYPTION_KEY) — unify or keep separate?

| Option | Description | Selected |
|--------|-------------|----------|
| Unify into one master key | Drop YOUTUBE_TOKEN_ENCRYPTION_KEY during the rotation. One TOKEN_ENCRYPTION_KEY_V<n> chain across all encrypted columns. | ✓ |
| Keep separate, version each independently | Each key has its own version chain. Preserves historical isolation but doubles op surface. | |
| Unify but keep namespaced kid | Single env var, kid encodes both version AND domain. Worst of both. | |

**User's choice:** Unify into one master key.
**Notes:** Reduces operational surface (one rotation flow instead of two). Original split was defensive; with versioning it adds no security.

### Q4: Handle the legacy kid-less ciphertext already in the database

| Option | Description | Selected |
|--------|-------------|----------|
| Implicit v0: kid-less = legacy key, on-write upgrade | If first byte is not a known kid, treat blob as legacy ciphertext, decrypt with current TOKEN_ENCRYPTION_KEY. Sweeper rewrites lazily. After sweeper completes, drop legacy env var. | ✓ |
| Eager migration: rewrite all rows on rollout | One-shot migration that decrypts every encrypted row + re-encrypts with v1 kid format before new code is deployed. | |
| Hybrid: short backcompat window, then enforce kid | Implicit v0 for 2 weeks, sweeper completes, then code drops v0 path. | |

**User's choice:** Implicit v0 + on-write upgrade.
**Notes:** Lazy upgrade; sweeper retires v0 within N days. No upfront migration, no data rewrite.

---

## JWT signing rotation strategy

### Q1: Core mechanism for rotating JWT signing keys without forced re-login

| Option | Description | Selected |
|--------|-------------|----------|
| kid header + multi-key validation, stay on HS256 | JWT header gains `kid`. Validators accept a list loaded from `JWT_SECRET_V<n>`. golang-jwt/v5 KeyFunc selects per kid. | ✓ |
| Migrate HS256 → RS256 + JWKS endpoint | auth-service signs with private key; validators fetch JWKS. Industry standard but big change surface. | |
| Keep HS256, no kid — rotate via forced re-login | Accept that rotation = revoke all sessions. Simplest possible but bad UX. | |
| kid + RS256 (full rebuild) | Full asymmetric setup with kid. Maximum change surface. | |

**User's choice:** kid header + multi-key, stay on HS256.
**Notes:** Smallest change, minimal moving parts. Rotation timeline (24h user TTL): T+0 add new kid, T+25h drop old kid from validators.

### Q2: How does global JWT_SECRET rotation interact with TTS overlay JWTs?

| Option | Description | Selected |
|--------|-------------|----------|
| TTS tokens are NOT signed with JWT_SECRET — unaffected | TTS JWTs are signed with per-overlay tts_signing_secret (Phase 13 design), NOT the global JWT_SECRET. Phase 14 only touches User/Viewer/Service JWTs. | ✓ |
| Add kid versioning to TTS tokens too | Apply same scheme: TTS JWTs get kid header tied to per-overlay key version. | |
| Add exp to TTS tokens (breaking change) | Add TTL to TTS tokens (e.g., 30d) and a refresh flow. Changes Phase 13's design. | |

**User's choice:** TTS tokens unaffected.
**Notes:** Two completely separate JWT chains, each with its own rotation story already designed.

### Q3: SERVICE_JWT_SECRET (separate env var) — same versioning scheme?

| Option | Description | Selected |
|--------|-------------|----------|
| Same scheme, same kid namespace, but separate key chain | Apply identical kid+multi-key logic. Separate env chain: SERVICE_JWT_SECRET_V1, _V2... | ✓ |
| Unify into one chain (drop SERVICE_JWT_SECRET) | Use JWT_SECRET for both user and service tokens; differentiate by claim audience. | |
| Defer service JWT rotation to a later phase | Service JWTs have shorter TTL and tighter blast radius. | |

**User's choice:** Same scheme, separate key chain.
**Notes:** Compromising service-to-service key shouldn't grant user impersonation. One mechanism in shared/auth, two key chains.

### Q4: Emergency immediate revocation (deny list)?

| Option | Description | Selected |
|--------|-------------|----------|
| Rotate + wait max TTL is enough | User JWT TTL = 24h. Acceptable blast radius. No new infrastructure. | ✓ |
| Add Redis-backed token denylist in Phase 14 | auth-service exposes /admin/revoke-token; validators check Redis on every JWT validation. | |
| Defer denylist; shorten User JWT TTL to 1h | No denylist, but cut blast-radius window from 24h to 1h. | |

**User's choice:** Rotate + wait max TTL.
**Notes:** Denylist deferred — noted as future option if targeted revocation ever needed.

---

## Phase scope + emergency timing

### Q1: What's the scope of Phase 14 — build everything, or split into a chain of phases?

| Option | Description | Selected |
|--------|-------------|----------|
| Phase 14 = all three rotation mechanisms | TOKEN versioning + sweeper, JWT kid+multi-key, DB password rotation tooling all in Phase 14. Likely 5-7 plans across two waves. | ✓ |
| Split: Phase 14 = TOKEN + JWT, Phase 15 = DB + plaintext gap | Keep Phase 14 focused on in-app crypto; DB and gap-fill go to Phase 15. | |
| Split: Phase 14 = TOKEN, Phase 15 = JWT, Phase 16 = DB + gap | One mechanism per phase. Tightest scope, most cycles. | |
| Phase 14 = mechanisms only; gap-fill is Phase 15 | All three mechanisms in 14, but defer Kick/TikTok plaintext encryption. | |

**User's choice:** Phase 14 = all three rotation mechanisms.
**Notes:** Keeps the "rotation infra" phase intellectually unified; the next leak only ever needs one phase to recover.

### Q2: Kick + TikTok OAuth tokens stored plaintext — encrypt as part of Phase 14, or defer?

| Option | Description | Selected |
|--------|-------------|----------|
| Encrypt as part of Phase 14 | Once versioned TOKEN_ENCRYPTION_KEY exists, encrypting two more columns is cheap (same code path). | ✓ |
| Defer to Phase 15 (gap-fill) | Phase 14 already three workstreams. Adding plaintext-encryption is a fourth. | |
| Encrypt only Kick (TikTok deferred) | Kick more actively used; TikTok lower-traffic. | |

**User's choice:** Encrypt as part of Phase 14.
**Notes:** Avoids leaving "we have rotation, but two whole platforms ship plaintext OAuth" as a known gap.

### Q3: When do we actually rotate the leaked keys?

| Option | Description | Selected |
|--------|-------------|----------|
| Build mechanism first, then rotate using it | Phase 14 ships infrastructure with both old + new keys loaded. Rotation = first run of new tooling. Silent for users, no downtime. Trade-off: leaked keys remain valid during Phase 14 work. | ✓ |
| Emergency one-shot rotation NOW; build mechanism after | Maintenance window this week, force-rotate everything via existing scripts, then build the proper mechanism. | |
| Hybrid: emergency rotate cheap ones now, do hard one after | Rotate JWT + DB password now (cheap-ish). Leave TOKEN_ENCRYPTION_KEY on leaked key until versioning ships. | |

**User's choice:** Build mechanism first, then rotate using it.
**Notes:** Leak window acceptable. Mitigation is execution speed, not architecture.

---

## DB password rotation mechanism

### Q1: How should DB password rotation work?

| Option | Description | Selected |
|--------|-------------|----------|
| Manual-but-safe runbook + helper script | Lightweight: documented runbook + cmd/db-password-rotate that uses kubectl patch (NOT SOPS edit) and triggers rolling restarts. | |
| Per-service DB users (architectural change) | Each service gets own DB user with scoped permissions. Smaller blast radius, big refactor. | |
| CNPG-managed rotation via operator | Configure CNPG to rotate the application-user password via ManagedRoles. Less control, more magic. | ✓ |
| Defer DB password rotation entirely | Out of scope for Phase 14. Accept current state. | |

**User's choice:** CNPG-managed rotation via operator.
**Notes:** Needs research validation; falls back to runbook+script if CNPG can't handle app-user rotation cleanly.

### Q2: Contract for the planner around CNPG-managed rotation?

| Option | Description | Selected |
|--------|-------------|----------|
| Researcher proves CNPG ManagedRoles works; if not, fall back to runbook+script | Locked direction: CNPG ManagedRoles preferred. Research verifies feasibility. If it doesn't work, fall back. | ✓ |
| Commit to CNPG-managed; if it doesn't work, deferred | If research finds CNPG can't handle this, drop DB rotation from Phase 14 entirely. | |
| Commit to CNPG; researcher figures out HOW | Trust that CNPG + glue will work. | |

**User's choice:** CNPG ManagedRoles + runbook fallback.
**Notes:** Two-track plan with clear fallback. Research-phase has explicit success criteria for the CNPG path.

---

## Claude's Discretion

- Exact Go API surface for the versioned encryption package (function names, package layout, whether `shared/encryption` and `shared/crypto` get unified).
- Sweeper batch size, throttling, telemetry shape.
- Migration numbering and exact SQL for the gap-fill encryptions.
- Test strategy details (table-driven decrypt-old/decrypt-new, golden ciphertexts, etc.).
- Whether the sweeper lives as a new top-level service, a `cmd/` under `auth-service`, or a `shared/cmd/key-rotator`.
- JWKS-readiness of the validator key-loading interface (don't build the endpoint, but keep the door open).

## Deferred Ideas

- Redis-backed JWT denylist for targeted/immediate revocation.
- JWKS endpoint + RS256 migration.
- Per-service DB users (least-privilege architecture change).
- Encryption-at-rest for OAuth client credentials and bot tokens (currently K8s-Secret-only).
- Reconciliation of SOPS-vs-live K8s Secret drift (31 live vs 11 in source).
- Interim mitigation during leak window (rate limiting, anomaly detection, audit logging).
- TTS overlay JWT versioning (regenerate-secret pattern stays).
