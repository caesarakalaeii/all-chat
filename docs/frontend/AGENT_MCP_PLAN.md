# Agent MCP wiring plan (for the omp config agent)

Status: implemented on 2026-10-01 (Playwright MCP only, per the order below;
DevTools MCP stays deferred until debugging gaps bite). This file started as
the handoff for per-machine wiring. The wiring itself now lives in the
committed `.mcp.json` — portable via `${PLAYWRIGHT_CHROMIUM_PATH}` and
`${ALL_CHAT_FRONTEND_DIR}`, which each machine resolves in its session
environment; nothing machine-specific is in git. The local `.gitignore`
entry for `.mcp.json` (a machine may keep secrets there) must be lifted
with `git add -f` for this file, or the repo's own config is invisible to it.

## What to wire and why

The frontend now has a pixel-baseline gate and a verification ritual
(`AGENT_UI_VERIFICATION.md`), but both are batch-mode. The gap that remains
is interactive: an agent implementing UI changes needs to drive the dev
server, click through flows, and read screenshots *during* the edit loop,
not only at handback. Two MCP servers close that.

### 1. Playwright MCP (primary)

- Repo: https://github.com/microsoft/playwright-mcp
- Install (per-machine, in the omp MCP config, not the repo; verified
  working on 2026-10-01, see also the committed `.mcp.json`):

  ```json
  {
    "mcpServers": {
      "playwright": {
        "type": "stdio",
        "command": "nix",
        "args": [
          "develop", "-c", "npx", "-y", "@playwright/mcp@latest",
          "--executable-path", "${PLAYWRIGHT_CHROMIUM_PATH}"
        ],
        "cwd": "${ALL_CHAT_FRONTEND_DIR}"
      }
    }
  }
  ```

  Note: `@playwright/mcp` has no `--cwd` flag; use the stdio `cwd` field
  instead. `${VAR}` placeholders are expanded by omp at discovery time and
  must resolve in the session environment.

- Why: drives the same browser the test suite pins; agent screenshots land
  in the conversation directly (`browser_take_screenshot`), which feeds the
  vision model; deterministic and scriptable for repeatable flows.
- Caveat for this host: NixOS — `npx` must resolve inside the nix dev shell
  (`nix develop -c`, launched from the repo root so the flake is found),
  and the browser must be a nix-provided Chromium via the `--executable-path`
  flag (the pinned downloaded browser cannot run without nix-ld). Point
  `PLAYWRIGHT_CHROMIUM_PATH` at the nix Chromium, e.g. the profile symlink
  `/etc/profiles/per-user/<user>/bin/chromium` (survives nixos-rebuild;
  a `/nix/store` literal goes stale on the next switch).

### 2. Chrome DevTools MCP (secondary, debugging)

- Repo: https://github.com/ChromeDevTools/chrome-devtools-mcp
- Same per-machine wiring shape.
- Why: attaches via CDP with the real DevTools surface — console errors,
  network panel, performance traces. Serves the cases Playwright MCP cannot:
  "why is this page slow", "what request is failing", state exploration of a
  flaky flow. Playwright MCP for repeatable verification; DevTools MCP for
  debugging (see AGENT_UI_VERIFICATION.md §tool choice).

## Order

1. Playwright MCP first — it covers the main agent dev loop.
2. DevTools MCP only when debugging gaps actually bite; it is a complement,
   not a second driver.

## Relationship to the repo

The repo side of this tooling (spec, baselines, workflow, ritual doc) is
already merged on `feat/agentic-ui-tooling`; see
`docs/frontend/AGENT_UI_VERIFICATION.md` for the full loop. This file only
covers harness wiring, which is per-machine by design.
