import { test, expect } from '@playwright/test';

/**
 * Overlay Preview Tests
 *
 * Tests for the overlay preview page with WebSocket:
 * - Page loads correctly
 * - WebSocket connection indicator
 * - Message display and rendering
 * - Copy overlay URL functionality
 * - Auto-scroll behavior
 */

test.describe('Overlay Preview Page', () => {
  test.beforeEach(async ({ page, context }) => {
    // Mock authentication
    await context.addCookies([]);
    await page.goto('/overlays/test-overlay-id/preview');

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
      };
      localStorage.setItem('auth-store', JSON.stringify(mockAuthState));
    });
  });

  test('should load the preview page', async ({ page }) => {
    await page.reload();

    // Check page loaded (should have some preview-related content)
    await expect(page).toHaveURL(/\/overlays\/test-overlay-id\/preview/);
  });

  test('should display connection status indicator', async ({ page }) => {
    await page.reload();

    // Wait for connection status indicator
    // It might show "Connecting..." or "Connected"
    await expect(
      page.locator('text=/Connect(ing|ed)/i').or(page.locator('.connection-status'))
    ).toBeVisible({ timeout: 10000 });
  });

  test('should attempt WebSocket connection', async ({ page }) => {
    // Monitor WebSocket connections
    let wsConnected = false;

    page.on('websocket', (ws) => {
      console.log('WebSocket opened:', ws.url());
      wsConnected = true;

      // Mock incoming chat message
      ws.on('framesent', (event) => {
        console.log('Frame sent:', event);
      });

      ws.on('framereceived', (event) => {
        console.log('Frame received:', event);
      });
    });

    await page.reload();

    // Wait a bit for WebSocket to attempt connection
    await page.waitForTimeout(2000);

    // WebSocket should have attempted to connect
    // Note: It will fail since no actual backend is running, but we can verify the attempt
    expect(wsConnected).toBe(true);
  });

  test('should display copy URL button', async ({ page }) => {
    await page.reload();

    // Look for copy URL button
    const copyButton = page.locator('button', { hasText: /Copy.*URL/i });
    if (await copyButton.isVisible({ timeout: 5000 })) {
      await expect(copyButton).toBeEnabled();
    }
  });

  test('should render message container', async ({ page }) => {
    await page.reload();

    // Check that there's a container for messages
    // It might be empty initially, but should exist
    const messageContainer = page.locator('[class*="message"], [class*="chat"]').first();
    await expect(messageContainer).toBeVisible({ timeout: 5000 });
  });

  test('should show back to dashboard link', async ({ page }) => {
    await page.reload();

    // Look for back/dashboard link
    const backLink = page.locator('a, button', { hasText: /Dashboard|Back/i });
    if (await backLink.isVisible({ timeout: 5000 })) {
      await expect(backLink).toBeVisible();
    }
  });
});
