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
import type { ChatBotStatus, ChatCommand, ChatCommandStatus } from '@/lib/types/engagement'
const api = vi.hoisted(() => ({
  toastAdd: vi.fn(),
  listChatCommands: vi.fn(),
  createChatCommand: vi.fn(),
  updateChatCommand: vi.fn(),
  deleteChatCommand: vi.fn(),
  previewChatCommand: vi.fn(),
  getChatCommandsConsentUrl: vi.fn(),
  getChatBotSettings: vi.fn(),
  updateChatBotSettings: vi.fn(),
  getChatTimers: vi.fn(),
  createChatTimer: vi.fn(),
  updateChatTimer: vi.fn(),
  deleteChatTimer: vi.fn(),
}))

const auth = vi.hoisted(() => ({
  getChatBotsStatus: vi.fn(),
  disconnectChatBot: vi.fn(),
  getChatBotGrantUrl: vi.fn(),
}))

const connect = vi.hoisted(() => ({ startChatBotConnect: vi.fn() }))
vi.mock('@/lib/api/engagement', () => ({
  engagementApi: api,
  startChatBotConnect: connect.startChatBotConnect,
}))
vi.mock('@/lib/api/auth', () => ({ authApi: auth }))
const media = vi.hoisted(() => ({ uploadMedia: vi.fn() }))
vi.mock('@/lib/api/media', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/api/media')>()),
  uploadMedia: media.uploadMedia,
}))
vi.mock('@/lib/toast', () => ({ toastManager: { add: api.toastAdd } }))

// The `?bot=` param drives the redirect toast; tests override per case.
let mockSearchParams = new URLSearchParams()
vi.mock('next/navigation', () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => '/commands',
  useSearchParams: () => mockSearchParams,
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
    image_url: '',
    sound_url: '',
    alert_text: '',
    min_role: 'everyone',
    cooldown_seconds: 5,
    enabled: true,
    use_count: 7,
    created_at: '2026-10-01T10:00:00Z',
    updated_at: '2026-10-01T10:00:00Z',
    ...over,
  }
}

function chatBotStatus(over: Partial<ChatBotStatus> = {}): ChatBotStatus {
  return {
    allchat: {
      twitch: { connected: true, login: 'allchatbot', display_name: 'AllChatBot' },
      kick: { connected: true },
      youtube: { connected: true, login: 'allchatbot', display_name: 'AllChatBot' },
    },
    custom: { twitch: null, kick: null, youtube: null },
    twitch_bot_granted: true,
    premium: true,
    ...over,
  }
}

