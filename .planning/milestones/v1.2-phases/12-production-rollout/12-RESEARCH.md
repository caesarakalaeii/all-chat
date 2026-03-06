# Phase 12: Production Rollout - Research

**Researched:** 2026-02-24
**Domain:** Kubernetes canary deployment with automatic rollback, Prometheus monitoring, and production observability
**Confidence:** HIGH

## Summary

Phase 12 deploys the InnerTube YouTube Listener to production using a gradual canary rollout strategy (10% → 50% → 100%) with automatic promotion based on Prometheus metrics and automatic rollback on threshold breaches. The technical challenge is implementing a time-based automatic promotion system that continuously monitors error rates and message throughput while providing fast rollback capabilities without manual intervention.

This requires three distinct capabilities: (1) Kubernetes deployment strategies for traffic splitting without service mesh overhead, (2) Prometheus-based analysis with PromQL queries that trigger rollback decisions, and (3) graceful pod draining to prevent thundering herd during rollback. The ecosystem offers two primary tools—Argo Rollouts and Flagger—both of which provide automated canary analysis with Prometheus integration, though with different architectural philosophies.

**Primary recommendation:** Use Argo Rollouts for automated canary deployment with AnalysisTemplate-driven promotion/rollback, Prometheus metrics with 1-minute scrape intervals for error rate and message throughput, Grafana dashboards with Rollout status panels, and 30-second graceful termination with preStop hooks to prevent connection loss during rollback.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Rollout Cadence & Promotion:**
- Fully automatic promotion (no manual approval gates)
- Conservative soak duration: 4-6 hours at each stage (10%, 50%) before auto-promoting
- Gate metrics for promotion:
  - Error rate < 1% (InnerTube-specific errors: HTTP, parsing, rate limiting)
  - Message rate match within 5% of official listener baseline
- Degradation handling: Auto-rollback immediately if metrics breach thresholds during soak

**Monitoring & Rollback Triggers:**
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

**Production Testing Approach:**
- Direct to canary: No shadow mode or synthetic traffic (Phase 11 tests are sufficient)
- Validation during canary:
  - Compare message counts (InnerTube vs official within threshold)
  - Spot-check message content (manual inspection of random samples)
- Critical behavior to validate: Offline detection (InnerTube correctly stops when stream ends)
- Issue handling: Fix in place (keep canary at current %, deploy fix, resume promotion after validation)

**Documentation & Communication:**
- Create troubleshooting guide (common issues, diagnosis, resolution steps)
- ToS disclosure: Internal note only in README (InnerTube is unofficial API)
- Notifications: Just logs (no active Slack/email/paging - rely on observability tools)
- Dashboard visibility: Create Grafana dashboard with canary metrics and rollout status

### Claude's Discretion

- Exact Grafana dashboard layout and panel organization
- Specific PromQL queries for metrics (as long as they track the required metrics above)
- Kubernetes manifest details (replica counts, resource limits)
- Troubleshooting guide structure and depth

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope

</user_constraints>

## Standard Stack

### Core Progressive Delivery Tools

| Tool | Version | Purpose | Why Standard |
|------|---------|---------|--------------|
| Argo Rollouts | v1.7+ | Canary deployment orchestration with automatic rollback | CNCF project (7.5k+ stars), native Prometheus integration, built-in AnalysisTemplate for metrics-driven decisions |
| Prometheus | v2.50+ | Metrics collection and querying | Already in architecture (Phase 8), industry standard for Kubernetes monitoring |
| Grafana | v10.0+ | Rollout visualization and dashboard | Already in architecture (Phase 8), Flagger/Argo provide pre-built dashboards |
| prometheus-operator | v0.70+ | ServiceMonitor CRDs for metric scraping | Simplifies Prometheus configuration, already used for existing metrics |

