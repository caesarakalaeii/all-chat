# ADR-0067: Desktop plugins move to their own repositories under GPL-3.0-or-later

**Date**: 2026-10-16
**Status**: Accepted
**Deciders**: caesarakalaeii, the sole copyright holder of the plugin code

(ADR numbering is shared with the caesar-deployment repo. If 0067 is already
taken there, renumber this file; the slug `desktop-plugins-in-own-repositories`
is the stable identifier.)

## Context and Problem Statement

ADR-0049 put two desktop plugins in this repository: `streamdeck-plugin/`
(TypeScript, Elgato SDK) and `streamcontroller-plugin/` (Python,
StreamController). Three things went wrong with that location.

- **The StreamController store cannot list a subdirectory.** Each entry in its
  `Plugins.json` is a repository URL and a commit hash, nothing more, so the
  plugin needs a repository of its own before it can be submitted.
- **ADR-0049 made licensing a release gate.** The StreamController plugin is
  loaded into a GPL-3.0 process while All-Chat is AGPL-3.0-or-later. No conflict
  was expected, but nobody had signed it off, and the gate blocked submission.
- **Contributing is expensive.** Someone who wants to fix one key action clones
  every Go service, the frontend and the migrations, and works inside CI and
  conventions built for all of them.

## Decision Drivers

- The plugins must be installable from their stores and from a release page.
- Drift between the two plugins is the risk ADR-0049 names; whatever replaces the
  monorepo must keep checking it.
- A contributor to one plugin should not need the platform.

## Considered Options

1. **Keep both in this repository** and mirror the StreamController plugin into a
   generated read-only repository for the store.
2. **One repository holding both plugins.** Only worth it if they shared code.
   They do not and cannot: StreamController imports Python into its own
   process, the Elgato app runs a Node process. What they share is the parity
   contract in `scripts/check-plugin-parity.py`, not code.
3. **One repository per plugin**, with the parity script staying here.

## Decision

Option 3. `caesarakalaeii/allchat-streamdeck` and
`caesarakalaeii/allchat-streamcontroller`, each split out with its history and
each licensed **GPL-3.0-or-later**: the StreamController host's own licence,
which closes the ADR-0049 gate outright rather than arguing section 13
compatibility. Neither plugin contains All-Chat server code; both talk to it only
over the public HTTP API.

`scripts/check-plugin-parity.py` stays in this repository, because All-Chat owns
the API both plugins implement. It expects the three repositories checked out
side by side under their repository names, and `KEEP IN SYNC` pointers name the
repository first (`allchat-streamdeck/src/allchat/api.ts`). Each plugin
repository runs it on its own PRs; `.github/workflows/plugins.yml` here runs it
when the script changes and weekly. Every one of those jobs checks out a branch
with the same name from the other repositories when one exists and `main`
otherwise, so a change spanning all three (a new button) is checked as a set
before any part of it merges.

## Consequences

- The StreamController plugin can be submitted to its store; releases of both
  plugins come from their own repositories.
- A cross-plugin change is now up to three PRs instead of one. The same-branch
  convention keeps them checkable together, but merging them is a manual,
  ordered step, and the weekly run is what notices when that order was wrong.
- The plugins no longer share this repository's licence. Code moved between
  All-Chat and a plugin from now on crosses a licence boundary: plugin code may
  come into All-Chat (GPL-3.0-or-later code may be combined with AGPL-3.0), but
  All-Chat code does not go into a plugin without its authors relicensing it.
- Relicensing needed the agreement of every copyright holder of the plugin code.
  All of it was written by caesarakalaeii, directly or through a bot account
  they own, who agreed on 2026-10-16.

## Links

- [ADR-0049](./0049-desktop-control-surfaces-via-paired-device-tokens.md): the two plugins, the parity requirement and the licence gate
- [`docs/guides/streamdeck.md`](../guides/streamdeck.md): the streamer-facing guide for both plugins
