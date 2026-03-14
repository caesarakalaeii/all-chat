import { test, expect } from '@playwright/test'

/**
 * Landing Page Tests
 *
 * Tests for the home page and initial user experience:
 * - Page loads correctly
 * - Login button is present
 * - Feature highlights are visible
 * - Platform icons are displayed
 */

test.describe('Landing Page', () => {
  test('should load the landing page', async ({ page }) => {
    await page.goto('/')

    // Check page title and main heading
    await expect(page.locator('h1')).toHaveText('All-Chat')

    // Check hero description
    await expect(page.locator('text=Aggregate chat from Twitch, YouTube')).toBeVisible()
  })

  test('should display login button', async ({ page }) => {
    await page.goto('/')

    // Check for "Login with Twitch" button
    const loginButton = page.locator('button', { hasText: 'Login with Twitch' })
    await expect(loginButton).toBeVisible()
    await expect(loginButton).toBeEnabled()
  })

  test('should display platform indicators', async ({ page }) => {
    await page.goto('/')

    // Check for platform names
    await expect(page.locator('text=Twitch')).toBeVisible()
    await expect(page.locator('text=YouTube')).toBeVisible()
  })

  test('should display feature highlights', async ({ page }) => {
    await page.goto('/')

    // Check for feature cards
    await expect(page.locator('text=Multi-Platform')).toBeVisible()
    await expect(page.locator('text=Real-Time')).toBeVisible()
    await expect(page.locator('text=Customizable')).toBeVisible()
  })

  test('should have correct styling and gradient', async ({ page }) => {
    await page.goto('/')

    // Check main container has gradient background
    const container = page.locator('.min-h-screen').first()
    await expect(container).toHaveClass(/bg-gradient-to-br/)
  })
})
