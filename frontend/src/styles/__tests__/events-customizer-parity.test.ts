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

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

/**
 * Regression guard for the "preview changes, live overlay doesn't" class of bug.
 *
 * The visual customizer injects `--chat-*` custom properties on `:root` for both
 * the preview/embed pages and the live OBS overlay (`/overlay/[id]`). Historically
 * the *consuming* rules were scoped only to `.overlay-preview-body`, so every
 * setting (emote scale, typography, colors, sizing, bubbles) silently did nothing
 * on the actual overlay — it only ever changed the preview.
 *
 * This test asserts that every `--chat-*` variable consumed under the
 * `.overlay-preview-body` scope is also honored by the live overlay: either via a
 * matching `.overlay-live-body` rule, or via one of the documented exceptions the
 * live overlay handles in React / structurally.
 */

const CSS = readFileSync(fileURLToPath(new URL('../events.css', import.meta.url)), 'utf8')

/** Variables the live overlay deliberately does NOT consume via CSS. */
const LIVE_EXCEPTIONS = new Set<string>([
  // Visibility toggles: the live overlay conditionally renders these in React
  // (showAvatars/showBadges/showUsername/showTimestamps state), so no CSS needed.
  '--chat-show-avatars',
  '--chat-show-badges',
  '--chat-show-username',
  '--chat-show-timestamps',
  // Body font-size: applied inline in React (visual `fontSize` ?? legacy
  // `display.font_size`) so it doesn't clobber the legacy display-settings path.
  '--chat-font-size',
  // Overlay padding: the live overlay owns its outer padding (`p-4`); this is not
  // a user-facing customizer control.
  '--chat-overlay-padding',
])

/** Extract the body of the first `@layer visual-customizer { ... }` block that
 *  contains the given scope selector, then collect the `--chat-*` vars used. */
