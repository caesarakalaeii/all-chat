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

import { isAllowedExternalRedirect } from '../redirect-allowlist'

/**
 * Guards the redirect-allowlist against open-redirect bypasses (audit M1).
 *
 * The critical case: `/\evil.com` (slash + backslash) passes the old
 * `startsWith('/') && !startsWith('//')` guard, but browsers normalize `\` → `/`
 * and navigate to `//evil.com` (evil.com). The backslash guard must reject it.
 */
describe('isAllowedExternalRedirect', () => {
  it('rejects backslash-based open-redirect bypass', () => {
    expect(isAllowedExternalRedirect('/\\evil.com')).toBe(false)
  })

  it('allows safe relative paths', () => {
    expect(isAllowedExternalRedirect('/dashboard')).toBe(true)
  })

  it('rejects protocol-relative URLs', () => {
    expect(isAllowedExternalRedirect('//evil.com')).toBe(false)
  })

  it('allows allowlisted external hosts', () => {
    expect(isAllowedExternalRedirect('https://twitch.tv/path')).toBe(true)
  })
})
