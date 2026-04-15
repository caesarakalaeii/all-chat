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
import { sortBadges } from '../badgeOrder'
import type { Badge } from '../types/message'

const makeBadge = (name: string): Badge => ({ name, version: '1', icon_url: '' })

describe('sortBadges', () => {
  it('allchat sorts before moderator', () => {
    const result = sortBadges([makeBadge('moderator'), makeBadge('allchat')])
    expect(result[0].name).toBe('allchat')
  })

  it('allchat-premium sorts before moderator', () => {
    const result = sortBadges([makeBadge('moderator'), makeBadge('allchat-premium')])
    expect(result[0].name).toBe('allchat-premium')
  })

  it('allchat sorts before allchat-premium', () => {
    const result = sortBadges([makeBadge('allchat-premium'), makeBadge('allchat')])
    expect(result[0].name).toBe('allchat')
  })

  it('all-chat badges sort before subscriber badges', () => {
    const result = sortBadges([makeBadge('subscriber'), makeBadge('allchat')])
    expect(result[0].name).toBe('allchat')
  })

  it('combined: [allchat, allchat-premium, vip] order from arbitrary input', () => {
    const result = sortBadges([makeBadge('vip'), makeBadge('allchat-premium'), makeBadge('allchat')])
    expect(result.map((b) => b.name)).toEqual(['allchat', 'allchat-premium', 'vip'])
  })
})
