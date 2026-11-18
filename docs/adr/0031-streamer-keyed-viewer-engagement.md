# ADR-0031: Streamer-Keyed Viewer Engagement Participation

**Date**: 2026-07-06
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

ADR-0028/0029/0030 delivered chat-command participation, the points economy and native mirroring; the richer surfaces are the no-install viewer page (`/overlay/{id}/participate`), the streamer control panel and the browser extension. Every viewer-facing participation endpoint was keyed by overlay id — fine for the web page, whose URL *is* the shared figure/overlay link the streamer gave the viewer, so the id is a capability the viewer legitimately holds.

The browser extension cannot use those endpoints. It attaches to a stream by streamer username via `wss://…/ws/chat/{streamer}` specifically because the overlay id is a bearer capability auth-service deliberately withholds from viewers (it is omitted from the public streamer-info response and tagged `json:"-"` in the poll/prediction snapshots). Handing the overlay id to every extension user would let any viewer open `/ws/overlay/{id}`, which activates the streamer's source listeners (e.g. triggers YouTube polling quota) and undoes that boundary. The alternatives were giving up on proper participation or voting by posting a visible chat command — explicitly rejected as the primary UX.

## Decision

Add streamer-keyed siblings of the viewer participation endpoints under `/api/v1/engagement/streamers/:username/…`, served by engagement-service and proxied by the api-gateway (public read in `publicAPI`; the rest in `protectedAPI`, viewer-JWT authed like the existing `viewers/me/*` routes):

- `GET  /streamers/:username/active` — public aggregate `{points_name, poll, prediction}`
- `GET  /streamers/:username/me` — viewer's balance plus their current vote/wager
- `POST /streamers/:username/vote` — `{poll_id, option_idx}`
- `POST /streamers/:username/wager` — `{prediction_id, outcome_idx, amount}`
- `POST /streamers/:username/heartbeat` — watch-time points

Resolution reuses the WebSocket path's query (`repository.PublicOverlayForStreamer`: oldest active, `is_public_for_viewers`, non-banned overlay), so the resolved overlay is the same one the extension already receives frames from. The handlers then call the unchanged `GetActiveDisplayPoll/Prediction`, `GetBalance`, `GetViewerVote/Entry`, `RecordVote`, `Wager` and `AwardPoints` primitives.

**Safety is structural.** `RecordVote` and `Wager` already reject a poll/prediction whose `overlay_id` differs from the overlay passed in (silent not-found, no cross-tenant existence oracle), so a caller who guesses another overlay's poll id cannot vote on it through a username they can resolve. The overlay id exists only inside the request handler and is never serialized to the client. No second implementation of the economy exists, so ADR-0029's integrity guarantees hold verbatim.

Alternatives considered: exposing the overlay id to the extension (breaks the boundary, re-enabling listener activation); the extension voting via chat commands (kept as the platform-agnostic fallback for TikTok/Discord and logged-out viewers, not the extension UX).

## Consequences

- The extension gets a proper, no-chat-spam participation UI using only the username and the viewer JWT; the overlay id never crosses the wire.
- A streamer with multiple overlays resolves to their oldest public one, the same overlay the viewer WebSocket already selects, so the participation surface and the streamed chat always refer to the same economy.
- The chat-command path (ADR-0028) remains the universal, zero-install baseline.
- No new points/tally logic and no new write path: a thin username→overlay adapter in front of the existing primitives.

## Implementation

- `services/engagement-service/handler/streamer.go`, `cmd/main.go` routes, `repository/predictions.go` etc. (resolution via `PublicOverlayForStreamer`).
- api-gateway: `services/api-gateway/cmd/main.go` public + protected registrations.
- Extension companion change lives in the separate browser-extension repo.

## Related

- [ADR-0028](./0028-engagement-chat-command-write-path.md) / [ADR-0029](./0029-viewer-points-economy-and-prediction-payout.md) — the primitives adapted here.
- Numbering is shared with caesar-deployment, where 0021–0022 live.
