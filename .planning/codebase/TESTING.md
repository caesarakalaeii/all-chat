# Testing Patterns

**Analysis Date:** 2026-02-16

## Test Framework

**Go:**
- Runner: `testing` (Go standard library)
- Assertion Library: `github.com/stretchr/testify/assert` (used in handler tests)
- Run Tests: `go test ./...`

**TypeScript/Frontend:**
- Unit/Component Tests: Vitest `^4.0.18`
- E2E Tests: Playwright `^1.58.2`
- Config: `vitest` (package.json), `playwright.config.ts` (E2E config)

**Run Commands:**
```bash
# Go tests
go test ./...                    # Run all Go tests
go test -v ./...                 # Verbose output
go test -cover ./...             # With coverage

# TypeScript tests
npm run test                      # Run vitest once
npm run test:watch               # Watch mode
npm run test:e2e                 # Run Playwright E2E tests
npm run test:e2e:ui              # Playwright UI mode
npm run test:e2e:debug           # Debug mode with inspector
npm run test:e2e:report          # View HTML report
```

## Test File Organization

**Location (Go):**
- Co-located with implementation: `overlay.go` and `overlay_test.go` in same directory
- Tests in `handlers/`, `models/`, `repository/`, and domain packages

**Naming (Go):**
- Pattern: `{implementation}_test.go`
- Test functions: `TestTypeName_FunctionName(t *testing.T)`
- Examples: `overlay_test.go`, `chat_source_test.go`, `health_test.go`

**Location (TypeScript):**
- E2E tests: `frontend/tests/e2e/` directory
- Unit tests: Not found in current codebase (only E2E tests observed)
- Examples: `landing.spec.ts`, `dashboard.spec.ts`, `overlay-editor.spec.ts`

**Structure (Go directories):**
```
services/overlay-manager/
├── models/
│   ├── overlay.go
│   ├── overlay_test.go
│   ├── chat_source.go
│   └── chat_source_test.go
├── handlers/
│   ├── overlay.go
│   ├── overlay_test.go
│   ├── health.go
│   └── health_test.go
└── repository/
    ├── overlay_repo.go
    └── overlay_repo_test.go
```

## Test Structure

**Suite Organization (Go):**

Use table-driven test pattern with struct slices:

```go
func TestOverlay_Validate(t *testing.T) {
	tests := []struct {
		name    string
		overlay *Overlay
		wantErr bool
	}{
		{
			name: "valid overlay",
			overlay: &Overlay{
				ID:          uuid.New().String(),
				UserID:      uuid.New().String(),
				Name:        "My Overlay",
				IsActive:    true,
			},
			wantErr: false,
		},
		{
			name: "missing user_id",
			overlay: &Overlay{
				ID:       uuid.New().String(),
				UserID:   "",
				Name:     "My Overlay",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.overlay.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Overlay.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
```

**Patterns (Go):**
- **Setup:** Helper function `setupTestRouter()` creates `*gin.Engine` in test mode (`gin.SetMode(gin.TestMode)`)
- **Teardown:** No explicit teardown observed; Go tests clean up automatically
- **Assertions:** Use `*testing.T` methods (`t.Errorf()`, `t.Fatal()`) and testify assertions (`assert.Equal()`)

## Mocking

**Framework (Go):** Hand-written mock implementations using interface-based design

**Patterns (Go):**

Define interface in handler package:
```go
type OverlayRepository interface {
	Create(ctx context.Context, overlay *models.Overlay) error
	GetByID(ctx context.Context, id string) (*models.Overlay, error)
	GetByIDAndUserID(ctx context.Context, id, userID string) (*models.Overlay, error)
	ListByUserID(ctx context.Context, userID string) ([]*models.Overlay, error)
	Update(ctx context.Context, overlay *models.Overlay) error
	Delete(ctx context.Context, id string) error
}
```

Create mock implementation in test file:
```go
type mockOverlayRepository struct {
	createFunc           func(context.Context, *models.Overlay) error
	getByIDFunc          func(context.Context, string) (*models.Overlay, error)
	getByIDAndUserIDFunc func(context.Context, string, string) (*models.Overlay, error)
	listByUserIDFunc     func(context.Context, string) ([]*models.Overlay, error)
	updateFunc           func(context.Context, *models.Overlay) error
	deleteFunc           func(context.Context, string) error
}

func (m *mockOverlayRepository) Create(ctx context.Context, overlay *models.Overlay) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, overlay)
	}
	return nil
}

// ... implement remaining interface methods
```

