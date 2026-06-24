# Request Signing Package

HMAC-SHA256 based request signing for secure inter-service communication in All-Chat.

> **Status:** Implemented but **not yet wired into any prod service** (see `RESIDUALS.md` L2).
> Kubernetes NetworkPolicies are the current service-to-service isolation control.
> This library is hardened and tested; wiring requires shared-secret distribution +
> a service-identity contract rollout.

## Features

- ✅ HMAC-SHA256 request signatures
- ✅ Automatic request signing via HTTP transport
- ✅ Gin middleware for signature verification
- ✅ Timestamp-based replay attack prevention (past + future skew)
- ✅ Constant-time signature comparison (`hmac.Equal`)
- ✅ Body integrity verification
- ✅ Minimum secret length enforcement (32 bytes)
- ✅ Query-parameter + service-name tamper prevention

## Security Properties

1. **Authentication**: Proves the request came from a service with the shared secret
2. **Integrity**: Ensures request hasn't been tampered with (method, path, query, service, body)
3. **Replay Prevention**: Rejects requests older than 5 minutes (`MaxRequestAge`) or more than 1 minute in the future (`MaxFutureSkew`)
4. **Timing Attack Resistant**: Uses constant-time comparison
5. **Weak-secret Rejection**: `NewSigner` returns `ErrSecretTooShort` for secrets < 32 bytes

## Quick Start

### Server Side (Verify Incoming Requests)

```go
import (
    "github.com/caesar/all-chat/shared/signing"
    "github.com/gin-gonic/gin"
)

func main() {
    router := gin.New()

    // NewSigner returns an error if the secret is < 32 bytes (audit L1).
    verifier, err := signing.NewSigner(
        "my-service",             // This service's name
        os.Getenv("SERVICE_SECRET"), // Shared secret (≥ 32 bytes, same across all services)
        log,
    )
    if err != nil {
        log.Fatal("Failed to init signer", zap.Error(err))
    }

    // Add verification middleware to protected routes
    internal := router.Group("/internal")
    internal.Use(verifier.VerifyMiddleware())
    {
        internal.POST("/callback", handleCallback)
        internal.GET("/status", handleStatus)
    }

    router.Run(":8080")
}
```

### Client Side (Sign Outgoing Requests)

#### Option 1: Auto-Signing HTTP Client

```go
import "github.com/caesar/all-chat/shared/signing"

// NewSigningClient returns an error if the secret is < 32 bytes.
client, err := signing.NewSigningClient(
    "api-gateway",              // This service's name
    os.Getenv("SERVICE_SECRET"), // Shared secret
    log,
)
if err != nil {
    log.Fatal("Failed to init signing client", zap.Error(err))
}

// Make requests normally - signatures added automatically
resp, err := client.Post(
    "http://auth-service:8081/internal/verify",
    "application/json",
    bytes.NewBuffer(data),
)
```

#### Option 2: Manual Signing

```go
signer, err := signing.NewSigner("my-service", secret, log)
if err != nil {
    log.Fatal("Failed to init signer", zap.Error(err))
}

req, _ := http.NewRequest("POST", "http://other-service/api", body)
if err := signer.SignRequest(req); err != nil {
    log.Fatal("Failed to sign request", zap.Error(err))
}

resp, err := http.DefaultClient.Do(req)
```

#### Option 3: Custom Transport

```go
signer, err := signing.NewSigner("my-service", secret, log)
if err != nil {
    log.Fatal("Failed to init signer", zap.Error(err))
}

// Wrap any http.RoundTripper (defaults to http.DefaultTransport if nil).
transport := signing.NewSigningTransport(http.DefaultTransport, signer)

client := &http.Client{Transport: transport, Timeout: 30 * time.Second}
```

## How It Works

### Signature Format

```
HMAC-SHA256(secret, "method|path|query|service|timestamp|body_hash")
```

Where:
- `method` — HTTP method (e.g. `POST`)
- `path` — URL path (e.g. `/api/users`)
- `query` — Raw query string (e.g. `foo=bar&baz=1`; empty if none)
- `service` — Caller's service name (from `X-Service-Name` header)
- `timestamp` — Unix timestamp in seconds
- `body_hash` — Hex-encoded SHA-256 of the request body

Example:
```
HMAC-SHA256(secret, "POST|/api/users|foo=bar|api-gateway|1640000000|a3f5b1c...")
```

The query string and service name are included to prevent tampering with query
parameters or spoofing a different service identity (audit M5).

### Request Headers

**Client adds these headers**:
- `X-Service-Signature`: HMAC-SHA256 hex signature
- `X-Service-Timestamp`: Unix timestamp (seconds)
- `X-Service-Name`: Calling service name

**Server verifies**:
1. All three headers are present
2. Timestamp is parseable as int64
3. Timestamp is not older than `MaxRequestAge` (5 min) — prevents replay
4. Timestamp is not more than `MaxFutureSkew` (1 min) in the future — closes the future-replay window (audit M4)
5. Signature matches computed HMAC (constant-time)
6. Sets `service_name` in Gin context for handlers

### Signature Computation

1. Hash request body with SHA-256
2. Create message: `method|path|query|service|timestamp|body_hash`
3. Compute HMAC-SHA256 with shared secret
4. Encode as hex string

## Configuration

### Shared Secret

