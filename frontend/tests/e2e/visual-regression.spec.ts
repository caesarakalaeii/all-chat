/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
 * Pixel-baseline suite — the visual gate for app chrome.
 *
 * Why it exists: every other frontend gate inspects source (lint, tokens,
 * contrast) or the accessibility tree (axe). None of them can see a collapsed
 * layout, a 1200px-wide button, or a nav that stacks into oblivion at 375px —
 * the classes of UI fuckup that repeatedly reached production while all
 * checks were green. This suite renders the real app in a real browser and
 * compares pixels against committed baselines, so a layout regression fails
 * the PR instead of the beta.
 *
 * Scope: chrome routes only (landing, dashboard, settings, upgrade, docs),
 * plus landing, dashboard and settings again in the light theme (ADR-0087),
 * plus the support screens: the overlay editor (default, Sources, OBS help),
 * new overlay, chat commands (list and editor dialog) and the alerts editor.
 * The support bot attaches these to answers (send_image, ADR-0093), so they
 * double as user-facing guide images: keep them free of test-only chrome.
 * Overlay surfaces (src/app/overlay/**, embed, ThemePreview) are
 * user-themable broadcast art with their own contrast floor
 * (theme-contrast.spec.ts) and are deliberately NOT baselined here — they
 * would flake on every theme edit and bury real chrome regressions.
 *
 * Auth/mocking follows a11y.spec.ts: network-route mocks on /api/v1/*, never
 * the legacy localStorage auth store. Anchors are asserted BEFORE the
 * screenshot so a broken mock fails loudly on the anchor, not as a pixel diff
 * of an error page.
 *
 * Baselines: generated on CI (see .github/workflows/frontend-visual.yml,
 * dispatch with update_snapshots=true), because they must be produced by
 * the exact browser/font stack CI uses. A baseline from a dev host is a
 * baseline CI will reject. Local runs (npm run test:visual) are for the
 * agent dev loop; see docs/frontend/AGENT_UI_VERIFICATION.md.
 *
 * Updating a baseline is a reviewable act: run the dispatch, commit the new
 * PNGs in the same PR as the visual change that justifies them.
 */

import { test, expect, type Page } from '@playwright/test'

const USER = {
  id: '11111111-1111-1111-1111-111111111111',
  username: 'visual_smoke',
  display_name: 'Visual Smoke',
  auth_provider: 'twitch',
  is_admin: false,
  is_premium: false,
  is_beta_tester: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  onboarding_completed_at: '2026-01-02T00:00:00Z',
}

const OVERLAY = {
  id: '22222222-2222-2222-2222-222222222222',
  user_id: USER.id,
  name: 'Visual Test Overlay',
  description: '',
  is_active: true,
  is_public_for_viewers: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const SOURCES = [
  {
    id: 'source-1',
    overlay_id: OVERLAY.id,
    platform: 'twitch',
    channel_id: '123',
    channel_name: 'somechannel',
    is_active: true,
  },
]

// Baseline widths: desktop 1280 and mobile 375. These two catch nearly
// everything a single width misses (overflow, stacking, horizontal scroll).
const WIDTHS = [
  { label: '1280', width: 1280, height: 720 },
  { label: '375', width: 375, height: 667 },
] as const

/**
 * Register mocks for an authenticated session. Order matters: Playwright
 * matches routes last-registered-first, so the JSON-404 catch-all goes first
 * and specific endpoints override it (same contract as a11y.spec.ts).
 */
async function mockAuthedApi(page: Page, overrides: Record<string, unknown> = {}) {
  await page.route('**/api/v1/**', (route) =>
    route.fulfill({ status: 404, contentType: 'application/json', body: '{}' })
  )
  const json = (body: unknown) => ({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify(body),
  })
  await page.route('**/api/v1/auth/me', (route) => route.fulfill(json(USER)))
  await page.route('**/api/v1/payment/status', (route) =>
    route.fulfill(json({ connected: false, is_premium: false }))
  )
  for (const [pattern, body] of Object.entries(overrides)) {
    await page.route(pattern, (route) => route.fulfill(json(body)))
  }
}

/**
 * Deterministic 1x1 transparent PNG. Landing marquee emotes include
 * ANIMATED webps that loop forever — image-internal animation is invisible
 * to every CSS/reduced-motion gate, and the pixels never settle. The
 * marquee is decorative (aria-hidden), so its emote art is stubbed to this
 * for pixel determinism (see the landing test).
 */
const TRANSPARENT_PNG = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
  'base64'
)

/** Stub decorative emote artwork (animated webp) for pixel determinism. */
async function stubEmotes(page: Page) {
  await page.route('**/emotes/**', (route) =>
    route.fulfill({ status: 200, contentType: 'image/png', body: TRANSPARENT_PNG })
  )
}

// The Next dev-server indicator ("N" badge, "N Issues" pill) is off for test
// runs: playwright.config.ts starts the dev server with PLAYWRIGHT_DEV_SERVER=1 and
// next.config.js sets devIndicators: false. A style tag cannot do it: the overlay
// editor's preview is a sandboxed iframe of the app with its own indicator.

/**
 * Wait for the page's rendered pixels to stop moving. Real pages keep
 * settling seconds after `networkidle`: webfonts swap in late and reflow
 * measured containers (the landing theme showcase re-measures its slide
 * ~800ms after load; the docs page's code-font swap settles at ~3.5s).
 * scrollHeight alone misses font re-renders at constant layout, so this
 * compares actual screenshot bytes until two consecutive shots are
 * identical. toHaveScreenshot does the same dance internally, but with a
 * 5s budget that races a slow webfont; settling here first makes the
 * assertion's own stability check a formality. The hard cap keeps a page
 * that never settles failing for the right reason instead of hanging.
 */
async function waitForPixelSettle(page: Page, fullPage: boolean, capMs = 12000) {
  const deadline = Date.now() + capMs
  let last: Buffer<ArrayBufferLike> = Buffer.alloc(0)
  while (Date.now() < deadline) {
    const shot = await page.screenshot({ fullPage })
    if (Buffer.compare(last, shot) === 0) return
    last = shot
    await page.waitForTimeout(500)
  }
  throw new Error(
    `pixels never settled within ${capMs}ms — the page renders nondeterministically; fix the spec (stub the nondeterministic source), do not mask the diff`
  )
}

/**
 * Assert a pixel baseline at both widths. Anchor first (fail loudly on a
 * broken mock), then wait for webfonts and pixel settle — toHaveScreenshot
 * freezes CSS animations but a font swapping in mid-flight still diffed
 * the baseline.
 */
async function expectBaseline(
  page: Page,
  routeName: string,
  anchor: () => Promise<void>,
  options: { fullPage?: boolean } = {}
) {
  const fullPage = options.fullPage ?? false
  for (const viewport of WIDTHS) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height })
    await anchor()
    await page.evaluate(() => document.fonts.ready)
    await waitForPixelSettle(page, fullPage)
    await expect(page).toHaveScreenshot(`${routeName}-${viewport.label}.png`, { fullPage })
  }
}

