# ADR-0071: The Alerts Editor — Premium Saving, Sandboxed Custom Code Under the Page CSP, and Client-Side KLIPY Search

**Date**: 2026-10-16
**Status**: Accepted (key delivery superseded by ADR-0073)
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

ADR-0070 gave alerts overlays stored profiles (`alert_profiles`) and a
premium-gated `PUT /overlays/:id/alerts/profiles`, but no editor: the dashboard
showed an alerts overlay the chat overlay's appearance sections and a chat
preview that said "Waiting for messages". Issue #954 asks for the editor, for a
premium tier of custom HTML/CSS/JS per alert, for GIF search through KLIPY
(the issue spells it "Klippy"), and for media uploads through media-service.

Four questions needed an answer:

1. **Which tier may edit?** The issue speaks of "free-tier editing". ADR-0070
   and migration 107 make saving profiles premium (`alerts_editor`), with the
   defaults and the test alert free. ADR-0068 says nothing about tiers.
2. **How is streamer JavaScript isolated**, and does it run at all under the
   overlay page's strict CSP? An `iframe srcdoc` inherits its parent's policy.
3. **Where does GIF search run, and with which key?**
4. **Can a browser upload media at all?** It turned out it could not (see
   below).

## Decision Drivers

- Streamer JS is arbitrary code on a public OBS page. The isolation must hold
  even when the code is hostile, and it must not depend on reviewing the code.
- The overlay page's CSP must not be weakened for the feature. A broader
  `script-src` would apply to every overlay, not just the sandbox.
- KLIPY's usage rules require requests and media loads from the user's
  browser, results in the order returned, media URLs used as returned, and a
  "Powered by KLIPY" attribution.
- Accepted decisions stand unless a new record changes them.

## Considered Options

### Tiering

1. **Keep saving premium (chosen).** The defaults are the free preset library;
   test alerts are free. Without premium the editor is read-only, and the
   standard upgrade link sits on it and on the custom-code section.
2. Make preset editing free and gate only the custom-code columns
   server-side. That reverses ADR-0070 and migration 107, which no record
   asked for. The alerts overlay kind is itself premium (`alerts_overlay`,
   migration 108), so a free tier could not create the overlay it would edit.

### Isolation of custom code

a. **`<iframe sandbox="allow-scripts" srcdoc=...>` (chosen).** No
   same-origin token, so the frame runs in a unique opaque origin. It cannot
   reach the overlay page, its storage or cookies, or send credentialed
   same-origin requests. The token list is exactly `allow-scripts`: no forms,
   popups or top navigation. A unit test asserts the attribute string.
b. A sandboxed route on a separate origin (e.g. a `sandbox.` host). It would
   have its own CSP, so external scripts and `eval` could be allowed. But it
   needs a new host, TLS and an ingress, for a capability nobody has asked for
   yet.
c. Run user JS in the page with a JS-level sandbox (e.g. a realm shim). Weaker
   than the browser's origin boundary, and blocked by the CSP anyway.

### GIF search

- **Browser-side calls to `api.klipy.com` with a public app key baked in at
  build time (chosen).** KLIPY's rules forbid proxying requests or caching
  media. The key is public by nature, because it travels in every request URL.
- A backend proxy: forbidden without written approval from KLIPY.

## Decision Outcome

- **Tiering: option 1.** `alerts_editor` keeps gating the PUT. The frontend
  reads `users.is_premium` like the other premium toggles do. A gate flipped
  free needs a public gate probe (the same caveat as `creatableOverlayKinds`).
- **Custom code: option (a).** Migration 109 adds `custom_html`, `custom_css`
  and `custom_js` (TEXT, default `''`, idempotent) to `alert_profiles`.
  `shared/alerts` caps each field at 64 KiB and validates nothing else: the
  sandbox is the isolation (ADR-0068). The gateway forwards the fields in the
  resolved profile. The renderer gives each alert a fresh frame and posts
  `{type: "alert", profile, event}` to it once the frame has loaded (target
  origin `*`, the only one that reaches an opaque origin). An injected
  bootstrap provides `onAlert(cb)`, `allchat.fill(template, event)` and the
  `data-alert-text` / `img[data-alert-image]` bindings. The parent keeps
  timing and sound. The contract is documented in
  `frontend/src/components/overlays/alertSandbox.ts`.
