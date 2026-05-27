# Troubleshooting Decision Tree

This guide helps you quickly diagnose and resolve common All-Chat issues through a structured decision tree approach.

---

## Quick Triage

**Start here**: What is the primary symptom?

```
┌─ A. Build or startup failure
├─ B. Service can't connect to database/Redis
├─ C. Messages not appearing in overlay
├─ D. Specific platform not working (Twitch/YouTube/Kick/TikTok)
├─ E. Performance issues (high CPU/memory, slow response)
└─ F. Deployment or Kubernetes issues
```

---

## A. Build or Startup Failure

### Symptom Tree

```
Build or startup fails?
│
├─ Go compilation error?
│  ├─ "undefined: <package>" → Missing import
│  │  Solution: go mod tidy && go mod download
│  │  Guide: build-errors.md#missing-imports
│  │
│  ├─ "module not found" → Dependency issue
│  │  Solution: Check go.mod replace directives
│  │  Guide: build-errors.md#module-resolution
│  │
│  └─ Type mismatch or interface error → Code issue
│     Solution: Check recent code changes
│     Guide: build-errors.md#type-errors
│
├─ Docker build fails?
│  ├─ "COPY failed" → File path issue
│  │  Solution: Check Dockerfile COPY paths relative to context
│  │  Guide: build-errors.md#docker-copy-errors
│  │
│  ├─ "go: not found" → Base image issue
│  │  Solution: Verify Dockerfile uses golang:1.23 or later
│  │  Guide: build-errors.md#docker-base-image
│  │
│  └─ Build context too large → .dockerignore missing
│     Solution: Add .dockerignore with node_modules, .git
│     Guide: build-errors.md#docker-context
│
└─ Service crashes on startup?
   ├─ "connection refused" → Database/Redis not ready
   │  Solution: Check DATABASE_HOST and REDIS_HOST env vars
   │  Guide: connection-errors.md#startup-connection-failures
   │
   ├─ "panic: environment variable not set" → Missing config
   │  Solution: Check .env file or Kubernetes secret
   │  Guide: build-errors.md#missing-env-vars
   │
   └─ "port already in use" → Port conflict
      Solution: Change PORT env var or stop conflicting service
      Guide: build-errors.md#port-conflicts
```

**→ See**: [build-errors.md](./build-errors.md)

---

## B. Connection Errors (Database/Redis)

### Symptom Tree

```
Service can't connect to dependencies?
│
├─ PostgreSQL connection fails?
│  ├─ "connection refused" → Database not running
│  │  Solution: docker-compose up postgres OR check k8s pod
│  │  Guide: connection-errors.md#postgres-not-running
│  │
│  ├─ "authentication failed" → Wrong credentials
│  │  Solution: Check DATABASE_USER and DATABASE_PASSWORD
│  │  Guide: connection-errors.md#postgres-auth
│  │
│  ├─ "database does not exist" → Missing schema
│  │  Solution: Run migrations: make migrate
│  │  Guide: connection-errors.md#postgres-schema-missing
│  │
│  └─ "timeout" → Network or firewall issue
│     Solution: Check DATABASE_HOST resolves correctly
│     Guide: connection-errors.md#postgres-timeout
│
└─ Redis connection fails?
   ├─ "connection refused" → Redis not running
   │  Solution: docker-compose up redis OR check k8s pod
   │  Guide: connection-errors.md#redis-not-running
   │
   ├─ "NOAUTH Authentication required" → Wrong password
   │  Solution: Check REDIS_PASSWORD env var
   │  Guide: connection-errors.md#redis-auth
   │
   └─ "timeout" → Network or firewall issue
      Solution: Check REDIS_HOST resolves correctly
      Guide: connection-errors.md#redis-timeout
```

**→ See**: [connection-errors.md](./connection-errors.md)

---

## C. Messages Not Appearing in Overlay

### Symptom Tree

