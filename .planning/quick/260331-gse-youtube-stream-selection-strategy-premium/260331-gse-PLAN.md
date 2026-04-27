# YouTube Stream Selection Strategy — Premium Feature

## Goal
When a YouTube channel has multiple concurrent live streams, let premium users choose how the innertube listener picks which stream to monitor via configurable selection strategies.

## Selection Strategies
- `first_found` (default, free) — current behavior
- `most_viewers` — parse viewCountText, pick highest
- `fewest_viewers` — same data, pick lowest
- `title_match` — user provides keyword, first stream whose title contains it

## Premium Gate
- One feature gate: `stream_selection` — controls access to non-default strategies
- Free users always get `first_found`

## Tasks

### 1. Backend: InnerTube Discovery Enhancement
**Files:** `services/youtube-listener-innertube/innertube/discovery.go`
- Add `LiveStreamCandidate` struct (VideoID, Title, ViewerCount)
- Modify `collectLiveVideoIDsFromBrowse` to return `[]LiveStreamCandidate` extracting title + viewCountText
- Add `SelectStream(candidates, strategy, matchTerm)` function
- Update `DiscoverLiveStream` to accept strategy params and use `SelectStream`

### 2. Backend: Source-Manager Config Propagation
**Files:** `shared/sourcemanager/types.go`, `services/source-manager/models/source.go`, `services/source-manager/registry/repository.go`
- Add `StreamSelect` and `StreamMatch` fields to ActiveSource models
- Update SQL queries to extract `stream_select` and `stream_match` from config JSONB

### 3. Backend: Stream Manager Integration
**Files:** `services/youtube-listener-innertube/streams/manager.go`
- Read `StreamSelect`/`StreamMatch` from source when calling discovery
- Pass strategy to `DiscoverLiveStream`

### 4. Backend: Overlay-Manager Config Validation
**Files:** `services/overlay-manager/handlers/sources.go`
- Validate `stream_select` values when present in YouTube source config
- Validate `stream_match` required when strategy is `title_match`

### 5. Migration: Feature Gate
**Files:** `migrations/045_stream_selection_gate.sql`
- Insert `stream_selection` into `feature_gates` table

### 6. Backend: Feature Gate Constant
**Files:** `services/share-service/featuregates/cache.go`
- Add `GateStreamSelection` constant

### 7. Frontend: Stream Selection UI
**Files:** `frontend/src/app/overlays/[id]/page.tsx`, `frontend/src/lib/types/overlay.ts`
- Add YouTube stream selection dropdown on existing YouTube sources
- Show only for premium users
- Optional keyword input for title_match strategy
- Use existing PATCH source config API
