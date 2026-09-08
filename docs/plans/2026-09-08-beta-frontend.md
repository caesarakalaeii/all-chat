# Beta frontend at beta.allch.at — required all-chat changes

Date: 2026-09-08

## Deployment side (done)

Branch `feat/beta-frontend` in `caesar-deployment`:

- `apps/workloads/all-chat/frontend-beta-deployment.yaml` — Deployment + Service
  `frontend-beta`, image `ghcr.io/caesarakalaeii/allchat-frontend:beta`, 1
  replica, keel `force`/`poll` (same annotations as prod frontend, so keel
  rolls it on every new `:beta` push). Same cluster-internal
  `NEXT_PUBLIC_API_URL`/`NEXT_PUBLIC_WS_URL` env as prod (SSR only).
- `apps/workloads/all-chat/network-policies.yaml` — `frontend-beta` policy
  (ingress-nginx + monitoring on 3000). Needed because of the namespace
  default-deny and the distinct `app: frontend-beta` label.
- `ingress/allchat-ingress.yaml` — second host rule `beta.allch.at`: `/api/`
  and `/ws/` → `api-gateway:8080`, `/` → `frontend-beta:3000`. TLS entry with
  secret `allchat-beta-tls` (issued by the existing `letsencrypt-prod`
  ClusterIssuer). `cors-allow-origin` extended with `https://beta.allch.at`.
  No `/webhooks/eventsub` on the beta host — EventSub stays on allch.at only.
- `apps/workloads/all-chat/kustomization.yaml` — new resource listed.

Prod is untouched: separate Service/labels, so the prod `frontend` Service
selector and `frontend-pdb` (`app: frontend`) never match beta pods.

## What needs no change in all-chat

- **Image build**: `.github/workflows/build-and-push.yml` builds on any branch
  with `type=ref,event=branch`. Pushing a `beta` branch that touches
  `frontend/**` produces `ghcr.io/caesarakalaeii/allchat-frontend:beta`.
  Operational only: maintain a `beta` branch (rebase/merge from `main` to
  ship to beta, force-push is fine — keel follows the tag).
- **Frontend API/WS base**: browser code already uses
  `window.location.origin` (`frontend/src/lib/api/client.ts:36-45`,
  `frontend/src/lib/api/websocket.ts:55-59`). Served from beta.allch.at with
  the same-origin `/api/` + `/ws/` proxy, it just works.
- **Cookies**: host-only cookie per `docs/pi/specs/2026-06-23-h3-cookie-auth-design.md`.
  Set via the `beta.allch.at/api/` callback response → scoped to beta.allch.at
  automatically. Verify no explicit `Domain=` attribute is ever set.
- **Extension auth postMessage**: `chat/auth-success/page.tsx` targets
  `window.location.origin` — correct per host without changes.

## Required code changes — implemented (2026-09-08)

### 1. OAuth flows return to the originating host

All streamer OAuth (Twitch/YouTube/Kick login, add-source, moderation
re-consent, delegated-mod consent) routes through `PlatformAuthHandlerV2`, so
that is the only integration point; the legacy `auth_handler.go` callbacks are
no longer routed.

- `services/auth-service/oauth/state.go` — `OAuthState.Origin` records the
  allowlisted origin the flow started from. Tampering is impossible: the
  callback byte-compares the query state against the Redis copy, and
  `stateOrigin()` re-checks the allowlist before use.
- `services/auth-service/oauth/{twitch,youtube,kick}.go` — `WithRedirectURL()`
  returns a copy with a swapped `redirect_uri`, so the authorize URL and the
  code exchange agree with the origin-specific callback registered at the
  provider.
- `services/auth-service/handlers/frontend_origin.go` — allowlist from
  `FRONTEND_URL` (canonical fallback) + `FRONTEND_URLS` (comma-separated);
  `requestFrontendOrigin()` resolves the starting origin from
  `X-Forwarded-Host` (the api-gateway rebuilds the request, so the original
  `Host` never arrives); `providerForOrigin()` swaps the provider.
- `services/api-gateway/handlers/proxy.go` — stamps `X-Forwarded-Host` with
  the original `Host` after `copyHeaders`, so a client-supplied value can
  never override it.
