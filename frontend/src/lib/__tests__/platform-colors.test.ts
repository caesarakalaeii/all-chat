import { describe, it, expect } from 'vitest'
import { PLATFORM_COLORS } from '@/lib/platform-colors'

describe('PLATFORM_COLORS', () => {
  it('twitch text class is text-twitch', () => {
    expect(PLATFORM_COLORS.twitch.text).toBe('text-twitch')
  })

  it('twitch bg class is bg-twitch', () => {
    expect(PLATFORM_COLORS.twitch.bg).toBe('bg-twitch')
  })

  it('youtube text class is text-youtube', () => {
    expect(PLATFORM_COLORS.youtube.text).toBe('text-youtube')
  })

  it('youtube bg class is bg-youtube', () => {
    expect(PLATFORM_COLORS.youtube.bg).toBe('bg-youtube')
  })

  it('kick text class is text-kick', () => {
    expect(PLATFORM_COLORS.kick.text).toBe('text-kick')
  })

  it('kick bg class is bg-kick', () => {
    expect(PLATFORM_COLORS.kick.bg).toBe('bg-kick')
  })

  it('tiktok text class is text-tiktok', () => {
    expect(PLATFORM_COLORS.tiktok.text).toBe('text-tiktok')
  })

  it('tiktok bg class is bg-tiktok', () => {
    expect(PLATFORM_COLORS.tiktok.bg).toBe('bg-tiktok')
  })

  it('system text class is text-text-sub', () => {
    expect(PLATFORM_COLORS.system.text).toBe('text-text-sub')
  })

  it('all keys are exactly: twitch, youtube, kick, tiktok, system, discord', () => {
    const keys = Object.keys(PLATFORM_COLORS).sort()
    expect(keys).toEqual(['discord', 'kick', 'system', 'tiktok', 'twitch', 'youtube'])
  })

  it('discord text class is text-discord', () => {
    expect(PLATFORM_COLORS.discord.text).toBe('text-discord')
  })

  it('discord bg class is bg-discord', () => {
    expect(PLATFORM_COLORS.discord.bg).toBe('bg-discord')
  })

  it('all keys include discord', () => {
    expect(Object.keys(PLATFORM_COLORS)).toContain('discord')
  })
})
