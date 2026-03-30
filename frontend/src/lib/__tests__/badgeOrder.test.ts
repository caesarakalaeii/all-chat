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
