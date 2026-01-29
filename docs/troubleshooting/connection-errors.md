# Troubleshooting: Connection Errors

PostgreSQL and Redis connection issues with diagnostic steps.

---

## PostgreSQL Connection Errors

### Connection Refused

**Symptom**:
```
dial tcp [::1]:5432: connect: connection refused
```

**Diagnosis**:
```bash
# Check PostgreSQL is running
docker-compose ps postgres
# OR
kubectl get pods -n allchat -l app=postgres

# Test connection
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat
```

**Solutions**:
1. **Local dev**: `docker-compose up postgres -d`
2. **Kubernetes**: `kubectl get pods -n allchat` (check postgres pods running)
3. **Check DATABASE_HOST**: Should be `localhost` (dev) or `allchat-cluster-rw` (k8s)

### Authentication Failed

**Symptom**:
```
FATAL: password authentication failed for user "allchat"
```

**Solution**:
```bash
# Check credentials
echo $DATABASE_USER
echo $DATABASE_PASSWORD

# Test with correct password
psql postgresql://allchat:CORRECT_PASSWORD@localhost:5432/allchat
```

### Database Does Not Exist

**Symptom**:
```
FATAL: database "allchat" does not exist
```

**Solution**:
```bash
# Create database
createdb allchat

# Run migrations
make migrate-up
```

### Timeout

**Symptom**:
```
dial tcp 10.0.0.1:5432: i/o timeout
```

**Solutions**:
1. Check firewall rules
2. Verify DATABASE_HOST resolves correctly: `ping $DATABASE_HOST`
3. Check Kubernetes NetworkPolicy allows connection

---

## Redis Connection Errors

### Connection Refused

**Symptom**:
```
dial tcp [::1]:6379: connect: connection refused
```

**Diagnosis**:
```bash
# Check Redis is running
docker-compose ps redis
# OR
kubectl get pods -n allchat -l app=redis

# Test connection
redis-cli ping
# Expected: PONG
```

**Solutions**:
1. **Local dev**: `docker-compose up redis -d`
2. **Kubernetes**: Check Redis pod running
3. **Check REDIS_HOST**: Should be `localhost` (dev) or `redis` (k8s)

### NOAUTH Authentication Required

**Symptom**:
```
NOAUTH Authentication required
```

**Solution**: Set REDIS_PASSWORD env var (if Redis configured with password)

### Timeout

**Symptom**:
```
dial tcp 10.0.0.2:6379: i/o timeout
```

**Solutions**:
1. Check firewall rules
2. Verify REDIS_HOST resolves: `ping $REDIS_HOST`
3. Check Kubernetes NetworkPolicy

---

## Health Check Failures

### /health/ready Returns 503

**Symptom**: Readiness probe fails, pod not ready

**Diagnosis**:
```bash
# Check health endpoint directly
kubectl exec -n allchat deployment/<service> -- wget -qO- http://localhost:<port>/health/ready

# Check logs
kubectl logs -n allchat deployment/<service> --tail=100
```

**Common Causes**:
1. Database connection pool exhausted
2. Redis connection failed
3. Dependency service (emote-service, youtube-listener) unreachable

**File**: `services/*/handlers/health.go`

---

## Related Documentation

- [build-errors.md](./build-errors.md) - Build and compilation issues
- [QUICK-REF-KUBERNETES-DEBUG.md](../llm-guides/QUICK-REF-KUBERNETES-DEBUG.md) - K8s network debugging
