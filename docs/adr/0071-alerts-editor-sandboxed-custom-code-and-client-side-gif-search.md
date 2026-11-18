# ADR-0071: The Alerts Editor: Free Editing, Sandboxed Custom Code, and Client-Side KLIPY Search

**Date**: 2026-10-16
**Status**: Accepted
**Revised**: 2026-11-15 (editing, custom code and GIF search are free; only uploads stay premium, via the `media_upload` gate)
**Deciders**: All-Chat Platform Team

## Context

ADR-0070 gave alerts overlays stored profiles and a premium-gated profile PUT but no
editor (the dashboard showed chat appearance sections and a "Waiting for messages"
preview). Four questions: which tier may edit (migration 107 made saving premium); how
streamer JS is isolated under the overlay page's strict CSP (an `iframe srcdoc` inherits
its parent's policy); where GIF search runs and with which key (KLIPY requires
browser-originated requests, order and media URLs as returned, "Powered by KLIPY"
attribution, forbids proxying without written approval); and whether a browser can upload
media at all (it could not).

## Considered options

- Editing tier: keep saving premium (chosen 2026-10-16) versus open the editor and keep
  only uploads premium (chosen 2026-11-15, current). Uploads are premium because they
  consume object storage, behind the separate `media_upload` gate (migration 128) on
  media-service's presign. The middle option, free presets with the custom-code columns
  still gated, reverses migration 107 unasked; and at the time the `alerts_overlay` kind
  itself was premium, so a free tier could not have created the overlay it edits.
- Isolation: `<iframe sandbox="allow-scripts" srcdoc>` (chosen): no same-origin token, so
  the frame runs in a unique opaque origin, unable to reach the overlay page, its storage
  or cookies, or send credentialed same-origin requests; the token list is exactly
  `allow-scripts` and a unit test asserts the string. A separate-origin `sandbox.` route
  could allow external scripts and `eval` but needs a new host, TLS and ingress for an
  unasked-for capability; a JS realm shim is weaker than the browser boundary and CSP-
  blocked anyway.
- GIF search: browser-side calls to `api.klipy.com` with a public app key (chosen); the
  key is public by nature, travelling in every request URL. A backend proxy is forbidden
  by KLIPY without written approval.

## Decision

`alerts_editor` still gates the PUT, now free, so it can be narrowed again without a
deploy. The editor is fully editable; `MediaField` reads `users.is_premium` from the auth
store and disables its upload button next to the upgrade link without premium (the server
answers 403 either way). The frontend does not read gate state; flipping a gate back to
premium needs a frontend change or a public gate probe (same caveat as
`creatableOverlayKinds`).

Migration 109 adds `custom_html`, `custom_css`, `custom_js` (TEXT, default '', idempotent)
to `alert_profiles`; `shared/alerts` caps each at 64 KiB and validates nothing else, the
sandbox is the isolation. The renderer gives each alert a fresh frame and posts
`{type, profile, event}` once loaded (target origin `*`, the only one reaching an opaque
origin); an injected bootstrap provides `onAlert(cb)`, `allchat.fill(template, event)` and
the `data-alert-text`/`img[data-alert-image]` bindings; the parent keeps timing and
sound. Contract documented in `frontend/src/components/overlays/alertSandbox.ts`.

CSP unchanged and verified: the srcdoc inherits `script-src 'self' 'unsafe-inline' ...`;
inline scripts run; external `<script src>` loads only from policy hosts; `eval` is
blocked; https images, media and fetch work (`frame-src` does not apply to
`about:srcdoc`). The e2e spec `alerts-editor.spec.ts` asserts the sandbox attribute and
in-sandbox DOM writes. If `script-src` moves to nonces (TODO in `next.config.js`),
`'unsafe-inline'` stops applying and the sandbox scripts die: that change must thread the
nonce into the srcdoc or adopt a separate-origin sandbox.

`frontend/src/lib/klipy.ts` calls `GET https://api.klipy.com/api/v1/{app_key}/gifs/search`,
stores the `hd` (fallback `md`) URL as returned, shows `sm` renditions in the grid, keeps
result order and shows "Powered by KLIPY". The key comes at runtime from the frontend
`GET /klipy-config` (ADR-0073/0074); without it or on failure the picker says "search
unavailable"; pasting a URL always works. The CSP already admits `api.klipy.com` and the
media hosts.

Uploads were made reachable by fixing three defects: the gateway had no `/api/v1/media`
route (proxies to media-service, no prefix strip); media-service presigned against its
in-cluster MinIO endpoint whose Host signature a browser cannot satisfy (now presigns for
`MEDIA_PUBLIC_URL`'s origin, `MINIO_REGION` keeps presigning offline); and MinIO's
path-style serving means `MEDIA_PUBLIC_URL` is `https://media.allch.at/allchat-media`.

## Consequences

- Streamer code can do anything a web page can inside its frame and nothing to the
  overlay page or other overlays; inline only, no external CDNs, no `eval`.
- No CSP change, no new host, no editor dependency (plain textareas; the test alert is
  the preview, on the real overlay socket `/overlays/:id/preview/alerts`, passive).
- GIF search costs no backend work and complies with KLIPY's rules by construction; it is
  unavailable while `KLIPY_API_KEY` is unset (rotation: secret change plus pod restart,
  no rebuild, ADR-0073).
- Every alert frame carries up to 3 x 64 KiB of custom code, acceptable at alert rates. A
  streamer who loses premium keeps existing uploads playing; `media_upload` protects new
  uploads only, and list/delete stay open for cleanup.
