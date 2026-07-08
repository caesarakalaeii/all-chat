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

/**
 * ThemeSwitcher — live, one-click preview of the built-in overlay themes.
 *
 * Used as the landing-page hero centerpiece: the built-in themes are a headline
 * selling point, so this shows what each one actually looks like instead of just
 * naming them. One live preview (reusing the marketplace `ThemePreview` engine,
 * which renders the real shipped CSS) plus a row of pills; clicking a pill
 * re-skins the same sample chat. Because it renders the bundled CSS directly
 * there are no screenshots to keep in sync, and only the active theme is mounted,
 * so exactly one theme's fonts load, on demand as the visitor clicks through.
 */

'use client'

import { useState } from 'react'
import Link from 'next/link'
import clsx from 'clsx'
import ThemePreview from '@/components/theme-marketplace/ThemePreview'
import { getBundledTheme } from '@/lib/theme-marketplace/bundled-themes'
import { SAMPLE_PREVIEW_MESSAGES } from '@/lib/theme-marketplace/constants'
import type { Theme } from '@/lib/theme-marketplace/types'
import { Palette } from 'lucide-react'
import { trackEvent } from '@/lib/analytics'

/**
 * Curated subset of the bundled themes, ordered for visual punch and given
 * short pill labels. Kept as an explicit id→label list (rather than iterating
 * all BUNDLED_THEMES) so the labels stay compact and we skip the near-duplicate
 * `minimal-theme-fixed` variant — this is the "12 themes" the homepage
 * advertises. Any id that no longer resolves is dropped silently below, so a
 * theme rename can never crash the landing page.
 */
const FEATURED_THEMES: ReadonlyArray<{ id: string; label: string; badge?: 'icon' }> = [
  // Minimal leads and is the default — it's the look most streamers end up using.
  { id: 'minimal-theme', label: 'Minimal' },
  { id: 'trading-card-theme', label: 'Trading Card' },
  { id: 'comic-speech-theme', label: 'Comic' },
  { id: 'sticky-notes-theme', label: 'Sticky Notes' },
  { id: 'neo-brutalist-theme', label: 'Neo Brutalist' },
  { id: 'vaporwave-theme', label: 'Vaporwave' },
  { id: 'cyberpunk-theme', label: 'Cyberpunk' },
  { id: 'neon-glass-theme', label: 'Neon Glass' },
  { id: 'modern-dark-theme', label: 'Modern Dark' },
  { id: 'terminal-hacker-theme', label: 'Terminal' },
  { id: 'minecraft-theme', label: 'Minecraft' },
  { id: 'win98-theme', label: 'Windows 98' },
  { id: 'pastel-cute-theme', label: 'Pastel' },
  { id: 'noita-minimal-theme', label: 'Noita' },
  { id: 'high-contrast-theme', label: 'High Contrast' },
  // Minimal Icon is built around the icon platform-badge style — preview it that way.
  { id: 'minimal-icon-theme', label: 'Minimal Icon', badge: 'icon' },
]

interface ResolvedTheme {
  id: string
  label: string
  badge?: 'icon'
  theme: Theme
}

const RESOLVED_THEMES: ResolvedTheme[] = FEATURED_THEMES.flatMap((entry) => {
  const theme = getBundledTheme(entry.id)
  return theme ? [{ ...entry, theme }] : []
})

interface ThemeSwitcherProps {
  /** Extra classes for the outer wrapper — the caller owns margins/positioning. */
  className?: string
  /** Preview viewport height in px. The hero uses a taller preview than the card. */
  previewHeight?: number
}

export function ThemeSwitcher({ className, previewHeight = 300 }: ThemeSwitcherProps) {
  const [activeId, setActiveId] = useState(RESOLVED_THEMES[0]?.id ?? '')

  // Defensive: nothing to show if the bundle was emptied/renamed wholesale.
  if (RESOLVED_THEMES.length === 0) return null

  const active = RESOLVED_THEMES.find((t) => t.id === activeId) ?? RESOLVED_THEMES[0]

  return (
    <div className={clsx('relative mx-auto w-full max-w-2xl', className)}>
      <div className="mb-3 text-xs font-bold tracking-widest text-text-sub uppercase">
        {RESOLVED_THEMES.length} built-in themes — no CSS required
      </div>

      {/* Live preview + caption — mirrors the marketplace ThemeCard composition */}
      <div className="overflow-hidden rounded-xl border border-border bg-surface shadow-2xl">
        <ThemePreview
          css={active.theme.css}
          messages={SAMPLE_PREVIEW_MESSAGES}
          themeId={active.theme.id}
          height={previewHeight}
          platformBadge={active.badge ?? 'text'}
        />
        <div aria-live="polite" className="border-t border-border p-4 text-left">
          <h3 className="mb-1 text-base font-semibold text-text">{active.theme.name}</h3>
          <p className="text-sm text-text-sub">{active.theme.description}</p>
        </div>
      </div>

      {/* Theme pills — click to re-skin the preview above */}
      <div
        role="group"
        aria-label="Preview a theme"
        className="mt-4 flex flex-wrap justify-center gap-2"
      >
        {RESOLVED_THEMES.map((t) => {
          const isActive = t.id === active.id
          return (
            <button
              key={t.id}
              type="button"
              aria-pressed={isActive}
              onClick={() => {
                setActiveId(t.id)
                trackEvent('theme_previewed', { theme: t.id })
              }}
              className={clsx(
                'rounded-full border px-3.5 py-1.5 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none',
                isActive
                  ? 'border-twitch bg-twitch text-white'
                  : 'border-border bg-surface text-text-sub hover:border-twitch/50 hover:text-text'
              )}
            >
              {t.label}
            </button>
          )
        })}
      </div>

      <p className="mt-4 flex items-center justify-center gap-1.5 text-center text-sm text-text-sub">
        <Palette className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>
          Make it yours — fonts, colors, sizing, spacing and more are all{' '}
          <Link
            href="/docs#customize"
            onClick={() =>
              trackEvent('cta_click', { cta: 'customize-gui', location: 'theme-switcher' })
            }
            className="font-medium text-text underline decoration-dotted underline-offset-2 hover:text-twitch"
          >
            point-and-click
          </Link>{' '}
          (no CSS needed), or{' '}
          <Link
            href="/docs#custom-css"
            onClick={() =>
              trackEvent('cta_click', { cta: 'customize-css', location: 'theme-switcher' })
            }
            className="font-medium text-text underline decoration-dotted underline-offset-2 hover:text-twitch"
          >
            go deeper with your own CSS
          </Link>
          .
        </span>
      </p>

      <p className="mt-1.5 text-center text-xs text-text-sub">
        Already have a theme from another tool?{' '}
        <a
          href="https://discord.gg/xCGBSuz39P"
          target="_blank"
          rel="noopener noreferrer"
          onClick={() => trackEvent('outbound_click', { dest: 'discord_theme_help' })}
          className="font-medium text-text underline decoration-dotted underline-offset-2 hover:text-twitch"
        >
          Ask on Discord
        </a>{' '}
        and we&apos;ll help you migrate it.
      </p>
    </div>
  )
}
