# Auth Service

The Auth Service handles multi-platform OAuth authentication (Twitch, YouTube, Kick) and issues JWT tokens for API access. It manages user sessions, OAuth token storage, and token refresh flows.

**Port**: 8081
**Status**: ✅ Production Ready

---

## Features

- **Multi-Platform OAuth**: Twitch, YouTube, Kick OAuth 2.0 authorization flows
- **JWT Token Issuance**: Generate secure JWT tokens for API authentication
- **Token Refresh**: Automatic OAuth token refresh before expiry
- **User Management**: Create/update user accounts linked to platform identities
- **Session Management**: Redis-based session storage with TTL
- **Token Encryption**: Basic encryption for OAuth tokens at rest
- **Health Checks**: Liveness and readiness probes for Kubernetes
- **Metrics**: Prometheus metrics for OAuth flows, token generation, errors

---

## Architecture

```
Frontend (Login Button)
  ↓ redirect
OAuth Provider (Twitch/YouTube/Kick)
  ↓ authorization code
Auth Service (/oauth/{platform}/callback)
  ↓ exchange code for tokens
OAuth Provider API
  ↓ access token + refresh token
Auth Service (store in database)
  ↓ generate JWT
Frontend (store JWT, redirect to dashboard)
```

---

## Environment Variables

### Required

```bash
# Twitch OAuth
TWITCH_CLIENT_ID=your_client_id
TWITCH_CLIENT_SECRET=your_client_secret

# YouTube OAuth
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxxxx
YOUTUBE_API_KEY=AIzaSyXXXXXX  # (optional, for API calls)

# Kick OAuth
KICK_CLIENT_ID=your_client_id
KICK_CLIENT_SECRET=your_client_secret

# JWT Configuration
JWT_SECRET=your-secret-key-min-32-chars
JWT_EXPIRY_HOURS=24  # Token validity duration

# Database connection
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis connection
REDIS_HOST=localhost
REDIS_PORT=6379

# Frontend URL (for OAuth redirects)
FRONTEND_URL=http://localhost:3000
```

### Optional

```bash
# Server configuration
PORT=8081
LOG_LEVEL=info  # debug, info, warn, error

# OpenTelemetry tracing
OTEL_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317

# Application
APP_VERSION=dev
ENVIRONMENT=development
```

---

## Running Locally

### Prerequisites

- Go 1.23+
- PostgreSQL with all-chat schema
- Redis
- OAuth credentials from Twitch, YouTube, Kick

### Get OAuth Credentials

**Twitch**:
1. Go to https://dev.twitch.tv/console/apps
2. Create application
3. Add OAuth redirect: `http://localhost:8080/api/v1/auth/twitch/callback`
4. Copy Client ID and Client Secret

**YouTube**:
1. Go to https://console.developers.google.com/
2. Create project
3. Enable YouTube Data API v3
4. Create OAuth 2.0 credentials
5. Add redirect: `http://localhost:8080/api/v1/auth/youtube/callback`
6. Copy Client ID and Client Secret

**Kick**:
1. Contact Kick developer support (no public developer portal yet)
2. Request OAuth application
3. Add redirect: `http://localhost:8080/api/v1/auth/kick/callback`
4. Copy Client ID and Client Secret

### Development

```bash
# Set environment variables
export TWITCH_CLIENT_ID=your_id
export TWITCH_CLIENT_SECRET=your_secret
export YOUTUBE_CLIENT_ID=your_id
export YOUTUBE_CLIENT_SECRET=your_secret
export JWT_SECRET=your-very-secure-secret-min-32-chars
export DATABASE_HOST=localhost
export REDIS_HOST=localhost

# Run the service
cd services/auth-service
go run ./cmd

# Or build and run
go build -o auth-service ./cmd
./auth-service
```

### With Docker Compose

```bash
# Add to .env file
TWITCH_CLIENT_ID=...
TWITCH_CLIENT_SECRET=...
YOUTUBE_CLIENT_ID=...
YOUTUBE_CLIENT_SECRET=...
JWT_SECRET=...

# Start service
make docker-up
```

---

## API Endpoints

### OAuth Flows

**Twitch OAuth**:
```bash
# 1. Initiate OAuth (redirect to Twitch)
GET /api/v1/auth/twitch/authorize
→ Redirects to: https://id.twitch.tv/oauth2/authorize?client_id=...&redirect_uri=...&scope=user:read:email

# 2. OAuth callback (Twitch redirects here)
GET /api/v1/auth/twitch/callback?code=...&state=...
→ Returns: { "token": "jwt-token", "user": {...} }
```

