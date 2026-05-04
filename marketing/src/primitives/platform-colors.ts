import type { Platform } from './types';

export const PLATFORM_HEX: Record<Platform, string> = {
  twitch: '#a37bff',
  youtube: '#ff4444',
  kick: '#53fc18',
  tiktok: '#69c9d0',
  discord: '#5865f2',
  system: '#9ca3af',
};

export const PLATFORM_LABEL: Record<Platform, string> = {
  twitch: 'Twitch',
  youtube: 'YouTube',
  kick: 'Kick',
  tiktok: 'TikTok',
  discord: 'Discord',
  system: 'System',
};