- Not a blocker after all: `token-refresh-service` only refreshes tokens —
  no `redirect_uri` is involved — and needs no change.
- Known gap, deliberately out of scope: the Discord account-link/bot-invite
  callback (`handlers/discord.go:297`) still redirects to the canonical
  FRONTEND_URL. A beta user linking Discord lands on allch.at/settings.
- payment-service / share / device-link stay canonical per the "no beta
  overlay links" decision.

### 2. WebSocket first-party origins

`loadFirstPartyWSOrigins()` (`services/api-gateway/handlers/websocket.go`)
now reads `FRONTEND_URL` + `FRONTEND_URLS`; beta is a first-party origin for
the cookie-authenticated owner socket (audit #8 semantics unchanged: exact
match, no wildcards).

### Deployment wiring (caesar, branch `feat/beta-frontend`)

- `allchat-config`: `FRONTEND_URLS=https://allch.at,https://beta.allch.at`;
  `CORS_ORIGIN` and `WEBSOCKET_ALLOWED_ORIGINS` extended with
  `https://beta.allch.at`.
- `auth-service-deployment.yaml`: explicit `FRONTEND_URLS` configMapKeyRef
  (auth-service uses selective key refs; api-gateway gets it via `envFrom`).

### Tests

- `services/auth-service/handlers/frontend_origin_test.go` — origin
  resolution (allowlist, spoofed/multi `X-Forwarded-Host`, no-allowlist
  fallback), `stateOrigin` trust rules, `WithRedirectURL` swap, and two
  end-to-end `HandleLogin` tests (beta origin recorded in state +
  `redirect_uri`; unknown host stays canonical).
- `services/api-gateway/handlers/websocket_firstparty_test.go` —
  `loadFirstPartyWSOrigins` env variants.
- `services/api-gateway/handlers/proxy_test.go` — original host forwarded,
  client header cannot override.
- `go build ./...` + `go test ./handlers/... ./oauth/...` green for
  auth-service and api-gateway (2026-09-08).

## Decisions (2026-09-08) — implemented

- **Analytics: beta and prod are separated.** `frontend/src/components/Analytics.tsx`
  now picks the Umami website ID by `window.location.hostname`
  (`WEBSITE_IDS`: prod ID for `allch.at`, a dedicated ID for `beta.allch.at`).
  Open operator step: create a second website in the Umami instance and set
  `BETA_WEBSITE_ID` — until then beta is untracked, which already guarantees
  no beta data lands in prod stats. `lib/analytics.ts` `trackEvent` already
  no-ops when the tracker is absent, so nothing else needs guarding.
- **Beta is not indexed.** `frontend/src/app/robots.ts` is now host-aware
  (dynamic via `headers()`): on `beta.allch.at` it returns
  `Disallow: /` for every agent. The canonical/metadataBase URLs in
  `layout.tsx`/`sitemap.ts` stay on allch.at, so what little beta does
  expose points crawlers at prod.
- **No beta overlay links (yet).** Server-generated links (share URLs, OBS
  overlay URLs, payment/Patreon return URLs) keep pointing at allch.at via
  `FRONTEND_URL` — no change. If beta-origin links are ever wanted, they
  fall out of change 1's origin plumbing for free.

## Verification after rollout

1. `kubectl -n allchat get pods -l app=frontend-beta` ready; cert
   `allchat-beta-tls` issued (`kubectl -n allchat get certificate`).
2. `https://beta.allch.at` serves the beta build; `/api/health` proxied.
3. Full login on beta.allch.at per provider: callback lands on
   beta.allch.at, post-login redirect stays on beta.allch.at, cookie is set
   host-only for beta.allch.at.
4. Owner dashboard WebSocket connects from beta (cookie auth path).
5. `https://beta.allch.at/robots.txt` returns `Disallow: /`;
   `https://allch.at/robots.txt` unchanged.
6. Beta page view sends no Umami event until `BETA_WEBSITE_ID` is set; after
   it is set, events land in the beta website, not the prod one.
7. Prod regression: login + WS on allch.at unchanged.
