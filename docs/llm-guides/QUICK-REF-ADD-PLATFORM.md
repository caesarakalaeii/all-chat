# Quick Reference: Add New Platform

**Time Estimate**: 6-8 hours | **Difficulty**: ⭐⭐⭐ Moderate

**Goal**: Implement support for a new streaming platform (e.g., Rumble, Facebook Gaming) by creating a new listener service and integrating it into the message processing pipeline.

---

## Prerequisites

- [ ] Platform API documentation reviewed
- [ ] Platform chat access method confirmed (IRC, WebSocket, HTTP polling)
- [ ] OAuth requirements identified (if any)
- [ ] Example chat message structure documented

---

## Step 1: Choose Template (Decision Tree)

```
Does the platform use IRC for chat?
├─ YES → Use twitch-listener as template
│         Files: services/twitch-listener/*
│         Pattern: IRC client with JOIN/PART
│         Auth: Bot username + OAuth token
│
└─ NO → Does the platform have a WebSocket API?
        ├─ YES → Use kick-listener as template
        │         Files: services/kick-listener/*
        │         Pattern: WebSocket client with subscriptions
        │         Auth: Public WebSocket (or API key)
        │
        └─ NO → Use youtube-listener as template
                  Files: services/youtube-listener/*
                  Pattern: HTTP polling with leader election
                  Auth: OAuth 2.0 per user
```

**Output**: Template choice → `<template>-listener` (twitch/kick/youtube)

---

## Step 2: Create Listener Service

### 2.1 Create Directory Structure

```bash
# From repository root
mkdir -p services/<platform>-listener/{cmd,handlers,channels,publisher}
cd services/<platform>-listener
```

**Files to create:**
```
services/<platform>-listener/
├── cmd/
│   └── main.go           # Service entry point
├── handlers/
│   ├── health.go         # Health check endpoints
│   └── status.go         # Status endpoint (optional)
├── channels/
│   └── manager.go        # Channel synchronization logic
├── publisher/
│   └── redis.go          # Publish to Redis Streams
├── <client-package>/     # Platform-specific client
│   ├── client.go         # IRC/WebSocket/HTTP client
│   └── parser.go         # Message parsing
├── go.mod                # Module dependencies
├── Dockerfile            # Container image
└── README.md             # Service documentation
```

### 2.2 Copy Template Files

**For IRC-based platform (like Twitch):**
```bash
cp -r services/twitch-listener/cmd services/<platform>-listener/
cp -r services/twitch-listener/irc services/<platform>-listener/<client-package>
cp -r services/twitch-listener/channels services/<platform>-listener/
cp -r services/twitch-listener/publisher services/<platform>-listener/
cp services/twitch-listener/Dockerfile services/<platform>-listener/
```

**For WebSocket platform (like Kick):**
```bash
cp -r services/kick-listener/cmd services/<platform>-listener/
cp -r services/kick-listener/websocket services/<platform>-listener/<client-package>
cp -r services/kick-listener/channels services/<platform>-listener/
cp -r services/kick-listener/publisher services/<platform>-listener/
cp services/kick-listener/Dockerfile services/<platform>-listener/
```

**For HTTP polling platform (like YouTube):**
```bash
cp -r services/youtube-listener/cmd services/<platform>-listener/
cp -r services/youtube-listener/youtube services/<platform>-listener/<client-package>
cp -r services/youtube-listener/streams services/<platform>-listener/
cp -r services/youtube-listener/channels services/<platform>-listener/
cp -r services/youtube-listener/publisher services/<platform>-listener/
cp services/youtube-listener/Dockerfile services/<platform>-listener/
```

### 2.3 Initialize Go Module

```bash
cd services/<platform>-listener
go mod init github.com/caesarakalaeii/all-chat/services/<platform>-listener
go mod edit -replace github.com/caesarakalaeii/all-chat/shared=../../shared
go get github.com/gin-gonic/gin
go get github.com/go-redis/redis/v9
# Add platform-specific dependencies
```

