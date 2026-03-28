---
phase: quick-260328-tqh
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - services/tiktok-listener/src/tracing.ts
  - /home/caesar/git/caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml
autonomous: true
requirements: [QUICK-01]
must_haves:
  truths:
    - "TikTok listener initializes OpenTelemetry tracing on startup when OTEL_ENABLED=true"
    - "Traces from tiktok-listener appear in Tempo via Grafana after deployment"
  artifacts:
    - path: "services/tiktok-listener/src/tracing.ts"
      provides: "Working OpenTelemetry tracing initialization"
      contains: "SemanticResourceAttributes"
    - path: "/home/caesar/git/caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml"
      provides: "OTEL env vars for K8s deployment"
      contains: "OTEL_ENABLED"
  key_links:
    - from: "services/tiktok-listener/src/tracing.ts"
      to: "tempo-distributor.monitoring.svc.cluster.local:4317"
      via: "OTLPTraceExporter gRPC"
      pattern: "OTLPTraceExporter"
---

<objective>
Enable distributed tracing in the tiktok-listener service by fixing broken OpenTelemetry imports in tracing.ts and adding OTEL environment variables to the Kubernetes deployment.

Purpose: The tracing code already exists but uses API symbols from a newer version of @opentelemetry packages than what's installed. The K8s deployment also lacks the OTEL env vars needed to activate tracing.

Output: Working tracing initialization + K8s deployment with OTEL env vars, matching the pattern used by overlay-manager and share-service.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@services/tiktok-listener/src/tracing.ts
@services/tiktok-listener/src/index.ts
@services/tiktok-listener/package.json
@/home/caesar/git/caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml
@/home/caesar/git/caesar-deployment/apps/workloads/all-chat/overlay-manager-deployment.yaml (reference pattern for OTEL env vars)
@/home/caesar/git/caesar-deployment/apps/workloads/all-chat/configmap.yaml

<interfaces>
<!-- Installed package versions and correct API: -->

@opentelemetry/resources v1.30.1:
- `Resource` class (constructor takes attributes object)
- NO `resourceFromAttributes` function

@opentelemetry/semantic-conventions v1.18.1:
- `SemanticResourceAttributes.SERVICE_NAME` = "service.name"
- `SemanticResourceAttributes.SERVICE_VERSION` = "service.version"
- `SemanticResourceAttributes.DEPLOYMENT_ENVIRONMENT` = "deployment.environment"
- NO `ATTR_SERVICE_NAME`, NO `ATTR_SERVICE_VERSION`, NO `SEMRESATTRS_DEPLOYMENT_ENVIRONMENT`

@opentelemetry/sdk-node v0.45.1:
- `NodeSDK` class

@opentelemetry/sdk-trace-node v1.18.1:
- `BatchSpanProcessor` class
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Fix tracing.ts broken imports and API usage</name>
  <files>services/tiktok-listener/src/tracing.ts</files>
  <action>
Fix three broken imports/usages in tracing.ts that reference symbols not available in the installed package versions:

1. Replace `import { resourceFromAttributes } from '@opentelemetry/resources'` with `import { Resource } from '@opentelemetry/resources'`

2. Replace the three named imports from `@opentelemetry/semantic-conventions`:
   - `ATTR_SERVICE_NAME` -> `SemanticResourceAttributes` (from same package)
   - `ATTR_SERVICE_VERSION` -> already covered by SemanticResourceAttributes
   - `SEMRESATTRS_DEPLOYMENT_ENVIRONMENT` -> already covered by SemanticResourceAttributes

   New import: `import { SemanticResourceAttributes } from '@opentelemetry/semantic-conventions';`

3. Replace resource creation:
   ```typescript
   // OLD (broken):
   const resource = resourceFromAttributes({
     [ATTR_SERVICE_NAME]: serviceName,
     [ATTR_SERVICE_VERSION]: serviceVersion,
     [SEMRESATTRS_DEPLOYMENT_ENVIRONMENT]: environment,
   });

   // NEW (works with installed versions):
   const resource = new Resource({
     [SemanticResourceAttributes.SERVICE_NAME]: serviceName,
     [SemanticResourceAttributes.SERVICE_VERSION]: serviceVersion,
     [SemanticResourceAttributes.DEPLOYMENT_ENVIRONMENT]: environment,
   });
   ```

Keep everything else the same -- the BatchSpanProcessor, OTLPTraceExporter, NodeSDK usage, shutdown handlers are all correct.
  </action>
  <verify>
    <automated>cd /home/caesar/git/all-chat/services/tiktok-listener && npx tsc --noEmit src/tracing.ts 2>&1 | head -20</automated>
  </verify>
  <done>tracing.ts compiles without errors using the installed @opentelemetry package versions</done>
</task>

<task type="auto">
  <name>Task 2: Add OTEL environment variables to K8s deployment</name>
  <files>/home/caesar/git/caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml</files>
  <action>
Add the OpenTelemetry environment variables to the tiktok-listener-deployment.yaml container env block, matching the exact pattern used by overlay-manager-deployment.yaml and share-service-deployment.yaml.

Add these env vars after the existing TIKTOK_DEDUP_MAX_CACHE_SIZE entry (before `resources:`):

```yaml
        # OpenTelemetry tracing configuration
        - name: OTEL_ENABLED
          value: "true"
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          valueFrom:
            configMapKeyRef:
              name: allchat-config
              key: OTEL_EXPORTER_OTLP_ENDPOINT
        - name: OTEL_EXPORTER_OTLP_PROTOCOL
          valueFrom:
            configMapKeyRef:
              name: allchat-config
              key: OTEL_EXPORTER_OTLP_PROTOCOL
        - name: OTEL_SERVICE_NAME
          value: "tiktok-listener"
        - name: OTEL_RESOURCE_ATTRIBUTES
          valueFrom:
            configMapKeyRef:
              name: allchat-config
              key: OTEL_RESOURCE_ATTRIBUTES
        - name: ENVIRONMENT
          value: "production"
```

Note: The tracing.ts code reads OTEL_ENABLED and OTEL_EXPORTER_OTLP_ENDPOINT directly from process.env. The OTEL_SERVICE_NAME is not currently read by tracing.ts (it hardcodes 'tiktok-listener'), but setting it follows the convention and enables future refactoring to use it.
  </action>
  <verify>
    <automated>cd /home/caesar/git/caesar-deployment && kubectl apply -f apps/workloads/all-chat/tiktok-listener-deployment.yaml --dry-run=client 2>&1</automated>
  </verify>
  <done>K8s deployment YAML passes dry-run validation with OTEL env vars matching the overlay-manager pattern</done>
</task>

</tasks>

<verification>
1. `cd /home/caesar/git/all-chat/services/tiktok-listener && npx tsc --noEmit src/tracing.ts` -- no errors
2. `kubectl apply --dry-run=client -f tiktok-listener-deployment.yaml` -- valid YAML
3. After commit+push+deploy: check pod logs for `[Tracing] OpenTelemetry tracer initialized successfully`
4. After deploy: query Tempo in Grafana for service=tiktok-listener traces
</verification>

<success_criteria>
- tracing.ts compiles cleanly with installed @opentelemetry package versions
- K8s deployment includes OTEL_ENABLED=true and all required OTEL env vars from configmap
- After deployment, tiktok-listener pods emit traces to Tempo
</success_criteria>

<output>
After completion, create `.planning/quick/260328-tqh-enable-tracing-in-the-tiktok-listener-se/260328-tqh-SUMMARY.md`
</output>
