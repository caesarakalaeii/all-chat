---
status: resolved
trigger: "Grafana dashboards partially showing no data after security hardening commits in caesar-deployment repo"
created: 2026-04-11T00:00:00Z
updated: 2026-04-11T00:00:00Z
---

## Current Focus

hypothesis: CONFIRMED - NetworkPolicies block all Prometheus scraping from monitoring namespace
test: Checked all 49 Prometheus targets for allchat namespace — all DOWN with "connection refused"
expecting: Fix requires adding monitoring namespace ingress rules to each NetworkPolicy
next_action: RESOLVED — human confirmed Grafana dashboards showing data after NetworkPolicy fix

## Symptoms

expected: All Grafana dashboard panels show metrics data from all allchat services
actual: Some dashboards/panels work, others show "No data" — partial metrics blackout
errors: No specific error messages reported yet — need to check Prometheus targets and pod logs
reproduction: Open Grafana dashboards — some panels display data, others are empty
started: After recent security hardening commits in caesar-deployment repo (5fae08d, 5d4df4d, eb09834, 024bb9f, 2a9184c, e7aeafe)

## Eliminated

## Evidence

- timestamp: 2026-04-11T00:01:00Z
  checked: Prometheus targets via API inside prometheus pod
  found: All 49 allchat namespace targets are DOWN with "connection refused" errors
  implication: Prometheus in monitoring namespace cannot reach any allchat pod

- timestamp: 2026-04-11T00:02:00Z
  checked: All 15 NetworkPolicies in allchat namespace
  found: default-deny-ingress blocks all ingress; per-service policies only allow app-level traffic (ingress-nginx, api-gateway, etc). NONE include ingress from monitoring namespace.
  implication: NetworkPolicies are the direct cause of Prometheus scrape failures

- timestamp: 2026-04-11T00:03:00Z
  checked: ServiceMonitors (allchat-listeners, allchat-services, allchat-source-manager) and PodMonitor (allchat-cluster)
  found: ServiceMonitors scrape port "http" (each service's main port). PodMonitor scrapes port "metrics" (9187) on CNPG pods.
  implication: Fix must allow monitoring namespace ingress on each service's HTTP port AND port 9187 for CNPG pods

- timestamp: 2026-04-11T00:04:00Z
  checked: Service port definitions for all allchat services
  found: All services serve metrics on their primary HTTP port (same port as application)
  implication: No separate metrics port needed; just allow monitoring namespace to reach each service's existing port

## Resolution

root_cause: Security hardening commit 5fae08d introduced NetworkPolicies with a default-deny-ingress policy and per-service allow-lists. None of the per-service policies include an ingress rule allowing traffic from the monitoring namespace (where Prometheus runs). This blocks all Prometheus scraping of /metrics endpoints across every allchat service and CNPG PostgreSQL pods. Additionally: (1) source-controller pods had no NetworkPolicy at all (only default-deny applied), (2) the listeners NetworkPolicy used label value "youtube-listener-innertube" but pods actually have label "youtube-listener".
fix: Three changes to network-policies.yaml: (1) Add monitoring namespace ingress rules to all 15 existing NetworkPolicies, allowing Prometheus to reach each service on its HTTP port and CNPG on port 9187. (2) Add new source-controller NetworkPolicy with namespace-internal + monitoring ingress on port 8088. (3) Fix listeners podSelector to use "youtube-listener" instead of "youtube-listener-innertube" to match actual pod labels.
verification: Self-verified (49/49 Prometheus targets UP) and human-confirmed (Grafana dashboards showing data in production).
files_changed: [/home/caesar/git/caesar-deployment/apps/workloads/all-chat/network-policies.yaml]
