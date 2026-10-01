# Agent MCP wiring plan (for the omp config agent)

Status: proposal, hand-written by the tooling work on `feat/agentic-ui-tooling`.
This file is the handoff: the repo cannot carry harness configuration
(`.mcp.json` is gitignored per-machine; the omp harness is not configured
from this repo), so the config changes below are applied on the machine by
whoever owns omp config. Nothing here is a repo change.

## What to wire and why

The frontend now has a pixel-baseline gate and a verification ritual
(`AGENT_UI_VERIFICATION.md`), but both are batch-mode. The gap that remains
is interactive: an agent implementing UI changes needs to drive the dev
server, click through flows, and read screenshots *during* the edit loop,
not only at handback. Two MCP servers close that.

### 1. Playwright MCP (primary)

- Repo: https://github.com/microsoft/playwright-mcp
- Install (per-machine, in the omp MCP config, not the repo):

  ```json
  {
    "mcpServers": {
      "playwright": {
        "command": "npx",
        "args": ["@playwright/mcp@latest", "--cwd", "<repo>/frontend"]
      }
    }
  }
  ```

- Why: drives the same browser the test suite pins; agent screenshots land
  in the conversation directly (`browser_take_screenshot`), which feeds the
  vision model; deterministic and scriptable for repeatable flows.
- Caveat for this host: NixOS — `npx` must resolve inside the nix dev shell
  (`nix develop -c`), and the browser must be a nix-provided Chromium via
  the `--executable-path` flag (the pinned downloaded browser cannot run
  without nix-ld). Suggested args addition:

  ```
  "--executable-path", "/nix/store/wjs8nzn73dd8kqkg1sl06071syl2hxxh-chromium-150.0.7871.186/bin/chromium"
  ```

  (Resolve the current nix chromium at config time; that store path is the
  one verified working on this machine on 2026-10-01.)

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
