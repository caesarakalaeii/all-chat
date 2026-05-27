# Development Scripts

Helper scripts for All-Chat development and testing.

## 📋 Available Scripts

### `seed-test-data.sh`

**Purpose**: Creates test user, overlay, and chat sources in the database.

**Usage**:
```bash
./scripts/seed-test-data.sh

# Or via Makefile
make frontend-seed
```

**Environment Variables**:
- `DB_HOST` - Database host (default: localhost)
- `DB_PORT` - Database port (default: 5432)
- `DB_NAME` - Database name (default: allchat)
- `DB_USER` - Database user (default: allchat)
- `DB_PASSWORD` - Database password (default: dev_password_123)

**Creates**:
- Test user: `teststreamer` (ID: `00000000-0000-0000-0000-000000000001`)
- Test overlay: "Frontend Test Overlay" (ID: `00000000-0000-0000-0000-000000000002`)
- Twitch source: `teststreamer` channel
- YouTube source: `UCtest12345` channel

**Requirements**:
- PostgreSQL client (`psql`)
- Database must be running and accessible

---

### `generate-test-messages.sh`

**Purpose**: Generates continuous stream of mock chat messages for testing.

**Usage**:
```bash
./scripts/generate-test-messages.sh

# Or via Makefile
make frontend-messages

# Custom interval (seconds between messages)
MESSAGE_INTERVAL=5 ./scripts/generate-test-messages.sh

# Limited message count
MESSAGE_COUNT=100 ./scripts/generate-test-messages.sh

# Custom overlay ID
TEST_OVERLAY_ID=your-uuid ./scripts/generate-test-messages.sh
```

