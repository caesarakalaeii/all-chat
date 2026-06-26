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

import { apiClient } from './client'
import { safeExternalRedirect } from '../auth/redirect-allowlist'
import type { DiscordSourceConfig } from '@/lib/types/overlay'

export interface DiscordGuild {
  id: string
  user_id: string
  guild_id: string
  guild_name: string
  guild_icon: string | null
  connected_at: string
}

export interface ChannelSummary {
  id: string
  name: string
  position: number
}

export interface ChannelCategory {
  id: string
  name: string
  channels: ChannelSummary[]
}

export interface GuildChannelsResponse {
  categories: ChannelCategory[]
}

export async function getGuilds(): Promise<DiscordGuild[]> {
  return apiClient.get<DiscordGuild[]>('/api/v1/auth/guilds')
}

export async function getGuildChannels(guildId: string): Promise<GuildChannelsResponse> {
  return apiClient.get<GuildChannelsResponse>(`/api/v1/auth/guilds/${guildId}/channels`)
}

export async function disconnectGuild(guildId: string): Promise<void> {
  return apiClient.delete(`/api/v1/auth/guilds/${guildId}`)
}

export async function startDiscordOAuth(): Promise<void> {
  const data = await apiClient.get<{ bot_invite_url: string }>('/api/v1/auth/discord/connect')
  if (data.bot_invite_url) {
    safeExternalRedirect(data.bot_invite_url)
  }
}

// startDiscordModerationReinvite fetches the elevated bot invite URL (ADR-0017) and
// redirects to it. Re-authorizing on an existing guild upgrades the bot's permissions in
// place (MANAGE_MESSAGES / MODERATE_MEMBERS / BAN_MEMBERS) so the streamer can moderate.
// Unlike Twitch/Kick/YouTube this is a bot RE-INVITE, not an OAuth re-consent.
export async function startDiscordModerationReinvite(): Promise<void> {
  const data = await apiClient.get<{ bot_invite_url: string }>(
    '/api/v1/auth/discord/connect?moderation=true',
  )
  if (data.bot_invite_url) {
    safeExternalRedirect(data.bot_invite_url)
  }
}

export async function updateSourceConfig(
  overlayId: string,
  sourceId: string,
  config: DiscordSourceConfig,
): Promise<void> {
  return apiClient.patch<void>(`/api/v1/overlays/${overlayId}/sources/${sourceId}`, { config })
}
