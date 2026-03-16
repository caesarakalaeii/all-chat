import { apiClient } from './client'
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
    window.location.href = data.bot_invite_url
  }
}

export async function updateSourceConfig(
  overlayId: string,
  sourceId: string,
  config: DiscordSourceConfig,
): Promise<void> {
  return apiClient.patch<void>(`/api/v1/overlays/${overlayId}/sources/${sourceId}`, { config })
}
