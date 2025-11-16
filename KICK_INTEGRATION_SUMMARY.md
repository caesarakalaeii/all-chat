# Kick Integration Implementation Summary

## Completed Work

### OAuth Implementation Status: ✅ FULLY WORKING

**Last Updated:** 2025-11-16
**Status:** Production Ready

### 1. OAuth Implementation ✅

**Files Created/Modified:**
- `services/auth-service/oauth/kick.go` - Complete Kick OAuth 2.1 with PKCE implementation
- `services/auth-service/oauth/platform.go` - Added KickUserInfoWrapper
- `services/auth-service/models/user.go` - Added KickUserInfo model and kick_id field
- `services/auth-service/handlers/platform_auth.go` - Updated to handle PKCE flow for Kick
- `services/auth-service/repository/user_repository.go` - Added GetByKickID, GetByTikTokID methods
- `services/auth-service/cmd/main.go` - Added Kick OAuth initialization and routes

**Key Features:**
- OAuth 2.1 with PKCE (Proof Key for Code Exchange) support
- Code verifier generation and validation
- S256 code challenge method
- Auth URL: `https://id.kick.com/oauth/authorize`
- Token URL: `https://id.kick.com/oauth/token`
- User API: `https://api.kick.com/public/v1/users` (Official Kick Public API v1)
- Scopes: `chat:read user:read channel:read`

**Known Limitations:**
- **Profile Pictures:** Kick blocks external hotlinking of profile images (returns 403 Forbidden). The backend correctly fetches and stores the profile picture URL, but browsers cannot load images directly from `kick.com/img/*`.
  - **Current Solution:** Frontend uses placeholder/fallback when Kick images fail to load
  - **Recommended:** Use Kick logo or generated avatar as placeholder for Kick users
  - **Alternative Solutions:**
    - Image proxy service to re-serve Kick images from our domain
    - Download and cache images in Redis/S3
    - Generate avatar from username initials

**OAuth Endpoints:**
- `GET /api/v1/auth/kick/login` - Initiates OAuth flow
- `GET /api/v1/auth/kick/callback` - Handles OAuth callback

**Environment Variables Required:**
```bash
KICK_CLIENT_ID=your_client_id
KICK_CLIENT_SECRET=your_client_secret
KICK_REDIRECT_URL=http://localhost:8080/api/v1/auth/kick/callback
```

### 2. Database Schema ✅

**Migration Created:**
- `migrations/005_kick_support.sql`

**Changes:**
- Added `kick_id` column to `users` table
- Created index on `kick_id`
- Added unique constraint for `kick_id`
- Updated `auth_provider` check constraint to include 'kick'
- Created `kick_oauth_tokens` table for storing OAuth tokens for Kick Listener

**Table Structure - kick_oauth_tokens:**
```sql
- id (SERIAL PRIMARY KEY)
- user_id (UUID, FK to users)
- channel_id (VARCHAR) - Kick channel slug or chatroom ID
- access_token (TEXT)
- refresh_token (TEXT)
- token_type (VARCHAR, default 'Bearer')
- expiry (TIMESTAMP)
- created_at, updated_at (TIMESTAMP)
```

## Remaining Work

### 3. Kick Listener Service (IN PROGRESS)

The Kick Listener service needs to be created following the pattern of Twitch and YouTube listeners. The key difference is that Kick uses Pusher WebSocket instead of IRC or HTTP polling.

**Architecture:**
```
services/kick-listener/
├── cmd/
│   └── main.go              # Service entry point
├── websocket/
│   ├── client.go            # Pusher WebSocket client
│   └── types.go             # Message type definitions
├── channels/
│   ├── manager.go           # Dynamic channel subscription management
│   └── repository.go        # Database access for active channels
├── publisher/
│   └── redis.go             # Publishes to Redis Streams
├── handlers/
│   └── health.go            # Health check handlers
├── go.mod
├── go.sum
├── Dockerfile
└── README.md
```

**Pusher WebSocket Details:**
- URL: `wss://ws-us2.pusher.com/app/32cbd69e4b950bf97679`
- Protocol: Pusher Protocol 7
- Channel format: `chatrooms.{chatroom_id}`
- Event: `App\\Events\\ChatMessageSentEvent`

**Implementation Steps:**

1. **Create Pusher WebSocket Client (`websocket/client.go`)**
   - Connect to Pusher WebSocket server
   - Implement Pusher protocol handshake
   - Subscribe to chatroom channels
   - Handle ping/pong for connection keepalive
   - Parse incoming chat messages
   - Implement automatic reconnection

2. **Channel Manager (`channels/manager.go`)**
   - Sync active Kick channels from database
   - Dynamically subscribe/unsubscribe from chatrooms
   - Map overlay_id to Kick channel_id/chatroom_id
   - Handle channel state changes

