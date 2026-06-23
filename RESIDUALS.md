# Security Audit — Residuals & Deferrals

Tracks items from `SECURITY_AUDIT_REPORT.md` that are **not yet fully closed**.
Status reflects the `security/audit-hardening` branch state as of the audit pass.

Legend: ✅ done · 🔶 minimum done / partial · ⏳ deferred · 🟡 done in working tree (pending commit)

---

## Genuinely deferred

| ID | Finding | Status | Rationale / Scope | Suggested next step |
|----|---------|--------|--------------------|--------------------|
| **H3** | JWT + refresh token in `localStorage` → XSS = account takeover | 🔶 | **Minimum applied:** refresh token moved out of `localStorage` to in-memory store (`frontend/src/lib/auth/in-memory-store.ts`). **Full fix deferred:** migrate JWT + refresh to `httpOnly; Secure; SameSite=Strict` cookies issued by auth-service, drop manual `Authorization` header from all frontend fetch wrappers. This is a multi-week migration touching token issuance (auth-service), every frontend fetch wrapper, CSRF surface (cookie → needs CSRF token), and viewer/extension WS auth flows. | Scope a cookie-auth migration plan; add CSRF token middleware; coordinate viewer-OAuth popup flow. |
| **L2** | `shared/signing/` implemented but not wired into any prod service | ⏳ | Package is hardened (min secret length, future-timestamp replay rejection, query + service-name signed, constant-time compare) but `grep shared/signing services/ --glob '*.go'` = **0 prod imports** (tests only). Documented as deferral — wiring requires shared secret distribution + service-identity contract rollout across all inter-service calls. Current isolation = k8s NetworkPolicies (default-deny added in this pass). | Wire `Signer.VerifyMiddleware()` into one pilot internal endpoint; design key distribution (secretKeyRef per service pair); then roll out. CLAUDE.md updated to state "implemented-not-deployed". |

## Completed in this hardening pass (working tree, pending commit)

These were listed as deferred in the initial commit message but the parallel
residual pass landed them. Listed here for traceability.

| ID | Finding | Status | Note |
|----|---------|--------|------|
| **H2** | Logout blacklist never checked | 🟡 | `JWTAuthWithRevocation(kc, rdb)` now used in all 5 services (api-gateway was done in initial commit; share/moderation/payment/overlay-manager wired in residual pass). |
| **H5** | JWT in WS URL query → access logs | 🟡 | Gateway now reads token via `Sec-WebSocket-Protocol` (`extractWSAuthToken`) with `?n=` query fallback for rollout; logging middleware redacts token. Frontend WS clients updated. |
| **H7** | Redis unauthenticated | 🟡 | k8s manifests committed (NetworkPolicy default-deny + Redis secret). Go clients across 16 services now read `REDIS_PASSWORD`; activation requires setting the secret in-cluster. |
| **M1** | Refresh token in URL fragment (streamer login) | 🟡 | Streamer OAuth redirect now uses short-lived `?code=` exchanged via Redis (mirrors viewer flow), not `#access_token=&refresh_token=`. |
| **M17** | TTS token + spoken text in URL query | 🟡 | Frontend sends `Authorization: Bearer` + JSON body; backend `HandleTTS` reads header with `?tts_token=` query fallback for backward compat. |

## Out of scope per audit (acknowledged, not deferred)

- **L2 (alt)** — wiring signing into a prod service is a deliberate rollout, not a quick fix; tracked above as ⏳.
- **L13** — OBS-mode unauth WS to any active overlay; audit marked "acceptable for OBS".
- **M2** — refresh-token rotation / reuse detection (family tracking); minimum rotation-tracking landed, full family revocation is a larger auth redesign.
- **S3** — `gorilla/websocket` pseudo-version pinning; blocked on upstream tag release (no tag exists yet).

---

*Cross-reference: `SECURITY_AUDIT_REPORT.md` finding IDs. Initial hardening commit: `🔒 fix(security): harden auth/crypto/gateway/infra/frontend per audit` on branch `security/audit-hardening`.*
