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
    { id: '1', platform: 'twitch' as const, channel_name: 'streamer123', borderClass: 'border-l-twitch' },
    { id: '2', platform: 'youtube' as const, channel_name: 'My Channel', borderClass: 'border-l-youtube' },
  ]

  return (
    <div className="flex h-screen bg-bg overflow-hidden">
      {/* Config panel */}
      <div className="w-[40%] flex-shrink-0 overflow-y-auto border-r border-border p-6">
        {/* Header */}
        <div className="mb-6">
          <p className="text-text-sub text-sm mb-1">← Back</p>
          <h1 className="text-xl font-bold text-text">My Overlay</h1>
        </div>

        {/* Action buttons */}
        <div className="flex gap-2 mb-6">
          <Button variant="outline" size="sm">Event Settings</Button>
          <Button variant="outline" size="sm">Credits</Button>
        </div>

        {/* Sources */}
        <h2 className="text-base font-semibold text-text mb-4">Sources</h2>
        <div className="space-y-3 mb-6">
          {SAMPLE_SOURCES.map(source => (
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
        <div className="border border-border rounded-xl p-4 space-y-3">
          <h3 className="text-sm font-semibold text-text">Add Source</h3>
          <div className="flex gap-2">
            <select
              className="flex-shrink-0 rounded-lg border border-border bg-surface text-text text-sm px-3 py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
              aria-label="Platform"
            >
              <option value="twitch">Twitch</option>
              <option value="youtube">YouTube</option>
              <option value="kick">Kick</option>
              <option value="tiktok">TikTok</option>
            </select>
            <Input placeholder="Channel ID or username" className="flex-1" aria-label="Channel ID or username" />
          </div>
          <Button variant="gradient" className="w-full">Add Source</Button>
        </div>
      </div>

      {/* Draggable divider */}
      <div
        className="w-1 flex-shrink-0 bg-border cursor-col-resize"
        role="separator"
        aria-orientation="vertical"
        aria-label="Drag to resize panels"
      />

      {/* Preview panel placeholder */}
      <div className="flex-1 bg-neutral-950 flex items-center justify-center">
        <p className="text-text-sub text-sm">Preview panel (iframe in production)</p>
      </div>
    </div>
  )
}

// Loading state story
function OverlayEditorLoading() {
  return (
    <div className="flex h-screen bg-bg overflow-hidden">
      <div className="w-[40%] flex-shrink-0 overflow-y-auto border-r border-border p-6">
        <Skeleton className="h-7 w-40 mb-6" />
        <Skeleton className="h-5 w-24 mb-4" />
        <div className="space-y-3">
          {Array.from({ length: 2 }).map((_, i) => (
            <div key={i} className="rounded-xl border border-border bg-surface p-4 flex items-center gap-3">
              <Skeleton className="size-8 rounded-full" />
              <div className="space-y-2 flex-1">
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