### Supporting Tools

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| kubectl-argo-rollouts | v1.7+ | CLI plugin for managing rollouts | Manual rollout inspection, promotion, abort during testing |
| kustomize | v5.0+ | Deployment manifest templating | Generate official vs InnerTube variants without duplication |
| jq | v1.7+ | JSON parsing for artifact analysis | Troubleshooting scripts that parse metric JSON from Prometheus API |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Argo Rollouts | Flagger | Flagger designed for service meshes (Istio/Linkerd), Argo lighter weight without mesh overhead |
| Argo Rollouts | Manual deployment + monitoring | Argo provides battle-tested rollback logic, custom scripts error-prone |
| ServiceMonitor CRDs | Manual Prometheus config | ServiceMonitor auto-discovers pods, prevents config drift |
| Grafana dashboards | Argo Rollouts UI | Both needed: UI for operator view, Grafana for historical analysis and correlation with other metrics |

**Installation:**
```bash
# Argo Rollouts controller
kubectl create namespace argo-rollouts
kubectl apply -n argo-rollouts -f https://github.com/argoproj/argo-rollouts/releases/latest/download/install.yaml

# CLI plugin
curl -LO https://github.com/argoproj/argo-rollouts/releases/latest/download/kubectl-argo-rollouts-linux-amd64
chmod +x kubectl-argo-rollouts-linux-amd64
sudo mv kubectl-argo-rollouts-linux-amd64 /usr/local/bin/kubectl-argo-rollouts

# Verify installation
kubectl argo rollouts version
```

## Architecture Patterns

### Recommended Project Structure

```
deployments/
├── k8s/
│   ├── youtube-listener-innertube/
│   │   ├── base/                           # Kustomize base
│   │   │   ├── rollout.yaml                # Argo Rollout (replaces Deployment)
│   │   │   ├── service.yaml                # Kubernetes Service (stable + canary)
│   │   │   ├── analysis-template.yaml     # Prometheus metrics analysis
│   │   │   └── kustomization.yaml
│   │   ├── production/                     # Production overlay
│   │   │   ├── kustomization.yaml          # Applies base with prod settings
│   │   │   └── rollout-patch.yaml          # Replica count, image tag
│   │   └── README.md                       # Deployment instructions
│   ├── youtube-listener/                   # Official listener (unchanged)
│   │   └── deployment.yaml
│   └── monitoring/
│       ├── servicemonitor-innertube.yaml   # Prometheus scrape config
│       └── grafana-dashboard-rollout.json  # Grafana dashboard JSON
├── scripts/
│   ├── promote-canary.sh                   # Manual promotion (emergency use)
│   ├── abort-rollout.sh                    # Manual abort and rollback
│   └── check-metrics.sh                    # CLI metrics check (debugging)
└── docs/
    ├── ROLLOUT_GUIDE.md                    # Operator runbook
    └── TROUBLESHOOTING.md                  # Issue diagnosis and resolution
```

### Pattern 1: Argo Rollout with AnalysisTemplate

**What:** Replace standard Kubernetes Deployment with Argo Rollout CRD that defines canary steps and references AnalysisTemplate for metrics-based promotion/rollback.

**When to use:** All InnerTube production deployments. Official listener remains standard Deployment (no rollout needed for stable service).

