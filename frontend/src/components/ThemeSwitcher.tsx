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
 * ThemeSwitcher — auto-advancing showcase of a few hand-picked overlay themes.
 *
 * The landing hero teases the *best, most distinct* themes rather than dumping the
 * whole catalogue on a new visitor: a small carousel cross-fades through a curated
 * set (rendered with the real shipped CSS via the marketplace `ThemePreview`), with
 * dots to step through manually. It shows only two sample messages with no timestamps
 * so no scrollbar ever appears, and uses platform icon badges for a cleaner look.
 *
 * The full "N built-in themes" claim links to the docs; the complete catalogue lives
 * in the dashboard, not the marketing page. (The per-theme contrast gate now lives on
 * the /dev/theme-contrast harness, decoupled from this component — see
 * tests/e2e/theme-contrast.spec.ts.)
 */

'use client'

import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import Link from 'next/link'
import clsx from 'clsx'
import ThemePreview from '@/components/theme-marketplace/ThemePreview'
import { getBundledTheme, getBundledThemes } from '@/lib/theme-marketplace/bundled-themes'
import { SAMPLE_PREVIEW_MESSAGES } from '@/lib/theme-marketplace/constants'
import type { Theme } from '@/lib/theme-marketplace/types'
import { Palette, Pause, Play } from 'lucide-react'
import { trackEvent } from '@/lib/analytics'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { useReducedMotion } from '@/hooks/useReducedMotion'

/**
 * The curated showcase — four visually distinct looks that read well at a glance:
 * a clean inline theme, comic speech balloons, tilted sticky notes, and a modern
 * card layout. All use icon platform badges and hide timestamps for a tidy sample.
 * Any id that no longer resolves is dropped silently so a rename can't crash the page.
 */
const SHOWCASE: ReadonlyArray<{ id: string; label: string }> = [
  { id: 'minimal-theme', label: 'Minimal' },
  { id: 'comic-speech-theme', label: 'Comic' },
  { id: 'sticky-notes-theme', label: 'Sticky Notes' },
  { id: 'modern-dark-theme', label: 'Modern Dark' },
]

interface ResolvedTheme {
  id: string
  label: string
  theme: Theme
}

const RESOLVED: ResolvedTheme[] = SHOWCASE.flatMap((entry) => {
  const theme = getBundledTheme(entry.id)
  return theme ? [{ ...entry, theme }] : []
})

/** Total shipped themes advertised in the caption (excludes the hidden alias). */
const BUILTIN_COUNT = getBundledThemes().filter((t) => t.id !== 'minimal-theme-fixed').length

/** Two short messages across two platforms — enough to show the look, never enough to scroll. */
const SHOWCASE_MESSAGES = SAMPLE_PREVIEW_MESSAGES.slice(0, 2)

const ROTATE_MS = 4200

interface ThemeSwitcherProps {
  /** Extra classes for the outer wrapper — the caller owns margins/positioning. */
  className?: string
}

