# Troubleshooting Guides

Structured diagnostic guides for common All-Chat issues.

---

## Quick Start

**Having issues?** Start with the [Decision Tree](./decision-tree.md) for high-level triage.

---

## Available Guides

| Issue Category | Guide | Est. Time |
|----------------|-------|-----------|
| **Build/compilation errors** | [build-errors.md](./build-errors.md) | 5-15 min |
| **Database/Redis connections** | [connection-errors.md](./connection-errors.md) | 10-20 min |
| **YouTube quota exhausted** | [youtube-quota-exceeded.md](./youtube-quota-exceeded.md) | 10-30 min |
| **Twitch IRC issues** | [twitch-irc-issues.md](./twitch-irc-issues.md) | 10-20 min |
| **WebSocket disconnects** | [websocket-disconnects.md](./websocket-disconnects.md) | 10-20 min |

---

## Triage Process

1. **Identify symptom** (error message, unexpected behavior)
2. **Check decision tree** for category
3. **Follow specific guide** for detailed diagnostics
4. **Escalate** if issue persists (file GitHub issue)

---

## Quick Command Reference

### Health Checks
```bash
# All services
kubectl get pods -n allchat

# Specific service logs
kubectl logs -n allchat deployment/<service> --tail=100 -f

# Health endpoints
curl http://localhost:<port>/health/ready
```

### Database Checks
```bash
# Access database
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat

# Check active overlays
SELECT id, name, is_active FROM overlays WHERE is_active = true;
```

### Redis Checks
```bash
# Access Redis
redis-cli

# Check streams
XINFO STREAM chat:raw

# Check pub/sub
PUBSUB CHANNELS overlay:*
```

---

## Related Documentation

- [CLAUDE.md](../../CLAUDE.md) - Project overview and navigation
- [Service READMEs](../../services/) - Service-specific troubleshooting
- [Architecture docs](../architecture/) - System architecture reference
