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
import React from 'react'
import { cn } from '@/lib/utils'
import { X } from 'lucide-react'

// Static visual representation of a toast for Storybook.
// The actual @base-ui/react/toast requires a live Provider context which is
// complex to set up in Storybook decorators. This previews the visual appearance
// for COMP-01/02. Runtime behavior (stacking, dismiss timers) is tested in-app.
function ToastPreview({
  title,
  description,
  type,
}: {
  title: string
  description?: string
  type: 'success' | 'error' | 'info'
}) {
  const borderClass =
    type === 'success' ? 'border-l-kick' : type === 'error' ? 'border-l-youtube' : 'border-l-tiktok'
  return (
    <div
      className={cn(
        'min-w-[280px] rounded-xl border border-border bg-surface-2 px-4 py-3 shadow-xl',
        'border-l-4',
        borderClass
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-text">{title}</p>
          {description && <p className="mt-0.5 text-xs text-text-sub">{description}</p>}
        </div>
        <button className="text-text-sub transition-colors hover:text-text" aria-label="Close">
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
export const Error: Story = {
  args: { title: 'Connection failed', description: 'Check your API credentials', type: 'error' },
}
export const Info: Story = { args: { title: 'Syncing sources...', type: 'info' } }
