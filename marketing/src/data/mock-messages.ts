import { staticFile } from 'remotion';
import type { ChatMessage } from '../primitives/types';

/**
 * Mock chat stream for the MultiPlatformChat scene.
 *
 * `revealAt` is in frames, relative to the SCENE start.
 *
 * Channel handle: `caesarlp` — caesar's actual Twitch channel. Same handle
 * across platforms (`@caesarlp` on YouTube, `caesarlp` on Kick, `@caesarlp.live`
 * on TikTok, `caesarlp` on Discord) as a reasonable approximation; swap if the
 * cross-platform handles differ.
 *
 * Emotes bundled locally in public/emotes/:
 *  - KEKW, pepelove, GIGACHAD, POG: from 7TV (PNG, first-frame for animated)
 *  - LUL: from Twitch CDN (global emote ID 425618)
 *  - caesar/cheers.png: code `caesarCHEERS`
 *  - caesar/a7.png: code `caesar51Pls`
 *
 * Twitch IRC convention: positions are 0-indexed character offsets, inclusive
 * on both ends. text.slice(start, end + 1) reconstructs the emote code.
 */

const KEKW = staticFile('emotes/kekw.png');
const LUL = staticFile('emotes/lul.png');
const PEPELOVE = staticFile('emotes/pepelove.png');
const GIGACHAD = staticFile('emotes/gigachad.png');
const POG = staticFile('emotes/pog.png');
const CAESAR_CHEERS = staticFile('emotes/caesar/cheers.png');
const CAESAR_PLS = staticFile('emotes/caesar/a7.png');

