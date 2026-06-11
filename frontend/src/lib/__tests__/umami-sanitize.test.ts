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
import { sanitizeUrl, umamiBeforeSend } from '@/lib/umami-sanitize'

const OVERLAY_ID = '3f2504e0-4f89-41d3-9a0c-0305e82c3301'
const SOURCE_ID = '11111111-2222-3333-4444-555555555555'

describe('sanitizeUrl', () => {
  it('leaves a plain path untouched', () => {
    expect(sanitizeUrl('/dashboard')).toBe('/dashboard')
  })

  it('collapses an overlay UUID in the path to :id', () => {
    expect(sanitizeUrl(`/overlays/${OVERLAY_ID}`)).toBe('/overlays/:id')
  })

  it('collapses UUIDs in nested overlay sub-routes', () => {
    expect(sanitizeUrl(`/overlays/${OVERLAY_ID}/preview/embed`)).toBe('/overlays/:id/preview/embed')
  })

  it('strips the token-bearing hash from an OAuth callback', () => {
    expect(sanitizeUrl('/auth/callback#access_token=abc.def.ghi&refresh_token=xyz')).toBe(
      '/auth/callback'
    )
  })

  it('drops sensitive query params (code, streamer)', () => {
    expect(sanitizeUrl('/chat/auth-success?code=secret123&streamer=somechannel')).toBe(
      '/chat/auth-success'
    )
  })

  it('preserves utm_* campaign params while dropping sensitive ones', () => {
    expect(sanitizeUrl('/?utm_source=reddit&utm_campaign=launch&code=secret')).toBe(
      '/?utm_source=reddit&utm_campaign=launch'
    )
  })

  it('collapses the overlay path and drops the source_added UUID together', () => {
    expect(sanitizeUrl(`/overlays/${OVERLAY_ID}?source_added=${SOURCE_ID}`)).toBe('/overlays/:id')
  })

  it('collapses UUIDs inside an absolute referrer URL', () => {
    expect(sanitizeUrl(`https://allch.at/overlays/${OVERLAY_ID}`)).toBe(
      'https://allch.at/overlays/:id'
    )
  })

  it('keeps non-sensitive query params on an external referrer', () => {
    expect(sanitizeUrl('https://www.google.com/search?q=all-chat')).toBe(
      'https://www.google.com/search?q=all-chat'
    )
  })
})

describe('umamiBeforeSend', () => {
  it('sanitises url and referrer in place and returns the same payload object', () => {
    const payload = {
      website: 'w',
      url: `/overlays/${OVERLAY_ID}?source_added=${SOURCE_ID}`,
      referrer: '/auth/callback#access_token=leak',
    }
    const out = umamiBeforeSend('event', payload)

    // Returning the same (mutated) object is what tells Umami to transmit it.
    expect(out).toBe(payload)
    expect(out.url).toBe('/overlays/:id')
    expect(out.referrer).toBe('/auth/callback')
  })

  it('is a no-op for payloads without url or referrer', () => {
    expect(umamiBeforeSend('event', { website: 'w', title: 'Home' })).toEqual({
      website: 'w',
      title: 'Home',
    })
  })
})
