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

/**
 * Static registry of overlay-editor settings sections (ADR-0042).
 *
 * The registry drives both the left nav (EditorNav) and the settings search
 * (SettingsSearch). Search matches against control labels and synonym
 * keywords declared here — NOT against the DOM — so a control is findable
 * even while its section isn't mounted. When you add a control to a section
 * component, add its entry here in the same PR.
 *
 * `anchorId` links an entry to a `data-setting-anchor` attribute in the
 * section's markup; selecting the search result scrolls to and highlights
 * that control (falls back to the section top when absent).
 */

export type EditorGroupId = 'setup' | 'appearance' | 'behavior' | 'advanced'

export type EditorSectionId =
  | 'theme'
  | 'sources'
  | 'moderators'
  | 'testing'
  | 'typography'
  | 'colors'
  | 'background'
  | 'visibility'
  | 'sizing'
  | 'platform-colors'
  | 'events'
  | 'messages'
  | 'filters'
  | 'sounds'
  | 'tts'
  | 'engagement'
  | 'custom-css'
  | 'danger-zone'

/**
 * Sections the onboarding guide can steer the nav to from a "Show me" click.
 *
 * A deliberate subset of EditorSectionId — only sections the checklist actually
 * points at — but declared once here so the editor page, its spotlight state and
 * OnboardingChecklist cannot drift apart.
 */
export type SpotlightSection = 'sources' | 'theme' | 'appearance' | 'moderators'

export interface EditorGroup {
  id: EditorGroupId
  label: string
}

export interface SectionSearchEntry {
  /** User-visible control label, as rendered in the section */
  label: string
  /** Space-separated synonyms; what users type when they can't name the control */
  keywords?: string
  /** Matches a data-setting-anchor attribute inside the section's markup */
  anchorId?: string
}

export interface EditorSection {
  id: EditorSectionId
  group: EditorGroupId
  title: string
  /** Shorter label for the nav column; defaults to title */
  navLabel?: string
  /** One-line, user-facing description shown under the section heading */
  description: string
  /** Extra keywords matching the section as a whole */
  keywords?: string
  entries: SectionSearchEntry[]
}

export const EDITOR_GROUPS: EditorGroup[] = [
  { id: 'setup', label: 'Setup' },
  { id: 'appearance', label: 'Appearance' },
  { id: 'behavior', label: 'Behavior' },
  { id: 'advanced', label: 'Advanced' },
]

