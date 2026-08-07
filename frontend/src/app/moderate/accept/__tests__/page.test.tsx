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

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AcceptInvitePage from '@/app/moderate/accept/page'
import { ApiError } from '@/lib/api/client'
import type { InvitePreview } from '@/lib/types/moderation'

// vi.hoisted, because vi.mock's factory is lifted above the imports and would otherwise
// reference these before initialization.
const api = vi.hoisted(() => ({ previewInvite: vi.fn(), acceptInvite: vi.fn() }))
const nav = vi.hoisted(() => ({ params: new URLSearchParams('token=SEEKRIT'), push: vi.fn() }))
const auth = vi.hoisted(() => ({
  state: { user: { id: 'u1' } as { id: string } | null, loading: false, init: vi.fn() },
}))

vi.mock('@/lib/api/moderation', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/moderation')>('@/lib/api/moderation')
  return { ...actual, moderationApi: api }
})

vi.mock('next/navigation', () => ({
  useSearchParams: () => nav.params,
  usePathname: () => '/moderate/accept',
  useRouter: () => ({ push: nav.push, replace: vi.fn() }),
}))

vi.mock('@/lib/stores/auth-store', () => ({ useAuthStore: () => auth.state }))
vi.mock('@/hooks/useHydrated', () => ({ useHydrated: () => true }))
vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

const OVERLAY = 'aaaaaaaa-1111-1111-1111-111111111111'

function preview(over: Partial<InvitePreview> = {}): InvitePreview {
  return {
    overlay_name: 'Main overlay',
    owner_display_name: 'SomeStreamer',
    actions: ['delete', 'timeout'],
    platforms: [{ platform: 'twitch', enabled: true, verification: 'unverified' }],
    expires_at: '2026-08-14T10:00:00Z',
    ...over,
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  nav.params = new URLSearchParams('token=SEEKRIT')
  auth.state = { user: { id: 'u1' }, loading: false, init: vi.fn() }
})
afterEach(cleanup)

describe('accepting a moderation invite', () => {
  it('shows who is asking, for what, and on which platforms', async () => {
    api.previewInvite.mockResolvedValue(preview())
    render(<AcceptInvitePage />)

    expect(await screen.findByText('Main overlay')).toBeInTheDocument()
    expect(screen.getByText('SomeStreamer')).toBeInTheDocument()
    expect(screen.getByText('Delete messages')).toBeInTheDocument()
    expect(screen.getByText('TWITCH')).toBeInTheDocument()
    // Ban was not delegated, so it must not appear as something they would gain.
    expect(screen.queryByText('Ban viewers')).not.toBeInTheDocument()
  })

  // Consent is deferred to first use (ADR-0048), so the page must not imply that
  // accepting hands anything over yet.
  it('says accepting asks nothing of them yet', async () => {
    api.previewInvite.mockResolvedValue(preview())
    render(<AcceptInvitePage />)

    expect(await screen.findByText(/Nothing is asked of you now/i)).toBeInTheDocument()
  })

  // The overlay id exists only in the accept response — preview withholds it, because an
  // overlay UUID already grants chat READ to whoever holds it.
  it('goes to the overlay the accept response discloses', async () => {
    api.previewInvite.mockResolvedValue(preview())
    api.acceptInvite.mockResolvedValue({
      grant_id: 'g1',
      overlay_id: OVERLAY,
      overlay_name: 'Main overlay',
      owner_display_name: 'SomeStreamer',
      actions: ['delete'],
      platforms: [],
    })
    render(<AcceptInvitePage />)

    fireEvent.click(await screen.findByRole('button', { name: /accept and start moderating/i }))

    await waitFor(() => expect(api.acceptInvite).toHaveBeenCalledWith('SEEKRIT'))
    expect(nav.push).toHaveBeenCalledWith(`/overlay/${OVERLAY}/view`)
  })

  // The server keeps unknown, already-redeemed and revoked deliberately
  // indistinguishable, so the copy must cover all three rather than guessing one.
  it('explains a dead invite without guessing why', async () => {
    api.previewInvite.mockRejectedValue(
      new ApiError(404, 'invite not found', { error: 'invite not found', code: 'invite_not_found' })
    )
    render(<AcceptInvitePage />)

    expect(await screen.findByText(/already have been used/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /accept/i })).not.toBeInTheDocument()
  })

  it('tells an expired invite to ask for a new one', async () => {
    api.previewInvite.mockRejectedValue(
      new ApiError(410, 'invite expired', { error: 'invite expired', code: 'invite_expired' })
    )
    render(<AcceptInvitePage />)

    expect(await screen.findByText(/expired\. Ask the streamer for a new one/i)).toBeInTheDocument()
  })

  // A pre-bound invite refused for the wrong account has to name the account, or the
  // reader has no way to tell which of their accounts to use.
  it('names the account a pre-bound invite belongs to', async () => {
    api.previewInvite.mockResolvedValue(preview())
    api.acceptInvite.mockRejectedValue(
      new ApiError(409, 'invite is bound to another account', {
        error: 'invite is bound to another account',
        code: 'invite_bound_to_other_account',
        expected_account: '@sarah',
        expected_platform: 'twitch',
      })
    )
    render(<AcceptInvitePage />)

    fireEvent.click(await screen.findByRole('button', { name: /accept and start moderating/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent('@sarah')
  })

  it('reports the owner accepting their own invite as the no-op it is', async () => {
    api.previewInvite.mockRejectedValue(
      new ApiError(409, 'the overlay owner cannot accept a delegation', {
        error: 'the overlay owner cannot accept a delegation',
        code: 'owner_cannot_accept',
      })
    )
    render(<AcceptInvitePage />)

    expect(await screen.findByText(/your own overlay/i)).toBeInTheDocument()
  })

  it('refuses a link with no invite code rather than calling the API', async () => {
    nav.params = new URLSearchParams()
    render(<AcceptInvitePage />)

    expect(await screen.findByText(/missing its invite code/i)).toBeInTheDocument()
    expect(api.previewInvite).not.toHaveBeenCalled()
  })

  // The URL is the only copy of the secret the recipient has. A redirect to the homepage
  // would take it with them, so the signed-out state stays on this page.
  it('keeps the invite in place and asks a signed-out visitor to sign in', async () => {
    auth.state = { user: null, loading: false, init: vi.fn() }
    render(<AcceptInvitePage />)

    expect(await screen.findByText(/sign in to accept this invite/i)).toBeInTheDocument()
    expect(api.previewInvite).not.toHaveBeenCalled()
  })
})