**Example:**
```yaml
# deployments/k8s/youtube-listener-innertube/base/rollout.yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: youtube-listener-innertube
  namespace: all-chat
spec:
  replicas: 10  # Total desired replicas (will split during canary)
  revisionHistoryLimit: 3
  selector:
    matchLabels:
      app: youtube-listener-innertube
  template:
    metadata:
      labels:
        app: youtube-listener-innertube
        version: innertube  # Distinguish from official listener
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      terminationGracePeriodSeconds: 30  # CRITICAL: Graceful shutdown
      containers:
        - name: youtube-listener-innertube
          image: allchat/youtube-listener-innertube:latest  # Updated via CI
          ports:
            - name: http
              containerPort: 8080
          env:
            - name: LOG_LEVEL
              value: "info"
            - name: REDIS_HOST
              valueFrom:
                configMapKeyRef:
                  name: all-chat-config
                  key: redis_host
          resources:
            requests:
              cpu: "200m"
              memory: "256Mi"
            limits:
              cpu: "1000m"
              memory: "1Gi"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8080
            initialDelaySeconds: 15
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 5
          lifecycle:
            preStop:
              exec:
                # Sleep 5s to allow endpoint removal from load balancer
                command: ["/bin/sh", "-c", "sleep 5"]
  strategy:
    canary:
      # Canary Service (receives traffic during rollout)
      canaryService: youtube-listener-innertube-canary
      # Stable Service (receives traffic for stable version)
      stableService: youtube-listener-innertube-stable
      # Traffic routing (native Kubernetes, no service mesh)
      trafficRouting:
        managedRoutes:
          - name: primary
      # Canary steps with analysis
      steps:
        # Step 1: Deploy 10% canary, soak for 4 hours
        - setWeight: 10
        - pause: {}  # Controlled by analysis
        - analysis:
            templates:
              - templateName: innertube-metrics-analysis
            args:
              - name: service-name
                value: youtube-listener-innertube-canary
              - name: baseline-service
                value: youtube-listener  # Official listener
        # Step 2: If analysis passes, promote to 50%, soak for 4 hours
        - setWeight: 50
        - pause: {}
        - analysis:
            templates:
              - templateName: innertube-metrics-analysis
            args:
              - name: service-name
                value: youtube-listener-innertube-canary
              - name: baseline-service
                value: youtube-listener
        # Step 3: If analysis passes, promote to 100%
        - setWeight: 100
```

**Key design decisions:**
- `terminationGracePeriodSeconds: 30`: Allows 30 seconds for graceful shutdown (matches user requirement for 30-second drain)
- `preStop` hook with 5-second sleep: Ensures Kubernetes endpoint controller removes pod from Service before connections terminate
- `trafficRouting.managedRoutes`: Uses native Kubernetes service splitting (no Istio/Linkerd required)
- Analysis runs continuously during pause, auto-promotes if metrics pass for configured duration

### Pattern 2: AnalysisTemplate with Prometheus Queries

**What:** Define metrics thresholds and PromQL queries that Argo Rollouts evaluates to decide promotion or rollback.

**When to use:** Referenced by Rollout during canary analysis phase. Defines what "success" means for this deployment.

**Example:**
```yaml
# deployments/k8s/youtube-listener-innertube/base/analysis-template.yaml
apiVersion: argoproj.io/v1alpha1
kind: AnalysisTemplate
metadata:
  name: innertube-metrics-analysis
  namespace: all-chat
spec:
  args:
    - name: service-name        # Canary service (InnerTube)
    - name: baseline-service    # Baseline service (official listener)
  metrics:
    # Metric 1: Error rate < 1% for promotion, > 5% triggers rollback
    - name: error-rate
      interval: 1m  # Check every minute
      successCondition: result < 0.01  # < 1% error rate (promotion gate)
      failureCondition: result > 0.05  # > 5% error rate (rollback trigger)
      failureLimit: 3  # Rollback after 3 consecutive failures
      provider:
        prometheus:
          address: http://prometheus.monitoring.svc.cluster.local:9090
          query: |
            sum(rate(youtube_listener_errors_total{service="{{args.service-name}}"}[5m]))
            /
            sum(rate(youtube_listener_requests_total{service="{{args.service-name}}"}[5m]))

    # Metric 2: Message rate within 5% of baseline
    - name: message-rate-deviation
      interval: 1m
      successCondition: result < 0.05  # < 5% deviation (promotion gate)
      failureCondition: result > 0.20  # > 20% deviation (rollback trigger)
      failureLimit: 5  # Allow some variance, rollback after 5 consecutive failures
      provider:
        prometheus:
          address: http://prometheus.monitoring.svc.cluster.local:9090
          query: |
            abs(
              sum(rate(youtube_listener_messages_published_total{service="{{args.service-name}}"}[5m]))
              -
              sum(rate(youtube_listener_messages_published_total{service="{{args.baseline-service}}"}[5m]))
            )
            /
            sum(rate(youtube_listener_messages_published_total{service="{{args.baseline-service}}"}[5m]))

    # Metric 3: Redis publish success rate (detect downstream failures)
    - name: redis-publish-success
      interval: 1m
      successCondition: result > 0.99  # > 99% success rate
      failureCondition: result < 0.95  # < 95% triggers rollback
      failureLimit: 2  # Fast rollback on downstream issues
      provider:
        prometheus:
          address: http://prometheus.monitoring.svc.cluster.local:9090
          query: |
            sum(rate(youtube_listener_redis_publish_success_total{service="{{args.service-name}}"}[5m]))
            /
            sum(rate(youtube_listener_redis_publish_attempts_total{service="{{args.service-name}}"}[5m]))

    # Metric 4: Pod crashloop detection (all pods restarting)
    - name: pod-restart-rate
      interval: 1m
      successCondition: result == 0  # No restarts
      failureCondition: result > 2  # More than 2 restarts/min triggers rollback
      failureLimit: 1  # Immediate rollback on crashloop
      provider:
        prometheus:
          address: http://prometheus.monitoring.svc.cluster.local:9090
          query: |
            sum(rate(kube_pod_container_status_restarts_total{
              namespace="all-chat",
              pod=~"youtube-listener-innertube-.*"
            }[5m])) * 60
```

