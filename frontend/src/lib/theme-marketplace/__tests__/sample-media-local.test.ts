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
 * The sample chat renders on the homepage for every visitor, before anyone has
 * signed in or connected a platform. Loading its avatars or badges from a third
 * party would hand every visitor's IP address to that host, which the privacy
 * policy does not disclose. So every sample image is a same-origin file.
 */

import { existsSync } from 'node:fs'
import path from 'node:path'

import { describe, expect, it } from 'vitest'

import { SAMPLE_PREVIEW_MESSAGES } from '../constants'

const publicDir = path.join(__dirname, '../../../../public')

function sampleImageUrls(): string[] {
  return SAMPLE_PREVIEW_MESSAGES.flatMap((msg) => [
    msg.user.avatar_url,
    ...(msg.user.badges ?? []).map((badge) => badge.icon_url),
  ]).filter((url): url is string => Boolean(url))
}

describe('sample preview messages', () => {
  it('load every avatar and badge from the All-Chat origin', () => {
    for (const url of sampleImageUrls()) {
      expect(url).toMatch(/^\/sample\//)
    }
  })

  it('point at files that exist in public/', () => {
    for (const url of sampleImageUrls()) {
      expect(existsSync(path.join(publicDir, url))).toBe(true)
    }
  })
})