describe('Chat commands page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockSearchParams = new URLSearchParams()
    api.listChatCommands.mockResolvedValue({ commands: [command()], status: readyStatus })
    api.previewChatCommand.mockResolvedValue({
      output: 'Hello ExampleViewer, welcome in!',
      image: '',
      errors: [],
    })
    api.getChatBotSettings.mockResolvedValue({ sender: 'bot' })
    api.getChatTimers.mockResolvedValue({ timers: [] })
    auth.getChatBotsStatus.mockResolvedValue(chatBotStatus())
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

  it('previews the image a command shows on the alerts overlay', async () => {
    const dog = 'https://images.dog.ceo/breeds/akita/1.jpg'
    api.previewChatCommand.mockResolvedValue({ output: 'A dog for you', image: dog, errors: [] })
    render(<CommandsPage />)

    fireEvent.click(await screen.findByRole('button', { name: 'Random dog' }))
    const dialog = await screen.findByRole('dialog')
    const image = within(dialog).getByLabelText('Image on stream')
    expect(image).toHaveValue(
      '{js JSON.parse({urlfetch https://dog.ceo/api/breeds/image/random}).message}'
    )
    expect(await within(dialog).findByRole('img', { name: 'Image preview' })).toHaveAttribute(
      'src',
      dog
    )
    expect(api.previewChatCommand).toHaveBeenCalledWith(
      expect.objectContaining({ image: (image as HTMLInputElement).value })
    )
  })

  it('gives a command its own alert sound and text', async () => {
    const bark = 'https://media.allch.at/allchat-media/u/bark.mp3'
    media.uploadMedia.mockResolvedValue(bark)
    api.createChatCommand.mockImplementation(async (input) =>
      command({ ...input, id: '44444444-4444-4444-4444-444444444444' })
    )
    render(<CommandsPage />)

    fireEvent.click(await screen.findByRole('button', { name: 'Random dog' }))
    const dialog = await screen.findByRole('dialog')
    const file = new File(['x'], 'bark.mp3', { type: 'audio/mpeg' })
    const picker = dialog.querySelector<HTMLInputElement>('input[type="file"]')!
    fireEvent.change(picker, { target: { files: [file] } })
    await waitFor(() => expect(within(dialog).getByLabelText('Alert sound')).toHaveValue(bark))
    expect(media.uploadMedia).toHaveBeenCalledWith(file, expect.anything(), expect.any(Function))

    fireEvent.change(within(dialog).getByLabelText('Alert text'), {
      target: { value: '{user} called the dog' },
    })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(api.createChatCommand).toHaveBeenCalledWith(
        expect.objectContaining({ sound_url: bark, alert_text: '{user} called the dog' })
      )
    )
    expect(await screen.findByText('Alert')).toBeInTheDocument()
  })

  it("says why the last bot reply did not arrive, in the platform's words", async () => {
    api.listChatCommands.mockResolvedValue({
      commands: [command()],
      status: readyStatus,
      send_failures: [
        {
          platform: 'twitch',
          sender: 'allchat_bot',
          error_kind: 'message_dropped',
          detail: 'A verified phone number is required to chat in this channel.',
          at: '2026-11-04T19:47:47Z',
        },
        {
          platform: 'kick',
          sender: 'allchat_bot',
          error_kind: 'reauth_required',
          at: '2026-11-04T19:50:00Z',
        },
      ],
    })
    render(<CommandsPage />)

    const panel = await screen.findByRole('region', { name: 'Bot messages that did not arrive' })
    expect(within(panel).getByText(/Twitch refused it/)).toBeInTheDocument()
    expect(
      within(panel).getByText(/A verified phone number is required to chat in this channel\./)
    ).toBeInTheDocument()
    expect(within(panel).getByText(/The All-Chat Bot lost its authorization/)).toBeInTheDocument()
    expect(
      within(panel).getByText(/Make the bot a moderator or VIP in your channel/)
    ).toBeInTheDocument()
  })

  it('shows no failure panel while every bot message arrives', async () => {
    render(<CommandsPage />)
    expect(await screen.findByText('!discord')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'Bot messages that did not arrive' })).toBeNull()
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

  it('shows the Twitch grant button while channel:bot is not granted, and hides it once granted', async () => {
    auth.getChatBotsStatus.mockResolvedValue(chatBotStatus({ twitch_bot_granted: false }))
    auth.getChatBotGrantUrl.mockRejectedValue(new Error('no redirect in jsdom'))
    render(<CommandsPage />)
    fireEvent.click(await screen.findByRole('button', { name: 'Allow in Twitch chat' }))
    await waitFor(() =>
      expect(auth.getChatBotGrantUrl).toHaveBeenCalledWith(readyStatus.consent_overlay_id)
    )

    cleanup()
    auth.getChatBotsStatus.mockResolvedValue(chatBotStatus({ twitch_bot_granted: true }))
    render(<CommandsPage />)
    await screen.findByText('!discord')
    expect(screen.queryByRole('button', { name: 'Allow in Twitch chat' })).not.toBeInTheDocument()
    expect(screen.getByText('The All-Chat Bot is allowed in your Twitch chat.')).toBeInTheDocument()
  })

  it('offers custom bot connect to premium users and the upgrade link otherwise', async () => {
    auth.getChatBotsStatus.mockResolvedValue(chatBotStatus({ premium: true }))
    connect.startChatBotConnect.mockResolvedValue(undefined)
    render(<CommandsPage />)

    fireEvent.click(await screen.findByRole('button', { name: 'Connect Kick bot' }))
    await waitFor(() => expect(connect.startChatBotConnect).toHaveBeenCalledWith('kick', 'custom'))

    cleanup()
    auth.getChatBotsStatus.mockResolvedValue(chatBotStatus({ premium: false }))
    render(<CommandsPage />)
    const upsell = (await screen.findByText(/Custom bot accounts are a premium feature/))
      .parentElement as HTMLElement
    expect(within(upsell).getByRole('link', { name: 'See premium' })).toHaveAttribute(
      'href',
      '/upgrade'
    )
    expect(screen.queryByRole('button', { name: 'Connect Kick bot' })).not.toBeInTheDocument()
  })

  it('rejects a timer with neither trigger set', async () => {
    render(<CommandsPage />)
    fireEvent.click(await screen.findByRole('button', { name: 'New timer' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('Name'), { target: { value: 'Shoutouts' } })
    fireEvent.change(within(dialog).getByLabelText('Message'), {
      target: { value: 'Come say hi' },
    })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(
      'Set an interval, a minimum message count, or both.'
    )
    expect(api.createChatTimer).not.toHaveBeenCalled()
  })

  it('toasts after the ?bot= redirect', async () => {
    mockSearchParams = new URLSearchParams('bot=connected')
    render(<CommandsPage />)
    expect(await screen.findByText('!discord')).toBeInTheDocument()
    await waitFor(() =>
      expect(api.toastAdd).toHaveBeenCalledWith(
        expect.objectContaining({ title: 'Bot connected.', type: 'success' })
      )
    )
  })
})
