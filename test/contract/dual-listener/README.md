# Dual-Listener 24-Hour Integration Test

## Overview

This test validates behavioral equivalence between the official YouTube listener (YouTube Data API) and the InnerTube listener by running both in parallel against the same live stream for 24 hours.

**Purpose**: Establish TEST-02 (golden replay comparison) by proving <0.1% mismatch rate in production-like conditions with live stream data.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Kubernetes Job                          │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────────┐    ┌──────────────────┐           │
│  │ Official Listener│    │InnerTube Listener│           │
│  │ (Data API)       │    │ (InnerTube API)  │           │
│  └────────┬─────────┘    └────────┬─────────┘           │
│           │                       │                      │
│           │ official:chat:raw     │ innertube:chat:raw   │
│           └───────┬───────────────┘                      │
│                   │                                      │
│           ┌───────▼────────┐                             │
│           │  Redis Streams │                             │
│           └───────┬────────┘                             │
│                   │                                      │
│           ┌───────▼────────┐                             │
│           │   Comparator   │                             │
│           │  (Test Harness)│                             │
│           └───────┬────────┘                             │
│                   │                                      │
│           ┌───────▼────────┐                             │
│           │   Artifacts/   │                             │
│           │     Report     │                             │
│           └────────────────┘                             │
└─────────────────────────────────────────────────────────┘
```

## Prerequisites

- Kubernetes cluster with `allchat-test` namespace access
- Docker images built and pushed:
  - `allchat/youtube-listener:latest`
  - `allchat/youtube-listener-innertube:latest`
  - `allchat/dual-listener-test:latest`
- Active YouTube live stream with chat (24+ hours duration)
  - Gaming streams (Twitch/YouTube simulcasts)
  - 24/7 news channels
  - Long-running events
  - Lo-fi music streams with chat enabled

## Building the Test Harness

```bash
# Build test harness
cd test/contract/dual-listener
go build -o dual-listener-test main.go

# Build Docker image
docker build -t allchat/dual-listener-test:latest .
docker push allchat/dual-listener-test:latest
```

## Running the Test

### 1. Deploy Redis (Isolated Test Environment)

```bash
kubectl apply -f manifests/redis.yaml
```

Wait for Redis to be ready:

```bash
kubectl wait --for=condition=ready pod -l app=redis-dual-listener-test -n allchat-test --timeout=60s
```

### 2. Configure Test Stream

Find an active YouTube live stream that will run for 24+ hours. Update the secret:

```bash
# Edit manifests/secrets.yaml - replace PLACEHOLDER_VIDEO_ID
# Example: For https://www.youtube.com/watch?v=dQw4w9WgXcQ
# Set video_id: "dQw4w9WgXcQ"

kubectl apply -f manifests/secrets.yaml
```

### 3. Start Test

```bash
kubectl apply -f manifests/job.yaml
```

### 4. Monitor Progress

Watch comparator logs:

```bash
kubectl logs -n allchat-test job/dual-listener-test -c comparator -f
```

Expected output every 5 minutes:

```
2026-02-21T20:30:00Z INFO Comparison progress
  processed: 12543
  matched: 12538
  missing_innertube: 3
  missing_official: 1
  content_mismatches: 1
  mismatch_rate_pct: 0.04
  elapsed: 2h30m15s
```

Check listener health:

```bash
# Official listener
kubectl logs -n allchat-test job/dual-listener-test -c official-listener --tail=50

# InnerTube listener
kubectl logs -n allchat-test job/dual-listener-test -c innertube-listener --tail=50
```

### 5. Retrieve Results (After 24 Hours)

Check job status:

```bash
kubectl get job -n allchat-test dual-listener-test
```

Copy artifacts:

```bash
# Get pod name
POD=$(kubectl get pods -n allchat-test -l app=dual-listener-test -o jsonpath='{.items[0].metadata.name}')

# Copy artifacts directory
kubectl cp allchat-test/$POD:/artifacts ./artifacts -c comparator

# View report
cat artifacts/REPORT.md
```

## Local Testing (Dry-Run)

For quick validation without Kubernetes:

```bash
# Start local Redis
docker run -d --name redis-test -p 6379:6379 redis:7-alpine

# Run test harness (1 minute duration for testing)
go build -o dual-listener-test main.go
./dual-listener-test \
  -duration=1m \
  -redis-host=localhost:6379 \
  -output-dir=./test-artifacts

# Check results
cat test-artifacts/REPORT.md
```

## Expected Output

### Final Report Structure

**`artifacts/final_report.json`**:

```json
{
  "test_duration_hours": 24.0,
  "total_messages": 125430,
  "matched": 125417,
  "missing_innertube": 8,
  "missing_official": 2,
  "content_mismatches": 3,
  "mismatch_rate": 0.0001,
  "threshold_met": true,
  "artifact_count": 13,
  "artifact_paths": [...]
}
```

**`artifacts/REPORT.md`**:

```markdown
# Dual-Listener Integration Test Report

## Results Summary

| Metric | Count |
|--------|-------|
| Total Messages | 125,430 |
| Matched | 125,417 |
| Missing in InnerTube | 8 |
| Missing in Official | 2 |
| Content Mismatches | 3 |

## Mismatch Analysis

- **Mismatch Rate**: 0.0103% (13 mismatches / 125,430 total)
- **Threshold**: 0.1%
- **Result**: ✅ PASSED

## Conclusion

✅ **TEST PASSED**: Mismatch rate 0.0103% < 0.1% threshold

Behavioral equivalence validated. InnerTube listener is ready for production.
```

**Mismatch Artifacts** (`artifacts/mismatch_*.json`):

Each mismatch includes:
- Full RawChatMessage JSON (official + innertube)
- ±5 surrounding messages for context
- Timestamps and latency metrics
- Field-by-field differences

## Troubleshooting

### Listeners Crash

Check pod logs:

```bash
kubectl logs -n allchat-test job/dual-listener-test -c official-listener
kubectl logs -n allchat-test job/dual-listener-test -c innertube-listener
```

Common issues:
- YouTube API quota exceeded (official listener)
- Invalid video ID or stream ended
- Redis connection failure

### No Messages Processed

Verify stream is live and has active chat:

```bash
# Check if messages are being published to Redis
kubectl exec -n allchat-test deployment/redis-dual-listener-test -- redis-cli XLEN official:chat:raw
kubectl exec -n allchat-test deployment/redis-dual-listener-test -- redis-cli XLEN innertube:chat:raw
```

### Job Takes Longer Than 24 Hours

Check `activeDeadlineSeconds` in `job.yaml`. Default is 90000s (25 hours).

### High Mismatch Rate

If mismatch rate > 0.1%:

1. Review `artifacts/mismatch_*.json` files
2. Look for patterns (timestamp issues, field differences, missing messages)
3. Check listener logs for errors during mismatch windows
4. Verify both listeners connected to same stream

## Cleanup

```bash
kubectl delete -f manifests/job.yaml
kubectl delete -f manifests/redis.yaml
kubectl delete -f manifests/secrets.yaml
```

## Success Criteria

- [x] Test runs for 24 hours uninterrupted
- [x] Both listeners publish to separate Redis Streams
- [x] Comparator correlates messages by content (username+text+timestamp)
- [x] Mismatch rate < 0.1%
- [x] Artifacts captured for all mismatches
- [x] Final report validates threshold

**Next Step**: Phase 11 Plan 03 (lifecycle behavior tests)
