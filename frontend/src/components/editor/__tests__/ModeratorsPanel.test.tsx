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

import { ModeratorsPanel } from '@/components/editor/ModeratorsPanel'
import { ApiError } from '@/lib/api/client'
import type { ModeratorGrant, ModeratorList } from '@/lib/types/moderation'

// Base UI's dialog measures the viewport and traps focus; jsdom lacks matchMedia.
if (typeof window.matchMedia !== 'function') {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    }) as unknown as MediaQueryList
}

// vi.hoisted, because vi.mock's factory is lifted above the imports and would
// otherwise reference these before initialization.
const api = vi.hoisted(() => ({
  listModerators: vi.fn(),
  createInvite: vi.fn(),
  updateGrant: vi.fn(),
  revokeGrant: vi.fn(),
  revokeAllModerators: vi.fn(),
}))

vi.mock('@/lib/api/moderation', async () => {
  const actual =
    await vi.importActual<typeof import('@/lib/api/moderation')>('@/lib/api/moderation')
  return { ...actual, moderationApi: api }
})

const OVERLAY = 'aaaaaaaa-1111-1111-1111-111111111111'

function grant(over: Partial<ModeratorGrant> = {}): ModeratorGrant {
  return {
    id: 'grant-1',
    status: 'active',
    moderator_user_id: 'user-2',
    display_name: 'Sarah',
    actions: ['delete', 'timeout'],
    platforms: [{ platform: 'twitch', enabled: true, verification: 'verified' }],
    created_at: '2026-08-01T10:00:00Z',
    accepted_at: '2026-08-01T11:00:00Z',
    ...over,
  }
}

function roster(over: Partial<ModeratorList> = {}): ModeratorList {
  return { moderators: [grant()], cap: 10, used: 1, ...over }
}

beforeEach(() => {
  vi.clearAllMocks()
  api.listModerators.mockResolvedValue(roster())
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
})

afterEach(() => cleanup())

async function renderPanel() {
  const view = render(<ModeratorsPanel overlayId={OVERLAY} />)
  await waitFor(() => expect(api.listModerators).toHaveBeenCalledWith(OVERLAY))
  return view
}

