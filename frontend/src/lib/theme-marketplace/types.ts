/**
 * Theme Marketplace Type Definitions
 */

export interface ThemeMetadata {
  name: string
  description: string
  tags: string[]
  author?: string
  version?: string
  updated?: string
}

export interface Theme extends ThemeMetadata {
  id: string
  filename: string
  css: string
}

export interface ThemeCacheData {
  timestamp: number
  ttl: number
  themes: Theme[]
}

export interface ChatMessagePreview {
  id: string
  overlay_id: string
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  channel_id: string
  channel_name: string
  user: {
    id: string
    username: string
    display_name: string
    avatar_url: string
    badges: Array<{
      name: string
      version: string
      icon_url: string
    }>
    color: string
  }
  message: {
    text: string
    emotes: Array<{
      code: string
      provider: string
      url: string
      positions: number[][]
    }>
  }
  timestamp: string
  metadata: Record<string, any>
}
