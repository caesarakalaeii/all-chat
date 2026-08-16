/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Settings → API tokens page tests.
 *
 * The load-bearing one is "shows the minted token once and never persists it":
 * the plaintext PAT is returned exactly once by the server and can never be
 * re-fetched, so a client that stashed it in web storage would silently convert
 * a one-shot secret into a durable, exfiltratable one. That test therefore spies
 * on the storage APIs themselves rather than trusting the rendered output.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ApiTokensPage from '@/app/settings/api-tokens/page'
import type { ApiToken, CreatedApiToken } from '@/lib/api/api-tokens'

// vi.hoisted, because vi.mock's factory is lifted above the imports and would
// otherwise reference these before initialization.
const api = vi.hoisted(() => ({
  listApiTokens: vi.fn(),
  createApiToken: vi.fn(),
  revokeApiToken: vi.fn(),
}))

vi.mock('@/lib/api/api-tokens', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/api-tokens')>('@/lib/api/api-tokens')
  return { ...actual, ...api }
})

vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/settings/api-tokens',
  useSearchParams: () => new URLSearchParams(),
}))

// The page sits behind ProtectedRoute, which pulls in the auth store and its
// network init. Token management is what is under test, so the guard is a
// pass-through here.
vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))
vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

const PLAINTEXT = 'allchat_pat_o5Yb3Rq7ZK9wLpX2cVn4TgHs8DmFj1Ea'

function tokenMeta(over: Partial<ApiToken> = {}): ApiToken {
  return {
    id: '11111111-1111-1111-1111-111111111111',
    name: 'Stream Deck (studio PC)',
    scopes: ['chat:write'],
    created_at: '2026-01-05T10:00:00Z',
    last_used_at: null,
    expires_at: null,
    revoked_at: null,
    ...over,
  }
}

function created(over: Partial<CreatedApiToken> = {}): CreatedApiToken {
  return { ...tokenMeta(), token: PLAINTEXT, ...over }
}

/** Fills in the name field and submits the create form. */
async function submitCreateForm(name = 'Stream Deck (studio PC)') {
  const nameInput = await screen.findByLabelText('Token name')
  fireEvent.change(nameInput, { target: { value: name } })
  fireEvent.click(screen.getByRole('button', { name: 'Create token' }))
}