export const EDITOR_SECTIONS: EditorSection[] = [
  // ---- Setup ----
  {
    id: 'theme',
    group: 'setup',
    title: 'Theme',
    description: 'Start from a preset look, then fine-tune anything below.',
    keywords: 'preset marketplace look style skin design',
    entries: [
      { label: 'Browse themes', keywords: 'preset marketplace apply look' },
      { label: 'Reset to theme defaults', keywords: 'reset restore defaults undo' },
    ],
  },
  {
    id: 'sources',
    group: 'setup',
    title: 'Sources',
    description: 'The chat platforms feeding this overlay.',
    keywords: 'platform channel connect',
    entries: [
      {
        label: 'Add source',
        keywords: 'connect twitch youtube kick tiktok discord platform channel account',
      },
      { label: 'Shared overlays', keywords: 'share partner collab accepted' },
      { label: 'Discord relay', keywords: 'relay channel server' },
      { label: 'YouTube stream selection', keywords: 'stream select strategy multiple' },
    ],
  },
  {
    id: 'moderators',
    group: 'setup',
    title: 'Moderators',
    description: 'People who may moderate this overlay’s chat with their own accounts.',
    keywords: 'mod mods moderator delegate invite permission team revoke trust helper',
    entries: [
      {
        label: 'Invite a moderator',
        keywords: 'add mod invite link code delegate trust helper volunteer',
      },
      { label: 'Which actions they may use', keywords: 'delete timeout ban unban permission' },
      { label: 'Which platforms they may act on', keywords: 'twitch youtube kick discord leg' },
      { label: 'Remove a moderator', keywords: 'revoke remove kick out delete access' },
      { label: 'Remove all moderators', keywords: 'revoke all kill switch panic emergency' },
    ],
  },
  {
    id: 'testing',
    group: 'setup',
    title: 'Testing',
    description: 'Inject mock messages and events to position and style the overlay.',
    keywords: 'mock fake preview obs simulate',
    entries: [
      {
        label: 'Inject mock message',
        keywords: 'mock fake test message send platform username avatar',
        anchorId: 'mockMessage',
      },
      { label: 'Sample chat', keywords: 'transcript demo conversation', anchorId: 'sampleChat' },
      {
        label: 'Sample events',
        keywords: 'follow subscription raid demo test',
        anchorId: 'sampleEvents',
      },
    ],
  },

  // ---- Appearance ----
  {
    id: 'typography',
    group: 'appearance',
    title: 'Typography',
    description: 'Fonts for messages, usernames, and timestamps.',
    keywords: 'font text',
    entries: [
      { label: 'Body Font', keywords: 'font family message text' },
      { label: 'Username Font', keywords: 'font name' },
      { label: 'Timestamp Font', keywords: 'font time' },
      { label: 'Font Weight', keywords: 'bold thin light black' },
      {
        label: 'Text Shadow',
        keywords: 'shadow outline readability contrast legibility glow drop stroke',
      },
      { label: 'Body Size', keywords: 'font size text bigger smaller px' },
      { label: 'Username Size', keywords: 'font size name px' },
      { label: 'Timestamp Size', keywords: 'font size time px' },
      { label: 'Line Height', keywords: 'spacing leading' },
      { label: 'Letter Spacing', keywords: 'tracking' },
    ],
  },
  {
    id: 'colors',
    group: 'appearance',
    title: 'Colors',
    description: 'Text colors. Per-platform accents live in Platform Colors.',
    entries: [
      { label: 'Message color', keywords: 'text color' },
      { label: 'Username color', keywords: 'name color' },
      { label: 'Timestamp color', keywords: 'time color' },
    ],
  },
  {
    id: 'background',
    group: 'appearance',
    title: 'Background & Bubbles',
    navLabel: 'Background',
    description: 'The box behind each message.',
    keywords: 'bubble box backdrop',
    entries: [
      { label: 'Overlay background', keywords: 'backdrop transparent' },
      { label: 'Bubble background', keywords: 'message box color' },
      { label: 'Border color', keywords: 'outline' },
      { label: 'Border radius', keywords: 'corner rounded round' },
      { label: 'Border width', keywords: 'outline thickness' },
      { label: 'Padding', keywords: 'spacing inside' },
      { label: 'Message gap', keywords: 'spacing between distance' },
      { label: 'Backdrop blur', keywords: 'glass frosted' },
    ],
  },
  {
    id: 'visibility',
    group: 'appearance',
    title: 'Visibility',
    description: 'Show or hide parts of every message.',
    keywords: 'hide show toggle display',
    entries: [
      {
        label: 'Show avatars',
        keywords: 'hide remove disable profile picture',
        anchorId: 'showAvatars',
      },
      {
        label: 'Show badges',
        keywords: 'hide remove disable mod sub vip moderator subscriber chat badge',
        anchorId: 'showBadges',
      },
      {
        label: 'Show timestamps',
        keywords: 'hide remove disable time clock',
        anchorId: 'showTimestamps',
      },
      { label: 'Show emotes', keywords: 'hide remove disable', anchorId: 'showEmotes' },
      {
        label: 'Show username',
        keywords: 'hide remove disable name anonymous',
        anchorId: 'showUsername',
      },
      {
        label: 'Show platform badge',
        keywords: 'hide remove disable platform label chip badge position style icon',
        anchorId: 'showPlatformBadge',
      },
      {
        label: 'Show platform indicators',
        keywords: 'hide remove disable ring color stripe',
        anchorId: 'showPlatformIndicators',
      },
      {
        label: 'Show pronouns',
        keywords: 'hide remove disable pronoun alejo position color',
        anchorId: 'showPronouns',
      },
    ],
  },
  {
    id: 'sizing',
    group: 'appearance',
    title: 'Sizing',
    description: 'How big avatars, badges, and emotes render.',
    keywords: 'size scale big small',
    entries: [
      { label: 'Avatar size', keywords: 'profile picture' },
      { label: 'Badge size', keywords: 'mod sub vip' },
      { label: 'Emote scale', keywords: '7tv bttv ffz size big' },
    ],
  },
  {
    id: 'platform-colors',
    group: 'appearance',
    title: 'Platform Colors',
    description: 'Accent color for each platform.',
    keywords: 'accent brand',
    entries: [
      { label: 'Twitch color', keywords: 'accent purple' },
      { label: 'YouTube color', keywords: 'accent red' },
      { label: 'Kick color', keywords: 'accent green' },
      { label: 'TikTok color', keywords: 'accent cyan' },
      { label: 'Discord color', keywords: 'accent blurple' },
    ],
  },
  {
    id: 'events',
    group: 'appearance',
    title: 'Events',
    description: 'Which stream events appear in chat, and how prominently.',
    keywords: 'alert celebration',
    entries: [
      { label: 'Super Chat', keywords: 'event superchat donation youtube show hide' },
      { label: 'Subscriptions', keywords: 'event sub resub show hide' },
      { label: 'Raids', keywords: 'event raid host show hide' },
      { label: 'Bits', keywords: 'event cheer bits show hide' },
      { label: 'Membership Gift', keywords: 'event gift sub show hide' },
      { label: 'Size modifier', keywords: 'event scale boost bigger' },
    ],
  },

  // ---- Behavior ----
  {
    id: 'messages',
    group: 'behavior',
    title: 'Messages',
    description: 'How messages appear, how many show, and how long they stay.',
    keywords: 'behavior chat flow',
    entries: [
      { label: 'Max Messages', keywords: 'limit count maximum', anchorId: 'maxMessages' },
      {
        label: 'Message Duration',
        keywords: 'fade timeout disappear stay seconds time',
        anchorId: 'messageDuration',
      },
      {
        label: 'Disable Message Fade Out',
        keywords: 'keep permanent stay forever',
        anchorId: 'disableFade',
      },
      // 'bottom' and 'direction' deliberately do NOT live here: searching
      // "bottom" used to land on this order toggle, which does the opposite of
      // anchoring the feed. Those keywords belong to Feed Anchor below (#728).
      {
        label: 'Invert Message Order',
        keywords: 'newest first reverse order sequence list',
        anchorId: 'invertOrder',
      },
      {
        label: 'Feed Anchor',
        keywords: 'bottom anchor upward grow start position align edge direction top',
        anchorId: 'feedAnchor',
      },
      {
        label: 'Entry Animation',
        keywords: 'animation animate fly slide bounce pop flip swoosh blur appear motion effect',
        anchorId: 'entryAnimation',
      },
      {
        label: 'Emote Providers',
        keywords: '7tv bttv ffz betterttv frankerfacez enable disable',
        anchorId: 'emoteProviders',
      },
      {
        label: '7TV Emote Set',
        keywords: 'seventv override custom set attach emotes',
        anchorId: 'seventvOverride',
      },
    ],
  },
  {
    id: 'filters',
    group: 'behavior',
    title: 'Filters',
    description: "Hide messages you don't want on stream.",
    keywords: 'block mute spam',
    entries: [
      { label: 'Blocked usernames', keywords: 'banned users block ignore mute bot' },
      { label: 'Blocked keywords', keywords: 'banned words filter profanity swear' },
      { label: 'Hide bot commands (!)', keywords: 'exclamation prefix commands nightbot' },
      { label: 'Min message length', keywords: 'short spam minimum characters' },
    ],
  },
  {
    id: 'sounds',
    group: 'behavior',
    title: 'Notification Sounds',
    navLabel: 'Sounds',
    description: 'Play a sound when a message lands.',
    keywords: 'audio ping alert',
    entries: [
      { label: 'Enable notification sounds', keywords: 'audio ping alert sound' },
      { label: 'Sound preset', keywords: 'audio chime pop' },
      { label: 'Volume', keywords: 'audio loud quiet' },
      { label: 'Cooldown', keywords: 'audio throttle rate limit' },
      { label: 'Custom sound URL', keywords: 'audio upload own premium' },
    ],
  },
  {
    id: 'tts',
    group: 'behavior',
    title: 'Text-to-Speech',
    description: 'Read chat messages aloud.',
    keywords: 'tts voice speech speak read aloud',
    entries: [
      { label: 'Enable TTS', keywords: 'voice read aloud speech speak' },
      { label: 'Voice', keywords: 'speaker language browser' },
      { label: 'Rate', keywords: 'speed fast slow' },
      { label: 'Pitch', keywords: 'tone high low' },
      { label: 'Throttling', keywords: 'rate limit messages per minute queue' },
      { label: 'ElevenLabs', keywords: 'premium ai voice api key eleven labs' },
    ],
  },
  {
    id: 'engagement',
    group: 'behavior',
    title: 'Engagement',
    description: 'Viewer points earned by watching and chatting.',
    keywords: 'points economy loyalty currency polls predictions',
    entries: [
      { label: 'Enable viewer points', keywords: 'economy loyalty currency points' },
      { label: 'Points name', keywords: 'currency rename' },
      { label: 'Earn rates', keywords: 'per minute message bonus watch' },
      { label: 'Widgets', keywords: 'links leaderboard obs' },
    ],
  },

  // ---- Advanced ----
  {
    id: 'custom-css',
    group: 'advanced',
    title: 'Custom CSS',
    description: 'Full control over overlay styling. Applied on top of all settings above.',
    keywords: 'expert code stylesheet',
    entries: [{ label: 'Custom CSS', keywords: 'css style stylesheet monaco editor code expert' }],
  },
  {
    id: 'danger-zone',
    group: 'advanced',
    title: 'Danger Zone',
    description: "Actions you can't undo.",
    keywords: 'reset delete',
    entries: [
      {
        label: 'Reset Overlay ID',
        keywords: 'revoke leaked url regenerate new id obs delete',
      },
    ],
  },
]

