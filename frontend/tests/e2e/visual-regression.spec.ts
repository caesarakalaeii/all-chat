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
 * Scope: chrome routes only (landing, dashboard, settings, upgrade, docs).
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

// The Next dev-server indicator (bottom-left "N Issues" pill) appears
// whenever an unmocked request errors in the dev proxy, which depends on
// timing, not on the page under test. It is dev-only chrome, so it is
// hidden in every shot instead of being baselined.
const HIDE_DEV_CHROME = 'nextjs-portal { display: none !important; }'

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
  await page.addStyleTag({ content: HIDE_DEV_CHROME })
  for (const viewport of WIDTHS) {
    await page.setViewportSize({ width: viewport.width, height: viewport.height })
    await anchor()
    await page.evaluate(() => document.fonts.ready)
    await waitForPixelSettle(page, fullPage)
    await expect(page).toHaveScreenshot(`${routeName}-${viewport.label}.png`, { fullPage })
  }
}

test.describe('visual baselines (chrome)', () => {
  test.beforeEach(async ({ page }) => {
    // Must be set BEFORE navigation: the landing theme carousel reads
    // prefers-reduced-motion at mount and otherwise keeps auto-advancing,
    // changing the DOM between shots (same reason as a11y.spec.ts).
    await page.emulateMedia({ reducedMotion: 'reduce' })
    // Monaco loads its CSS from cdn.jsdelivr.net, which the dev CSP rejects;
    // the async failure flakes the shot. Block it outright.
    await page.route('https://cdn.jsdelivr.net/**', (route) => route.abort())
    // The cookie banner mounts 1s after load. Whether the shot lands before
    // or after that depends on runner speed, so mark it acknowledged before
    // the app boots.
    await page.addInitScript(() => localStorage.setItem('cookieBannerAcknowledged', 'true'))
  })

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
    await mockAuthedApi(page)
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
