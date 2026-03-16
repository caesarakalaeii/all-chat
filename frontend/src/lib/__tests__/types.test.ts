import { describe, it, expectTypeOf } from 'vitest'
import type { ChatSource, DiscordSourceConfig } from '@/lib/types/overlay'

describe('overlay types', () => {
  it('ChatSource.platform accepts discord', () => {
    // Compile-time check: if 'discord' is not in the union this file fails tsc
    const platform: ChatSource['platform'] = 'discord'
    expectTypeOf(platform).toBeString()
  })

  it('DiscordSourceConfig has required fields', () => {
    const config: DiscordSourceConfig = {
      guild_id: 'g',
      inbound_channel_id: 'c',
      relay_enabled: false,
      relay_channel_id: null,
    }
    expectTypeOf(config.guild_id).toBeString()
    expectTypeOf(config.relay_enabled).toBeBoolean()
  })
})
