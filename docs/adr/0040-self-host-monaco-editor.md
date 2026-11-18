# ADR-0040: Self-Host the Monaco CSS Editor (No Third-Party CDN)

**Date**: 2026-07-19
**Status**: Accepted
**Revised**: 2026-11-18 (trimmed; record the current CSP)
**Deciders**: caesarakalaeii

## Context

The overlay editor's Custom CSS field (`frontend/src/components/MonacoCSSEditor.tsx`) is built on `@monaco-editor/react`, whose loader default fetches the Monaco engine from a public CDN (`cdn.jsdelivr.net`). The application CSP (`frontend/next.config.js`) deliberately enumerates every allowed script host and did not include jsdelivr, so the browser blocked the loader script and the editor hung forever on "Loading editor…" (reproduced in production via Playwright; console shows the CSP violation and "Monaco initialization: error").

## Decision

**Self-host Monaco from our own origin.** Alternative rejected: adding `cdn.jsdelivr.net` to `script-src`, which creates a permanent runtime dependency on a third-party CDN (outage, corporate/geo block, offline dev), widens the script-execution surface against the tightly enumerated CSP, and still does not fix Monaco's workers (see below).

- `frontend/scripts/copy-monaco.mjs` copies `node_modules/monaco-editor/min/vs` to `frontend/public/monaco/vs` at build time. `monaco-editor` is a peer dependency of `@monaco-editor/react` (pinned in the lockfile), so `npm ci` supplies it; the output is gitignored (16 MB) and regenerated via the `prebuild`/`predev` hooks, with a `.monaco-version` marker skipping the copy when current.
- `frontend/src/lib/monaco.ts` calls `loader.config({ paths: { vs: '/monaco/vs' } })` and is imported for its side effect at the top of `MonacoCSSEditor.tsx`.
- **CSP: add `worker-src 'self' blob:`.** Monaco runs its CSS language services in Web Workers instantiated from same-origin `blob:` URLs regardless of where the engine is hosted. Without `worker-src`, worker creation falls back to `script-src` (no `blob:`) and the editor would render but its language features would silently die. `'self'` covers direct same-origin workers; `blob:` covers Monaco's blob workers (built by same-origin script — far narrower than trusting an external script host). **No `script-src` change**: the CDN is never added.

## Consequences

- The editor loads and its validation/autocomplete work under the production CSP (verified end-to-end with Playwright: mounts, renders, and a planted typo produces a validation marker). No third-party CDN in the runtime path: works offline, during a CDN outage, in restricted networks.
- The Docker runner stage already copies `public/` into the standalone image, so no Dockerfile change was needed.
- Upgrading `monaco-editor` auto-refreshes the vendored copy on the next build (the copy script keys off the installed version).
- The only CSP broadening is `worker-src 'self' blob:`, applied globally; overlay/widget routes create no workers.
- `script-src` has since shrunk to `'self' 'unsafe-inline' https://analytics.allch.at`: `embed.twitch.tv` was dropped and the credits clip embed runs as an iframe under `frame-src https://clips.twitch.tv`. The CSP stays first-party.
