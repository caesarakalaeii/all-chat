# Token Refresh Service

The Token Refresh Service is an always-on Deployment whose internal ticker (default 5 minutes, `TOKEN_REFRESH_INTERVAL`) refreshes OAuth tokens for YouTube, Kick, and other platforms before they expire. It prevents service interruptions from expired tokens. It is the single owner of scheduled token freshness — consumer services (moderation, the youtube-listener-innertube subscriber loop) only refresh reactively on a 401 as a fallback.

**Refresh cadence**: every 5 minutes, picking up tokens expiring within `TOKEN_REFRESH_BUFFER` (30m default — safely below the shortest access-token lifetime, ~1h for Google/YouTube). Worst case a token is refreshed ~35 minutes before expiry; no consumer service routinely sees a stale token.
**Status**: ✅ Production Ready

---

## Features

- **Automated Token Refresh**: Refreshes OAuth tokens within `TOKEN_REFRESH_BUFFER` of expiry (30m default)
- **Multi-Platform Support**: Twitch, YouTube, Kick token refresh flows
- **Error Handling**: Retries with exponential backoff, alerts on repeated failures
- **Database Updates**: Updates token expiry timestamps after successful refresh
- **Health Notifications**: Sends alerts when tokens cannot be refreshed
- **Metrics**: Prometheus metrics for refresh success/failure rates

---

## Architecture

```
Kubernetes Deployment (internal ticker, every 5 minutes by default)
  ↓
Token Refresh Service
  ↓ query expiring tokens
PostgreSQL (oauth_tokens table)
  ↓ tokens expiring within TOKEN_REFRESH_BUFFER (30m default)
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

- Go 1.26+
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

### Kubernetes Deployment

The service runs as a single-replica Deployment with an internal ticker (see
`deployments/k8s/base/token-refresh-service/deployment.yaml`):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: token-refresh-service
  namespace: allchat
spec:
  replicas: 1  # Single replica to prevent duplicate refreshes
  # ... template with env vars:
  #   TOKEN_REFRESH_INTERVAL: "5m"
  #   TOKEN_REFRESH_BUFFER: "30m"
```

---

## Token Refresh Logic

### Credential sources

Each batch queries every credential source separately and refreshes them in one pass. The
`token_type` on each row decides which table the refreshed token is written back to — writing one
source's token through another's updater would target the wrong row, and for the channel-keyed
tables could make a credential visible to a listener that selects by channel with no user scoping.

| `token_type` | Table | What it is |
|---|---|---|
| `user` | `users` | The account's login credential |
| `viewer` | `viewer_sessions` | Viewer sessions |
| `youtube_channel` | `youtube_oauth_tokens` | Per-channel YouTube grants, keyed `(user_id, channel_id)` |
| `twitch_link` | `twitch_oauth_tokens` | Linked Twitch credentials (ADR-0016), keyed `(user_id, twitch_login)` |
| `mod_credential` | `mod_oauth_credentials` | Delegated-moderator credentials (ADR-0048), keyed `(user_id, platform)` |

`mod_credential` is the only source that **spans platforms**, so its platform is read from the row
rather than assumed. It is not optional: without it a moderator's credential simply expires, the
grant keeps looking active in the UI while every action fails, and the 90-day dormancy suspension
can never fire because the moderator stopped being able to act long before that. A refresh never
rewrites `granted_scopes` — a periodic job must not be able to narrow what someone consented to.

### Query Expiring Tokens

```sql
SELECT id, user_id, platform, refresh_token, expiry
FROM oauth_tokens
WHERE expiry < NOW() + INTERVAL '30 minutes'  -- Expiring within TOKEN_REFRESH_BUFFER (30m)
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

**Symptom**: Service logs show 400/401 errors from OAuth APIs

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

**File**: `refresher/manager.go` (batch loop; platform refresh via `services/auth-service/oauth/`)

---

## Production Considerations

1. **Refresh cadence**: `TOKEN_REFRESH_INTERVAL` (5m) with `TOKEN_REFRESH_BUFFER` (30m) keeps the worst-case refresh ~35 minutes ahead of expiry — no consumer service should routinely see a stale token. Each batch touches only in-window tokens, so cadence multiplies batch overhead, not total refreshes.
2. **Retry Limits**: Max 3 retries with exponential backoff
3. **Alert Integration**: Configure `ALERT_WEBHOOK_URL` for Slack/PagerDuty
4. **Token Encryption**: Tokens encrypted at rest via AES-256-GCM (`shared/encryption/`)
5. **Audit Logging**: Log all token refresh attempts (success/failure) for compliance
6. **Failed Batches**: Alert if the batch error metric (`token_refresh_attempts_total{result="error"}`) stays non-zero across consecutive ticker runs.

## Related Services

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
