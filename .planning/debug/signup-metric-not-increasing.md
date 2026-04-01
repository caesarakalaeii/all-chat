---
status: awaiting_human_verify
trigger: "signup-metric-not-increasing: Sign-up metric in Grafana never increases despite new users appearing in admin panel"
created: 2026-04-01T00:00:00Z
updated: 2026-04-01T18:30:00Z
---

## Current Focus

hypothesis: CONFIRMED — three compounding issues caused the metric to never show up in Grafana
test: Fix applied; needs human verification post-deploy
expecting: allchat_user_registrations_total and allchat_viewer_registrations_total now pre-initialised and persist correctly
next_action: deploy and verify in Grafana

## Symptoms
<!-- Written during gathering, then IMMUTABLE -->

expected: Sign-up metric in Grafana should increment when new streamers register via OAuth
actual: Metric stays flat / never increases, even though new users are confirmed in the admin panel
errors: None reported
reproduction: New users sign up via OAuth, admin panel shows them, but Grafana metric doesn't change
started: Metric has possibly never worked correctly

## Eliminated

- hypothesis: Metric name mismatch between code and Grafana
  evidence: business.go uses "allchat_user_registrations_total"; Grafana queries same name — no mismatch
  timestamp: 2026-04-01

- hypothesis: businessMetrics not wired to handlers
  evidence: main.go lines 211-212 clearly call .WithMetrics(businessMetrics) on both v1 and v2 handlers
  timestamp: 2026-04-01

- hypothesis: Registration code path not calling RecordUserRegistration
  evidence: platform_auth_v2.go:755 and auth_handler.go:182 both call h.metrics.RecordUserRegistration(); viewer_auth had NO call (confirmed bug)
  timestamp: 2026-04-01

## Evidence

- timestamp: 2026-04-01T15:00:00Z
  checked: kubectl exec auth-service pod /metrics endpoint
  found: Zero allchat_* metrics present — no HELP/TYPE headers for any allchat_ metric
  implication: Counter not appearing because no registrations happened in current pod's 18h lifetime

- timestamp: 2026-04-01T15:10:00Z
  checked: PostgreSQL users table
  found: 2 users on 2026-03-31, 4 on 2026-03-30, 3 on 2026-03-29 — users ARE being created
  implication: Users register, but the Prometheus counter resets on every pod restart and pre-initialisation was missing

- timestamp: 2026-04-01T15:15:00Z
  checked: viewer_auth.go for metrics
  found: ViewerAuthHandler has no metrics field, no RecordViewerRegistration call anywhere
  implication: Viewer sign-up metric has never existed

- timestamp: 2026-04-01T15:20:00Z
  checked: Grafana dashboard query
  found: Uses allchat_user_registrations_total which is correct name; no viewer query exists
  implication: Dashboard missing viewer metrics entirely

## Resolution

root_cause: |
  Three compounding issues:
  1. allchat_user_registrations_total is a CounterVec that only appears in /metrics after its
     first .Inc() call. With 2 replicas and frequent Keel auto-deployments (multiple per day),
     most pods never serve a registration event before being replaced, so the metric is perpetually
     absent from /metrics and thus from Prometheus scrapes.
  2. The counter has no persistent baseline — every pod restart resets it to zero, and Prometheus
     increase() on a counter that never existed in a scrape window shows nothing.
  3. ViewerAuthHandler had no metrics instrumentation at all (no RecordViewerRegistration calls).

fix: |
  1. Pre-initialise allchat_user_registrations_total and new allchat_viewer_registrations_total
     with all known platform label values at startup (so metric always appears in /metrics).
  2. Added allchat_total_users_by_platform gauge seeded from PostgreSQL at startup — provides
     persistent baseline that survives pod restarts.
  3. Added RecordViewerRegistration() to BusinessMetrics and wired it into ViewerAuthHandler
     at all three new-viewer creation points (Twitch, YouTube, Kick).
  4. Added CountByAuthProvider() to UserRepository for startup DB seeding.
  5. Updated Grafana dashboard to show viewer registrations and total users gauge.

verification:
files_changed:
  - shared/metrics/business.go
  - shared/metrics/business_test.go
  - services/auth-service/handlers/viewer_auth.go
  - services/auth-service/repository/user_repository.go
  - services/auth-service/cmd/main.go
  - caesar-deployment/apps/platform/allchat-monitoring/allchat-grafana-dashboards.yaml
