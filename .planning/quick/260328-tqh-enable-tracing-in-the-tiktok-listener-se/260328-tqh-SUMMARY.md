---
phase: quick-260328-tqh
plan: 01
subsystem: tiktok-listener, k8s-deployment
tags: [tracing, opentelemetry, tiktok-listener, kubernetes]
dependency_graph:
  requires: []
  provides: [tiktok-listener-otel-tracing]
  affects: [tiktok-listener]
tech_stack:
  added: []
  patterns: [OpenTelemetry OTLP gRPC export, SemanticResourceAttributes v1 enum API]
key_files:
  created: []
  modified:
    - services/tiktok-listener/src/tracing.ts
    - /home/caesar/git/caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml
decisions:
  - SemanticResourceAttributes enum used instead of newer ATTR_* constants — matches @opentelemetry/semantic-conventions v1.18.1 installed
  - new Resource() constructor used instead of resourceFromAttributes() — matches @opentelemetry/resources v1.30.1 installed
metrics:
  duration: 35s
  completed: 2026-03-28
  tasks: 2
  files: 2
---

# Quick Task 260328-tqh: Enable Tracing in the TikTok Listener Service — Summary

**One-liner:** Fixed broken OpenTelemetry imports to use installed package APIs (Resource + SemanticResourceAttributes) and added OTEL env vars to K8s deployment, enabling Tempo tracing for tiktok-listener.

## Tasks Completed

| # | Task | Commit | Repo |
|---|------|--------|------|
| 1 | Fix tracing.ts broken imports and API usage | 01effa3 | all-chat |
| 2 | Add OTEL environment variables to K8s deployment | 5965d5f | caesar-deployment |

## What Was Done

### Task 1 — Fix tracing.ts broken imports (01effa3)

The existing `tracing.ts` referenced three symbols not present in the installed package versions:

- `resourceFromAttributes` from `@opentelemetry/resources` v1.30.1 — this function does not exist in v1.30.1; replaced with `new Resource()` constructor which does.
- `ATTR_SERVICE_NAME`, `ATTR_SERVICE_VERSION` from `@opentelemetry/semantic-conventions` v1.18.1 — these `ATTR_*` constants were introduced in a later version; replaced with `SemanticResourceAttributes.SERVICE_NAME` and `SemanticResourceAttributes.SERVICE_VERSION` from the same package.
- `SEMRESATTRS_DEPLOYMENT_ENVIRONMENT` — replaced with `SemanticResourceAttributes.DEPLOYMENT_ENVIRONMENT`.

The rest of tracing.ts (BatchSpanProcessor, OTLPTraceExporter, NodeSDK, shutdown handlers) was correct and left unchanged. The file now compiles cleanly with `npx tsc --noEmit`.

### Task 2 — K8s deployment OTEL env vars (5965d5f)

Added six environment variables to `tiktok-listener-deployment.yaml` after `TIKTOK_DEDUP_MAX_CACHE_SIZE` and before `resources:`, matching the exact pattern used by overlay-manager-deployment.yaml:

- `OTEL_ENABLED=true` — activates the tracing initialization path in tracing.ts
- `OTEL_EXPORTER_OTLP_ENDPOINT` — from allchat-config configmap (points to tempo-distributor)
- `OTEL_EXPORTER_OTLP_PROTOCOL` — from allchat-config configmap
- `OTEL_SERVICE_NAME=tiktok-listener` — service name label for Tempo
- `OTEL_RESOURCE_ATTRIBUTES` — from allchat-config configmap (cluster/namespace attributes)
- `ENVIRONMENT=production` — read by tracing.ts as deployment.environment resource attribute

Validated with `kubectl apply --dry-run=client` — both Deployment and Service resources pass.

## Deviations from Plan

None — plan executed exactly as written.

## Verification

1. `npx tsc --noEmit src/tracing.ts` — exits 0, no errors
2. `kubectl apply --dry-run=client -f tiktok-listener-deployment.yaml` — `configured (dry run)` for both resources
3. After commit+push+deploy: check pod logs for `[Tracing] OpenTelemetry tracer initialized successfully`
4. After deploy: query Tempo in Grafana for `service=tiktok-listener` traces

## Self-Check: PASSED

- `/home/caesar/git/all-chat/services/tiktok-listener/src/tracing.ts` — modified, committed at 01effa3
- `/home/caesar/git/caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml` — modified, committed at 5965d5f