describe('Settings → API tokens page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listApiTokens.mockResolvedValue([])
    api.createApiToken.mockResolvedValue(created())
    api.revokeApiToken.mockResolvedValue(undefined)
    window.localStorage.clear()
    window.sessionStorage.clear()
    window.history.replaceState(null, '', '/settings/api-tokens')
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true,
      writable: true,
    })
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('shows the minted token once and never persists it', async () => {
    // Spy on the real storage APIs: the assertion has to be that no code path
    // wrote the secret, not merely that no element rendered it.
    const localSet = vi.spyOn(Storage.prototype, 'setItem')
    const sessionSet = vi.spyOn(window.sessionStorage, 'setItem')
    const cookieBefore = document.cookie

    render(<ApiTokensPage />)
    await waitFor(() => expect(api.listApiTokens).toHaveBeenCalled())

    await submitCreateForm()

    // 1. The plaintext is shown exactly once, right after creation.
    const reveal = await screen.findByTestId('minted-token-value')
    expect(reveal).toHaveTextContent(PLAINTEXT)
    expect(screen.getAllByText(PLAINTEXT)).toHaveLength(1)

    // 2. …with the "you will not see this again" warning beside it.
    expect(screen.getByText(/only time/i)).toBeInTheDocument()

    // 3. Nothing wrote the token to localStorage. Storage.prototype.setItem
    //    covers localStorage AND sessionStorage, so this catches either.
    for (const call of localSet.mock.calls) {
      expect(String(call[1])).not.toContain(PLAINTEXT)
      expect(String(call[0])).not.toContain(PLAINTEXT)
    }
    for (const call of sessionSet.mock.calls) {
      expect(String(call[1])).not.toContain(PLAINTEXT)
    }

    // 4. …and, belt and braces, the stores are actually empty of it.
    expect(JSON.stringify(window.localStorage)).not.toContain(PLAINTEXT)
    expect(JSON.stringify(window.sessionStorage)).not.toContain(PLAINTEXT)
    expect(window.localStorage.getItem('allchat_pat')).toBeNull()

    // 5. Not in the URL (path, query or hash) and not in a cookie either.
    expect(window.location.href).not.toContain(PLAINTEXT)
    expect(window.location.search).not.toContain(PLAINTEXT)
    expect(window.location.hash).not.toContain(PLAINTEXT)
    expect(document.cookie).toBe(cookieBefore)
    expect(document.cookie).not.toContain(PLAINTEXT)

    // 6. Dismissing the reveal discards the only copy that existed.
    fireEvent.click(screen.getByRole('button', { name: /saved it/i }))
    await waitFor(() => expect(screen.queryByTestId('minted-token-value')).not.toBeInTheDocument())
    expect(screen.queryByText(PLAINTEXT)).not.toBeInTheDocument()

    // Nothing wrote it on the way out, either.
    for (const call of localSet.mock.calls) {
      expect(String(call[1])).not.toContain(PLAINTEXT)
    }
    expect(JSON.stringify(window.localStorage)).not.toContain(PLAINTEXT)
    expect(JSON.stringify(window.sessionStorage)).not.toContain(PLAINTEXT)
  })

  it('creates a token with the name and selected scopes', async () => {
    render(<ApiTokensPage />)
    await waitFor(() => expect(api.listApiTokens).toHaveBeenCalled())

    // chat:write is on by default; add engagement:write.
    fireEvent.click(screen.getByRole('switch', { name: /Run polls and predictions/i }))
    await submitCreateForm('Deck in the studio')

    await waitFor(() =>
      expect(api.createApiToken).toHaveBeenCalledWith('Deck in the studio', [
        'chat:write',
        'engagement:write',
      ])
    )
  })

  it('lists token metadata only and never a secret', async () => {
    api.listApiTokens.mockResolvedValue([
      tokenMeta({ name: 'Living room deck', last_used_at: '2026-02-01T12:00:00Z' }),
    ])

    render(<ApiTokensPage />)

    const row = (await screen.findByText('Living room deck')).closest('li')
    expect(row).not.toBeNull()
    expect(row).toHaveTextContent(/Last used/)
    expect(row).toHaveTextContent('chat:write')
    // Metadata only: no shape of a plaintext token anywhere on the page.
    expect(screen.queryByText(/allchat_pat_/)).not.toBeInTheDocument()
    expect(screen.queryByTestId('minted-token-value')).not.toBeInTheDocument()
  })

  it('shows an empty state explaining tokens and linking the plugin READMEs', async () => {
    render(<ApiTokensPage />)

    expect(await screen.findByText(/don't have any API tokens yet/i)).toBeInTheDocument()

    const streamDeck = screen.getByRole('link', { name: /Stream Deck plugin README/i })
    expect(streamDeck).toHaveAttribute('href', expect.stringContaining('streamdeck-plugin'))
    const streamController = screen.getByRole('link', {
      name: /StreamController plugin README/i,
    })
    expect(streamController).toHaveAttribute(
      'href',
      expect.stringContaining('streamcontroller-plugin')
    )
  })

  it('requires a confirmation step before revoking', async () => {
    api.listApiTokens.mockResolvedValue([tokenMeta({ name: 'Old laptop' })])

    render(<ApiTokensPage />)
    fireEvent.click(await screen.findByRole('button', { name: 'Revoke Old laptop' }))

    // Arming the control must not revoke on its own.
    expect(api.revokeApiToken).not.toHaveBeenCalled()
    expect(await screen.findByText('Revoke this token?')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Revoke token' }))

    await waitFor(() =>
      expect(api.revokeApiToken).toHaveBeenCalledWith('11111111-1111-1111-1111-111111111111')
    )
    await waitFor(() => expect(screen.queryByText('Old laptop')).not.toBeInTheDocument())
  })

  it('cancelling the confirmation leaves the token alone', async () => {
    api.listApiTokens.mockResolvedValue([tokenMeta({ name: 'Old laptop' })])

    render(<ApiTokensPage />)
    fireEvent.click(await screen.findByRole('button', { name: 'Revoke Old laptop' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Cancel' }))

    await waitFor(() => expect(screen.queryByText('Revoke this token?')).not.toBeInTheDocument())
    expect(api.revokeApiToken).not.toHaveBeenCalled()
    expect(screen.getByText('Old laptop')).toBeInTheDocument()
  })

  it('copies the plaintext to the clipboard on request', async () => {
    render(<ApiTokensPage />)
    await waitFor(() => expect(api.listApiTokens).toHaveBeenCalled())
    await submitCreateForm()

    fireEvent.click(await screen.findByRole('button', { name: 'Copy token' }))

    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(PLAINTEXT))
  })
})
