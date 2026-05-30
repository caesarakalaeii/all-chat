# ADR-0013: Public Overlay Observability View + Shared `useOverlayStream` Hook

**Date**: 2026-05-31
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

The only way to watch an overlay's chat was the OBS render route (`/overlay/[id]`): transparent, theme-styled, auto-fading, and tuned for compositing on a broadcast — not for *reading*. Streamers asked for a second-monitor "dashboard" to monitor chat and events with full readability and observability. We needed a second surface that reuses the exact realtime pipeline (so it stays in sync with what hits chat) without duplicating the subtle connection logic, and without coupling the two very different UIs.

## Decision Drivers

- Reuse the battle-tested connection logic (reconnect/backoff, `?since=` replay watermark, id dedup, deletion handling, `message_update` aggregation, `platform_status` gating) — duplicating it is a maintenance hazard.
- Keep the production OBS overlay's behavior byte-for-byte unchanged.
- The view must be readable: scrollback, no fade, no theme CSS, no animations, its own light/dark mode.
- Minimize new surface area and dependencies.

## Considered Options

1. **Standalone view with its own duplicated stream logic**
   - ✅ Pros: zero risk to the overlay page.
   - ❌ Cons: two copies of the subtle reconnect/replay/dedup/watermark logic drift apart.

2. **Extract a shared `useOverlayStream` hook; refactor the overlay to consume it; build the view on top**
   - ✅ Pros: one implementation of the hard part; both surfaces stay in sync; each page keeps its own message-array/display policy via callbacks.
   - ❌ Cons: touches the production overlay page (mitigated: behavior-preserving move + unit/E2E coverage).

3. **Authenticated, owner-only dashboard route**
   - ✅ Pros: more private.
   - ❌ Cons: needs auth plumbing; the same chat/events are already public via the overlay endpoints, so it adds friction without adding protection.

## Decision Outcome

**Chosen**: Option 2, served as a **public** route `/overlay/[id]/view` (Option 1's access model rejected in favor of public-by-UUID, consistent with the OBS overlay).

**Rationale**: The connection logic is the only genuinely shared, high-risk code — extracting it into `useOverlayStream` (callback-based: `onChat`/`onMessageUpdate`/`onDeletion`) lets the overlay keep its filter→sound→TTS→fade policy and the view keep its no-filter scrollback + moderation-marking policy, with one source of truth for the socket. The view is public by UUID because it exposes only data already public through `/overlays/public/:id/config`, the new `/overlays/public/:id/event-settings`, and `/ws/overlay/:id` — no new exposure, no login friction.

## Consequences

### Positive
- Single implementation of reconnect/replay/dedup/watermark/platform-status, unit-tested without a socket.
- Overlay behavior preserved (verified by unit tests + E2E + manual stack run).
- View ignores overlay themes entirely (never imports `events.css` / `visual_settings` / `custom_css`), giving "no fancy animations" and theme isolation for free.
- Observability extras: filtered messages are shown, moderated messages stay visible (struck-through + tagged), and deletions are logged in the activity feed.

### Negative
- The overlay page's bubble/timestamp classes were migrated `gray-*` → `slate-*` (design-system-mandated; near-imperceptible tint shift) to satisfy the ENFORCE-03 lint rules when the file was touched.
- The "configured events" panel needs the public event-settings route enabled at the gateway; it degrades gracefully (observed event types) if absent.

## Implementation

- **Frontend (new)**: `frontend/src/hooks/useOverlayStream.ts`; `frontend/src/lib/utils/overlayStreamCore.ts` + `overlayViewModel.ts` (pure helpers); `frontend/src/app/overlay/[id]/view/{layout,page}.tsx`; `frontend/src/components/ResizableSplit.tsx`; `frontend/src/components/overlay/{ChatPanel,ActivityPanel,ObservabilitySummary,ChatRow,CompactEvent,ConnectionBadge,OverlayViewThemeToggle,PlatformGlyph}.tsx`.
- **Frontend (modified)**: `frontend/src/app/overlay/[id]/page.tsx` (consume the hook, behavior-preserving); `frontend/src/components/PlatformStatusIndicators.tsx` (`variant` prop); `frontend/src/app/globals.css` (`.overlay-view` light/dark tokens); `frontend/src/lib/types/overlay.ts` (`PublicOverlayConfig`, `EventSettings`); `frontend/src/app/overlays/[id]/preview/page.tsx` ("Monitor view" link).
- **Backend**: `services/api-gateway/cmd/main.go` — expose the existing, already-public `HandleGetPublicEventSettings` via `GET /overlays/public/:id/event-settings`.
- **Tests**: pure-helper + hook (mock WebSocket) + view component unit tests; `frontend/tests/e2e/overlay-view.spec.ts` smoke test.
- **Timeline**: 2026-05-31.

## Related Decisions

- [ADR-0002: Redis Streams + Pub/Sub](./0002-redis-streams-pubsub.md) — the pipeline this view consumes.
- [ADR-0005: React + Next.js App Router](./0005-react-nextjs-frontend.md) — the frontend conventions followed.
