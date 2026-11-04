# ADR-0001: Standard Go Layout (Not Hexagonal Architecture)

**Date**: 2025-11-11
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Architecture Team, LLM Development Lead

---

## Context and Problem Statement

All-Chat is a microservices platform built with an **LLM-first development approach**, meaning AI agents (Claude, GPT-4) generate the majority of code. We need a service structure that:
- LLMs can generate accurately (familiar patterns)
- Minimizes boilerplate and abstraction layers
- Supports testability through dependency injection
- Scales to a growing set of microservices

**Initial Plan**: Hexagonal Architecture (ports/adapters pattern) was proposed for "clean architecture" benefits.

**Problem**: Should we use Hexagonal Architecture (ports/adapters) or Standard Go project layout?

---

## Decision Drivers

1. **LLM Code Generation Accuracy**: LLMs must generate correct code with minimal manual fixes
2. **Development Velocity**: Faster to implement features with less ceremony
3. **Maintainability**: Code should be easy to understand for future developers
4. **Testing**: Must support unit testing with dependency injection
5. **Go Community Standards**: Align with common Go patterns and examples
6. **Boilerplate Overhead**: Minimize repetitive code across services

---

## Considered Options

### Option 1: Hexagonal Architecture (Ports & Adapters)

**Structure**:
```
services/auth-service/
├── cmd/main.go
├── domain/
│   ├── user.go           # Core business logic
│   └── ports/            # Interfaces
│       ├── repositories.go
│       └── services.go
├── adapters/
│   ├── inbound/
│   │   └── http/         # HTTP handlers (implement ports)
│   │       ├── auth_handler.go
│   │       └── auth_handler_test.go
│   └── outbound/
│       └── postgres/     # Database (implement ports)
│           ├── user_repo.go
│           └── user_repo_test.go
└── go.mod
```

**✅ Pros**:
- Clear separation of concerns (domain vs infrastructure)
- Easy to swap implementations (mock repositories for tests)
- Testable in isolation (domain logic has no external dependencies)
- "Clean architecture" compliance

**❌ Cons**:
- **High boilerplate**: Every service needs ports (interfaces) + adapters (implementations)
- **LLM confusion**: LLMs struggle with abstract interfaces, often generate incorrect wiring
- **Over-engineering**: Most services are simple CRUD, don't need this complexity
- **Non-standard in Go**: Few Go examples use hexagonal, LLMs trained on simpler patterns
- **Code duplication**: Interfaces mirror concrete types, adding ~60% more code

**Example Boilerplate** (for single user repository):
```go
// domain/ports/repositories.go (25 lines)
type UserRepository interface {
    Create(ctx context.Context, user *User) error
    GetByID(ctx context.Context, id string) (*User, error)
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id string) error
}

// adapters/outbound/postgres/user_repo.go (150 lines)
type PostgresUserRepository struct {
    db *pgxpool.Pool
}

func NewPostgresUserRepository(db *pgxpool.Pool) *PostgresUserRepository {
    return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) Create(ctx context.Context, user *User) error {
    // Implementation...
}
// ... +120 lines for other methods

// cmd/main.go (wiring) (50 lines)
userRepo := postgres.NewPostgresUserRepository(db)
userService := services.NewUserService(userRepo)
authHandler := http.NewAuthHandler(userService)
```

**Total**: ~225 lines for single repository with 4 methods.

---

### Option 2: Standard Go Project Layout

**Structure**:
```
services/auth-service/
├── cmd/main.go           # Entry point
├── handlers/
│   ├── auth.go           # HTTP handlers
│   └── auth_test.go
├── models/
│   ├── user.go           # Data models
│   └── user_test.go
├── repository/
│   ├── user_repo.go      # Database layer
│   └── user_repo_test.go
├── oauth/                # Domain-specific package
│   ├── twitch.go
│   └── twitch_test.go
└── go.mod
```

**✅ Pros**:
- **LLM-friendly**: Standard pattern LLMs are trained on (thousands of examples)
- **60% less code**: No interface duplication, direct implementation
- **Go community standard**: Matches golang-standards/project-layout
- **Clear separation**: Packages provide natural boundaries (handlers, repository, domain)
- **Testable**: Dependency injection through constructors (no interfaces needed)
- **Familiar**: New developers recognize structure immediately