describe('ModeratorsPanel roster', () => {
  it('names each moderator and distinguishes granted actions from withheld ones', async () => {
    // Every action renders as a chip so the streamer can grant it in one click; what the
    // grant actually carries has to be visible in the pressed state, not just the label.
    await renderPanel()
    expect(await screen.findByText('Sarah')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /delete messages/i })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByRole('button', { name: /^timeout$/i })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(screen.getByRole('button', { name: /^ban$/i })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: /^unban$/i })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('grants a withheld action in one click, sending the full new set', async () => {
    api.updateGrant.mockResolvedValue(grant({ actions: ['delete', 'timeout', 'ban'] }))
    await renderPanel()

    fireEvent.click(await screen.findByRole('button', { name: /^ban$/i }))

    await waitFor(() =>
      expect(api.updateGrant).toHaveBeenCalledWith(OVERLAY, 'grant-1', {
        actions: ['delete', 'timeout', 'ban'],
      })
    )
  })

  // Removing the last action would be a 400 and a grant that can do nothing; the right
  // answer is to remove the person.
  it('refuses to empty a grant action set, pointing at removal instead', async () => {
    api.listModerators.mockResolvedValue(roster({ moderators: [grant({ actions: ['delete'] })] }))
    await renderPanel()

    fireEvent.click(await screen.findByRole('button', { name: /delete messages/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/at least one action/i)
    expect(api.updateGrant).not.toHaveBeenCalled()
  })

  it('labels an unredeemed invite by what the streamer typed, not as a moderator', async () => {
    api.listModerators.mockResolvedValue(
      roster({
        moderators: [
          grant({
            id: 'g2',
            status: 'pending',
            display_name: undefined,
            moderator_user_id: undefined,
            invitee_label: 'Bob from Discord',
            invite_expires_at: '2026-08-14T10:00:00Z',
          }),
        ],
      })
    )
    await renderPanel()
    expect(await screen.findByText('Bob from Discord')).toBeInTheDocument()
    expect(screen.getByText(/invite pending/i)).toBeInTheDocument()
  })

  it('explains itself when nobody is delegated yet', async () => {
    api.listModerators.mockResolvedValue(roster({ moderators: [], used: 0 }))
    await renderPanel()
    expect(await screen.findByText(/no one moderates this overlay yet/i)).toBeInTheDocument()
  })

  it('surfaces a load failure with a way to retry instead of an empty panel', async () => {
    api.listModerators.mockRejectedValueOnce(new Error('network'))
    render(<ModeratorsPanel overlayId={OVERLAY} />)

    const retry = await screen.findByRole('button', { name: /try again/i })
    api.listModerators.mockResolvedValue(roster())
    fireEvent.click(retry)
    expect(await screen.findByText('Sarah')).toBeInTheDocument()
  })
})

describe('ModeratorsPanel cap', () => {
  // The backend refuses the 11th invite with 409. Explaining a failure the UI could
  // have prevented is worse than not offering the button.
  it('refuses to offer an invite at the cap, and says why', async () => {
    api.listModerators.mockResolvedValue(
      roster({ moderators: Array.from({ length: 10 }, (_, i) => grant({ id: `g${i}` })), used: 10 })
    )
    await renderPanel()

    const invite = await screen.findByRole('button', { name: /invite a moderator/i })
    expect(invite).toBeDisabled()
    expect(screen.getByText(/10 of 10/i)).toBeInTheDocument()
  })

  it('offers the invite below the cap', async () => {
    await renderPanel()
    expect(await screen.findByRole('button', { name: /invite a moderator/i })).toBeEnabled()
  })
})

describe('ModeratorsPanel invite', () => {
  async function openInvite() {
    await renderPanel()
    fireEvent.click(await screen.findByRole('button', { name: /invite a moderator/i }))
    return screen.findByRole('button', { name: /create invite/i })
  }

  it('sends exactly the actions and platforms the streamer picked', async () => {
    api.createInvite.mockResolvedValue({
      grant_id: 'g9',
      invite_token: 'SEEKRIT-TOKEN-VALUE',
      expires_at: '2026-08-14T10:00:00Z',
      actions: ['delete'],
      platforms: ['twitch'],
    })
    const create = await openInvite()

    // Defaults are delete+timeout; drop timeout and add the twitch leg.
    fireEvent.click(screen.getByRole('checkbox', { name: /timeout/i }))
    fireEvent.click(screen.getByRole('checkbox', { name: /twitch/i }))
    fireEvent.click(create)

    await waitFor(() => expect(api.createInvite).toHaveBeenCalled())
    expect(api.createInvite).toHaveBeenCalledWith(OVERLAY, {
      actions: ['delete'],
      platforms: ['twitch'],
      invitee_label: '',
    })
  })

  // `actions: []` is a 400 server-side, deliberately, so it must be unreachable here.
  it('never submits an empty action set', async () => {
    const create = await openInvite()
    fireEvent.click(screen.getByRole('checkbox', { name: /delete/i }))
    fireEvent.click(screen.getByRole('checkbox', { name: /timeout/i }))

    expect(create).toBeDisabled()
    fireEvent.click(create)
    expect(api.createInvite).not.toHaveBeenCalled()
  })

  // Handed over as the link that redeems it, not a bare secret: a code the recipient has
  // to be told where to paste is not something a streamer can just send.
  it('shows the redemption link once, says it cannot be shown again, and copies it', async () => {
    api.createInvite.mockResolvedValue({
      grant_id: 'g9',
      invite_token: 'SEEKRIT-TOKEN-VALUE',
      expires_at: '2026-08-14T10:00:00Z',
      actions: ['delete', 'timeout'],
      platforms: [],
    })
    const create = await openInvite()
    fireEvent.click(create)

    const link = `${window.location.origin}/moderate/accept?token=SEEKRIT-TOKEN-VALUE`
    expect(await screen.findByText(link)).toBeInTheDocument()
    // The digest is all that is stored, so the copy must not promise a second chance.
    expect(screen.getByText(/won't be shown again/i)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /copy/i }))
    await waitFor(() => expect(navigator.clipboard.writeText).toHaveBeenCalledWith(link))
  })

  it('drops the secret from the DOM once the reveal is dismissed', async () => {
    api.createInvite.mockResolvedValue({
      grant_id: 'g9',
      invite_token: 'SEEKRIT-TOKEN-VALUE',
      expires_at: '2026-08-14T10:00:00Z',
      actions: ['delete'],
      platforms: [],
    })
    const create = await openInvite()
    fireEvent.click(create)
    await screen.findByText(/SEEKRIT-TOKEN-VALUE/)

    fireEvent.click(screen.getByRole('button', { name: /done/i }))
    await waitFor(() => expect(screen.queryByText(/SEEKRIT-TOKEN-VALUE/)).not.toBeInTheDocument())
  })

  // A moderator must never be shown an upgrade prompt, but the OWNER here is the caller,
  // and a vanishing toast is the wrong home for "your plan does not include this".
  it('renders the gate refusal inline with an upgrade link, not as a toast', async () => {
    api.createInvite.mockRejectedValue(
      new ApiError(403, 'delegating moderation requires All-Chat premium', {
        error: 'delegating moderation requires All-Chat premium',
        code: 'delegation_unavailable',
        upgrade_url: '/upgrade',
      })
    )
    const create = await openInvite()
    fireEvent.click(create)

    expect(await screen.findByRole('link', { name: /upgrade/i })).toHaveAttribute(
      'href',
      '/upgrade'
    )
  })

  it('reports a cap refusal from the server if it slips past the local check', async () => {
    api.createInvite.mockRejectedValue(
      new ApiError(409, 'this overlay already has the maximum number of moderators', {
        error: 'this overlay already has the maximum number of moderators',
        code: 'moderator_cap_reached',
        cap: 10,
      })
    )
    const create = await openInvite()
    fireEvent.click(create)
    expect(await screen.findByText(/maximum number of moderators/i)).toBeInTheDocument()
  })
})

describe('ModeratorsPanel grant edits', () => {
  it('sends only the platform that moved, as a partial map', async () => {
    api.updateGrant.mockResolvedValue(
      grant({ platforms: [{ platform: 'kick', enabled: true, verification: 'unverified' }] })
    )
    await renderPanel()

    fireEvent.click(await screen.findByRole('switch', { name: /kick/i }))

    await waitFor(() => expect(api.updateGrant).toHaveBeenCalled())
    expect(api.updateGrant).toHaveBeenCalledWith(OVERLAY, 'grant-1', {
      platforms: { kick: true },
    })
  })

  it('turning a platform off disables that leg rather than dropping the grant', async () => {
    api.updateGrant.mockResolvedValue(
      grant({ platforms: [{ platform: 'twitch', enabled: false, verification: 'verified' }] })
    )
    await renderPanel()

    fireEvent.click(await screen.findByRole('switch', { name: /twitch/i }))
    await waitFor(() =>
      expect(api.updateGrant).toHaveBeenCalledWith(OVERLAY, 'grant-1', {
        platforms: { twitch: false },
      })
    )
    expect(api.revokeGrant).not.toHaveBeenCalled()
  })

  // verification is telemetry: a single transient 403 can set not_a_moderator, so it
  // must read as something to check, never as "this person is blocked".
  it('renders a not_a_moderator leg as advisory readiness, not a denial', async () => {
    api.listModerators.mockResolvedValue(
      roster({
        moderators: [
          grant({
            platforms: [{ platform: 'twitch', enabled: true, verification: 'not_a_moderator' }],
          }),
        ],
      })
    )
    await renderPanel()

    const note = await screen.findByText(/not a moderator on twitch yet/i)
    expect(note).toBeInTheDocument()
    expect(screen.queryByText(/blocked|denied|cannot moderate/i)).not.toBeInTheDocument()
  })
})

describe('ModeratorsPanel revocation', () => {
  it('confirms before revoking, then drops the row', async () => {
    api.revokeGrant.mockResolvedValue({ revoked: true })
    await renderPanel()

    fireEvent.click(await screen.findByRole('button', { name: /remove sarah/i }))
    expect(api.revokeGrant).not.toHaveBeenCalled()

    api.listModerators.mockResolvedValue(roster({ moderators: [], used: 0 }))
    fireEvent.click(await screen.findByRole('button', { name: /^remove$/i }))

    await waitFor(() => expect(api.revokeGrant).toHaveBeenCalledWith(OVERLAY, 'grant-1'))
    await waitFor(() => expect(screen.queryByText('Sarah')).not.toBeInTheDocument())
  })

  it('confirms the kill switch and reports how many it removed', async () => {
    api.listModerators.mockResolvedValue(
      roster({ moderators: [grant(), grant({ id: 'g2', display_name: 'Bob' })], used: 2 })
    )
    api.revokeAllModerators.mockResolvedValue({ revoked: 2 })
    await renderPanel()

    fireEvent.click(await screen.findByRole('button', { name: /remove all/i }))
    expect(api.revokeAllModerators).not.toHaveBeenCalled()

    api.listModerators.mockResolvedValue(roster({ moderators: [], used: 0 }))
    fireEvent.click(await screen.findByRole('button', { name: /remove everyone/i }))

    await waitFor(() => expect(api.revokeAllModerators).toHaveBeenCalledWith(OVERLAY))
    expect(await screen.findByText(/removed 2 moderators/i)).toBeInTheDocument()
  })

  // Revocation is never gated server-side, precisely so a rollback cannot trap a streamer
  // with moderators they cannot remove. The UI must not re-introduce that trap.
  it('keeps revocation available after the gate refuses an invite', async () => {
    api.createInvite.mockRejectedValue(
      new ApiError(403, 'delegating moderation requires All-Chat premium', {
        error: 'delegating moderation requires All-Chat premium',
        code: 'delegation_unavailable',
      })
    )
    await renderPanel()

    fireEvent.click(await screen.findByRole('button', { name: /invite a moderator/i }))
    fireEvent.click(await screen.findByRole('button', { name: /create invite/i }))
    await screen.findByRole('link', { name: /upgrade/i })

    fireEvent.click(screen.getByRole('button', { name: /close/i }))
    expect(await screen.findByRole('button', { name: /remove sarah/i })).toBeEnabled()
    expect(screen.getByRole('button', { name: /remove all/i })).toBeEnabled()
  })
})

describe('ModeratorsPanel authorization', () => {
  // An unknown overlay, an unauthorized caller and a delegated moderator all get the same
  // code-less 403. Reading a role out of that would be a security-relevant mistake.
  it('does not present a code-less 403 as anything about the caller', async () => {
    api.listModerators.mockRejectedValue(
      new ApiError(403, 'not authorized for this overlay', {
        error: 'not authorized for this overlay',
      })
    )
    render(<ModeratorsPanel overlayId={OVERLAY} />)

    expect(await screen.findByRole('button', { name: /try again/i })).toBeInTheDocument()
    expect(screen.queryByText(/you are a moderator/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /upgrade/i })).not.toBeInTheDocument()
  })
})
