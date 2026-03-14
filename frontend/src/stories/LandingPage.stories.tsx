import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { PlatformBadge } from '@/components/ui/badge'
import { PLATFORM_COLORS } from '@/lib/platform-colors'
import { LayoutGrid, Zap, Palette } from 'lucide-react'

// Render the hero card section in isolation for a11y testing
// (real page.tsx has external OAuth dependencies; this story is self-contained)
function LandingHeroCards() {
  return (
    <div className="min-h-screen p-8" style={{ background: '#07070a' }}>
      <h1 className="mb-8 text-center text-4xl font-extrabold text-text">all-chat</h1>

      {/* Platform stat cards */}
      <div className="mx-auto mb-12 grid max-w-3xl grid-cols-2 gap-4 md:grid-cols-4">
        {(['twitch', 'youtube', 'kick', 'tiktok'] as const).map((platform) => (
          <div key={platform} className="rounded-xl border border-border bg-surface p-6">
            <PlatformBadge platform={platform} className="mb-3" />
            <div className={['text-xl font-bold', PLATFORM_COLORS[platform].text].join(' ')}>
              Live
            </div>
          </div>
        ))}
      </div>

      {/* Login buttons — aria-label required for a11y audit */}
      <div className="mb-12 flex flex-col justify-center gap-3 sm:flex-row">
        <button
          className="rounded-lg bg-twitch px-6 py-3 font-semibold text-bg focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          aria-label="Sign in with Twitch"
        >
          Sign in with Twitch
        </button>
        <button
          className="rounded-lg bg-youtube px-6 py-3 font-semibold text-bg focus-visible:ring-2 focus-visible:ring-youtube focus-visible:outline-none"
          aria-label="Sign in with YouTube"
        >
          Sign in with YouTube
        </button>
        <button
          className="rounded-lg px-6 py-3 font-semibold text-bg focus-visible:ring-2 focus-visible:ring-kick focus-visible:outline-none"
          style={{ backgroundColor: 'var(--color-kick)' }}
          aria-label="Sign in with Kick"
        >
          Sign in with Kick
        </button>
      </div>

      {/* Feature grid */}
      <div className="mx-auto grid max-w-4xl grid-cols-1 gap-4 md:grid-cols-3">
        {[
          { icon: LayoutGrid, title: 'Unified Chat', desc: 'All platforms in one overlay.' },
          { icon: Zap, title: 'Real-time Sync', desc: 'Low-latency delivery under 500ms.' },
          { icon: Palette, title: 'Marketplace Themes', desc: '7TV, BTTV, FFZ emotes and themes.' },
        ].map(({ icon: Icon, title, desc }) => (
          <div key={title} className="rounded-xl border border-border bg-surface p-6">
            <Icon className="mb-4 h-7 w-7 text-text-sub" aria-hidden="true" />
            <h2 className="mb-2 text-lg font-semibold text-text">{title}</h2>
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