**Key design decisions:**
- Different `failureLimit` values: Strict for crashes (1), lenient for message rate variance (5)
- 5-minute rate windows: Balances responsiveness with noise reduction
- Absolute value for message rate comparison: Catches both over-publishing and under-publishing
- Multiple conditions: `successCondition` gates promotion, `failureCondition` triggers rollback

### Pattern 3: Grafana Dashboard for Canary Observability

**What:** Pre-built dashboard showing canary rollout progress, metric thresholds, and promotion/rollback status.

**When to use:** Continuous monitoring during rollout, post-mortem analysis after rollback.

**Example structure:**
```json
{
  "dashboard": {
    "title": "YouTube Listener InnerTube Rollout",
    "panels": [
      {
        "title": "Rollout Status",
        "type": "stat",
        "targets": [{
          "expr": "argo_rollouts_info{name='youtube-listener-innertube'}"
        }],
        "description": "Current rollout phase: Progressing, Paused, Degraded, Healthy"
      },
      {
        "title": "Canary Weight",
        "type": "gauge",
        "targets": [{
          "expr": "argo_rollouts_canary_weight{name='youtube-listener-innertube'}"
        }],
        "description": "Current traffic percentage to canary (0-100%)"
      },
      {
        "title": "Error Rate (InnerTube vs Official)",
        "type": "timeseries",
        "targets": [
          {
            "expr": "sum(rate(youtube_listener_errors_total{service='youtube-listener-innertube-canary'}[5m])) / sum(rate(youtube_listener_requests_total{service='youtube-listener-innertube-canary'}[5m]))",
            "legendFormat": "InnerTube Canary"
          },
          {
            "expr": "sum(rate(youtube_listener_errors_total{service='youtube-listener'}[5m])) / sum(rate(youtube_listener_requests_total{service='youtube-listener'}[5m]))",
            "legendFormat": "Official Listener"
          }
        ],
        "thresholds": [
          { "value": 0.01, "color": "green" },
          { "value": 0.05, "color": "red" }
        ]
      },
      {
        "title": "Message Rate Comparison",
        "type": "timeseries",
        "targets": [
          {
            "expr": "sum(rate(youtube_listener_messages_published_total{service='youtube-listener-innertube-canary'}[5m]))",
            "legendFormat": "InnerTube Canary"
          },
          {
            "expr": "sum(rate(youtube_listener_messages_published_total{service='youtube-listener'}[5m]))",
            "legendFormat": "Official Listener"
          }
        ]
      },
      {
        "title": "Analysis Run Status",
        "type": "table",
        "targets": [{
          "expr": "argo_rollouts_analysis_run_info{rollout='youtube-listener-innertube'}"
        }],
        "description": "Current and historical analysis runs with success/failure status"
      }
    ]
  }
}
```

**Panel purposes:**
1. **Rollout Status**: High-level health (Progressing, Degraded, Healthy)
2. **Canary Weight**: Visual indicator of current traffic split
3. **Error Rate**: Side-by-side comparison with promotion (1%) and rollback (5%) thresholds
4. **Message Rate**: Validates InnerTube producing similar volume to official listener
5. **Analysis Run Status**: Shows which metrics passed/failed, helpful for debugging rollback