**❌ Cons**:
- Less "pure" than hexagonal (handlers might know about database types)
- Cannot swap implementations as easily (but we never need to in practice)
- Domain logic may have direct dependencies on infrastructure (acceptable trade-off)

**Example Code** (same user repository):
```go
// repository/user_repo.go (100 lines)
package repository

type UserRepository struct {
    db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
    return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
    // Implementation...
}
// ... +70 lines for other methods

// cmd/main.go (wiring) (20 lines)
userRepo := repository.NewUserRepository(db)
authHandler := handlers.NewAuthHandler(userRepo, logger)
```

**Total**: ~120 lines (vs 225 with hexagonal) - **47% less code**.

---

### Option 3: Hybrid Approach (Interfaces Only Where Needed)

Define interfaces **only** where:
- Multiple implementations exist (e.g., Redis vs in-memory cache)
- Mocking is critical (complex external APIs)

**✅ Pros**:
- Best of both worlds (pragmatic abstraction)
- Reduced boilerplate compared to full hexagonal

**❌ Cons**:
- Inconsistent patterns across services (confuses LLMs)
- Developers must decide when to use interfaces (cognitive load)
- Half the boilerplate is still too much

---

## Decision Outcome

**Chosen**: **Option 2 - Standard Go Project Layout**

**Rationale**:

1. **LLM Code Generation** (Primary Driver):
   - LLMs trained on thousands of Standard Go Layout examples
   - Fewer interface wiring bugs than the hexagonal pattern produced

2. **Development Velocity**:
   - 60% less code to write/review/maintain
   - Features implemented faster (no interface ceremony)
   - Easier code reviews (less abstraction to understand)

3. **Go Community Alignment**:
   - Matches golang-standards/project-layout
   - Tutorials, blog posts, examples use this structure
   - New Go developers familiar with pattern

4. **Testability Not Compromised**:
   - Dependency injection through constructors still works
   - `repository` package can be mocked with structs (no interfaces needed)
   - Table-driven tests work identically

