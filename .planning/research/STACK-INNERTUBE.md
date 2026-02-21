# Technology Stack: InnerTube YouTube Listener

**Project:** All-Chat InnerTube YouTube Listener
**Researched:** 2026-02-21
**Confidence:** MEDIUM (ecosystem verified, versions confirmed, integration patterns validated)

## Executive Summary

**Recommendation:** Node.js service with masterchat library, called from Go services via HTTP/gRPC. Do NOT rewrite in Go.

**Rationale:** InnerTube ecosystem is strongest in JavaScript/TypeScript (masterchat, YouTube.js) and Python (pytchat - archived). No mature Go InnerTube libraries exist for live chat. The 3-year maturity advantage of masterchat outweighs language consistency concerns. Go ↔ Node.js integration via HTTP is standard microservices pattern, already proven in All-Chat architecture (API Gateway WebSocket, existing service mesh).

**Trade-off:** Language heterogeneity (Go + Node.js) vs ecosystem maturity. Chose maturity.

---

## Recommended Stack

### Core Framework

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **Node.js** | 20 LTS (20.18+) | Runtime | Active LTS until 2026-10, masterchat targets Node.js ecosystem |
| **TypeScript** | 5.x | Type safety | Matches masterchat's TypeScript implementation, reduces bugs |
| **masterchat** | [@stu43005/masterchat](https://www.npmjs.com/package/@stu43005/masterchat) 1.5.0 | InnerTube client | Most mature library, 20+ action types, active fork (April 2025 update) |

**Alternative considered:** [sigvt/masterchat](https://github.com/sigvt/masterchat) v1.1.0 (June 2022) - original but stale. HolodexNet/masterchat is fork from sigvt but unclear maintenance status. **Use @stu43005/masterchat** - most recent updates, published to npm.

### Redis Integration

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **ioredis** | 5.x | Redis client | Industry standard, Streams + Pub/Sub support, TypeScript types |
| **redis-streams** | Built-in | Message queue | XADD to `chat:raw` stream (same contract as youtube-listener) |

**Why not go-redis:** Node.js service needs Node.js Redis client. ioredis has superior TypeScript support vs node-redis.

### Inter-Service Communication

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| **Express** | 4.x | HTTP server | Health checks, status endpoints (optional - can use Node built-in http) |
| **@grpc/grpc-js** | 1.x (optional) | gRPC server | If Go services need high-performance RPC (alternative to HTTP) |

**Decision:** Start with HTTP REST (simpler), migrate to gRPC if performance bottleneck discovered. Existing Go services already use HTTP (`SOURCE_MANAGER_URL`, `overlay-manager` → `youtube-listener` quota tracking).

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| **winston** | 3.x | Structured logging | Match Go zap logging patterns (JSON output) |
| **uuid** | 9.x | Message IDs | Generate `message_id` for RawChatMessage |
| **dotenv** | 16.x | Config | Load .env (dev only, K8s uses ConfigMaps) |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| **InnerTube Library** | @stu43005/masterchat (Node.js) | pytchat (Python) | **ARCHIVED** Jan 2022, unmaintained |
| **InnerTube Library** | @stu43005/masterchat | YouTube.js | No live chat docs found, v16.0.1 (Oct 2025) active but unclear chat support |
| **InnerTube Library** | @stu43005/masterchat | innertube-go (Go) | **No live chat support**, v0.0.0 unstable, no OAuth |
| **Runtime** | Node.js 20 LTS | Go | No mature Go InnerTube libraries exist |
| **Redis Client** | ioredis | node-redis | ioredis has better TypeScript support, widely adopted |
| **HTTP Framework** | Express | Fastify | Express more familiar, adequate for health checks |

### Why NOT YouTube.js?

[YouTube.js](https://github.com/LuanRT/YouTube.js) v16.0.1 (Oct 2025) is actively maintained (14.5k dependents, 104 releases) but:
- Documentation doesn't detail live chat API methods
- Codebase large and complex (full YouTube API wrapper)
- masterchat is **purpose-built for live chat**, 3+ years proven

**Confidence:** LOW - didn't verify YouTube.js chat capabilities exhaustively, but masterchat's specialized focus is safer bet.

### Why NOT pytchat?

[pytchat](https://github.com/taizan-hokuto/pytchat) was strong candidate (Python InnerTube client) but:
- **Repository archived Jan 25, 2022** (3+ years unmaintained)
- Last release v0.5.5 (July 2021)
- No updates for InnerTube API changes
- Adding Python runtime increases operational complexity (Go + Node.js already heterogeneous)

### Why NOT Go Native?

Searched for Go InnerTube libraries:
- [innertube-go](https://pkg.go.dev/github.com/nezbut/innertube-go): No live chat methods, v0.0.0 unstable, no OAuth
- [youtube-go](https://github.com/wslyyy/youtube-go): No live chat docs
- **Finding:** Go InnerTube ecosystem immature for live chat

**Could we build Go InnerTube client from scratch?**
- Feasible: Reverse-engineer InnerTube protocol from masterchat/YouTube.js
- Cost: 2-4 weeks implementation + maintenance burden (InnerTube API changes unpredictably)
- Risk: Breaking changes from YouTube (unofficial API)
- **Decision:** NOT WORTH IT. Use battle-tested masterchat.

---

## Integration Architecture

### Service Boundary

```
┌─────────────────────────────────────────────────────────┐
│ innertube-listener (Node.js + TypeScript)              │
│                                                         │
│  ┌──────────────┐    ┌─────────────┐    ┌───────────┐ │
│  │ masterchat   │───▶│ Normalizer  │───▶│  Redis    │ │
│  │ (InnerTube)  │    │ (to unified │    │  Streams  │ │
│  └──────────────┘    │  format)    │    │ (chat:raw)│ │
│         │            └─────────────┘    └───────────┘ │
│         │                                              │
│  ┌──────▼────────┐   ┌─────────────────────────────┐  │
│  │ Stream        │   │ Health/Status HTTP Endpoint │  │
│  │ Discovery     │   └─────────────────────────────┘  │
│  └───────────────┘                                     │
└─────────────────────────────────────────────────────────┘
         │
         │ HTTP REST (or gRPC)
         │
         ▼
┌─────────────────────────────────────────────────────────┐
│ Go Services (overlay-manager, source-manager, etc)     │
│  - Request stream monitoring: POST /streams/monitor     │
│  - Stop stream monitoring: DELETE /streams/{streamId}   │
│  - Health checks: GET /health/ready                     │
│  - Status query: GET /status                            │
└─────────────────────────────────────────────────────────┘
```

### Communication Patterns

**Go → Node.js (Control Plane):**
- overlay-manager calls innertube-listener to start/stop monitoring streams
- HTTP REST with JSON (matches existing `youtube-listener` → `source-manager` pattern)
- Authorization: Shared secret (env var `SERVICE_SECRET`, matches existing pattern)

**Node.js → Redis (Data Plane):**
- Publish `RawChatMessage` JSON to Redis Streams `chat:raw`
- Same format as existing youtube-listener (drop-in replacement contract)
- No Go service awareness needed (decoupled via Redis)

**Docker Compose Networking:**
```yaml
services:
  innertube-listener:
    image: allchat-innertube-listener
    networks:
      - allchat-network
    environment:
      REDIS_HOST: redis
      SOURCE_MANAGER_URL: http://source-manager:8088

  overlay-manager:
    # Go service calls innertube-listener:8087
    environment:
      INNERTUBE_LISTENER_URL: http://innertube-listener:8087
```

Services use **service name as hostname** in Docker Compose (not localhost). Existing pattern in All-Chat.

---

## Installation

### Prerequisites

```bash
# Node.js 20 LTS
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# Or with nvm
nvm install 20
nvm use 20
```

### Dependencies

```json
{
  "dependencies": {
    "@stu43005/masterchat": "^1.5.0",
    "ioredis": "^5.3.0",
    "uuid": "^9.0.0",
    "winston": "^3.11.0",
    "express": "^4.18.0"
  },
  "devDependencies": {
    "@types/node": "^20.10.0",
    "@types/express": "^4.17.0",
    "@types/uuid": "^9.0.0",
    "typescript": "^5.3.0",
    "ts-node": "^10.9.0",
    "nodemon": "^3.0.0"
  }
}
```

```bash
npm install
```

### TypeScript Configuration

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "lib": ["ES2022"],
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

---

## Docker Image

### Multi-Stage Build (Optimized)

```dockerfile
# Build stage
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY tsconfig.json ./
COPY src ./src
RUN npm run build

# Production stage
FROM node:20-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev
COPY --from=builder /app/dist ./dist
USER node
EXPOSE 8087
CMD ["node", "dist/index.js"]
```

**Why Alpine:** Smaller image (50MB vs 300MB node:20), faster pulls, security (less attack surface).

---

## Contract: RawChatMessage Output

**CRITICAL:** InnerTube listener MUST output identical format to official youtube-listener for drop-in replacement.

### JSON Schema

```typescript
interface RawChatMessage {
  message_id: string;           // UUID v4
  platform: "youtube";          // Fixed string
  channel_id: string;           // YouTube channel ID (UC...)
  stream_id?: string;           // YouTube video ID (optional)
  user_id: string;              // YouTube user channel ID
  username: string;             // Display name
  text: string;                 // Message text
  timestamp: string;            // ISO 8601 UTC (YYYY-MM-DDTHH:mm:ss.sssZ)
  tags: {
    channel_url?: string;
    profile_image?: string;
    is_verified?: "true" | "false";
    is_owner?: "true" | "false";
    is_sponsor?: "true" | "false";
    is_moderator?: "true" | "false";
    super_chat?: string;        // Amount in micros ("0" if not super chat)
    super_sticker?: string;     // Amount in micros ("0" if not super sticker)
  };

  // Event support (backwards compatible)
  event_type?: string;          // "super_chat", "member_milestone"
  event_data?: Record<string, unknown>;
}
```

### Masterchat Action Mapping

| Masterchat Action | event_type | Notes |
|-------------------|------------|-------|
| `addChatItemAction` | (omit) | Regular chat message |
| `addSuperChatItemAction` | `"super_chat"` | Set `tags.super_chat` amount |
| `addSuperStickerItemAction` | `"super_sticker"` | Set `tags.super_sticker` amount |
| `addMembershipItemAction` | `"member_join"` | New sponsor |
| `addMembershipMilestoneItemAction` | `"member_milestone"` | Milestone message |
| `markChatItemAsDeletedAction` | (special) | Handle in processor or ignore |
| `markChatItemsByAuthorAsDeletedAction` | (special) | Ban/timeout event |

**Message Deletion Support:** Existing youtube-listener doesn't publish deletions to `chat:raw`. InnerTube listener can detect deletions (`markChatItemAsDeletedAction`) but should maintain contract parity. Consider Phase 2 feature if frontend needs deletion support.

---

## Sources & Confidence

### HIGH Confidence
- **masterchat capabilities**: [Manual documentation](https://github.com/sigvt/masterchat/blob/master/MANUAL.md), [action types verified](https://github.com/sigvt/masterchat/blob/master/MANUAL.md)
- **@stu43005/masterchat version**: [npm package](https://www.npmjs.com/package/@stu43005/masterchat) 1.5.0 (published 10 months ago from Feb 2026 = April 2025)
- **RawChatMessage contract**: youtube-listener source code verified
- **Docker Compose networking**: [Docker forums](https://forums.docker.com/t/cross-container-communication-via-http-post-request/54605), [Medium article](https://medium.com/@datails/full-stack-development-with-docker-compose-c517ec826696)

### MEDIUM Confidence
- **pytchat archived status**: [GitHub verified](https://github.com/taizan-hokuto/pytchat) "archived Jan 25, 2022"
- **innertube-go limitations**: [pkg.go.dev documentation](https://pkg.go.dev/github.com/nezbut/innertube-go) - no live chat methods listed
- **Go ↔ Node.js gRPC**: [Medium tutorial](https://medium.com/nerd-for-tech/build-a-microservice-app-using-grpc-python-and-golang-part-2-ac93541e4d0d), [Real Python guide](https://realpython.com/python-microservices-grpc/) (Python examples but pattern applies)

### LOW Confidence
- **YouTube.js live chat support**: No official docs found, inferred from general "live chat" mention. Needs verification if masterchat fails.
- **HolodexNet/masterchat fork status**: [GitHub shows fork](https://github.com/HolodexNet/masterchat) but unclear if actively maintained vs sigvt/masterchat.

### Verification Needed
- [ ] masterchat stream discovery API (not in MANUAL.md excerpt) - verify with full docs or code
- [ ] Reconnection handling in masterchat (error events documented, auto-reconnect unclear)
- [ ] YouTube.js as fallback option (if needed)

---

## Open Questions

1. **Quota tracking**: Official youtube-listener has sophisticated reserve-confirm-rollback quota system. InnerTube doesn't use official API so no quota limits. Do we need usage tracking for monitoring? (Answer: Yes, track message rate for observability, but no quota DB needed)

2. **OAuth tokens**: Official listener uses OAuth for per-user authenticated access. InnerTube works unauthenticated (web scraping). Do we need credentials storage? (Answer: No, major simplification)

3. **Stream discovery**: How does masterchat discover live streams? `Masterchat.init(videoId)` requires knowing video ID upfront. Need channel → video ID resolution (might need overlay-manager to pass video ID, or implement channel monitoring). **Flag for Phase 1 research.**

4. **Reconnection strategy**: masterchat emits "error" events. Do we need manual reconnect logic or is it built-in? **Flag for implementation phase.**

5. **Rate limiting**: YouTube may rate-limit InnerTube requests. Monitor in production. **Flag for Phase 2 if issues discovered.**

---

## Next Steps

1. **Verify stream discovery**: Read masterchat full docs or source code for channel monitoring
2. **Prototype**: Build minimal Node.js service with masterchat → Redis Streams
3. **Contract validation**: Test RawChatMessage output matches message-processor expectations
4. **Integration test**: Go overlay-manager HTTP calls to Node.js service
5. **Load test**: Verify ioredis can handle message throughput (2-5 second polling intervals)