### Anti-Patterns to Avoid

- **Manual traffic splitting**: Don't manually adjust replica counts, let Argo Rollouts manage traffic weights
- **Skipping preStop hooks**: Immediate termination causes connection drops and "connection reset by peer" errors
- **Short soak times**: User specified 4-6 hours for good reason—fast promotion hides rare edge cases
- **Ignoring baseline comparison**: InnerTube error rate alone isn't enough, must compare to official listener to detect regressions
- **No graceful shutdown**: Services that don't handle SIGTERM gracefully will terminate mid-request during rollback

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Canary orchestration | Custom traffic splitting with kubectl scale | Argo Rollouts | Handles race conditions between replica scaling and service endpoint updates, battle-tested rollback logic |
| Metrics-based promotion | Shell script polling Prometheus + kubectl | AnalysisTemplate | Argo Rollouts handles analysis lifecycle, automatic rollback on failure, historical run tracking |
| Graceful shutdown | Custom signal handling in Go | terminationGracePeriodSeconds + preStop hook | Kubernetes already provides proper shutdown sequencing, reinventing introduces timing bugs |
| Rollout dashboard | Custom Grafana dashboard from scratch | Argo Rollouts pre-built dashboard | Community-maintained dashboard covers all rollout phases, saves 8+ hours of panel configuration |
| Traffic weight calculation | Manual percentage-to-replica math | Argo Rollouts `setWeight` | Handles edge cases like odd replica counts, ensures at least 1 canary pod exists |

**Key insight:** Progressive delivery is deceptively complex—edge cases like "what if promotion happens mid-analysis?" or "how to handle pods evicted during canary?" are already solved by Argo Rollouts. Custom scripts inevitably rediscover these edge cases the hard way.

## Common Pitfalls

### Pitfall 1: Thundering Herd During Rollback

**What goes wrong:** All canary pods terminate simultaneously during rollback, causing spike in load on remaining official listener pods. Redis connections surge, CPU spikes, and official listener can become overloaded.

**Why it happens:** Default Kubernetes behavior terminates all pods in a ReplicaSet at once. When 10% of workload (1-2 pods) terminates, it's fine. When 50% terminates (5 pods), official listener sees 50% traffic increase instantly.

**How to avoid:**
1. Set `terminationGracePeriodSeconds: 30` to allow 30-second drain window
2. Add `preStop` hook with 5-second sleep to stagger terminations
3. Ensure official listener has capacity buffer (run at 70% CPU, not 90%)
4. Configure HPA on official listener to scale up during rollback

**Warning signs:**
- Prometheus alerts for official listener CPU > 80% during rollback
- Redis connection pool exhaustion errors in logs
- Increased latency in `youtube_listener_redis_publish_latency_seconds`

### Pitfall 2: Promotion Before Metrics Stabilize

**What goes wrong:** Canary promoted to 50% after exactly 4 hours, but metrics haven't stabilized yet. Error rate spikes 10 minutes after promotion, triggering rollback from 50% (more disruptive than rollback from 10%).

**Why it happens:** Argo Rollouts' `pause: { duration: "4h" }` is clock-based, not metrics-based. If analysis runs every 1 minute but needs 5 consecutive successes to be confident, 4 hours might not be enough samples.

**How to avoid:**
1. Use `pause: {}` (indefinite) instead of fixed duration
2. Let AnalysisTemplate control promotion via `successCondition` and sufficient measurement interval
3. Set analysis `interval: 1m` with `count: 240` (240 samples = 4 hours of 1-minute checks)
4. Require minimum consecutive successes: `consecutiveSuccessfulLimit: 10`

**Warning signs:**
- Rollout promoted despite recent metric spikes visible in Grafana
- Analysis run shows < 50% of samples succeeded (e.g., 120/240 passed)
- Canary pods show high restart count but rollout still progressed

### Pitfall 3: Missing Baseline Comparison

