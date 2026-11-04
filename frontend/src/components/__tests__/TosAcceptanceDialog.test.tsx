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
import { render, screen, cleanup, fireEvent, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from '@/lib/types/auth'

const pathname = vi.hoisted(() => ({ current: '/dashboard' }))
vi.mock('next/navigation', () => ({ usePathname: () => pathname.current }))

const authApi = vi.hoisted(() => ({ acceptTos: vi.fn() }))
vi.mock('@/lib/api/auth', () => ({ authApi }))

const store = vi.hoisted(() => ({
  user: null as Partial<User> | null,
  isImpersonating: false,
  init: vi.fn(),
  logout: vi.fn(),
}))
vi.mock('@/lib/stores/auth-store', () => ({ useAuthStore: () => store }))

import TosAcceptanceDialog from '@/components/TosAcceptanceDialog'

beforeEach(() => {
  pathname.current = '/dashboard'
  store.user = { id: 'u1', tos_accepted: false }
  store.isImpersonating = false
  authApi.acceptTos.mockResolvedValue({ tos_accepted: true })
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

describe('TosAcceptanceDialog', () => {
  it('lets the user open both documents before accepting', () => {
    render(<TosAcceptanceDialog />)
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
    const terms = screen.getByRole('link', { name: /Terms of Service/ })
    const privacy = screen.getByRole('link', { name: /Privacy Policy/ })
    expect(terms).toHaveAttribute('href', '/legal/terms')
    expect(privacy).toHaveAttribute('href', '/legal/privacy')
    // A new tab keeps the prompt (and the session) where it was.
    expect(terms).toHaveAttribute('target', '_blank')
    expect(privacy).toHaveAttribute('target', '_blank')
  })

  it('accepts only after the checkbox is ticked, then refreshes the user', async () => {
    render(<TosAcceptanceDialog />)
    const accept = screen.getByRole('button', { name: 'Accept' })
    expect(accept).toBeDisabled()

    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(accept)

    await waitFor(() => expect(store.init).toHaveBeenCalled())
    expect(authApi.acceptTos).toHaveBeenCalledTimes(1)
  })

  it('shows an error and stays open when saving fails', async () => {
    authApi.acceptTos.mockRejectedValueOnce(new Error('network'))
    render(<TosAcceptanceDialog />)
    fireEvent.click(screen.getByRole('checkbox'))
    fireEvent.click(screen.getByRole('button', { name: 'Accept' }))

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(store.init).not.toHaveBeenCalled()
    expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  })

  it.each([
    ['already accepted', () => (store.user = { id: 'u1', tos_accepted: true })],
    ['field absent (older backend)', () => (store.user = { id: 'u1' })],
    ['signed out', () => (store.user = null)],
    ['impersonating', () => (store.isImpersonating = true)],
    ['on a legal page', () => (pathname.current = '/legal/terms')],
    ['on an OBS overlay', () => (pathname.current = '/overlay/abc')],
  ])('is not shown when %s', (_, arrange) => {
    arrange()
    render(<TosAcceptanceDialog />)
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })
})
