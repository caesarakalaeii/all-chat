# Security Audit — Residuals

Tracks the items from [SECURITY_AUDIT_REPORT.md](./SECURITY_AUDIT_REPORT.md)
that are not fully closed. Status follows the hardening work that landed on
the `security/audit-hardening` branch (subsequently merged to main). Finding
IDs refer to that report.

## Open

- **L2** — `shared/signing/` is hardened (min secret length, future-timestamp
  replay rejection, query and service name signed, constant-time compare)
  but no prod service imports it; k8s NetworkPolicies remain the
  service-to-service isolation control. Next: wire `Signer.VerifyMiddleware()`
  into one pilot internal endpoint and design secret distribution
  (`secretKeyRef` per service pair) before rolling out.
- **ENG1** — auth-service `/internal/chat/announce` (engagement-service
  round-start posts, ADR-0028), `/internal/chat/bot-send` (chat command
  replies and timers, ADR-0079/0081) and `/internal/twitch/clip` post or clip
  as any streamer whose body the caller names, gated only by the shared
  `SERVICE_JWT_SECRET`, a wider surface than `/chat/send` (which derives the
  streamer from the token). Not externally reachable: `/internal` only
  mounts when `SERVICE_JWT_SECRET` is set, and NetworkPolicy restricts it to
  in-cluster callers. Keep the allow-list tight and consider an
  engagement-only key so a leaked secret cannot mint announces for unrelated
  services; sign these calls once `shared/signing` (L2) rolls out.
- **ENG2** — outbound fetches of user-chosen URLs (`{urlfetch}` in
  engagement-service, Owncast instance URLs) rely on `shared/safehttp` only
  (refuses non-public IPs at dial time, after DNS and on every redirect, no
  proxy; `{urlfetch}` is https/443 only). Cluster egress is otherwise open,
  so a code path that dials a user URL without `safehttp` would escape.
  Next: egress NetworkPolicies for engagement-service and owncast-listener
  that deny private, link-local and CGNAT ranges.

## Accepted, not deferred

- **L13** — OBS-mode unauthenticated WebSocket to any active overlay;
  judged acceptable for OBS.
- **M2** — refresh-token rotation: minimum rotation tracking landed, full
  family revocation is a larger auth redesign.
- **S3** — `gorilla/websocket` pseudo-version: block on an upstream tag.

Earlier advisories from the audit (H3 cookie migration, H2 blacklist check,
H5 token-in-query, H7 Redis auth, M1 code exchange, M17 TTS token) were
closed in the hardening pass; the report records where.
