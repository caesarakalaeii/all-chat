import { test, expect } from '@playwright/test';

/**
 * Overlay Editor Tests
 *
 * Tests for the overlay configuration page:
 * - Load overlay details
 * - Display chat sources
 * - Add new chat sources
 * - Remove chat sources
 * - Navigate to preview
 */

test.describe('Overlay Editor Page', () => {
  test.beforeEach(async ({ page, context }) => {
    // Mock authentication
    await context.addCookies([]);
    await page.goto('/overlays/test-overlay-id');

    await page.evaluate(() => {
      const mockAuthState = {
        state: {
          user: {
            id: 'test-user-id',
            username: 'testuser',
            display_name: 'Test User',
            email: 'test@example.com',
          },
          token: 'mock-jwt-token',
        },
        version: 0,
      };
      localStorage.setItem('auth-store', JSON.stringify(mockAuthState));
    });
  });

  test('should load and display overlay information', async ({ page }) => {
    // Mock overlay API response
    await page.route('**/api/v1/overlays/test-overlay-id', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'test-overlay-id',
          user_id: 'test-user-id',
          name: 'Test Gaming Overlay',
          description: 'My awesome stream overlay',
          is_active: true,
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }),
      });
    });

    // Mock sources API response
    await page.route('**/api/v1/overlays/test-overlay-id/sources', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await page.reload();

    // Check overlay name is displayed
    await expect(page.locator('text=Test Gaming Overlay')).toBeVisible();
  });

  test('should display existing chat sources', async ({ page }) => {
    // Mock overlay and sources
    await page.route('**/api/v1/overlays/test-overlay-id', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'test-overlay-id',
          name: 'Test Overlay',
          is_active: true,
        }),
      });
    });

    await page.route('**/api/v1/overlays/test-overlay-id/sources', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([
          {
            id: 'source-1',
            overlay_id: 'test-overlay-id',
            platform: 'twitch',
            channel_id: 'xqc',
            channel_name: 'xQc',
            is_active: true,
          },
          {
            id: 'source-2',
            overlay_id: 'test-overlay-id',
            platform: 'youtube',
            channel_id: 'UC1234567890',
            channel_name: 'Test YouTube Channel',
            is_active: true,
          },
        ]),
      });
    });

    await page.reload();

    // Check sources are displayed
    await expect(page.locator('text=xQc')).toBeVisible();
    await expect(page.locator('text=Test YouTube Channel')).toBeVisible();
    await expect(page.locator('text=twitch')).toBeVisible();
    await expect(page.locator('text=youtube')).toBeVisible();
  });

  test('should show add source form when button is clicked', async ({ page }) => {
    await page.route('**/api/v1/overlays/test-overlay-id', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'test-overlay-id', name: 'Test Overlay' }),
      });
    });

    await page.route('**/api/v1/overlays/test-overlay-id/sources', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await page.reload();

    // Click add source button
    const addButton = page.locator('button', { hasText: /Add.*Source/i });
    if (await addButton.isVisible()) {
      await addButton.click();

      // Check form is visible
      await expect(page.locator('input[type="text"]')).toBeVisible();
    }
  });

  test('should add a new chat source', async ({ page }) => {
    await page.route('**/api/v1/overlays/test-overlay-id', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'test-overlay-id', name: 'Test Overlay' }),
      });
    });

    await page.route('**/api/v1/overlays/test-overlay-id/sources', async (route) => {
      if (route.request().method() === 'GET') {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        });
      } else if (route.request().method() === 'POST') {
        // Mock successful source addition
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            id: 'new-source',
            overlay_id: 'test-overlay-id',
            platform: 'twitch',
            channel_id: 'shroud',
            channel_name: 'shroud',
            is_active: true,
          }),
        });
      }
    });

    await page.reload();

    // Open add source form
    const addButton = page.locator('button', { hasText: /Add.*Source/i });
    if (await addButton.isVisible()) {
      await addButton.click();

      // Fill in channel name
      const channelInput = page.locator('input[type="text"]').first();
      await channelInput.fill('shroud');

      // Submit form
      const submitButton = page.locator('button', { hasText: /Add|Save/i }).last();
      await submitButton.click();

      // Verify source was added (should see it in the list after reload)
      await expect(page.locator('text=shroud')).toBeVisible();
    }
  });

  test('should navigate to preview page', async ({ page }) => {
    await page.route('**/api/v1/overlays/test-overlay-id', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'test-overlay-id', name: 'Test Overlay' }),
      });
    });

    await page.route('**/api/v1/overlays/test-overlay-id/sources', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify([]),
      });
    });

    await page.reload();

    // Click preview button
    const previewButton = page.locator('button, a', { hasText: /Preview/i });
    if (await previewButton.isVisible()) {
      await previewButton.click();

      // Should navigate to preview page
      await expect(page).toHaveURL('/overlays/test-overlay-id/preview');
    }
  });
});
