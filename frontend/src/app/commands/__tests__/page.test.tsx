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
 * Chat commands editor (/commands, ADR-0079): listing, the permission panel,
 * creating from an example with a live preview, toggling and deleting.
 */

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CommandsPage from '@/app/commands/page'
import type { ChatCommand, ChatCommandStatus } from '@/lib/types/engagement'

const api = vi.hoisted(() => ({
  listChatCommands: vi.fn(),
  createChatCommand: vi.fn(),
  updateChatCommand: vi.fn(),
  deleteChatCommand: vi.fn(),
  previewChatCommand: vi.fn(),
  getChatCommandsConsentUrl: vi.fn(),
}))

vi.mock('@/lib/api/engagement', () => ({ engagementApi: api }))
vi.mock('@/lib/toast', () => ({ toastManager: { add: vi.fn() } }))
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/commands',
  useSearchParams: () => new URLSearchParams(),
}))
vi.mock('@/components/ProtectedRoute', () => ({
  ProtectedRoute: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}))
vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

const readyStatus: ChatCommandStatus = {
  is_premium: false,
  twitch_connected: true,
  twitch_can_send: true,
  twitch_can_clip: true,
  consent_overlay_id: '33333333-3333-3333-3333-333333333333',
}

function command(over: Partial<ChatCommand> = {}): ChatCommand {
  return {
    id: '11111111-1111-1111-1111-111111111111',
    kind: 'command',
    trigger: '!discord',
    response: 'Join us: https://discord.gg/x',
    create_clip: false,
    min_role: 'everyone',
    cooldown_seconds: 5,
    enabled: true,
    use_count: 7,
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...over,
  }
}

describe('Chat commands page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.listChatCommands.mockResolvedValue({ commands: [command()], status: readyStatus })
    api.previewChatCommand.mockResolvedValue({
      output: 'Hello ExampleViewer, welcome in!',
      errors: [],
    })
  })

  afterEach(() => cleanup())

  it('lists commands with their trigger, kind, role and use count', async () => {
    render(<CommandsPage />)
    const row = (await screen.findByText('!discord')).closest('li') as HTMLElement
    expect(within(row).getByText('Command')).toBeInTheDocument()
    expect(within(row).getByText('Everyone')).toBeInTheDocument()
    expect(within(row).getByText('7 uses')).toBeInTheDocument()
  })

  it('offers the Twitch grant when clips are not allowed yet, and YouTube replies as premium', async () => {
    api.listChatCommands.mockResolvedValue({
      commands: [],
      status: { ...readyStatus, twitch_can_clip: false },
    })
    api.getChatCommandsConsentUrl.mockRejectedValue(new Error('offline'))
    render(<CommandsPage />)

    fireEvent.click(await screen.findByRole('button', { name: 'Allow on Twitch' }))
    await waitFor(() =>
      expect(api.getChatCommandsConsentUrl).toHaveBeenCalledWith(readyStatus.consent_overlay_id)
    )
    expect(screen.getByText(/Replies in YouTube chat are premium/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'See premium' })).toHaveAttribute('href', '/upgrade')
  })

  it('creates a command from an example, previewing the reply on the way', async () => {
    const saved = command({ id: '22222222-2222-2222-2222-222222222222', trigger: '!hello' })
    api.createChatCommand.mockResolvedValue(saved)
    render(<CommandsPage />)

    fireEvent.click(await screen.findByRole('button', { name: 'Greeting' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByLabelText('Trigger')).toHaveValue('!hello')

    await waitFor(() =>
      expect(within(dialog).getByText('Hello ExampleViewer, welcome in!')).toBeInTheDocument()
    )
    expect(api.previewChatCommand).toHaveBeenCalledWith(
      expect.objectContaining({ response: 'Hello {user}, welcome in!' })
    )

    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.createChatCommand).toHaveBeenCalledWith(
        expect.objectContaining({ kind: 'command', trigger: '!hello', min_role: 'everyone' })
      )
    )
    expect(await screen.findByText('!hello')).toBeInTheDocument()
  })

  it('inserts a variable at the cursor', async () => {
    render(<CommandsPage />)
    fireEvent.click(await screen.findByRole('button', { name: 'New command' }))
    const dialog = await screen.findByRole('dialog')
    const reply = within(dialog).getByLabelText('Reply') as HTMLTextAreaElement
    fireEvent.change(reply, { target: { value: 'hi ' } })
    reply.setSelectionRange(3, 3)
    fireEvent.click(within(dialog).getByRole('button', { name: 'Insert {user}' }))
    expect(reply).toHaveValue('hi {user}')
  })

  it('shows the server error when a save is rejected', async () => {
    const { ApiError } = await import('@/lib/api/client')
    api.createChatCommand.mockRejectedValue(
      new ApiError(409, 'a chat command with this trigger already exists', {})
    )
    render(<CommandsPage />)
    fireEvent.click(await screen.findByRole('button', { name: 'Greeting' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'a chat command with this trigger already exists'
    )
  })

  it('toggles and deletes a command', async () => {
    api.updateChatCommand.mockImplementation(async (_id: string, body: object) => ({
      ...command(),
      ...body,
    }))
    api.deleteChatCommand.mockResolvedValue(undefined)
    render(<CommandsPage />)

    fireEvent.click(await screen.findByRole('switch', { name: 'Enable !discord' }))
    await waitFor(() =>
      expect(api.updateChatCommand).toHaveBeenCalledWith(
        command().id,
        expect.objectContaining({ enabled: false, trigger: '!discord' })
      )
    )

    fireEvent.click(screen.getByRole('button', { name: 'Delete !discord' }))
    const confirm = await screen.findByRole('alertdialog')
    fireEvent.click(within(confirm).getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(api.deleteChatCommand).toHaveBeenCalledWith(command().id))
    await waitFor(() => expect(screen.queryByText('!discord')).not.toBeInTheDocument())
  })
})
