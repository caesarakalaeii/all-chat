# Phase 12: Production Rollout - Context

**Gathered:** 2026-02-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Deploy InnerTube YouTube Listener to production with gradual canary rollout (10% → 50% → 100%), comprehensive monitoring, and automatic rollback capabilities. This phase focuses on safe production deployment and observability, not new features.

</domain>

<decisions>
## Implementation Decisions

### Rollout Cadence & Promotion
- Fully automatic promotion (no manual approval gates)
- Conservative soak duration: 4-6 hours at each stage (10%, 50%) before auto-promoting
- Gate metrics for promotion:
  - Error rate < 1% (InnerTube-specific errors: HTTP, parsing, rate limiting)
  - Message rate match within 5% of official listener baseline
- Degradation handling: Auto-rollback immediately if metrics breach thresholds during soak

### Monitoring & Rollback Triggers
- Prometheus metrics to track continuously:
  - Error rate by type (HTTP, parse, rate limit)
  - Messages per second (per pod, compared to baseline)
  - Reconnection frequency (poll failures requiring retry)
  - Redis publish latency (InnerTube receive → Redis publish)
- Automatic rollback triggered by:
  - Error rate > 5%
  - Message rate drops > 20% below baseline
  - All InnerTube pods crashlooping
  - Redis publish failures (downstream broken)
- Rollback execution: Fast drain over 30 seconds (scale down gradually to avoid thundering herd)

### Production Testing Approach
- Direct to canary: No shadow mode or synthetic traffic (Phase 11 tests are sufficient)
- Validation during canary:
  - Compare message counts (InnerTube vs official within threshold)
  - Spot-check message content (manual inspection of random samples)
- Critical behavior to validate: Offline detection (InnerTube correctly stops when stream ends)
- Issue handling: Fix in place (keep canary at current %, deploy fix, resume promotion after validation)

### Documentation & Communication
- Create troubleshooting guide (common issues, diagnosis, resolution steps)
- ToS disclosure: Internal note only in README (InnerTube is unofficial API)
- Notifications: Just logs (no active Slack/email/paging - rely on observability tools)
- Dashboard visibility: Create Grafana dashboard with canary metrics and rollout status

### Claude's Discretion
- Exact Grafana dashboard layout and panel organization
- Specific PromQL queries for metrics (as long as they track the required metrics above)
- Kubernetes manifest details (replica counts, resource limits)
- Troubleshooting guide structure and depth

</decisions>

<specifics>
## Specific Ideas

- Rollout timeline: ~8-12 hours total for full rollout (4-6h at 10%, 4-6h at 50%, immediate 100% if metrics pass)
- Error rate threshold is 5% (matches Phase 12 requirement stated in ROADMAP.md)
- Message rate comparison baseline comes from official YouTube listener's steady-state throughput

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 12-production-rollout*
*Context gathered: 2026-02-24*
