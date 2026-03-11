import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import React from 'react'
import { cn } from '@/lib/utils'
import { X } from 'lucide-react'

// Static visual representation of a toast for Storybook.
// The actual @base-ui/react/toast requires a live Provider context which is
// complex to set up in Storybook decorators. This previews the visual appearance
// for COMP-01/02. Runtime behavior (stacking, dismiss timers) is tested in-app.
function ToastPreview({ title, description, type }: {
  title: string
  description?: string
  type: 'success' | 'error' | 'info'
}) {
  const borderClass = type === 'success' ? 'border-l-kick' : type === 'error' ? 'border-l-youtube' : 'border-l-tiktok'
  return (
    <div className={cn(
      "bg-surface-2 border border-border rounded-xl px-4 py-3 shadow-xl min-w-[280px]",
      "border-l-4", borderClass
    )}>
      <div className="flex items-start justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-text">{title}</p>
          {description && <p className="text-xs text-text-sub mt-0.5">{description}</p>}
        </div>
        <button className="text-text-sub hover:text-text transition-colors" aria-label="Close">
          <X className="size-4" />
        </button>
      </div>
    </div>
  )
}

const meta = {
  title: 'UI/Toast',
  component: ToastPreview,
  parameters: { layout: 'centered' },
  tags: ['autodocs'],
} satisfies Meta<typeof ToastPreview>

export default meta
type Story = StoryObj<typeof meta>

export const Success: Story = { args: { title: 'Overlay saved', type: 'success' } }
export const Error: Story = { args: { title: 'Connection failed', description: 'Check your API credentials', type: 'error' } }
export const Info: Story = { args: { title: 'Syncing sources...', type: 'info' } }