**What goes wrong:** InnerTube canary shows 2% error rate. Is that bad? Without comparing to official listener's current error rate (which might be 3% due to YouTube API issues), you don't know if InnerTube is better or worse.

**Why it happens:** Absolute thresholds (e.g., "error rate < 1%") don't account for baseline conditions. If YouTube API is having issues, both listeners should see elevated error rates.

**How to avoid:**
1. Always compare canary to baseline in PromQL: `(canary_errors / canary_requests) - (baseline_errors / baseline_requests)`
2. Use relative thresholds: "canary error rate no more than 10% higher than baseline"
3. Track baseline metrics in separate panel on Grafana dashboard
4. Alert on baseline degradation separately from canary issues

**Warning signs:**
- Rollback triggered but official listener shows same error rate as canary
- YouTube API outage causes rollback despite InnerTube performing identically to official
- Post-mortem reveals canary was actually *better* than baseline but still rolled back

### Pitfall 4: Ignoring Pod-Level Metrics

**What goes wrong:** Aggregate metrics look healthy (1% error rate across 10 pods), but 1 pod is failing 50% of requests while 9 pods are perfect. Rollout promotes based on aggregate, then the broken pod receives more traffic at 50% weight.

**Why it happens:** Prometheus queries sum across all pods without checking per-pod distribution. A single misconfigured pod (wrong environment variable, bad network route) can be masked by healthy majority.

**How to avoid:**
1. Add per-pod metric check to AnalysisTemplate: `max(rate(errors) by (pod)) < threshold`
2. Monitor pod-level metrics in Grafana with heatmap or table panel
3. Use `readinessProbe` to automatically remove unhealthy pods from traffic
4. Set `maxUnavailable: 0` in rollout strategy to ensure at least all pods are healthy

**Warning signs:**
- Logs show one pod with repeated errors while others are silent
- Kubernetes events show pod restarts for specific pod name
- Per-pod metrics dashboard shows outlier pod with 10x error rate

### Pitfall 5: No Rollback Testing

**What goes wrong:** Rollback logic is untested until production incident. When actual rollback needed, discover that rollback takes 10 minutes (not 30 seconds as expected) because of misconfigured drain logic.

**Why it happens:** Rollback paths are rarely exercised. Unlike deployments (which happen frequently), rollbacks only happen during failures, so bugs aren't discovered until critical moment.

**How to avoid:**
1. Manually test rollback in staging: Deploy canary, trigger rollback with `kubectl argo rollouts abort`
2. Inject failures during canary testing to trigger automatic rollback: Use chaos engineering (kill pods, inject errors)
3. Measure actual rollback duration and compare to 30-second target
4. Document rollback procedure in runbook with expected duration

**Warning signs:**
- No evidence of rollback testing in Phase 11 contract tests
- Runbook says "rollback should be fast" without specific duration
- No Prometheus recording rules for rollback duration

## Code Examples

Verified patterns from official Argo Rollouts documentation:

### Manual Rollout Management

```bash
# Check rollout status
kubectl argo rollouts get rollout youtube-listener-innertube -n all-chat

# Watch rollout progress (real-time updates)
kubectl argo rollouts get rollout youtube-listener-innertube -n all-chat --watch

# Manually promote canary to next step (emergency use)
kubectl argo rollouts promote youtube-listener-innertube -n all-chat

# Abort rollout and trigger rollback
kubectl argo rollouts abort youtube-listener-innertube -n all-chat

# Restart rollout (after fixing issue)
kubectl argo rollouts restart youtube-listener-innertube -n all-chat

# View analysis run details
kubectl argo rollouts get analysisrun -n all-chat
```

### ServiceMonitor for Prometheus Scraping

```yaml
# deployments/k8s/monitoring/servicemonitor-innertube.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: youtube-listener-innertube
  namespace: all-chat
  labels:
    app: youtube-listener-innertube
    prometheus: kube-prometheus  # Matches Prometheus selector
spec:
  selector:
    matchLabels:
      app: youtube-listener-innertube
  endpoints:
    - port: http
      path: /metrics
      interval: 1m  # Scrape every minute (matches analysis interval)
      scrapeTimeout: 10s
```