export function ThemeSwitcher({ className }: ThemeSwitcherProps) {
  const [active, setActive] = useState(0)
  const [paused, setPaused] = useState(false)
  // Explicit pause via the visible control (WCAG 2.2.2 Pause, Stop, Hide) —
  // unlike hover/focus pause, this persists until the visitor resumes.
  const [userPaused, setUserPaused] = useState(false)
  const [frameH, setFrameH] = useState<number>()
  const slideRef = useRef<HTMLDivElement>(null)
  const reducedMotion = useReducedMotion()

  // Auto-advance, paused while the visitor is hovering/focusing the widget or
  // has hit the pause control, and disabled entirely under
  // prefers-reduced-motion (live, not just at mount).
  useEffect(() => {
    if (paused || userPaused || reducedMotion) return
    if (RESOLVED.length < 2) return
    const id = setInterval(() => setActive((a) => (a + 1) % RESOLVED.length), ROTATE_MS)
    return () => clearInterval(id)
  }, [paused, userPaused, reducedMotion])

  // Drive the frame height from the active slide's real content so every theme
  // shows at its true size — no scrollbar and no dead padding. A ResizeObserver
  // keeps it in sync as the theme's webfont finishes loading and reflows.
  useLayoutEffect(() => {
    const el = slideRef.current
    if (!el) return
    const update = () => setFrameH(el.offsetHeight)
    update()
    const ro = new ResizeObserver(update)
    ro.observe(el)
    return () => ro.disconnect()
  }, [active])

  // Defensive: nothing to show if the bundle was emptied/renamed wholesale.
  if (RESOLVED.length === 0) return null

  const current = RESOLVED[active] ?? RESOLVED[0]

  const select = (i: number) => {
    setActive(i)
    trackEvent('theme_previewed', { theme: RESOLVED[i].id })
  }

  return (
    // The wrapper is a purely presentational layout container: the hover/focus
    // listeners only pause the auto-rotation (WCAG 2.2.2) and are not an
    // interaction affordance, so role="presentation" is the honest signal (the
    // carousel semantics live on the role="group" element below). Descendant
    // semantics are unaffected.
    <div
      role="presentation"
      className={clsx('relative mx-auto w-full max-w-2xl', className)}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocusCapture={() => setPaused(true)}
      onBlurCapture={() => setPaused(false)}
    >
      <p className="mb-3 text-xs font-bold tracking-widest text-text-sub uppercase">Themes</p>

      <div
        className="overflow-hidden rounded-xl border border-border-md bg-surface"
        role="group"
        aria-roledescription="carousel"
        aria-label="Featured themes"
      >
        {/* Only the active slide is mounted (one webfont at a time); the frame
            eases between each theme's natural height. */}
        <div
          className="overflow-hidden transition-[height] duration-500 ease-out motion-reduce:transition-none"
          style={{ height: frameH }}
        >
          <div
            key={current.id}
            ref={slideRef}
            className="animate-[theme-fade_0.5s_ease-out] motion-reduce:animate-none"
          >
            <ThemePreview
              css={current.theme.css}
              messages={SHOWCASE_MESSAGES}
              themeId={current.theme.id}
              platformBadge="icon"
              showTimestamp={false}
              fit
            />
          </div>
        </div>

        {/* Caption + dots */}
        <div className="flex items-center justify-between gap-4 border-t border-border p-4 text-left">
          <div className="min-w-0">
            <h3 className="truncate text-base font-semibold text-text">{current.theme.name}</h3>
            <p className="truncate text-sm text-text-sub">{current.theme.description}</p>
          </div>
          <div
            className="flex shrink-0 items-center gap-2"
            role="group"
            aria-label="Choose a theme"
          >
            {/* Under reduced motion the rotation is off entirely, so the
                pause control would be inert — hide it. */}
            {RESOLVED.length > 1 && !reducedMotion && (
              <button
                type="button"
                onClick={() => setUserPaused((p) => !p)}
                aria-label={userPaused ? 'Resume theme rotation' : 'Pause theme rotation'}
                aria-pressed={userPaused}
                className="flex h-6 w-6 items-center justify-center rounded-full text-text-sub hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-surface focus-visible:outline-none"
              >
                {userPaused ? (
                  <Play className="h-3.5 w-3.5" aria-hidden="true" />
                ) : (
                  <Pause className="h-3.5 w-3.5" aria-hidden="true" />
                )}
              </button>
            )}
            {RESOLVED.map((t, i) => {
              const isActive = i === active
              return (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => select(i)}
                  aria-label={`Show ${t.label}`}
                  aria-current={isActive}
                  className={clsx(
                    'h-2.5 rounded-full transition-all focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-surface focus-visible:outline-none',
                    isActive ? 'w-6 bg-twitch' : 'w-2.5 bg-border-md hover:bg-text-sub'
                  )}
                />
              )
            })}
          </div>
        </div>
      </div>

      <p className="mt-4 flex items-center justify-center gap-1.5 text-center text-sm text-text-sub">
        <Palette className="h-4 w-4 shrink-0" aria-hidden="true" />
        <span>
          {BUILTIN_COUNT} built-in themes — restyle any{' '}
          <Link
            href="/docs#customize"
            onClick={() =>
              trackEvent('cta_click', { cta: 'customize-gui', location: 'theme-switcher' })
            }
            className="font-medium text-text underline decoration-dotted underline-offset-2 hover:text-twitch"
          >
            point-and-click
          </Link>
          , or{' '}
          <Link
            href="/docs#custom-css"
            onClick={() =>
              trackEvent('cta_click', { cta: 'customize-css', location: 'theme-switcher' })
            }
            className="font-medium text-text underline decoration-dotted underline-offset-2 hover:text-twitch"
          >
            write your own CSS
          </Link>
          .
        </span>
      </p>

      <p className="mt-1.5 text-center text-sm text-text-sub">
        Coming from another tool?{' '}
        <a
          href={DISCORD_INVITE_URL}
          target="_blank"
          rel="noopener noreferrer"
          onClick={() => trackEvent('outbound_click', { dest: 'discord_theme_help' })}
          className="font-medium text-text underline decoration-dotted underline-offset-2 hover:text-twitch"
        >
          Ask on Discord
        </a>{' '}
        and we&apos;ll port your theme.
      </p>
    </div>
  )
}
