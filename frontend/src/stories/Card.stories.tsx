import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'

// Placeholder until frontend/src/components/ui/card.tsx is created in Plan 02
function Card({ children, className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return <div data-slot="card" className={className} {...props}>{children}</div>
}

const meta = {
  title: 'UI/Card',
  component: Card,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof Card>

export default meta
type Story = StoryObj<typeof meta>

export const Default: Story = { args: { children: 'Card content' } }
export const Interactive: Story = { args: { children: 'Hover me', className: 'hover:scale-[1.02] transition-all' } }
