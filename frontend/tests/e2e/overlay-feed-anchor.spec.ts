import { test, expect } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { resolveFeedAnchorLayout, type FeedAnchor } from '../../src/lib/utils/feedAnchor'

/**
 * `display_settings.feed_anchor` must glue the feed to the edge the user picked.
 *
 * Reported as "I'd like the messages to start at the bottom and add upwards".
 * The overlay only ever exposed the ORDER axis (`invert_message_order`), so the
 * stack always rested on the TOP edge and grew downward, with blank space at
 * the bottom. A full canvas hid this — `scrollIntoView` kept the newest row in
 * frame — but a quiet chat visibly hung off the top edge.
 *
 * The technique under test is deliberately narrow: a flex-column WRAPPER plus
 * `margin-top: auto` on `.overlay-live-body` itself. The margin must not land
 * on the list's children, because `.overlay-live-body > * + *` (events.css,
 * `@layer visual-customizer`) and `.scroll-anchor` (globals.css) are both
 * `!important` and would win. So this drives the REAL events.css and the REAL
 * globals.css read from source rather than restating them.
 */

const REPO = join(__dirname, '..', '..', '..')
const EVENTS_CSS = readFileSync(join(REPO, 'frontend/src/styles/events.css'), 'utf8')

/**
 * The `.scroll-anchor` block from globals.css. Extracted rather than inlining
 * the whole file, which is a Tailwind v4 source with `@import`/`@theme` that a
 * bare browser cannot parse.
 */
const SCROLL_ANCHOR_CSS = (() => {
  const css = readFileSync(join(REPO, 'frontend/src/app/globals.css'), 'utf8')
  const start = css.indexOf('.space-y-3 > div.scroll-anchor')
  const end = css.indexOf('}', css.indexOf('animation: none !important', start))
  return css.slice(start, end + 1)
})()

/** The Tailwind utilities the overlay wrapper and list actually depend on. */
const TAILWIND = `
  *, ::before, ::after { box-sizing: border-box; margin: 0; padding: 0; }
  html, body { line-height: 1.5; font-family: system-ui, sans-serif; height: 100%; }
  .min-h-screen { min-height: 100vh; }
  .w-full { width: 100%; }
  .h-full { height: 100%; }
  .max-h-full { max-height: 100%; }
  .h-screen { height: 100vh; }
  .p-4 { padding: 1rem; }
  .p-3 { padding: 0.75rem; }
  .flex { display: flex; }
  .flex-col { flex-direction: column; }
  .mt-auto { margin-top: auto; }
  .overflow-hidden { overflow: hidden; }
  .overflow-y-auto { overflow-y: auto; }
  .rounded-lg { border-radius: 0.5rem; }
  .relative { position: relative; }
  .space-y-3 > :not(:last-child) { margin-block-end: 0.75rem; }
`

const ROW = `<div class="chat-message rounded-lg p-3" data-row>Welcome to the overlay!</div>`
const SENTINEL = `<div class="scroll-anchor" data-sentinel></div>`

/** Mirrors the live overlay's wrapper + list (src/app/overlay/[id]/page.tsx). */
function liveFixture(anchor: FeedAnchor, invert: boolean, rows: number): string {
  const layout = resolveFeedAnchorLayout(anchor, invert)
  const body = [
    layout.sentinelPosition === 'start' ? SENTINEL : '',
    ROW.repeat(rows),
    layout.sentinelPosition === 'end' ? SENTINEL : '',
  ].join('')
  return `<!doctype html><html><head><meta charset="utf-8">
<style>@layer base, design-system, marketplace-themes, user-overrides;</style>
<style>${TAILWIND}</style>
<style>${EVENTS_CSS}</style>
<style>${SCROLL_ANCHOR_CSS}</style>
</head><body>
  <div class="min-h-screen w-full p-4 ${layout.wrapperClass}" data-feed-anchor="${layout.dataAnchor}" data-wrapper>
    <div class="overlay-live-body space-y-3 ${layout.bodyClass}" data-body>${body}</div>
  </div>
</body></html>`
}

/** Mirrors the embed preview, where the LIST is the scroll container. */
function previewFixture(anchor: FeedAnchor, invert: boolean, rows: number): string {
  const layout = resolveFeedAnchorLayout(anchor, invert)
  return `<!doctype html><html><head><meta charset="utf-8">
<style>@layer base, design-system, marketplace-themes, user-overrides;</style>
<style>${TAILWIND}</style>
<style>${EVENTS_CSS}</style>
<style>${SCROLL_ANCHOR_CSS}</style>
</head><body>
  <div class="overlay-preview-root relative h-screen overflow-hidden ${layout.wrapperClass}" data-feed-anchor="${layout.dataAnchor}" data-wrapper>
    <div class="overlay-preview-body space-y-3 overflow-y-auto p-4 ${layout.scrollBodyClass}" data-body>${ROW.repeat(rows)}</div>
  </div>
</body></html>`
}

interface Box {
  bodyTop: number
  bodyBottom: number
  wrapperTop: number
  wrapperBottom: number
  firstRowTop: number
  sentinelMarginTop: string
}

