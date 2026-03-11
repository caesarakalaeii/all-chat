import type { Meta, StoryObj } from '@storybook/react'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { PlatformBadge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { MonitorPlay, Plus } from 'lucide-react'

// Skeleton loading state — inline (mirrors OverlayGridSkeleton)
function DashboardLoadingStory() {
  return (
    <div className="min-h-screen bg-bg p-8">
      <div className="max-w-7xl mx-auto">
        <div className="flex items-center justify-between mb-8">
          <Skeleton className="h-8 w-32" />
          <Skeleton className="h-10 w-32 rounded-lg" />
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="rounded-xl border border-border bg-surface overflow-hidden">
              <div className="h-[3px] w-full bg-surface-2" />
              <div className="p-6 space-y-3">
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-3 w-3/4" />
                <div className="flex gap-1.5 mt-2">
                  <Skeleton className="h-4 w-12 rounded-full" />
                  <Skeleton className="h-4 w-12 rounded-full" />
                </div>
                <Skeleton className="h-3 w-1/3 mt-3" />
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
      <div className="max-w-7xl mx-auto">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-2xl font-bold text-text">Overlays</h1>
          <Button variant="gradient">
            <Plus className="size-4 mr-2" aria-hidden="true" />
            New Overlay
          </Button>
        </div>
        <div className="flex flex-col items-center py-24 text-center gap-4">
          <MonitorPlay className="size-16 text-text-dim" strokeWidth={1} aria-hidden="true" />
          <h2 className="text-xl font-semibold text-text">No overlays yet</h2>
          <p className="text-text-sub text-sm max-w-sm">
            Create your first overlay to start aggregating chat across platforms.
          </p>
          <div className="flex gap-1.5 mt-2" aria-hidden="true">
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
      sources: [
        { platform: 'twitch' },
        { platform: 'kick' },
        { platform: 'tiktok' },
      ],
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
      <div className="max-w-7xl mx-auto">
        <div className="flex items-center justify-between mb-8">
          <h1 className="text-2xl font-bold text-text">Overlays</h1>
          <Button variant="gradient">
            <Plus className="size-4 mr-2" aria-hidden="true" />
            New Overlay
          </Button>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {SAMPLE_OVERLAYS.map((overlay) => (
            <Card key={overlay.id} interactive className="overflow-hidden cursor-pointer">
              <div style={{ height: '3px', ...getBorder(overlay.sources) }} />
              <div className="p-6">
                <h3 className="font-semibold text-text mb-3">{overlay.name}</h3>
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
