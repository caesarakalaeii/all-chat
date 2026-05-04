import type { Platform } from '../primitives/types';

/**
 * Overlay names are user-defined in the real product (no canonical roster).
 * These are demo entries — short, plausible names a multistreamer might pick.
 *
 * Fields below correspond ONLY to fields the real /dashboard route renders
 * (frontend/src/app/dashboard/page.tsx:235-313):
 *   - name (string)
 *   - is_public_for_viewers (Extension badge)
 *   - sources (platform array — drives top border + badges + count)
 *
 * Do NOT add invented metrics like viewers/min or msgs/min — those don't
 * exist on the real dashboard.
 */
export interface OverlayCard {
  id: string;
  name: string;
  platforms: Platform[];
  isExtension: boolean;
}

export const OVERLAYS: OverlayCard[] = [
  {
    id: 'main',
    name: 'Main Stream',
    platforms: ['twitch', 'youtube', 'kick', 'tiktok', 'discord'],
    isExtension: true,
  },
  {
    id: 'jc',
    name: 'Just Chatting',
    platforms: ['twitch', 'tiktok'],
    isExtension: false,
  },
  {
    id: 'collab',
    name: 'Co-Stream',
    platforms: ['twitch', 'youtube'],
    isExtension: false,
  },
  {
    id: 'late',
    name: 'After Hours',
    platforms: ['twitch', 'discord'],
    isExtension: false,
  },
  {
    id: 'speedrun',
    name: 'Speedruns',
    platforms: ['twitch', 'youtube', 'kick'],
    isExtension: false,
  },
  {
    id: 'stash',
    name: 'Mod Stash',
    platforms: ['twitch'],
    isExtension: false,
  },
];