**Environment Variables**:
- `REDIS_HOST` - Redis host (default: localhost)
- `REDIS_PORT` - Redis port (default: 6379)
- `MESSAGE_PROCESSOR_URL` - Message processor URL (default: http://localhost:8087)
- `MESSAGE_PROCESSOR_API_KEY` - Token for `X-Internal-Token` header (default: dev-frontend-key)
- `TEST_OVERLAY_ID` - Target overlay ID (default: test overlay UUID)
- `MESSAGE_INTERVAL` - Seconds between messages (default: 3)
- `MESSAGE_COUNT` - Number of messages to send (default: 0 = infinite)

**Requirements**:
- `curl` command
- `redis-cli` command
- Message processor service running

**Customization**:

Edit the script to modify:
- `MESSAGES` array - Different message texts
- `USERNAMES` array - Different usernames
- `PLATFORMS` array - Twitch, YouTube, Kick, TikTok

---

### `verify-frontend-setup.sh`

**Purpose**: Verifies all frontend development services are healthy and ready.

**Usage**:
```bash
./scripts/verify-frontend-setup.sh

# Or via Makefile
make frontend-verify
```

**Checks**:
- ✅ Docker services running
- ✅ PostgreSQL accessible
- ✅ Redis accessible
- ✅ API Gateway health
- ✅ Overlay Manager health
- ✅ Message Processor health
- ✅ Test data exists

**Requirements**:
- `docker-compose` command
- `psql` command
- `redis-cli` command
- `curl` command

**Exit Codes**:
- `0` - All checks passed
- `1` - One or more checks failed

---

### `test-websocket.js`

**Purpose**: Node.js WebSocket test client to monitor overlay messages.

**Usage**:
```bash
# Install dependencies first (one-time)
cd scripts
npm install

# Run with default overlay
node test-websocket.js

# Run with custom overlay ID
node test-websocket.js 00000000-0000-0000-0000-000000000002

# Custom WebSocket URL
WS_URL=ws://localhost:8080 node test-websocket.js
```

**Environment Variables**:
- `WS_URL` - WebSocket base URL (default: ws://localhost:8080)

**Features**:
- Colored output by platform
- Badge display
- Emote listing
- Message counter
- Automatic reconnection handling
- Heartbeat/ping to keep connection alive

**Requirements**:
- Node.js 14+
- `ws` npm package (see package.json)

**Output Format**:
```
[HH:MM:SS] [PLATFORM] Username: Message text
  [badge1] [badge2]
  Emotes: emote1, emote2
```

---

## 🔧 Script Dependencies

### System Requirements

**All scripts require**:
- Bash shell
- Docker and Docker Compose

**Individual requirements**:

| Script | Requirements |
|--------|--------------|
| `seed-test-data.sh` | `psql` |
| `generate-test-messages.sh` | `curl`, `redis-cli` |
| `verify-frontend-setup.sh` | `psql`, `redis-cli`, `curl` |
| `test-websocket.js` | Node.js, npm |

### Installing Dependencies

**PostgreSQL Client**:
```bash
# Ubuntu/Debian
sudo apt-get install postgresql-client

# macOS
brew install postgresql

# Arch Linux
sudo pacman -S postgresql-libs
```

**Redis Client**:
```bash
# Ubuntu/Debian
sudo apt-get install redis-tools

# macOS
brew install redis

# Arch Linux
sudo pacman -S redis
```

**Node.js**:
```bash
# Ubuntu/Debian
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install nodejs

# macOS
brew install node

# Arch Linux
sudo pacman -S nodejs npm
```

---

## 🎯 Typical Workflow

### 1. First Time Setup
```bash
# Start services
make frontend-dev

# Seed database
./scripts/seed-test-data.sh

# Verify everything
./scripts/verify-frontend-setup.sh
```

### 2. Development Session
```bash
# Terminal 1: Generate messages
./scripts/generate-test-messages.sh

# Terminal 2: Monitor WebSocket
cd scripts && npm install  # First time only
node test-websocket.js

# Terminal 3: Frontend dev server
cd frontend && npm run dev
```

### 3. Testing Specific Scenarios
```bash
# Burst test (100 messages fast)
MESSAGE_COUNT=100 MESSAGE_INTERVAL=0.1 ./scripts/generate-test-messages.sh

# Slow steady messages
MESSAGE_INTERVAL=10 ./scripts/generate-test-messages.sh

# Monitor Redis directly
redis-cli SUBSCRIBE "overlay:00000000-0000-0000-0000-000000000002"
```

### 4. Cleanup
```bash
# Stop services
make frontend-down

# Reset everything
make frontend-reset
```

---

## 🐛 Troubleshooting

### Scripts Won't Execute
```bash
# Make scripts executable
chmod +x scripts/*.sh

# Check for DOS line endings
dos2unix scripts/*.sh
```

### Database Connection Errors
```bash
# Check PostgreSQL is running
docker ps | grep postgres

# Test connection manually
PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat -c '\dt'

# Check environment variables
echo $DB_HOST $DB_PORT $DB_NAME
```

### Message Generator Fails
```bash
# Verify message processor is running
curl http://localhost:8087/health/live

# Check Redis connection
redis-cli -h localhost -p 6379 ping

# Verify API key
echo $MESSAGE_PROCESSOR_API_KEY
```

### WebSocket Test Fails
```bash
# Install dependencies
cd scripts && npm install

# Check WebSocket endpoint
curl http://localhost:8080/health/live

# Verify overlay exists
PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat \
  -c "SELECT id, name FROM overlays WHERE id = '00000000-0000-0000-0000-000000000002';"
```

---

## 📝 Customization Examples

### Add New Message Types
Edit `generate-test-messages.sh`:
```bash
MESSAGES=(
    "Standard message"
    "Message with long text: $(head -c 200 /dev/urandom | base64)"
    "Emoji test: 😀 😃 😄"
    "Special chars: !@#$%^&*()"
)
```

### Add New Test Users
Edit `seed-test-data.sh`:
```sql
INSERT INTO users (id, twitch_id, username, display_name, ...)
VALUES ('custom-uuid', 'twitch_id', 'new_user', 'New User', ...);
```

### Custom WebSocket Monitor
Modify `test-websocket.js`:
```javascript
ws.on('message', (data) => {
  const message = JSON.parse(data);
  // Custom processing logic
  console.log('Custom format:', message);
});
```

---

## 🚀 Performance Tips

### Fast Message Generation
```bash
# Parallel generators for stress testing
for i in {1..5}; do
  MESSAGE_INTERVAL=1 ./scripts/generate-test-messages.sh &
done
```

### Batch Seeding
```bash
# Seed multiple overlays at once
for i in {1..10}; do
  # Modify seed script to accept overlay ID
  TEST_OVERLAY_ID="overlay-$i" ./scripts/seed-test-data.sh
done
```

### Monitor Performance
```bash
# Watch Redis memory
redis-cli INFO memory | grep used_memory_human

# Monitor message rate
redis-cli --stat

# Track service resource usage
docker stats allchat-frontend-gateway allchat-frontend-processor
```

---

## 📚 Related Documentation

- [FRONTEND_QUICK_START.md](../FRONTEND_QUICK_START.md) - Quick start guide
- [FRONTEND_DEV_SETUP.md](../FRONTEND_DEV_SETUP.md) - Complete setup guide
- [.github/FRONTEND_DEV_SUMMARY.md](../.github/FRONTEND_DEV_SUMMARY.md) - Architecture and design

---

## ✅ Script Checklist

Before running scripts, verify:

- [ ] Docker Compose services are running
- [ ] PostgreSQL is accessible
- [ ] Redis is accessible
- [ ] Message Processor is healthy
- [ ] Scripts are executable (`chmod +x`)
- [ ] Required CLI tools installed
- [ ] Node.js dependencies installed (for test-websocket.js)
