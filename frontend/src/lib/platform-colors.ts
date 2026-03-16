/**
 * Static platform color mapping for Tailwind JIT safety.
 *
 * Uses complete literal class strings — no dynamic construction.
 * Tailwind JIT requires full class names to be present as static strings.
 */
export const PLATFORM_COLORS = {
  twitch: { text: 'text-twitch', bg: 'bg-twitch' },
  youtube: { text: 'text-youtube', bg: 'bg-youtube' },
  kick: { text: 'text-kick', bg: 'bg-kick' },
  tiktok: { text: 'text-tiktok', bg: 'bg-tiktok' },
  discord: { text: 'text-discord', bg: 'bg-discord' },
  system: { text: 'text-text-sub', bg: 'bg-surface' },
} as const

export type Platform = keyof typeof PLATFORM_COLORS
