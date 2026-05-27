# Fixing Service Issues in Production

This runbook covers the workflow for diagnosing and fixing service issues in the Kubernetes cluster.

## Quick Reference

**When to use**: Service pods are unhealthy, failing readiness/liveness probes, or crashing repeatedly.

**Key principle**: Build locally → Commit → Push → Let CI/CD deploy automatically.

## Workflow

### 1. Investigate the Issue

Check pod status and events:

```bash
# Verify kubectl context
kubectl config current-context

# List pods for the service
kubectl -n allchat get pods -l app=<service-name> -o wide

# Describe pods to see events
kubectl -n allchat describe pod <pod-name>

# Check recent events
kubectl -n allchat get events --sort-by=.lastTimestamp --field-selector involvedObject.name=<pod-name>

# View pod logs
kubectl -n allchat logs <pod-name> --tail=100
kubectl -n allchat logs <pod-name> --previous  # For crashed pods
```

Check for autoscaling issues:

```bash
# Check HPA status
kubectl -n allchat get hpa

# Describe HPA
kubectl -n allchat describe hpa <service-name>-hpa

# Check resource usage
kubectl -n allchat top pods -l app=<service-name>
```

### 2. Identify Root Cause

Common issues:

- **JSON unmarshaling errors**: API response format changed
- **Connection failures**: Database, Redis, external APIs
- **Configuration issues**: Missing environment variables, wrong secrets
- **Resource constraints**: OOMKilled, CPU throttling
- **Application bugs**: Panics, deadlocks, logic errors

### 3. Fix the Code Locally

Make necessary changes to the codebase.

### 4. Build Locally to Verify

**Always build locally first** to catch compilation errors before CI/CD:

```bash
# For Go services
cd services/<service-name>
go build -o /tmp/<service-name>-test ./cmd

# Or with race detection
go build -race -o /tmp/<service-name>-test ./cmd
```

If the build fails, fix errors and repeat until successful.

### 5. Commit and Push

```bash
# Stage changes
git add <files>

# Commit with descriptive message
git commit -m "fix(<service>): Brief description of fix

Detailed explanation of:
- What was broken
- Why it was broken
- How the fix works
- Any side effects or considerations

Co-Authored-By: Claude <noreply@anthropic.com>"

# Push to trigger CI/CD
git push
```

### 6. Monitor Deployment

The CI/CD pipeline (GitHub Actions + Keel) will:

1. Build Docker image
2. Push to container registry (ghcr.io)
3. Keel automatically updates Kubernetes deployment
4. Rolling update replaces pods

Monitor the rollout:

```bash
# Watch deployment status
kubectl -n allchat rollout status deployment/<service-name>

# Watch pods
watch kubectl -n allchat get pods -l app=<service-name>

# Check new pod logs
kubectl -n allchat logs -f deployment/<service-name>
```

### 7. Verify Fix

```bash
# Check pod health
kubectl -n allchat get pods -l app=<service-name>

# Verify readiness probe
kubectl -n allchat exec deployment/<service-name> -- wget -qO- http://localhost:8080/health/ready

# Check events (should have no unhealthy warnings)
kubectl -n allchat get events --sort-by=.lastTimestamp | grep <service-name>

# Monitor metrics/logs
kubectl -n allchat logs -f deployment/<service-name>
```

## Example: Kick Listener Issue (Jan 2026)

### Symptoms
- Pods failing readiness probes with 503 errors
- HPA cycling pods every 3-5 minutes
- Logs showing JSON unmarshaling errors
- 0 active subscriptions

### Investigation
```bash
kubectl -n allchat get pods -l app=kick-listener
kubectl -n allchat logs kick-listener-xxx --tail=100 | grep error
```

### Root Cause
Two JSON parsing issues:
1. Pusher WebSocket sends connection data as double-encoded JSON string
2. Kick API changed `playback_url` from object to string

### Fix Applied
- Modified `websocket/client.go` to handle both string-encoded and direct JSON
- Changed `playback_url` field type to `json.RawMessage` for flexibility

### Verification
```bash
# After CI/CD completed
kubectl -n allchat get pods -l app=kick-listener
# All pods now show 1/1 READY

kubectl -n allchat logs -f deployment/kick-listener
# No more JSON unmarshaling errors
# WebSocket connection established successfully
# Channels subscribed properly
```

## Best Practices

1. **Always build locally first** - Catches errors before CI/CD
2. **Write descriptive commit messages** - Helps with debugging future issues
3. **Monitor during deployment** - Catch issues early in rollout
4. **Check logs after fix** - Verify the issue is actually resolved
5. **Document the fix** - Update troubleshooting guides if it's a common issue

## Rollback

If the fix makes things worse:

```bash
# Rollback deployment
kubectl -n allchat rollout undo deployment/<service-name>

# Check rollback status
kubectl -n allchat rollout status deployment/<service-name>

# Or rollback to specific revision
kubectl -n allchat rollout history deployment/<service-name>
kubectl -n allchat rollout undo deployment/<service-name> --to-revision=<N>
```

## Related Documentation

- [Kubernetes Debug Guide](../../llm-guides/QUICK-REF-KUBERNETES-DEBUG.md)
- [Troubleshooting Decision Tree](../../troubleshooting/decision-tree.md)
- [Deployment Guide](../../DEPLOYMENT.md)
