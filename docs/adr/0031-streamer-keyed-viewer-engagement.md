# ADR-0031: Streamer-Keyed Viewer Engagement Participation

**Date**: 2026-07-06
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Issue #523 gives All-Chat its own polls, predictions, and viewer points economy: a
chat-command write-path (ADR-0028), a per-overlay points economy (ADR-0029), and
Twitch-native mirroring (ADR-0030). PR #524 shipped the richer, non-chat participation
surfaces — the no-install viewer page and the streamer control panel — plus the browser
extension as an intended *"Viewer participation (web page / extension)"* surface.

Every viewer-facing participation endpoint is keyed by **overlay id**
(`/api/v1/engagement/overlays/:id/polls/:pollId/vote`, `…/viewers/me/engagement?overlay_id=…`).
That works for the web page, whose URL *is* `/overlay/{id}/participate` — the streamer
shared that link, so the overlay id is a capability the viewer legitimately holds.

The **browser extension cannot use those endpoints.** It attaches to a stream by *streamer
username* via `wss://…/ws/chat/{streamer}` specifically so it never learns the overlay id:
the overlay id is a bearer capability that auth-service **deliberately withholds** from
viewers (`auth-service/handlers/streamer_info.go`: overlay_id is omitted from the public
streamer-info response, and the poll/prediction snapshots tag it `json:"-"`). Handing the
overlay id to every extension user would let any viewer open `/ws/overlay/{id}` — which
activates the streamer's source listeners (e.g. triggers YouTube polling quota) — and would
undo that boundary. So the extension had only two options: give up on proper (non-chat)
participation, or vote by *posting a chat command* (`!vote 2`) as the ingested viewer — a
visible, awkward side channel we explicitly rejected as the primary UX.

We want the extension (and any future username-scoped surface) to vote, wager, read its
balance, and heartbeat **without ever seeing an overlay id**, and without weakening the
existing boundary or opening a new write path.

## Decision Drivers

- Preserve the "viewers never learn the overlay id" boundary end-to-end.
- Reuse the exact overlay-keyed vote/wager/balance primitives — no second implementation of
  the points economy, no new tally path, no divergence from ADR-0029's integrity guarantees.
- Reuse the streamer→overlay resolution the viewer WebSocket already performs
  (`GetPublicOverlayByUsername`), so a username maps to the *same* overlay the extension is
  already streaming chat from.
- No new authority: possessing a username must grant nothing beyond what a viewer already
  has (read the public round; vote/wager on the *resolved* overlay only).

## Considered Options

1. **Expose the overlay id to the extension** (new endpoint or in the snapshot), then reuse
   the existing overlay-keyed endpoints.
   - ✅ Zero new backend endpoints.
   - ❌ Breaks the deliberate boundary: the id leaks to every viewer, re-enabling
     `/ws/overlay/{id}` (listener activation / quota) and future misuse. Rejected.
2. **Extension votes by sending a chat command** (`!vote N`) through the existing chat-send.
   - ✅ Zero backend change; rides ADR-0028's universal path.
   - ❌ Posts a visible chat message per vote; no balance/"your vote" read; not a "proper" UI.
     Kept as the platform-agnostic fallback (TikTok/Discord, logged-out), not the extension UX.
3. **Streamer-keyed viewer endpoints that resolve username→public overlay server-side and
   reuse the overlay-keyed primitives.** *(chosen)*

## Decision

Add a streamer-keyed sibling of the viewer participation surface under
`/api/v1/engagement/streamers/:username/…`, served by engagement-service and proxied by the
api-gateway (public read in `publicAPI`; the rest in `protectedAPI`, viewer-JWT authed like
the existing `viewers/me/*` routes):

- `GET  /streamers/:username/active`   — public aggregate `{points_name, poll, prediction}`
- `GET  /streamers/:username/me`       — viewer: balance + this viewer's current vote/wager
- `POST /streamers/:username/vote`     — viewer: `{poll_id, option_idx}`
- `POST /streamers/:username/wager`    — viewer: `{prediction_id, outcome_idx, amount}`
- `POST /streamers/:username/heartbeat`— viewer: watch-time points

Resolution reuses the WebSocket path's query
(`repository.PublicOverlayForStreamer` ≡ the api-gateway's `GetPublicOverlayByUsername`:
oldest active, `is_public_for_viewers`, non-banned overlay), so the resolved overlay is the
same one the extension already receives `poll_update`/`prediction_update` frames from. The
handlers then call the **unchanged** `GetActiveDisplayPoll/Prediction`, `GetBalance`,
`GetViewerVote/Entry`, `RecordVote`, `Wager`, and `AwardPoints` primitives.

**Safety is structural, not additive:** `RecordVote` and `Wager` already reject a poll /
prediction whose `overlay_id` ≠ the overlay passed in (silent `false` / `not_found` — no
cross-tenant existence oracle), so a caller who guesses another overlay's `poll_id` still
cannot vote on it through a username they can resolve. The overlay id exists only inside the
request handler; it is never serialized to the client. The economy, payout conservation, and
native-round isolation (ADR-0029/0030) are inherited verbatim because the same primitives run.

## Consequences

- The browser extension gets a proper, no-chat-spam participation UI (vote/wager/balance)
  using only the streamer username and the viewer JWT it already holds; the overlay id never
  crosses the wire. See the companion change in `all-chat-extension`.
- No new points/tally logic and no new write path: this is a thin username→overlay adapter in
  front of the ADR-0028/0029 primitives, so their integrity properties hold unchanged.
- A streamer with multiple overlays resolves to their oldest public one — the same overlay the
  viewer WebSocket already selects — so the participation surface and the chat the extension
  shows always refer to the same economy.
- The chat-command path (ADR-0028) remains the universal, zero-install baseline for platforms
  and viewers the extension/web page can't cover (TikTok/Discord, logged-out).

(ADR numbering is shared with caesar-deployment; ADR-0021/0022 live there, so this file is 0031.)