**YouTube OAuth**:
```bash
# 1. Initiate OAuth (redirect to Google)
GET /api/v1/auth/youtube/authorize
→ Redirects to: https://accounts.google.com/o/oauth2/v2/auth?client_id=...&scope=youtube.readonly

# 2. OAuth callback (Google redirects here)
GET /api/v1/auth/youtube/callback?code=...&state=...
→ Returns: { "token": "jwt-token", "user": {...} }
```

**Kick OAuth**:
```bash
# 1. Initiate OAuth (redirect to Kick)
GET /api/v1/auth/kick/authorize
→ Redirects to: https://kick.com/oauth2/authorize?client_id=...

# 2. OAuth callback (Kick redirects here)
GET /api/v1/auth/kick/callback?code=...&state=...
→ Returns: { "token": "jwt-token", "user": {...} }
```

### Token Management

```bash
# Refresh JWT token
POST /api/v1/auth/refresh
Authorization: Bearer <jwt-token>
→ Returns: { "token": "new-jwt-token" }

# Verify JWT token
POST /api/v1/auth/verify
Body: { "token": "jwt-token" }
→ Returns: { "valid": true, "user_id": "uuid", "expires_at": "..." }

# Logout (invalidate token)
POST /api/v1/auth/logout
Authorization: Bearer <jwt-token>
→ Returns: { "success": true }
```

### Health Checks

```bash
# Liveness probe
GET /health/live

# Readiness probe (checks DB + Redis)
GET /health/ready
```

### Metrics

```bash
# Prometheus metrics
GET /metrics
```

---

## OAuth Flow Details

### Twitch OAuth Flow

```
1. User clicks "Login with Twitch" on frontend
   ↓
2. Frontend redirects to: /api/v1/auth/twitch/authorize
   ↓
3. Auth Service redirects to Twitch authorization URL
   ↓
4. User authorizes on Twitch
   ↓
5. Twitch redirects to: /api/v1/auth/twitch/callback?code=abc123
   ↓
6. Auth Service exchanges code for access token
   ↓
7. Auth Service fetches user info from Twitch API
   ↓
8. Auth Service creates/updates user in database
   ↓
9. Auth Service generates JWT token
   ↓
10. Auth Service returns JWT to frontend
```

**Scopes Requested**:
- `user:read:email` - Read user email address

**Token Storage**:
- Access token encrypted and stored in `oauth_tokens` table
- Refresh token encrypted and stored (for token refresh)
- JWT Secret never stored (in-memory only)

### YouTube OAuth Flow

**Scopes Requested**:
- `https://www.googleapis.com/auth/youtube.readonly` - Read YouTube data
- `https://www.googleapis.com/auth/userinfo.profile` - User profile info
- `https://www.googleapis.com/auth/userinfo.email` - User email

**Token Storage**:
- Per-user OAuth tokens stored in `youtube_oauth_tokens` table
- Used by YouTube Listener to poll Live Chat API

---

## JWT Token Format

**Claims**:
```json
{
  "sub": "user-uuid",               // Subject (user ID)
  "username": "viewer123",          // Platform username
  "platform": "twitch",             // OAuth platform
  "platform_user_id": "12345678",   // Platform-specific user ID
  "iat": 1706400000,                // Issued at (Unix timestamp)
  "exp": 1706486400,                // Expires at (Unix + 24 hours)
  "iss": "allchat-auth-service",    // Issuer
  "aud": "allchat-api"              // Audience
}
```

**Example JWT** (for testing):
```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyLWlkIiwidXNlcm5hbWUiOiJ0ZXN0dXNlciIsInBsYXRmb3JtIjoidHdpdGNoIiwiaWF0IjoxNzA2NDAwMDAwLCJleHAiOjE3MDY0ODY0MDB9.signature
```

**Decode** (for debugging):
```bash
# Use jwt.io or jwt-cli
echo "eyJ..." | jwt decode -
```

---

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Run specific package tests
go test ./oauth -v
go test ./handlers -v
go test ./repository -v
```

---

## Troubleshooting

### OAuth Callback Fails (Code Exchange Error)

**Symptom**: `GET /oauth/{platform}/callback` returns 500 error

**Common Causes**:
1. **Invalid client secret**: Check `{PLATFORM}_CLIENT_SECRET` env var
2. **Redirect URI mismatch**: Must match exactly in OAuth app settings
3. **Expired authorization code**: Code valid for 30-60 seconds only
4. **Network error**: Cannot reach OAuth provider API

**Solution**:
```bash
# Check logs for specific error
kubectl logs -n allchat deployment/auth-service | grep "OAuth exchange failed"

