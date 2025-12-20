# Request Signing Package

HMAC-SHA256 based request signing for secure inter-service communication in All-Chat.

## Features

- ✅ HMAC-SHA256 request signatures
- ✅ Automatic request signing via HTTP transport
- ✅ Gin middleware for signature verification
- ✅ Timestamp-based replay attack prevention
- ✅ Constant-time signature comparison
- ✅ Body integrity verification

## Security Properties

1. **Authentication**: Proves the request came from a service with the shared secret
2. **Integrity**: Ensures request hasn't been tampered with (method, path, body)
3. **Replay Prevention**: Rejects requests older than 5 minutes
4. **Timing Attack Resistant**: Uses constant-time comparison

## Quick Start

### Server Side (Verify Incoming Requests)

```go
import (
    "github.com/caesar/all-chat/shared/signing"
    "github.com/gin-gonic/gin"
)

func main() {
    router := gin.New()

    // Create verifier with shared secret
    verifier := signing.NewSigner(
        "my-service",           // This service's name
        getEnv("SERVICE_SECRET"), // Shared secret (same across all services)
        log,
    )

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

// Create client that automatically signs all requests
client := signing.NewSigningClient(
    "api-gateway",          // This service's name
    getEnv("SERVICE_SECRET"), // Shared secret
    log,
)

// Make requests normally - signatures added automatically
resp, err := client.Post(
    "http://auth-service:8081/internal/verify",
    "application/json",
    bytes.NewBuffer(data),
)
```

#### Option 2: Manual Signing

```go
signer := signing.NewSigner("my-service", secret, log)

req, _ := http.NewRequest("POST", "http://other-service/api", body)
signer.SignRequest(req)

resp, err := http.DefaultClient.Do(req)
```

## How It Works

### Signature Format

```
HMAC-SHA256(secret, "METHOD|PATH|TIMESTAMP|BODY_HASH")
```

Example:
```
HMAC-SHA256(secret, "POST|/api/users|1640000000|a3f5b1c...")
```

### Request Headers

**Client adds these headers**:
- `X-Service-Signature`: HMAC-SHA256 hex signature
- `X-Service-Timestamp`: Unix timestamp (seconds)
- `X-Service-Name`: Calling service name

**Server verifies**:
1. Headers are present
2. Timestamp is recent (< 5 minutes old)
3. Signature matches computed HMAC
4. Adds `service_name` to Gin context for handlers

### Signature Computation

1. Hash request body with SHA256
2. Create message: `METHOD|PATH|TIMESTAMP|BODY_HASH`
3. Compute HMAC-SHA256 with shared secret
4. Encode as hex string

## Configuration

### Shared Secret

**CRITICAL**: All services must use the same secret for inter-service communication.

```bash
# .env for all services
SERVICE_SECRET=your-256-bit-secret-key-here

# Generate a strong secret:
openssl rand -base64 32
```

### Request Age Limit

Default: 5 minutes (prevents replay attacks)

To customize, modify `MaxRequestAge` constant in `signing.go`.

## Example: Service-to-Service Call

### Scenario: API Gateway → Auth Service

**API Gateway** (client):
```go
// Initialize once at startup
authClient := signing.NewSigningClient(
    "api-gateway",
    os.Getenv("SERVICE_SECRET"),
    log,
)

// Use in handler
func proxyToAuth(c *gin.Context) {
    resp, err := authClient.Post(
        "http://auth-service:8081/internal/verify-token",
        "application/json",
        bytes.NewBuffer(tokenData),
    )
    // Handle response...
}
```

**Auth Service** (server):
```go
// Initialize once at startup
verifier := signing.NewSigner(
    "auth-service",
    os.Getenv("SERVICE_SECRET"),
    log,
)

router := gin.New()

// Protect internal routes
internal := router.Group("/internal")
internal.Use(verifier.VerifyMiddleware())
{
    internal.POST("/verify-token", handleVerifyToken)
}
```

## Error Responses

### 401 Unauthorized - Missing Signature
```json
{
  "error": "missing signature"
}
```

### 401 Unauthorized - Invalid Signature
```json
{
  "error": "invalid signature"
}
```

### 401 Unauthorized - Request Too Old
```json
{
  "error": "request too old"
}
```

## Advanced Usage

### Custom Verification Logic

```go
signer := signing.NewSigner("my-service", secret, log)

// Manual verification
err := signer.VerifySignature(
    method,
    path,
    timestamp,
    body,
    signature,
)

if err == signing.ErrInvalidSignature {
    // Handle invalid signature
} else if err == signing.ErrRequestTooOld {
    // Handle expired request
}
```

### Accessing Service Name in Handlers

```go
func myHandler(c *gin.Context) {
    serviceName, exists := c.Get("service_name")
    if !exists {
        // Request wasn't signed
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

1. **Don't** use weak secrets (< 32 bytes)
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
- ✅ Signature consistency
- ✅ HTTP client integration

## Integration Checklist

- [ ] Generate strong shared secret
- [ ] Add `SERVICE_SECRET` to all service .env files
- [ ] Update internal routes to use verification middleware
- [ ] Update inter-service HTTP clients to use signing transport
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