3. **Redis Publisher (`publisher/redis.go`)**
   - Publish raw messages to Redis Stream `chat:raw`
   - Message format:
   ```json
   {
     "platform": "kick",
     "overlay_id": "uuid",
     "channel_id": "kick_channel_slug",
     "channel_name": "Display Name",
     "raw_message": {...}  // Raw Kick message
   }
   ```

4. **Main Service (`cmd/main.go`)**
   - Initialize logger, database, Redis connections
   - Create WebSocket client
   - Start channel manager
   - Health check HTTP server on port 8089
   - Graceful shutdown handling

**Environment Variables:**
```bash
PORT=8089
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat
REDIS_HOST=localhost
REDIS_PORT=6379
LOG_LEVEL=info
```

**Dockerfile:**
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o kick-listener ./services/kick-listener/cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/kick-listener .
EXPOSE 8089
CMD ["./kick-listener"]
```

### 4. Message Processor - Kick Normalizer

**File to Create:**
`services/message-processor/normalizer/kick_normalizer.go`

**Purpose:**
Convert raw Kick messages into the unified message format.

**Implementation:**
```go
package normalizer

import (
    "encoding/json"
    "time"
)

type KickNormalizer struct{}

func NewKickNormalizer() *KickNormalizer {
    return &KickNormalizer{}
}

// Normalize converts Kick message to unified format
func (n *KickNormalizer) Normalize(rawMessage json.RawMessage) (*UnifiedMessage, error) {
    var kickMsg KickChatMessage
    if err := json.Unmarshal(rawMessage, &kickMsg); err != nil {
        return nil, err
    }

    // Parse Kick message structure
    // Kick messages come through Pusher with event data

    return &UnifiedMessage{
        ID:          generateID(),
        Platform:    "kick",
        ChannelID:   kickMsg.ChatroomID,
        ChannelName: kickMsg.ChatroomName,
        User: UnifiedUser{
            ID:          kickMsg.Sender.ID,
            Username:    kickMsg.Sender.Username,
            DisplayName: kickMsg.Sender.Username,
            Badges:      parseBadges(kickMsg.Sender.Identity),
            Color:       kickMsg.Sender.Identity.Color,
        },
        Message: UnifiedMessageContent{
            Text: kickMsg.Content,
        },
        Timestamp: parseTime(kickMsg.CreatedAt),
        Metadata: UnifiedMetadata{
            IsSubscriber: kickMsg.Sender.Identity.Badges.Contains("subscriber"),
            IsModerator:  kickMsg.Sender.Identity.Badges.Contains("moderator"),
        },
    }, nil
}
```

**Update Message Router:**
Add Kick case to `services/message-processor/router/router.go`:
```go
case "kick":
    normalizer = normalizer.NewKickNormalizer()
```

### 5. Kubernetes Deployment

**Files to Create:**

`deployments/k8s/base/kick-listener/deployment.yaml`:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kick-listener
  namespace: allchat
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kick-listener
  template:
    metadata:
      labels:
        app: kick-listener
    spec:
      containers:
      - name: kick-listener
        image: your-registry/kick-listener:latest
        ports:
        - containerPort: 8089
        env:
        - name: DATABASE_HOST
          valueFrom:
            configMapKeyRef:
              name: allchat-config
              key: database_host
        - name: REDIS_HOST
          valueFrom:
            configMapKeyRef:
              name: allchat-config
              key: redis_host
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8089
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8089
          initialDelaySeconds: 5
          periodSeconds: 10
```

`deployments/k8s/base/kick-listener/service.yaml`:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: kick-listener
  namespace: allchat
spec:
  selector:
    app: kick-listener
  ports:
  - port: 8089
    targetPort: 8089
  type: ClusterIP
```

`deployments/k8s/base/kick-listener/hpa.yaml`:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: kick-listener-hpa
  namespace: allchat
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: kick-listener
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### 6. Documentation

**Create `services/kick-listener/README.md`:**
- Service overview
- Architecture details
- Pusher WebSocket protocol
- Environment variables
- API endpoints (health checks)
- Development setup
- Testing guide
- Deployment instructions

**Update Main Documentation:**
- `README.md` - Add Kick to supported platforms
- `CLAUDE.md` - Add Kick Listener service details
- `.env.example` - Add Kick environment variables

### 7. Ansible Deployment Integration

**Location:** `../caesar-deployment`

**Files to Update:**

1. **Inventory/Variables:**
   - Add Kick OAuth credentials to secrets
   - Add Kick Listener service configuration

2. **Playbooks:**
   - Add Kick Listener deployment tasks
   - Update ConfigMap with Kick variables
   - Add Kick Listener to service deployment list

3. **Templates:**
   - Add Kick Listener Kubernetes manifests
   - Update ingress rules if needed

**Example Ansible Task:**
```yaml
- name: Deploy Kick Listener
  kubernetes.core.k8s:
    state: present
    definition: "{{ lookup('template', 'kick-listener-deployment.yaml.j2') }}"
    namespace: allchat

