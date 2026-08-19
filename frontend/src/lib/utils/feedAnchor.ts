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
 * Which edge of the OBS canvas the chat feed is glued to.
 *
 * There are two independent axes in the feed's layout and it is worth keeping
 * them apart, because conflating them is exactly what made issue #728 happen:
 *
 * - **Order** (`display_settings.invert_message_order`) — which END OF THE LIST
 *   holds the newest message. Presentation-only; the append path is always
 *   push-to-end.
 * - **Anchor** (`display_settings.feed_anchor`, this module) — which EDGE OF
 *   THE CANVAS the stack rests on, therefore where blank space accumulates and
 *   therefore which way a short feed grows.
 *
 * All four combinations are coherent. `'bottom'` with order NOT inverted is the
 * Twitch-like arrangement: newest at the bottom, older messages pushed upward,
 * blank space at the top.
 *
 * The default is `'top'` and an absent key must resolve to `'top'`: flipping it
 * would relocate messages on every existing overlay whose canvas is not full,
 * which is a visible change to somebody else's live scene.
 */
export type FeedAnchor = 'top' | 'bottom'

/** Default anchor. An absent/invalid `feed_anchor` resolves to this. */
export const DEFAULT_FEED_ANCHOR: FeedAnchor = 'top'

const VALID_ANCHORS: ReadonlySet<string> = new Set<FeedAnchor>(['top', 'bottom'])

/**
 * Narrow an untrusted `display_settings.feed_anchor` to a `FeedAnchor`.
 * `DisplaySettings` round-trips through a `map[string]any` on the backend with
 * no key whitelist, so anything at all can arrive here.
 */
export function parseFeedAnchor(value: unknown): FeedAnchor {
  return typeof value === 'string' && VALID_ANCHORS.has(value)
    ? (value as FeedAnchor)
    : DEFAULT_FEED_ANCHOR
}

/** The layout decision for one (anchor, invert) pair. */
export interface FeedAnchorLayout {
  /** Resolved anchor, echoed so callers can use one object. */
  anchor: FeedAnchor
  /**
   * Classes for the OUTER wrapper (the full-canvas box that contains the
   * message list). Bottom anchoring makes it a flex column so the body's auto
   * margin has something to resolve against.
   */
  wrapperClass: string
  /**
   * Classes for the message list itself (`.overlay-live-body` /
   * `.overlay-preview-body`).
   *
   * `mt-auto` is deliberately on the list, NOT on its children:
   * `.overlay-live-body > * + * { margin-top: ... !important }` in events.css
   * (`@layer visual-customizer`) would win over any child rule, and the
   * `:first-child` variant would fight `.scroll-anchor`'s
   * `margin: 0 !important` in globals.css when the sentinel leads the list.
   *
   * When the content is taller than the wrapper the auto margin resolves to
   * `0`, so a busy chat behaves exactly as it does today — the mode is a no-op
   * rather than an overflow hazard. That is why there is no
   * `justify-content: flex-end` here.
   */
  bodyClass: string
  /**
   * Stable theme hook: the value for `data-feed-anchor` on the wrapper, so a
   * theme can react to the mode instead of fighting it. Always emitted (never
   * `undefined`) so `[data-feed-anchor='top']` is a usable selector too.
   */
  dataAnchor: FeedAnchor
  /**
   * Whether the newest message sits at the END of the rendered list. Follows
   * the order axis only: `true` unless the order is inverted.
   */
  newestAtEnd: boolean
  /**
   * Where the `.scroll-anchor` sentinel belongs. It must sit next to the newest
   * message, which is the START of the list when the order is inverted.
   */
  sentinelPosition: 'start' | 'end'
  /**
   * Whether the auto-scroll `scrollIntoView` should run at all when the feed
   * is SHORTER than its container (`contentOverflows === false` below).
   *
   * A bottom-anchored short feed is already resting against the edge the newest
   * message is on, so scrolling it is a no-op at best and a jitter source at
   * worst. Top-anchored feeds keep today's unconditional behaviour.
   */
  autoScrollWhenShort: boolean
}

/**
 * Map the two orthogonal settings onto the classes, the sentinel placement and
 * the auto-scroll decision. Pure: no DOM, no React, no `window`.
 *
 * @param anchor which canvas edge the stack rests on
 * @param invertMessageOrder whether the newest message renders first
 */
export function resolveFeedAnchorLayout(
  anchor: FeedAnchor,
  invertMessageOrder: boolean
): FeedAnchorLayout {
  const bottom = anchor === 'bottom'
  return {
    anchor,
    wrapperClass: bottom ? 'flex flex-col' : '',
    bodyClass: bottom ? 'mt-auto' : '',
    dataAnchor: anchor,
    newestAtEnd: !invertMessageOrder,
    sentinelPosition: invertMessageOrder ? 'start' : 'end',
    autoScrollWhenShort: !bottom,
  }
}

/**
 * Should the auto-scroll effect call `scrollIntoView` this tick?
 *
 * Split out from the layout so the effect can pass the measured overflow state
 * and stay testable. When the content overflows, every mode scrolls: the
 * bottom-anchored auto margin has collapsed to `0` and the feed is once again a
 * plain scrolling list whose newest row must be brought into view.
 */
export function shouldAutoScroll(layout: FeedAnchorLayout, contentOverflows: boolean): boolean {
  return contentOverflows || layout.autoScrollWhenShort
}

/**
 * Apply the ORDER axis to a message list. Returns the array in render order,
 * newest last unless the order is inverted. Never mutates the input.
 */
export function orderMessages<T>(messages: readonly T[], invertMessageOrder: boolean): T[] {
  return invertMessageOrder ? [...messages].reverse() : [...messages]
}
