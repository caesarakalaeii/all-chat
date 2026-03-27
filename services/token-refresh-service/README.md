# Token Refresh Service

The Token Refresh Service is a background job (Kubernetes CronJob) that refreshes expiring OAuth tokens for YouTube, Kick, and other platforms before they expire. It prevents service interruptions from expired tokens.

**Schedule**: Every 6 hours (Kubernetes CronJob)
**Status**: ✅ Production Ready

---

## Features

- **Automated Token Refresh**: Refreshes OAuth tokens 24 hours before expiry
- **Multi-Platform Support**: Twitch, YouTube, Kick token refresh flows
- **Error Handling**: Retries with exponential backoff, alerts on repeated failures
- **Database Updates**: Updates token expiry timestamps after successful refresh
- **Health Notifications**: Sends alerts when tokens cannot be refreshed
- **Metrics**: Prometheus metrics for refresh success/failure rates

---

## Architecture

```
Kubernetes CronJob (every 6 hours)
  ↓
Token Refresh Service
  ↓ query expiring tokens
PostgreSQL (oauth_tokens table)
  ↓ tokens expiring in <24 hours
Platform OAuth APIs (Twitch, YouTube, Kick)
  ↓ POST /oauth2/token (refresh_token grant)
New Access Token + Refresh Token
  ↓ encrypt and update
PostgreSQL (update oauth_tokens table)
  ↓ (if refresh fails repeatedly)
Alert (Slack/Email - user must re-authorize)
```

---

## Environment Variables

### Required

```bash
# Database connection
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# OAuth credentials (for token refresh)
TWITCH_CLIENT_ID=your_client_id
TWITCH_CLIENT_SECRET=your_client_secret

YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxxxx

KICK_CLIENT_ID=your_client_id
KICK_CLIENT_SECRET=your_client_secret
```

### Optional

```bash
LOG_LEVEL=info

# Token refresh settings
TOKEN_EXPIRY_THRESHOLD_HOURS=24  # Refresh tokens expiring within this window

# Retry configuration
MAX_RETRY_ATTEMPTS=3
RETRY_BACKOFF_SECONDS=60  # Exponential backoff base

# Alerting (if refresh fails)
ALERT_WEBHOOK_URL=https://hooks.slack.com/services/...

# Application
APP_VERSION=dev
ENVIRONMENT=development
```

---

## Running Locally

### Prerequisites

- Go 1.25+
- PostgreSQL with all-chat schema
- Valid OAuth credentials for platforms

### One-Time Execution

```bash
# Set environment variables
export DATABASE_HOST=localhost
export TWITCH_CLIENT_ID=...
export YOUTUBE_CLIENT_ID=...

# Run token refresh job
cd services/token-refresh-service
go run ./cmd

# Job will:
# 1. Query expiring tokens
# 2. Refresh each token with platform OAuth API
# 3. Update database
# 4. Exit (0 = success, 1 = failures occurred)
```

### Kubernetes CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: token-refresh
  namespace: allchat
spec:
  schedule: "0 */6 * * *"  # Every 6 hours
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: token-refresh
            image: ghcr.io/caesarakalaeii/allchat-token-refresh:main
            env:
            - name: DATABASE_HOST
              value: allchat-cluster-rw
            # ... other env vars from secrets
          restartPolicy: OnFailure
```

---

## Token Refresh Logic

### Query Expiring Tokens

```sql
SELECT id, user_id, platform, refresh_token, expiry
FROM oauth_tokens
WHERE expiry < NOW() + INTERVAL '24 hours'  -- Expiring within 24 hours
  AND refresh_token IS NOT NULL               -- Has refresh token
ORDER BY expiry ASC;                          -- Refresh soonest first
```

### Platform-Specific Refresh

**Twitch**:
```bash
POST https://id.twitch.tv/oauth2/token
Content-Type: application/x-www-form-urlencoded

client_id=<TWITCH_CLIENT_ID>
&client_secret=<TWITCH_CLIENT_SECRET>
&grant_type=refresh_token
&refresh_token=<token>

→ Returns: {
  "access_token": "new_access_token",
  "refresh_token": "new_refresh_token",  # May be same or new
  "expires_in": 3600,
  "token_type": "Bearer"
}
```

**YouTube**:
```bash
POST https://oauth2.googleapis.com/token
Content-Type: application/x-www-form-urlencoded