- name: Create Kick Listener Service
  kubernetes.core.k8s:
    state: present
    definition: "{{ lookup('template', 'kick-listener-service.yaml.j2') }}"
    namespace: allchat
```

## Testing Checklist

### OAuth Flow Testing:
- [ ] Test Kick OAuth login initiation
- [ ] Verify PKCE code challenge generation
- [ ] Test OAuth callback handling
- [ ] Verify code verifier validation
- [ ] Test token exchange
- [ ] Test user creation with Kick ID
- [ ] Test existing user login
- [ ] Test token refresh

### Listener Service Testing:
- [ ] Test Pusher WebSocket connection
- [ ] Test channel subscription
- [ ] Test message reception
- [ ] Test Redis Stream publishing
- [ ] Test channel manager sync
- [ ] Test health endpoints
- [ ] Test graceful shutdown
- [ ] Test reconnection logic

### Integration Testing:
- [ ] End-to-end: OAuth → Listener → Message Processor → WebSocket
- [ ] Test with multiple overlays
- [ ] Test with multiple Kick channels
- [ ] Test error handling and recovery
- [ ] Load testing with multiple concurrent connections

## Dependencies

**Go Packages Needed:**
- `github.com/pusher/pusher-http-go` - Official Pusher Go SDK (or implement custom WebSocket client)
- Or use generic WebSocket library: `github.com/gorilla/websocket`

**Installation:**
```bash
go get github.com/gorilla/websocket
```

## API Documentation References

- **Kick Dev Docs:** https://docs.kick.com
- **GitHub:** https://github.com/KickEngineering/KickDevDocs
- **OAuth 2.1 Spec:** https://datatracker.ietf.org/doc/html/draft-ietf-oauth-v2-1-07
- **PKCE Spec:** https://datatracker.ietf.org/doc/html/rfc7636
- **Pusher Protocol:** https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol/

## Environment Variables Summary

Add to `.env.example`:
```bash
# Kick OAuth (Required for Kick support)
KICK_CLIENT_ID=
KICK_CLIENT_SECRET=
KICK_REDIRECT_URL=http://localhost:8080/api/v1/auth/kick/callback

# Kick Listener (Port 8089)
# Uses shared DATABASE_* and REDIS_* variables
```

## Next Steps

1. **Implement Kick Listener Service** - Highest priority
2. **Create Kick Normalizer** - Depends on Listener
3. **Create K8s Manifests** - Can be done in parallel
4. **Write Documentation** - Can be done in parallel
5. **Integrate with Ansible** - Final step
6. **Testing** - Throughout all steps

## Notes

- Kick uses Pusher WebSocket which is different from Twitch IRC and YouTube HTTP polling
- Need to obtain Kick chatroom_id from channel API before subscribing
- Kick API: `GET https://kick.com/api/v2/channels/{channel_slug}` returns chatroom info
- Consider rate limiting for Kick API calls
- Kick messages include rich identity/badge information
- May need to handle Kick-specific emotes differently from Twitch/BTTV/FFZ

## Troubleshooting

### OAuth Issues Fixed (2025-11-16)

**Problem 1: "Failed to get user info" - 403 Forbidden**
- **Cause:** Using wrong API endpoint and missing headers
- **Solution:** Changed to `https://api.kick.com/public/v1/users` with proper User-Agent header

**Problem 2: "Failed to decode response" - Invalid JSON**
- **Cause:** API response has `{"data": [...], "message": "OK"}` wrapper structure
- **Solution:** Updated parsing to handle wrapper object

**Problem 3: Field mapping errors**
- **Cause:** API uses `user_id`, `name`, `profile_picture` instead of `id`, `username`, `profile_pic`
- **Solution:** Updated KickUserInfo model to match actual API field names

**Problem 4: Profile pictures not loading in browser**
- **Cause:** Kick blocks external hotlinking with 403 Forbidden
- **Solution:** Documented limitation; frontend uses placeholder fallback

### Common Issues

**OAuth callback fails:**
- Verify `KICK_CLIENT_ID` and `KICK_CLIENT_SECRET` are set correctly
- Check `KICK_REDIRECT_URL` matches what's configured in Kick Developer Portal
- Ensure Redis is running for state/verifier storage

**User info fetch fails:**
- Verify `user:read` scope is included in OAuth request
- Check that access token is valid and not expired

## Questions / Decisions Needed

1. Should we use official Pusher SDK or implement custom WebSocket client?
2. How to handle Kick channel discovery (API call vs database)?
3. Should Kick Listener support multiple WebSocket connections or single connection with multiple subscriptions?
4. How to handle Kick API rate limits?
5. Should we store Kick OAuth tokens in separate table or reuse existing users table?
