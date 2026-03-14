import { test, expect } from '@playwright/test'

/**
 * Dashboard Tests
 *
 * Tests for the main dashboard page:
 * - Redirects to login if not authenticated
 * - Displays user information
 * - Shows overlay list
 * - Create overlay button works
 * - Logout functionality
 */

test.describe('Dashboard Page', () => {
  test('should redirect to login when not authenticated', async ({ page }) => {
    await page.goto('/dashboard')

    // Should redirect to landing page
    await expect(page).toHaveURL('/')
  })

  test.describe('Authenticated User', () => {
    test.beforeEach(async ({ page, context }) => {
      // Mock authentication by setting localStorage
      await context.addCookies([])

      await page.goto('/dashboard')

      // Inject mock auth data into localStorage
      await page.evaluate(() => {
        const mockAuthState = {
          state: {
            user: {
              id: 'test-user-id',
              username: 'testuser',
              display_name: 'Test User',
              profile_image_url: 'https://static-cdn.jtvnw.net/user-default-pictures-uv/test.png',
            },
            token: 'mock-jwt-token',
          },
          version: 0,
        }
        localStorage.setItem('auth-store', JSON.stringify(mockAuthState))
      })

      // Reload to apply auth state
      await page.reload()
    })

    test('should display user information in navbar', async ({ page }) => {
      // Check for user display name
      await expect(page.locator('text=Test User')).toBeVisible()

      // Check for logout button
      await expect(page.locator('button', { hasText: 'Logout' })).toBeVisible()
    })

    test('should display "My Overlays" heading', async ({ page }) => {
      await expect(page.locator('h1', { hasText: 'My Overlays' })).toBeVisible()
    })

    test('should display create overlay button', async ({ page }) => {
      const createButton = page.locator('button', { hasText: 'Create Overlay' })
      await expect(createButton).toBeVisible()
      await expect(createButton).toBeEnabled()
    })

    test('should show empty state when no overlays exist', async ({ page }) => {
      // Mock empty overlays response
      await page.route('**/api/v1/overlays', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([]),
        })
      })

      await page.reload()

      // Check for empty state message
      await expect(page.locator('text=No overlays yet')).toBeVisible()
      await expect(page.locator('text=Create your first overlay')).toBeVisible()
    })

    test('should display overlay cards when overlays exist', async ({ page }) => {
      // Mock overlays response
      await page.route('**/api/v1/overlays', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([
            {
              id: 'overlay-1',
              user_id: 'test-user-id',
              name: 'Test Overlay 1',
              description: 'My first overlay',
              is_active: true,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
            {
              id: 'overlay-2',
              user_id: 'test-user-id',
              name: 'Test Overlay 2',
              description: 'My second overlay',
              is_active: false,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          ]),
        })
      })

      await page.reload()

      // Check for overlay cards
      await expect(page.locator('text=Test Overlay 1')).toBeVisible()
      await expect(page.locator('text=Test Overlay 2')).toBeVisible()

      // Check for active/inactive badges
      await expect(page.locator('text=Active')).toBeVisible()
      await expect(page.locator('text=Inactive')).toBeVisible()
    })

    test('should navigate to overlay editor when card is clicked', async ({ page }) => {
      // Mock overlays response
      await page.route('**/api/v1/overlays', async (route) => {
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify([
            {
              id: 'overlay-1',
              user_id: 'test-user-id',
              name: 'Test Overlay 1',
              description: 'My first overlay',
              is_active: true,
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          ]),
        })
      })

      await page.reload()

      // Click overlay card
      await page.locator('text=Test Overlay 1').click()

      // Should navigate to overlay editor
      await expect(page).toHaveURL('/overlays/overlay-1')
    })

    test('should logout and redirect to landing page', async ({ page }) => {
      // Click logout button
      await page.locator('button', { hasText: 'Logout' }).click()

      // Should redirect to landing page
      await expect(page).toHaveURL('/')

      // Auth should be cleared (check if login button is visible)
      await expect(page.locator('button', { hasText: 'Login with Twitch' })).toBeVisible()
    })
  })
})