client_id=<YOUTUBE_CLIENT_ID>
&client_secret=<YOUTUBE_CLIENT_SECRET>
&grant_type=refresh_token
&refresh_token=<token>

→ Returns: {
  "access_token": "new_access_token",
  "expires_in": 3600,
  "scope": "...",
  "token_type": "Bearer"
}
# Note: YouTube refresh tokens do NOT rotate (same refresh_token reused)
```

**Kick**:
```bash
POST https://kick.com/oauth2/token
Content-Type: application/json

{
  "client_id": "<KICK_CLIENT_ID>",
  "client_secret": "<KICK_CLIENT_SECRET>",
  "grant_type": "refresh_token",
  "refresh_token": "<token>"
}

→ Returns: {
  "access_token": "new_access_token",
  "refresh_token": "new_refresh_token",
  "expires_in": 3600
}
```

### Error Handling

**Retry Logic** (exponential backoff):
```
Attempt 1: Immediate
Attempt 2: 60s delay
Attempt 3: 120s delay
Attempt 4: 240s delay
→ After 4 failures: Alert user (must re-authorize)
```

**Error Types**:
- **400 Bad Request**: Invalid refresh token → Alert user (re-auth required)
- **401 Unauthorized**: Token revoked → Alert user (re-auth required)
- **429 Rate Limited**: Too many requests → Retry with backoff
- **5xx Server Error**: Platform issue → Retry with backoff

---

## Monitoring

### Metrics

```promql
# Token refresh success rate (target: >99%)
rate(token_refresh_attempts_total{result="success"}[6h]) / rate(token_refresh_attempts_total[6h])

# Tokens requiring refresh per run
token_refresh_expiring_tokens_total

# Platform-specific errors
rate(token_refresh_attempts_total{result="error", platform="youtube"}[6h])
```

### Alerts

**Token Refresh Failures**:
```yaml
alert: TokenRefreshFailed
expr: rate(token_refresh_attempts_total{result="error"}[6h]) > 0.05
for: 1h
severity: warning
```

**Many Expiring Tokens**:
```yaml
alert: ManyExpiringTokens
expr: token_refresh_expiring_tokens_total > 100
for: 1h
severity: info
```

---

## Troubleshooting

### Refresh Tokens Failing

**Symptom**: CronJob logs show 400/401 errors from OAuth APIs

**Check tokens**:
```bash
kubectl exec -n allchat allchat-cluster-1 -- psql -U allchat -c "
  SELECT platform, COUNT(*) as count, MIN(expiry) as soonest_expiry
  FROM oauth_tokens
  WHERE expiry < NOW() + INTERVAL '24 hours'
  GROUP BY platform;
"
```

**Solutions**:
1. **400 Invalid Token**: Refresh token revoked → User must re-authorize
2. **401 Unauthorized**: OAuth credentials expired → Update `{PLATFORM}_CLIENT_SECRET`
3. **Platform API down**: Check platform status pages, retry later

**Alert Users** (if refresh repeatedly fails):
```
Email/Slack: "Your YouTube connection expired. Please re-authorize at https://allchat.example.com/settings"
```

**File**: `refresh/youtube.go:RefreshToken()`, `refresh/twitch.go:RefreshToken()`

---

## Production Considerations

1. **CronJob Schedule**: Run every 6 hours (4× per day) to catch tokens expiring within 24 hours
2. **Retry Limits**: Max 3 retries with exponential backoff
3. **Alert Integration**: Configure `ALERT_WEBHOOK_URL` for Slack/PagerDuty
4. **Token Encryption**: Tokens encrypted at rest (basic XOR currently, TODO: AES-GCM)
5. **Audit Logging**: Log all token refresh attempts (success/failure) for compliance
6. **Failed Jobs**: Monitor CronJob failures in Kubernetes (alert if job fails 3× consecutively)

---

## Related Services

- **Auth Service**: Issues initial OAuth tokens
- **YouTube Listener**: Uses refreshed YouTube tokens to poll Live Chat API
- **PostgreSQL**: Stores OAuth tokens with expiry timestamps

---

## Further Reading

- **[05-SECURITY.md](../../docs/architecture/05-SECURITY.md)** - Token encryption, security considerations
- **Twitch Token Refresh**: https://dev.twitch.tv/docs/authentication/refresh-tokens
- **YouTube Token Refresh**: https://developers.google.com/identity/protocols/oauth2/web-server#offline

---

## License

Copyright © 2025 All-Chat. All rights reserved.
