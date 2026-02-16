# Coding Conventions

**Analysis Date:** 2026-02-16

## Naming Patterns

**Files:**
- Go files: lowercase with underscores (e.g., `overlay_manager.go`, `chat_source_test.go`)
- TypeScript/React files: PascalCase for components (e.g., `ErrorDisplay.tsx`, `ProtectedRoute.tsx`), camelCase for utilities (e.g., `errorParser.ts`, `badgeOrder.ts`)
- Test files: appended with `_test.go` (Go) or `.test.ts`/`.spec.ts` (TypeScript)

**Functions:**
- Go: PascalCase for exported functions, camelCase for unexported (e.g., `NewOverlayHandler()` exported, `setupTestRouter()` unexported)
- TypeScript: camelCase for all functions (e.g., `createOverlay()`, `parseApiError()`, `formatErrorMessage()`)
- Test functions: Go uses `TestTypeName_FunctionName()` pattern (e.g., `TestOverlay_Validate()`)

**Variables:**
- Go: camelCase (e.g., `userID`, `overlayID`, `dbPool`)
- TypeScript: camelCase (e.g., `loading`, `error`, `showBetaWarning`)
- Constants in Go: All caps with underscores (e.g., `MAX_NAME_LENGTH`, `MIN_DESCRIPTION_LENGTH`)

**Types:**
- Go: PascalCase structs and interfaces (e.g., `type Overlay struct`, `type OverlayRepository interface`)
- TypeScript: PascalCase for interfaces and types (e.g., `interface OverlayStore`, `type ChatError`)
- Optional fields in Go: use pointers for truly optional values (e.g., `TwitchID *string` in `User` struct)

## Code Style

**Formatting:**
- Go: Uses `gofmt` standard (enforced by language)
- TypeScript: ESLint configured via `.eslintrc.json` in `frontend/` directory
- ESLint extends: `next/core-web-vitals` for Next.js best practices

**Linting:**
- TypeScript: ESLint with Next.js configuration (`frontend/.eslintrc.json`)
- Go: Standard Go tooling (no custom linter config found, follows Go conventions)

## Import Organization

**Order (Go):**
1. Standard library imports (e.g., `"context"`, `"net/http"`, `"testing"`)
2. Third-party imports (e.g., `"github.com/gin-gonic/gin"`, `"go.uber.org/zap"`)
3. Internal imports from same package or shared packages (e.g., `"github.com/caesar/all-chat/services/overlay-manager/models"`)

Example from `handlers/overlay.go`:
```go
import (
	"context"
	"net/http"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
)
```

**Order (TypeScript):**
1. Third-party framework imports (e.g., `import { create } from 'zustand'`)
2. Type imports (e.g., `import type { Overlay } from '../types/overlay'`)
3. Local imports using path alias `@/` (e.g., `import { overlaysApi } from '@/lib/api/overlays'`)

**Path Aliases:**
- TypeScript: `@/*` maps to `./src/*` (configured in `tsconfig.json`)

## Error Handling

**Patterns (Go):**
- Explicit error returns as last return value: `func Create(ctx context.Context, overlay *models.Overlay) error`
- Check for nil before using error values: `if err != nil { ... }`
- Return early on error (error-first approach)
- Use structured errors with context when needed

Example from `handlers/overlay.go`:
```go
if err := overlay.Validate(); err != nil {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	return
}
```

**Patterns (TypeScript):**
- Use typed error classes extending base `ChatError` type
- Implement error parser functions for API responses: `parseApiError(response: Response, data?: any): ChatError`
- Error objects include `type`, `message`, `userMessage`, and `actionableSteps` fields
- Handle errors with try-catch in async operations: `try { ... } catch (error) { ... }`
- Error instanceof checks for type narrowing (e.g., `error instanceof Error`)

Example from `lib/errorParser.ts`:
```typescript
export function parseApiError(response: Response, data?: any): ChatError {
  const statusCode = response.status;
  if (statusCode === 401 || fullErrorText.includes('unauthorized')) {
    return createUnauthorizedError(errorMessage, platform, statusCode);
  }
  // ... more specific error type detection
}
```

## Logging

**Framework:** Go uses `go.uber.org/zap` for structured logging

**Patterns (Go):**
- Initialize logger once in `cmd/main.go`: `log := logger.NewLogger("service-name", logLevel)`
- Pass logger through dependency injection to handlers
- Use structured logging with zap fields: `log.Info("message", zap.String("key", value))`

Example from `cmd/main.go`:
```go
log.Info("Starting Overlay Manager Service",
	zap.String("version", getEnv("APP_VERSION", "0.1.0")),
)
```

**Patterns (TypeScript):**
- Use `console` for frontend logging (no custom logger found)
- Log error states in stores using `error` state field

## Comments

**When to Comment:**
- JSDoc comments on exported functions and types in TypeScript
- Brief block comments explaining non-obvious business logic
- Avoid commenting obvious code; focus on "why" rather than "what"

**JSDoc/TSDoc (TypeScript):**
- Use `/** ... */` format for exported functions
- Include parameter descriptions and return type documentation

Example from `lib/stores/overlay-store.ts`:
```typescript
/**
 * Overlay Store (Zustand)
 *
 * Global state management for user's overlays.
 * Handles CRUD operations and caching.
 *
 * Usage:
 *   const { overlays, fetchOverlays, createOverlay } = useOverlayStore();
 */
```

**Go Comments:**
- Exported functions should have comment starting with function name: `// NewOverlayHandler creates a new overlay handler`

## Function Design

**Size (Go):**
- Handler functions: 40-80 lines typical
- Keep validation, database calls, and response serialization in separate methods when possible
- Handlers follow consistent pattern: extract user, validate input, call domain logic, return response

**Parameters:**
- Go handlers receive `*gin.Context` as first parameter for HTTP context
- Go functions pass `context.Context` for cancellation and deadline support
- TypeScript functions avoid excessive parameters; use destructuring for options objects

**Return Values:**
- Go: Functions return business logic results plus explicit error
- TypeScript: Async functions return `Promise<Type>` with errors thrown as exceptions
- Go test functions use struct-based test tables (see Testing section below)

## Module Design

**Exports (Go):**
- Exported functions: Start with capital letter (e.g., `NewOverlayHandler()`, `Create()`)
- Unexported: lowercase (e.g., `setupTestRouter()`)
- Interfaces define contracts for testing and dependency injection

Example from `handlers/overlay.go`:
```go
// OverlayRepository defines the interface for overlay persistence
type OverlayRepository interface {
	Create(ctx context.Context, overlay *models.Overlay) error
	GetByID(ctx context.Context, id string) (*models.Overlay, error)
	// ...
}
```

**Exports (TypeScript):**
- Use `export` keyword for public APIs, default exports for single-export modules
- Store modules export Zustand hooks and types

**Barrel Files:**
- Not extensively used; imports are specific to modules
- No `index.ts` barrels observed in core source directories

---

*Convention analysis: 2026-02-16*