**CRITICAL**: All services must use the same secret for inter-service communication.
Must be at least 32 bytes — `NewSigner` / `NewSigningClient` return `ErrSecretTooShort` otherwise.

```bash
# .env for all services
SERVICE_SECRET=your-256-bit-secret-key-here

# Generate a strong secret (≥ 32 bytes):
openssl rand -base64 32
```

### Request Age Limits

| Constant | Value | Purpose |
|----------|-------|---------|
| `MaxRequestAge` | 5 minutes | Rejects stale (replayed) requests |
| `MaxFutureSkew` | 1 minute | Rejects future-dated timestamps (audit M4) |

## Manual Verification (Without Middleware)

```go
signer, err := signing.NewSigner("my-service", secret, log)
if err != nil {
    log.Fatal("Failed to init signer", zap.Error(err))
}

// VerifySignature includes rawQuery and serviceName (audit M5) — they must
// match what the signer used. Future-dated timestamps are rejected (audit M4).
err := signer.VerifySignature(
    method,      // HTTP method
    path,        // URL path
    rawQuery,    // Raw query string (may be empty)
    serviceName, // Caller's service name
    timestamp,   // Unix seconds (from X-Service-Timestamp)
    body,        // Request body bytes
    signature,   // Hex signature (from X-Service-Signature)
)

switch {
case errors.Is(err, signing.ErrInvalidSignature):
    // Handle invalid signature
case errors.Is(err, signing.ErrRequestTooOld):
    // Handle expired request
case errors.Is(err, signing.ErrRequestInFuture):
    // Handle future-dated request
case err != nil:
    // Other error
default:
    // Signature valid
}
```

### Error Reference

| Error | Meaning |
|-------|---------|
| `ErrSecretTooShort` | Secret < 32 bytes (from `NewSigner` / `NewSigningClient`) |
| `ErrMissingSignature` | `X-Service-Signature` header absent |
| `ErrMissingTimestamp` | `X-Service-Timestamp` header absent |
| `ErrMissingService` | `X-Service-Name` header absent |
| `ErrInvalidTimestamp` | Timestamp not parseable as int64 |
| `ErrInvalidSignature` | HMAC does not match |
| `ErrRequestTooOld` | Timestamp older than `MaxRequestAge` (5 min) |
| `ErrRequestInFuture` | Timestamp more than `MaxFutureSkew` (1 min) ahead |

> **Note:** The middleware returns HTTP 401 with a short JSON `{"error": "..."}`
> body for all rejection cases. The manual `VerifySignature` method returns the
> sentinel error instead (see table above).

### Accessing Service Name in Handlers

```go
func myHandler(c *gin.Context) {
    serviceName, exists := c.Get("service_name")
    if !exists {
        // Request wasn't signed (VerifyMiddleware not applied)
        return
    }

    log.Info("Request from service",
        zap.String("service", serviceName.(string)))
}
```

## Security Considerations

### ✅ Good Practices

1. **Use environment variables** for secrets (never hardcode)
2. **Rotate secrets periodically** (use graceful rotation with multiple valid secrets)
3. **Use HTTPS** for all inter-service communication (signing alone doesn't encrypt)
4. **Monitor failed verifications** (potential attack indicator)
5. **Keep timestamps synchronized** (use NTP across all services)

### ❌ Avoid

1. **Don't** use weak secrets (< 32 bytes — rejected by `NewSigner`)
2. **Don't** disable timestamp checks (enables replay attacks)
3. **Don't** log signatures or secrets
4. **Don't** use for public-facing APIs (use OAuth/JWT instead)
5. **Don't** skip signature verification in production

## Testing

```bash
go test ./shared/signing/...
```

Tests cover:
- ✅ Sign and verify round-trip
- ✅ Missing headers rejection
- ✅ Invalid signature rejection
- ✅ Expired request rejection
- ✅ Future-dated request rejection
- ✅ Short secret rejection
- ✅ Signature consistency
- ✅ HTTP client integration

## Integration Checklist

- [ ] Generate strong shared secret (≥ 32 bytes)
- [ ] Add `SERVICE_SECRET` to all service .env files
- [ ] Update internal routes to use `VerifyMiddleware()`
- [ ] Update inter-service HTTP clients to use `NewSigningClient` / `NewSigningTransport`
- [ ] Monitor logs for signature verification failures
- [ ] Document which endpoints require signatures

## Performance

- **Overhead**: ~1ms per request (HMAC computation)
- **Memory**: Minimal (streaming body hash)
- **Scalability**: No central coordination needed

## Monitoring

Watch for these log messages:

**Warnings** (potential attacks):
- `Request missing signature header`
- `Invalid request signature`
- `Request timestamp too old`
- `Request timestamp too far in the future`

**Debug** (successful):
- `Request signature verified`

## Future Enhancements

1. **Secret rotation**: Support multiple valid secrets for zero-downtime rotation
2. **Nonce tracking**: Additional replay protection for critical operations
3. **Asymmetric signing**: Use Ed25519 instead of HMAC for non-repudiation
4. **Rate limiting**: Combine with rate limiting per service

## Resources

- [HMAC-SHA256 Specification](https://tools.ietf.org/html/rfc2104)
- [Request Signing Best Practices](https://cloud.google.com/storage/docs/authentication/signatures)
