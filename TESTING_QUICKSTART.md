# All-Chat Testing - Quick Start Guide

**Time Required**: 30 minutes for basic testing
**Full Test Suite**: See `docs/TESTING_COMPREHENSIVE.md` (2-4 hours)

---

## Step 1: Fill in Environment Variables (5 minutes)

```bash
cd /home/moersener/Hobby/all-chat
cp .env.example deployments/.env
nano deployments/.env  # or vim
```

**Required credentials:**
- `TWITCH_CLIENT_ID` - From https://dev.twitch.tv/console/apps
- `TWITCH_CLIENT_SECRET` - From Twitch app
- `TWITCH_BOT_USERNAME` - Your Twitch bot account
- `TWITCH_BOT_OAUTH` - From https://twitchapps.com/tmi/
- `YOUTUBE_CLIENT_ID` - From https://console.cloud.google.com/
- `YOUTUBE_CLIENT_SECRET` - From Google Cloud Console
- `JWT_SECRET` - Generate with: `openssl rand -base64 32`

---

## Step 2: Run Quick Test (5 minutes)

```bash
# Run the quick test script
curl -fsSL https://raw.githubusercontent.com/caesarakalaeii/all-chat/main/scripts/quick-test.sh | bash

# Or manually:
cd /home/moersener/Hobby/all-chat

# Run unit tests
go test -short ./...

# Start infrastructure
cd deployments
docker-compose up -d postgres redis
sleep 10

# Apply migrations
cd ..
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat < migrations/001_initial_schema.sql
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat < migrations/003_youtube_support.sql

# Create consumer group
redis-cli -h localhost XGROUP CREATE chat:raw message-processor $ MKSTREAM

# Start all services
cd deployments
docker-compose up -d
sleep 30

# Check health
curl http://localhost:8080/health
curl http://localhost:8086/status | jq
curl http://localhost:8088/status | jq
```

**Expected**: All services healthy

---

## Step 3: Test Twitch Integration (10 minutes)

```bash
# 1. Get OAuth token
curl http://localhost:8080/api/v1/auth/login
# Open returned URL in browser, authorize, get JWT token

TOKEN="paste-your-jwt-token-here"

# 2. Create overlay
OVERLAY_ID=$(curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Overlay"}' \
  http://localhost:8080/api/v1/overlays | jq -r '.id')

echo "Overlay ID: $OVERLAY_ID"

# 3. Add Twitch source
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"twitch","channel_id":"xqc"}' \
  http://localhost:8080/api/v1/overlays/$OVERLAY_ID/sources | jq

# 4. Wait 30 seconds for Twitch Listener to JOIN
sleep 30

# 5. Check status
curl http://localhost:8085/status | jq '.irc.channels'
# Expected: ["xqc"]

# 6. Connect WebSocket
websocat "ws://localhost:8080/ws/overlay/$OVERLAY_ID?token=$TOKEN"

# 7. Send message in xqc's chat
# Expected: Message appears in WebSocket within 500ms
```

---

## Step 4: Test YouTube Integration (10 minutes)

**Note**: Requires YouTube channel that is currently live streaming

```bash
TOKEN="your-jwt-token"

# 1. Create YouTube overlay
YOUTUBE_OVERLAY_ID=$(curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"YouTube Test"}' \
  http://localhost:8080/api/v1/overlays | jq -r '.id')

# 2. Insert YouTube OAuth token manually
# (In production, this would be through OAuth flow)
PGPASSWORD=allchat_dev_password psql -h localhost -U allchat -d allchat << EOF
INSERT INTO youtube_oauth_tokens (user_id, channel_id, access_token, refresh_token, token_type, expiry)
SELECT
  u.id,
  'UCxxxxxx',  -- Your YouTube channel ID
  'ya29.your-access-token',
  'your-refresh-token',
  'Bearer',
  NOW() + INTERVAL '1 hour'
FROM users u
WHERE u.twitch_id = (
  SELECT twitch_id FROM users ORDER BY created_at DESC LIMIT 1
)
LIMIT 1;
EOF

# 3. Add YouTube source
curl -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"platform":"youtube","channel_id":"UCxxxxxx"}' \
  http://localhost:8080/api/v1/overlays/$YOUTUBE_OVERLAY_ID/sources | jq

# 4. Wait for stream detection
sleep 60

# 5. Check YouTube Listener status
curl http://localhost:8086/status | jq

# Expected: If channel is live, streams.active_count > 0

# 6. Connect WebSocket
websocat "ws://localhost:8080/ws/overlay/$YOUTUBE_OVERLAY_ID?token=$TOKEN"

# 7. Send message in YouTube live chat
# Expected: Message appears within 2-5 seconds
```

---

## Troubleshooting Quick Fixes

### Services Won't Start
```bash
docker-compose down
docker-compose up -d postgres redis
sleep 10
docker-compose up -d
docker-compose logs
```

### Tests Failing
```bash
# Check .env file
cat deployments/.env | grep -v "^#" | grep "="

# Run go mod tidy
cd services/youtube-listener && go mod tidy
cd ../source-manager && go mod tidy

# Re-run tests
cd ../.. && go test -short ./...
```

### No Messages Appearing
```bash
# Check Redis Streams
redis-cli -h localhost XLEN chat:raw

# Check logs
docker-compose logs twitch-listener | tail -50
docker-compose logs message-processor | tail -50

# Check consumer group
redis-cli -h localhost XINFO GROUPS chat:raw
```

---

## Success Criteria

✅ **Basic Testing Complete When:**
- All unit tests pass
- All services start and are healthy
- Twitch OAuth works
- Can create overlay
- Can add Twitch source
- Messages flow from Twitch to WebSocket

✅ **Full Testing Complete When:**
- All above ✅ PLUS
- YouTube OAuth works
- Can add YouTube source
- Messages flow from YouTube to WebSocket
- Multi-platform overlay works
- Leader election verified
- Load tests pass

---

## Next Steps

After testing is complete:

1. **Document Issues**: Note any bugs or problems found
2. **Fix Bugs**: Address any failures
3. **Performance**: Optimize slow components
4. **Deploy**: Use Ansible to deploy to production
5. **Monitor**: Set up Grafana dashboards

---

**For complete testing procedures, see: `docs/TESTING_COMPREHENSIVE.md`**

**Created**: November 13, 2025
**Estimated Time**: 30 minutes (quick) or 2-4 hours (comprehensive)
