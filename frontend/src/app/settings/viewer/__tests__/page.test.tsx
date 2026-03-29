/**
 * Viewer Settings Page Tests
 *
 * Tests for the Viewer Identity section — Solid Color + Gradient tabbed editor.
 * Environment: jsdom (DOM APIs required for localStorage + React rendering)
 */

// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, act, cleanup } from '@testing-library/react'
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

// Import the page under test
import ViewerSettingsPage from '../page'

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

// Stub localStorage per-test
function stubLocalStorage(initialValues: Record<string, string> = {}) {
  const store: Record<string, string> = { ...initialValues }
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => { store[key] = value },
    removeItem: (key: string) => { delete store[key] },
    clear: () => { Object.keys(store).forEach(k => delete store[k]) },
  })
}

// ---------------------------------------------------------------------------
// Test suite
// ---------------------------------------------------------------------------

describe('Viewer Settings Page — Viewer Identity section', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockFetch.mockResolvedValue({ ok: true, json: () => Promise.resolve({}) })
  })

  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  it('renders Viewer Identity section for authenticated user', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      is_viewer: true,
      platform: 'twitch',
    })
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    // Wait for hydration (three-state guard: undefined → claims)
    await waitFor(() => {
      expect(screen.getAllByText('Viewer Identity').length).toBeGreaterThan(0)
    })

    expect(screen.getByRole('button', { name: /Solid Color/i })).toBeInTheDocument()
  })

  it('Solid Color tab is active by default', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      platform: 'twitch',
    })
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Solid Color/i })).toBeInTheDocument()
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
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Gradient/i })).toBeInTheDocument()
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
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Gradient/i })).toBeInTheDocument()
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
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Gradient/i })).toBeInTheDocument()
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
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Solid Color/i })).toBeInTheDocument()
    })

    // The preview section should contain the display name
    // (multiple instances: profile + preview)
    const instances = screen.getAllByText('StreamFan')
    expect(instances.length).toBeGreaterThanOrEqual(1)
  })

  it('solid color autosaves on native color input change', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      platform: 'twitch',
    })
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Solid Color/i })).toBeInTheDocument()
    })

    // Find the native color input (type="color") — the first one in the solid color tab
    const colorInputs = document.querySelectorAll('input[type="color"]')
    expect(colorInputs.length).toBeGreaterThan(0)
    const nativeColorInput = colorInputs[0] as HTMLInputElement

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
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Gradient/i })).toBeInTheDocument()
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
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Gradient/i })).toBeInTheDocument()
    })

    // Switch to gradient tab
    const gradientTab = screen.getByRole('button', { name: /Gradient/i })
    fireEvent.click(gradientTab)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /\+ Add stop/i })).toBeInTheDocument()
    })

    // Start with 2 stops, add 2 more to reach max 4
    const addStopBtn = screen.getByRole('button', { name: /\+ Add stop/i })
    expect(addStopBtn).not.toBeDisabled()

    fireEvent.click(addStopBtn) // 3 stops
    fireEvent.click(addStopBtn) // 4 stops

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /\+ Add stop/i })).toBeDisabled()
    })
  })

  it('Linked Platforms shows Connected for current platform', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-123',
      display_name: 'StreamFan',
      platform: 'twitch',
    })
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByText('Linked Platforms')).toBeInTheDocument()
    })

    // Twitch should show "Connected" badge
    expect(screen.getByText('Connected')).toBeInTheDocument()

    // YouTube and Kick should show "Connect" buttons
    const connectButtons = screen.getAllByRole('button', { name: /^Connect$/ })
    expect(connectButtons.length).toBe(2)
  })

  it('Connect button calls login API with link_viewer_id', async () => {
    const viewerID = 'v-twitch-123'
    const jwt = buildFakeJWT({
      viewer_id: viewerID,
      display_name: 'StreamFan',
      platform: 'twitch',
    })
    stubLocalStorage({ viewer_jwt_token: jwt })

    // Mock fetch to return an auth_url (but not actually redirect)
    mockFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ auth_url: 'https://oauth.example.com' }),
    })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /^Connect$/ }).length).toBeGreaterThan(0)
    })

    // Click the first "Connect" button (YouTube when logged in via Twitch)
    const connectBtn = screen.getAllByRole('button', { name: /^Connect$/ })[0]
    await act(async () => {
      fireEvent.click(connectBtn)
    })

    // Verify that fetch was called and the URL included link_viewer_id
    await waitFor(() => {
      const calls = mockFetch.mock.calls
      const connectCall = calls.find((args) =>
        typeof args[0] === 'string' &&
        args[0].includes('/api/v1/auth/viewer/') &&
        args[0].includes('link_viewer_id=' + viewerID)
      )
      expect(connectCall).toBeDefined()
    })
  })

  it('gradient save sends null name_color', async () => {
    const jwt = buildFakeJWT({
      viewer_id: 'v-premium',
      display_name: 'PremiumViewer',
      platform: 'twitch',
      is_premium: true,
    })
    stubLocalStorage({ viewer_jwt_token: jwt })

    render(<ViewerSettingsPage />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Gradient/i })).toBeInTheDocument()
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
