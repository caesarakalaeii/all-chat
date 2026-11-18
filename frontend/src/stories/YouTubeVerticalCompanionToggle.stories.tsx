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

import { YouTubeVerticalCompanionToggle } from '@/components/overlay/YouTubeVerticalCompanionToggle'
import type { ChatSource } from '@/lib/types/overlay'

// Saving calls the real API client; the stories only cover rendering and the axe gate
// (every switch needs an accessible name).
function source(config: Record<string, unknown>): ChatSource {
  return {
    id: 'src-1',
    overlay_id: 'ovl-1',
    platform: 'youtube',
    channel_id: 'UCabc',
    config,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
    is_active: true,
  }
}

const meta = {
  title: 'Overlay/YouTubeVerticalCompanionToggle',
  component: YouTubeVerticalCompanionToggle,
  parameters: { layout: 'padded' },
  decorators: [
    (Story) => (
      <div className="max-w-md rounded-lg bg-surface-2/50 p-4">
        <Story />
      </div>
    ),
  ],
  args: { source: source({}), overlayId: 'ovl-1', onSaved: () => {} },
} satisfies Meta<typeof YouTubeVerticalCompanionToggle>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = {}
export const UnmarkedTitle: Story = { args: { source: source({ vertical_companion: 'on' }) } }
export const Off: Story = { args: { source: source({ vertical_companion: 'off' }) } }
export const MultiStreamStrategy: Story = { args: { source: source({ stream_select: 'all' }) } }