### 2.4 Update cmd/main.go

**File**: `services/<platform>-listener/cmd/main.go`

**Changes**:
1. Update service name in logger initialization:
   ```go
   logger := shared_logger.InitLogger("<platform>-listener")
   ```

2. Update port (increment from last listener):
   ```go
   port := os.Getenv("PORT")
   if port == "" {
       port = "8090"  // Choose next available port
   }
   ```

3. Initialize platform-specific client:
   ```go
   // Example for IRC
   client := irc.NewClient(username, oauthToken, logger)

   // Example for WebSocket
   wsClient := websocket.NewClient(websocketURL, logger)

   // Example for HTTP
   httpClient := platform.NewClient(apiKey, logger)
   ```

4. Update health check service name

### 2.5 Implement Client Logic

**File**: `services/<platform>-listener/<client-package>/client.go`

**Required methods**:
```go
type Client interface {
    Connect(ctx context.Context) error
    Disconnect() error
    JoinChannel(channel string) error    // IRC/WebSocket
    LeaveChannel(channel string) error   // IRC/WebSocket
    OnMessage(handler func(RawMessage))  // Message callback
}
```

**Implementation checklist**:
- [ ] Connection establishment (with retry logic)
- [ ] Channel join/subscription logic
- [ ] Message parsing from platform format
- [ ] Reconnection on disconnect
- [ ] Graceful shutdown

### 2.6 Implement Channel Manager

**File**: `services/<platform>-listener/channels/manager.go`

**Database query to sync channels:**
```go
const queryActiveChannels = `
    SELECT DISTINCT ocs.overlay_id, ocs.channel_identifier
    FROM overlay_chat_sources ocs
    JOIN overlays o ON o.id = ocs.overlay_id
    WHERE o.is_active = true
      AND ocs.platform = $1
      AND ocs.is_active = true
`

// Use in sync loop
rows, err := db.Query(ctx, queryActiveChannels, "<platform>")
```

**Sync logic**:
1. Query active channels from database (every 30s)
2. Compare with currently joined channels
3. Join new channels (add to client)
4. Leave removed channels (remove from client)
5. Log join/leave operations

### 2.7 Implement Redis Publisher

**File**: `services/<platform>-listener/publisher/redis.go`

**Message format for Redis Streams (`chat:raw`):**
```go
message := map[string]interface{}{
    "platform":    "<platform>",
    "overlay_id":  overlayID,
    "channel_id":  channelID,
    "channel_name": channelName,
    "raw_message": rawMessage,  // Full platform message as JSON
    "timestamp":   time.Now().UTC().Format(time.RFC3339),
}

err := redisClient.XAdd(ctx, &redis.XAddArgs{
    Stream: "chat:raw",
    Values: message,
}).Err()
```

---

## Step 3: Add Message Normalizer

### 3.1 Create Normalizer File

**File**: `services/message-processor/normalizer/<platform>_normalizer.go`

```bash
cd services/message-processor/normalizer
touch <platform>_normalizer.go
```

### 3.2 Implement Normalizer

