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
