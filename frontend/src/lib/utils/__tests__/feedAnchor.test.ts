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

import { describe, it, expect } from 'vitest'

import {
  DEFAULT_FEED_ANCHOR,
  orderMessages,
  parseFeedAnchor,
  resolveFeedAnchorLayout,
  shouldAutoScroll,
  type FeedAnchor,
} from '../feedAnchor'

describe('parseFeedAnchor', () => {
  it("defaults to 'top' so existing overlays do not move", () => {
    expect(DEFAULT_FEED_ANCHOR).toBe('top')
    expect(parseFeedAnchor(undefined)).toBe('top')
    expect(parseFeedAnchor(null)).toBe('top')
  })

  it('accepts the two documented values', () => {
    expect(parseFeedAnchor('top')).toBe('top')
    expect(parseFeedAnchor('bottom')).toBe('bottom')
  })

  it('rejects anything else that survives the untyped backend map', () => {
    for (const junk of ['Bottom', 'end', '', 0, 1, true, {}, [], NaN]) {
      expect(parseFeedAnchor(junk)).toBe('top')
    }
  })
})

/** All four combinations of the two orthogonal axes. */
const COMBOS: { anchor: FeedAnchor; invert: boolean }[] = [
  { anchor: 'top', invert: false },
  { anchor: 'top', invert: true },
  { anchor: 'bottom', invert: false },
  { anchor: 'bottom', invert: true },
]

describe('resolveFeedAnchorLayout — all four combinations', () => {
  it('top + not inverted: today’s default, no extra layout at all', () => {
    const l = resolveFeedAnchorLayout('top', false)
    expect(l.anchor).toBe('top')
    expect(l.wrapperClass).toBe('')
    expect(l.bodyClass).toBe('')
    expect(l.scrollBodyClass).toBe('h-full')
    expect(l.dataAnchor).toBe('top')
    expect(l.newestAtEnd).toBe(true)
    expect(l.sentinelPosition).toBe('end')
    expect(l.autoScrollWhenShort).toBe(true)
  })

  it('top + inverted: order flips, layout does not', () => {
    const l = resolveFeedAnchorLayout('top', true)
    expect(l.wrapperClass).toBe('')
    expect(l.bodyClass).toBe('')
    expect(l.newestAtEnd).toBe(false)
    expect(l.sentinelPosition).toBe('start')
    expect(l.autoScrollWhenShort).toBe(true)
  })

  it('bottom + not inverted: the Twitch-like arrangement that was requested', () => {
    const l = resolveFeedAnchorLayout('bottom', false)
    expect(l.anchor).toBe('bottom')
    expect(l.wrapperClass).toContain('flex')
    expect(l.wrapperClass).toContain('flex-col')
    expect(l.bodyClass).toBe('mt-auto')
    // The scroll container must NOT keep `h-full`: it would fill the flex line
    // and leave the auto margin nothing to resolve against.
    expect(l.scrollBodyClass).toBe('mt-auto max-h-full')
    expect(l.scrollBodyClass).not.toContain('h-full ')
    expect(l.dataAnchor).toBe('bottom')
    expect(l.newestAtEnd).toBe(true)
    expect(l.sentinelPosition).toBe('end')
    expect(l.autoScrollWhenShort).toBe(false)
  })

  it('bottom + inverted: newest hugs the bottom edge but renders first', () => {
    const l = resolveFeedAnchorLayout('bottom', true)
    expect(l.wrapperClass).toContain('flex-col')
    expect(l.bodyClass).toBe('mt-auto')
    expect(l.newestAtEnd).toBe(false)
    expect(l.sentinelPosition).toBe('start')
    expect(l.autoScrollWhenShort).toBe(false)
  })

  it('never uses justify-end or a reversed column (overflow would be unreachable)', () => {
    for (const { anchor, invert } of COMBOS) {
      const l = resolveFeedAnchorLayout(anchor, invert)
      expect(l.wrapperClass).not.toContain('justify-end')
      expect(l.wrapperClass).not.toContain('flex-col-reverse')
      expect(l.bodyClass).not.toContain('justify-end')
      expect(l.scrollBodyClass).not.toContain('justify-end')
      expect(l.scrollBodyClass).not.toContain('reverse')
    }
  })

  it('puts the auto margin on the list, never on its children', () => {
    // `.overlay-live-body > * + * { margin-top: ... !important }` in events.css
    // and `.scroll-anchor { margin: 0 !important }` in globals.css would both
    // beat a child-level rule, so the margin must live on the body element.
    for (const { anchor, invert } of COMBOS) {
      const l = resolveFeedAnchorLayout(anchor, invert)
      expect(l.wrapperClass).not.toContain('mt-auto')
      if (anchor === 'bottom') expect(l.bodyClass).toBe('mt-auto')
    }
  })

  it('keeps the two axes orthogonal', () => {
    for (const anchor of ['top', 'bottom'] as const) {
      const plain = resolveFeedAnchorLayout(anchor, false)
      const inverted = resolveFeedAnchorLayout(anchor, true)
      // Order must not perturb the anchor's layout output…
      expect(plain.wrapperClass).toBe(inverted.wrapperClass)
      expect(plain.bodyClass).toBe(inverted.bodyClass)
      expect(plain.scrollBodyClass).toBe(inverted.scrollBodyClass)
      // …and the anchor must not perturb the order's output.
      expect(plain.newestAtEnd).toBe(true)
      expect(inverted.newestAtEnd).toBe(false)
    }
  })

  it('always emits a data hook so themes can select on either mode', () => {
    for (const { anchor, invert } of COMBOS) {
      expect(resolveFeedAnchorLayout(anchor, invert).dataAnchor).toBe(anchor)
    }
  })

  it('keeps the sentinel adjacent to the newest message', () => {
    for (const { anchor, invert } of COMBOS) {
      const l = resolveFeedAnchorLayout(anchor, invert)
      expect(l.sentinelPosition).toBe(l.newestAtEnd ? 'end' : 'start')
    }
  })
})

describe('shouldAutoScroll', () => {
  it('scrolls in every mode once the content overflows', () => {
    for (const { anchor, invert } of COMBOS) {
      expect(shouldAutoScroll(resolveFeedAnchorLayout(anchor, invert), true)).toBe(true)
    }
  })

  it('skips the pointless scroll on a short bottom-anchored feed', () => {
    expect(shouldAutoScroll(resolveFeedAnchorLayout('bottom', false), false)).toBe(false)
    expect(shouldAutoScroll(resolveFeedAnchorLayout('bottom', true), false)).toBe(false)
  })

  it('preserves today’s unconditional scroll for top-anchored feeds', () => {
    expect(shouldAutoScroll(resolveFeedAnchorLayout('top', false), false)).toBe(true)
    expect(shouldAutoScroll(resolveFeedAnchorLayout('top', true), false)).toBe(true)
  })
})

describe('orderMessages', () => {
  const msgs = ['oldest', 'middle', 'newest'] as const

  it('leaves the append order alone when not inverted', () => {
    expect(orderMessages(msgs, false)).toEqual(['oldest', 'middle', 'newest'])
  })

  it('reverses when inverted', () => {
    expect(orderMessages(msgs, true)).toEqual(['newest', 'middle', 'oldest'])
  })

  it('never mutates the caller’s array', () => {
    const input = [...msgs]
    orderMessages(input, true)
    expect(input).toEqual(['oldest', 'middle', 'newest'])
  })

  it('handles the empty feed', () => {
    expect(orderMessages([], false)).toEqual([])
    expect(orderMessages([], true)).toEqual([])
  })
})