```
Messages not showing up?
│
├─ No messages from ANY platform?
│  ├─ WebSocket not connected?
│  │  Solution: Check browser console for WebSocket errors
│  │  Guide: websocket-disconnects.md#client-side-issues
│  │
│  ├─ API Gateway not publishing?
│  │  Solution: Check Redis Pub/Sub: PUBSUB CHANNELS overlay:*
│  │  Guide: QUICK-REF-REDIS-OPERATIONS.md#pubsub-debug
│  │
│  └─ Message Processor not running?
│     Solution: kubectl get pods -l app=message-processor
│     Guide: connection-errors.md#service-discovery
│
├─ No messages from SPECIFIC platform?
│  ├─ Twitch messages missing?
│  │  Solution: Check IRC connection and channel joins
│  │  Guide: twitch-irc-issues.md#no-messages
│  │
│  ├─ YouTube messages missing?
│  │  Solution: Check quota and OAuth token validity
│  │  Guide: youtube-quota-exceeded.md OR youtube-listener/README.md
│  │
│  ├─ Kick messages missing?
│  │  Solution: Check WebSocket connection and subscriptions
│  │  Guide: kick-listener/README.md#troubleshooting
│  │
│  └─ TikTok messages missing?
│     Solution: Check TikTok Listener status
│     Guide: tiktok-listener/README.md#troubleshooting
│
└─ Messages arrive delayed (>5 seconds)?
   ├─ Redis Streams backlog high?
   │  Solution: Check XPENDING chat:raw message-processors
   │  Guide: QUICK-REF-REDIS-OPERATIONS.md#streams-backlog
   │
   ├─ Message Processor slow?
   │  Solution: Check CPU/memory limits, scale up replicas
   │  Guide: QUICK-REF-SCALING.md#message-processor
   │
   └─ Network latency?
      Solution: Check service-to-service latency with kubectl exec
      Guide: QUICK-REF-KUBERNETES-DEBUG.md#network-latency
```

**→ See**: Platform-specific guides below

---

## D. Platform-Specific Issues

### D1. Twitch Issues

```
Twitch not working?
│
├─ Bot not joining channels?
│  ├─ OAuth token invalid? → Get new token from twitchapps.com/tmi
│  ├─ Channel name wrong? → Check spelling and case sensitivity
│  └─ Rate limit hit? → Max 20 JOIN per 10 seconds, check logs
│
├─ Messages not received after joining?
│  ├─ Channel actually has chat? → Verify on Twitch website
│  ├─ IRC connection dropped? → Check logs for reconnection
│  └─ Redis publish failing? → Check Redis connection
│
└─ Emotes not displaying?
   ├─ Twitch emotes → Parsed from IRC tags, check normalizer
   ├─ 7TV/BTTV/FFZ → Enriched by Message Processor, check logs
   └─ User-specific emotes → Check 7TV EventAPI WebSocket connection
```

**→ See**: [twitch-irc-issues.md](./twitch-irc-issues.md)

### D2. YouTube Issues

```
YouTube not working?
│
├─ Quota exceeded? (HTTP 403)
│  Solution: Check /quota/status, wait for midnight PT reset
│  Guide: QUICK-REF-DEBUG-QUOTA.md
│
├─ OAuth token expired or invalid?
│  ├─ Check token in database: SELECT * FROM youtube_oauth_tokens
│  ├─ Re-authorize through auth-service OAuth flow
│  └─ Verify token refresh is working (check logs)
│
├─ No live streams found?
│  ├─ Channel actually live? → Verify on YouTube website
│  ├─ OAuth scopes correct? → Must include youtube.readonly
│  └─ Search API failing? → Check quota and API key
│
└─ Polling not happening?
   ├─ Leader election failed? → Check Source Manager status
   ├─ Quota in DEPLETED state? → All polling stops, wait for reset
   └─ Video cache stale? → Check logs for cache clearing events
```

**→ See**: [youtube-quota-exceeded.md](./youtube-quota-exceeded.md)

### D3. Kick Issues

```
Kick not working?
│
├─ WebSocket connection fails?
│  ├─ Network firewall blocking wss://? → Check corporate firewall
│  ├─ Pusher URL changed? → Kick may update app key
│  └─ Too many connections? → Check connection pooling
│
├─ Chatroom ID not found?
│  ├─ Channel slug wrong? → Verify on kick.com
│  ├─ API call failing? → Check https://kick.com/api/v2/channels/{slug}
│  └─ Metadata not saved? → Check database metadata JSONB field
│
└─ Subscribed but no messages?
   ├─ Channel actually has chat? → Verify on Kick website
   ├─ Subscription confirmed? → Check /status endpoint
   └─ Message parsing error? → Check logs for JSON errors
```