/** Shared by both themes: everything that makes a shot deterministic. */
async function prepare(page: Page, theme: 'dark' | 'light') {
  // Must be set BEFORE navigation: the landing theme carousel reads
  // prefers-reduced-motion at mount and otherwise keeps auto-advancing,
  // changing the DOM between shots (same reason as a11y.spec.ts).
  await page.emulateMedia({ reducedMotion: 'reduce' })
  // Monaco loads its CSS from cdn.jsdelivr.net, which the dev CSP rejects;
  // the async failure flakes the shot. Block it outright.
  await page.route('https://cdn.jsdelivr.net/**', (route) => route.abort())
  // The cookie banner mounts 1s after load. Whether the shot lands before
  // or after that depends on runner speed, so mark it acknowledged before
  // the app boots. The site theme (ADR-0087) is a stored choice the same way.
  await page.addInitScript((light) => {
    localStorage.setItem('cookieBannerAcknowledged', 'true')
    if (light) localStorage.setItem('allchat_theme', 'light')
  }, theme === 'light')
}

// /settings: the language card probes its gate, and the open state is the steady one to
// baseline; the connected platforms card is baselined with a sign-in and a linked platform.
const SETTINGS_API = {
  '**/api/v1/ui-languages': { enabled: true },
  '**/api/v1/auth/me/connections': {
    connections: [
      { platform: 'twitch', accounts: ['smoke_channel'], is_login: true },
      { platform: 'youtube', accounts: ['UCsmokeChannel'], is_login: false },
    ],
  },
}

