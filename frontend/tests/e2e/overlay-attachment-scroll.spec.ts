import { test, expect, type Page } from '@playwright/test'

/**
 * A chat attachment that finishes loading AFTER its row arrived must still end
 * up in frame on the live overlay.
 *
 * Reported from Discord: "when a person sends an attachment ... you have to
 * chat again to show the attachment". The overlay scrolls to the newest row
 * when `messages` changes, but at that moment the `<img>` has not loaded and
 * takes no space. The image then grows the row by up to 256px, nothing scrolls
 * again, and `body { overflow: hidden }` in OBS clips it below the canvas: the
 * row reads as a bare "username:" until the next message triggers a scroll.
 *
 * Driven over a real WebSocket against the real live overlay and the editor's
 * embed preview (same feed, own scroll container), with the image response held
 * back so it lands well after the row rendered.
 */

const OVERLAY_ID = 'attachment-scroll-overlay'
const WS_GLOB = '**/ws/overlay/**'
const IMAGE_URL = 'https://media.discordapp.net/attachments/1/2/late.png'
const IMAGE_DELAY_MS = 1000
const VIEWPORT = { width: 600, height: 400 }

/** A 200x200 solid PNG, so the loaded image has a real, known size. */
async function pngBytes(page: Page): Promise<Buffer> {
  const base64 = await page.evaluate(() => {
    const canvas = document.createElement('canvas')
    canvas.width = 200
    canvas.height = 200
    const ctx = canvas.getContext('2d')
    if (ctx) {
      ctx.fillStyle = '#e11d48'
      ctx.fillRect(0, 0, 200, 200)
    }
    return canvas.toDataURL('image/png').split(',')[1]
  })
  return Buffer.from(base64, 'base64')
}

const frame = (id: string, text: string, attachments?: unknown[]) =>
  JSON.stringify({
    type: 'chat_message',
    data: {
      id,
      overlay_id: OVERLAY_ID,
      platform: 'discord',
      channel_id: 'c1',
      channel_name: 'chan',
      user: { id: 'u1', username: 'tester', display_name: 'Tester', badges: [] },
      message: { text, emotes: [], ...(attachments ? { attachments } : {}) },
      timestamp: '2026-11-15T12:00:00.000Z',
      metadata: {},
    },
  })

const SURFACES = [
  {
    name: 'live overlay',
    path: `/overlay/${OVERLAY_ID}`,
    configUrl: `**/api/v1/overlays/public/${OVERLAY_ID}/config`,
  },
  {
    name: 'embed preview',
    path: `/overlays/${OVERLAY_ID}/preview/embed`,
    configUrl: `**/api/v1/overlays/${OVERLAY_ID}/config`,
  },
]

for (const surface of SURFACES) {
  test(`${surface.name}: an attachment that loads late is scrolled into frame without another message`, async ({
    page,
  }) => {
    await page.setViewportSize(VIEWPORT)
    await page.goto('about:blank')
    const png = await pngBytes(page)

    const sockets: Array<{ send: (s: string) => void }> = []
    await page.routeWebSocket(WS_GLOB, (ws) => {
      sockets.push(ws)
      ws.send(JSON.stringify({ type: 'connected', data: { overlay_id: OVERLAY_ID } }))
    })
    // The embed preview sits under /overlays, whose layout sends a signed-out
    // visitor home; the live overlay never asks.
    await page.route('**/api/v1/auth/me', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          id: 'u1',
          username: 'streamer',
          display_name: 'Streamer',
          onboarding_completed_at: '2026-01-01T00:00:00Z',
        }),
      })
    )
    await page.route(surface.configUrl, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          display_settings: { disable_message_fade: true, max_messages: 50 },
          visual_settings: {},
          custom_css: '',
        }),
      })
    )
    await page.route(IMAGE_URL, async (route) => {
      await new Promise((resolve) => setTimeout(resolve, IMAGE_DELAY_MS))
      await route.fulfill({ status: 200, contentType: 'image/png', body: png })
    })

    await page.goto(surface.path)
    await expect.poll(() => sockets.length, { timeout: 15_000 }).toBeGreaterThan(0)
    const send = (data: string) => sockets[sockets.length - 1].send(data)

    // Fill the canvas so the feed overflows and auto-scroll is in play.
    for (let i = 0; i < 12; i++) {
      send(frame(`m${i}`, `filler message ${i}`))
    }
    await expect(page.getByText('filler message 11', { exact: true })).toBeVisible()

    send(
      frame('att', '', [
        { type: 'image', url: IMAGE_URL, content_type: 'image/png', width: 200, height: 200 },
      ])
    )

    const image = page.locator(`img[src="${IMAGE_URL}"]`)
    await expect(image).toHaveJSProperty('complete', true, { timeout: 10_000 })
    await expect.poll(() => image.evaluate((img: HTMLImageElement) => img.naturalHeight)).toBe(200)

    // The whole image must sit inside the canvas once any scroll has settled.
    await expect
      .poll(
        () =>
          image.evaluate((img) => {
            const r = img.getBoundingClientRect()
            return r.height > 0 && r.top >= 0 && r.bottom <= window.innerHeight
          }),
        { timeout: 5_000 }
      )
      .toBe(true)
  })
}
