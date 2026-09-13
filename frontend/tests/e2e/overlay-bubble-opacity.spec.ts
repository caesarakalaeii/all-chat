import { test, expect, type Page } from '@playwright/test'
import { visualSettingsToCss } from '../../src/lib/utils/visual-settings-to-css'
import type { VisualSettings } from '../../src/lib/types/visual-settings'

/**
 * Bubble opacity must survive the theme cascade (ADR-0050).
 *
 * The opacity of a customizer color rides in the color value itself
 * (`#rrggbbaa`) precisely because a bundled theme paints the bubble from
 * `background: var(--chat-bubble-bg-color, …) !important`, which outranks the
 * GUI layer's normal-weight rule. Only a real engine can confirm that an
 * 8-digit hex survives a custom property and that the `!important` theme rule
 * beats the layered GUI rule — jsdom computes neither.
 *
 * The fixture drives the *real* module (visualSettingsToCss, which now emits
 * the flat bubble fill as a rule inside @layer visual-customizer) so the
 * assertion covers the shipped pipeline, not a restatement of it.
 */

/** Bubble background rule as bundled themes write it (minimal-icon, cyberpunk, …). */
const THEME_CSS = `
  .chat-message {
    background: var(--chat-bubble-bg-color, rgba(0, 0, 0, 0.85)) !important;
  }
`

/** Mirror the live overlay's feed markup so the generated rule's selector applies. */
function fixture(settings: Partial<VisualSettings>, themed: boolean): string {
  const css = visualSettingsToCss(settings)
  return `<!doctype html>
<html>
<head><meta charset="utf-8">
  <style>@layer base, design-system, marketplace-themes, visual-customizer, user-overrides;</style>
  <style>${css}</style>
  ${themed ? `<style>${THEME_CSS}</style>` : ''}
  <style>/* Tailwind default the customizer has to override */
    .chat-message { background-color: rgba(15, 23, 42, 0.9); }
  </style>
</head>
<body>
  <div class="overlay-preview-body">
    <div data-testid="bubble" class="chat-message">A chat message.</div>
  </div>
</body>
</html>`
}

async function bubbleBackground(
  page: Page,
  settings: Partial<VisualSettings>,
  themed: boolean
): Promise<string> {
  await page.setContent(fixture(settings, themed))
  return page.evaluate(
    () => getComputedStyle(document.querySelector('[data-testid="bubble"]')!).backgroundColor
  )
}

test.describe('chat bubble background opacity', () => {
  test('a themed bubble honours the alpha channel of the bubble color', async ({ page }) => {
    // The reported case: a Minimal-theme variant, "how do I make the bubbles
    // transparent?". With the opacity in a sibling field this stayed opaque,
    // because the theme's !important rule reads only the color variable.
    expect(await bubbleBackground(page, { bubbleBgColor: '#1a1a2e00' }, true)).toBe(
      'rgba(26, 26, 46, 0)'
    )
    // Partial alpha too — the engine serializes the 0x80 channel as 0.5.
    expect(await bubbleBackground(page, { bubbleBgColor: '#1a1a2e80' }, true)).toBe(
      'rgba(26, 26, 46, 0.5)'
    )
  })

  test('an untouched bubble color still paints fully opaque under a theme', async ({ page }) => {
    expect(await bubbleBackground(page, { bubbleBgColor: '#1a1a2e' }, true)).toBe('rgb(26, 26, 46)')
  })

  test('without a theme the GUI rule carries the same alpha', async ({ page }) => {
    expect(await bubbleBackground(page, { bubbleBgColor: '#1a1a2e00' }, false)).toBe(
      'rgba(26, 26, 46, 0)'
    )
  })

  test('settings saved with the legacy sibling opacity still render', async ({ page }) => {
    // Pre-ADR-0050 rows keep working: the GUI rule folds the sibling opacity
    // into the hex alpha channel on emission.
    expect(
      await bubbleBackground(page, { bubbleBgColor: '#1a1a2e', bubbleBgOpacity: '0.85' }, false)
    ).toBe('rgba(26, 26, 46, 0.85)')
  })
})