Use in test:
```go
func TestOverlayHandler_HandleCreateOverlay(t *testing.T) {
	repo := &mockOverlayRepository{
		createFunc: func(ctx context.Context, overlay *models.Overlay) error {
			return nil // Test success case
		},
	}
	handler := NewOverlayHandler(repo)
	// ... test handler
}
```

**What to Mock:**
- Repository/database layer (always mock for unit tests)
- External API clients
- Time-based dependencies for predictable testing

**What NOT to Mock:**
- Model validation logic (business rules should be tested directly)
- HTTP request/response handling (use httptest recorder)
- Core domain logic unless testing integration

## Fixtures and Factories

**Test Data (Go):**

Use UUID generation for unique test data:
```go
overlay := &Overlay{
	ID:       uuid.New().String(),
	UserID:   uuid.New().String(),
	Name:     "My Overlay",
	IsActive: true,
	CreatedAt: time.Now(),
	UpdatedAt: time.Now(),
}
```

No separate factory functions observed; inline struct construction is preferred in this codebase.

**Location:**
- Test data defined inline within test functions in same `_test.go` files
- Example: `services/overlay-manager/models/overlay_test.go`

## Coverage

**Requirements:** Not enforced (no `go test -coverprofile` or CI coverage gates found)

**View Coverage (Go):**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # Opens HTML report in browser
```

## Test Types

**Unit Tests (Go):**
- **Scope:** Individual functions and methods
- **Approach:** Table-driven tests with mock dependencies
- **Count:** ~50 test functions across services
- **Examples:** Model validation (`TestOverlay_Validate`), OAuth flows (`TestTwitchOAuth_GetAuthURL`)

Co-located in same package as implementation (`overlay_test.go` in same directory as `overlay.go`).

**Integration Tests (Go):**
- **Scope:** Multi-component flows (e.g., normalizer + enricher pipeline in message-processor)
- **Approach:** Process end-to-end message flow with real domain logic
- **Location:** `services/message-processor/integration/event_flow_test.go`
- **Note:** Limited integration test coverage; most focus is on unit tests with mocks

**E2E Tests (Playwright):**
- **Scope:** Complete user flows in browser
- **Location:** `frontend/tests/e2e/`
- **Approach:** Page object interactions, assertions on visible elements
- **Configuration:**
  - Base URL: `http://localhost:3000` (configurable via env var)
  - Browsers: Chromium, Firefox, WebKit, Mobile Chrome, Mobile Safari
  - Parallel: Enabled locally (workers per env), disabled on CI
  - Retries: 2 retries on CI only
  - Screenshots/Video: On failure only
  - Trace: On first retry

**Examples:**
```typescript
test('should load the landing page', async ({ page }) => {
  await page.goto('/');
  await expect(page.locator('h1')).toHaveText('All-Chat');
  await expect(page.locator('text=Aggregate chat from Twitch, YouTube')).toBeVisible();
});

test('should display login button', async ({ page }) => {
  await page.goto('/');
  const loginButton = page.locator('button', { hasText: 'Login with Twitch' });
  await expect(loginButton).toBeVisible();
  await expect(loginButton).toBeEnabled();
});
```

## Common Patterns

**Async Testing (Go):**

Context is passed through test functions for cancellation support:
```go
func TestOverlayHandler_HandleCreateOverlay(t *testing.T) {
	repo := &mockOverlayRepository{
		createFunc: func(ctx context.Context, overlay *models.Overlay) error {
			return nil
		},
	}
	// Test calls handler which passes context through: h.repo.Create(c.Request.Context(), overlay)
}
```

**Error Testing (Go):**

Verify error behavior with separate `wantErr` field in test table:
```go
tests := []struct {
	name    string
	overlay *Overlay
	wantErr bool
}{
	{
		name:    "missing user_id",
		overlay: &Overlay{UserID: "", Name: "Test"},
		wantErr: true,
	},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		err := tt.overlay.Validate()
		if (err != nil) != tt.wantErr {
			t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
		}
	})
}
```

**HTTP Testing (Go):**

Use `net/http/httptest` for testing handlers:
```go
import (
	"net/http/httptest"
)

router := setupTestRouter()  // gin.New() in test mode
handler := NewHealthHandler(nil, nil)
router.GET("/health/live", handler.CheckLive)

req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
w := httptest.NewRecorder()

router.ServeHTTP(w, req)

if w.Code != http.StatusOK {
	t.Errorf("status = %v, want %v", w.Code, http.StatusOK)
}
```

**Note on Coverage Gaps:**
Health check handlers note difficulty testing with concrete types: "Health check tests require actual database and Redis connections" or refactoring handlers to use interfaces (from `health_test.go` comments).

---

*Testing analysis: 2026-02-16*
