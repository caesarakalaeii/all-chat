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