- **CSP: unchanged, verified.** The srcdoc inherits
  `script-src 'self' 'unsafe-inline' …`. Inline scripts (the bootstrap and
  the streamer's code) run. External `<script src>` loads only from the
  hosts that policy lists. `eval`/`new Function` are blocked; there is no
  `'unsafe-eval'`. Images, media and `fetch` to any https host work
  (`img-src`/`media-src`/`connect-src https:`). `frame-src` does not apply to
  `about:srcdoc`. The e2e spec `alerts-editor.spec.ts` runs against a
  production build with the real headers. It asserts that a script in the
  sandbox writes to its own DOM.
  **If `script-src` moves to nonces** (the TODO in `next.config.js`),
  `'unsafe-inline'` is ignored and the sandbox scripts stop running. That
  change must then either pass the nonce into the srcdoc's script tags or
  move to option (b). This record is the reminder.
- **GIF search:** `frontend/src/lib/klipy.ts` calls
  `GET https://api.klipy.com/api/v1/{app_key}/gifs/search` and stores the
  `hd` (fallback `md`) GIF URL as returned. It shows the `sm` rendition in the
  grid, keeps result order, and shows "Powered by KLIPY". The key is
  `NEXT_PUBLIC_KLIPY_API_KEY`, set from the `KLIPY_API_KEY` Docker build arg,
  which CI fills from the repository variable `KLIPY_API_KEY`. Without a key,
  or on any failure, the picker says "search unavailable". Pasting a URL and
  uploading still work. The existing CSP already admits `api.klipy.com`
  (`connect-src https:`) and `static*.klipy.com` (`img-src https:`).
- **Media upload made reachable.** Three defects meant no browser upload could
  ever have succeeded (`media_objects` was empty in production):
  1. the gateway had no `/api/v1/media` route: it now proxies to
     media-service (`MEDIA_SERVICE_URL`, no prefix strip);
  2. media-service presigned PUTs against its in-cluster endpoint
     (`minio.allchat.svc…:9000`). The signature covers the Host header, so a
     browser cannot use such a URL. It now presigns with a second client for
     the origin of `MEDIA_PUBLIC_URL`, and `MINIO_REGION` (default
     `us-east-1`) keeps presigning offline;
  3. MinIO serves path-style, so a public URL needs the bucket segment:
     production's `MEDIA_PUBLIC_URL` must be `https://media.allch.at/allchat-media`
     (a GitOps change).

## Consequences

### Positive

- Streamer code can do anything a web page can do inside its frame, and
  nothing to the overlay page or other overlays.
- No CSP change, no new host, no editor dependency (the editor uses plain
  textareas; the test alert is the preview).
- GIF search costs no backend work and complies with KLIPY's rules by
  construction.
- The alerts editor's preview pane is the real alert box on the overlay
  socket (`/overlays/:id/preview/alerts`, passive), so a test alert previews
  custom code exactly as OBS will render it.

### Negative

- Custom code cannot load scripts from arbitrary CDNs or use `eval`. Inline
  code only.
- Search is unavailable until someone registers a KLIPY key and sets the
  repository variable. The image is rebuilt with the key baked in, so
  rotating the key needs a rebuild.
- Every alert frame carries its profile's custom code (up to 3 × 64 KiB). That
  is acceptable at alert rates.
- A streamer who loses premium keeps their saved custom code rendering. The
  gate protects writes, not reads, as with every other saved premium setting.

## Implementation

- `migrations/109_alert_profiles_custom_code.sql` (+ down); `shared/alerts`
  (`CustomHTML/CSS/JS`, `MaxCustomCodeLen`).
- `services/overlay-manager/repository/alert_profile_repo.go`;
  `services/api-gateway/subscription/repository.go` (`AlertProfile`),
  `models/service_config.go` (media route).
- `services/media-service/storage/storage.go` (public-host presigner,
  `MINIO_REGION`).
- Frontend: `components/overlays/AlertsEditor.tsx`, `alertSandbox.ts`,
  `AlertBox.tsx` (`CustomCodeAlert`), `lib/klipy.ts`, `lib/api/media.ts`,
  `app/overlays/[id]/preview/alerts/page.tsx`, `components/editor/sectionRegistry.ts`
  (`ALERT_OVERLAY_SECTIONS`), editor page wiring.
- Build: `frontend/Dockerfile` (`KLIPY_API_KEY` arg),
  `.github/workflows/build-and-push.yml` (`vars.KLIPY_API_KEY`).

## Related Decisions

- ADR-0068: the alerts platform, media-service, and the accepted risk of
  unrestricted streamer JS.
- ADR-0070: profiles, the gateway resolver, the `alerts_editor` gate.
- ADR-0008: feature gates.
- ADR-0040: the precedent for self-hosting what the strict CSP would
  otherwise block.
