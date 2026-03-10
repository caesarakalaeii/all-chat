import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

// Placeholder until frontend/src/components/ui/badge.tsx is created in Plan 02
function Badge({ children, className, ...props }: React.HTMLAttributes<HTMLSpanElement>) {
  return <span data-slot="badge" className={className} {...props}>{children}</span>
}

const meta = {
  title: 'UI/Badge',
  component: Badge,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Badge>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = { args: { children: 'Badge' } }
export const Twitch: Story = { args: { children: 'Twitch', className: 'bg-[var(--color-twitch)] text-[var(--color-twitch-text)]' } }
export const YouTube: Story = { args: { children: 'YouTube', className: 'bg-[var(--color-youtube)] text-[var(--color-youtube-text)]' } }
export const Kick: Story = { args: { children: 'Kick', className: 'bg-[var(--color-kick)] text-[var(--color-kick-text)]' } }
export const TikTok: Story = { args: { children: 'TikTok', className: 'bg-[var(--color-tiktok)] text-[var(--color-tiktok-text)]' } }
