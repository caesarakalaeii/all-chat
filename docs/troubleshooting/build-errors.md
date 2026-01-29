# Troubleshooting: Build Errors

Common build, compilation, and startup errors with solutions.

---

## Go Compilation Errors

### Missing Imports

**Symptom**:
```
undefined: gin
undefined: zap
```

**Solution**:
```bash
go mod tidy
go mod download
```

### Module Not Found

**Symptom**:
```
go: module github.com/caesarakalaeii/all-chat/shared not found
```

**Solution**:
```bash
# Check go.mod has replace directive
cat go.mod | grep replace

# Should have:
# replace github.com/caesarakalaeii/all-chat/shared => ../../shared

# If missing, add it:
go mod edit -replace github.com/caesarakalaeii/all-chat/shared=../../shared
```

### Type Errors

**Symptom**:
```
cannot use db (type *pgxpool.Pool) as type *sql.DB
```

**Solution**: Check imports use correct types (pgx vs database/sql)

---

## Docker Build Errors

### COPY Failed

**Symptom**:
```
COPY failed: file not found in build context
```

**Solution**: Check Dockerfile COPY paths relative to build context
```dockerfile
# Build from repository root
COPY services/api-gateway/ /app/
COPY shared/ /app/shared/
```

### Build Context Too Large

**Symptom**:
```
Sending build context to Docker daemon: 2.5GB
```

**Solution**: Add `.dockerignore`
```
node_modules/
.git/
*.md
vendor/
```

---

## Startup Errors

### Port Already in Use

**Symptom**:
```
bind: address already in use
```

**Solution**:
```bash
# Find process using port
lsof -i :8080

# Change PORT env var
export PORT=8081
```

### Missing Environment Variable

**Symptom**:
```
panic: TWITCH_BOT_USERNAME environment variable not set
```

**Solution**: Check `.env` file or Kubernetes secret has required variables

---

## Related Documentation

- [connection-errors.md](./connection-errors.md) - Database/Redis connection issues
- [QUICK-REF-KUBERNETES-DEBUG.md](../llm-guides/QUICK-REF-KUBERNETES-DEBUG.md) - K8s debugging