export const MOCK_MESSAGES: ChatMessage[] = [
  {
    id: '1',
    platform: 'twitch',
    channel_name: '#caesarlp',
    user: {
      id: 'u1',
      username: 'pixel_witch',
      display_name: 'pixel_witch',
      badges: [],
      color: '#ff79c6',
    },
    message: {
      text: 'POG i love this LUL',
      emotes: [
        { code: 'POG', provider: '7tv', url: POG, positions: [[0, 2]] },
        { code: 'LUL', provider: 'twitch', url: LUL, positions: [[16, 18]] },
      ],
    },
    timestamp: '',
    revealAt: 8,
  },
  {
    id: '2',
    platform: 'youtube',
    channel_name: '@caesarlp',
    user: {
      id: 'u2',
      username: 'tomato_jpg',
      display_name: 'tomato.jpg',
      badges: [],
      color: '#ffb86c',
    },
    message: {
      text: 'finally one overlay for all of them 🔥',
      emotes: [],
    },
    timestamp: '',
    revealAt: 22,
  },
  {
    id: '3',
    platform: 'kick',
    channel_name: 'caesarlp',
    user: {
      id: 'u3',
      username: 'shadowfax',
      display_name: 'shadowfax',
      badges: [],
      color: '#8be9fd',
    },
    message: {
      text: 'kick gang represent KEKW',
      emotes: [{ code: 'KEKW', provider: '7tv', url: KEKW, positions: [[20, 23]] }],
    },
    timestamp: '',
    revealAt: 36,
  },
  {
    id: '4',
    platform: 'tiktok',
    channel_name: '@caesarlp.live',
    user: {
      id: 'u4',
      username: 'glitchqueen',
      display_name: 'glitchqueen',
      badges: [],
      color: '#bd93f9',
    },
    message: {
      text: 'oh nice, all five platforms in one chat 👀',
      emotes: [],
    },
    timestamp: '',
    revealAt: 50,
  },
  {
    id: '5',
    platform: 'discord',
    channel_name: '#stream-chat',
    user: {
      id: 'u5',
      username: 'bytecrash',
      display_name: 'bytecrash',
      badges: [],
      color: '#5865f2',
    },
    message: {
      text: 'discord relay just works 🎯',
      emotes: [],
    },
    timestamp: '',
    revealAt: 64,
  },
  {
    id: '6',
    platform: 'twitch',
    channel_name: '#caesarlp',
    user: {
      id: 'u6',
      username: 'doomscroll',
      display_name: 'doomscroll',
      badges: [],
      color: '#50fa7b',
    },
    message: {
      text: 'pepelove pepelove pepelove',
      emotes: [
        {
          code: 'pepelove',
          provider: '7tv',
          url: PEPELOVE,
          positions: [
            [0, 7],
            [9, 16],
            [18, 25],
          ],
        },
      ],
    },
    timestamp: '',
    revealAt: 78,
  },
  {
    id: '7',
    platform: 'youtube',
    channel_name: '@caesarlp',
    user: {
      id: 'u7',
      username: 'helix_tv',
      display_name: 'helix.tv',
      badges: [],
      color: '#ff5555',
    },
    message: {
      text: 'POG super chat lands here too?',
      emotes: [{ code: 'POG', provider: '7tv', url: POG, positions: [[0, 2]] }],
    },
    timestamp: '',
    revealAt: 92,
  },
  {
    id: '8',
    platform: 'kick',
    channel_name: 'caesarlp',
    user: {
      id: 'u8',
      username: 'midnight_run',
      display_name: 'midnight_run',
      badges: [],
      color: '#f1fa8c',
    },
    message: {
      text: 'GIGACHAD setup caesar51Pls',
      emotes: [
        { code: 'GIGACHAD', provider: '7tv', url: GIGACHAD, positions: [[0, 7]] },
        { code: 'caesar51Pls', provider: 'twitch', url: CAESAR_PLS, positions: [[15, 25]] },
      ],
    },
    timestamp: '',
    revealAt: 106,
  },
  {
    id: '9',
    platform: 'discord',
    channel_name: '#stream-chat',
    user: {
      id: 'u9',
      username: 'plasma_dev',
      display_name: 'plasma_dev',
      badges: [],
      color: '#a78bfa',
    },
    message: {
      text: 'no bots, no setup, just an OBS source 👌',
      emotes: [],
    },
    timestamp: '',
    revealAt: 120,
  },
  {
    id: '10',
    platform: 'tiktok',
    channel_name: '@caesarlp.live',
    user: {
      id: 'u10',
      username: 'pinklemon',
      display_name: 'pinklemon',
      badges: [],
      color: '#ff79c6',
    },
    message: {
      text: 'sent a rose 🌹',
      emotes: [],
    },
    timestamp: '',
    revealAt: 134,
  },
  {
    id: '11',
    platform: 'twitch',
    channel_name: '#caesarlp',
    user: {
      id: 'u11',
      username: 'frostbyte',
      display_name: 'frostbyte',
      badges: [],
      color: '#8be9fd',
    },
    message: {
      text: 'LUL the YT chatters showed up',
      emotes: [{ code: 'LUL', provider: 'twitch', url: LUL, positions: [[0, 2]] }],
    },
    timestamp: '',
    revealAt: 148,
  },
  {
    id: '12',
    platform: 'kick',
    channel_name: 'caesarlp',
    user: {
      id: 'u12',
      username: 'binarywolf',
      display_name: 'binarywolf',
      badges: [],
      color: '#50fa7b',
    },
    message: {
      text: 'low latency too caesarCHEERS',
      emotes: [
        {
          code: 'caesarCHEERS',
          provider: 'twitch',
          url: CAESAR_CHEERS,
          positions: [[15, 26]],
        },
      ],
    },
    timestamp: '',
    revealAt: 162,
  },
  {
    id: '13',
    platform: 'tiktok',
    channel_name: '@caesarlp.live',
    user: {
      id: 'u13',
      username: 'quasar',
      display_name: 'quasar',
      badges: [],
      color: '#ffb86c',
    },
    message: {
      text: 'ok im sold KEKW',
      emotes: [{ code: 'KEKW', provider: '7tv', url: KEKW, positions: [[11, 14]] }],
    },
    timestamp: '',
    revealAt: 176,
  },
];
