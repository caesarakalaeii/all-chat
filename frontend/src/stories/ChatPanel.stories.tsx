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

import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { useEffect, useRef, useState } from 'react'

import { ChatPanel } from '@/components/overlay/ChatPanel'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

// Mock chatters pinned to one platform each, so the 1:1 username filter
// (which is a per-platform identity) behaves predictably in the demo.
const USERS = [
  { id: 'u1', username: 'pixelpanda', color: '#7ee787', platform: 'twitch' as const },
  { id: 'u2', username: 'novanight', color: '#79c0ff', platform: 'youtube' as const },
  { id: 'u3', username: 'quietfox', color: '#ffa657', platform: 'kick' as const },
  { id: 'u4', username: 'streamfan42', color: '#d2a8ff', platform: 'twitch' as const },
]

const LINES = [
  'hi chat',
  'that was insane',
  'W',
  'what game is this?',
  'gg',
  'no way that worked',
  'clip it',
  'first time here, loving the stream',
  'POG',
  'can you do that again?',
]

function makeItem(i: number): ViewItem {
  const user = USERS[i % USERS.length]
  return {
    id: `m${i}`,
    overlay_id: 'story',
    platform: user.platform,
    channel_id: 'c1',
    channel_name: 'storychannel',
    user: { id: user.id, username: user.username, display_name: user.username, badges: [], color: user.color },
    message: { text: LINES[i % LINES.length], emotes: [] },
    timestamp: new Date().toISOString(),
    metadata: {},
  }
}

const meta = {
  title: 'Overlay/ChatPanel',
  component: ChatPanel,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
  decorators: [
    (Story) => (
      <div className="h-[480px] w-[560px] overflow-hidden rounded-lg border border-border bg-bg">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ChatPanel>

export default meta
type Story = StoryObj<typeof meta>

export const Static: Story = {
  args: { items: Array.from({ length: 12 }, (_, i) => makeItem(i)) },
}

/**
 * Simulates a fast-moving chat: scroll up to pause the feed (the pill counts
 * missed messages), click a username to narrow the panel to that chatter.
 */
const FastChatDemo = () => {
  const [items, setItems] = useState<ViewItem[]>(() =>
    Array.from({ length: 40 }, (_, i) => makeItem(i))
  )
  const counter = useRef(40)
  useEffect(() => {
    const t = setInterval(() => {
      setItems((prev) => [...prev, makeItem((counter.current += 1))].slice(-500))
    }, 400)
    return () => clearInterval(t)
  }, [])
  return <ChatPanel items={items} />
}

export const FastChat: Story = {
  args: { items: [] },
  render: () => <FastChatDemo />,
}
