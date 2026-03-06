# Phase 11: Contract Validation - Context

**Gathered:** 2026-02-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Build comprehensive contract tests to prove the InnerTube YouTube listener produces byte-for-byte identical output to the official YouTube listener. This validates the drop-in replacement guarantee before production rollout.

Scope includes: schema validation tests, 24-hour dual-listener integration test, lifecycle behavior tests, deletion event tests.

</domain>

<decisions>
## Implementation Decisions

### Test Data Collection
- **Live streams only** — All tests run against real-time YouTube live streams, not pre-recorded fixtures
- **Golden files captured from official listener** — Run official youtube-listener, save its RawChatMessage output as ground truth
- **Stream variety:** Test both extremes
  - High-volume chat (>100 messages/sec) — stress test parsing under load
  - Low-activity streams (<5 messages/min) — validate edge cases like empty continuations
- **Data volume:** Sample 100+ messages from 5-10 different streams (not 100 distinct streams)

### Comparison Methodology
- **Field-by-field semantic match** — Compare parsed JSON objects field by field, not byte-for-byte string comparison
- **Allow internal IDs to differ** — InnerTube and official API may use different message ID schemes; compare content, not IDs
- **Allow message reordering within time window** — Messages within ~1 second can arrive out of order due to network timing
- **Mismatch calculation:** Both missing messages AND field differences count toward the <0.1% threshold
  - Missing in one listener but not the other = mismatch
  - Present in both but fields differ (excluding allowlisted IDs) = mismatch

### Dual-Listener Orchestration
- **Parallel execution** — Official and InnerTube listeners both connect to same live stream simultaneously
- **Message matching by content** — Correlate messages by comparing text/author, robust to ID differences
- **Separate Kubernetes pods** — Deploy each listener in isolated pods for realistic production-like testing
- **Redis Streams production path** — Both listeners publish to Redis, comparator consumes from production pipeline

### Failure Investigation
- **Artifacts captured on mismatch:**
  - Raw JSON from both listeners (full RawChatMessage objects)
  - Surrounding context (±5 messages before/after mismatch)
  - InnerTube API response (raw continuation payload)
  - Timestamps and latency metrics (when each listener received message)
- **Diff format:** JSON diff using git-style unified diff format (familiar to developers)
- **Monitoring strategy:** Continuous monitoring with final report after 24 hours (let test run to completion)

### Claude's Discretion
- Reproduction workflow for debugging mismatches (replay, harness, or re-run)
- Exact tolerance value for timestamp window reordering
- Lifecycle test implementation details (connection gating, offline detection)

</decisions>

<specifics>
## Specific Ideas

- The 24-hour dual-listener test should run uninterrupted to completion, not stop on first mismatch
- Golden file schema tests (100+) are separate from the 24-hour integration test
- Use production Redis Streams path to validate full pipeline, not just parser logic

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 11-contract-validation*
*Context gathered: 2026-02-21*
