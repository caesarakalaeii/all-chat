import { test, expect } from '@playwright/test'

/**
 * Every platform's icon on the REAL live overlay (`/overlay/[id]`), driven over
 * a real WebSocket.
 *
 * The overlay and the preview embed used to carry their own copies of the
 * platform icon switch, and neither copy had Discord: a Discord message showed
 * its icon in the monitor (which used the shared PlatformGlyph) and none on
 * stream. Both now render through PlatformGlyph; this pins the overlay to it.
 */

const OVERLAY_ID = 'platform-icon-overlay'

const PLATFORMS = [
  'twitch',
  'youtube',
  'kick',
  'tiktok',
  'discord',
  'owncast',
  'goodgame',
  'picarto',
  'facebook',
  'instagram',
  'altstream',
]

const chatFrame = (platform: string) =>
  JSON.stringify({
    type: 'chat_message',
    data: {
      id: `m-${platform}`,
      overlay_id: OVERLAY_ID,
      platform,
      channel_id: 'c1',
      channel_name: 'chan',
      user: { id: 'u1', username: 'tester', display_name: 'Tester', badges: [] },
      message: { text: `hello from ${platform}`, emotes: [] },
      timestamp: '2026-11-11T12:00:00.000Z',
      metadata: {},
    },
  })

test('the live overlay shows a platform icon for every platform', async ({ page }) => {
  const sockets: Array<{ send: (s: string) => void }> = []
  await page.routeWebSocket('**/ws/overlay/**', (ws) => {
    sockets.push(ws)
    ws.send(JSON.stringify({ type: 'connected', data: { overlay_id: OVERLAY_ID } }))
  })
  await page.route(`**/api/v1/overlays/public/${OVERLAY_ID}/config`, (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        display_settings: {
          platform_badge_style: 'icon',
          show_platform_badge: true,
          disable_message_fade: true,
          max_messages: 50,
        },
        visual_settings: {},
        custom_css: '',
      }),
    })
  )
  await page.goto(`/overlay/${OVERLAY_ID}`)
  await expect.poll(() => sockets.length, { timeout: 15_000 }).toBeGreaterThan(0)

  for (const platform of PLATFORMS) {
    sockets[sockets.length - 1].send(chatFrame(platform))
    const row = page.locator(`[data-message-id="m-${platform}"]`)
    await expect(row).toBeVisible()
    await expect(row.locator('.platform-badge-icon svg'), platform).toBeVisible()
  }
})