**Note:** Assumes prometheus-operator is installed. If using plain Prometheus, add scrape config to `prometheus.yml` instead.

### Graceful Shutdown Handler (Go)

```go
// services/youtube-listener-innertube/cmd/main.go
func main() {
    // ... initialization ...

    // HTTP server in goroutine
    srv := &http.Server{Addr: ":8080", Handler: router}
    go func() {
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatal("Server failed", zap.Error(err))
        }
    }()

    // Graceful shutdown on SIGTERM (Kubernetes sends this)
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Info("Shutting down gracefully...")

    // Give ongoing requests 25s to complete (5s preStop hook + 20s buffer)
    ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
    defer cancel()

    if err := srv.Shutdown(ctx); err != nil {
        log.Error("Server forced shutdown", zap.Error(err))
    }

    log.Info("Shutdown complete")
}
```

**Source:** Standard Go HTTP server graceful shutdown pattern, combined with Kubernetes best practices ([Google Cloud Blog: Terminating with Grace](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-terminating-with-grace))

### Kustomize Overlay for Production

```yaml
# deployments/k8s/youtube-listener-innertube/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: all-chat

resources:
  - ../base

images:
  - name: allchat/youtube-listener-innertube
    newTag: v1.2.0  # Updated by CI/CD pipeline

patchesStrategicMerge:
  - rollout-patch.yaml
```

```yaml
# deployments/k8s/youtube-listener-innertube/production/rollout-patch.yaml
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: youtube-listener-innertube
spec:
  replicas: 10  # Production scale (10% = 1 pod, 50% = 5 pods)
  template:
    spec:
      containers:
        - name: youtube-listener-innertube
          resources:
            requests:
              cpu: "200m"
              memory: "256Mi"
            limits:
              cpu: "1000m"
              memory: "1Gi"
```

