# All-Chat Frontend - Testing Guide

This document provides comprehensive instructions for testing the All-Chat frontend application using Playwright.

## Overview

The All-Chat frontend is built with:

- **Next.js 16+** with App Router
- **TypeScript** for type safety
- **Tailwind CSS** for styling
- **Zustand** for state management
- **Playwright** for E2E testing

## Table of Contents

1. [Quick Start](#quick-start)
2. [Running Tests](#running-tests)
3. [Test Structure](#test-structure)
4. [Writing Tests](#writing-tests)
5. [Debugging Tests](#debugging-tests)
6. [CI/CD Integration](#cicd-integration)
7. [Best Practices](#best-practices)

---

## Quick Start

### Prerequisites

```bash
# Install dependencies
npm install

# Install Playwright browsers (first time only)
npx playwright install
```

### Run Tests

```bash
# Run all E2E tests (headless)
npm run test:e2e

# Run tests in UI mode (interactive)
npm run test:e2e:ui

# Run tests in debug mode
npm run test:e2e:debug

# Run tests for a specific file
npx playwright test tests/e2e/landing.spec.ts

# Run tests in a specific browser
npx playwright test --project=chromium
npx playwright test --project=firefox
npx playwright test --project=webkit
```

---

## Running Tests

### Development Mode

During development, use the UI mode for instant feedback:

```bash
npm run test:e2e:ui
```

This opens an interactive browser where you can:

- See tests run in real-time
- Step through test execution
- Inspect element locators
- View screenshots and videos
- Debug failures interactively

### CI Mode

In CI/CD pipelines, run tests headlessly:

```bash
# Run all tests
npm run test:e2e

# Generate HTML report
npm run test:e2e:report
```

### Local Backend Testing

To test against your local backend:

```bash
# Terminal 1: Start backend services
cd ../deployments
docker-compose up

# Terminal 2: Start frontend dev server
npm run dev

# Terminal 3: Run Playwright tests
npm run test:e2e
```

Tests will automatically use `http://localhost:3000` as the base URL.

---

## Test Structure

### Test Organization

```
frontend/
├── tests/
│   └── e2e/
│       ├── landing.spec.ts           # Landing page tests
│       ├── dashboard.spec.ts         # Dashboard tests
│       ├── overlay-editor.spec.ts    # Overlay management tests
│       └── overlay-preview.spec.ts   # Preview & WebSocket tests
└── playwright.config.ts              # Playwright configuration
```

### Test Coverage

| Test File                 | Coverage                                    |
| ------------------------- | ------------------------------------------- |
| `landing.spec.ts`         | Landing page, login button, features        |
| `dashboard.spec.ts`       | Auth flow, overlay listing, CRUD operations |
| `overlay-editor.spec.ts`  | Overlay editing, source management          |
| `overlay-preview.spec.ts` | WebSocket connection, message display       |

---

## Writing Tests

### Basic Test Structure

```typescript
import { test, expect } from '@playwright/test'

test.describe('Feature Name', () => {
  test.beforeEach(async ({ page }) => {
    // Setup: navigate to page, set auth, etc.
    await page.goto('/dashboard')
  })

  test('should perform action', async ({ page }) => {
    // Arrange
    const button = page.locator('button', { hasText: 'Click Me' })

    // Act
    await button.click()

    // Assert
    await expect(page.locator('text=Success')).toBeVisible()
  })
})
```

### Mocking Authentication

```typescript
test.beforeEach(async ({ page, context }) => {
  await context.addCookies([])
  await page.goto('/dashboard')

  // Inject mock auth into localStorage
  await page.evaluate(() => {
    const mockAuthState = {
      state: {
        user: {
          id: 'test-user-id',
          username: 'testuser',
          display_name: 'Test User',
        },
        token: 'mock-jwt-token',
      },
      version: 0,
    }
    localStorage.setItem('auth-store', JSON.stringify(mockAuthState))
  })

  await page.reload()
})
```

### Mocking API Responses

```typescript
test('should display overlays', async ({ page }) => {
  // Mock API response
  await page.route('**/api/v1/overlays', async (route) => {
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify([
        {
          id: 'overlay-1',
          name: 'Test Overlay',
          description: 'My test overlay',
          is_active: true,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
      ]),
    })
  })

  await page.reload()

  // Verify overlay is displayed
  await expect(page.locator('text=Test Overlay')).toBeVisible()
})
```

### Testing WebSocket Connections

```typescript
test('should establish WebSocket connection', async ({ page }) => {
  let wsConnected = false

  page.on('websocket', (ws) => {
    console.log('WebSocket opened:', ws.url())
    wsConnected = true

    // Listen to messages
    ws.on('framereceived', (event) => {
      console.log('Received:', event.payload)
    })
  })

  await page.goto('/overlays/test-id/preview')

  // Wait for WebSocket to connect
  await page.waitForTimeout(2000)

  expect(wsConnected).toBe(true)
})
```

---

## Debugging Tests

### Using Debug Mode

```bash
# Run specific test in debug mode
npm run test:e2e:debug -- tests/e2e/dashboard.spec.ts
```

This will:

- Open a browser window
- Pause execution at breakpoints
- Allow step-by-step execution
- Show Playwright Inspector

### Using UI Mode

```bash
npm run test:e2e:ui
```

Benefits:

- See test execution visually
- Time-travel debugging
- Inspect DOM at any point
- View network requests
- See console logs

### Screenshots and Videos

Tests automatically capture:

- **Screenshots**: On failure only
- **Videos**: Retained on failure
- **Traces**: On first retry

View artifacts in `test-results/` directory.

### Console Output

```typescript
test('should log debug info', async ({ page }) => {
  // Listen to console logs
  page.on('console', (msg) => console.log('PAGE LOG:', msg.text()))

  await page.goto('/dashboard')
})
```

---

## CI/CD Integration

### GitHub Actions

```yaml
name: E2E Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-node@v7
        with:
          node-version: '22'

      - name: Install dependencies
        run: npm ci

      - name: Install Playwright Browsers
        run: npx playwright install --with-deps

      - name: Run Playwright tests
        run: npm run test:e2e

      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v7
        with:
          name: playwright-report
          path: playwright-report/
```

### Environment Variables

Set these in your CI environment:

```bash
PLAYWRIGHT_TEST_BASE_URL=http://localhost:3000
CI=true
```

---

## Best Practices

### 1. Use Descriptive Test Names

✅ **Good:**

```typescript
test('should display error message when login fails', async ({ page }) => {
  // ...
})
```

❌ **Bad:**

```typescript
test('test 1', async ({ page }) => {
  // ...
})
```

### 2. Use Locators Wisely

✅ **Good:** Use semantic locators

```typescript
page.locator('button', { hasText: 'Login' })
page.getByRole('button', { name: 'Login' })
page.getByLabel('Email')
```

❌ **Bad:** Use brittle selectors

```typescript
page.locator('.btn-primary-123')
page.locator('#submit-button')
```

### 3. Wait for Conditions

✅ **Good:**

```typescript
await expect(page.locator('text=Success')).toBeVisible()
```

❌ **Bad:**

```typescript
await page.waitForTimeout(5000) // Flaky!
```

### 4. Independent Tests

Each test should:

- Be independent (no shared state)
- Clean up after itself
- Not depend on test order

### 5. Mock External Services

Always mock:

- API responses
- WebSocket connections
- OAuth redirects
- Third-party services

### 6. Test Critical User Flows

Focus on:

- Authentication (login/logout)
- Main user journeys
- Payment flows
- Data submission

### 7. Keep Tests Fast

- Use parallel execution
- Mock heavy operations
- Skip unnecessary waits
- Cache dependencies

---

## Troubleshooting

### Issue: Tests timeout

**Solution:** Increase timeout in `playwright.config.ts`:

```typescript
use: {
  actionTimeout: 10000, // 10 seconds
  navigationTimeout: 30000, // 30 seconds
}
```

### Issue: Flaky tests

**Causes:**

- Race conditions
- Hard-coded waits
- Network issues

**Solutions:**

- Use `waitForSelector` instead of `waitForTimeout`
- Enable retries for flaky tests
- Mock network requests

### Issue: Browser not found

**Solution:**

```bash
npx playwright install --force
```

### Issue: WebSocket tests fail

**Solution:** Start local backend:

```bash
cd ../deployments
docker-compose up
```

---

## Additional Resources

- [Playwright Documentation](https://playwright.dev)
- [Next.js Testing Guide](https://nextjs.org/docs/testing)
- [All-Chat Backend README](../README.md)
- [Main Project Documentation](../docs/GETTING_STARTED.md)

---

## Support

For issues or questions:

1. Check [GitHub Issues](https://github.com/caesarakalaeii/all-chat/issues)
2. Review [Playwright Troubleshooting](https://playwright.dev/docs/troubleshooting)
3. Ask in project discussions

---

**Last Updated**: November 13, 2025
**Playwright Version**: 1.56+
**Next.js Version**: 14.2+
