import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

import { Card } from '@/components/ui/card'

const meta = {
  title: 'UI/Card',
  component: Card,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Card>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = { args: { children: 'Card content' } }
export const Interactive: Story = { args: { children: 'Hover me', interactive: true } }
export const WithContent: Story = {
  args: {
    children: React.createElement('div', { className: 'p-6' }, 'Card with padding'),
    interactive: false,
  },
}
