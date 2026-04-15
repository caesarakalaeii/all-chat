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
import clsx from 'clsx'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { PlatformBadge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { X } from 'lucide-react'

// Mocked split-view layout (no iframe in Storybook)
function OverlayEditorPreview() {
  const SAMPLE_SOURCES = [
    {
      id: '1',
      platform: 'twitch' as const,
      channel_name: 'streamer123',
      borderClass: 'border-l-twitch',
    },
    {
      id: '2',
      platform: 'youtube' as const,
      channel_name: 'My Channel',
      borderClass: 'border-l-youtube',
    },
  ]

  return (
    <div className="flex h-screen overflow-hidden bg-bg">
      {/* Config panel */}
      <div className="w-[40%] flex-shrink-0 overflow-y-auto border-r border-border p-6">
        {/* Header */}
        <div className="mb-6">
          <p className="mb-1 text-sm text-text-sub">← Back</p>
          <h1 className="text-xl font-bold text-text">My Overlay</h1>
        </div>

        {/* Action buttons */}
        <div className="mb-6 flex gap-2">
          <Button variant="outline" size="sm">
            Event Settings
          </Button>
          <Button variant="outline" size="sm">
            Credits
          </Button>
        </div>

        {/* Sources */}
        <h2 className="mb-4 text-base font-semibold text-text">Sources</h2>
        <div className="mb-6 space-y-3">
          {SAMPLE_SOURCES.map((source) => (
            <Card key={source.id} className={clsx('border-l-2 p-4', source.borderClass)}>
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <PlatformBadge platform={source.platform} />
                  <div>
                    <p className="text-sm font-medium text-text">{source.channel_name}</p>
                    <p className="text-xs text-text-sub capitalize">{source.platform}</p>
                  </div>
                </div>
                <Button variant="ghost" size="icon" aria-label={`Remove ${source.channel_name}`}>
                  <X className="size-4" />
                </Button>
              </div>
            </Card>
          ))}
        </div>

        {/* Add source form */}
        <div className="space-y-3 rounded-xl border border-border p-4">
          <h3 className="text-sm font-semibold text-text">Add Source</h3>
          <div className="flex gap-2">
            <select
              className="flex-shrink-0 rounded-lg border border-border bg-surface px-3 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              aria-label="Platform"
            >
              <option value="twitch">Twitch</option>
              <option value="youtube">YouTube</option>
              <option value="kick">Kick</option>
              <option value="tiktok">TikTok</option>
            </select>
            <Input
              placeholder="Channel ID or username"
              className="flex-1"
              aria-label="Channel ID or username"
            />
          </div>
          <Button variant="gradient" className="w-full">
            Add Source
          </Button>
        </div>
      </div>

      {/* Draggable divider */}
      <div
        className="w-1 flex-shrink-0 cursor-col-resize bg-border"
        role="separator"
        aria-orientation="vertical"
        aria-label="Drag to resize panels"
      />

      {/* Preview panel placeholder */}
      <div className="flex flex-1 items-center justify-center bg-neutral-950">
        <p className="text-sm text-text-sub">Preview panel (iframe in production)</p>
      </div>
    </div>
  )
}

// Loading state story
function OverlayEditorLoading() {
  return (
    <div className="flex h-screen overflow-hidden bg-bg">
      <div className="w-[40%] flex-shrink-0 overflow-y-auto border-r border-border p-6">
        <Skeleton className="mb-6 h-7 w-40" />
        <Skeleton className="mb-4 h-5 w-24" />
        <div className="space-y-3">
          {Array.from({ length: 2 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center gap-3 rounded-xl border border-border bg-surface p-4"
            >
              <Skeleton className="size-8 rounded-full" />
              <div className="flex-1 space-y-2">
                <Skeleton className="h-4 w-1/3" />
                <Skeleton className="h-3 w-1/4" />
              </div>
            </div>
          ))}
        </div>
      </div>
      <div className="flex-1 bg-neutral-950" />
    </div>
  )
}

const meta: Meta<typeof OverlayEditorPreview> = {
  title: 'Pages/OverlayEditor',
  component: OverlayEditorPreview,
}
export default meta
type Story = StoryObj<typeof OverlayEditorPreview>

export const Default: Story = {}
export const Loading: StoryObj<typeof OverlayEditorLoading> = {
  render: () => <OverlayEditorLoading />,
}
