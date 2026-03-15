/**
 * Viewer Settings Page Tests
 *
 * Tests for the Viewer Identity section — Solid Color + Gradient tabbed editor.
 * Environment: jsdom (DOM APIs required for localStorage + React rendering)
 */

// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { expect as jestExpect } from 'vitest'
import * as matchers from '@testing-library/jest-dom/matchers'
jestExpect.extend(matchers)

// Mock next/navigation to prevent router errors in unit tests
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/',
  useSearchParams: () => new URLSearchParams(),
}))

// Mock AppNav to avoid complex component dependencies
vi.mock('@/components/AppNav', () => ({
  AppNav: () => <nav data-testid="app-nav" />,
}))

// Mock fetch globally
const mockFetch = vi.fn().mockResolvedValue({
  ok: true,
  json: () => Promise.resolve({}),
})
global.fetch = mockFetch

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Build a fake base64-encoded JWT payload.
 * We only need the claims payload (middle part of JWT).
 */
function buildFakeJWT(claims: Record<string, unknown>): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const payload = btoa(JSON.stringify({ exp: Math.floor(Date.now() / 1000) + 3600, ...claims }))
  const signature = 'fakesig'
  return `${header}.${payload}.${signature}`
}

// Dynamically import page component so we can reset module state between tests
async function importPage() {
  const mod = await import('../page')
  return mod.default
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe('Viewer Settings Page — Viewer Identity section', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockFetch.mockResolvedValue({ ok: true, json: () => Promise.resolve({}) })
    // Clear localStorage before each test using vitest's mock
    vi.stubGlobal('localStorage', {
      _store: {} as Record<string, string>,
      getItem(key: string) { return this._store[key] ?? null },
      setItem(key: string, value: string) { this._store[key] = value },
      removeItem(key: string) { delete this._store[key] },
      clear() { this._store = {} },
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders Viewer Identity section for authenticated user', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      is_viewer: true,
      platform: 'twitch',
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    // Wait for hydration (three-state guard: undefined → claims)
    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    expect(screen.getByRole('button', { name: /Solid Color/i })).toBeInTheDocument()
  })

  it('Solid Color tab is active by default', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      platform: 'twitch',
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    // Solid Color tab should have active styling (border-b-2 border-primary)
    const solidTab = screen.getByRole('button', { name: /Solid Color/i })
    expect(solidTab.className).toContain('border-b-2')
  })

  it('Gradient tab is disabled for non-premium user', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-456',
      display_name: 'FreeViewer',
      platform: 'twitch',
      is_premium: false,
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    const gradientTab = screen.getByRole('button', { name: /Gradient/i })
    expect(gradientTab).toBeDisabled()
  })

  it('Gradient tab is enabled for premium user', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-789',
      display_name: 'PremiumViewer',
      platform: 'twitch',
      is_premium: true,
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    const gradientTab = screen.getByRole('button', { name: /Gradient/i })
    expect(gradientTab).not.toBeDisabled()
  })

  it('Premium badge shown on gradient tab for non-premium', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-456',
      display_name: 'FreeViewer',
      platform: 'twitch',
      is_premium: false,
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    // "Premium" badge should be visible near the gradient tab
    expect(screen.getByText('Premium')).toBeInTheDocument()
  })

  it('live preview shows display name in solid color tab', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      platform: 'twitch',
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    // The preview section should contain the display name
    // (there may be multiple instances: profile + preview)
    const previewInstances = screen.getAllByText('StreamFan')
    expect(previewInstances.length).toBeGreaterThanOrEqual(1)
  })

  it('solid color autosaves on native color input change', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      platform: 'twitch',
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    // Find the native color input (type="color")
    const colorInput = screen
      .getAllByRole('textbox')
      .find(el => (el as HTMLInputElement).type === 'color') as HTMLInputElement | undefined

    // Get color input by type attribute
    const allInputs = document.querySelectorAll('input[type="color"]')
    expect(allInputs.length).toBeGreaterThan(0)

    const nativeColorInput = allInputs[0] as HTMLInputElement

    await act(async () => {
      fireEvent.change(nativeColorInput, { target: { value: '#ff0000' } })
    })

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/auth/viewer/cosmetics',
        expect.objectContaining({
          method: 'PATCH',
          body: expect.stringContaining('"name_color":"#ff0000"'),
        })
      )
    })
  })

  it('gradient editor shows add stop button', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-premium',
      display_name: 'PremiumViewer',
      platform: 'twitch',
      is_premium: true,
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    // Switch to gradient tab
    const gradientTab = screen.getByRole('button', { name: /Gradient/i })
    fireEvent.click(gradientTab)

    await waitFor(() => {
      expect(screen.getByText(/Add stop/i)).toBeInTheDocument()
    })
  })

  it('gradient editor enforces max 4 stops', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-premium',
      display_name: 'PremiumViewer',
      platform: 'twitch',
      is_premium: true,
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    // Switch to gradient tab
    const gradientTab = screen.getByRole('button', { name: /Gradient/i })
    fireEvent.click(gradientTab)

    await waitFor(() => {
      expect(screen.getByText(/Add stop/i)).toBeInTheDocument()
    })

    // Start with 2 stops, add 2 more to reach 4
    const addStopBtn = screen.getByRole('button', { name: /Add stop/i })
    expect(addStopBtn).not.toBeDisabled()

    fireEvent.click(addStopBtn) // 3 stops
    fireEvent.click(addStopBtn) // 4 stops

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Add stop/i })).toBeDisabled()
    })
  })

  it('gradient save sends null name_color', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-premium',
      display_name: 'PremiumViewer',
      platform: 'twitch',
      is_premium: true,
    })
    localStorage.setItem('viewer_jwt_token', jwt)

    const ViewerSettingsPage = await importPage()
    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Viewer Identity')).toBeInTheDocument()
    })

    // Switch to gradient tab
    const gradientTab = screen.getByRole('button', { name: /Gradient/i })
    fireEvent.click(gradientTab)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Save gradient/i })).toBeInTheDocument()
    })

    // Click Save gradient
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Save gradient/i }))
    })

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/auth/viewer/cosmetics',
        expect.objectContaining({
          method: 'PATCH',
          body: expect.stringContaining('"name_color":null'),
        })
      )
    })
  })
})