**Usage:**
```bash
# Apply production configuration
kubectl apply -k deployments/k8s/youtube-listener-innertube/production/
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual canary (kubectl scale) | Argo Rollouts / Flagger | 2020-2021 | Automated rollback reduces MTTR from hours to minutes |
| Fixed soak times | Metrics-driven promotion | 2021+ | Rollouts adapt to actual service health, not arbitrary clocks |
| Service mesh required | Native Kubernetes traffic splitting | 2022+ (Argo Rollouts v1.2) | Canary deployments without Istio/Linkerd overhead |
| Separate monitoring + deployment | Integrated AnalysisTemplate | 2020+ (Argo Rollouts GA) | Single source of truth for promotion criteria |
| Blue/green only | Progressive canary (multiple steps) | 2019+ | Gradual rollout reduces blast radius (10% → 50% → 100%) |

**Deprecated/outdated:**
- **Manual replica scaling for canary**: kubectl scale deployment --replicas=1 (canary) + deployment --replicas=9 (stable) → Error-prone, no automatic rollback, requires manual service label updates
- **Spinnaker for Kubernetes canaries**: Spinnaker Netflix OSS tool provided canary analysis but required heavy infrastructure → Argo Rollouts/Flagger are Kubernetes-native, lighter weight
- **Helm hooks for progressive deployment**: Using Helm pre-upgrade/post-upgrade hooks for canary → Not designed for this, Helm doesn't support traffic splitting or automatic rollback

## Open Questions

1. **What is the exact PromQL query for "messages per second per pod"?**
   - What we know: Phase 8 implemented Prometheus metrics, Phase 9-10 added InnerTube-specific metrics
   - What's unclear: Exact metric name (is it `youtube_listener_messages_published_total` or `youtube_streamlist_messages_total`?)
   - Recommendation: Audit existing metrics in `services/youtube-listener/metrics/youtube_metrics.go` and ensure InnerTube exports identical metric names with `service` label

2. **How to handle "fix in place" during canary?**
   - What we know: User wants to deploy fix without aborting entire rollout
   - What's unclear: Does Argo Rollouts support updating canary pods while rollout is paused?
   - Recommendation: Test in staging—likely need to abort, deploy fix to all pods (canary + stable), then restart rollout from 0%

3. **Should official listener run as Rollout too (for consistency)?**
   - What we know: Official listener is stable, no canary needed
   - What's unclear: Will mixing Deployment (official) + Rollout (InnerTube) complicate operations?
   - Recommendation: Keep official listener as Deployment for now—only convert if we need canary analysis for official listener updates

4. **How to compare message rate when both listeners serve different channel sets?**
   - What we know: Canary receives 10% of *traffic*, but traffic is distributed per-channel (different channels have different message rates)
   - What's unclear: If canary gets 10% of channels (not 10% of traffic per channel), message rates won't be directly comparable
   - Recommendation: Clarify with user—likely need to use aggregate message rate across all channels, not per-channel comparison

## Sources

### Primary (HIGH confidence)

- [Argo Rollouts Documentation](https://argoproj.github.io/argo-rollouts/) - Official docs for canary deployment, AnalysisTemplate syntax, and traffic splitting
- [Argo Rollouts Canary Features](https://argo-rollouts.readthedocs.io/en/stable/features/canary/) - Canary strategy configuration, step definitions, and pause behavior
- [Performing Canary Deployments with Prometheus and Flagger (AWS Open Source Blog)](https://aws.amazon.com/blogs/opensource/performing-canary-deployments-and-metrics-driven-rollback-with-amazon-managed-service-for-prometheus-and-flagger/) - Metrics-driven rollback with Prometheus integration
- [Kubernetes Best Practices: Terminating with Grace (Google Cloud Blog)](https://cloud.google.com/blog/products/containers-kubernetes/kubernetes-best-practices-terminating-with-grace) - Graceful shutdown, preStop hooks, terminationGracePeriodSeconds
- [Prometheus Alerting Rules Documentation](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) - PromQL syntax for metrics-based alerting

### Secondary (MEDIUM confidence)

- [Flagger vs Argo Rollouts for Progressive Delivery (Buoyant Blog)](https://www.buoyant.io/blog/flagger-vs-argo-rollouts-for-progressive-delivery-on-linkerd) - Tool comparison, architectural differences
- [Automating Canary Analysis with Prometheus + Argo Rollouts (Medium)](https://medium.com/@tuteja_lovish/automating-canary-analysis-in-spring-boot-using-prometheus-argo-rollouts-code-thresholds-f56760c8ff9b) - Real-world thresholds and PromQL examples
- [Practical Progressive Delivery with Argo Rollouts (PongZT)](https://pongzt.com/post/argo-rollouts-metric/) - Metrics-based auto rollback implementation
- [Canary Deployment Best Practices (Octopus Deploy)](https://octopus.com/devops/software-deployments/canary-deployment/) - Common issues checklist, monitoring requirements
- [Google SRE Workbook: Canarying Releases](https://sre.google/workbook/canarying-releases/) - Best practices from Google's SRE team

### Tertiary (LOW confidence - requires verification)

- [Kubernetes Deployment Strategies Comparison (LaunchDarkly)](https://launchdarkly.com/blog/kubernetes-deployment-strategies/) - General overview of canary, blue-green, rolling update strategies
- [Grafana Dashboard for Flagger Canary Status](https://grafana.com/grafana/dashboards/15158-flagger-canary-status/) - Pre-built dashboard (Flagger-specific, needs adaptation for Argo Rollouts)

## Metadata

**Confidence breakdown:**
- Argo Rollouts architecture: HIGH - Official documentation with production examples
- Prometheus metrics integration: HIGH - Verified with AnalysisTemplate examples from official docs
- Graceful shutdown patterns: HIGH - Kubernetes official best practices (Google Cloud Blog)
- Grafana dashboard specifics: MEDIUM - Dashboard structure is Claude's discretion per CONTEXT.md, layout not dictated by sources
- Message rate comparison queries: MEDIUM - Requires verification against actual metric names in codebase
- Troubleshooting guide content: MEDIUM - Drawn from real-world canary issues (AWS blog, SRE workbook) but needs adaptation to InnerTube specifics

**Research date:** 2026-02-24
**Valid until:** 2026-03-24 (30 days - Argo Rollouts is stable, but new versions may introduce features like improved traffic splitting)