test.describe('visual baselines (chrome)', () => {
  test.beforeEach(async ({ page }) => prepare(page, 'dark'))

  test('landing', async ({ page }) => {
    // The hero marquee emotes include ANIMATED webps (catjam, clap, ...) that
    // loop forever inside the <img> — no CSS gate reaches image-internal
    // animation, so the pixels never stabilize. The marquee is decorative
    // texture (aria-hidden, opacity .75); stubbing the emotes keeps the
    // baseline deterministic without changing what the gate protects: the
    // landing layout, type and chrome.
    await stubEmotes(page)
    await page.goto('/')
    // Full page: the landing page's value is below the fold (feature cards,
    // platform strip); a viewport-only baseline would hide a collapsed tail.
    await expectBaseline(
      page,
      'landing',
      async () => {
        await expect(page.getByRole('main')).toBeVisible({ timeout: 20_000 })
      },
      { fullPage: true }
    )
  })

  test('docs', async ({ page }) => {
    await page.goto('/docs')
    await expectBaseline(
      page,
      'docs',
      async () => {
        await expect(page.getByRole('heading', { name: 'Streamer guide' })).toBeVisible({
          timeout: 20_000,
        })
      },
      { fullPage: true }
    )
  })

  test('dashboard with overlays', async ({ page }) => {
    await mockAuthedApi(page, {
      '**/api/v1/overlays': [OVERLAY],
      [`**/api/v1/overlays/${OVERLAY.id}/sources`]: SOURCES,
    })
    await page.goto('/dashboard')
    await expectBaseline(page, 'dashboard-populated', async () => {
      await expect(page.getByText(OVERLAY.name)).toBeVisible({ timeout: 20_000 })
    })
  })

  test('dashboard empty state', async ({ page }) => {
    await mockAuthedApi(page, { '**/api/v1/overlays': [] })
    await page.goto('/dashboard')
    await expectBaseline(page, 'dashboard-empty', async () => {
      await expect(page.getByText('No overlays yet')).toBeVisible({ timeout: 20_000 })
    })
  })

  test('settings', async ({ page }) => {
    await mockAuthedApi(page, SETTINGS_API)
    await page.goto('/settings')
    await expectBaseline(
      page,
      'settings',
      async () => {
        await expect(page.getByText(USER.username)).toBeVisible({ timeout: 20_000 })
      },
      { fullPage: true }
    )
  })

  test('upgrade', async ({ page }) => {
    await mockAuthedApi(page)
    await page.goto('/upgrade')
    await expectBaseline(
      page,
      'upgrade',
      async () => {
        await expect(page.getByRole('main')).toBeVisible({ timeout: 20_000 })
      },
      { fullPage: true }
    )
  })
})

// The light theme (ADR-0087) on the routes that cover its three surfaces: the
// lanes marketing page, a lanes product page and a token-styled page.
test.describe('visual baselines (light theme)', () => {
  test.beforeEach(async ({ page }) => prepare(page, 'light'))

  test('landing', async ({ page }) => {
    await stubEmotes(page)
    await page.goto('/')
    await expectBaseline(
      page,
      'light-landing',
      async () => {
        await expect(page.getByRole('main')).toBeVisible({ timeout: 20_000 })
      },
      { fullPage: true }
    )
  })

  test('dashboard empty state', async ({ page }) => {
    await mockAuthedApi(page, { '**/api/v1/overlays': [] })
    await page.goto('/dashboard')
    await expectBaseline(page, 'light-dashboard-empty', async () => {
      await expect(page.getByText('No overlays yet')).toBeVisible({ timeout: 20_000 })
    })
  })

  test('settings', async ({ page }) => {
    await mockAuthedApi(page, SETTINGS_API)
    await page.goto('/settings')
    await expectBaseline(
      page,
      'light-settings',
      async () => {
        await expect(page.getByText(USER.username)).toBeVisible({ timeout: 20_000 })
      },
      { fullPage: true }
    )
  })
})

// Screens the support bot shows users (`send_image`, services/support-bot/tools/image.go):
// the pages people ask "where is X" about. They are app chrome like the routes above,
// so the same rules apply: mocked API, anchor before the shot, pixels must settle.
const CONFIG = {
  id: 'config-1',
  overlay_id: OVERLAY.id,
  display_settings: {},
  filter_settings: {},
  visual_settings: {},
  enable_7tv: true,
  enable_bttv: false,
  enable_ffz: false,
  theme_id: null,
  custom_css: '',
}

const EDITOR_API = {
  [`**/api/v1/overlays/${OVERLAY.id}`]: OVERLAY,
  [`**/api/v1/overlays/${OVERLAY.id}/sources`]: SOURCES,
  [`**/api/v1/overlays/${OVERLAY.id}/config`]: CONFIG,
  // Unmocked, it 404s and the editor logs the error, which puts Next's dev "1 Issue"
  // pill into the shot.
  [`**/api/v1/overlays/${OVERLAY.id}/tts-config`]: { has_elevenlabs_config: false },
  '**/api/v1/shares/accepted': [],
  '**/api/v1/auth/guilds': [],
}

const COMMANDS_API = {
  '**/api/v1/engagement/chat-commands': {
    commands: [
      {
        id: 'cmd-1',
        kind: 'keyword',
        trigger: 'clip that',
        response: 'Clipped! {clip}',
        create_clip: true,
        min_role: 'subscriber',
        cooldown_seconds: 30,
        enabled: true,
        use_count: 3,
      },
    ],
    status: {
      is_premium: false,
      twitch_connected: true,
      twitch_can_send: true,
      twitch_can_clip: false,
      consent_overlay_id: OVERLAY.id,
    },
  },
}

