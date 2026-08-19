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
   * (`@layer visual-customizer`) would win over any child rule. Note that
   * `.scroll-anchor`'s own `margin: 0 !important` in globals.css does NOT win
   * against it — for IMPORTANT declarations a cascade layer outranks unlayered
   * styles — which is why events.css excludes the sentinel by selector.
   *
   * When the content is taller than the wrapper the auto margin resolves to
   * `0`, so a busy chat behaves exactly as it does today — the mode is a no-op
   * rather than an overflow hazard. That is why there is no
   * `justify-content: flex-end` here.
   */
  bodyClass: string
  /**
   * Body classes for a surface where the LIST ITSELF is the scroll container
   * (the editor's embed preview: `overlay-preview-body ... overflow-y-auto`),
   * rather than the page.
   *
   * Such a container is normally `h-full`, which would fill the flex line and
   * leave the auto margin nothing to resolve against. Bottom anchoring swaps
   * that for `max-h-full`, so the list shrinks to its content and gets pushed
   * to the bottom edge — and once the content is taller than the frame the cap
   * bites, the auto margin collapses to `0`, and `overflow-y-auto` scrolls
   * exactly as it does today. `justify-end` would instead make the overflowing
   * start edge unreachable, which is why it is not used.
   */
  scrollBodyClass: string
  /**
   * Stable theme hook: the value for `data-feed-anchor` on the wrapper, so a
   * theme can react to the mode instead of fighting it. Always emitted (never
   * `undefined`) so `[data-feed-anchor='top']` is a usable selector too.
   */
  dataAnchor: FeedAnchor
  /**
   * Stable hook for the ORDER axis: the value for `data-feed-order` on the
   * wrapper. Sibling of `dataAnchor`, and deliberately a separate attribute —
   * conflating the two axes is what made #728.
   *
   * globals.css keys the direction-dependent entry animations off this: a
   * message that lands at the START of the stack has to enter from the top
   * edge, not the bottom. It flips `--msg-enter-dir` / `--msg-enter-origin`,
   * which the `msg-anim-*` keyframes read, so the `.msg-anim-*` rules stay
   * single unlayered class selectors that a theme can still override wholesale.
   */
  dataOrder: 'newest-first' | 'newest-last'
  /**
   * Entry animation used when `visual_settings.messageAnimation` is unset.
   *
   * The stock `slide-in-from-bottom-2` assumes the new row appears at the
   * BOTTOM of the stack. When the order is inverted it appears at the top, and
   * sliding it up from below makes it emerge from underneath the row beneath
   * it. The `msg-anim-*` presets solve this in CSS (see `dataOrder`); this
   * fallback is a pair of utility classes, so it is picked here instead.
   */
  defaultEntryAnimationClass: string
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
   * A short bottom-anchored feed has nothing to scroll — the list is sized to
   * its content and the auto margin has already parked it on the anchored edge
   * — so the call is a no-op at best and a jitter source at worst. Top-anchored
   * feeds keep today's unconditional behaviour.
   *
   * This follows the ANCHOR only. It is tempting to reason "…because the feed
   * is already resting against the edge the newest message is on", but that is
   * false when the order is inverted: the newest row then sits at the far edge
   * from the anchor. The conclusion survives anyway, because a feed that does
   * not overflow cannot be scrolled in either direction.
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
    scrollBodyClass: bottom ? 'mt-auto max-h-full' : 'h-full',
    dataAnchor: anchor,
    dataOrder: invertMessageOrder ? 'newest-first' : 'newest-last',
    defaultEntryAnimationClass: invertMessageOrder
      ? 'animate-in duration-300 slide-in-from-top-2'
      : 'animate-in duration-300 slide-in-from-bottom-2',
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
 *
 * @param contentOverflows must mean "the feed no longer fits the canvas it is
 *   anchored in", i.e. the SCROLL CONTAINER overflows — not merely "the list is
 *   taller than the viewport". Those differ by the wrapper's padding, and on the
 *   live overlay (`min-h-screen p-4`) getting it wrong left a ~32px band where
 *   a bottom-anchored feed clipped its newest row instead of scrolling to it.
 *   Measure the wrapper's LAYOUT height against the viewport (live overlay) or
 *   the scroll container's `scrollHeight > clientHeight` (embed preview, where
 *   the list itself scrolls).
 *
 *   Prefer a layout measurement (`offsetHeight`) over a scrollable one
 *   (`scrollHeight`) wherever the caller can: entry animations translate the new
 *   row outside the list, transformed descendants count toward scrollable
 *   overflow, and a bottom-anchored feed — whose list ends flush with the canvas
 *   — then reports a phantom overflow for the length of the animation. Pair it
 *   with `scrollIntoView({ block: 'nearest' })` so even a phantom `true` cannot
 *   move a feed that is already showing its newest row.
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
