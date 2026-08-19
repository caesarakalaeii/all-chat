import { test, expect, type Page } from '@playwright/test'

/**
 * The ORDER axis (`display_settings.invert_message_order`) on the REAL live
 * overlay, driven over a real WebSocket — combined with the ANCHOR axis
 * (`feed_anchor`, #729), which is where it fell apart.
 *
 * `overlay-feed-anchor.spec.ts` covers the static geometry of the four
 * combinations with hand-built fixtures. It cannot see any of the bugs below,
 * because all of them only exist once messages ARRIVE ONE AT A TIME:
 *
 * 1. The row key was `${message.id}-${index}` — the index into the RENDERED
 *    list. Inverting the order makes every arrival a prepend, so every index
 *    (and therefore every key) changed at once and React remounted the entire
 *    feed, replaying every row's entry animation on every message. The same
 *    thing happened without inversion once `max_messages` or the fade timer
 *    started dropping the head.
 * 2. The `.scroll-anchor` sentinel was not invisible: `@layer visual-customizer`
 *    in events.css dressed it as a chat bubble (an `!important` inside a layer
 *    outranks the unlayered `!important` guard in globals.css), so ~36px of
 *    padded ghost sat between the newest message and the edge it was anchored
 *    to.
 * 3. Entry animations are direction-dependent and all of them assumed the new
 *    row lands at the BOTTOM of the stack. Inverted, it lands at the top and
 *    slid up out from under its neighbour.
 * 4. A row animating past the list's bottom edge counts toward the document's
 *    SCROLLABLE height, so a bottom-anchored feed that fits reported an
 *    overflow for the length of the animation and auto-scrolled itself.
 *
 * These assertions are all "does the feed hold still and stay where it was
 * told", which is what an OBS scene actually needs.
 */

const OVERLAY_ID = 'feed-order-overlay'
const WS_GLOB = '**/ws/overlay/**'
const VIEWPORT = { width: 600, height: 400 }
/** `min-h-screen p-4` on the wrapper — the canvas is the viewport minus this. */
const WRAPPER_PADDING = 16

const connectedFrame = JSON.stringify({ type: 'connected', data: { overlay_id: OVERLAY_ID } })

const chatFrame = (id: string, text: string) =>
  JSON.stringify({
    type: 'chat_message',
    data: {
      id,
      overlay_id: OVERLAY_ID,
      platform: 'kick',
      channel_id: 'c1',
      channel_name: 'chan',
      user: { id: 'u1', username: 'tester', display_name: 'Tester', badges: [] },
      message: { text, emotes: [] },
      timestamp: '2026-08-19T12:00:00.000Z',
      metadata: {},
    },
  })

interface Options {
  invert?: boolean
  anchor?: 'top' | 'bottom'
  /** Entry animation preset; omitted means the built-in fallback. */
  animation?: string
  /** `display_settings.max_messages`; the default is deliberately generous. */
  maxMessages?: number
}

/**
 * Wait for every running CSS animation to finish. Entry animations translate a
 * row, and `getBoundingClientRect` includes transforms — measuring geometry
 * mid-flight reads the animation, not the layout.
 */
async function animationsSettled(page: Page) {
  await page.waitForFunction(
    () => document.getAnimations().every((a) => a.playState !== 'running'),
    undefined,
    { timeout: 5000 }
  )
}

/**
 * Boot the real `/overlay/[id]` with a mocked public config and a socket this
 * test drives. Returns a `send` that pushes one chat frame.
 */
