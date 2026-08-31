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

/**
 * Copy lock for the strings shared across surfaces. See
 * __tests__/dashboard.test.ts for why the copy is pinned here rather than
 * through a rendered-output diff.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('shared platform names', () => {
  it('names every platform the UI labels', () => {
    // Two surfaces read these: the moderator roster, which delegates to Twitch,
    // YouTube, Kick and Discord, and the bubble colour picker, which colours
    // rows from all five. They live in common.* rather than in either namespace
    // because neither surface owns them.
    expect(t('common.platforms.twitch')).toBe('Twitch')
    expect(t('common.platforms.youtube')).toBe('YouTube')
    expect(t('common.platforms.kick')).toBe('Kick')
    expect(t('common.platforms.tiktok')).toBe('TikTok')
    expect(t('common.platforms.discord')).toBe('Discord')
  })
})

describe('Patreon subscription copy', () => {
  it('keeps the strings /settings/premium and /settings/viewer/premium both render', () => {
    // Both pages drive the same Patreon connect flow with byte-identical copy.
    // Only the strings that differ between them stay in settings.*.
    expect(t('common.patreon.heading')).toBe('Patreon')
    expect(t('common.patreon.connect')).toBe('Connect Patreon')
    expect(t('common.patreon.connecting')).toBe('Redirecting…')
    expect(t('common.patreon.notAPatron')).toBe('Not a patron yet?')
    expect(t('common.patreon.subscribe')).toBe('Subscribe on Patreon')
    expect(t('common.patreon.subscriptionRow')).toBe('Subscription')
    expect(t('common.patreon.renewsRow')).toBe('Renews')
    expect(t('common.patreon.active')).toBe('Active')
    expect(t('common.patreon.inactive')).toBe('Inactive')
    expect(t('common.patreon.disconnect')).toBe('Disconnect Patreon')
    expect(t('common.patreon.disconnectTitle')).toBe('Disconnect Patreon?')
    expect(t('common.patreon.disconnectCancel')).toBe('Cancel')
    expect(t('common.patreon.disconnectConfirm')).toBe('Yes, disconnect')
  })

  it('keeps the four shared pledge statuses', () => {
    // The fifth, "expired", differs per page ("Below premium tier" vs "Below
    // viewer tier"), so it stays in each page's own namespace.
    expect(t('common.patreon.statusActive')).toBe('Active')
    expect(t('common.patreon.statusDeclined')).toBe('Payment declined')
    expect(t('common.patreon.statusFormer')).toBe('Ended')
    expect(t('common.patreon.statusNotSubscribed')).toBe('Not subscribed')
  })
})

describe('shared sound preset names', () => {
  it('names the three soundPlayer presets', () => {
    // Two surfaces read these: the overlay editor's on-stream notification
    // sounds and the monitor view's private activity sound. Casing rules are
    // language-specific, so the display name cannot be derived from the stored
    // lowercase value by capitalising it.
    expect(t('common.soundPresets.chime')).toBe('Chime')
    expect(t('common.soundPresets.pop')).toBe('Pop')
    expect(t('common.soundPresets.ping')).toBe('Ping')
  })
})

describe('shared wordmark', () => {
  it('spells the product wordmark once for every nav that renders it', () => {
    // Four surfaces render it: the marketing header, the app nav, the admin rail
    // and the admin top nav. It is lowercase deliberately — the wordmark is set
    // in lowercase everywhere, unlike the 'All-Chat' of prose and aria labels.
    expect(t('common.brand.wordmark')).toBe('all-chat')
  })
})
