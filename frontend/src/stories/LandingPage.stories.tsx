import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { PlatformBadge } from '@/components/ui/badge'
import { PLATFORM_COLORS } from '@/lib/platform-colors'
import { LayoutGrid, Zap, Palette } from 'lucide-react'

// Render the hero card section in isolation for a11y testing
// (real page.tsx has external OAuth dependencies; this story is self-contained)
function LandingHeroCards() {
  return (
    <div className="min-h-screen p-8" style={{ background: '#07070a' }}>
      <h1 className="text-4xl font-extrabold text-text text-center mb-8">all-chat</h1>

      {/* Platform stat cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 max-w-3xl mx-auto mb-12">
        {(['twitch', 'youtube', 'kick', 'tiktok'] as const).map(platform => (
          <div key={platform} className="rounded-xl border border-border bg-surface p-6">
            <PlatformBadge platform={platform} className="mb-3" />
            <div className={['text-xl font-bold', PLATFORM_COLORS[platform].text].join(' ')}>Live</div>
          </div>
        ))}
      </div>

      {/* Login buttons — aria-label required for a11y audit */}
      <div className="flex flex-col sm:flex-row gap-3 justify-center mb-12">
        <button
          className="px-6 py-3 rounded-lg bg-twitch text-white font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch"
          aria-label="Sign in with Twitch"
        >
          Sign in with Twitch
        </button>
        <button
          className="px-6 py-3 rounded-lg bg-youtube text-white font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-youtube"
          aria-label="Sign in with YouTube"
        >
          Sign in with YouTube
        </button>
        <button
          className="px-6 py-3 rounded-lg text-bg font-semibold focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-kick"
          style={{ backgroundColor: 'var(--color-kick)' }}
          aria-label="Sign in with Kick"
        >
          Sign in with Kick
        </button>
      </div>

      {/* Feature grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 max-w-4xl mx-auto">
        {[
          { icon: LayoutGrid, title: 'Unified Chat', desc: 'All platforms in one overlay.' },
          { icon: Zap, title: 'Real-time Sync', desc: 'Low-latency delivery under 500ms.' },
          { icon: Palette, title: 'Marketplace Themes', desc: '7TV, BTTV, FFZ emotes and themes.' },
        ].map(({ icon: Icon, title, desc }) => (
          <div key={title} className="rounded-xl border border-border bg-surface p-6">
            <Icon className="w-7 h-7 text-text-sub mb-4" aria-hidden="true" />
            <h3 className="text-lg font-semibold text-text mb-2">{title}</h3>
            <p className="text-sm text-text-sub">{desc}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

const meta: Meta<typeof LandingHeroCards> = {
  title: 'Pages/Landing',
  component: LandingHeroCards,
}
export default meta
type Story = StoryObj<typeof LandingHeroCards>

export const HeroWithLoginButtons: Story = {}
export const Default: Story = {}