async function boot(page: Page, opts: Options = {}) {
  const sockets: Array<{ send: (s: string) => void }> = []
  await page.routeWebSocket(WS_GLOB, (ws) => {
    sockets.push(ws)
    ws.send(connectedFrame)
  })
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/config`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        display_settings: {
          invert_message_order: opts.invert === true,
          feed_anchor: opts.anchor ?? 'top',
          disable_message_fade: true,
          max_messages: opts.maxMessages ?? 50,
        },
        visual_settings: opts.animation ? { messageAnimation: opts.animation } : {},
        custom_css: '',
      }),
    })
  )
  await page.setViewportSize(VIEWPORT)
  await page.goto(`/overlay/${OVERLAY_ID}`)
  // The socket and the public config are fetched independently, and the wrapper
  // renders with the DEFAULT anchor until the config lands — so waiting on the
  // attribute alone would race a config whose anchor happens to be the default.
  await expect.poll(() => sockets.length, { timeout: 15_000 }).toBeGreaterThan(0)
  await expect(page.locator('[data-feed-anchor]')).toHaveAttribute(
    'data-feed-anchor',
    opts.anchor ?? 'top'
  )

  return async (id: string, text: string) => {
    sockets[sockets.length - 1].send(chatFrame(id, text))
    await expect(page.locator(`[data-message-id="${id}"]`)).toBeVisible()
  }
}

/** Message ids in render order, top of the feed first. */
async function renderedOrder(page: Page): Promise<string[]> {
  return page
    .locator('[data-message-id]')
    .evaluateAll((els) => els.map((e) => (e as HTMLElement).dataset.messageId ?? ''))
}

/**
 * Stamp every rendered row so a later check can tell a surviving DOM node from
 * a remounted one. React never restores an attribute it does not know about, so
 * a missing stamp is proof the element was destroyed and rebuilt.
 */
async function stampRows(page: Page) {
  await page
    .locator('[data-message-id]')
    .evaluateAll((els) => els.forEach((e) => e.setAttribute('data-survived', '1')))
}

test.describe('Live overlay — invert_message_order with feed_anchor', () => {
  test('renders newest-first and exposes the order as its own data hook', async ({ page }) => {
    const send = await boot(page, { invert: true, anchor: 'bottom' })
    await send('m1', 'first')
    await send('m2', 'second')
    await send('m3', 'third')

    expect(await renderedOrder(page)).toEqual(['m3', 'm2', 'm1'])
    // Two independent axes, two independent hooks — conflating them made #728.
    const wrapper = page.locator('[data-feed-anchor]')
    await expect(wrapper).toHaveAttribute('data-feed-anchor', 'bottom')
    await expect(wrapper).toHaveAttribute('data-feed-order', 'newest-first')
  })

  test('a new message leaves every existing row mounted', async ({ page }) => {
    // The regression: an inverted feed prepends, which shifted every index and
    // therefore every `${id}-${index}` key, so React rebuilt the whole list.
    const send = await boot(page, { invert: true, anchor: 'bottom' })
    await send('m1', 'first')
    await send('m2', 'second')
    await stampRows(page)

    await send('m3', 'third')

    // The two older rows must be the SAME DOM nodes, and only the new row new.
    await expect(page.locator('[data-message-id="m1"]')).toHaveAttribute('data-survived', '1')
    await expect(page.locator('[data-message-id="m2"]')).toHaveAttribute('data-survived', '1')
    await expect(page.locator('[data-message-id="m3"]')).not.toHaveAttribute('data-survived', '1')
  })

  test('a dropped head leaves the surviving rows mounted (fade / max_messages path)', async ({
    page,
  }) => {
    // Same defect, reachable with the order NOT inverted: `slice(1)` (fade) and
    // `slice(-max)` (cap) both shift every index that follows. A 2-message cap
    // makes the third arrival drop the head.
    const send = await boot(page, { maxMessages: 2 })
    await send('m1', 'first')
    await send('m2', 'second')
    await stampRows(page)

    await send('m3', 'third')

    expect(await renderedOrder(page)).toEqual(['m2', 'm3'])
    await expect(page.locator('[data-message-id="m2"]')).toHaveAttribute('data-survived', '1')
  })

  test('the sentinel adds no box, so the feed rests flush on the anchored edge', async ({
    page,
  }) => {
    // `@layer visual-customizer` used to give `.scroll-anchor` the chat bubble's
    // 12px padding plus a message gap: ~36px of dead space glued to the newest
    // message, which a bottom-anchored feed shows as "it will not touch the edge".
    for (const invert of [false, true]) {
      const send = await boot(page, { invert, anchor: 'bottom' })
      await send('m1', 'first')
      await send('m2', 'second')
      await animationsSettled(page)

      const box = await page.evaluate(() => {
        const sentinel = document.querySelector('.scroll-anchor') as HTMLElement
        const cs = getComputedStyle(sentinel)
        const rows = [...document.querySelectorAll('[data-message-id]')] as HTMLElement[]
        const list = document.querySelector('.overlay-live-body') as HTMLElement
        return {
          height: sentinel.getBoundingClientRect().height,
          padding: cs.padding,
          marginTop: cs.marginTop,
          marginBottom: cs.marginBottom,
          listBottom: list.getBoundingClientRect().bottom,
          lastRowBottom: rows[rows.length - 1].getBoundingClientRect().bottom,
        }
      })

      expect(box.height, 'sentinel must occupy no space').toBe(0)
      expect(box.padding).toBe('0px')
      expect(box.marginTop).toBe('0px')
      expect(box.marginBottom).toBe('0px')
      // Nothing between the last row and the end of the list…
      expect(box.lastRowBottom).toBeCloseTo(box.listBottom, 0)
      // …and the list itself sits on the canvas edge it was anchored to.
      expect(box.listBottom).toBeCloseTo(VIEWPORT.height - WRAPPER_PADDING, 0)
    }
  })

  test('entry animations enter from the end the newest row lands on', async ({ page }) => {
    // `msg-anim-bounce` starts at translateY(110%): correct when the new row is
    // appended at the bottom, backwards when inversion puts it on top.
    const firstFrameOffset = async (invert: boolean) => {
      const send = await boot(page, { invert, anchor: 'bottom', animation: 'bounce' })
      await send('m1', 'first')
      await send('m2', 'second')
      return page.evaluate(() => {
        const el = document.querySelector('[data-message-id="m2"]') as HTMLElement
        const anim = el.getAnimations()[0]
        anim.pause()
        anim.currentTime = 0
        // translateY lands in the last cell of the 2D matrix.
        return new DOMMatrixReadOnly(getComputedStyle(el).transform).m42
      })
    }

    expect(await firstFrameOffset(false), 'newest-last enters from below').toBeGreaterThan(0)
    expect(await firstFrameOffset(true), 'newest-first enters from above').toBeLessThan(0)
  })

  test('a bottom-anchored feed that fits does not scroll itself while a row animates', async ({
    page,
  }) => {
    // A row translated past the list's bottom edge counts toward the document's
    // scrollable height. Measuring THAT made a feed that fits look overflowing
    // mid-animation, so the page smooth-scrolled and snapped back on every
    // message. The predicate has to use layout height.
    const send = await boot(page, { invert: false, anchor: 'bottom', animation: 'bounce' })
    await send('m1', 'first')

    const scrollYs: number[] = []
    for (let i = 0; i < 6; i++) {
      scrollYs.push(await page.evaluate(() => window.scrollY))
      await page.waitForTimeout(60)
    }
    await send('m2', 'second')
    for (let i = 0; i < 8; i++) {
      scrollYs.push(await page.evaluate(() => window.scrollY))
      await page.waitForTimeout(60)
    }

    expect(Math.max(...scrollYs), 'the page must never scroll while the feed fits').toBe(0)
  })
})
