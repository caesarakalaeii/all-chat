# All-Chat: Component Architecture

**Version:** 1.0
**Last Updated:** 2025-11-11
**Related Docs**: [Architecture Overview](./ARCHITECTURE_OVERVIEW.md)

---

## Table of Contents

1. [Introduction](#introduction)
2. [Hexagonal Architecture Pattern](#hexagonal-architecture-pattern)
3. [Auth Service](#auth-service)
4. [Overlay Manager](#overlay-manager)
5. [Emote Service](#emote-service)
6. [Source Controller](#source-controller)
7. [Platform Listeners](#platform-listeners)
8. [Message Processor](#message-processor)
9. [API Gateway](#api-gateway)
10. [Shared Packages](#shared-packages)

---

## Introduction

This document provides detailed component-level architecture for each microservice in the All-Chat platform. Every service follows the **Hexagonal Architecture** (Ports & Adapters) pattern to ensure:

- **Testability**: Core business logic is independent of external infrastructure
- **Maintainability**: Infrastructure changes don't require business logic changes
- **Flexibility**: Easy to swap implementations (e.g., PostgreSQL → MongoDB)
- **Clear Boundaries**: Explicit separation between domain and infrastructure

---

## Hexagonal Architecture Pattern

### Standard Directory Structure

All services follow this structure:

```
internal/<service-name>/
├── adapters/              # Infrastructure layer (outer hexagon)
│   ├── api/              # HTTP handlers (Gin)
│   │   ├── handlers.go   # Request/response handling
│   │   ├── dto.go        # Data transfer objects
│   │   └── routes.go     # Route registration
│   ├── repository/       # Database implementations (pgx)
│   │   └── postgres_<entity>_repository.go
│   └── clients/          # External API clients
│       └── <external>_client.go
└── core/                  # Business logic layer (inner hexagon)
    ├── domain/           # Domain models (entities, value objects)
    │   └── <entity>.go
    ├── ports/            # Interfaces (contracts)
    │   ├── service.go    # Service interface
    │   └── repository.go # Repository interface
    └── services/         # Business logic implementations
        └── <entity>_service.go
```

### Dependency Rule

```mermaid
graph LR
    subgraph "Core Domain"
        DOMAIN[Domain Models]
        PORTS[Ports/Interfaces]
        SERVICE[Business Services]
    end

    subgraph "Adapters"
        API[HTTP Handlers]
        REPO[Repositories]
        CLIENT[API Clients]
    end

    subgraph "External"
        HTTP[HTTP Requests]
        DB[(Database)]
        EXT[External APIs]
    end

    HTTP --> API
    API --> PORTS
    PORTS --> SERVICE
    SERVICE --> DOMAIN
    SERVICE --> PORTS
    PORTS --> REPO
    REPO --> DB
    PORTS --> CLIENT
    CLIENT --> EXT

    style DOMAIN fill:#e1f5ff
    style SERVICE fill:#e1f5ff
    style PORTS fill:#ffe1f5
```

**Key Rules**:
- Core domain NEVER imports adapters
- Adapters import and implement ports
- All external dependencies injected via interfaces
- Business logic lives in `core/services/`
- Infrastructure lives in `adapters/`

---

## Auth Service

**Port**: 8081
**Purpose**: Twitch OAuth 2.0 authentication and JWT token management
**Status**: ✅ Complete (100%)

### Component Diagram

```mermaid
graph TB
    subgraph "External"
        TWITCH[Twitch OAuth API]
        CLIENT[HTTP Client]
    end

    subgraph "Adapters - API Layer"
        HANDLERS[auth/handlers.go]
        DTO[DTOs]
    end

    subgraph "Core - Domain"
        USER[domain/user.go<br/>User entity]
        TOKEN[domain/token.go<br/>Token value object]
    end

    subgraph "Core - Ports"
        SVC_PORT[ports/service.go<br/>AuthService interface]
        REPO_PORT[ports/repository.go<br/>UserRepository interface]
    end

    subgraph "Core - Services"
        AUTH_SVC[services/auth_service.go<br/>OAuth flow logic]
    end

    subgraph "Adapters - Repository"
        REPO_IMPL[repository/postgres_user_repository.go]
    end

    subgraph "Infrastructure"
        PG[(PostgreSQL<br/>users table)]
        REDIS[(Redis<br/>sessions)]
    end

    CLIENT --> HANDLERS
    HANDLERS --> SVC_PORT
    SVC_PORT --> AUTH_SVC
    AUTH_SVC --> USER
    AUTH_SVC --> TOKEN
    AUTH_SVC --> REPO_PORT
    REPO_PORT --> REPO_IMPL
    REPO_IMPL --> PG
    AUTH_SVC --> REDIS
    AUTH_SVC --> TWITCH
```

### Domain Models

```go
// core/domain/user.go
type User struct {
    ID              string    // UUID
    TwitchID        string    // Twitch user ID
    Username        string    // Lowercase username
    DisplayName     string    // Display name
    ProfileImageURL string    // Avatar URL
    Email           string    // User email (optional)
    AccessToken     string    // Encrypted OAuth token
    RefreshToken    string    // Encrypted refresh token
    TokenExpiresAt  time.Time // Token expiry
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// core/domain/token.go
type TokenPair struct {
    AccessToken  string
    RefreshToken string
    ExpiresIn    int64 // Seconds until expiry
}
```

### Port Interfaces

```go
// core/ports/service.go
type AuthService interface {
    // OAuth flow
    GetAuthURL() string
    ExchangeCodeForToken(ctx context.Context, code string) (*domain.User, *domain.TokenPair, error)
    RefreshUserToken(ctx context.Context, userID string) (*domain.TokenPair, error)

    // User management
    GetUserByID(ctx context.Context, userID string) (*domain.User, error)
    GetUserByTwitchID(ctx context.Context, twitchID string) (*domain.User, error)
    UpdateUser(ctx context.Context, user *domain.User) error

    // Session management
    ValidateToken(ctx context.Context, tokenString string) (*jwt.Claims, error)
    Logout(ctx context.Context, userID string) error
}

// core/ports/repository.go
type UserRepository interface {
    Create(ctx context.Context, user *domain.User) error
    GetByID(ctx context.Context, id string) (*domain.User, error)
    GetByTwitchID(ctx context.Context, twitchID string) (*domain.User, error)
    Update(ctx context.Context, user *domain.User) error
    Delete(ctx context.Context, id string) error
}
```

### API Endpoints

| Method | Path | Handler | Auth Required |
|--------|------|---------|---------------|
| GET | `/auth/login` | `HandleLogin` | No |
| GET | `/auth/callback` | `HandleCallback` | No |
| POST | `/auth/refresh` | `HandleRefresh` | Yes (refresh token) |
| GET | `/auth/me` | `HandleGetMe` | Yes (JWT) |
| POST | `/auth/logout` | `HandleLogout` | Yes (JWT) |

### Service Logic Flow

```mermaid
sequenceDiagram
    participant User
    participant Handler
    participant AuthService
    participant TwitchAPI
    participant Repository
    participant DB
    participant JWT

    User->>Handler: GET /auth/login
    Handler->>AuthService: GetAuthURL()
    AuthService-->>Handler: Twitch OAuth URL
    Handler-->>User: 302 Redirect to Twitch

    User->>Twitch: Login & authorize
    Twitch->>Handler: GET /auth/callback?code=ABC
    Handler->>AuthService: ExchangeCodeForToken(code)
    AuthService->>TwitchAPI: POST /oauth2/token
    TwitchAPI-->>AuthService: {access_token, refresh_token}
    AuthService->>TwitchAPI: GET /helix/users (with token)
    TwitchAPI-->>AuthService: User profile data
    AuthService->>Repository: GetByTwitchID(twitchID)
    alt User exists
        Repository-->>AuthService: Existing user
        AuthService->>Repository: Update(user with new tokens)
    else New user
        Repository-->>AuthService: nil
        AuthService->>Repository: Create(new user)
    end
    Repository->>DB: UPDATE/INSERT users
    AuthService->>JWT: Generate JWT token
    AuthService-->>Handler: User + TokenPair
    Handler-->>User: {jwt, refresh_token, user}
```

### Key Files

| File | Purpose | Lines of Code |
|------|---------|---------------|
| `cmd/auth-service/main.go` | Service entry point, dependency injection | ~150 |
| `core/domain/user.go` | User entity definition | ~50 |
| `core/ports/service.go` | Service interface contract | ~30 |
| `core/services/auth_service.go` | OAuth flow and business logic | ~400 |
| `adapters/api/handlers.go` | HTTP request/response handling | ~250 |
| `adapters/repository/postgres_user_repository.go` | PostgreSQL CRUD operations | ~200 |

### Configuration

```yaml
# Environment variables
TWITCH_CLIENT_ID: "your-client-id"
TWITCH_CLIENT_SECRET: "your-client-secret"
TWITCH_REDIRECT_URL: "http://localhost:8080/api/v1/auth/callback"
JWT_SECRET: "your-secret-key"
JWT_EXPIRY_HOURS: "24"

# Database connection
DATABASE_URL: "postgresql://user:pass@localhost:5432/allchat"

# Redis connection
REDIS_URL: "redis://localhost:6379/0"
```

---

## Overlay Manager

**Port**: 8082
**Purpose**: CRUD operations for overlays and multi-source chat configurations
**Status**: ✅ Complete (100%)

### Component Diagram

```mermaid
graph TB
    subgraph "Adapters - API"
        HANDLERS[handlers.go<br/>Overlay CRUD + Sources]
    end

    subgraph "Core - Domain"
        OVERLAY[domain/overlay.go]
        CONFIG[domain/config.go]
        SOURCE[domain/chat_source.go]
    end

    subgraph "Core - Ports"
        SVC[ports/service.go<br/>OverlayService]
        REPO[ports/repository.go<br/>OverlayRepository<br/>ChatSourceRepository]
    end

    subgraph "Core - Services"
        OVL_SVC[services/overlay_service.go<br/>Business logic]
    end

    subgraph "Adapters - Repository"
        OVL_REPO[repository/postgres_overlay_repository.go]
        SRC_REPO[repository/postgres_source_repository.go]
    end

    subgraph "Infrastructure"
        PG[(PostgreSQL<br/>overlays<br/>overlay_configs<br/>overlay_chat_sources)]
    end

    HANDLERS --> SVC
    SVC --> OVL_SVC
    OVL_SVC --> OVERLAY
    OVL_SVC --> CONFIG
    OVL_SVC --> SOURCE
    OVL_SVC --> REPO
    REPO --> OVL_REPO
    REPO --> SRC_REPO
    OVL_REPO --> PG
    SRC_REPO --> PG
```

### Domain Models

```go
// core/domain/overlay.go
type Overlay struct {
    ID          string    // UUID
    UserID      string    // Owner UUID (FK to users)
    Name        string    // Display name
    Description string    // Optional description
    IsActive    bool      // Enable/disable flag
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// core/domain/config.go
type OverlayConfig struct {
    ID              string                 // UUID
    OverlayID       string                 // FK to overlays
    DisplaySettings map[string]interface{} // JSONB: font, colors, animations
    FilterSettings  map[string]interface{} // JSONB: banned words, user filters
    Enable7TV       bool                   // Third-party emotes
    EnableBTTV      bool
    EnableFFZ       bool
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// core/domain/chat_source.go
type ChatSource struct {
    ID           string                 // UUID
    OverlayID    string                 // FK to overlays
    Platform     string                 // "twitch", "youtube", "kick", "tiktok"
    ChannelID    string                 // Platform-specific identifier
    ChannelName  string                 // Display name
    AuthRequired bool                   // Requires OAuth?
    Config       map[string]interface{} // Platform-specific settings (JSONB)
    IsActive     bool                   // Enable/disable this source
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### Port Interfaces

```go
// core/ports/service.go
type OverlayService interface {
    // Overlay CRUD
    CreateOverlay(ctx context.Context, userID string, name, description string) (*domain.Overlay, error)
    GetOverlay(ctx context.Context, overlayID string) (*domain.Overlay, error)
    ListOverlays(ctx context.Context, userID string) ([]*domain.Overlay, error)
    UpdateOverlay(ctx context.Context, overlay *domain.Overlay) error
    DeleteOverlay(ctx context.Context, overlayID string) error

    // Config management
    GetConfig(ctx context.Context, overlayID string) (*domain.OverlayConfig, error)
    UpdateConfig(ctx context.Context, config *domain.OverlayConfig) error

    // Chat source management
    AddChatSource(ctx context.Context, source *domain.ChatSource) error
    GetChatSources(ctx context.Context, overlayID string) ([]*domain.ChatSource, error)
    UpdateChatSource(ctx context.Context, source *domain.ChatSource) error
    RemoveChatSource(ctx context.Context, sourceID string) error
}
```

### API Endpoints

| Method | Path | Purpose | Auth |
|--------|------|---------|------|
| GET | `/overlays` | List user's overlays | JWT |
| POST | `/overlays` | Create new overlay | JWT |
| GET | `/overlays/:id` | Get overlay details | JWT |
| PUT | `/overlays/:id` | Update overlay metadata | JWT |
| DELETE | `/overlays/:id` | Delete overlay (cascade) | JWT |
| GET | `/overlays/:id/config` | Get display/filter config | JWT |
| PUT | `/overlays/:id/config` | Update configuration | JWT |
| GET | `/overlays/:id/sources` | List chat sources | JWT |
| POST | `/overlays/:id/sources` | Add chat source | JWT |
| PUT | `/overlays/:id/sources/:sid` | Update source config | JWT |
| DELETE | `/overlays/:id/sources/:sid` | Remove source | JWT |

### Multi-Source Configuration Flow

```mermaid
sequenceDiagram
    participant User
    participant Handler
    participant Service
    participant Repository
    participant DB

    User->>Handler: POST /overlays/:id/sources<br/>{platform: "twitch", channel: "shroud"}
    Handler->>Handler: Validate JWT, extract userID
    Handler->>Service: AddChatSource(source)
    Service->>Service: Validate platform is supported
    Service->>Service: Check for duplicate source
    Service->>Repository: GetChatSources(overlayID)
    Repository->>DB: SELECT * FROM overlay_chat_sources WHERE overlay_id = ?
    DB-->>Repository: Existing sources
    Repository-->>Service: []ChatSource
    Service->>Service: No duplicate found
    Service->>Repository: CreateChatSource(source)
    Repository->>DB: INSERT INTO overlay_chat_sources
    DB-->>Repository: Success
    Repository-->>Service: Created source
    Service-->>Handler: ChatSource object
    Handler-->>User: 201 Created {source}

    Note over User,DB: Source Controller will detect this new source<br/>and start the appropriate Platform Listener
```

### Key Files

| File | Purpose | LOC |
|------|---------|-----|
| `core/domain/overlay.go` | Overlay entity | ~40 |
| `core/domain/config.go` | Configuration entity | ~50 |
| `core/domain/chat_source.go` | Chat source entity | ~45 |
| `core/services/overlay_service.go` | Business logic | ~500 |
| `adapters/api/handlers.go` | HTTP handlers | ~400 |
| `adapters/repository/postgres_overlay_repository.go` | Overlay persistence | ~250 |
| `adapters/repository/postgres_source_repository.go` | Source persistence | ~200 |

---

## Emote Service

**Port**: 8083
**Purpose**: Fetch and cache third-party emotes (7TV, BTTV, FFZ)
**Status**: ✅ Complete (100%)

### Component Diagram

```mermaid
graph TB
    subgraph "Adapters - API"
        HANDLERS[handlers.go<br/>Emote endpoints]
    end

    subgraph "Core - Domain"
        EMOTE[domain/emote.go<br/>Emote entity]
    end

    subgraph "Core - Ports"
        SVC[ports/service.go<br/>EmoteService]
        CLIENT[ports/client.go<br/>EmoteProviderClient]
    end

    subgraph "Core - Services"
        EMOTE_SVC[services/emote_service.go<br/>Caching + aggregation]
    end

    subgraph "Adapters - Clients"
        SEVTV[clients/seventv_client.go]
        BTTV[clients/bttv_client.go]
        FFZ[clients/ffz_client.go]
    end

    subgraph "External APIs"
        API_7TV[7TV API]
        API_BTTV[BTTV API]
        API_FFZ[FFZ API]
    end

    subgraph "Infrastructure"
        REDIS[(Redis<br/>Cached emotes<br/>TTL: 1 hour)]
    end

    HANDLERS --> SVC
    SVC --> EMOTE_SVC
    EMOTE_SVC --> EMOTE
    EMOTE_SVC --> CLIENT
    EMOTE_SVC --> REDIS
    CLIENT --> SEVTV
    CLIENT --> BTTV
    CLIENT --> FFZ
    SEVTV --> API_7TV
    BTTV --> API_BTTV
    FFZ --> API_FFZ
```

### Domain Models

```go
// core/domain/emote.go
type Emote struct {
    Code     string   // Emote code (e.g., "Kappa", "KEKW")
    Provider string   // "7tv", "bttv", "ffz", "twitch"
    URL      string   // Image URL (1x size)
    URLs     EmoteURL // Multiple sizes
}

type EmoteURL struct {
    Size1x string // 28x28
    Size2x string // 56x56
    Size4x string // 112x112
}

type EmoteSet struct {
    Channel  string   // Channel name
    Emotes   []Emote  // List of emotes
    CachedAt time.Time
}
```

### Port Interfaces

```go
// core/ports/service.go
type EmoteService interface {
    // Get all emotes for a channel (7TV + BTTV + FFZ)
    GetChannelEmotes(ctx context.Context, channel string) (*domain.EmoteSet, error)

    // Get emotes from specific provider
    Get7TVEmotes(ctx context.Context, channel string) ([]domain.Emote, error)
    GetBTTVEmotes(ctx context.Context, channel string) ([]domain.Emote, error)
    GetFFZEmotes(ctx context.Context, channel string) ([]domain.Emote, error)

    // Cache management
    InvalidateCache(ctx context.Context, channel string) error
}

// core/ports/client.go
type EmoteProviderClient interface {
    FetchEmotes(ctx context.Context, channel string) ([]domain.Emote, error)
}
```

### API Endpoints

| Method | Path | Purpose | Cache TTL |
|--------|------|---------|-----------|
| GET | `/emotes/channel/:channel` | All emotes (7TV+BTTV+FFZ) | 1 hour |
| GET | `/emotes/7tv/:channel` | 7TV emotes only | 1 hour |
| GET | `/emotes/bttv/:channel` | BTTV emotes only | 1 hour |
| GET | `/emotes/ffz/:channel` | FFZ emotes only | 1 hour |
| DELETE | `/emotes/channel/:channel` | Invalidate cache | N/A |

### Caching Strategy

```mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant Service
    participant Redis
    participant ProviderAPI as External Provider API

    Client->>Handler: GET /emotes/channel/shroud
    Handler->>Service: GetChannelEmotes("shroud")
    Service->>Redis: GET emotes:all:shroud
    alt Cache hit
        Redis-->>Service: Cached emote set
        Service-->>Handler: EmoteSet (from cache)
    else Cache miss
        Redis-->>Service: nil
        par Fetch 7TV
            Service->>ProviderAPI: GET 7tv.io/v3/users/twitch/shroud
            ProviderAPI-->>Service: 7TV emotes
        and Fetch BTTV
            Service->>ProviderAPI: GET api.betterttv.net/3/cached/users/twitch/shroud
            ProviderAPI-->>Service: BTTV emotes
        and Fetch FFZ
            Service->>ProviderAPI: GET api.frankerfacez.com/v1/room/shroud
            ProviderAPI-->>Service: FFZ emotes
        end
        Service->>Service: Aggregate emotes
        Service->>Redis: SETEX emotes:all:shroud 3600 {emotes}
        Service-->>Handler: EmoteSet (fresh)
    end
    Handler-->>Client: JSON emote list
```

### Key Files

| File | Purpose | LOC |
|------|---------|-----|
| `core/domain/emote.go` | Emote entity | ~40 |
| `core/services/emote_service.go` | Caching + aggregation | ~300 |
| `adapters/clients/seventv_client.go` | 7TV API integration | ~150 |
| `adapters/clients/bttv_client.go` | BTTV API integration | ~120 |
| `adapters/clients/ffz_client.go` | FFZ API integration | ~120 |
| `adapters/api/handlers.go` | HTTP handlers | ~200 |

---

## Source Controller

**Port**: 8084
**Purpose**: Control plane for managing platform listener lifecycle
**Status**: 🟡 In Progress (70%)

### Component Diagram

```mermaid
graph TB
    subgraph "Core - Domain"
        CMD[domain/control_command.go]
        STATE[domain/listener_state.go]
    end

    subgraph "Core - Services"
        CTRL[services/controller_service.go<br/>Polling + diff logic]
        LEADER[services/leader_election.go<br/>Redis-based leadership]
    end

    subgraph "Adapters - Repository"
        REPO[repository/postgres_source_repository.go<br/>Query active sources]
    end

    subgraph "Infrastructure"
        PG[(PostgreSQL<br/>overlay_chat_sources<br/>overlays)]
        REDIS[(Redis<br/>Streams: control-commands<br/>Keys: leader-lock)]
    end

    LEADER --> REDIS
    LEADER --> CTRL
    CTRL --> CMD
    CTRL --> STATE
    CTRL --> REPO
    REPO --> PG
    CTRL --> REDIS

    style LEADER fill:#ffe1e1
```

### Domain Models

```go
// core/domain/control_command.go
type ControlCommand struct {
    ID        string                 // Unique command ID
    Action    string                 // "start", "stop", "status"
    Platform  string                 // "twitch", "youtube", "kick", "tiktok"
    ChannelID string                 // Platform-specific channel ID
    OverlayID string                 // Associated overlay
    Config    map[string]interface{} // Platform-specific configuration
    Timestamp time.Time
}

// core/domain/listener_state.go
type ListenerState struct {
    Platform   string    // "twitch", "youtube"
    ChannelID  string    // Channel identifier
    Status     string    // "connecting", "connected", "disconnected", "error"
    LastSeen   time.Time // Last heartbeat
    ErrorMsg   string    // If status == "error"
}
```

### Control Loop Logic

```mermaid
sequenceDiagram
    participant CTRL as Source Controller<br/>(Leader)
    participant DB as PostgreSQL
    participant Redis as Redis Streams
    participant Listener as Platform Listener

    loop Every 10 seconds
        CTRL->>DB: SELECT active sources<br/>FROM overlay_chat_sources<br/>JOIN overlays WHERE is_active = true
        DB-->>CTRL: List of active sources
        CTRL->>CTRL: Compare with last known state
        alt New source detected
            CTRL->>Redis: XADD stream:control-commands<br/>{action: "start", platform: "twitch", channel: "shroud"}
            Redis->>Listener: XREADGROUP consumer-group
            Listener->>Listener: Connect to Twitch IRC
            Listener->>Listener: JOIN #shroud
            Listener->>Redis: XADD stream:control-commands<br/>{action: "status", status: "connected"}
        else Source removed
            CTRL->>Redis: XADD stream:control-commands<br/>{action: "stop", platform: "twitch", channel: "shroud"}
            Redis->>Listener: XREADGROUP consumer-group
            Listener->>Listener: PART #shroud
            Listener->>Listener: Disconnect if no more channels
            Listener->>Redis: XADD stream:control-commands<br/>{action: "status", status: "disconnected"}
        end
    end
```

### Leader Election

```go
// core/services/leader_election.go
type LeaderElection interface {
    // Try to acquire leadership
    TryAcquire(ctx context.Context) (bool, error)

    // Renew leadership (extend TTL)
    Renew(ctx context.Context) error

    // Release leadership
    Release(ctx context.Context) error

    // Check if this instance is leader
    IsLeader(ctx context.Context) bool
}

// Implementation uses Redis SET NX with TTL
// Key: "leader:source-controller"
// Value: instance-id
// TTL: 30 seconds (renewed every 10 seconds)
```

### Key Files

| File | Purpose | LOC |
|------|---------|-----|
| `core/domain/control_command.go` | Command entity | ~40 |
| `core/services/controller_service.go` | Control loop logic | ~350 |
| `core/services/leader_election.go` | Redis-based leader election | ~150 |
| `adapters/repository/postgres_source_repository.go` | Query active sources | ~100 |
| `cmd/source-controller/main.go` | Entry point + election setup | ~200 |

---

## Platform Listeners

**Ports**: 8085 (Twitch), 8086 (YouTube), 8087+ (future)
**Purpose**: Connect to streaming platforms and capture raw chat messages
**Status**: 🟡 In Progress (Twitch 75%, YouTube 60%)

### General Architecture (All Listeners)

```mermaid
graph TB
    subgraph "Control Plane"
        CTRL_CONSUMER[Control Command Consumer<br/>XREADGROUP control-commands]
    end

    subgraph "Connection Manager"
        CONN[Connection Pool<br/>Manages active connections]
    end

    subgraph "Message Handler"
        PARSER[Platform Message Parser]
        PRODUCER[Raw Message Producer<br/>XADD stream:raw-messages]
    end

    subgraph "External Platform"
        PLATFORM[Twitch IRC / YouTube API / etc]
    end

    subgraph "Infrastructure"
        REDIS[(Redis Streams)]
    end

    CTRL_CONSUMER --> REDIS
    REDIS --> CONN
    CONN --> PLATFORM
    PLATFORM --> PARSER
    PARSER --> PRODUCER
    PRODUCER --> REDIS
```

---

### Twitch Listener

**Port**: 8085
**Connection**: IRC (gempir/go-twitch-irc/v4)
**Status**: 🟡 75% Complete

#### Architecture

```mermaid
graph TB
    subgraph "Twitch IRC Connection"
        IRC[IRC Client<br/>go-twitch-irc]
        CHANNELS[Channel Manager<br/>JOIN/PART commands]
    end

    subgraph "Message Processing"
        PARSER[IRC Message Parser<br/>Parse PRIVMSG]
        NORMALIZER[Message Normalizer<br/>Extract user, badges, emotes]
    end

    subgraph "Output"
        PRODUCER[Redis Producer<br/>XADD stream:raw-messages]
    end

    subgraph "Control"
        CTRL[Control Consumer<br/>Listen for start/stop]
    end

    CTRL --> CHANNELS
    CHANNELS --> IRC
    IRC --> PARSER
    PARSER --> NORMALIZER
    NORMALIZER --> PRODUCER
```

#### Message Flow

```go
// Twitch IRC message
:user!user@user.tmi.twitch.tv PRIVMSG #shroud :Hello world! Kappa

// Parsed to raw message
{
  "platform": "twitch",
  "channel_id": "shroud",
  "user": {
    "id": "12345",
    "username": "user",
    "display_name": "User",
    "badges": ["subscriber"],
    "color": "#FF0000"
  },
  "text": "Hello world! Kappa",
  "raw_emotes": "25:13-17", // Twitch emote positions
  "timestamp": "2025-11-11T12:34:56Z"
}
```

#### Rate Limits

- **JOIN**: 20 channels per 10 seconds
- **PRIVMSG**: 100 messages per 30 seconds (authenticated)
- **Reconnection**: Exponential backoff (1s, 2s, 4s, 8s, max 60s)

#### Key Files

| File | Purpose | LOC |
|------|---------|-----|
| `cmd/twitch-listener/main.go` | Entry point | ~100 |
| `internal/twitch-listener/adapters/irc/client.go` | IRC connection wrapper | ~250 |
| `internal/twitch-listener/core/services/message_handler.go` | Parse and normalize | ~200 |
| `internal/twitch-listener/core/services/channel_manager.go` | JOIN/PART logic | ~150 |

---

### YouTube Listener

**Port**: 8086
**Connection**: YouTube Live Chat API v3 (REST polling)
**Status**: 🟡 60% Complete

#### Architecture

```mermaid
graph TB
    subgraph "YouTube API Client"
        AUTH[OAuth Client<br/>Per-stream OAuth]
        POLLER[Live Chat Poller<br/>Poll every 5-10s]
        LEADER[Leader Election<br/>One poller per stream]
    end

    subgraph "Message Processing"
        PARSER[API Response Parser]
        NORMALIZER[Message Normalizer]
    end

    subgraph "Output"
        PRODUCER[Redis Producer<br/>XADD stream:raw-messages]
    end

    LEADER --> AUTH
    AUTH --> POLLER
    POLLER --> PARSER
    PARSER --> NORMALIZER
    NORMALIZER --> PRODUCER
```

#### API Polling Strategy

```mermaid
sequenceDiagram
    participant YT as YouTube Listener<br/>(Leader for stream X)
    participant API as YouTube Live Chat API
    participant Redis

    loop Every 5-10 seconds
        YT->>API: GET liveChatMessages<br/>?liveChatId=X&pageToken=Y
        API-->>YT: {messages: [...], nextPageToken: Z}
        YT->>YT: Parse messages
        loop For each message
            YT->>Redis: XADD stream:raw-messages {raw message}
        end
        YT->>YT: Update pageToken = Z
    end
```

#### Rate Limits & Quotas

- **Quota**: 10,000 units per day per project
- **liveChatMessages.list**: 5 units per request
- **Polling interval**: Dynamic based on message rate (5-10 seconds)
- **Leadership**: One poller per stream (avoid quota waste)

#### Key Files

| File | Purpose | LOC |
|------|---------|-----|
| `cmd/youtube-listener/main.go` | Entry point + leader election | ~150 |
| `internal/youtube-listener/adapters/api/client.go` | YouTube API wrapper | ~300 |
| `internal/youtube-listener/core/services/poller_service.go` | Polling loop | ~250 |
| `internal/youtube-listener/core/services/message_handler.go` | Parse and normalize | ~200 |

---

## Message Processor

**Port**: 8087
**Purpose**: Normalize, enrich, and route messages to overlays
**Status**: 🟡 50% Complete

### Component Diagram

```mermaid
graph TB
    subgraph "Input"
        CONSUMER[Redis Stream Consumer<br/>XREADGROUP stream:raw-messages]
    end

    subgraph "Processing Pipeline"
        NORMALIZE[Normalizer<br/>Platform → Unified format]
        ENRICH[Enricher<br/>Fetch emotes from Emote Service]
        ROUTE[Router<br/>Determine target overlays]
    end

    subgraph "Output"
        PUBLISHER[Redis Pub/Sub<br/>PUBLISH overlay:{id}]
    end

    subgraph "External Services"
        EMOTE_SVC[Emote Service<br/>GET /emotes/channel/:channel]
    end

    subgraph "Infrastructure"
        REDIS[(Redis)]
    end

    REDIS --> CONSUMER
    CONSUMER --> NORMALIZE
    NORMALIZE --> ENRICH
    ENRICH --> EMOTE_SVC
    ENRICH --> ROUTE
    ROUTE --> PUBLISHER
    PUBLISHER --> REDIS
```

### Processing Pipeline

```mermaid
sequenceDiagram
    participant Stream as Redis Stream<br/>stream:raw-messages
    participant Processor as Message Processor
    participant Emote as Emote Service
    participant PubSub as Redis Pub/Sub

    Stream->>Processor: Raw message (Twitch format)
    Processor->>Processor: Step 1: Normalize to unified format
    Note over Processor: Extract user, badges, text<br/>Map platform-specific fields
    Processor->>Emote: GET /emotes/channel/shroud
    Emote-->>Processor: {7TV, BTTV, FFZ emotes}
    Processor->>Processor: Step 2: Parse text for emotes<br/>Match emote codes<br/>Build emote positions
    Processor->>Processor: Step 3: Route to overlays<br/>Query: which overlays have this source?
    loop For each target overlay
        Processor->>PubSub: PUBLISH overlay:{overlay_id} {enriched message}
    end
```

### Domain Models

```go
// Unified message format (output)
type UnifiedMessage struct {
    ID          string              `json:"id"`          // UUID
    OverlayID   string              `json:"overlay_id"`  // Target overlay
    Platform    string              `json:"platform"`    // "twitch", "youtube"
    ChannelID   string              `json:"channel_id"`
    ChannelName string              `json:"channel_name"`
    User        UnifiedUser         `json:"user"`
    Message     UnifiedMessageBody  `json:"message"`
    Timestamp   time.Time           `json:"timestamp"`
    Metadata    MessageMetadata     `json:"metadata"`
}

type UnifiedUser struct {
    ID          string   `json:"id"`
    Username    string   `json:"username"`
    DisplayName string   `json:"display_name"`
    AvatarURL   string   `json:"avatar_url"`
    Badges      []string `json:"badges"`
    Color       string   `json:"color"` // Hex color
}

type UnifiedMessageBody struct {
    Text   string  `json:"text"`
    Emotes []Emote `json:"emotes"`
}

type Emote struct {
    Code      string     `json:"code"`      // "Kappa"
    Provider  string     `json:"provider"`  // "twitch", "7tv", "bttv", "ffz"
    URL       string     `json:"url"`
    Positions [][]int    `json:"positions"` // [[start, end]]
}

type MessageMetadata struct {
    IsSubscriber    bool   `json:"is_subscriber"`
    IsModerator     bool   `json:"is_moderator"`
    IsVIP           bool   `json:"is_vip"`
    Bits            int    `json:"bits"`
    SuperChatAmount int    `json:"super_chat_amount"`
    MembershipMonths int   `json:"membership_months"`
}
```

### Key Files

| File | Purpose | LOC |
|------|---------|-----|
| `core/domain/unified_message.go` | Unified message format | ~80 |
| `core/services/normalizer_service.go` | Platform → Unified | ~400 |
| `core/services/enricher_service.go` | Emote enrichment | ~250 |
| `core/services/router_service.go` | Overlay routing | ~150 |
| `cmd/message-processor/main.go` | Pipeline orchestration | ~200 |

---

## API Gateway

**Port**: 8080
**Purpose**: HTTP reverse proxy + WebSocket hub for overlays
**Status**: 🟡 60% Complete (HTTP ✅, WebSocket ⏳)

### Component Diagram

```mermaid
graph TB
    subgraph "External Clients"
        WEB[Web Frontend]
        OVERLAY[Overlay WebSocket]
    end

    subgraph "API Gateway - HTTP Layer"
        PROXY[Reverse Proxy<br/>Gin router]
        MIDDLEWARE[Middleware<br/>CORS, Auth, Logging]
    end

    subgraph "API Gateway - WebSocket Layer"
        WS_MGR[WebSocket Manager]
        SUB_MGR[Subscription Manager<br/>Redis Pub/Sub]
        CONN_POOL[Connection Pool<br/>overlay_id → []WebSocket]
    end

    subgraph "Backend Services"
        AUTH[Auth Service :8081]
        OVM[Overlay Manager :8082]
        EMOTE[Emote Service :8083]
    end

    subgraph "Infrastructure"
        REDIS[(Redis Pub/Sub<br/>overlay:{id})]
    end

    WEB --> MIDDLEWARE
    MIDDLEWARE --> PROXY
    PROXY --> AUTH
    PROXY --> OVM
    PROXY --> EMOTE

    OVERLAY --> WS_MGR
    WS_MGR --> CONN_POOL
    WS_MGR --> SUB_MGR
    SUB_MGR --> REDIS
```

### HTTP Routing

| Path Pattern | Upstream Service | Port |
|--------------|------------------|------|
| `/api/v1/auth/*` | Auth Service | 8081 |
| `/api/v1/overlays/*` | Overlay Manager | 8082 |
| `/api/v1/emotes/*` | Emote Service | 8083 |
| `/ws/overlay/:id` | WebSocket Manager | N/A (in-process) |

### WebSocket Flow

```mermaid
sequenceDiagram
    participant Overlay
    participant Gateway as API Gateway<br/>WebSocket Manager
    participant Redis as Redis Pub/Sub
    participant Processor as Message Processor

    Overlay->>Gateway: WS Connect /ws/overlay/:id?token=JWT
    Gateway->>Gateway: Validate JWT token
    Gateway->>Gateway: Add connection to pool[overlay_id]
    Gateway->>Redis: SUBSCRIBE overlay:uuid-123
    Note over Gateway,Redis: Gateway subscribes to this overlay's channel

    Processor->>Redis: PUBLISH overlay:uuid-123 {enriched message}
    Redis->>Gateway: Message received
    Gateway->>Gateway: Lookup connections in pool[uuid-123]
    loop For each WebSocket connection
        Gateway->>Overlay: WebSocket push {enriched message}
    end
    Overlay->>Overlay: Render message in overlay UI
```

### Key Files

| File | Purpose | LOC |
|------|---------|-----|
| `cmd/api-gateway/main.go` | Entry point + routing | ~200 |
| `internal/api-gateway/adapters/proxy/reverse_proxy.go` | HTTP proxy | ~150 |
| `internal/api-gateway/adapters/websocket/manager.go` | WebSocket lifecycle | ~300 |
| `internal/api-gateway/core/services/subscription_service.go` | Redis pub/sub | ~250 |
| `pkg/middleware/auth.go` | JWT validation middleware | ~100 |
| `pkg/middleware/cors.go` | CORS middleware | ~50 |

---

## Shared Packages

### `pkg/` Directory Structure

```
pkg/
├── auth/              # JWT utilities
│   ├── jwt.go        # Generate, validate, parse JWT
│   └── claims.go     # Custom JWT claims
├── database/          # PostgreSQL connection
│   ├── postgres.go   # Connection pooling (pgx)
│   └── health.go     # Health check
├── redis/             # Redis client
│   ├── client.go     # Connection wrapper
│   └── streams.go    # Streams helper functions
├── logger/            # Structured logging
│   └── zap.go        # Zap logger setup
├── middleware/        # HTTP middleware
│   ├── auth.go       # JWT validation
│   ├── cors.go       # CORS headers
│   └── logging.go    # Request logging
└── streams/           # Redis Streams utilities
    ├── producer.go   # XADD helper
    └── consumer.go   # XREADGROUP helper
```

### Usage Example

```go
// Service initialization
logger := logger.NewLogger("auth-service", "info")
db := database.NewPostgresConnection(os.Getenv("DATABASE_URL"))
redisClient := redis.NewClient(os.Getenv("REDIS_URL"))

// JWT middleware
router := gin.Default()
router.Use(middleware.CORS())
protected := router.Group("/")
protected.Use(middleware.JWTAuth(jwtSecret))
```

---

## Summary

This document provides the detailed component architecture for each service in the All-Chat platform. Key takeaways:

1. **Consistent Hexagonal Architecture**: All services follow the same pattern
2. **Clear Boundaries**: Core domain is isolated from infrastructure
3. **Interface-Driven Design**: Ports define contracts between layers
4. **Shared Utilities**: `pkg/` provides common functionality
5. **Scalable Components**: Each service can scale independently

**Next Steps**:
- [DATA_FLOW_INTEGRATION.md](./DATA_FLOW_INTEGRATION.md) - Message flows and Redis patterns
- [DEPLOYMENT_KUBERNETES.md](./DEPLOYMENT_KUBERNETES.md) - Kubernetes resource specifications
- [SCALING_PERFORMANCE.md](./SCALING_PERFORMANCE.md) - Scaling strategies per service

---

**Document Maintainers**: Development Team
**Last Review**: 2025-11-11
