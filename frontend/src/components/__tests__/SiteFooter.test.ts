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

import { describe, expect, it } from 'vitest'
import { showsSiteFooter } from '../SiteFooter'

describe('showsSiteFooter', () => {
  it('shows on app and marketing pages, so the Impressum is reachable everywhere', () => {
    for (const path of [
      '/',
      '/dashboard',
      '/settings',
      '/overlays/new',
      '/overlays/abc',
      '/overlays/abc/events',
      '/upgrade',
      '/legal/impressum',
      '/docs',
    ]) {
      expect(showsSiteFooter(path), path).toBe(true)
    }
  })

  it('never shows on surfaces OBS captures', () => {
    for (const path of [
      '/overlay',
      '/overlay/abc',
      '/overlay/abc/view',
      '/overlays/abc/preview',
      '/overlays/abc/preview/embed',
    ]) {
      expect(showsSiteFooter(path), path).toBe(false)
    }
  })

  it('stays off OAuth popups, redirect hops and dev harnesses', () => {
    for (const path of ['/chat/auth-success', '/auth/callback', '/dev/theme-contrast']) {
      expect(showsSiteFooter(path), path).toBe(false)
    }
  })
})
