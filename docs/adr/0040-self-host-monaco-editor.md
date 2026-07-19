# ADR-0040: Self-host the Monaco CSS editor (no third-party CDN)

**Date**: 2026-07-19
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The overlay editor's Custom CSS field (`frontend/src/components/MonacoCSSEditor.tsx`,
used from `overlays/[id]` and `overlays/[id]/credits`) is built on
`@monaco-editor/react`. That wrapper delegates engine loading to
`@monaco-editor/loader`, whose **default configuration fetches the Monaco
engine from a public CDN**:

```
paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.55.1/min/vs' }
```

Our application CSP (`frontend/next.config.js`) deliberately enumerates every
allowed script host and does **not** include `cdn.jsdelivr.net`:

```
script-src 'self' 'unsafe-inline' https://embed.twitch.tv https://analytics.allch.at
```

So the browser blocked the loader script and the editor hung **forever on
"Loading editor…"**. Reproduced in production via Playwright — the console
showed:

```
Loading the script 'https://cdn.jsdelivr.net/npm/monaco-editor@0.55.1/min/vs/loader.js'
violates the following Content Security Policy directive:
"script-src 'self' 'unsafe-inline' https://embed.twitch.tv https://analytics.allch.at".
The action has been blocked.
→ Monaco initialization: error
```

Two ways to make the editor load again:

1. **Add `cdn.jsdelivr.net` to `script-src`.** One line, but it (a) permanently
   introduces a runtime dependency on a third-party CDN — a jsdelivr outage or
   a corporate/geo block breaks the editor, and offline dev breaks; (b) widens
   the script-execution surface to a large external origin, against the grain
   of a CSP we otherwise keep tightly enumerated; and (c) does not by itself fix
   Monaco's web workers (see below), so extra CSP relaxations are needed anyway.
2. **Self-host Monaco from our own origin.** Vendor the engine into `public/`
   and point the loader at it, so every request is same-origin `'self'`.

## Decision

**Self-host Monaco (option 2).**

- `frontend/scripts/copy-monaco.mjs` copies `node_modules/monaco-editor/min/vs`
  → `frontend/public/monaco/vs` at build time. `monaco-editor` is a peer
  dependency of `@monaco-editor/react`, auto-installed and pinned in the
  lockfile (0.55.1), so `npm ci` supplies it deterministically without a
  separate direct-dependency entry (kept out to avoid npm-version lockfile
  churn — see [[reference_frontend_npmci_lock_cache_masking]]). The output is
  gitignored (16 MB) and regenerated via the `prebuild`/`predev` npm hooks; a
  `.monaco-version` marker skips the copy when already current.
- `frontend/src/lib/monaco.ts` calls `loader.config({ paths: { vs: '/monaco/vs' } })`
  and is imported for its side effect at the top of `MonacoCSSEditor.tsx`,
  before any `<Editor>` mounts.
- **CSP: add `worker-src 'self' blob:`.** Monaco 0.55 runs its CSS language
  services (validation, autocomplete) in Web Workers that it instantiates from
  **same-origin `blob:` URLs** — this is independent of where the engine is
  hosted. With no explicit `worker-src`, worker creation falls back to
  `script-src` (no `blob:`) and is blocked, so the editor would render but its
  language features silently die. `'self'` covers direct same-origin workers;
  `blob:` covers Monaco's blob workers (blobs are built by same-origin script,
  so this is far narrower than trusting an external script host, and the CSP
  already trusts `blob:` in `media-src`). **No `script-src` change** — the
  third-party CDN is never added.

## Consequences

- The CSS editor loads and its CSS validation/autocomplete work under the exact
  production CSP. Verified end-to-end with Playwright: the real component
  mounts (no "Loading editor…"), Monaco renders, and the CSS worker produces a
  validation marker for a planted typo.
- No third-party CDN in the runtime path: editor works offline, during a
  jsdelivr outage, and in restricted networks; the script-execution surface
  stays first-party.
- Build produces a 16 MB `public/monaco/` (gitignored). The Docker builder runs
  the copy via `prebuild`; the runner stage already copies `public/` into the
  standalone image, so no Dockerfile change is required.
- Upgrading `monaco-editor` (via a `@monaco-editor/react` bump) auto-refreshes
  the vendored copy on the next build because `loader.config` is
  version-agnostic (`/monaco/vs`) and the copy script keys off the installed
  version.
- The only CSP broadening is `worker-src 'self' blob:`, applied globally in
  `cspBase`. Overlay/widget routes create no workers, so this does not
  meaningfully weaken them.
