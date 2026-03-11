/**
 * Local PlatformBadge re-export.
 *
 * Adapts the shares-domain `source` object shape to the shared
 * PlatformBadge component from @/components/ui/badge.
 */
import { PlatformBadge as SharedPlatformBadge } from '@/components/ui/badge';
import type { Platform } from '@/lib/platform-colors';

interface PlatformBadgeProps {
  source: {
    platform: string;
    channel_name: string;
  };
}

export function PlatformBadge({ source }: PlatformBadgeProps) {
  const knownPlatforms: Platform[] = ['twitch', 'youtube', 'kick', 'tiktok', 'system'];
  const platform: Platform = knownPlatforms.includes(source.platform as Platform)
    ? (source.platform as Platform)
    : 'system';

  return (
    <SharedPlatformBadge platform={platform} />
  );
}
