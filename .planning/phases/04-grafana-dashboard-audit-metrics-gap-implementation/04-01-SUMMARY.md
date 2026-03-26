---
phase: 04-grafana-dashboard-audit-metrics-gap-implementation
plan: "01"
subsystem: observability
tags: [prometheus, servicemonitor, gap-matrix, audit]
dependency_graph:
  requires: []
  provides: [servicemonitor-complete, gap-matrix]
  affects: [04-02, 04-03, 04-04, 04-05]
tech_stack:
  added: []
  patterns: [kubernetes-servicemonitor, prometheus-matchexpressions]
key_files:
  created:
    - .planning/phases/04-grafana-dashboard-audit-metrics-gap-implementation/04-GAP-MATRIX.md
  modified:
    - ../caesar-deployment/apps/workloads/all-chat/servicemonitor.yaml
    - ../caesar-deployment/apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml
decisions:
  - "youtube-listener-innertube Service had no app label — added app: youtube-listener-innertube so the ServiceMonitor matchExpressions selector can match it"
  - "Live Prometheus audit via Grafana MCP was not possible (cluster unreachable during automated execution); gap matrix produced from code audit instead — live verification is the purpose of the Task 3 checkpoint"
metrics:
  duration_seconds: 658
  tasks_completed: 2
  tasks_total: 3
  files_modified: 3
  completed_date: "2026-03-26"
---

# Phase 04 Plan 01: ServiceMonitor Fix & Gap Matrix Summary

**One-liner:** ServiceMonitor extended to cover all 7 Go listeners (added discord-listener, youtube-listener-innertube, twitch-eventsub-listener) with Service label fix; comprehensive code-audit gap matrix produced documenting per-service metric wiring status across all 14 services.

---

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Add three missing listeners to ServiceMonitor | 0a77d69 (caesar-deployment) | servicemonitor.yaml, youtube-listener-innertube-deployment.yaml |
| 2 | Run audit and produce confirmed gap matrix | a9a75de | .planning/04-GAP-MATRIX.md |

## Task 3 (Checkpoint)

Task 3 is `type="checkpoint:human-verify"` — paused for user review of gap matrix and ServiceMonitor changes.

---

## Key Findings

### ServiceMonitor Fix (Task 1)

The allchat-listeners ServiceMonitor previously covered 4 listeners. Three new listeners added in v1.5/v1.6 were missing:

| Listener | Root Cause | Fix Applied |
|----------|-----------|-------------|
| discord-listener | Not added when service was created in Phase 37 | Added to ServiceMonitor values |
| youtube-listener-innertube | Not added when service was created | Added to ServiceMonitor values + added `app: youtube-listener-innertube` label to the Service (Service had no app label, so matchExpressions couldn't match it) |
| twitch-eventsub-listener | Not added when service was created in Phase 38 | Added to ServiceMonitor values |

### Gap Matrix Findings (Task 2)

**Metrics wiring status (from code audit):**
- **Fully wired:** twitch-listener (all RecordX calls present), message-processor (pipeline stages)
- **Quota-only wired:** youtube-listener (quota/tracker.go only)
- **Local package (partial):** kick-listener, discord-listener, youtube-listener-innertube, tiktok-listener (Node.js), api-gateway
- **Zero wiring:** twitch-eventsub-listener, auth-service, overlay-manager, emote-service, token-refresh-service

**Highest priority gaps for Plans 02-05:**
1. twitch-eventsub-listener — zero RecordX() calls; needs full wiring from scratch
2. youtube-listener — no connection/message metrics; only quota tracked
3. kick-listener — no messages-received counter (publish + socket state covered by local pkg)
4. discord-listener — no messages-received counter (gateway events covered by local pkg)
5. api-gateway — no per-message delivery counter

**Dashboard gaps:**
- 6 dashboards exist but none cover Discord, InnerTube, or twitch-eventsub
- allchat-listener-health.json and allchat-listener-observability.json both missing 3 listeners

---

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing config] Added `app: youtube-listener-innertube` label to Service**
- **Found during:** Task 1
- **Issue:** The youtube-listener-innertube Kubernetes Service had no `app` label. The ServiceMonitor uses `matchExpressions` with key `app` to select Services. Without the label, adding `youtube-listener-innertube` to the values list would have no effect — Prometheus would never discover the service.
- **Fix:** Added `labels: app: youtube-listener-innertube` to the Service metadata in `youtube-listener-innertube-deployment.yaml`
- **Files modified:** `apps/workloads/all-chat/youtube-listener-innertube-deployment.yaml`
- **Commit:** 0a77d69 (caesar-deployment)

**2. [Rule 1 - Live audit unavailable] Gap matrix produced from code audit only**
- **Found during:** Task 2
- **Issue:** Grafana MCP tools (`query_prometheus`, `search_dashboards`) were not injected into this agent session's tool list. Kubernetes cluster was also unreachable (timeout after 20s).
- **Fix:** Produced gap matrix from code grep audit (04-RESEARCH.md) with explicit notation. All RESEARCH.md claims were independently verified via direct grep of service source files. The code audit provides high confidence for wiring status; live Prometheus scrape status will be confirmed by user during checkpoint review.
- **Impact:** The gap matrix "Scrape Status" column shows expected state (not live-confirmed). The "Metrics Emission" column is fully code-audit confirmed.

---

## Self-Check

### Files exist:
- [x] `.planning/phases/04-grafana-dashboard-audit-metrics-gap-implementation/04-GAP-MATRIX.md` — FOUND
- [x] `../caesar-deployment/apps/workloads/all-chat/servicemonitor.yaml` — modified with 3 new values

### Commits exist:
- [x] 0a77d69 — ServiceMonitor fix (in caesar-deployment repo)
- [x] a9a75de — Gap matrix creation (in all-chat repo)

### Acceptance criteria:
- [x] servicemonitor.yaml contains `- discord-listener`
- [x] servicemonitor.yaml contains `- youtube-listener-innertube`
- [x] servicemonitor.yaml contains `- twitch-eventsub-listener`
- [x] allchat-source-manager and allchat-services sections unchanged
- [x] Total values in allchat-listeners is now 7 (was 4)
- [x] 04-GAP-MATRIX.md exists with "Scrape Status" section
- [x] 04-GAP-MATRIX.md contains "Metrics Emission" section
- [x] 04-GAP-MATRIX.md contains "Dashboard Gaps" section
- [x] 04-GAP-MATRIX.md covers all 14 services

## Self-Check: PASSED
