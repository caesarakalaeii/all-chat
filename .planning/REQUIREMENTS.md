# Requirements: All-Chat InnerTube YouTube Listener

**Defined:** 2026-02-21
**Core Value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing and auto-scaling.

## v1.2 Requirements

Requirements for InnerTube YouTube listener (drop-in replacement for quota-limited official API listener).

### Core Ingestion

- [ ] **CORE-01**: Service can initialize masterchat client for live YouTube streams
- [ ] **CORE-02**: Service can poll InnerTube API for live chat messages via masterchat
- [ ] **CORE-03**: Service can normalize InnerTube message format to RawChatMessage schema
- [ ] **CORE-04**: Service can publish RawChatMessage to Redis Stream (chat:raw)
- [ ] **CORE-05**: Service exposes /health/live endpoint (always 200 OK)
- [ ] **CORE-06**: Service exposes /health/ready endpoint (checks Redis connectivity)

### Stream Management

- [ ] **STREAM-01**: Service can discover latest live stream video ID from channel ID
- [ ] **STREAM-02**: Service can filter out premieres (only target "live" streams)
- [ ] **STREAM-03**: Service can start monitoring a live stream on demand
- [ ] **STREAM-04**: Service can stop monitoring a stream gracefully
- [ ] **STREAM-05**: Service can detect when stream goes offline and stop polling
- [ ] **STREAM-06**: Service can reconnect on network errors with exponential backoff
- [ ] **STREAM-07**: Service can handle SIGTERM with graceful shutdown (cleanup connections, flush buffers)

### Event Support

- [ ] **EVENT-01**: Service can parse regular chat messages from InnerTube
- [ ] **EVENT-02**: Service can extract user metadata (username, avatar, badges)
- [ ] **EVENT-03**: Service can parse Super Chat messages with amount and color
- [ ] **EVENT-04**: Service can parse Super Sticker messages with sticker metadata
- [ ] **EVENT-05**: Service can parse membership welcome messages
- [ ] **EVENT-06**: Service can parse membership milestone messages
- [ ] **EVENT-07**: Service can parse ticker events (pinned/highlighted messages)

### Deletion Detection

- [ ] **DEL-01**: Service can detect single message deletion events from InnerTube
- [ ] **DEL-02**: Service can emit deletion event with EventType="message_deletion"
- [ ] **DEL-03**: Service can detect batch deletion events (ban/timeout)
- [ ] **DEL-04**: Service can emit batch deletion with deletion_type="batch" and ban metadata
- [ ] **DEL-05**: Service can buffer deletion events to handle race conditions (deletion before original message)

### Contract Validation

- [ ] **TEST-01**: Schema tests validate RawChatMessage JSON matches official listener output
- [ ] **TEST-02**: Golden replay tests compare InnerTube vs official listener outputs
- [ ] **TEST-03**: Lifecycle tests verify connection gating behavior
- [ ] **TEST-04**: Lifecycle tests verify stream offline detection and cleanup

### Production Readiness

- [ ] **PROD-01**: Service exposes Prometheus metrics endpoint (/metrics)
- [ ] **PROD-02**: Service tracks messages/sec, errors, reconnections in metrics
- [ ] **PROD-03**: README documents ToS disclosure (InnerTube is unofficial API)
- [ ] **PROD-04**: Deployment guide explains Docker image swap process
- [ ] **PROD-05**: Migration guide explains self-hoster transition from official listener

## Future Requirements

Deferred to future releases (v1.3+).

### Advanced Deployment

- **DEPLOY-01**: Kubernetes manifests (Deployment, Service, ConfigMap)
- **DEPLOY-02**: Canary deployment strategy (10%→50%→100% rollout)
- **DEPLOY-03**: Automatic rollback on error rate spike (>5%)
- **DEPLOY-04**: Cross-listener comparison tool for production validation

### Advanced Features

- **ADV-01**: Leader election integration with source-manager (multi-pod coordination)
- **ADV-02**: Load balancing support (hash-based sharding across pods)
- **ADV-03**: Connection gating (stop polling when overlay disconnected)
- **ADV-04**: Fast resume from Redis state (restore active streams on startup)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Quota tracking database | InnerTube has no quotas - removes 5+ tables and state machine complexity |
| OAuth token storage | InnerTube works unauthenticated - no token refresh needed |
| YouTube API quota coordination | InnerTube bypasses quota system entirely |
| Cross-service quota endpoints | /quota/record and YouTubeQuotaClient not needed |
| Polls/Creator goals events | Unstable InnerTube schema (research PITFALLS.md) |
| Viewer leaderboard rank | Low value, increases schema drift risk |
| Sending messages to chat | Out of scope - listener is read-only |
| Go language implementation | No mature Go InnerTube libraries exist (research STACK.md) |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| CORE-01 | Phase 9 | Pending |
| CORE-02 | Phase 9 | Pending |
| CORE-03 | Phase 9 | Pending |
| CORE-04 | Phase 9 | Pending |
| CORE-05 | Phase 9 | Pending |
| CORE-06 | Phase 9 | Pending |
| STREAM-01 | Phase 10 | Pending |
| STREAM-02 | Phase 10 | Pending |
| STREAM-03 | Phase 10 | Pending |
| STREAM-04 | Phase 10 | Pending |
| STREAM-05 | Phase 10 | Pending |
| STREAM-06 | Phase 10 | Pending |
| STREAM-07 | Phase 10 | Pending |
| EVENT-01 | Phase 9 | Pending |
| EVENT-02 | Phase 9 | Pending |
| EVENT-03 | Phase 10 | Pending |
| EVENT-04 | Phase 10 | Pending |
| EVENT-05 | Phase 10 | Pending |
| EVENT-06 | Phase 10 | Pending |
| EVENT-07 | Phase 10 | Pending |
| DEL-01 | Phase 11 | Pending |
| DEL-02 | Phase 11 | Pending |
| DEL-03 | Phase 13 | Pending |
| DEL-04 | Phase 13 | Pending |
| DEL-05 | Phase 13 | Pending |
| TEST-01 | Phase 11 | Pending |
| TEST-02 | Phase 11 | Pending |
| TEST-03 | Phase 11 | Pending |
| TEST-04 | Phase 11 | Pending |
| PROD-01 | Phase 12 | Pending |
| PROD-02 | Phase 12 | Pending |
| PROD-03 | Phase 12 | Pending |
| PROD-04 | Phase 12 | Pending |
| PROD-05 | Phase 12 | Pending |

**Coverage:**
- v1.2 requirements: 35 total
- Mapped to phases: 35 (100% coverage)
- Unmapped: 0 ✓

**Phase Distribution:**
- Phase 9 (Core Ingestion PoC): 8 requirements
- Phase 10 (Production Minimum): 12 requirements
- Phase 11 (Contract Validation): 6 requirements
- Phase 12 (Production Rollout): 5 requirements
- Phase 13 (Feature Parity): 4 requirements

---
*Requirements defined: 2026-02-21*
*Last updated: 2026-02-21 after roadmap creation*
