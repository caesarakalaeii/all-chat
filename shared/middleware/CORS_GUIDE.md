# CORS Configuration Guide

Environment-aware CORS configuration for All-Chat services.

## Features

- ✅ Environment-specific origin lists (production, staging, development)
- ✅ Custom origin support via config or env variables
- ✅ Browser extension support (chrome-extension://)
- ✅ Wildcard extension support in development
- ✅ Secure defaults for production
- ✅ Comprehensive header allowlist
- ✅ Credentials support

## Environment Configurations

### Production (`ENVIRONMENT=production`)

```
Allowed Origins:
- https://allch.at
- https://www.allch.at
- chrome-extension://{BROWSER_EXTENSION_ID} (if set)
- Custom origins from CORS_ORIGINS
```

### Staging (`ENVIRONMENT=staging`)

```
Allowed Origins:
- https://staging.allch.at
- http://localhost:3000 (for local testing)
- Custom origins from CORS_ORIGINS
```

### Development (`ENVIRONMENT=development` or not set)

```
Allowed Origins:
- http://localhost:3000
- http://localhost:3001
- http://localhost:8080
- http://127.0.0.1:3000
- chrome-extension://* (wildcard - any extension)
- Custom origins from CORS_ORIGINS
```

## Quick Start

### Option 1: Auto-Configuration from Environment

```go
import (
    "github.com/caesar/all-chat/shared/middleware"
    "github.com/gin-gonic/gin"
)

func main() {
    router := gin.New()

    // Automatically configures based on ENVIRONMENT variable
    router.Use(middleware.CORSFromEnv(log))

    router.Run(":8080")
}
```

**Environment Variables**:
```bash
ENVIRONMENT=production              # Environment: production, staging, development
CORS_ORIGINS=https://custom.com     # Additional origins (comma-separated)
BROWSER_EXTENSION_ID=abc123...      # Chrome extension ID (optional)
```

### Option 2: Explicit Configuration

```go
router.Use(middleware.CORSMiddleware(middleware.CORSConfig{
    AllowedOrigins: []string{"https://custom.com"},
    Environment:    "production",
    Logger:         log,
}))
```

## Configuration Options

| Field | Description | Default |
|-------|-------------|---------|
| `AllowedOrigins` | Additional origins to allow | `[]` |
| `Environment` | Environment name | `"development"` |
| `Logger` | Zap logger for debugging | `zap.NewNop()` |

## Headers Configuration

### Allowed Methods
```
GET, POST, PUT, PATCH, DELETE, OPTIONS
```

### Allowed Headers (Request)
```
Origin, Content-Type, Authorization, X-Requested-With
```

### Exposed Headers (Response)
```
Content-Length
X-RateLimit-Limit
X-RateLimit-Remaining
X-RateLimit-Reset
Retry-After
```

### Other Settings
- `AllowCredentials`: `true` (cookies, auth headers)
- `MaxAge`: `12 hours` (preflight cache)

## Environment Variables

### Required

```bash
ENVIRONMENT=production|staging|development
```

### Optional

```bash
# Additional allowed origins (comma-separated)
CORS_ORIGINS=https://app1.com,https://app2.com

# Chrome extension ID (for production)
BROWSER_EXTENSION_ID=abcdefghijklmnopqrstuvwxyz123456
```

## Browser Extension Support

### Development
In development mode, **all** chrome-extension origins are allowed automatically:
- `chrome-extension://abc123...`
- `chrome-extension://xyz789...`
- Any unpacked extension

### Production
In production, only the specified extension ID is allowed:
```bash
BROWSER_EXTENSION_ID=abcdefghijklmnopqrstuvwxyz123456
```

This allows:
- `chrome-extension://abcdefghijklmnopqrstuvwxyz123456`

## Migration from Legacy CORS

### Before (Insecure)
```go
// Allows all origins (*)
router.Use(cors.Default())
```

### After (Secure)
```go
// Environment-specific origins
router.Use(middleware.CORSFromEnv(log))
```

## Testing CORS

### Development
```bash
curl -H "Origin: http://localhost:3000" \
     -H "Access-Control-Request-Method: POST" \
     -X OPTIONS \
     http://localhost:8080/api/v1/overlays

# Should return:
# Access-Control-Allow-Origin: http://localhost:3000
# Access-Control-Allow-Credentials: true
```

### Production
```bash
curl -H "Origin: https://allch.at" \
     -H "Access-Control-Request-Method: POST" \
     -X OPTIONS \
     https://allch.at/api/v1/overlays
```

### Browser Console
```javascript
fetch('http://localhost:8080/api/v1/health', {
  credentials: 'include',
  headers: {'Content-Type': 'application/json'}
})
.then(r => r.json())
.then(console.log)
```

## Troubleshooting

### CORS Error: Origin not allowed

**Check**:
1. Environment variable is set correctly: `echo $ENVIRONMENT`
2. Origin matches expected format (http vs https)
3. Custom origins are in CORS_ORIGINS
4. Browser extension ID is set (if using extension)

**Solution**:
```bash
# Add custom origin
export CORS_ORIGINS=https://my-custom-domain.com

# Restart service
```

### Preflight OPTIONS requests failing

**Symptom**: Complex requests (POST with JSON) fail

**Check**:
- Server handles OPTIONS method
- `Access-Control-Allow-Methods` includes your method
- `Access-Control-Allow-Headers` includes your headers

**Solution**: Already handled by middleware

### Credentials not being sent

**Check**:
- `AllowCredentials` is `true` (already set)
- Frontend uses `credentials: 'include'`
- Origin is not wildcard `*` (wildcards don't work with credentials)

## Security Best Practices

### ✅ Do

1. **Set ENVIRONMENT variable** in production
2. **Use HTTPS** in production (http:// origins rejected by browsers)
3. **Whitelist specific origins** (never use `*` with credentials)
4. **Monitor CORS errors** (could indicate attacks or misconfig)
5. **Document allowed origins** in deployment docs

### ❌ Don't

1. **Don't** use `*` wildcard with credentials
2. **Don't** allow HTTP origins in production
3. **Don't** forget to set ENVIRONMENT
4. **Don't** hardcode origins (use env variables)
5. **Don't** allow untrusted origins

## Production Deployment

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-gateway-config
data:
  ENVIRONMENT: "production"
  CORS_ORIGINS: "https://allch.at,https://www.allch.at"
  BROWSER_EXTENSION_ID: "your-extension-id"
```

### Docker Environment

```bash
docker run \
  -e ENVIRONMENT=production \
  -e CORS_ORIGINS=https://allch.at \
  -e BROWSER_EXTENSION_ID=abc123... \
  api-gateway:latest
```

## Monitoring

Monitor these logs:

```
INFO: CORS configured
  allowed_origins: ["https://allch.at", "https://www.allch.at"]
  environment: production
  allow_extension_wildcard: false
```

Watch for rejected origins in access logs.

## Examples

### Example 1: API Gateway (Production)

```go
router := gin.New()
router.Use(middleware.CORSFromEnv(log))
```

With env:
```bash
ENVIRONMENT=production
BROWSER_EXTENSION_ID=abcdef123456
```

Allows:
- `https://allch.at`
- `https://www.allch.at`
- `chrome-extension://abcdef123456`

### Example 2: Development with Multiple Frontends

```bash
ENVIRONMENT=development
CORS_ORIGINS=http://localhost:3001,http://localhost:3002
```

Allows:
- `http://localhost:3000` (default)
- `http://localhost:3001` (custom)
- `http://localhost:3002` (custom)
- `chrome-extension://*` (wildcard)

### Example 3: Staging

```bash
ENVIRONMENT=staging
CORS_ORIGINS=https://preview.allch.at
```

Allows:
- `https://staging.allch.at` (default)
- `https://preview.allch.at` (custom)
- `http://localhost:3000` (for local testing)

## Resources

- [MDN CORS Guide](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)
- [OWASP CORS Security](https://owasp.org/www-community/controls/CORS)
