# ADR-0001: Standard Go Layout (Not Hexagonal Architecture)

**Date**: 2025-11-11
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: Architecture Team, LLM Development Lead

## Context

All-Chat is built with an LLM-first development approach: agents generate most of the code. We needed a service structure that agents generate accurately, that minimizes abstraction layers, supports testability, and scales across a growing set of microservices. Hexagonal architecture (ports/adapters) was proposed for its clean-architecture benefits, but every service is mostly simple CRUD and the pattern costs substantial boilerplate (an interface mirroring every repository, adapter wiring in main).

## Decision

Use the standard Go project layout for every service, without ports/adapters abstractions:

```
services/<service-name>/
├── cmd/main.go           # Entry point (logger, DB/Redis, HTTP server)
├── handlers/             # HTTP handlers (Gin)
├── models/               # Data models
├── repository/           # Database layer (optional)
├── <domain-packages>/    # Domain-specific logic (oauth/, streams/, channels/, ...)
├── go.mod                # Dependencies
└── Dockerfile            # Container image
```

Rationale: the pattern dominates both training data and Go community examples, so agents generate it accurately with few manual fixes; it removes the interface duplication of hexagonal (~60% more code in the option sketches); and dependency injection through constructors keeps code testable without interfaces. In practice PostgreSQL is a fixed choice and implementations are never swapped, so swappability would be unused abstraction.

Consequences:

- Handlers may import repository types directly and domain logic may depend on infrastructure types. Accepted trade-off; package boundaries provide separation.
- Swapping a backing store (e.g. PostgreSQL for something else) is not cheap. Accepted; PostgreSQL is locked in via ADR-0003.

Alternatives rejected:

- Hexagonal architecture: heavy boilerplate, confusing interface wiring for agents, non-standard in Go.
- Hybrid (interfaces only where needed): inconsistent across services, still half the boilerplate; where multiple implementations really exist, interfaces are used locally.

## Implementation

All services under `services/` follow this layout and have since the first service, it is the template in CLAUDE.md, and an example of the constructor-based wiring is `services/auth-service/cmd/main.go`. A summary of the structure lives in [docs/architecture/00-OVERVIEW.md](../architecture/00-OVERVIEW.md).

References:

- https://github.com/golang-standards/project-layout
- https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- ADR-0004: No Hexagonal Architecture (analysis of this choice from the same review)