async function measure(page: import('@playwright/test').Page): Promise<Box> {
  return page.evaluate(() => {
    const q = (s: string) => document.querySelector(s) as HTMLElement
    const body = q('[data-body]').getBoundingClientRect()
    const wrapper = q('[data-wrapper]').getBoundingClientRect()
    const sentinel = q('[data-sentinel]')
    return {
      bodyTop: +body.top.toFixed(1),
      bodyBottom: +body.bottom.toFixed(1),
      wrapperTop: +wrapper.top.toFixed(1),
      wrapperBottom: +wrapper.bottom.toFixed(1),
      firstRowTop: +q('[data-row]').getBoundingClientRect().top.toFixed(1),
      sentinelMarginTop: sentinel ? getComputedStyle(sentinel).marginTop : 'none',
    }
  })
}

test.describe('feed anchor — live overlay', () => {
  for (const invert of [false, true]) {
    const order = invert ? 'inverted' : 'normal'

    test(`top anchor (default), ${order} order: short feed still hugs the top edge`, async ({
      page,
    }) => {
      await page.setContent(liveFixture('top', invert, 2))
      const m = await measure(page)
      // 16px of p-4 below the wrapper's top edge — today's behaviour, unchanged.
      expect(m.bodyTop - m.wrapperTop).toBeCloseTo(16, 0)
      expect(m.wrapperBottom - m.bodyBottom).toBeGreaterThan(100)
    })

    test(`bottom anchor, ${order} order: short feed rests on the bottom edge`, async ({ page }) => {
      await page.setContent(liveFixture('bottom', invert, 2))
      const m = await measure(page)
      // The blank space has moved to the TOP, which is the whole request.
      expect(m.wrapperBottom - m.bodyBottom).toBeCloseTo(16, 0)
      expect(m.bodyTop - m.wrapperTop).toBeGreaterThan(100)
    })

    test(`bottom anchor, ${order} order: a new message pushes older ones upward`, async ({
      page,
    }) => {
      await page.setContent(liveFixture('bottom', invert, 1))
      const before = await measure(page)
      await page.setContent(liveFixture('bottom', invert, 2))
      const after = await measure(page)
      // Growing the feed moves its top edge UP while the bottom edge holds.
      expect(after.bodyTop).toBeLessThan(before.bodyTop)
      expect(after.bodyBottom).toBeCloseTo(before.bodyBottom, 0)
    })

    test(`bottom anchor, ${order} order: overflow collapses the auto margin to a no-op`, async ({
      page,
    }) => {
      await page.setContent(liveFixture('bottom', invert, 60))
      const m = await measure(page)
      // Taller than the viewport: `margin-top: auto` resolves to 0 and a busy
      // chat behaves exactly as it does today, starting at the wrapper's top.
      expect(m.bodyTop - m.wrapperTop).toBeCloseTo(16, 0)
    })

    test(`bottom anchor, ${order} order: the !important child rules are not fought`, async ({
      page,
    }) => {
      await page.setContent(liveFixture('bottom', invert, 2))
      const m = await measure(page)
      // The auto margin is on the list, so the sentinel keeps its forced 0 and
      // the first row still sits flush with the list's own content box.
      if (m.sentinelMarginTop !== 'none') expect(m.sentinelMarginTop).toBe('0px')
      expect(m.firstRowTop).toBeCloseTo(m.bodyTop, 0)
    })
  }
})

test.describe('feed anchor — embed preview parity', () => {
  test('bottom anchor rests the preview list on the bottom edge', async ({ page }) => {
    await page.setContent(previewFixture('bottom', false, 2))
    const m = await measure(page)
    expect(m.wrapperBottom - m.bodyBottom).toBeCloseTo(0, 0)
    expect(m.bodyTop - m.wrapperTop).toBeGreaterThan(100)
  })

  test('bottom anchor keeps the preview scrollbar working when the list overflows', async ({
    page,
  }) => {
    await page.setContent(previewFixture('bottom', false, 60))
    const scrollable = await page.evaluate(() => {
      const body = document.querySelector('[data-body]') as HTMLElement
      body.scrollTop = 1e6
      return {
        overflows: body.scrollHeight > body.clientHeight,
        scrolledToBottom: body.scrollTop > 0,
        fitsFrame: body.getBoundingClientRect().height <= window.innerHeight + 1,
      }
    })
    // `max-h-full` must cap the list at the frame, or the scrollbar is dead and
    // the overflow spills into the `overflow-hidden` root instead.
    expect(scrollable.overflows).toBe(true)
    expect(scrollable.scrolledToBottom).toBe(true)
    expect(scrollable.fitsFrame).toBe(true)
  })

  test('top anchor leaves the preview exactly as it was', async ({ page }) => {
    await page.setContent(previewFixture('top', false, 2))
    const m = await measure(page)
    expect(m.bodyTop).toBeCloseTo(m.wrapperTop, 0)
    expect(m.bodyBottom).toBeCloseTo(m.wrapperBottom, 0)
  })
})

test.describe('feed anchor — theme hook', () => {
  for (const anchor of ['top', 'bottom'] as const) {
    test(`data-feed-anchor="${anchor}" is exposed for themes to select on`, async ({ page }) => {
      await page.setContent(liveFixture(anchor, false, 2))
      await expect(page.locator(`[data-feed-anchor="${anchor}"]`)).toHaveCount(1)
    })
  }
})