# Verify credentials
kubectl exec -n allchat deployment/auth-service -- env | grep CLIENT

# Test OAuth provider reachability
curl https://id.twitch.tv/oauth2/token  # Should return 400 (missing params), not timeout
```

**File**: `oauth/twitch.go:ExchangeCode()`, `oauth/youtube.go:ExchangeCode()`, `oauth/kick.go:ExchangeCode()`

---

### JWT Token Invalid or Expired

**Symptom**: API requests return 401 Unauthorized

**Check token validity**:
```bash
# Decode JWT to check expiry
echo "eyJ..." | jwt decode -

# Verify signature with correct JWT_SECRET
# (must match across all services)
```

**Solutions**:
1. **Token expired**: Use `/api/v1/auth/refresh` to get new token
2. **Wrong JWT_SECRET**: Ensure all services use same secret (Kubernetes Secret)
3. **Token corrupted**: Client must re-login

**File**: `shared/auth/jwt.go:ValidateToken()`

---

### User Not Created in Database

**Symptom**: OAuth succeeds but user not found in database

**Check database**:
```bash
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat -c "SELECT * FROM users WHERE platform='twitch';"
```

**Solutions**:
1. Check database connection (auth-service logs)
2. Verify migrations applied (`users` table exists)
3. Check OAuth handler creates user: `repository/user_repo.go:CreateOrUpdateUser()`

**File**: `handlers/oauth.go:handleCallback()`, `repository/user_repo.go:CreateOrUpdateUser()`

---

## Production Considerations

1. **JWT Secret**: Use strong, random secret (min 32 characters). Store in Kubernetes Secret.
2. **OAuth Credentials**: Store in Kubernetes Secrets (not ConfigMaps or environment variables)
3. **Token Encryption**: Basic XOR encryption currently (TODO: migrate to AES-GCM)
4. **Session Storage**: Redis sessions expire after 24 hours (matches JWT expiry)
5. **Rate Limiting**: Consider adding rate limits to OAuth endpoints (prevent abuse)
6. **HTTPS Only**: OAuth callbacks must use HTTPS in production (Twitch/Google requirement)
7. **CORS**: Configure allowed origins (currently allows `*` in dev)

---

## Database Schema

### users Table

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) NOT NULL,
    platform VARCHAR(50) NOT NULL,             -- 'twitch', 'youtube', 'kick'
    platform_user_id VARCHAR(255) NOT NULL,    -- Platform-specific user ID
    email VARCHAR(255),
    avatar_url TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(platform, platform_user_id)
);
```

### oauth_tokens Table

```sql
CREATE TABLE oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    access_token TEXT NOT NULL,            -- Encrypted
    refresh_token TEXT,                    -- Encrypted
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP,
    scope TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, platform)
);
```

---

## Monitoring

### Key Metrics

```promql
# OAuth success rate
rate(auth_oauth_requests_total{result="success"}[5m]) / rate(auth_oauth_requests_total[5m])

# JWT generation rate
rate(auth_jwt_tokens_issued_total[5m])

# OAuth errors
rate(auth_oauth_requests_total{result="error"}[5m])

# Token refresh success rate
rate(auth_token_refresh_total{result="success"}[5m]) / rate(auth_token_refresh_total[5m])
```

### Alerts

**High OAuth Error Rate**:
```yaml
alert: AuthOAuthErrors
expr: rate(auth_oauth_requests_total{result="error"}[5m]) > 0.1
for: 5m
severity: warning
```

**Token Refresh Failures**:
```yaml
alert: AuthTokenRefreshFailed
expr: rate(auth_token_refresh_total{result="error"}[5m]) > 0.05
for: 10m
severity: warning
```

---

## Related Services

- **API Gateway**: Routes OAuth requests to Auth Service
- **YouTube Listener**: Uses stored YouTube OAuth tokens to poll Live Chat API
- **Overlay Manager**: Requires JWT authentication for CRUD operations
- **Token Refresh Service**: Background job to refresh expiring OAuth tokens

---

## Further Reading

- **[05-SECURITY.md](../../docs/architecture/05-SECURITY.md)** - Security architecture, OAuth security
- **[ADR-0005](../../docs/adr/0005-react-nextjs-frontend.md)** - Frontend OAuth integration
- **Twitch OAuth Docs**: https://dev.twitch.tv/docs/authentication/
- **YouTube OAuth Docs**: https://developers.google.com/youtube/v3/guides/auth/server-side-web-apps
- **JWT Best Practices**: https://tools.ietf.org/html/rfc8725

---

## License

Copyright © 2025 All-Chat. All rights reserved.