```go
package normalizer

import (
    "encoding/json"
    "time"
    "github.com/caesarakalaeii/all-chat/services/message-processor/models"
)

// Parse<Platform>Message converts platform-specific message to unified format
func Parse<Platform>Message(rawMsg map[string]interface{}) (*models.UnifiedMessage, error) {
    // 1. Extract platform message from raw_message field
    var platformMsg <Platform>Message
    if err := json.Unmarshal(rawMsg["raw_message"], &platformMsg); err != nil {
        return nil, err
    }

    // 2. Create unified message structure
    unified := &models.UnifiedMessage{
        ID:          generateUUID(),
        OverlayID:   rawMsg["overlay_id"].(string),
        Platform:    "<platform>",
        ChannelID:   rawMsg["channel_id"].(string),
        ChannelName: rawMsg["channel_name"].(string),
        User: models.User{
            ID:          platformMsg.UserID,
            Username:    platformMsg.Username,
            DisplayName: platformMsg.DisplayName,
            AvatarURL:   platformMsg.AvatarURL,
            Color:       platformMsg.Color,
            Badges:      extractBadges(platformMsg),
        },
        Message: models.Message{
            Text:   platformMsg.Text,
            Emotes: extractEmotes(platformMsg),
        },
        Timestamp: parseTimestamp(platformMsg.Timestamp),
        Metadata:  extractMetadata(platformMsg),
    }

    return unified, nil
}

// Helper functions
func extractBadges(msg <Platform>Message) []string {
    // Convert platform badges to string array
    // Example: ["subscriber", "moderator"]
}

func extractEmotes(msg <Platform>Message) []models.Emote {
    // Extract platform-native emotes with positions
    // Example: Twitch emotes from IRC tags
}

func extractMetadata(msg <Platform>Message) models.Metadata {
    // Extract platform-specific metadata
    return models.Metadata{
        IsSubscriber: msg.IsSubscriber,
        IsModerator:  msg.IsModerator,
        // Add platform-specific fields
    }
}
```

### 3.3 Update Router

**File**: `services/message-processor/router/router.go`

Add case for new platform:
```go
func (r *Router) RouteMessage(rawMsg map[string]interface{}) (*models.UnifiedMessage, error) {
    platform, ok := rawMsg["platform"].(string)
    if !ok {
        return nil, errors.New("missing platform field")
    }

    switch platform {
    case "twitch":
        return normalizer.ParseTwitchMessage(rawMsg)
    case "youtube":
        return normalizer.ParseYouTubeMessage(rawMsg)
    case "kick":
        return normalizer.ParseKickMessage(rawMsg)
    case "<platform>":  // ADD THIS CASE
        return normalizer.Parse<Platform>Message(rawMsg)
    default:
        return nil, fmt.Errorf("unsupported platform: %s", platform)
    }
}
```

---

## Step 4: Database Migration

### 4.1 Create Migration File

```bash
cd migrations
# Find next migration number
ls -1 *.sql | tail -1  # e.g., 022_stream_sessions.sql
touch 023_<platform>_support.sql
```

### 4.2 Write Migration SQL

**File**: `migrations/023_<platform>_support.sql`

```sql
-- Add new platform to supported_platforms table
INSERT INTO supported_platforms (name, display_name, requires_oauth, icon_url)
VALUES ('<platform>', '<Platform>', false, 'https://example.com/icon.png')
ON CONFLICT (name) DO NOTHING;

-- Add any platform-specific tables (if needed)
-- Example: OAuth tokens table
CREATE TABLE IF NOT EXISTS <platform>_oauth_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    channel_id VARCHAR(255) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    token_type VARCHAR(50) DEFAULT 'Bearer',
    expiry TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(user_id, channel_id)
);

-- Update metadata schema if needed
COMMENT ON COLUMN overlay_chat_sources.metadata IS
'Platform-specific metadata (JSONB). Examples:
- YouTube: {"video_id": "abc123"}
- Kick: {"chatroom_id": 123456}
- <Platform>: {"<key>": "<value>"}';
```

### 4.3 Run Migration

```bash
make migrate
# Or manually:
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat -f migrations/023_<platform>_support.sql
```

---

## Step 5: Kubernetes Deployment

### 5.1 Create Deployment Files

```bash
mkdir -p deployments/k8s/base/<platform>-listener
cd deployments/k8s/base/<platform>-listener
```

**Files to create:**
- `deployment.yaml` - Pod specification
- `service.yaml` - ClusterIP service
- `hpa.yaml` - Horizontal Pod Autoscaler
- `kustomization.yaml` - Kustomize config

### 5.2 deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: <platform>-listener
  namespace: allchat
  labels:
    app: <platform>-listener
