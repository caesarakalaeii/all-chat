---
phase: 04
slug: grafana-dashboard-audit-metrics-gap-implementation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-26
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go services) / vitest (tiktok-listener) / curl + jq (metrics endpoints) / Grafana MCP (live queries) |
| **Config file** | Per-service go.mod / services/tiktok-listener/package.json |
| **Quick run command** | `curl -s http://localhost:{port}/metrics \| grep {metric_name}` |
| **Full suite command** | `make build-all && for svc in services/*/; do (cd "$svc" && go test ./... 2>/dev/null); done` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `curl -s http://localhost:{port}/metrics | grep {metric_name}` for affected service
- **After every plan wave:** Run `make build-all` to verify all services compile
- **Before `/gsd:verify-work`:** Full suite must be green + live Prometheus query verification via Grafana MCP
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 04-01-01 | 01 | 1 | Audit | manual+code | `grep -r "RecordMessage\|RecordConnection" services/` | N/A | ⬜ pending |
| 04-02-01 | 02 | 2 | Metrics wiring | build | `make build-all` | ✅ | ⬜ pending |
| 04-03-01 | 03 | 3 | Dashboard JSON | file exists | `ls ../caesar-deployment/apps/workloads/all-chat/grafana-dashboards/` | ❌ W0 | ⬜ pending |
| 04-04-01 | 04 | 3 | Alert rules | file exists | `cat ../caesar-deployment/apps/workloads/all-chat/grafana-alerts/` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Verify `make build-all` passes before any changes
- [ ] Verify Grafana MCP connectivity for live Prometheus queries
- [ ] Confirm ServiceMonitor entries for existing services via `kubectl get servicemonitor -n allchat`

*Existing infrastructure covers metric endpoint exposure — Wave 0 is verification only.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dashboard renders correctly in Grafana | Dashboard strategy | JSON can be valid but visually broken | Open dashboard in Grafana UI, verify all panels show data |
| Alerts fire on threshold breach | Alerting gaps | Requires simulated failure condition | Stop a listener pod, verify alert fires within 2min |
| Live Prometheus scrape for new services | ServiceMonitor | Requires running cluster | `up{job=~"allchat-.*"}` via Grafana MCP |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