/**
 * The editors' live preview opens an overlay websocket. Unanswered, it shows a
 * reconnecting state that keeps changing; a socket that says "connected" and stays
 * quiet is the steady state a user sees with no chat going.
 */
async function stubOverlaySocket(page: Page) {
  await page.context().routeWebSocket('**/ws/**', (ws) => {
    ws.send(JSON.stringify({ type: 'connected', data: { overlay_id: OVERLAY.id } }))
  })
}

async function openEditor(page: Page) {
  await mockAuthedApi(page, EDITOR_API)
  await stubOverlaySocket(page)
  await page.goto(`/overlays/${OVERLAY.id}`)
  await expect(page.getByText(OVERLAY.name).first()).toBeVisible({ timeout: 20_000 })
}

test.describe('visual baselines (support screens)', () => {
  test.beforeEach(async ({ page }) => prepare(page, 'dark'))

  test('overlay editor', async ({ page }) => {
    await openEditor(page)
    await expectBaseline(page, 'overlay-editor', async () => {
      await expect(page.getByText(OVERLAY.name).first()).toBeVisible()
    })
  })

  test('overlay editor sources', async ({ page }) => {
    await openEditor(page)
    await page
      .getByRole('navigation', { name: 'Overlay settings' })
      .getByRole('button', { name: 'Sources' })
      .click()
    await expectBaseline(page, 'overlay-editor-sources', async () => {
      // At 375 the panel stacks under the header; show the sources, not the header.
      const heading = page.getByRole('heading', { name: 'Sources', level: 2 })
      await heading.scrollIntoViewIfNeeded()
      await expect(page.getByText('somechannel').first()).toBeVisible({ timeout: 20_000 })
    })
  })

  // "How do I add this to OBS?": the step most new users never finish.
  test('overlay editor obs help', async ({ page }) => {
    await openEditor(page)
    await page.getByRole('button', { name: 'How do I add this to OBS?' }).click()
    await expectBaseline(page, 'overlay-editor-obs-help', async () => {
      await expect(page.getByRole('dialog')).toBeVisible({ timeout: 20_000 })
    })
  })

  test('new overlay', async ({ page }) => {
    await mockAuthedApi(page, { '**/api/v1/overlays': [OVERLAY] })
    await page.goto('/overlays/new')
    await expectBaseline(
      page,
      'overlay-new',
      async () => {
        await expect(page.getByRole('main')).toBeVisible({ timeout: 20_000 })
      },
      { fullPage: true }
    )
  })

  test('chat commands', async ({ page }) => {
    await mockAuthedApi(page, COMMANDS_API)
    await page.goto('/commands')
    await expectBaseline(
      page,
      'commands',
      async () => {
        await expect(page.getByRole('heading', { name: 'Chat commands', level: 1 })).toBeVisible({
          timeout: 20_000,
        })
      },
      { fullPage: true }
    )
  })

  test('chat command editor', async ({ page }) => {
    await mockAuthedApi(page, {
      ...COMMANDS_API,
      '**/api/v1/engagement/chat-commands/preview': { output: 'Hello Visual', errors: [] },
    })
    await page.goto('/commands')
    await page.getByRole('button', { name: 'Greeting' }).click()
    await expectBaseline(page, 'commands-editor', async () => {
      await expect(page.getByRole('dialog')).toBeVisible({ timeout: 20_000 })
    })
  })

  test('alerts editor', async ({ page }) => {
    const alerts = { ...OVERLAY, name: 'Alert box', overlay_type: 'alerts' }
    const profiles = ['all', 'subscription', 'bits'].map((event_type) => ({
      event_type,
      text_template: event_type === 'all' ? '' : '{username} just subscribed!',
      sound_url: '',
      image_url: '',
      duration_ms: 4000,
      volume: 100,
      enabled: true,
      sort_after: 0,
      custom_html: '',
      custom_css: '',
      custom_js: '',
    }))
    await mockAuthedApi(page, {
      [`**/api/v1/overlays/${OVERLAY.id}`]: alerts,
      [`**/api/v1/overlays/${OVERLAY.id}/sources`]: [],
      [`**/api/v1/overlays/${OVERLAY.id}/config`]: CONFIG,
      [`**/api/v1/overlays/${OVERLAY.id}/tts-config`]: { has_elevenlabs_config: false },
      [`**/api/v1/overlays/public/${OVERLAY.id}/config`]: {
        overlay_type: 'alerts',
        display_settings: {},
        sources: [],
      },
      [`**/api/v1/overlays/${OVERLAY.id}/alerts/profiles`]: { profiles, saved: [] },
    })
    await stubOverlaySocket(page)
    await page.goto(`/overlays/${OVERLAY.id}`)
    await expectBaseline(page, 'alerts-editor', async () => {
      await expect(page.getByLabel('Image or GIF')).toBeVisible({ timeout: 30_000 })
    })
  })
})