**→ See**: [kick-listener/README.md#troubleshooting](../../services/kick-listener/README.md)

### D4. TikTok Issues

```
TikTok not working?
│
├─ Library installation failed?
│  Solution: Check TikTokLiveRust dependencies
│  Guide: tiktok-listener/README.md#installation
│
├─ Username not resolving?
│  ├─ TikTok account exists? → Verify on tiktok.com
│  ├─ Account must be live? → TikTok Live API requires active stream
│  └─ Rate limited? → Check logs for API errors
│
└─ Connection unstable?
   Solution: Unofficial library, TikTok may block or change API
   Guide: tiktok-listener/README.md#known-limitations
```

**→ See**: [tiktok-listener/README.md](../../services/tiktok-listener/README.md)

---

## E. Performance Issues

### Symptom Tree

```
Service slow or high resource usage?
│
├─ High CPU usage?
│  ├─ Message Processor overloaded?
│  │  Solution: Scale up replicas, check emote enrichment latency
│  │  Guide: QUICK-REF-SCALING.md#message-processor
│  │
│  ├─ Listener in tight loop?
│  │  Solution: Check logs for error retry storms
│  │  Guide: QUICK-REF-KUBERNETES-DEBUG.md#cpu-profiling
│  │
│  └─ Redis CPU high?
│     Solution: Check SLOWLOG, consider Redis cluster
│     Guide: QUICK-REF-REDIS-OPERATIONS.md#performance
│
├─ High memory usage?
│  ├─ Memory leak in service?
│  │  Solution: Check for goroutine leaks, connection leaks
│  │  Guide: QUICK-REF-KUBERNETES-DEBUG.md#memory-profiling
│  │
│  ├─ Redis memory high?
│  │  Solution: Check stream trimming, set MAXMEMORY policy
│  │  Guide: QUICK-REF-REDIS-OPERATIONS.md#memory-management
│  │
│  └─ PostgreSQL memory?
│     Solution: Check connection pooling, query performance
│     Guide: QUICK-REF-DATABASE-MIGRATION.md#query-optimization
│
└─ Slow API responses?
   ├─ Database slow?
   │  Solution: Check query plans, add indexes
   │  Guide: QUICK-REF-DATABASE-MIGRATION.md#performance
   │
   ├─ Redis slow?
   │  Solution: Check SLOWLOG, network latency
   │  Guide: QUICK-REF-REDIS-OPERATIONS.md#slowlog
   │
   └─ External API latency?
      Solution: Check YouTube/Kick/TikTok API response times
      Guide: Platform-specific README files
```

**→ See**: [QUICK-REF-SCALING.md](../llm-guides/QUICK-REF-SCALING.md)

---

## F. Deployment and Kubernetes Issues

### Symptom Tree

```
Kubernetes deployment problems?
│
├─ Pod not starting?
│  ├─ ImagePullBackOff?
│  │  Solution: Check image exists, registry auth correct
│  │  Guide: QUICK-REF-KUBERNETES-DEBUG.md#image-pull-errors
│  │
│  ├─ CrashLoopBackOff?
│  │  Solution: Check logs: kubectl logs <pod> --previous
│  │  Guide: QUICK-REF-KUBERNETES-DEBUG.md#crashloop
│  │
│  ├─ Pending (no resources)?
│  │  Solution: Check node resources, adjust limits
│  │  Guide: QUICK-REF-SCALING.md#resource-limits
│  │
│  └─ Init container failed?
│     Solution: Check init logs, usually DB migration
│     Guide: QUICK-REF-DATABASE-MIGRATION.md#k8s-migrations
│
├─ Service not reachable?
│  ├─ Service selector wrong?
│  │  Solution: Check labels match: kubectl get svc,pods --show-labels
│  │  Guide: QUICK-REF-KUBERNETES-DEBUG.md#service-discovery
│  │
│  ├─ Network policy blocking?
│  │  Solution: Check NetworkPolicies in namespace
│  │  Guide: QUICK-REF-KUBERNETES-DEBUG.md#network-policies
│  │
│  └─ Ingress not working?
│     Solution: Check Ingress controller logs
│     Guide: QUICK-REF-KUBERNETES-DEBUG.md#ingress-debug
│
└─ ConfigMap or Secret missing?
   ├─ Secret not created?
   │  Solution: kubectl create secret ... OR check sealed-secrets
   │  Guide: QUICK-REF-KUBERNETES-DEBUG.md#secrets
   │
   └─ Wrong namespace?
      Solution: Verify namespace: kubectl get secrets -n allchat
      Guide: QUICK-REF-KUBERNETES-DEBUG.md#namespace-issues
```

**→ See**: [QUICK-REF-KUBERNETES-DEBUG.md](../llm-guides/QUICK-REF-KUBERNETES-DEBUG.md)

---

## Quick Command Reference

### Health Checks

```bash
# Check all services
kubectl get pods -n allchat

# Check specific service logs
kubectl logs -n allchat -l app=<service-name> --tail=100 -f

# Check health endpoints
kubectl exec -n allchat deployment/<service> -- \
  wget -qO- http://localhost:<port>/health/ready
```

### Database Checks

```bash
# Access database
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat

# Check active overlays
SELECT id, name, is_active FROM overlays WHERE is_active = true;

# Check active chat sources
SELECT overlay_id, platform, channel_identifier, is_active
FROM overlay_chat_sources
WHERE is_active = true;
```

### Redis Checks

```bash
# Access Redis
kubectl exec -n allchat deployment/redis -- redis-cli

# Check Streams
XINFO STREAM chat:raw

# Check Pub/Sub
PUBSUB CHANNELS overlay:*

# Check consumer groups
XINFO GROUPS chat:raw
```

### Message Flow Tracing

```bash
# 1. Check listener published to stream
redis-cli XREAD COUNT 10 STREAMS chat:raw 0

# 2. Check processor consumed from stream
redis-cli XINFO GROUPS chat:raw

# 3. Check pub/sub published to overlay
redis-cli PUBSUB CHANNELS overlay:*

# 4. Check API Gateway subscribed
# View WebSocket connections in API Gateway logs
kubectl logs -n allchat -l app=api-gateway | grep WebSocket
```

---

## Escalation Path

If the issue is not resolved by following the decision tree:

1. **Gather diagnostic information**:
   - Service logs (last 500 lines)
   - `/health/ready` and `/status` endpoints
   - Database query results (active overlays, sources)
   - Redis info (stream length, pub/sub channels)

2. **Check documentation**:
   - Service-specific README files
   - Architecture documentation (docs/architecture/)
   - Quick reference guides (docs/llm-guides/)

3. **Review recent changes**:
   - Git log for recent commits
   - Recent deployments or configuration changes
   - Database migrations applied

4. **File GitHub issue**:
   - Use issue template
   - Include diagnostic output
   - Describe steps to reproduce

---

## Related Guides

### Quick Reference Cards
- [QUICK-REF-ADD-PLATFORM.md](../llm-guides/QUICK-REF-ADD-PLATFORM.md) - Add new platform support
- [QUICK-REF-DEBUG-QUOTA.md](../llm-guides/QUICK-REF-DEBUG-QUOTA.md) - YouTube quota debugging
- [QUICK-REF-KUBERNETES-DEBUG.md](../llm-guides/QUICK-REF-KUBERNETES-DEBUG.md) - K8s troubleshooting
- [QUICK-REF-REDIS-OPERATIONS.md](../llm-guides/QUICK-REF-REDIS-OPERATIONS.md) - Redis debugging

### Detailed Troubleshooting Guides
- [build-errors.md](./build-errors.md) - Build and compilation issues
- [connection-errors.md](./connection-errors.md) - Database and Redis connection issues
- [twitch-irc-issues.md](./twitch-irc-issues.md) - Twitch IRC specific issues
- [youtube-quota-exceeded.md](./youtube-quota-exceeded.md) - YouTube API quota issues
- [websocket-disconnects.md](./websocket-disconnects.md) - WebSocket connection issues

### Architecture Documentation
- [DATA_FLOW_INTEGRATION.md](../architecture/01-DATA-FLOW.md) - Message flow architecture
- [DEPLOYMENT_KUBERNETES.md](../architecture/02-DEPLOYMENT.md) - Kubernetes deployment
- [SECURITY_ARCHITECTURE.md](../architecture/05-SECURITY.md) - Security considerations

---

## Tips for Effective Troubleshooting

1. **Start with health checks**: Always check `/health/ready` first
2. **Follow the data flow**: Trace messages from listener → Redis Stream → processor → pub/sub → WebSocket
3. **Check logs with context**: Use `kubectl logs --tail=500` for sufficient context
4. **One change at a time**: Restart one service, verify, then move to next
5. **Document findings**: Note what you tried and what happened
6. **Check recent changes**: Issues often correlate with recent deployments or config changes

**Remember**: Most issues fall into one of the categories above. Use this decision tree to quickly narrow down the problem domain, then refer to the specific detailed guide.