export interface SearchHit {
  section: EditorSection
  /** Undefined for a section-level hit (query matched the section itself) */
  entry?: SectionSearchEntry
  score: number
}

/**
 * Case-insensitive search over the registry.
 * Ranking: label word-prefix (3) > label substring (2) > keyword substring (1),
 * registry order within equal scores — so "badge" ranks "Show badges" (word
 * prefix, earlier section) above "Badge size" (word prefix, later section)
 * above anything matching only via keywords. Section-level hits use the
 * section title/keywords the same way.
 */
export function searchSettings(
  query: string,
  sections: EditorSection[] = EDITOR_SECTIONS,
  limit = 8
): SearchHit[] {
  const q = query.trim().toLowerCase()
  if (q.length < 2) return []

  const scoreText = (label: string, keywords?: string): number => {
    const l = label.toLowerCase()
    if (l.split(/[^a-z0-9]+/).some((word) => word.startsWith(q))) return 3
    if (l.includes(q)) return 2
    if (keywords !== undefined && keywords.toLowerCase().includes(q)) return 1
    return 0
  }

  const hits: SearchHit[] = []
  for (const section of sections) {
    const sectionScore = scoreText(section.title, section.keywords)
    if (sectionScore > 0) {
      hits.push({ section, score: sectionScore })
    }
    for (const entry of section.entries) {
      const entryScore = scoreText(entry.label, entry.keywords)
      if (entryScore > 0) {
        hits.push({ section, entry, score: entryScore })
      }
    }
  }

  // Stable sort: score desc, registry order preserved within equal scores
  return hits.sort((a, b) => b.score - a.score).slice(0, limit)
}
