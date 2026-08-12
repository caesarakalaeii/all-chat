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
import { render, screen, fireEvent, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ModerationControls } from '@/components/overlay/ModerationControls'
import { buildDeleteRequest, moderationApi } from '@/lib/api/moderation'
import type { SourceCapability } from '@/lib/types/moderation'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

// Mock the whole moderation API module (auto-mock → vi.fn() for each export).
vi.mock('@/lib/api/moderation', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api/moderation')>()
  return {
    ...actual,
    moderationApi: {
      getCapabilities: vi.fn(),
      deleteMessage: vi.fn().mockResolvedValue({ status: 'ok' }),
      timeoutUser: vi.fn().mockResolvedValue({ status: 'ok' }),
      banUser: vi.fn().mockResolvedValue({ status: 'ok' }),
      unbanUser: vi.fn().mockResolvedValue({ status: 'ok' }),
    },
  }
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function makeItem(platform: ViewItem['platform']): ViewItem {
  return {
    id: 'msg-uuid-1',
    overlay_id: 'o1',
    platform,
    channel_id: 'chan-1',
    channel_name: 'chan',
    user: { id: 'user-1', username: 'spammer', display_name: 'Spammer', badges: [] },
    message: { text: 'spam', emotes: [] },
    timestamp: '2026-05-31T10:00:00.000Z',
    // Twitch normalizer stamps the native message id here; delete must send it
    // (not the internal UUID) so the Helix call targets the right message.
    metadata: { twitch_message_id: 'twitch-native-99' },
  }
}

const twitchCap: SourceCapability = {
  platform: 'twitch',
  channel_id: 'chan-1',
  channel_name: 'chan',
  moderatable: true,
  actions: ['delete', 'timeout', 'ban', 'unban'],
}

const noop = () => {}

describe('ModerationControls', () => {
  it('Twitch: clicking delete sends the correct body to moderationApi.deleteMessage', () => {
    const item = makeItem('twitch')
    // Wire onDelete exactly as the monitor page does (real builder, mocked api).
    const onDelete = (it: ViewItem) => {
      void moderationApi.deleteMessage('overlay-1', buildDeleteRequest(it))
    }
    render(
      <ModerationControls
        item={item}
        capability={twitchCap}
        onDelete={onDelete}
        onTimeout={noop}
        onBan={noop}
        onUnban={noop}
      />
    )

    const deleteBtn = screen.getByLabelText('Delete message')
    expect(deleteBtn).toBeEnabled()
    fireEvent.click(deleteBtn)

    expect(moderationApi.deleteMessage).toHaveBeenCalledTimes(1)
    expect(moderationApi.deleteMessage).toHaveBeenCalledWith('overlay-1', {
      platform: 'twitch',
      channel_id: 'chan-1',
      native_message_id: 'twitch-native-99',
      target_uuid: 'msg-uuid-1',
    })
  })

  it('Twitch: ban sends the correct body (display name + empty reason omitted)', () => {
    const item = makeItem('twitch')
    const onBan = (it: ViewItem) => {
      void moderationApi.banUser('overlay-1', {
        platform: it.platform,
        channel_id: it.channel_id,
        target_user_id: it.user.id,
        target_username: it.user.display_name,
      })
    }
    render(
      <ModerationControls
        item={item}
        capability={twitchCap}
        onDelete={noop}
        onTimeout={noop}
        onBan={onBan}
        onUnban={noop}
      />
    )
    // Open the per-user popover, then click Ban.
    fireEvent.click(screen.getByLabelText('Moderate user'))
    fireEvent.click(screen.getByText('Ban user'))
    expect(moderationApi.banUser).toHaveBeenCalledWith('overlay-1', {
      platform: 'twitch',
      channel_id: 'chan-1',
      target_user_id: 'user-1',
      target_username: 'Spammer',
    })
  })

  it('TikTok: controls are disabled with a "no moderation API" tooltip', () => {
    const item = makeItem('tiktok')
    render(
      <ModerationControls
        item={item}
        capability={undefined}
        onDelete={noop}
        onTimeout={noop}
        onBan={noop}
        onUnban={noop}
      />
    )
    const deleteBtn = screen.getByLabelText('Delete message')
    expect(deleteBtn).toBeDisabled()
    expect(deleteBtn).toHaveAttribute('title', 'TikTok has no moderation API')
    // The per-user trigger is disabled too, so its popover never opens.
    expect(screen.getByLabelText('Moderate user')).toBeDisabled()
  })

  it('Discord: a bot with only the delete permission shows delete but no per-user menu', () => {
    const item: ViewItem = { ...makeItem('discord'), id: 'discord-snowflake-1', metadata: {} }
    const cap: SourceCapability = {
      platform: 'discord',
      channel_id: 'chan-1',
      channel_name: 'chan',
      moderatable: true,
      actions: ['delete'],
    }
    let deleted: ViewItem | null = null
    render(
      <ModerationControls
        item={item}
        capability={cap}
        onDelete={(it) => {
          deleted = it
        }}
        onTimeout={noop}
        onBan={noop}
        onUnban={noop}
      />
    )

    const deleteBtn = screen.getByLabelText('Delete message')
    expect(deleteBtn).toBeEnabled()
    // No timeout/ban/unban on Discord → the per-user trigger must not render.
    expect(screen.queryByLabelText('Moderate user')).not.toBeInTheDocument()

    fireEvent.click(deleteBtn)
    // The native Discord message id is the message id itself (no twitch_message_id).
    expect(buildDeleteRequest(deleted!)).toEqual({
      platform: 'discord',
      channel_id: 'chan-1',
      native_message_id: 'discord-snowflake-1',
      target_uuid: 'discord-snowflake-1',
    })
  })

  it('Discord: a fully-permissioned bot exposes delete AND the per-user ban/timeout menu', () => {
    const item: ViewItem = { ...makeItem('discord'), id: 'discord-snowflake-2', metadata: {} }
    const cap: SourceCapability = {
      platform: 'discord',
      channel_id: 'chan-1',
      channel_name: 'chan',
      moderatable: true,
      actions: ['delete', 'timeout', 'ban', 'unban'],
    }
    let banned: ViewItem | null = null
    render(
      <ModerationControls
        item={item}
        capability={cap}
        onDelete={noop}
        onTimeout={noop}
        onBan={(it) => {
          banned = it
        }}
        onUnban={noop}
      />
    )

    expect(screen.getByLabelText('Delete message')).toBeEnabled()
    // With ban/timeout/unban granted, the per-user trigger renders and opens.
    fireEvent.click(screen.getByLabelText('Moderate user'))
    fireEvent.click(screen.getByText('Ban user'))
    expect(banned).not.toBeNull()
  })

  // Kick grants delete under a second scope (moderation:chat_message:manage), so a
  // streamer who consented before it existed holds ban-only. The controls follow the
  // capability, so that credential must simply show no delete button.
  it('Kick: a ban-scope-only source offers timeout/ban/unban and no delete', () => {
    const item = makeItem('kick')
    const cap: SourceCapability = {
      platform: 'kick',
      channel_id: 'chan-1',
      channel_name: 'chan',
      moderatable: true,
      actions: ['timeout', 'ban', 'unban'],
    }
    const onBan = (it: ViewItem) => {
      void moderationApi.banUser('overlay-1', {
        platform: it.platform,
        channel_id: it.channel_id,
        target_user_id: it.user.id,
        target_username: it.user.display_name,
      })
    }
    render(
      <ModerationControls
        item={item}
        capability={cap}
        onDelete={noop}
        onTimeout={noop}
        onBan={onBan}
        onUnban={noop}
      />
    )

    // No delete in the capability → the delete button must not render.
    expect(screen.queryByLabelText('Delete message')).not.toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Moderate user'))
    fireEvent.click(screen.getByText('Ban user'))
    expect(moderationApi.banUser).toHaveBeenCalledWith('overlay-1', {
      platform: 'kick',
      channel_id: 'chan-1',
      target_user_id: 'user-1',
      target_username: 'Spammer',
    })
  })

  // Kick's message id IS our message id, so the delete builder's `item.id` branch is the
  // native id here rather than a fallback. Pinning it: if the Kick normalizer ever starts
  // minting its own UUIDs, this breaks instead of silently sending an id Kick rejects.
  it('Kick: delete sends the Kick message UUID as the native id', () => {
    const item: ViewItem = { ...makeItem('kick'), id: 'kick-msg-uuid-7', metadata: {} }
    const cap: SourceCapability = {
      platform: 'kick',
      channel_id: 'chan-1',
      channel_name: 'chan',
      moderatable: true,
      actions: ['delete', 'timeout', 'ban', 'unban'],
    }
    const onDelete = (it: ViewItem) => {
      void moderationApi.deleteMessage('overlay-1', buildDeleteRequest(it))
    }
    render(
      <ModerationControls
        item={item}
        capability={cap}
        onDelete={onDelete}
        onTimeout={noop}
        onBan={noop}
        onUnban={noop}
      />
    )

    const deleteBtn = screen.getByLabelText('Delete message')
    expect(deleteBtn).toBeEnabled()
    fireEvent.click(deleteBtn)
    expect(moderationApi.deleteMessage).toHaveBeenCalledWith('overlay-1', {
      platform: 'kick',
      channel_id: 'chan-1',
      native_message_id: 'kick-msg-uuid-7',
      target_uuid: 'kick-msg-uuid-7',
    })
  })

  it('YouTube: ban-only — no delete button, no timeout, no unban', () => {
    const item = makeItem('youtube')
    const cap: SourceCapability = {
      platform: 'youtube',
      channel_id: 'UCabc',
      channel_name: 'chan',
      moderatable: true,
      actions: ['ban'], // v1 is ban-only (unban needs the YouTube ban resource id)
    }
    render(
      <ModerationControls
        item={item}
        capability={cap}
        onDelete={noop}
        onTimeout={noop}
        onBan={noop}
        onUnban={noop}
      />
    )

    expect(screen.queryByLabelText('Delete message')).not.toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Moderate user'))
    // Only ban is offered; timeout and unban are not YouTube v1 actions.
    expect(screen.getByText('Ban user')).toBeInTheDocument()
    expect(screen.queryByText('Timeout')).not.toBeInTheDocument()
    expect(screen.queryByText('Unban user')).not.toBeInTheDocument()
  })

  it('missing scope: controls are disabled with a "grant permissions" tooltip', () => {
    const item = makeItem('twitch')
    const cap: SourceCapability = {
      platform: 'twitch',
      channel_id: 'chan-1',
      channel_name: 'chan',
      moderatable: false,
      reason: 'missing_scope',
      actions: [],
    }
    render(
      <ModerationControls
        item={item}
        capability={cap}
        onDelete={noop}
        onTimeout={noop}
        onBan={noop}
        onUnban={noop}
      />
    )
    const deleteBtn = screen.getByLabelText('Delete message')
    expect(deleteBtn).toBeDisabled()
    expect(deleteBtn).toHaveAttribute('title', 'Grant moderation permissions to enable mod actions')
  })
})
