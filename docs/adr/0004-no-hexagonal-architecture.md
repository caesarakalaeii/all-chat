# ADR-0004: No Hexagonal Architecture

**Date**: 2025-11-11
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: Architecture Team, Development Lead

## Context

The earliest architecture plan sketched hexagonal architecture (ports/adapters) for the services. It was never implemented: the services were built with standard Go layout, and the Phase 1 contradiction audit surfaced the mismatch between plan and reality.

Adopting hexagonal after the fact would have cost an interface plus adapter for every component across the services, plus the per-feature ceremony forever. Trials at decision time measured ~40% of agent-generated code against the hexagonal sketch with interface wiring bugs, versus ~90% accurate against the standard layout, and roughly 3x faster feature work (3h vs 9h for an OAuth flow). No implementation is ever swapped in practice: PostgreSQL is a locked choice (ADR-0003) and the Redis transport is core (ADR-0002).

## Decision

Do not adopt hexagonal architecture. No ports/adapters layer and no interface mirroring of concrete repositories. Dependency injection happens through constructors; tests pass mock structs. Where multiple implementations genuinely exist, an interface is used locally, without ceremony elsewhere.

Consequences (less "clean architecture"): handlers may import repository types directly and domain logic may depend on infrastructure types. Accepted; package boundaries (handlers/repository/`<domain-packages>/`) provide the separation. Swapping a backing store is harder in theory but never needed.

Alternatives rejected: full adoption (boilerplate and slow reviews with no practical benefit); hybrid with ports only for external dependencies (inconsistent per-service patterns, "does this need a port?" decisions, still too much ceremony).

See ADR-0001 for the chosen layout and its implementation details.
