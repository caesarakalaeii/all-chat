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

// @vitest-environment jsdom
import { describe, expect, it, vi } from 'vitest'

import {
  ALERT_SANDBOX,
  ALERT_SANDBOX_BOOTSTRAP,
  alertBridgeMessage,
  buildAlertSrcdoc,
  hasCustomCode,
} from '@/components/overlays/alertSandbox'
import type { AlertFrame, AlertProfile } from '@/lib/types/alert'

const profile: AlertProfile = {
  event_type: 'bits',
  text_template: '{username} cheered {amount}!',
  sound_url: '',
  image_url: 'https://static.klipy.com/a.gif',
  duration_ms: 3000,
  volume: 80,
  enabled: true,
  sort_after: 0,
  text: 'Viewer cheered 100 bits!',
  custom_html: '<h1 data-alert-text></h1>',
  custom_css: 'h1 { color: gold }',
  custom_js: 'onAlert(function (m) { window.got = m })',
}

const frame: AlertFrame & { profile: AlertProfile } = {
  alert_id: 'a1',
  overlay_id: 'o1',
  platform: 'twitch',
  event_type: 'bits',
  event_data: {
    type: 'bits',
    value: { amount: 100, currency: 'bits', display_text: '100 bits' },
  },
  user: { id: 'u1', name: 'Viewer', avatar_url: '' },
  message: 'take my bits',
  occurred_at: '2026-10-16T12:00:00Z',
  profile,
}

/** Run the bootstrap in this window, as the frame would before the streamer's code. */
function bootRuntime() {
  new Function(ALERT_SANDBOX_BOOTSTRAP)()
  return window as unknown as {
    onAlert: (cb: (m: unknown) => void) => void
    allchat: { fill: (t: string, e: unknown) => string }
  }
}

describe('alert sandbox', () => {
  it('grants scripts and nothing else', () => {
    // Security contract (issue #954): any additional token, above all the
    // same-origin one, breaks the isolation the custom-code tier relies on.
    expect(ALERT_SANDBOX).toBe('allow-scripts')
  })

  it('treats only non-blank custom code as custom', () => {
    expect(hasCustomCode(profile)).toBe(true)
    expect(hasCustomCode({ ...profile, custom_html: ' ', custom_css: '', custom_js: '\n' })).toBe(
      false
    )
    expect(
      hasCustomCode({ ...profile, custom_html: undefined, custom_css: 'p{}', custom_js: undefined })
    ).toBe(true)
    expect(hasCustomCode(undefined)).toBe(false)
  })

  it('posts { type, profile, event } with the envelope minus its profile', () => {
    const msg = alertBridgeMessage(frame)
    expect(Object.keys(msg).sort()).toEqual(['event', 'profile', 'type'])
    expect(msg.type).toBe('alert')
    expect(msg.profile).toBe(profile)
    expect(msg.event).not.toHaveProperty('profile')
    expect(msg.event).toMatchObject({
      alert_id: 'a1',
      user: { name: 'Viewer' },
      message: 'take my bits',
    })
  })

  it('builds a document with the bootstrap ahead of the streamer code', () => {
    const doc = buildAlertSrcdoc(profile)
    const bootstrapAt = doc.indexOf('window.allchat')
    expect(bootstrapAt).toBeGreaterThan(0)
    expect(doc.indexOf('<h1 data-alert-text>')).toBeGreaterThan(bootstrapAt)
    expect(doc.indexOf('onAlert(function (m)')).toBeGreaterThan(bootstrapAt)
    expect(doc).toContain('h1 { color: gold }')
  })

  it('keeps a closing tag inside custom code from ending its element', () => {
    const doc = buildAlertSrcdoc({
      ...profile,
      custom_js: 'x = "</script><b>"',
      custom_css: '/* </style> */',
    })
    expect(doc).toContain('x = "<\\/script><b>"')
    expect(doc).toContain('/* <\\/style> */')
  })

  it('falls back to the default layout when only CSS is custom', () => {
    const doc = buildAlertSrcdoc({ ...profile, custom_html: '', custom_js: '' })
    expect(doc).toContain('data-alert-text')
    expect(doc).toContain('data-alert-image')
  })

  it('the runtime fills placeholders like the gateway and keeps missing ones literal', () => {
    const rt = bootRuntime()
    const event = alertBridgeMessage(frame).event
    expect(rt.allchat.fill('{username} cheered {amount} ({count}): {message}', event)).toBe(
      'Viewer cheered 100 bits (100): take my bits'
    )
    expect(
      rt.allchat.fill('{username} {amount}', { ...event, event_data: { type: 'follow' } })
    ).toBe('Viewer {amount}')
  })

  it('the runtime delivers parent messages to onAlert and binds the text', () => {
    document.body.innerHTML = '<h1 data-alert-text></h1><img data-alert-image alt="">'
    const rt = bootRuntime()
    const got = vi.fn()
    rt.onAlert(got)
    const msg = alertBridgeMessage(frame)

    window.dispatchEvent(new MessageEvent('message', { data: msg, source: window.parent }))

    expect(got).toHaveBeenCalledWith(msg)
    expect(document.querySelector('h1')?.textContent).toBe('Viewer cheered 100 bits!')
    expect(document.querySelector('img')?.getAttribute('src')).toBe(
      'https://static.klipy.com/a.gif'
    )
    // A handler registered after the alert arrived still sees it.
    const late = vi.fn()
    rt.onAlert(late)
    expect(late).toHaveBeenCalledWith(msg)
  })

  it('the runtime ignores messages from anything but the parent', () => {
    const rt = bootRuntime()
    const got = vi.fn()
    rt.onAlert(got)
    window.dispatchEvent(
      new MessageEvent('message', { data: alertBridgeMessage(frame), source: null })
    )
    expect(got).not.toHaveBeenCalled()
  })
})