spec:
  replicas: 2
  selector:
    matchLabels:
      app: <platform>-listener
  template:
    metadata:
      labels:
        app: <platform>-listener
    spec:
      containers:
      - name: <platform>-listener
        image: ghcr.io/caesarakalaeii/allchat-<platform>-listener:main
        ports:
        - containerPort: 8090  # Use chosen port
        env:
        - name: DATABASE_HOST
          value: allchat-cluster-rw
        - name: DATABASE_PORT
          value: "5432"
        - name: DATABASE_NAME
          value: allchat
        - name: DATABASE_USER
          valueFrom:
            secretKeyRef:
              name: allchat-db-secret
              key: username
        - name: DATABASE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: allchat-db-secret
              key: password
        - name: REDIS_HOST
          value: redis
        - name: REDIS_PORT
          value: "6379"
        - name: LOG_LEVEL
          value: info
        # Add platform-specific env vars
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8090
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8090
          initialDelaySeconds: 5
          periodSeconds: 10
```

### 5.3 service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  name: <platform>-listener
  namespace: allchat
spec:
  selector:
    app: <platform>-listener
  ports:
  - port: 8090
    targetPort: 8090
    protocol: TCP
  type: ClusterIP
```

### 5.4 hpa.yaml

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: <platform>-listener
  namespace: allchat
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: <platform>-listener
  minReplicas: 2
  maxReplicas: 5
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

---

## Step 6: Update Documentation

### 6.1 Create Service README

**File**: `services/<platform>-listener/README.md`

Use template from `services/twitch-listener/README.md`. Include:
- [ ] Features list
- [ ] Architecture diagram (ASCII)
- [ ] Environment variables (required + optional)
- [ ] API endpoints (health checks)
- [ ] Message format (Redis Streams)
- [ ] Running locally instructions
- [ ] Testing commands
- [ ] Troubleshooting common issues

### 6.2 Update CLAUDE.md

**File**: `CLAUDE.md`

Add to "Service Details" section:
```markdown
### <Platform> Listener (Port 8090) ✅
**Purpose**: Connect to <Platform> chat, publish messages to Redis Streams

**Key Files**:
- `services/<platform>-listener/cmd/main.go` - Entry point
- `services/<platform>-listener/<client-package>/client.go` - Platform client
- `services/<platform>-listener/channels/manager.go` - Channel management
- `services/<platform>-listener/publisher/redis.go` - Redis publisher

**Features**:
- <Platform> <connection-type> connection
- Dynamic channel management
- Rate limiting (if applicable)
- Publishes to Redis Streams (`chat:raw`)
- Health checks and status

**Environment**:
- `<PLATFORM>_API_KEY=` (or OAuth credentials)
```

### 6.3 Update Platform Status

**File**: `CLAUDE.md` (Project Overview section)

Update platform status:
```markdown
**Platform Status**:
- ✅ **Twitch**: Fully implemented
- ✅ **YouTube**: Fully implemented
- ✅ **Kick**: Fully implemented
- ✅ **<Platform>**: Fully implemented  # ADD THIS
```

---

## Step 7: Testing

### 7.1 Unit Tests

Create test files for each component:

```bash
# Listener client tests
touch services/<platform>-listener/<client-package>/client_test.go

# Normalizer tests
touch services/message-processor/normalizer/<platform>_normalizer_test.go
```

**Example test structure:**
```go
func TestParse<Platform>Message(t *testing.T) {
    rawMsg := map[string]interface{}{
        "platform":    "<platform>",
        "overlay_id":  "test-overlay-id",
        "channel_id":  "test-channel",
        "channel_name": "TestChannel",
        "raw_message": json.RawMessage(`{...}`),
        "timestamp":   "2025-01-28T10:00:00Z",
    }

    unified, err := normalizer.Parse<Platform>Message(rawMsg)

    assert.NoError(t, err)
    assert.Equal(t, "<platform>", unified.Platform)
    assert.NotEmpty(t, unified.User.Username)
    // Add more assertions
}
```

### 7.2 Integration Tests

**Test checklist:**
- [ ] Service starts successfully
- [ ] Health endpoints respond (200 OK)
- [ ] Database connection established
- [ ] Redis connection established
- [ ] Channels sync from database
- [ ] Messages published to Redis Streams
- [ ] Message format matches specification