function varsConsumedUnderScope(scope: string): Set<string> {
  const vars = new Set<string>()
  // Match each rule whose selector list mentions the scope, capture its decls.
  const ruleRe = new RegExp(`${scope.replace('.', '\\.')}[^{]*\\{([^}]*)\\}`, 'g')
  let m: RegExpExecArray | null
  while ((m = ruleRe.exec(CSS)) !== null) {
    const decls = m[1]
    const varRe = /var\((--chat-[a-z-]+)/g
    let v: RegExpExecArray | null
    while ((v = varRe.exec(decls)) !== null) {
      vars.add(v[1])
    }
  }
  return vars
}

describe('events.css visual-customizer scope parity', () => {
  it('defines a live-overlay scope', () => {
    expect(CSS).toContain('.overlay-live-body')
  })

  it('honors every preview-consumed --chat-* variable on the live overlay', () => {
    const preview = varsConsumedUnderScope('.overlay-preview-body')
    const live = varsConsumedUnderScope('.overlay-live-body')

    expect(preview.size).toBeGreaterThan(0)

    const unhandled = [...preview].filter((v) => !live.has(v) && !LIVE_EXCEPTIONS.has(v))

    expect(
      unhandled,
      `These customizer variables apply in the preview but are not honored by ` +
        `the live overlay. Add a rule under \`.overlay-live-body\` in events.css, ` +
        `or document a React/structural exception in this test's LIVE_EXCEPTIONS.`
    ).toEqual([])
  })

  it('keeps emote scale wired up on the live overlay', () => {
    const live = varsConsumedUnderScope('.overlay-live-body')
    expect(live.has('--chat-emote-scale')).toBe(true)
  })

  /**
   * Regression guard for #438 (emotes overlap at larger sizes). Emote scale must
   * grow the layout box (height), NOT `transform: scale()` — a transform grows
   * the emote visually without reserving space, so scaled emotes overlap text and
   * wrap-lines. And the line box must be floored at the emote height so a tall
   * emote can't bleed into adjacent lines.
   */
  it('scales emotes via the layout box, never transform: scale()', () => {
    expect(CSS).not.toMatch(/transform:\s*scale\(var\(--chat-emote-scale/)
    // both scopes size the emote height by the scale factor
    const heightScaleRules = CSS.match(/height:\s*calc\(1\.4em \* var\(--chat-emote-scale, 1\)\)/g)
    expect(heightScaleRules?.length).toBe(2)
  })

  it('floors the message line-height at the scaled emote height', () => {
    const floorRules = CSS.match(
      /line-height:\s*max\(\s*calc\(var\(--chat-line-height, 1\.5\) \* 1em\),\s*calc\(1\.4em \* var\(--chat-emote-scale, 1\)\)\s*\)/g
    )
    expect(floorRules?.length).toBe(2)
  })

  /**
   * The emote-height floor must apply ONLY to messages that actually contain an
   * emote. Flooring every `.break-words` inflated plain-text line spacing and
   * pinned it above the line-height slider's minimum even with no emote present.
   * The floor lives on `:has(img)`; the base rule uses the unfloored line-height.
   */
  it('scopes the emote-height line floor to messages containing an emote', () => {
    const scopedFloor = CSS.match(/\.break-words:has\(img\)\s*\{[^}]*line-height:\s*max\(/g)
    expect(scopedFloor?.length).toBe(2)
  })

  /**
   * The bubble-forcing block uses `!important` inside a cascade layer, which
   * beats a theme's unlayered `!important` — nothing a theme author writes can
   * override it. While it matched `.event-message` it pinned every event to the
   * chat bubble's border/padding: the tier borders never rendered, and themes
   * had no way to restyle the event card. Events take those values as defaults
   * (via `--event-*` in marketplace-themes) where a theme can still win.
   */
  it('excludes events from the unbeatable bubble-forcing rules', () => {
    const forcing = CSS.match(/\.overlay-(preview|live)-body > div[^{]*\{/g) ?? []
    expect(forcing.length).toBe(2)
    for (const rule of forcing) {
      expect(rule, `${rule} must not force chat-bubble chrome onto event rows`).toContain(
        ':not(.event-message)'
      )
    }
  })

  /**
   * The whole point of the token layer: a theme sets `--event-*` instead of
   * hunting down and `!important`-ing every rule. If a default gets hardcoded
   * back into a rule, themed overlays silently regress to the gold glowing card.
   */
  it('drives the event card chrome through --event-* tokens', () => {
    const eventBase = CSS.match(/\n {2}\.event-message \{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    expect(eventBase).not.toBe('')
    for (const token of [
      '--event-min-height',
      '--event-padding',
      '--event-radius',
      '--event-border-width',
      '--event-bg',
      '--event-glow',
      '--event-scale',
      '--event-animation',
    ]) {
      expect(eventBase, `.event-message should read ${token}`).toContain(`var(${token}`)
    }
  })

  /** Structure follows the customizer so an unthemed event matches the overlay. */
  it('defaults event sizing to the chat-bubble customizer values', () => {
    expect(CSS).toContain('var(--event-padding, var(--chat-bubble-padding')
    expect(CSS).toContain('var(--event-radius, var(--chat-bubble-border-radius')
  })

  it('lets plain-text messages use the unfloored line-height', () => {
    const baseLineHeight = CSS.match(
      /line-height:\s*calc\(var\(--chat-line-height, 1\.5\) \* 1em\)\s*!important/g
    )
    // one per scope (preview + live), in the base `.break-words` rule
    expect(baseLineHeight?.length).toBe(2)
  })
})

/**
 * Regression guard for renderer drift (the bug this commit fixes): the live OBS
 * overlay (`/overlay/[id]`) and the editor preview pane (`/overlays/[id]/preview/embed`)
 * each used to carry their own copy of the event renderer. System-notice cases were
 * added to the live overlay but not the preview, so every notice rendered as a
 * generic "EVENT!" in the (narrow) preview pane. The renderer now lives only in the
 * shared `<EventContent>` — this asserts neither route re-introduces its own.
 */
describe('overlay event-renderer parity', () => {
  const read = (rel: string) => readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')
  const live = read('../../app/overlay/[id]/page.tsx')
  const preview = read('../../app/overlays/[id]/preview/embed/page.tsx')

  it('renders events on BOTH surfaces via the shared <EventContent>', () => {
    for (const [name, src] of [
      ['live overlay', live],
      ['preview/embed', preview],
    ] as const) {
      expect(src, `${name} should import the shared EventContent`).toContain(
        "from '@/components/overlay/EventContent'"
      )
      expect(src, `${name} should render <EventContent`).toContain('<EventContent')
    }
  })

  it('keeps the event renderer in one place — no inline event-title switch in either route', () => {
    expect(live, 'live overlay must not re-define getEventTitle').not.toContain('getEventTitle')
    expect(preview, 'preview/embed must not re-define getEventTitle').not.toContain('getEventTitle')
  })

  /**
   * Event chrome is theme-owned, so its size/colour/indent must live in
   * events.css where a theme can reach it — not in unlayered Tailwind utilities
   * that a theme can only fight with `!important`. This is what made an event
   * pop up in default styling on top of a themed overlay.
   */
  it('leaves event size/colour/indent to events.css, not Tailwind utilities', () => {
    // Comments stripped: the file explains this rule by naming the utilities it
    // no longer uses, and that prose must not trip the guard.
    const renderer = read('../../components/overlay/EventContent.tsx').replace(
      /\/\*[\s\S]*?\*\//g,
      ''
    )
    for (const utility of [
      'text-4xl',
      'text-yellow-300',
      'text-slate-200',
      'text-slate-400',
      'ml-14',
    ]) {
      expect(
        renderer,
        `EventContent should not hardcode "${utility}" — express it as an --event-* token in events.css`
      ).not.toContain(utility)
    }
  })
})
