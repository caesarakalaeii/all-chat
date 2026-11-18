# ADR-0013: Public Overlay Observability View + Shared `useOverlayStream` Hook

**Date**: 2026-05-31
**Status**: Accepted
**Revised**: 2026-11-03
**Deciders**: caesarakalaeii

## Context

The only way to watch an overlay's chat was the OBS render route (`/overlay/[id]`): transparent, theme-styled, auto-fading, tuned for broadcast compositing rather than reading. Streamers asked for a second-monitor dashboard for monitoring chat and events. The requirement: a second surface that reuses the realtime pipeline (reconnect/backoff, `?since=` replay watermark, id dedup, deletion handling, `message_update` aggregation, `platform_status` gating) without duplicating it or coupling the two UIs.

Alternatives rejected: a standalone view with its own copy of the connection logic (the copies drift); an authenticated owner-only dashboard route (the data is already public via the public config and WebSocket endpoints, so auth adds only friction).

## Decision

Extract the connection logic into a shared `useOverlayStream` hook (callback-based: `onChat`/`onMessageUpdate`/`onDeletion`), refactor the overlay page to consume it behavior-preserving, and build a public route `/overlay/[id]/view` on top. Public by UUID, like the OBS overlay: it exposes nothing beyond what `/overlays/public/:id/config` and `/ws/overlay/:id` already serve.

The two pages share the socket but keep separate display policies: the overlay keeps its filter-to-sound-TTS-fade pipeline; the view has scrollback, no fade, no theme CSS, its own light/dark mode, shows filtered messages, keeps moderated messages visible (struck-through and tagged), and logs deletions in an activity feed. The view never imports overlay themes (`events.css`, `visual_settings`, `custom_css`). A "configured events" panel uses the public event-settings route (`GET /api/v1/overlays/public/:id/event-settings`) and degrades to observed event types if it is not available.

## Implementation

- Hook and pipeline: `frontend/src/hooks/useOverlayStream.ts`, `frontend/src/core/overlayStreamCore.ts`, `frontend/src/lib/utils/overlayViewModel.ts`.
- View: `frontend/src/app/overlay/[id]/view/`, components under `frontend/src/components/overlay/` (`ChatPanel`, `ActivityPanel`, `ObservabilitySummary`, `ConnectionBadge`, `OverlayViewThemeToggle`, ...), split pane in `frontend/src/components/ResizableSplit.tsx`.
- Backend: the gateway exposes overlay-manager's public event-settings at `/api/v1/overlays/public/:id/event-settings`.
- Tests: pure-helper, hook (mock WebSocket), view unit tests, plus `frontend/tests/e2e/overlay-view.spec.ts`.

Related: ADR-0002 (the pipeline), ADR-0005 (frontend conventions).
