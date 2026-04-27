# Quick Task State

**Task**: YouTube Stream Selection Strategy (Premium Feature)
**Status**: Complete
**Branch**: feature/youtube-stream-selection-strategy

## Summary
When a YouTube channel has multiple concurrent live streams (e.g. @LofiGirl), premium users can choose how the innertube listener selects which stream to monitor.

## Strategies
- `first_found` (default, free) — current behavior
- `most_viewers` — highest viewer count
- `fewest_viewers` — lowest viewer count
- `title_match` — first stream whose title contains a keyword

## Files Changed

### Backend
- `services/youtube-listener-innertube/innertube/discovery.go` — `LiveStreamCandidate` struct, `collectLiveCandidatesFromBrowse`, `SelectStream`, `extractTitle`, `extractViewerCount`
- `services/youtube-listener-innertube/innertube/discovery_test.go` — Updated test calls for new signature
- `services/youtube-listener-innertube/streams/manager.go` — Pass strategy from source config to discovery
- `services/source-manager/models/source.go` — Added `StreamSelect`, `StreamMatch` fields
- `services/source-manager/registry/repository.go` — Extract `stream_select`, `stream_match` from config JSONB
- `shared/sourcemanager/types.go` — Added `StreamSelect`, `StreamMatch` to shared type
- `services/overlay-manager/handlers/sources.go` — Validate `stream_select` and `stream_match` in PATCH config
- `services/share-service/featuregates/cache.go` — `GateStreamSelection` constant

### Migration
- `migrations/045_stream_selection_gate.sql` — Feature gate row

### Frontend
- `frontend/src/lib/types/overlay.ts` — `StreamSelectionStrategy`, `YouTubeSourceConfig` types
- `frontend/src/lib/api/overlays.ts` — Generic `updateSourceConfig` method
- `frontend/src/app/overlays/[id]/page.tsx` — `StreamSelectionPanel` component, YouTube "Stream selection" button on source cards
