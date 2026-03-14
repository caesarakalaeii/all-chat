import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'
import { PlatformBadge, Badge } from '@/components/ui/badge'
import type { Platform } from '@/lib/platform-colors'

const platformMeta = {
  title: 'UI/PlatformBadge',
  component: PlatformBadge,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof PlatformBadge>

export default platformMeta
type Story = StoryObj<typeof platformMeta>

export const Twitch: Story = { args: { platform: 'twitch' } }
export const YouTube: Story = { args: { platform: 'youtube' } }
export const Kick: Story = { args: { platform: 'kick' } }
export const TikTok: Story = { args: { platform: 'tiktok' } }
export const TwitchSmall: Story = { args: { platform: 'twitch', size: 'sm' }, name: 'Twitch (sm)' }
export const AllPlatforms: Story = {
  args: { platform: 'twitch' },
  render: () =>
    React.createElement(
      'div',
      { className: 'flex gap-2' },
      React.createElement(PlatformBadge, { platform: 'twitch' as Platform }),
      React.createElement(PlatformBadge, { platform: 'youtube' as Platform }),
      React.createElement(PlatformBadge, { platform: 'kick' as Platform }),
      React.createElement(PlatformBadge, { platform: 'tiktok' as Platform })
    ),
}
export const AllPlatformsSmall: Story = {
  args: { platform: 'twitch', size: 'sm' },
  render: () =>
    React.createElement(
      'div',
      { className: 'flex gap-2' },
      React.createElement(PlatformBadge, { platform: 'twitch' as Platform, size: 'sm' }),
      React.createElement(PlatformBadge, { platform: 'youtube' as Platform, size: 'sm' }),
      React.createElement(PlatformBadge, { platform: 'kick' as Platform, size: 'sm' }),
      React.createElement(PlatformBadge, { platform: 'tiktok' as Platform, size: 'sm' })
    ),
  name: 'All Platforms (sm)',
}
export const GenericBadge: Story = {
  args: { platform: 'twitch' },
  render: () => React.createElement(Badge, null, 'Generic Badge'),
}
