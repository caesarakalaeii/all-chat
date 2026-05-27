# Quick Reference: Security Audit

**Time Estimate**: 2-4 hours | **Difficulty**: ⭐⭐⭐ Advanced

**Goal**: Perform security review of All-Chat services.

---

## Security Checklist

### Authentication & Authorization

- [ ] All API endpoints (except health checks) require JWT authentication
- [ ] JWT tokens have reasonable expiry (24 hours max)
- [ ] JWT secret is strong (min 32 chars) and stored in Kubernetes Secret
- [ ] Service-to-service authentication uses shared secrets
- [ ] No hardcoded credentials in code

**Check**:
```bash
# Find hardcoded secrets (should return nothing)
grep -r "password\|secret\|api_key" services/ --include="*.go" | grep -v "os.Getenv"

# Verify JWT middleware applied
grep -r "JWTAuth" services/*/cmd/main.go
```

### Data Protection

- [x] OAuth tokens encrypted at rest via AES-256-GCM (`shared/encryption/`, with versioned key rotation in `versioned.go`)
- [ ] Passwords hashed (bcrypt, if applicable)
- [ ] TLS enabled for production (HTTPS)
- [ ] Database credentials stored in Kubernetes Secrets
- [ ] Redis password configured (if applicable)

### Network Security

- [ ] Kubernetes NetworkPolicies defined (restrict pod-to-pod communication)
- [ ] Services use ClusterIP (not NodePort or LoadBalancer unless necessary)
- [ ] CORS configured for production domain (not wildcard `*`)
- [ ] API Gateway is only external-facing service

### Input Validation

- [ ] All user inputs validated (use Gin binding tags)
- [ ] SQL queries use parameterized statements (prevent SQL injection)
- [ ] No command execution with user input (prevent command injection)
- [ ] WebSocket messages size-limited (prevent DoS)

**Check for SQL injection**:
```bash
# Search for string concatenation in SQL (should use $1, $2 params)
grep -r "fmt.Sprintf.*SELECT\|INSERT\|UPDATE" services/ --include="*.go"
```

### Known Vulnerabilities

**From historical security audit** (see [phase-reports](../phase-reports/CRITICAL_ARCHITECTURE_ANALYSIS.md)):
1. ~~Token encryption is basic~~ — now AES-256-GCM via `shared/encryption/` with versioned key rotation
2. ~~No service-to-service auth~~ — now signed via `shared/signing/`; NetworkPolicies still recommended
3. CORS allows `*` in dev (must configure for production)

---

## OWASP Top 10 Quick Check

1. **Broken Access Control**: Check JWT middleware on all protected routes
2. **Cryptographic Failures**: Check token encryption (AES-256-GCM in use; verify key rotation policy)
3. **Injection**: Search for SQL injection, command injection vectors
4. **Insecure Design**: Review ADRs for security considerations
5. **Security Misconfiguration**: Check default passwords, exposed debug endpoints
6. **Vulnerable Components**: Run `go list -m all | nancy sleuth` (check CVEs)
7. **Authentication Failures**: Check JWT expiry, refresh flows
8. **Data Integrity Failures**: Check TLS, message signing (if applicable)
9. **Logging Failures**: Check sensitive data not logged (tokens, passwords)
10. **SSRF**: Check external API calls validated

---

## Related Documentation

- [05-SECURITY.md](../architecture/05-SECURITY.md) - Security architecture
- [CRITICAL_ARCHITECTURE_ANALYSIS.md](../phase-reports/CRITICAL_ARCHITECTURE_ANALYSIS.md) - Known vulnerabilities