**Commands:**
```bash
# Run listener service
cd services/<platform>-listener
go run ./cmd &
LISTENER_PID=$!

# Check health
curl http://localhost:8090/health/live
curl http://localhost:8090/health/ready

# Check Redis Stream
redis-cli XREAD COUNT 10 STREAMS chat:raw 0

# Stop listener
kill $LISTENER_PID
```

### 7.3 End-to-End Test

1. Create test overlay with <platform> source
2. Activate overlay
3. Verify listener joins channel
4. Send test message in platform chat
5. Verify message appears in Redis Stream
6. Verify message processed by message-processor
7. Verify message appears in API Gateway WebSocket

---

## Validation Checklist

### Code Quality
- [ ] All imports use absolute paths (github.com/caesarakalaeii/all-chat/...)
- [ ] No hardcoded credentials
- [ ] Error handling on all external calls
- [ ] Structured logging (Zap) with context
- [ ] Graceful shutdown implemented
- [ ] Health checks return correct status

### Documentation
- [ ] Service README created (`services/<platform>-listener/README.md`)
- [ ] CLAUDE.md updated with service details
- [ ] Migration SQL documented
- [ ] Platform status updated (✅ marker)

### Deployment
- [ ] Dockerfile builds successfully
- [ ] Kubernetes manifests created (deployment, service, HPA)
- [ ] Environment variables documented
- [ ] Resource limits set appropriately

### Integration
- [ ] Messages published to `chat:raw` stream
- [ ] Normalizer converts to unified format
- [ ] Router case added for platform
- [ ] Database migration applied
- [ ] Platform added to `supported_platforms` table

### Testing
- [ ] Unit tests written and passing
- [ ] Integration tests verify message flow
- [ ] Health endpoints return correct status
- [ ] End-to-end test with real chat message

---

## Common Issues & Solutions

### Issue 1: Messages Not Appearing in Redis Stream

**Symptom**: Listener logs show messages received, but Redis Stream is empty

**Solutions**:
1. Check Redis connection: `redis-cli ping`
2. Verify stream name is `chat:raw` (exact match)
3. Check Redis permissions
4. Verify message format matches specification

**Command**:
```bash
redis-cli XINFO STREAM chat:raw
redis-cli XREAD COUNT 10 STREAMS chat:raw 0
```

### Issue 2: Channel Sync Not Working

**Symptom**: Listener doesn't join channels from database

**Solutions**:
1. Check database query returns channels: `SELECT ... FROM overlay_chat_sources WHERE platform='<platform>'`
2. Verify `is_active=true` for overlays and sources
3. Check sync interval (default 30s)
4. Review logs for sync errors

**File**: `services/<platform>-listener/channels/manager.go:sync()`

### Issue 3: Normalizer Fails to Parse

**Symptom**: Message Processor logs parsing errors

**Solutions**:
1. Verify platform message structure matches code
2. Check for nil pointer dereferences
3. Add defensive nil checks
4. Log raw message for debugging

**File**: `services/message-processor/normalizer/<platform>_normalizer.go`

---

## Related Documentation

- [DATA_FLOW_INTEGRATION.md](../architecture/01-DATA-FLOW.md) - Message flow architecture
- [Twitch Listener README](../../services/twitch-listener/README.md) - IRC template
- [Kick Listener README](../../services/kick-listener/README.md) - WebSocket template
- [YouTube Listener README](../../services/youtube-listener/README.md) - HTTP polling template
- [Message Processor README](../../services/message-processor/README.md) - Normalizer details

---

## Success Criteria

✅ Task complete when:
1. New listener service built and running
2. Messages published to Redis Streams with correct format
3. Normalizer converts platform messages to unified format
4. End-to-end test passes (platform chat → overlay display)
5. All tests passing
6. Documentation updated (service README + CLAUDE.md)
7. Kubernetes deployment manifests created
