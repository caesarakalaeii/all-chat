# Quick Reference: Security Audit

Goal: perform a security review of All-Chat services.

## Checklist

### Authentication and authorization
- All API endpoints except health checks require JWT authentication
- JWT secret is strong and stored in a Kubernetes Secret
- No hardcoded credentials in code

```bash
# Find hardcoded secrets (should return nothing)
grep -r "password\|secret\|api_key" services/ --include="*.go" | grep -v "os.Getenv"
```

### Data protection
- OAuth tokens encrypted at rest: AES-256-GCM via `shared/encryption/`, versioned key rotation in `shared/encryption/versioned.go`
- TLS enabled for production
- Database credentials in Kubernetes Secrets

### Network security
- Kubernetes NetworkPolicies restrict pod-to-pod traffic
- Services use ClusterIP; no NodePort/LoadBalancer unless necessary
- CORS configured for production domain, not `*`
- API Gateway is the only external-facing service

### Input validation
- User inputs validated (Gin binding tags)
- SQL queries use parameterized statements (`$1, $2`, no `fmt.Sprintf`)

```bash
grep -r "fmt.Sprintf.*SELECT\|INSERT\|UPDATE" services/ --include="*.go"
```

### Known issues
- Service-to-service signing exists in `shared/signing/` (HMAC-SHA256, replay-protected) but is not wired into any prod service; NetworkPolicies are the current isolation control.
- CORS allows `*` in dev; must be configured for production.

See [docs/security/SECURITY_AUDIT_REPORT.md](../security/SECURITY_AUDIT_REPORT.md) and [docs/security/RESIDUALS.md](../security/RESIDUALS.md), plus [docs/architecture/05-SECURITY.md](../architecture/05-SECURITY.md).
