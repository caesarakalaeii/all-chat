import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { PlatformBadge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { MonitorPlay, Plus } from 'lucide-react'

// Skeleton loading state — inline (mirrors OverlayGridSkeleton)
function DashboardLoadingStory() {
  return (
    <div className="min-h-screen bg-bg p-8">
      <div className="mx-auto max-w-7xl">
        <div className="mb-8 flex items-center justify-between">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="h-10 w-32 rounded-lg" />
        </div>
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="overflow-hidden rounded-xl border border-border bg-surface">
              <div className="h-[3px] w-full bg-surface-2" />
              <div className="space-y-3 p-6">
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-3 w-3/4" />
                <div className="mt-2 flex gap-1.5">
                  <Skeleton className="h-4 w-12 rounded-full" />
                  <Skeleton className="h-4 w-12 rounded-full" />
                </div>
                <Skeleton className="mt-3 h-3 w-1/3" />
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

// Empty state
function DashboardEmptyStory() {
  return (
    <div className="min-h-screen bg-bg p-8">
      <div className="mx-auto max-w-7xl">
        <div className="mb-8 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-text">Overlays</h1>
          <Button variant="gradient">
            <Plus className="mr-2 size-4" aria-hidden="true" />
            New Overlay
          </Button>
        </div>
        <div className="flex flex-col items-center gap-4 py-24 text-center">
          <MonitorPlay className="size-16 text-text-dim" strokeWidth={1} aria-hidden="true" />
          <h2 className="text-xl font-semibold text-text">No overlays yet</h2>
          <p className="max-w-sm text-sm text-text-sub">
            Create your first overlay to start aggregating chat across platforms.
          </p>
          <div className="mt-2 flex gap-1.5" aria-hidden="true">
            {(['twitch', 'youtube', 'kick', 'tiktok'] as const).map((p) => (
              <PlatformBadge key={p} platform={p} size="sm" />
            ))}
          </div>
          <Button variant="gradient" size="lg" className="mt-4">
            Create your first overlay
          </Button>
        </div>
      </div>
    </div>
  )
}

// Default (with overlay cards)
function DashboardDefaultStory() {
  const SAMPLE_OVERLAYS = [
    {
      id: '1',
      name: 'Main Stream',
      sources: [{ platform: 'twitch' }, { platform: 'youtube' }],
    },
    { id: '2', name: 'TikTok Only', sources: [{ platform: 'tiktok' }] },
    {
      id: '3',
      name: 'Multistream',
      sources: [{ platform: 'twitch' }, { platform: 'kick' }, { platform: 'tiktok' }],
    },
  ]

  const PLATFORM_HEX: Record<string, string> = {
    twitch: '#A37BFF',
    youtube: '#FF4444',
    kick: '#53FC18',
    tiktok: '#69C9D0',
  }

  function getBorder(sources: Array<{ platform: string }>): React.CSSProperties {
    if (sources.length === 1)
      return { background: PLATFORM_HEX[sources[0].platform] ?? 'var(--color-border)' }
    const colors = sources.map((s) => PLATFORM_HEX[s.platform] ?? '#888')
    const seg = 100 / colors.length
    const stops: string[] = []
    colors.forEach((c, i) => {
      stops.push(
        `${c} calc(${i * seg}% + ${i > 0 ? 5 : 0}%)`,
        `${c} calc(${(i + 1) * seg}% - ${i < colors.length - 1 ? 5 : 0}%)`
      )
    })
    return { background: `linear-gradient(90deg, ${stops.join(', ')})` }
  }

  return (
    <div className="min-h-screen bg-bg p-8">
      <div className="mx-auto max-w-7xl">
        <div className="mb-8 flex items-center justify-between">
          <h1 className="text-2xl font-bold text-text">Overlays</h1>
          <Button variant="gradient">
            <Plus className="mr-2 size-4" aria-hidden="true" />
            New Overlay
          </Button>
        </div>
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
          {SAMPLE_OVERLAYS.map((overlay) => (
            <Card key={overlay.id} interactive className="cursor-pointer overflow-hidden">
              <div style={{ height: '3px', ...getBorder(overlay.sources) }} />
              <div className="p-6">
                <h3 className="mb-3 font-semibold text-text">{overlay.name}</h3>
                <div className="flex flex-wrap gap-1.5">
                  {overlay.sources.map((s, i) => (
                    <PlatformBadge
                      key={i}
                      platform={s.platform as 'twitch' | 'youtube' | 'kick' | 'tiktok'}
                      size="sm"
                    />
                  ))}
                </div>
              </div>
            </Card>
          ))}
        </div>
      </div>
    </div>
  )
}

const metaLoading: Meta<typeof DashboardLoadingStory> = {
  title: 'Pages/Dashboard',
  component: DashboardLoadingStory,
}
export default metaLoading
type Story = StoryObj<typeof DashboardLoadingStory>
export const Loading: Story = {}
export const Empty: StoryObj<typeof DashboardEmptyStory> = {
  render: () => <DashboardEmptyStory />,
}
export const Default: StoryObj<typeof DashboardDefaultStory> = {
  render: () => <DashboardDefaultStory />,
}