5. **Real-World Pragmatism**:
   - We never swap implementations in practice (PostgreSQL is fixed choice)
   - YAGNI (You Aren't Gonna Need It) applies to abstract ports

---

## Consequences

### Positive

1. **Faster Development** (~60% less code vs the hexagonal sketch):
   - Fewer interface/adapter layers to write and review per service

2. **Higher LLM Accuracy**:
   - Standard patterns dominate LLM training data; fewer manual fixes

3. **Easier Onboarding**:
   - New developers recognize standard patterns
   - Less time learning custom abstractions

4. **Cleaner Codebase**:
   - Fewer files, less directory nesting
   - Code intent clearer without indirection

### Negative

1. **Less "Clean Architecture"**:
   - Handlers may import repository types directly
   - Domain logic may depend on infrastructure (PostgreSQL types)
   - Acceptable trade-off for pragmatism

2. **Harder to Swap Implementations**:
   - Cannot easily replace PostgreSQL with MongoDB
   - Mitigation: We never need to (locked to PostgreSQL via ADR-0003)

3. **Potential Coupling**:
   - Without interfaces, services may become tightly coupled to dependencies
   - Mitigation: Clear package boundaries (handlers, repository, domain) provide separation

---

## Implementation

### Files Affected

All services follow Standard Go Layout (the complete `services/` directory):
- `services/alert-processor/`
- `services/altstream-listener/`
- `services/api-gateway/`
- `services/auth-service/`
- `services/discord-bot/`
- `services/discord-listener/`
- `services/emote-service/`
- `services/engagement-service/`
- `services/facebook-listener/`
- `services/goodgame-listener/`
- `services/instagram-listener/`
- `services/kick-listener/`
- `services/media-service/`
- `services/message-processor/`
- `services/moderation-service/`
- `services/overlay-manager/`
- `services/owncast-listener/`
- `services/payment-service/`
- `services/picarto-listener/`
- `services/share-service/`
- `services/source-manager/`
- `services/support-bot/`
- `services/tiktok-listener/`
- `services/tiktok-relay/`
- `services/tiktok-signer/`
- `services/token-refresh-service/`
- `services/twitch-eventsub-listener/`
- `services/twitch-listener/`
- `services/youtube-listener/`
- `services/youtube-listener-innertube/`
- `services/youtube-quota-monitor/`

### Migration

**From**: Hexagonal architecture (ports/adapters) was **never implemented**.
**To**: All services built with Standard Go Layout from day one.

### Service Template

```
services/<service-name>/
├── cmd/
│   └── main.go           # Entry point (logger, DB/Redis, HTTP server)
├── handlers/             # HTTP handlers (Gin)
│   ├── <feature>.go
│   └── <feature>_test.go
├── models/               # Data models
│   ├── <entity>.go
│   └── <entity>_test.go
├── repository/           # Database layer (optional, if service uses DB)
│   ├── <entity>_repo.go
│   └── <entity>_repo_test.go
├── <domain-packages>/    # Domain-specific logic
│   ├── oauth/            # Example: OAuth flows
│   ├── streams/          # Example: Stream management
│   └── channels/         # Example: Channel coordination
├── go.mod                # Dependencies
└── Dockerfile            # Container image
```

### Dependency Injection Pattern

```go
// cmd/main.go — illustrative wiring, following services/auth-service/cmd/main.go
func main() {
    log := logger.NewLogger("auth-service", getEnvOrDefault("LOG_LEVEL", "info"))
    db, err := database.NewPostgresPool(connString) // DATABASE_HOST/PORT/NAME/USER/PASSWORD

    // Inject dependencies via constructors
    userRepo := repository.NewUserRepository(db, tokenCipher)
    platformAuthHandler := handlers.NewPlatformAuthHandlerV2(providers, userRepo, redisClient, log)

    // Register routes
    router := gin.New()
    router.POST("/login", platformAuthHandler.HandleLogin(oauth.PlatformTwitch))

    // Start server in a goroutine and shut down on signal (25s grace deadline)
    srv := &http.Server{Addr: ":" + getEnvOrDefault("PORT", "8081"), Handler: router}
    ...
}
```

No interfaces, no adapters, direct dependency injection. Testable by passing mock implementations.

---

## Related Decisions

- **ADR-0004**: [No Hexagonal Architecture](./0004-no-hexagonal-architecture.md) - Detailed analysis of removing ports/adapters
- **Architecture**: [00-OVERVIEW.md](../architecture/00-OVERVIEW.md) - System architecture
- **Historical**: [KUBERNETES_CONTROLLER_ANALYSIS.md](../phase-reports/KUBERNETES_CONTROLLER_ANALYSIS.md) - Kubernetes operator analysis from the same review cycle (archived)

---

## Validation

### Code Generation Test (2025-11-12)

**Prompt**: "Create a new HTTP handler for listing overlays with pagination"

**Standard Go Layout**:
- ✅ Correct handler signature
- ✅ Proper error handling
- ✅ Database query logic accurate
- ✅ **Manual fixes**: 2 minor issues (imports, pagination offset)

**Hexagonal Architecture**:
- ❌ Incorrect interface wiring (forgot to inject port)
- ❌ Wrong adapter method signature (didn't match port)
- ❌ Missing domain service layer
- ❌ **Manual fixes**: 8 issues across 4 files

**Verdict**: Standard Go Layout generated significantly more accurate code.

---

## References

- **Standard Go Layout**: https://github.com/golang-standards/project-layout
- **Clean Architecture (Robert Martin)**: https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- **Hexagonal Architecture**: https://alistair.cockburn.us/hexagonal-architecture/
- **Pragmatic Go**: https://dave.cheney.net/practical-go/presentations/qcon-china.html

---

## Summary

**Decision**: Use Standard Go project layout for all microservices, reject hexagonal architecture.

**Reason**: ~60% less boilerplate than the hexagonal sketch, accurate LLM generation, Go community standard, faster development.

**Trade-off**: Less "clean architecture" purity, but pragmatic for LLM-first development approach.

**Status**: Implemented, all Go services use this layout, validated in production.
