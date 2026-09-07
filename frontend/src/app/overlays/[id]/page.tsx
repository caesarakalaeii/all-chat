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
 * Overlay Editor Page
 *
 * Edit an existing overlay and manage its chat sources.
 * Displays a split-view layout: full config panel left, live embed preview iframe right.
 *
 * Features:
 * - Platform-colored source cards with PlatformBadge
 * - Draggable split-view with live overlay embed preview
 * - Dialog confirmation for source removal
 * - Toast feedback for all actions
 * - Skeleton loading states
 * - Shared overlay sources (v1.3) with revocation support
 * - WebSocket listener for real-time share_revoked notifications
 * - OAuth callback handling (source_added / error query params)
 * - Customization controls (font size, max messages, duration, badges, emotes)
 * - Mock message injection
 * - Custom CSS editor with theme marketplace
 * - Copy OBS URL button
 */

'use client'

import { use, useCallback, useEffect, useId, useRef, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { ChevronLeft, ChevronRight, X, Clipboard, Share2, Puzzle } from 'lucide-react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { overlaysApi } from '@/lib/api/overlays'
import type { TTSConfigMetadata, ElevenLabsVoice, TestKeyResult } from '@/lib/api/overlays'
import { startAddSourceReflow } from '@/lib/api/add-source'
import { sharesApi } from '@/lib/api/shares'
import { getGuilds, getGuildChannels, updateSourceConfig } from '@/lib/api/discord'
import type { DiscordGuild, ChannelCategory } from '@/lib/api/discord'
import { ApiError } from '@/lib/api/client'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { OnboardingChecklist } from '@/components/onboarding/OnboardingChecklist'
import { ObsDockHelpContent } from '@/components/onboarding/ObsDockHelpContent'
import { ObsHelpContent } from '@/components/onboarding/ObsHelpContent'
import { deriveSteps, useOnboardingStore } from '@/lib/stores/onboarding-store'
import { engagementApi } from '@/lib/api/engagement'
import type { EarnConfig } from '@/lib/types/engagement'
import type {
  Overlay,
  ChatSource,
  DiscordSourceConfig,
  FilterSettings,
  DisplaySettings,
} from '@/lib/types/overlay'
import type { ChatMessage } from '@/lib/types/message'
import type { AcceptedShare } from '@/lib/types/share'
import {
  isMessageAnimation,
  type MessageAnimation,
  type VisualSettings,
} from '@/lib/types/visual-settings'
import { visualSettingsToCss } from '@/lib/utils/visual-settings-to-css'
import { useNotificationSocket } from '@/hooks/useNotificationSocket'
import { parseCssToVisualSettings } from '@/lib/utils/theme-css-parser'
import { getBundledTheme } from '@/lib/theme-marketplace/bundled-themes'
import { DEFAULT_FEED_ANCHOR, parseFeedAnchor, type FeedAnchor } from '@/lib/utils/feedAnchor'
import { isCustomCssForked } from '@/lib/utils/custom-css'
import type { CssIssue } from '@/lib/utils/custom-css'
import { computeThemeCssDiff, reconstructEditorCss } from '@/lib/utils/theme-css-diff'
import type { CustomCssMode } from '@/lib/utils/theme-css-diff'
import type { Theme } from '@/lib/theme-marketplace/types'
import { toastManager } from '@/lib/toast'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { trackEvent } from '@/lib/analytics'
import { useTrackOnce } from '@/hooks/useTrackOnce'
import { AppNav } from '@/components/AppNav'
import { SplitView } from '@/components/SplitView'
import { PremiumUpsellLink } from '@/components/PremiumUpsellLink'
import { Card } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Dialog } from '@/components/ui/dialog'
import { PlatformBadge } from '@/components/ui/badge'
import { StatusBadge } from '@/app/dashboard/shares/components/StatusBadge'
import { RevocationConfirmModal } from '@/app/dashboard/shares/components/RevocationConfirmModal'
import { cn } from '@/lib/utils'
import { TypographyGroup } from '@/components/appearance/TypographyGroup'
import { BubbleColorsGroup } from '@/components/appearance/BubbleColorsGroup'
import { ColorsGroup } from '@/components/appearance/ColorsGroup'
import { BackgroundGroup } from '@/components/appearance/BackgroundGroup'
import { VisibilityGroup } from '@/components/appearance/VisibilityGroup'
import { SizingGroup } from '@/components/appearance/SizingGroup'
import { PlatformColorsGroup } from '@/components/appearance/PlatformColorsGroup'
import { EventsGroup } from '@/components/appearance/EventsGroup'
import { FilterGroup } from '@/components/appearance/FilterGroup'
import { SoundGroup } from '@/components/appearance/SoundGroup'
import { TTSGroup } from '@/components/appearance/TTSGroup'
import { EditorNav } from '@/components/editor/EditorNav'
import { EditorSectionHeader } from '@/components/editor/EditorSectionHeader'
import { SettingsSearch } from '@/components/editor/SettingsSearch'
import { AdvancedDisclosure } from '@/components/editor/AdvancedDisclosure'
import { ModeratorsPanel } from '@/components/editor/ModeratorsPanel'
import { getTranslations, useTranslations, type TFunction } from '@/lib/i18n'
import { emphasise, interpolateElements } from '@/lib/i18n/emphasise'
import {
  EDITOR_SECTIONS,
  type EditorSectionId,
  type SpotlightSection,
} from '@/components/editor/sectionRegistry'
import dynamic from 'next/dynamic'

// Read through getTranslations() rather than the hook: the dynamic() loading
// callback below is not a component, so it cannot call one. Same pattern as
// SAY_HI_TOGGLE_LABEL in components/appearance/FilterGroup.tsx.
const EDITOR_LOADING_LABEL = getTranslations()('overlayEditor.page.loadingEditor')

// Dynamically import Monaco Editor to avoid SSR issues
const MonacoCSSEditor = dynamic(() => import('@/components/MonacoCSSEditor'), {
  ssr: false,
  loading: () => (
    <div
      className={
        // eslint-disable-next-line tailwindcss/no-unnecessary-arbitrary-value -- px is intentional here: this is a fixed-size loading placeholder, matched to the Monaco editor it is replaced by, sized to the layout it sits in rather than to the reading text, so it must not grow with the root font size the way the suggested rem-relative utility would
        'flex h-[300px] items-center justify-center rounded-lg border border-border bg-surface-2'
      }
    >
      <div className="text-sm text-text-sub">{EDITOR_LOADING_LABEL}</div>
    </div>
  ),
})

// Dynamically import ThemeContent for inline Theme section (SSR unsafe — uses hooks)
const ThemeContent = dynamic(
  () =>
    import('@/components/theme-marketplace/ThemeContent').then((m) => ({
      default: m.ThemeContent,
    })),
  { ssr: false }
)

// Static platform border mapping — full literal class strings for Tailwind JIT safety
const PLATFORM_BORDER: Record<string, string> = {
  twitch: 'border-l-twitch',
  youtube: 'border-l-youtube',
  kick: 'border-l-kick',
  tiktok: 'border-l-tiktok',
  shared_overlay: 'border-l-twitch',
  discord: 'border-l-discord',
}

// Last-active settings section (ADR-0042); replaces the retired per-drawer
// open/closed maps (editor-panel-sections-v1 / appearance-panel-sections-v1).
const ACTIVE_SECTION_STORAGE_KEY = 'editor-active-section-v1'

/**
 * '3 errors' or '1 warning', in words the catalog owns.
 *
 * The pre-migration code appended an 's' when the count was above one. That is
 * an English rule, so the singular and plural are separate catalog keys and the
 * choice between them is made here.
 *
 * Takes `t` as a parameter because it is not a component and cannot call the hook.
 */
function cssProblemCount(t: TFunction, kind: 'error' | 'warning', count: number): string {
  if (kind === 'error') {
    return count === 1
      ? t('overlayEditor.customCss.errorCountOne', { count })
      : t('overlayEditor.customCss.errorCountMany', { count })
  }
  return count === 1
    ? t('overlayEditor.customCss.warningCountOne', { count })
    : t('overlayEditor.customCss.warningCountMany', { count })
}

/** A 7TV reference the API resolved: always a set ID, optionally named and counted. */
type ResolvedSevenTvReference = {
  status: 'resolved'
  setID: string
  name?: string
  emoteCount?: number
}

/**
 * The notice shown once a 7TV reference resolves.
 *
 * The name and the emote count are each optional, so there are four reachable
 * sentences. They are four separate catalog keys rather than one key plus two
 * appended fragments: a translator cannot reorder across a fragment boundary,
 * and the pre-migration code built exactly that kind of sentence.
 *
 * Takes `t` as a parameter because it is not a component and cannot call the hook.
 */
function resolvedSevenTvNotice(t: TFunction, state: ResolvedSevenTvReference): string {
  const { name, emoteCount } = state
  const hasCount = typeof emoteCount === 'number'
  if (name && hasCount) {
    return t('overlayEditor.messages.seventvResolvedNamedCounted', { name, count: emoteCount })
  }
  if (name) return t('overlayEditor.messages.seventvResolvedNamed', { name })
  if (hasCount) return t('overlayEditor.messages.seventvResolvedCounted', { count: emoteCount })
  return t('overlayEditor.messages.seventvResolved')
}

// Entry-animation choices for new chat messages ('' = default fade + slide up).
// Values map to .msg-anim-* classes in globals.css via visual_settings.
// Labels live in the catalog under `overlayEditor.messages.animation<Stem>`;
// this list holds only the class-name values. `as const satisfies` keeps each
// stem a string literal so a typo fails tsc at the lookup.
const ENTRY_ANIMATION_OPTIONS = [
  { value: '', messageStem: 'Default' },
  { value: 'fly-left', messageStem: 'FlyLeft' },
  { value: 'fly-right', messageStem: 'FlyRight' },
  { value: 'fly-spring', messageStem: 'FlySpring' },
  { value: 'pop', messageStem: 'Pop' },
  { value: 'bounce', messageStem: 'Bounce' },
  { value: 'flip', messageStem: 'Flip' },
  { value: 'swoosh', messageStem: 'Swoosh' },
  { value: 'soft-focus', messageStem: 'SoftFocus' },
] as const satisfies ReadonlyArray<{ value: MessageAnimation | ''; messageStem: string }>

// Maps onboarding spotlight targets to left-nav sections (ADR-0042). The
// guide's 'appearance' target predates the flat nav; Typography is the first
// Appearance-group section, so the "customize" step lands there.
// Which nav section a spotlight target opens. Keyed on SpotlightSection so adding a
// "Show me" target is a compile error until it is mapped, rather than a silent no-op.
const SPOTLIGHT_SECTION: Record<SpotlightSection, EditorSectionId> = {
  sources: 'sources',
  theme: 'theme',
  appearance: 'typography',
  moderators: 'moderators',
}

type MockMessageFormState = {
  platform: ChatMessage['platform']
  displayName: string
  username: string
  avatarUrl: string
  message: string
  color: string
}

const DEFAULT_MOCK_FORM: MockMessageFormState = {
  platform: 'twitch',
  displayName: 'Overlay Fan',
  username: 'overlayfan',
  avatarUrl: '',
  message: 'This overlay looks great! PogChamp',
  color: '#9146ff',
}

const SAMPLE_MOCK_MESSAGES: Array<Omit<ChatMessage, 'id' | 'timestamp' | 'overlay_id'>> = [
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'sample-user-1',
      username: 'retro_mod',
      display_name: 'RetroMod',
      avatar_url: 'https://i.pravatar.cc/100?img=13',
      badges: [],
      color: '#fbbf24',
    },
    message: { text: 'Welcome to the overlay preview! PogChamp', emotes: [] },
    metadata: { mock: true },
  },
  {
    platform: 'youtube',
    channel_id: 'sample-youtube',
    channel_name: 'Sample YouTube',
    user: {
      id: 'sample-user-2',
      username: 'cybercritic',
      display_name: 'CyberCritic',
      avatar_url: 'https://i.pravatar.cc/100?img=32',
      badges: [],
      color: '#f87171',
    },
    message: { text: 'Picked up the neon CSS preset and it SLAPS 🔥', emotes: [] },
    metadata: { mock: true },
  },
  {
    platform: 'kick',
    channel_id: 'sample-kick',
    channel_name: 'Sample Kick',
    user: {
      id: 'sample-user-3',
      username: 'emote_master',
      display_name: 'EmoteMaster',
      avatar_url: 'https://i.pravatar.cc/100?img=56',
      badges: [],
      color: '#4ade80',
    },
    message: { text: 'Drop your favorite emotes in chat 😎', emotes: [] },
    metadata: { mock: true },
  },
]

const SAMPLE_EVENT_MESSAGES: Array<Omit<ChatMessage, 'id' | 'timestamp' | 'overlay_id'>> = [
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-1',
      username: 'generousviewer',
      display_name: 'GenerousViewer',
      avatar_url: 'https://i.pravatar.cc/100?img=45',
      badges: [],
      color: '#ff6b6b',
    },
    message: { text: 'Love the stream! Keep it up!', emotes: [] },
    event: {
      type: 'subscription',
      tier: 'high',
      duration: 30,
      is_update: false,
      metadata: { sub_tier: '1000', months: 1, streak: 1 },
    },
    metadata: { mock: true, event: true },
  },
  {
    platform: 'youtube',
    channel_id: 'sample-youtube',
    channel_name: 'Sample YouTube',
    user: {
      id: 'event-user-2',
      username: 'superfan',
      display_name: 'SuperFan',
      avatar_url: 'https://i.pravatar.cc/100?img=67',
      badges: [],
      color: '#e91e63',
    },
    message: { text: 'Amazing content! Thanks for all you do!', emotes: [] },
    event: {
      type: 'super_chat',
      tier: 'high',
      value: { amount: 50, currency: 'USD', display_text: '$50.00' },
      duration: 60,
      is_update: false,
      metadata: {},
    },
    metadata: { mock: true, event: true },
  },
  {
    platform: 'twitch',
    channel_id: 'sample-twitch',
    channel_name: 'Sample Twitch',
    user: {
      id: 'event-user-3',
      username: 'bigstreamer',
      display_name: 'BigStreamer',
      avatar_url: 'https://i.pravatar.cc/100?img=23',
      badges: [],
      color: '#9146ff',
    },
    message: { text: 'is raiding with 2,500 viewers!', emotes: [] },
    event: {
      type: 'raid',
      tier: 'high',
      duration: 40,
      is_update: false,
      metadata: { viewer_count: 2500 },
    },
    metadata: { mock: true, event: true },
  },
]

function SourceCard({
  source,
  onRemove,
  onRevoke,
  onConfigureRelay,
  onConfigureStreamSelect,
  isOwnChannel,
  onReconnectChat,
}: {
  source: ChatSource
  onRemove: (id: string) => void
  onRevoke?: (source: ChatSource) => void
  onConfigureRelay?: (source: ChatSource) => void
  onConfigureStreamSelect?: (source: ChatSource) => void
  // isOwnChannel is true when this Twitch source belongs to the logged-in user's own
  // channel (only then can they grant the chat scopes); onReconnectChat starts the
  // add-source OAuth reflow that requests them.
  isOwnChannel?: boolean
  onReconnectChat?: () => void
}) {
  const t = useTranslations()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const isTwitch = source.platform === 'twitch'
  const isShared = source.platform === 'shared_overlay'
  const isInactiveShared = isShared && !source.is_active
  const isDiscord = source.platform === 'discord'
  const isYoutube = source.platform === 'youtube'

  const discordConfig = isDiscord ? (source.config as DiscordSourceConfig) : null
  const discordLabel =
    isDiscord && discordConfig
      ? `${(source.config as DiscordSourceConfig & { guild_name?: string }).guild_name ?? ''} › #${source.channel_name ?? source.channel_id}`
      : null

  return (
    <Card
      className={cn(
        'border-l-2 p-4',
        PLATFORM_BORDER[source.platform] ?? 'border-l-border',
        isInactiveShared && 'opacity-50'
      )}
    >
      <div className="flex items-center justify-between">
        <div className="flex min-w-0 items-center gap-3">
          <PlatformBadge
            platform={source.platform as 'twitch' | 'youtube' | 'kick' | 'tiktok'}
            size="sm"
          />
          <div className="min-w-0">
            <span className="truncate text-sm font-medium text-text">
              {discordLabel ?? source.channel_name ?? source.channel_id}
            </span>
            {isDiscord && (
              <div className="mt-1 flex flex-wrap items-center gap-2">
                {/* Connection status badge */}
                <span
                  className={cn(
                    'inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium',
                    source.is_active
                      ? 'border-green-500/20 bg-green-500/10 text-green-400'
                      : 'border-border bg-surface-2/40 text-text-sub'
                  )}
                >
                  <span
                    className={cn(
                      'size-1.5 rounded-full',
                      source.is_active ? 'bg-green-400' : 'bg-text-sub'
                    )}
                  />
                  {source.is_active ? 'Connected' : 'Disconnected'}
                </span>
                {/* Relay ON/OFF badge */}
                {discordConfig && (
                  <span
                    className={cn(
                      'inline-flex items-center rounded-full border px-2 py-0.5 text-xs font-medium',
                      discordConfig.relay_enabled
                        ? 'border-green-600/30 bg-green-600/20 text-green-300'
                        : 'border-border bg-surface-2 text-text-sub'
                    )}
                  >
                    {discordConfig.relay_enabled ? 'Relay ON' : 'Relay OFF'}
                  </span>
                )}
              </div>
            )}
            {isInactiveShared && source.share_status && (
              <StatusBadge status={source.share_status} size="sm" />
            )}
            {/* Twitch: surface whether chat is read via EventSub, and offer a reconnect
                CTA on the owner's own channel when it is not yet enabled. */}
            {isTwitch && source.chat_via_eventsub && (
              <div className="mt-1">
                <span className="inline-flex items-center gap-1 rounded-full border border-green-500/20 bg-green-500/10 px-2 py-0.5 text-xs font-medium text-green-400">
                  <span className="size-1.5 rounded-full bg-green-400" />
                  {t('overlayEditor.sources.chatViaEventsub')}
                </span>
              </div>
            )}
            {isTwitch && !source.chat_via_eventsub && isOwnChannel && onReconnectChat && (
              <Button
                onClick={onReconnectChat}
                variant="link"
                size="xs"
                className="mt-1 h-auto p-0 text-text-sub hover:text-text"
              >
                {t('overlayEditor.sources.reconnectChat')}
              </Button>
            )}
          </div>
        </div>
        <div className="ml-3 flex shrink-0 items-center gap-2">
          {isShared && source.is_active && onRevoke && (
            <Button
              variant="outline"
              size="sm"
              className="border-destructive/40 text-xs text-destructive hover:bg-destructive/10"
              onClick={() => onRevoke(source)}
            >
              {t('overlayEditor.sources.revoke')}
            </Button>
          )}
          <Dialog.Root open={confirmOpen} onOpenChange={setConfirmOpen}>
            <Dialog.Trigger
              render={
                <Button
                  variant="ghost"
                  size="icon-xs"
                  className="hover:text-destructive"
                  aria-label={t('overlayEditor.sources.removeLabel', {
                    channel: source.channel_name || source.channel_id,
                  })}
                >
                  <X className="size-4" />
                </Button>
              }
            />
            <Dialog.Content>
              <Dialog.Title>{t('overlayEditor.sources.removeConfirmTitle')}</Dialog.Title>
              <Dialog.Description>
                {emphasise(
                  t('overlayEditor.sources.removeConfirmBody', {
                    emphasis: source.channel_name || source.channel_id,
                  }),
                  source.channel_name || source.channel_id,
                  (run) => (
                    <strong>{run}</strong>
                  )
                )}
              </Dialog.Description>
              <div className="flex justify-end gap-3 pt-2">
                <Dialog.Close
                  render={<Button variant="outline">{t('overlayEditor.sources.cancel')}</Button>}
                />
                <Button
                  variant="destructive"
                  onClick={() => {
                    setConfirmOpen(false)
                    onRemove(source.id)
                  }}
                >
                  {t('overlayEditor.sources.remove')}
                </Button>
              </div>
            </Dialog.Content>
          </Dialog.Root>
        </div>
      </div>
      {/* Configure relay button — Discord only */}
      {isDiscord && onConfigureRelay && (
        <Button
          variant="ghost"
          size="xs"
          className="mt-2 px-0"
          onClick={() => onConfigureRelay(source)}
        >
          <ChevronRight className="size-3" />
          {t('overlayEditor.sources.configureRelay')}
        </Button>
      )}
      {/* Stream selection button — YouTube only */}
      {isYoutube && onConfigureStreamSelect && (
        <Button
          variant="ghost"
          size="xs"
          className="mt-2 px-0"
          onClick={() => onConfigureStreamSelect(source)}
        >
          <ChevronRight className="size-3" />
          {t('overlayEditor.sources.streamSelection')}
        </Button>
      )}
    </Card>
  )
}

// The wire values only. Their labels and descriptions live in the catalog under
// `overlayEditor.streamSelection.<camelCasedValue>Label|Description`, so this
// list stays the single source of the API contract and holds no copy.
const STREAM_STRATEGIES = [
  { value: 'first_found', messageStem: 'firstFound' },
  { value: 'most_viewers', messageStem: 'mostViewers' },
  { value: 'fewest_viewers', messageStem: 'fewestViewers' },
  { value: 'title_match', messageStem: 'titleMatch' },
  { value: 'title_match_all', messageStem: 'titleMatchAll' },
  { value: 'all', messageStem: 'all' },
] as const

function StreamSelectionPanel({
  source,
  overlayId,
  isPremium,
  onSaved,
}: {
  source: ChatSource
  overlayId: string
  isPremium: boolean
  onSaved: () => void
}) {
  const t = useTranslations()
  const ytConfig = (source.config ?? {}) as import('@/lib/types/overlay').YouTubeSourceConfig
  const [strategy, setStrategy] = useState(ytConfig.stream_select ?? 'first_found')
  const [matchTerm, setMatchTerm] = useState(ytConfig.stream_match ?? '')
  const selectedStrategy = STREAM_STRATEGIES.find((s) => s.value === strategy)
  const [saving, setSaving] = useState(false)
  const strategySelectId = useId()
  const strategyHintId = useId()
  const matchInputId = useId()

  const hasChanges =
    strategy !== (ytConfig.stream_select ?? 'first_found') ||
    matchTerm !== (ytConfig.stream_match ?? '')

  async function handleSave() {
    setSaving(true)
    try {
      const needsMatch = strategy === 'title_match' || strategy === 'title_match_all'
      const config: Record<string, unknown> = {
        ...source.config,
        stream_select: strategy === 'first_found' ? undefined : strategy,
        stream_match: needsMatch ? matchTerm : undefined,
      }
      // Clean undefined keys
      Object.keys(config).forEach((k) => config[k] === undefined && delete config[k])
      await overlaysApi.updateSourceConfig(overlayId, source.id, config)
      trackEvent('yt_stream_strategy_set', { strategy })
      toastManager.add({ title: t('overlayEditor.toasts.streamSelectionSaved'), type: 'success' })
      onSaved()
    } catch {
      toastManager.add({
        title: t('overlayEditor.toasts.streamSelectionSaveFailed'),
        type: 'error',
      })
    } finally {
      setSaving(false)
    }
  }

  const isNonDefault = strategy !== 'first_found'
  const locked = !isPremium && isNonDefault

  return (
    <Card className="mt-1 rounded-t-none border-t-0 bg-surface-2/50 p-4">
      <div className="space-y-3">
        <div>
          <label
            htmlFor={strategySelectId}
            className="mb-1 block text-xs font-medium text-text-sub"
          >
            {t('overlayEditor.streamSelection.strategyLabel')}
          </label>
          <p id={strategyHintId} className="mb-2 text-xs text-text-sub/70">
            {t('overlayEditor.streamSelection.strategyHint')}
          </p>
          <select
            id={strategySelectId}
            aria-describedby={strategyHintId}
            value={strategy}
            onChange={(e) => setStrategy(e.target.value as typeof strategy)}
            className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-xs text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            {STREAM_STRATEGIES.map((s) => (
              <option
                key={s.value}
                value={s.value}
                disabled={!isPremium && s.value !== 'first_found'}
              >
                {t(`overlayEditor.streamSelection.${s.messageStem}Label`)}
                {!isPremium && s.value !== 'first_found'
                  ? t('overlayEditor.streamSelection.premiumSuffix')
                  : ''}
              </option>
            ))}
          </select>
          {selectedStrategy && (
            <p className="mt-1 text-xs text-text-sub/60">
              {t(`overlayEditor.streamSelection.${selectedStrategy.messageStem}Description`)}
            </p>
          )}
          {!isPremium && (
            <p className="mt-1 text-xs text-text-dim">
              <PremiumUpsellLink />
              {t('overlayEditor.streamSelection.upsellSuffix')}
            </p>
          )}
        </div>

        {(strategy === 'title_match' || strategy === 'title_match_all') && (
          <div>
            <label htmlFor={matchInputId} className="mb-1 block text-xs font-medium text-text-sub">
              {t('overlayEditor.streamSelection.matchLabel')}
            </label>
            <Input
              id={matchInputId}
              value={matchTerm}
              onChange={(e) => setMatchTerm(e.target.value)}
              placeholder={t('overlayEditor.streamSelection.matchPlaceholder')}
              className="text-xs"
            />
          </div>
        )}

        {locked && (
          <p className="text-xs text-yellow-400/80">{t('overlayEditor.streamSelection.locked')}</p>
        )}

        <Button
          size="sm"
          variant="outline"
          className="w-full"
          disabled={
            !hasChanges ||
            saving ||
            locked ||
            ((strategy === 'title_match' || strategy === 'title_match_all') && !matchTerm.trim())
          }
          onClick={handleSave}
        >
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </div>
    </Card>
  )
}

function RelayPanel({
  source,
  overlayId,
  onSaved,
}: {
  source: ChatSource
  overlayId: string
  onSaved: (updated: ChatSource) => void
}) {
  const t = useTranslations()
  const discordConfig = source.config as DiscordSourceConfig
  const [relayEnabled, setRelayEnabled] = useState(discordConfig.relay_enabled ?? false)
  const [relayChannelId, setRelayChannelId] = useState<string>(discordConfig.relay_channel_id ?? '')
  const [channels, setChannels] = useState<ChannelCategory[]>([])
  // Starts true rather than being flipped from the effect body: setState in an effect body is a
  // build error (`react-hooks/set-state-in-effect`), and the channel list is always in flight on
  // the first render anyway. This is safe only because guild_id cannot change under a mounted
  // panel — the panel is rendered per source, inside `<div key={source.id}>`, and a source's
  // guild_id is fixed — so the effect runs exactly once and never has to re-arm the skeleton.
  const [channelsLoading, setChannelsLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const relayChannelSelectId = useId()

  useEffect(() => {
    getGuildChannels(discordConfig.guild_id)
      .then((res) => setChannels(res.categories))
      .catch(() => setChannels([]))
      .finally(() => setChannelsLoading(false))
  }, [discordConfig.guild_id])

  const handleSave = async () => {
    setSaving(true)
    const newConfig: DiscordSourceConfig = {
      ...discordConfig,
      relay_enabled: relayEnabled,
      relay_channel_id: relayEnabled ? relayChannelId : null,
    }
    // Optimistic update
    onSaved({ ...source, config: newConfig })
    try {
      await updateSourceConfig(overlayId, source.id, newConfig)
      toastManager.add({ title: t('overlayEditor.toasts.relaySaved'), type: 'success' })
    } catch {
      onSaved(source) // rollback
      toastManager.add({ title: t('overlayEditor.toasts.relaySaveFailed'), type: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const isSaveDisabled = saving || (relayEnabled && !relayChannelId)

  return (
    <div className="mt-1 ml-4 space-y-3 rounded-lg border border-border bg-surface-2 p-4">
      {/* Loop filter info — static */}
      <p className="text-xs text-text-sub">{t('overlayEditor.relay.loopFilter')}</p>

      {/* Relay toggle */}
      <label className="flex cursor-pointer items-center gap-2 text-sm text-text">
        <input
          type="checkbox"
          checked={relayEnabled}
          onChange={(e) => setRelayEnabled(e.target.checked)}
          className="size-4 accent-discord"
        />
        {t('overlayEditor.relay.enable')}
      </label>

      {/* Outbound channel picker — visible only when relay enabled */}
      {relayEnabled && (
        <div>
          <label htmlFor={relayChannelSelectId} className="mb-1 block text-xs text-text-sub">
            {t('overlayEditor.relay.outboundChannelLabel')}
          </label>
          {channelsLoading ? (
            <Skeleton className="h-9 w-full rounded-lg" />
          ) : (
            <select
              id={relayChannelSelectId}
              value={relayChannelId}
              onChange={(e) => setRelayChannelId(e.target.value)}
              className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              <option value="">{t('overlayEditor.relay.selectChannel')}</option>
              {channels.map((cat) => (
                <optgroup key={cat.id || 'uncategorized'} label={cat.name}>
                  {cat.channels.map((ch) => (
                    <option key={ch.id} value={ch.id}>
                      #{ch.name}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          )}
        </div>
      )}

      <Button
        size="sm"
        disabled={isSaveDisabled}
        onClick={() => void handleSave()}
        className="w-full"
      >
        {saving ? 'Saving...' : 'Save'}
      </Button>
    </div>
  )
}

function SourceListSkeleton() {
  return (
    <div className="space-y-3">
      {[0, 1].map((i) => (
        // eslint-disable-next-line tailwindcss/no-unnecessary-arbitrary-value -- the skeleton stands in for a 60px-tall source row and must match that literal height; the suggested h-15 is rem-relative while the row it replaces is not, so the placeholder would jump as the real content swaps in
        <Skeleton key={i} className="h-[60px] w-full rounded-xl" />
      ))}
    </div>
  )
}

// ---- Engagement (polls, predictions & viewer points — issue #523) -----------

type EarnNumberKey =
  | 'bits_multiplier'
  | 'usd_multiplier'
  | 'sub_high'
  | 'sub_medium'
  | 'sub_low'
  | 'gift_per_sub'
  | 'chat_per_minute'
  | 'watch_per_minute'

// The wire keys and their numeric behaviour only. Labels and hints live in the
// catalog under `overlayEditor.engagement.<messageStem>Label|Hint`.
// `float` and `comingSoon` are spelled out on every entry rather than left
// optional: `as const` narrows each element to its own object type, and an
// absent optional property is then not on the union at all. `as const` is what
// keeps messageStem a string literal, so a stem with no matching catalog key
// fails tsc at the lookup below instead of rendering the key name to a streamer.
const EARN_NUMBER_FIELDS = [
  { key: 'bits_multiplier', messageStem: 'bitsMultiplier', float: true, comingSoon: false },
  { key: 'usd_multiplier', messageStem: 'usdMultiplier', float: true, comingSoon: false },
  { key: 'sub_high', messageStem: 'subHigh', float: false, comingSoon: false },
  { key: 'sub_medium', messageStem: 'subMedium', float: false, comingSoon: false },
  { key: 'sub_low', messageStem: 'subLow', float: false, comingSoon: false },
  { key: 'gift_per_sub', messageStem: 'giftPerSub', float: false, comingSoon: false },
  // chat_per_minute has no producer in v1 (nothing publishes engagement:chat), so it
  // never accrues — flag it disabled so streamers don't configure a dead dimension (M6).
  { key: 'chat_per_minute', messageStem: 'chatPerMinute', float: false, comingSoon: true },
  // watch_per_minute rewards participation-PAGE focus time (heartbeat), not stream-watch time (M6).
  { key: 'watch_per_minute', messageStem: 'watchPerMinute', float: false, comingSoon: false },
] as const satisfies ReadonlyArray<{
  key: EarnNumberKey
  messageStem: string
  float: boolean
  comingSoon: boolean
}>

// Earn config lives on the engagement-service (own endpoint, like the TTS
// config), so this panel loads and saves independently of Save Configuration.
// The chat commands a viewer literally types. They are protocol the engagement
// service parses, not copy: translating `!vote` would stop the command working.
// Named constants rather than JSX literals so the i18n gate does not have to
// carry an exception for them.
const EXAMPLE_VOTE_COMMAND = '!vote 2'
const EXAMPLE_BARE_VOTE = '2'
const EXAMPLE_PREDICT_COMMAND = '!predict 1 500'

function EngagementPanel({ overlayId }: { overlayId: string }) {
  const t = useTranslations()
  const [config, setConfig] = useState<EarnConfig | null>(null)
  const [numbers, setNumbers] = useState<Record<EarnNumberKey, string> | null>(null)
  const [loadError, setLoadError] = useState(false)
  const [saving, setSaving] = useState(false)
  const [copiedPath, setCopiedPath] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    engagementApi
      .getConfig(overlayId)
      .then((cfg) => {
        if (cancelled) return
        setConfig(cfg)
        setNumbers(
          Object.fromEntries(EARN_NUMBER_FIELDS.map((f) => [f.key, String(cfg[f.key])])) as Record<
            EarnNumberKey,
            string
          >
        )
      })
      .catch(() => {
        if (!cancelled) setLoadError(true)
      })
    return () => {
      cancelled = true
    }
  }, [overlayId])

  const handleSave = async () => {
    if (!config || !numbers) return
    const parsed = {} as Record<EarnNumberKey, number>
    for (const f of EARN_NUMBER_FIELDS) {
      if (f.comingSoon) continue // not editable yet; the stored value is preserved via ...config
      const n = Number(numbers[f.key])
      const fieldLabel = t(`overlayEditor.engagement.${f.messageStem}Label`)
      if (!Number.isFinite(n) || n < 0) {
        toastManager.add({
          title: t('overlayEditor.engagement.invalidValue', { field: fieldLabel }),
          type: 'error',
        })
        return
      }
      // All point amounts are int64 server-side; only the multipliers take decimals.
      if (!f.float && !Number.isInteger(n)) {
        toastManager.add({
          title: t('overlayEditor.engagement.mustBeWhole', { field: fieldLabel }),
          type: 'error',
        })
        return
      }
      parsed[f.key] = n
    }
    setSaving(true)
    try {
      // PUT is a full upsert — send the complete object.
      const saved = await engagementApi.updateConfig(overlayId, {
        ...config,
        ...parsed,
        points_name: config.points_name.trim() || 'Points',
      })
      setConfig(saved)
      setNumbers(
        Object.fromEntries(EARN_NUMBER_FIELDS.map((f) => [f.key, String(saved[f.key])])) as Record<
          EarnNumberKey,
          string
        >
      )
      toastManager.add({ title: t('overlayEditor.toasts.engagementSaved'), type: 'success' })
    } catch (err) {
      toastManager.add({
        title: t('overlayEditor.toasts.engagementSaveFailed'),
        description: err instanceof ApiError ? err.message : undefined,
        type: 'error',
      })
    } finally {
      setSaving(false)
    }
  }

  const shareLinks: ReadonlyArray<{ label: string; path: string; desc: string }> = [
    {
      label: t('overlayEditor.engagement.pollWidgetLabel'),
      path: `/overlay/${overlayId}/poll`,
      desc: t('overlayEditor.engagement.pollWidgetDescription'),
    },
    {
      label: t('overlayEditor.engagement.predictionWidgetLabel'),
      path: `/overlay/${overlayId}/prediction`,
      desc: t('overlayEditor.engagement.predictionWidgetDescription'),
    },
    {
      label: t('overlayEditor.engagement.participateLabel'),
      path: `/overlay/${overlayId}/participate`,
      desc: t('overlayEditor.engagement.participateDescription'),
    },
  ]

  const copyLink = async (path: string) => {
    try {
      await navigator.clipboard.writeText(`${window.location.origin}${path}`)
      setCopiedPath(path)
      setTimeout(() => setCopiedPath(null), 2000)
    } catch {
      toastManager.add({
        title: t('overlayEditor.engagement.copyLinkFailed'),
        type: 'error',
      })
    }
  }

  // Twitch native mirroring opt-in (ADR-0030). Adds read-only channel:read:polls /
  // predictions scopes; the consent flow returns to the Monitor view.
  const startMirrorConsent = async () => {
    try {
      window.location.href = await engagementApi.getTwitchMirrorConsentUrl(overlayId)
    } catch {
      toastManager.add({
        title: t('overlayEditor.toasts.twitchConsentFailed'),
        type: 'error',
      })
    }
  }

  if (loadError) {
    return <p className="text-xs text-destructive">{t('overlayEditor.engagement.loadError')}</p>
  }
  if (!config || !numbers) {
    return <Skeleton className="h-40 w-full rounded-lg" />
  }

  // Resolved in two passes: `t()` substitutes the points name as plain text and
  // leaves the three command placeholders standing, then interpolateElements
  // replaces those with <code> runs. Doing it the other way round would need the
  // commands as strings, and `2` occurring inside `!vote 2` makes that ambiguous.
  const pointsExplainer = interpolateElements(
    t('overlayEditor.engagement.pointsExplainer', {
      pointsName: config.points_name.trim() || t('overlayEditor.engagement.pointsNamePlaceholder'),
    }),
    {
      voteCommand: <code>{EXAMPLE_VOTE_COMMAND}</code>,
      bareVote: <code>{EXAMPLE_BARE_VOTE}</code>,
      predictCommand: <code>{EXAMPLE_PREDICT_COMMAND}</code>,
    }
  )

  return (
    <div className="space-y-4">
      <label className="flex cursor-pointer items-center gap-2 text-sm text-text">
        <input
          type="checkbox"
          checked={config.enabled}
          onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
          className="size-4 accent-twitch"
        />
        {t('overlayEditor.engagement.enablePoints')}
      </label>
      <p className="text-xs text-text-sub">{pointsExplainer}</p>

      <div>
        <label className="flex cursor-pointer items-center gap-2 text-sm text-text">
          <input
            type="checkbox"
            checked={config.announce_on_start}
            onChange={(e) => setConfig({ ...config, announce_on_start: e.target.checked })}
            className="size-4 accent-twitch"
          />
          {t('overlayEditor.engagement.announceRounds')}
        </label>
        <p className="mt-0.5 text-[11px] text-text-sub">
          {t('overlayEditor.engagement.announceRoundsHint')}
        </p>
      </div>

      <div>
        <label htmlFor="earn-points-name" className="mb-1 block text-xs text-text-sub">
          {t('overlayEditor.engagement.pointsNameLabel')}
        </label>
        <Input
          id="earn-points-name"
          value={config.points_name}
          onChange={(e) => setConfig({ ...config, points_name: e.target.value })}
          placeholder={t('overlayEditor.engagement.pointsNamePlaceholder')}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        {EARN_NUMBER_FIELDS.map((f) => (
          <div key={f.key}>
            <label htmlFor={`earn-${f.key}`} className="mb-1 block text-xs text-text-sub">
              {t(`overlayEditor.engagement.${f.messageStem}Label`)}
            </label>
            <Input
              id={`earn-${f.key}`}
              type="number"
              min={0}
              step={f.float ? 'any' : 1}
              value={numbers[f.key]}
              disabled={f.comingSoon}
              onChange={(e) => setNumbers({ ...numbers, [f.key]: e.target.value })}
              size="sm"
              className={cn(f.comingSoon && 'opacity-50')}
            />
            <p className="mt-0.5 text-[11px] text-text-sub">
              {t(`overlayEditor.engagement.${f.messageStem}Hint`)}
              {f.comingSoon && t('overlayEditor.engagement.comingSoonSuffix')}
            </p>
          </div>
        ))}
      </div>

      <Button size="sm" className="w-full" disabled={saving} onClick={() => void handleSave()}>
        {saving ? t('overlayEditor.engagement.saving') : t('overlayEditor.engagement.save')}
      </Button>

      <div className="space-y-2 border-t border-border pt-3">
        <p className="text-xs font-medium text-text">
          {t('overlayEditor.engagement.linksHeading')}
        </p>
        {shareLinks.map((link) => (
          <div key={link.path} className="flex items-center gap-2">
            <div className="min-w-0 flex-1">
              <p className="text-xs text-text">{link.label}</p>
              <p className="text-[11px] text-text-sub">{link.desc}</p>
            </div>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="shrink-0 text-xs"
              onClick={() => void copyLink(link.path)}
            >
              {copiedPath === link.path
                ? t('overlayEditor.engagement.copiedLink')
                : t('overlayEditor.engagement.copyLink')}
            </Button>
          </div>
        ))}
        {/* L-Docs1: browser-source setup guidance for the two OBS widgets. */}
        <p className="text-[11px] text-text-sub">
          {emphasise(
            t('overlayEditor.engagement.browserSourceHint', {
              emphasis: t('overlayEditor.engagement.browserSourceHintEmphasis'),
            }),
            t('overlayEditor.engagement.browserSourceHintEmphasis'),
            (run) => (
              <span className="font-medium">{run}</span>
            )
          )}
        </p>
        {/* L-U9: the participation link is meant to be shared with viewers (on-screen / panels). */}
        <p className="text-[11px] text-text-sub">
          {t('overlayEditor.engagement.participateShareHint')}
        </p>
      </div>

      {/* Twitch native mirroring (M4/M5): a labelled, discoverable control with the
          resync-expectation note, so a streamer who opts in knows it activates on the
          next channel sync rather than immediately. The consent flow returns to the
          Monitor view, where the mirrored rounds appear. */}
      <div className="space-y-2 border-t border-border pt-3">
        <p className="text-xs font-medium text-text">
          {t('overlayEditor.engagement.mirroringHeading')}
        </p>
        <p className="text-[11px] text-text-sub">{t('overlayEditor.engagement.mirroringBody')}</p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="text-xs"
          onClick={() => void startMirrorConsent()}
        >
          {t('overlayEditor.engagement.enableMirroring')}
        </Button>
      </div>
    </div>
  )
}

// Platform source buttons — OAuth redirect for Twitch/YouTube/Kick,
// text input for TikTok (no OAuth required), dialog for Discord.
// Admins also get a manual channel ID form for any platform.
function AddSourceForm({
  overlayId,
  onAddTikTok,
  onAddManual,
  onSourceAdded,
  isAdmin = false,
}: {
  overlayId: string
  onAddTikTok: (username: string) => void
  onAddManual?: (platform: string, channelId: string) => void
  onSourceAdded?: () => void
  isAdmin?: boolean
}) {
  const t = useTranslations()
  const [tiktokUsername, setTiktokUsername] = useState('')
  const [tiktokDialogOpen, setTiktokDialogOpen] = useState(false)
  const [isAdding, setIsAdding] = useState(false)
  const [adminPlatform, setAdminPlatform] = useState('twitch')

  // Discord dialog state
  const [guilds, setGuilds] = useState<DiscordGuild[]>([])
  const [guildsLoaded, setGuildsLoaded] = useState(false)
  const [discordDialogOpen, setDiscordDialogOpen] = useState(false)
  const [discordStep, setDiscordStep] = useState<1 | 2>(1)
  const [selectedGuild, setSelectedGuild] = useState<DiscordGuild | null>(null)
  const [selectedChannelId, setSelectedChannelId] = useState<string>('')
  const [selectedChannelName, setSelectedChannelName] = useState<string>('')
  const [guildChannels, setGuildChannels] = useState<ChannelCategory[]>([])
  const [channelsLoading, setChannelsLoading] = useState(false)
  const [isAddingDiscord, setIsAddingDiscord] = useState(false)
  const discordChannelSelectId = useId()

  useEffect(() => {
    getGuilds()
      .then((data) => setGuilds(data))
      .catch(() => setGuilds([]))
      .finally(() => setGuildsLoaded(true))
  }, [])

  const handleDiscordButtonClick = () => {
    if (guilds.length === 0) return
    setDiscordStep(1)
    setSelectedGuild(null)
    setSelectedChannelId('')
    setSelectedChannelName('')
    setGuildChannels([])
    setDiscordDialogOpen(true)
  }

  const fetchChannelsForGuild = async (guildId: string) => {
    setChannelsLoading(true)
    try {
      const res = await getGuildChannels(guildId)
      setGuildChannels(res.categories)
    } catch {
      setGuildChannels([])
    } finally {
      setChannelsLoading(false)
    }
  }

  const handleSelectGuild = (guild: DiscordGuild) => {
    setSelectedGuild(guild)
    setSelectedChannelId('')
    setSelectedChannelName('')
    setDiscordStep(2)
    void fetchChannelsForGuild(guild.guild_id)
  }

  const handleAddDiscordSource = async () => {
    if (!selectedGuild || !selectedChannelId) return
    setIsAddingDiscord(true)
    try {
      const config: DiscordSourceConfig & { guild_name: string } = {
        guild_id: selectedGuild.guild_id,
        guild_name: selectedGuild.guild_name,
        inbound_channel_id: selectedChannelId,
        relay_enabled: false,
        relay_channel_id: null,
      }
      await overlaysApi.addSource(overlayId, {
        platform: 'discord',
        channel_id: selectedChannelId,
        channel_name: selectedChannelName,
        config,
      })
      setDiscordDialogOpen(false)
      setDiscordStep(1)
      setSelectedGuild(null)
      setSelectedChannelId('')
      setSelectedChannelName('')
      onSourceAdded?.()
      trackEvent('source_configured', { platform: 'discord' })
      toastManager.add({ title: t('overlayEditor.toasts.discordSourceAdded'), type: 'success' })
    } catch (err) {
      // Surface the server's reason: the Discord source guard (ADR-0048) refuses channels in
      // servers the user has not connected and explains what to do, which a generic failure
      // toast would throw away.
      toastManager.add({
        title: t('overlayEditor.toasts.discordSourceAddFailed'),
        description: err instanceof ApiError ? err.message : undefined,
        type: 'error',
      })
    } finally {
      setIsAddingDiscord(false)
    }
  }

  // Start the add-source OAuth reflow and act on the result. Routed through
  // startAddSourceReflow (apiClient) so under H3 cookie auth an expired access
  // cookie is refreshed and the request retried, instead of dead-ending with
  // "Authorization header required".
  const startOAuth = async (endpoint: string) => {
    const result = await startAddSourceReflow(endpoint)
    if (result.kind === 'redirect') {
      safeExternalRedirect(result.authUrl)
      return
    }
    if (result.kind === 'added') {
      // Backend short-circuit: the streamer already has valid credentials with
      // the required scopes (e.g. reconnecting Twitch after removing the
      // source), so the source was added directly without an OAuth reflow.
      // Refresh the source list instead of silently doing nothing.
      onSourceAdded?.()
      toastManager.add({ title: t('overlayEditor.toasts.sourceAdded'), type: 'success' })
      return
    }
    // Surface the failure rather than failing silently.
    toastManager.add({
      title: t('overlayEditor.toasts.oauthConnectFailed'),
      description: result.message,
      type: 'error',
    })
  }
  const [adminChannelId, setAdminChannelId] = useState('')
  const [isAdminAdding, setIsAdminAdding] = useState(false)
  const [youtubeResolved, setYoutubeResolved] = useState<{
    channel_id: string
    title?: string
    custom_url?: string
    thumbnail?: string
  } | null>(null)
  const [youtubeResolveError, setYoutubeResolveError] = useState<string | null>(null)

  const handleTikTokSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    const username = tiktokUsername.trim().replace(/^@/, '')
    if (!username) return
    setIsAdding(true)
    try {
      await onAddTikTok(username)
      setTiktokUsername('')
      setTiktokDialogOpen(false)
    } finally {
      setIsAdding(false)
    }
  }

  return (
    <div className="space-y-3">
      <p className="text-xs text-text-sub">{t('overlayEditor.addSource.intro')}</p>

      {/* OAuth buttons — fetch auth_url then redirect, same pattern as login */}
      <div className="grid grid-cols-1 gap-2">
        <Button
          onClick={() => startOAuth(`/api/v1/auth/twitch/add-source/${overlayId}`)}
          size="lg"
          className="gap-2.5"
        >
          <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
            <path
              fill="currentColor"
              d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z"
            />
          </svg>
          {t('overlayEditor.addSource.connectTwitch')}
        </Button>

        <Button
          onClick={() => startOAuth(`/api/v1/auth/youtube/add-source/${overlayId}`)}
          size="lg"
          className="gap-2.5 text-bg"
          style={
            { backgroundColor: '#FF0000', '--tw-ring-color': '#FF0000' } as React.CSSProperties
          }
        >
          <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
            <path
              fill="#FFFFFF"
              d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"
            />
          </svg>
          {t('overlayEditor.addSource.connectYoutube')}
        </Button>

        <Button
          onClick={() => startOAuth(`/api/v1/auth/kick/add-source/${overlayId}`)}
          size="lg"
          className="gap-2.5 text-bg"
          style={{ backgroundColor: 'var(--color-kick)' }}
        >
          <svg className="h-4 w-4 shrink-0" viewBox="0 0 512 512" aria-hidden="true">
            <path
              fill="currentColor"
              d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z"
            />
          </svg>
          {t('overlayEditor.addSource.connectKick')}
        </Button>

        {/* TikTok — no OAuth; the button matches the OAuth buttons but opens a
            username dialog, since TikTok Live is keyed on a creator handle. */}
        <Button
          onClick={() => setTiktokDialogOpen(true)}
          size="lg"
          className="gap-2.5 text-white"
          style={
            { backgroundColor: '#010101', '--tw-ring-color': '#69C9D0' } as React.CSSProperties
          }
        >
          <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
            <path
              fill="#FFFFFF"
              d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z"
            />
          </svg>
          {t('overlayEditor.addSource.connectTiktok')}
        </Button>

        {/* Discord — guild dialog or settings prompt */}
        {guildsLoaded && guilds.length === 0 ? (
          <p className="rounded-lg border border-border bg-surface px-4 py-2.5 text-sm text-text-sub">
            {emphasise(
              t('overlayEditor.addSource.discordNeedsServer', {
                emphasis: t('overlayEditor.addSource.discordNeedsServerEmphasis'),
              }),
              t('overlayEditor.addSource.discordNeedsServerEmphasis'),
              (run) => (
                <Link href="/settings" className="text-discord underline hover:opacity-80">
                  {run}
                </Link>
              )
            )}
          </p>
        ) : (
          <Button
            onClick={handleDiscordButtonClick}
            size="lg"
            className="gap-2.5 text-white"
            style={
              { backgroundColor: '#5865F2', '--tw-ring-color': '#5865F2' } as React.CSSProperties
            }
          >
            {/* Discord logo mark */}
            <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" aria-hidden="true">
              <path
                fill="#FFFFFF"
                d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057c.001.022.015.043.03.056a19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"
              />
            </svg>
            {t('overlayEditor.addSource.connectDiscord')}
          </Button>
        )}
      </div>

      {/* Discord 2-step dialog */}
      <Dialog.Root
        open={discordDialogOpen}
        onOpenChange={(open) => {
          setDiscordDialogOpen(open)
          if (!open) {
            setDiscordStep(1)
            setSelectedGuild(null)
            setSelectedChannelId('')
            setSelectedChannelName('')
          }
        }}
      >
        <Dialog.Content>
          <Dialog.Title>
            {discordStep === 1 ? 'Select a Discord Server' : 'Select a Channel'}
          </Dialog.Title>
          <Dialog.Description>
            {discordStep === 1
              ? 'Choose which Discord server to add as a source.'
              : `Picking a channel from ${selectedGuild?.guild_name ?? ''}`}
          </Dialog.Description>

          {discordStep === 1 && (
            <div className="mt-3 space-y-2">
              {guilds.map((guild) => (
                <Button
                  key={guild.guild_id}
                  onClick={() => handleSelectGuild(guild)}
                  variant="outline"
                  size="lg"
                  className="w-full justify-start gap-3"
                >
                  {guild.guild_icon ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={`https://cdn.discordapp.com/icons/${guild.guild_id}/${guild.guild_icon}.png?size=32`}
                      alt=""
                      className="size-8 rounded-full object-cover"
                    />
                  ) : (
                    <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-discord/20 text-xs font-bold text-discord">
                      {guild.guild_name.charAt(0).toUpperCase()}
                    </div>
                  )}
                  <span className="truncate font-medium">{guild.guild_name}</span>
                  <ChevronRight className="ml-auto size-4 shrink-0 text-text-sub" />
                </Button>
              ))}
            </div>
          )}

          {discordStep === 2 && (
            <div className="mt-3 space-y-3">
              {channelsLoading ? (
                <Skeleton className="h-9 w-full rounded-lg" />
              ) : (
                <div>
                  <label
                    htmlFor={discordChannelSelectId}
                    className="mb-1 block text-xs text-text-sub"
                  >
                    {t('overlayEditor.addSource.channelLabel')}
                  </label>
                  <select
                    id={discordChannelSelectId}
                    value={selectedChannelId}
                    onChange={(e) => {
                      setSelectedChannelId(e.target.value)
                      const ch = guildChannels
                        .flatMap((cat) => cat.channels)
                        .find((c) => c.id === e.target.value)
                      setSelectedChannelName(ch?.name ?? '')
                    }}
                    className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                  >
                    <option value="">{t('overlayEditor.addSource.selectChannel')}</option>
                    {guildChannels.map((cat) => (
                      <optgroup key={cat.id || 'uncategorized'} label={cat.name}>
                        {cat.channels.map((ch) => (
                          <option key={ch.id} value={ch.id}>
                            #{ch.name}
                          </option>
                        ))}
                      </optgroup>
                    ))}
                  </select>
                </div>
              )}
            </div>
          )}

          <div className="mt-4 flex justify-end gap-2">
            {discordStep === 2 && (
              <Button
                variant="outline"
                onClick={() => {
                  setDiscordStep(1)
                  setSelectedChannelId('')
                  setSelectedChannelName('')
                }}
              >
                {t('overlayEditor.addSource.back')}
              </Button>
            )}
            <Dialog.Close
              render={<Button variant="outline">{t('overlayEditor.addSource.cancel')}</Button>}
            />
            {discordStep === 2 && (
              <Button
                disabled={!selectedChannelId || isAddingDiscord}
                onClick={() => void handleAddDiscordSource()}
              >
                {isAddingDiscord
                  ? t('overlayEditor.addSource.addingEllipsis')
                  : t('overlayEditor.addSource.add')}
              </Button>
            )}
          </div>
        </Dialog.Content>
      </Dialog.Root>

      {/* TikTok — username dialog opened from the "Connect TikTok" button above */}
      <Dialog.Root
        open={tiktokDialogOpen}
        onOpenChange={(open) => {
          setTiktokDialogOpen(open)
          if (!open) setTiktokUsername('')
        }}
      >
        <Dialog.Content>
          <Dialog.Title>{t('overlayEditor.addSource.tiktokTitle')}</Dialog.Title>
          <Dialog.Description>{t('overlayEditor.addSource.tiktokBody')}</Dialog.Description>
          <form onSubmit={handleTikTokSubmit} className="mt-3">
            <Input
              value={tiktokUsername}
              onChange={(e) => setTiktokUsername(e.target.value)}
              placeholder={t('overlayEditor.addSource.tiktokPlaceholder')}
            />
            <div className="mt-4 flex justify-end gap-2">
              <Dialog.Close
                render={
                  <Button variant="outline" type="button">
                    {t('overlayEditor.addSource.cancel')}
                  </Button>
                }
              />
              <Button type="submit" disabled={isAdding || !tiktokUsername.trim()}>
                {isAdding ? t('overlayEditor.addSource.adding') : t('overlayEditor.addSource.add')}
              </Button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Root>

      {/* Admin manual entry — any platform, any channel ID */}
      {isAdmin && onAddManual && (
        <details className="border-t border-border pt-1">
          <summary className="cursor-pointer py-1 text-xs text-text-sub select-none hover:text-text">
            {t('overlayEditor.addSource.adminSummary')}
          </summary>
          <form
            className="mt-2 space-y-2"
            onSubmit={async (e) => {
              e.preventDefault()
              if (!adminChannelId.trim()) return
              setIsAdminAdding(true)
              setYoutubeResolveError(null)
              try {
                if (adminPlatform === 'youtube') {
                  const resolved = await overlaysApi.resolveYouTubeChannel(adminChannelId.trim())
                  setYoutubeResolved(resolved)
                  await onAddManual(adminPlatform, resolved.channel_id)
                  setAdminChannelId('')
                  setYoutubeResolved(null)
                } else {
                  await onAddManual(adminPlatform, adminChannelId.trim())
                  setAdminChannelId('')
                }
              } catch (err) {
                if (adminPlatform === 'youtube') {
                  setYoutubeResolveError(
                    err instanceof Error ? err.message : 'Failed to resolve YouTube channel'
                  )
                  setYoutubeResolved(null)
                }
              } finally {
                setIsAdminAdding(false)
              }
            }}
          >
            <div className="flex gap-2">
              <select
                value={adminPlatform}
                onChange={(e) => {
                  setAdminPlatform(e.target.value)
                  setYoutubeResolved(null)
                  setYoutubeResolveError(null)
                }}
                className="rounded-lg border border-border bg-surface px-2 py-1.5 text-xs text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                <option value="twitch">{t('common.platforms.twitch')}</option>
                <option value="youtube">{t('common.platforms.youtube')}</option>
                <option value="kick">{t('common.platforms.kick')}</option>
                <option value="tiktok">{t('common.platforms.tiktok')}</option>
              </select>
              <Input
                value={adminChannelId}
                onChange={(e) => {
                  setAdminChannelId(e.target.value)
                  setYoutubeResolved(null)
                  setYoutubeResolveError(null)
                }}
                placeholder={
                  adminPlatform === 'youtube'
                    ? t('overlayEditor.addSource.adminYoutubePlaceholder')
                    : t('overlayEditor.addSource.adminChannelPlaceholder')
                }
                className="flex-1 text-xs"
              />
            </div>
            {adminPlatform === 'youtube' && youtubeResolved && (
              <div className="flex items-center gap-2 rounded-lg bg-surface px-2 py-1.5 text-xs text-text-sub">
                {youtubeResolved.thumbnail && (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={youtubeResolved.thumbnail}
                    alt=""
                    className="h-6 w-6 rounded-full object-cover"
                  />
                )}
                <span className="font-medium text-text">
                  {youtubeResolved.title ?? youtubeResolved.channel_id}
                </span>
                {youtubeResolved.custom_url && (
                  <span className="text-text-sub">{youtubeResolved.custom_url}</span>
                )}
                <span className="ml-auto font-mono text-[10px] text-text-sub">
                  {youtubeResolved.channel_id}
                </span>
              </div>
            )}
            {adminPlatform === 'youtube' && youtubeResolveError && (
              <p className="text-xs text-red-500">{youtubeResolveError}</p>
            )}
            <Button
              type="submit"
              disabled={isAdminAdding || !adminChannelId.trim()}
              size="sm"
              variant="outline"
              className="w-full"
            >
              {isAdminAdding
                ? adminPlatform === 'youtube'
                  ? t('overlayEditor.addSource.adminResolving')
                  : t('overlayEditor.addSource.adding')
                : t('overlayEditor.addSource.adminAdd')}
            </Button>
          </form>
        </details>
      )}
    </div>
  )
}

export default function OverlayEditorPage({ params }: { params: Promise<{ id: string }> }) {
  const t = useTranslations()
  const { id } = use(params)
  const router = useRouter()
  const searchParams = useSearchParams()
  const { user } = useAuthStore()
  const mockFieldId = useId()

  // Activation funnel: the editor is the most-used, least-instrumented surface.
  useTrackOnce('editor_opened')

  // --- Overlay / sources state ---
  const [overlay, setOverlay] = useState<Overlay | null>(null)
  const [sources, setSources] = useState<ChatSource[]>([])
  const [isLoading, setIsLoading] = useState(true)
  const [acceptedShares, setAcceptedShares] = useState<AcceptedShare[]>([])
  const [revokeTarget, setRevokeTarget] = useState<ChatSource | null>(null)

  // --- First-run setup guide (onboarding) ---
  const onboardingActive = useOnboardingStore((s) => s.status === 'active')
  const setActiveOverlay = useOnboardingStore((s) => s.setActiveOverlay)
  const obsCopiedStep = useOnboardingStore((s) => s.sessionSteps.obsCopied)
  // Explicit spotlight from a checklist "Show me" click; while unset the
  // active step's own section is spotlighted.
  const [spotlightOverride, setSpotlightOverride] = useState<SpotlightSection | null>(null)

  // --- Settings navigation (ADR-0042): left nav, one section at a time ---
  const [activeSection, setActiveSection] = useState<EditorSectionId>(() => {
    if (typeof window === 'undefined') return 'theme'
    try {
      const stored = localStorage.getItem(ACTIVE_SECTION_STORAGE_KEY)
      if (stored !== null && EDITOR_SECTIONS.some((s) => s.id === stored)) {
        return stored as EditorSectionId
      }
    } catch {
      // localStorage unavailable — fall through to the default section
    }
    return 'theme'
  })
  // Search jump target: a data-setting-anchor value inside the activating section
  const [pendingAnchor, setPendingAnchor] = useState<string | null>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  // --- Customization state ---
  const entryAnimationSelectId = useId()
  const feedAnchorSelectId = useId()
  const [maxMessages, setMaxMessages] = useState(50)
  const [messageDuration, setMessageDuration] = useState(15)
  const [disableMessageFade, setDisableMessageFade] = useState(false)
  const [invertMessageOrder, setInvertMessageOrder] = useState(false)
  // Which canvas edge the feed rests on — orthogonal to invertMessageOrder.
  const [feedAnchor, setFeedAnchor] = useState<FeedAnchor>(DEFAULT_FEED_ANCHOR)
  const [enable7tv, setEnable7tv] = useState(true)
  const [enableBttv, setEnableBttv] = useState(true)
  const [enableFfz, setEnableFfz] = useState(true)
  // Per-overlay 7TV override input. The user can paste an emote-set ID
  // (24-char hex or 26-char ULID), a 7tv.app emote-set URL, or a 7tv.app user
  // profile URL; the backend resolves and stores the canonical set ID.
  const [seventvOverrideInput, setSeventvOverrideInput] = useState('')
  const [seventvOverrideSavedID, setSeventvOverrideSavedID] = useState('')
  // Descriptor for the currently-saved set, populated by an auto-resolve on
  // config load so the editor shows the human name + emote count instead of
  // an opaque ULID. Cleared whenever the saved ID changes to "".
  const [seventvSavedDescriptor, setSeventvSavedDescriptor] = useState<{
    name?: string
    emoteCount?: number
  } | null>(null)
  const [seventvResolveState, setSeventvResolveState] = useState<
    | { status: 'idle' }
    | { status: 'resolving' }
    | ResolvedSevenTvReference
    | { status: 'error'; message: string }
  >({ status: 'idle' })
  const [seventvRemoving, setSeventvRemoving] = useState(false)
  const [isPublicForViewers, setIsPublicForViewers] = useState(false)
  const [configLoaded, setConfigLoaded] = useState(false)
  const [isSavingConfig, setIsSavingConfig] = useState(false)
  const [configAlert, setConfigAlert] = useState<{
    type: 'success' | 'error'
    message: string
  } | null>(null)

  // --- Mock messages state ---
  const [mockForm, setMockForm] = useState<MockMessageFormState>(DEFAULT_MOCK_FORM)

  // --- Custom CSS state (ADR-0043) ---
  // customCss is the CSS shown in the Advanced editor. On a themed overlay it is
  // PRELOADED with the bundled theme's CSS (pristineThemeCss) so users see and can
  // edit real CSS instead of a blank box. While customCss still equals the pristine
  // theme the overlay stays linked to the bundle (theme fixes propagate on deploy);
  // editing it detaches this overlay onto its own saved custom_css copy. "Forked"
  // is derived (customCss present && != pristine), not a separate flag.
  const [customCss, setCustomCss] = useState('')
  const [pristineThemeCss, setPristineThemeCss] = useState('')
  const [cssIssues, setCssIssues] = useState<CssIssue[]>([])
  const [themeId, setThemeId] = useState('')
  // How the current editor content maps to storage (ADR-0043): 'linked' = equals
  // theme, 'diff' = only-changes stored (theme still updates), 'fork' = full copy
  // stored because theme rules were deleted (this overlay stops auto-updating).
  const [customCssMode, setCustomCssMode] = useState<CustomCssMode>('linked')
  // Debounce handle for pushing raw custom CSS to the live preview as the user types.
  const customCssDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Setup-guide wiring: bind steps 2-4 to this overlay, and while the guide
  // is active steer the left nav to the step's section (progressive
  // disclosure for first-run users; the user's persisted last-active section
  // is untouched, see the spotlight effect below).
  useEffect(() => {
    setActiveOverlay(id)
  }, [id, setActiveOverlay])

  // Re-arm the guide after HARD navigations: connecting Twitch/YouTube/Kick
  // is a full-window OAuth redirect back to this page, which resets the
  // in-memory store. A flag-null, non-impersonated user in the editor is
  // mid-onboarding by construction (no zero-overlay guard here — they're
  // editing an overlay), so the guide continues where the derived steps say.
  const startOnboarding = useOnboardingStore((s) => s.start)
  useEffect(() => {
    if (!user || user.impersonating) return
    if (user.onboarding_completed_at !== null) return
    startOnboarding('auto')
  }, [user, startOnboarding])

  const onboardingActiveStep = onboardingActive
    ? deriveSteps({
        overlayCount: 1,
        sourceCount: sources.length,
        themeId: themeId || null,
        obsCopied: obsCopiedStep,
      }).find((s) => s.active)?.id
    : undefined
  const spotlight: SpotlightSection | null = !onboardingActive
    ? null
    : (spotlightOverride ??
      (onboardingActiveStep === 'connect_source'
        ? 'sources'
        : onboardingActiveStep === 'choose_theme'
          ? 'theme'
          : null))

  // While the guide is active, steer the nav to the spotlighted section. This
  // is a one-shot navigation, not a lock: the user can still browse other
  // sections, and each step change (or "Show me" click) re-steers. The user's
  // persisted last-active section is untouched — spotlight-driven activations
  // are not written to localStorage. Implemented as setState-during-render
  // (React's "adjusting state when props change" pattern) so the redirected
  // section renders in the same pass instead of flashing the old one.
  const [lastSpotlight, setLastSpotlight] = useState<typeof spotlight>(null)
  if (spotlight !== lastSpotlight) {
    setLastSpotlight(spotlight)
    if (spotlight !== null) {
      setActiveSection(SPOTLIGHT_SECTION[spotlight])
    }
  }

  function handleSelectSection(section: EditorSectionId): void {
    setSpotlightOverride(null)
    setActiveSection(section)
    try {
      localStorage.setItem(ACTIVE_SECTION_STORAGE_KEY, section)
    } catch {
      // localStorage unavailable — the selection just won't persist
    }
  }

  function handleSearchNavigate(section: EditorSectionId, anchorId?: string): void {
    handleSelectSection(section)
    setPendingAnchor(anchorId ?? null)
  }

  // After a search jump renders the target section, scroll to and flash the
  // anchored control; a control inside an AdvancedDisclosure gets its
  // <details> forced open first so the target is actually on screen.
  useEffect(() => {
    if (pendingAnchor === null) return
    const frame = requestAnimationFrame(() => {
      const el = panelRef.current?.querySelector<HTMLElement>(
        `[data-setting-anchor="${pendingAnchor}"]`
      )
      if (el !== null && el !== undefined) {
        const details = el.closest('details')
        if (details !== null) details.open = true
        el.scrollIntoView({ behavior: 'smooth', block: 'center' })
        el.classList.add('setting-flash')
        window.setTimeout(() => el.classList.remove('setting-flash'), 2000)
      }
      setPendingAnchor(null)
    })
    return () => cancelAnimationFrame(frame)
  }, [pendingAnchor, activeSection])

  function handleSpotlightSection(section: SpotlightSection) {
    setSpotlightOverride(section)
    // Let the section render land before scrolling the panel into view.
    requestAnimationFrame(() => {
      panelRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
  }

  // --- Visual appearance settings state ---
  const [visualSettings, setVisualSettings] = useState<Partial<VisualSettings>>({})
  // Server-resolved `bubble_colors` gate (ADR-0008). Ships open, so this stays
  // false unless the gate is flipped to premium in the admin UI — at which point
  // the controls lock without a deploy.
  const [bubbleColorsLocked, setBubbleColorsLocked] = useState(false)
  const [iframeVisibilityDefaults, setIframeVisibilityDefaults] = useState<Partial<VisualSettings>>(
    {}
  )
  const [parsedThemeSettings, setParsedThemeSettings] = useState<Partial<VisualSettings>>({})
  const [showThemeConfirm, setShowThemeConfirm] = useState(false)
  const [pendingTheme, setPendingTheme] = useState<Theme | null>(null)

  // --- Iframe ref for live preview communication ---
  const iframeRef = useRef<HTMLIFrameElement | null>(null)

  // --- Latest-value ref for visualSettings (avoids stale closure in the
  // preview handshakes); kept current by the mirroring effect below. ---
  const visualSettingsRef = useRef<Partial<VisualSettings>>({})

  // --- Filter settings state (Phase 11) ---
  const [filterSettings, setFilterSettings] = useState<FilterSettings>({})
  const filterSettingsRef = useRef<FilterSettings>({})

  // --- Sound settings state (Phase 12) ---
  const [soundSettings, setSoundSettings] = useState<Partial<DisplaySettings>>({})
  const soundSettingsRef = useRef<Partial<DisplaySettings>>({})

  // --- TTS settings state (Phase 13 — Plans 01 & 03) ---
  // `ttsSettings` holds the tts_* fields that live in display_settings (persisted
  // via the existing updateConfig flow). `hasElevenLabsConfig` + `obsUrl` come
  // from the separate GET /tts-config endpoint (Plan 02) — the ElevenLabs key
  // and voice_id are NEVER in display_settings.
  const [ttsSettings, setTtsSettings] = useState<Partial<DisplaySettings>>({})
  const ttsSettingsRef = useRef<Partial<DisplaySettings>>({})
  const [hasElevenLabsConfig, setHasElevenLabsConfig] = useState(false)
  const [obsUrl, setObsUrl] = useState<string | undefined>(undefined)
  // Persisted ElevenLabs voice_id (Issue #276). Lives outside display_settings
  // because the voice_id column is on overlay_tts_configs, not the overlay
  // configs blob. Drives TTSGroup's picker initial value and the visibility of
  // the "Save voice" button.
  const [elevenLabsVoiceId, setElevenLabsVoiceId] = useState<string | undefined>(undefined)

  // --- OBS URL copy state ---
  const [copiedObs, setCopiedObs] = useState(false)
  const [copiedDock, setCopiedDock] = useState(false)

  // --- Discord relay state ---
  const [relayExpandedSourceId, setRelayExpandedSourceId] = useState<string | null>(null)
  const [streamSelectExpandedSourceId, setStreamSelectExpandedSourceId] = useState<string | null>(
    null
  )

  // --- Share overlay state ---
  const [showShareModal, setShowShareModal] = useState(false)
  const [showPremiumRequired, setShowPremiumRequired] = useState(false)
  const [shareRecipient, setShareRecipient] = useState('')
  const [shareLoading, setShareLoading] = useState(false)
  const shareInputRef = useRef<HTMLInputElement>(null)

  // --- Clone / Reset Overlay ID state ---
  const [isCloning, setIsCloning] = useState(false)
  const [showResetConfirm, setShowResetConfirm] = useState(false)
  const [isResetting, setIsResetting] = useState(false)

  // Mirror the four settings slices into the refs the preview handshakes read.
  //
  // The two handshake paths — `handleIframeReady` (handed to the preview iframe
  // as a ref callback) and the `EMBED_READY` listener — must post the settings
  // the user has NOW, but neither may take the settings as a dependency: a new
  // `handleIframeReady` identity makes React re-attach the iframe ref, which
  // re-runs the getComputedStyle probe and its setState on every keystroke, and
  // a new listener identity re-binds the window handler just as often.
  //
  // The mirroring happens in an effect rather than during render because a
  // render may be discarded or replayed under concurrent rendering, so writing
  // a ref there can publish a value that was never committed. Both readers run
  // from events after commit, so the one-commit lag this introduces is not
  // observable to them.
  useEffect(() => {
    visualSettingsRef.current = visualSettings
    filterSettingsRef.current = filterSettings
    soundSettingsRef.current = soundSettings
    ttsSettingsRef.current = ttsSettings
  }, [visualSettings, filterSettings, soundSettings, ttsSettings])

  // --- sendFilterSettingsToIframe: post filter settings to the embed iframe (Phase 11) ---
  const sendFilterSettingsToIframe = useCallback((settings: FilterSettings) => {
    iframeRef.current?.contentWindow?.postMessage(
      { type: 'FILTER_SETTINGS_UPDATE', filterSettings: settings },
      '*'
    )
  }, [])

  // --- sendSoundSettingsToIframe: post sound settings to the embed iframe (Phase 12) ---
  const sendSoundSettingsToIframe = useCallback((settings: Partial<DisplaySettings>) => {
    iframeRef.current?.contentWindow?.postMessage(
      { type: 'SOUND_SETTINGS_UPDATE', soundSettings: settings },
      '*'
    )
  }, [])

  // --- sendTtsSettingsToIframe: post TTS settings to the embed iframe (Phase 13 D-22) ---
  // No debounce — fires on every change so the editor preview reflects the tuning
  // immediately. Only the 20 tts_* display_settings fields travel here; the
  // ElevenLabs runtime (endpoint/token/voiceId) is loaded by the iframe itself
  // via GET /tts-config and is preserved across these postMessages.
  const sendTtsSettingsToIframe = useCallback((settings: Partial<DisplaySettings>) => {
    iframeRef.current?.contentWindow?.postMessage(
      {
        type: 'TTS_SETTINGS_UPDATE',
        ttsSettings: {
          tts_enabled: settings.tts_enabled,
          tts_provider: settings.tts_provider,
          tts_volume: settings.tts_volume,
          tts_voice_uri: settings.tts_voice_uri,
          tts_rate: settings.tts_rate,
          tts_pitch: settings.tts_pitch,
          tts_filter_mode: settings.tts_filter_mode,
          tts_sample_rate: settings.tts_sample_rate,
          tts_max_queue: settings.tts_max_queue,
          tts_messages_per_minute: settings.tts_messages_per_minute,
          tts_user_cooldown_seconds: settings.tts_user_cooldown_seconds,
          tts_staleness_seconds: settings.tts_staleness_seconds,
          tts_priority_events: settings.tts_priority_events,
          tts_priority_bits_min: settings.tts_priority_bits_min,
          tts_read_username: settings.tts_read_username,
          tts_read_platform: settings.tts_read_platform,
          tts_max_message_chars: settings.tts_max_message_chars,
          tts_skip_emote_only: settings.tts_skip_emote_only,
          tts_skip_links: settings.tts_skip_links,
          tts_enabled_platforms: settings.tts_enabled_platforms,
        },
      },
      '*'
    )
  }, [])

  // --- sendCssToIframe: post CSS generated from visualSettings to the iframe ---
  const sendCssToIframe = useCallback((settings: Partial<VisualSettings>) => {
    const css = visualSettingsToCss(settings)
    iframeRef.current?.contentWindow?.postMessage({ type: 'VISUAL_CSS_UPDATE', css }, '*')
    // Send non-CSS visual settings (platform badge position/style, indicators toggle)
    iframeRef.current?.contentWindow?.postMessage(
      {
        type: 'VISUAL_SETTINGS_UPDATE',
        settings: {
          platformBadgePosition: settings.platformBadgePosition,
          platformBadgeStyle: settings.platformBadgeStyle,
          showPlatformBadge: settings.showPlatformBadge,
          showPlatformIndicators: settings.showPlatformIndicators,
          messageAnimation: settings.messageAnimation,
          // The palette's CSS rules key on a per-row attribute the preview has
          // to render itself, so it needs the list, not just the CSS.
          bubblePalette: settings.bubblePalette,
        },
      },
      '*'
    )
  }, [])

  // --- handleVisualSettingsChange: merge patch, update state, send CSS ---
  const handleVisualSettingsChange = useCallback(
    (patch: Partial<VisualSettings>) => {
      setVisualSettings((prev) => {
        const next = { ...prev, ...patch }
        sendCssToIframe(next)
        return next
      })
    },
    [sendCssToIframe]
  )

  // --- handleFilterSettingsChange: merge patch, update state, send to iframe immediately (D-07 WYSIWYG) ---
  const handleFilterSettingsChange = useCallback(
    (patch: Partial<FilterSettings>) => {
      setFilterSettings((prev) => {
        const next = { ...prev, ...patch }
        sendFilterSettingsToIframe(next)
        return next
      })
    },
    [sendFilterSettingsToIframe]
  )

  // --- handleSoundSettingsChange: merge patch, update state, send to iframe (Phase 12) ---
  const handleSoundSettingsChange = useCallback(
    (patch: Partial<DisplaySettings>) => {
      setSoundSettings((prev) => {
        const next = { ...prev, ...patch }
        sendSoundSettingsToIframe(next)
        return next
      })
    },
    [sendSoundSettingsToIframe]
  )

  // --- handleTTSSettingsChange: merge patch, update state, send to embed iframe (Phase 13 D-22) ---
  const handleTTSSettingsChange = useCallback(
    (patch: Partial<DisplaySettings>) => {
      setTtsSettings((prev) => {
        const next = { ...prev, ...patch }
        sendTtsSettingsToIframe(next)
        return next
      })
    },
    [sendTtsSettingsToIframe]
  )

  const handleSaveTTSKey = useCallback(
    async (apiKey: string, voiceId: string): Promise<void> => {
      await overlaysApi.saveTTSKey(id, apiKey, voiceId)
      // Refresh metadata so the OBS URL + Test button render.
      const meta = await overlaysApi.getTTSConfig(id)
      setHasElevenLabsConfig(meta.has_elevenlabs_config)
      setObsUrl(meta.obs_url)
      setElevenLabsVoiceId(meta.voice_id)
    },
    [id]
  )

  // Issue #276 — voice-only update path. Persists to PATCH /tts-config/voice
  // and refreshes local state so the picker no longer shows the "Save voice"
  // button (pickedVoiceId === savedVoiceId).
  const handleSaveTTSVoice = useCallback(
    async (voiceId: string): Promise<void> => {
      await overlaysApi.saveTTSVoice(id, voiceId)
      setElevenLabsVoiceId(voiceId)
    },
    [id]
  )

  const handleRemoveTTSKey = useCallback(async (): Promise<void> => {
    await overlaysApi.removeTTSKey(id)
    setHasElevenLabsConfig(false)
    setObsUrl(undefined)
    setElevenLabsVoiceId(undefined)
  }, [id])

  const handleRotateTTSToken = useCallback(async (): Promise<{ obsUrl: string }> => {
    const result = await overlaysApi.rotateTTSToken(id)
    setObsUrl(result.obsUrl)
    return result
  }, [id])

  const handleTestTTSKey = useCallback(async (): Promise<TestKeyResult> => {
    const result = await overlaysApi.testTTSKey(id)
    if (result.ok && result.audioBlob) {
      // Autoplay is user-initiated (Test key click), so the browser grants playback.
      try {
        const audio = new Audio(URL.createObjectURL(result.audioBlob))
        await audio.play()
        audio.onended = (): void => {
          URL.revokeObjectURL(audio.src)
        }
      } catch {
        // Swallow — the quota number still renders, and the caller shows
        // verbose errors for non-ok cases.
      }
    }
    return result
  }, [id])

  const handleFetchTTSVoices = useCallback(async (): Promise<ElevenLabsVoice[]> => {
    return overlaysApi.getTTSVoices(id)
  }, [id])

  const handlePreviewTTSVoices = useCallback(
    async (apiKey: string): Promise<ElevenLabsVoice[]> => {
      return overlaysApi.previewTTSVoices(id, apiKey)
    },
    [id]
  )

  // --- sendCustomCssToIframe: post custom/theme CSS to the embed preview ---
  const sendCustomCssToIframe = useCallback((css: string) => {
    iframeRef.current?.contentWindow?.postMessage({ type: 'CUSTOM_CSS_UPDATE', css }, '*')
  }, [])

  // --- handleCustomCssChange: user typed in the Advanced CSS editor ---
  // WYSIWYG for raw CSS: debounce-push to the preview as they type so edits show
  // live (ADR-0043). Guarded so a mid-edit unclosed brace can't blank the preview.
  const handleCustomCssChange = useCallback(
    (value: string) => {
      setCustomCss(value)
      if (customCssDebounceRef.current) clearTimeout(customCssDebounceRef.current)
      customCssDebounceRef.current = setTimeout(() => {
        const opens = (value.match(/{/g) ?? []).length
        const closes = (value.match(/}/g) ?? []).length
        // Only push when braces balance; otherwise keep the last good preview and
        // let Monaco's inline markers guide the user until the rule is closed.
        if (opens === closes) sendCustomCssToIframe(value)
        // Recompute the storage mode (linked / diff / fork) so the status pill tells
        // the user whether this overlay still tracks theme updates.
        void computeThemeCssDiff(pristineThemeCss, value).then((d) => setCustomCssMode(d.mode))
      }, 300)
    },
    [sendCustomCssToIframe, pristineThemeCss]
  )

  // Clear the pending debounce on unmount so it can't setState after teardown.
  useEffect(
    () => () => {
      if (customCssDebounceRef.current) clearTimeout(customCssDebounceRef.current)
    },
    []
  )

  // --- handleResetCustomCss: revert the editor to the bundled theme (re-link) ---
  // Restores the pristine theme CSS (or empties the box when the overlay has no
  // theme). Since customCss then equals pristineThemeCss, the overlay is no longer
  // "forked" and starts receiving bundled theme updates again on save.
  const handleResetCustomCss = useCallback(() => {
    setCustomCss(pristineThemeCss)
    setCssIssues([])
    setCustomCssMode('linked')
    sendCustomCssToIframe(pristineThemeCss)
  }, [pristineThemeCss, sendCustomCssToIframe])

  // --- applyThemeImmediately: reference the theme by id + apply its parsed
  // settings, and PRELOAD its full CSS into the Advanced editor (ADR-0043) so it
  // is visible and directly editable. The overlay stays linked to the bundled
  // theme (theme_id) — the preloaded copy is persisted to custom_css only if the
  // user edits it (fork-on-save), so untouched themed overlays keep receiving
  // theme fixes on deploy. ---
  const applyThemeImmediately = useCallback(
    (theme: Theme) => {
      const parsed = parseCssToVisualSettings(theme.css)
      setThemeId(theme.id)
      setPristineThemeCss(theme.css)
      setCustomCss(theme.css)
      setCustomCssMode('linked')
      setVisualSettings(parsed)
      setParsedThemeSettings(parsed)
      sendCssToIframe(parsed)
      sendCustomCssToIframe(theme.css)
    },
    [sendCssToIframe, sendCustomCssToIframe]
  )

  // --- handleResetToTheme: restore visualSettings to parsedThemeSettings (or {}) ---
  const handleResetToTheme = useCallback(() => {
    setVisualSettings(parsedThemeSettings)
    sendCssToIframe(parsedThemeSettings)
  }, [parsedThemeSettings, sendCssToIframe])

  // --- handleThemeApply: apply a marketplace theme from ThemeContent ---
  // Prompts for confirmation if visual settings are already customized.
  function handleThemeApply(theme: Theme): void {
    const hasExisting = Object.keys(visualSettings).length > 0
    if (hasExisting) {
      setPendingTheme(theme)
      setShowThemeConfirm(true)
    } else {
      applyThemeImmediately(theme)
    }
  }

  // --- EMBED_READY: re-send CSS, filter settings, sound settings, and TTS settings when embed page signals its listener is registered ---
  useEffect(() => {
    const handleEmbedReady = (event: MessageEvent) => {
      // audit #23: validate the sender origin, mirroring the embed-side M11 check.
      // The preview iframe is always same-origin (components/SplitView.tsx), so any
      // cross-origin EMBED_READY is not from our embed and must be ignored.
      if (event.origin !== window.location.origin) return
      if (event.data?.type !== 'EMBED_READY') return
      sendCssToIframe(visualSettingsRef.current)
      // Also send current filter settings to the embed on ready
      sendFilterSettingsToIframe(filterSettingsRef.current)
      // Also send current sound settings to the embed on ready
      sendSoundSettingsToIframe(soundSettingsRef.current)
      sendTtsSettingsToIframe(ttsSettingsRef.current)
    }
    window.addEventListener('message', handleEmbedReady)
    return () => window.removeEventListener('message', handleEmbedReady)
  }, [
    sendCssToIframe,
    sendFilterSettingsToIframe,
    sendSoundSettingsToIframe,
    sendTtsSettingsToIframe,
  ])

  // --- handleIframeReady: store iframe ref, send initial CSS, and query visibility defaults ---
  const handleIframeReady = useCallback(
    (iframe: HTMLIFrameElement) => {
      iframeRef.current = iframe
      sendCssToIframe(visualSettingsRef.current)
      const doc = iframe.contentDocument ?? iframe.contentWindow?.document
      if (doc) {
        const style = getComputedStyle(doc.documentElement)
        const visFields: Array<[keyof VisualSettings, string]> = [
          ['showAvatars', '--chat-show-avatars'],
          ['showBadges', '--chat-show-badges'],
          ['showTimestamps', '--chat-show-timestamps'],
          ['showPlatformBadge', '--chat-show-platform-badge'],
          ['showEmotes', '--chat-show-emotes'],
          ['showUsername', '--chat-show-username'],
        ]
        const defaults: Partial<VisualSettings> = {}
        for (const [field, cssVar] of visFields) {
          const v = style.getPropertyValue(cssVar).trim()
          if (v) {
            ;(defaults as Record<string, string>)[field] = v
          }
        }
        setIframeVisibilityDefaults(defaults)
      }
    },
    [sendCssToIframe]
  )

  // Load overlay, sources, accepted shares and config
  // Note: `router` is intentionally excluded from deps — it is only used for
  // the redirect guard which ProtectedRoute already handles. Including router
  // causes loadData to re-run whenever the Next.js router object reference changes
  // (e.g. during API proxy processing), which would overwrite user-set extension
  // overlay state with stale DB data.
  useEffect(() => {
    if (!user) {
      router.push('/')
      return
    }

    let cancelled = false

    const loadData = async () => {
      try {
        const overlayData = await overlaysApi.get(id)
        if (cancelled) return
        setOverlay(overlayData)
        setIsPublicForViewers(overlayData.is_public_for_viewers)

        try {
          const sourcesData = await overlaysApi.getSources(id)
          if (cancelled) return
          setSources(sourcesData)
        } catch {
          if (!cancelled) setSources([])
        }

        // Load accepted shares — non-critical
        try {
          const sharesData = await sharesApi.getAcceptedShares()
          if (!cancelled) setAcceptedShares(sharesData)
        } catch {
          // Non-critical
        }

        // Load overlay config for customization defaults
        try {
          const config = await overlaysApi.getConfig(id)
          if (cancelled) return
          const display = config.display_settings || {}

          if (typeof display.max_messages === 'number') setMaxMessages(display.max_messages)
          if (typeof display.message_duration === 'number')
            setMessageDuration(display.message_duration)
          if (typeof display.disable_message_fade === 'boolean')
            setDisableMessageFade(display.disable_message_fade)
          setInvertMessageOrder(display.invert_message_order === true)
          setFeedAnchor(parseFeedAnchor(display.feed_anchor))

          if (typeof config.enable_7tv === 'boolean') setEnable7tv(config.enable_7tv)
          if (typeof config.enable_bttv === 'boolean') setEnableBttv(config.enable_bttv)
          if (typeof config.enable_ffz === 'boolean') setEnableFfz(config.enable_ffz)
          if (typeof config.seventv_emote_set_id === 'string') {
            const saved = config.seventv_emote_set_id
            setSeventvOverrideInput(saved)
            setSeventvOverrideSavedID(saved)
            // Auto-resolve the saved set so the editor can show "Currently
            // active: <name> (N emotes)" instead of a raw ULID — otherwise
            // the user has no way to tell what's attached.
            if (saved.trim() !== '') {
              overlaysApi
                .resolveSevenTV(id, saved)
                .then((result) => {
                  if (cancelled) return
                  setSeventvSavedDescriptor({
                    name: result.name,
                    emoteCount: result.emote_count,
                  })
                })
                .catch(() => {
                  // Best-effort: if 7TV is unreachable or the saved ID is
                  // stale, fall back silently to showing just the ID.
                  if (cancelled) return
                  setSeventvSavedDescriptor(null)
                })
            } else {
              setSeventvSavedDescriptor(null)
            }
          }

          const savedCss = config.custom_css || ''
          setBubbleColorsLocked(config.bubble_colors_locked === true)

          const tid = typeof config.theme_id === 'string' ? config.theme_id : ''
          setThemeId(tid)
          // Resolve the bundled theme CSS for this overlay: preloaded into the
          // Advanced editor and used as the "reset to theme" baseline (ADR-0043).
          const themeCss = tid ? (getBundledTheme(tid)?.css ?? '') : ''
          setPristineThemeCss(themeCss)
          // Reconstruct the full editable CSS from the stored delta/fork (ADR-0043):
          // 'linked' → the bundled theme (preloaded, not a blank box), 'diff' → the
          // theme merged with the user's saved changes, 'fork' → the saved copy as-is.
          const { editor: editorCss, mode: cssMode } = await reconstructEditorCss(
            themeCss,
            savedCss
          )
          setCustomCss(editorCss)
          setCustomCssMode(cssMode)

          // Parse CSS → parsedThemeSettings so "Reset to theme defaults" works after
          // save+reload. Prefer the bundled theme CSS (the actual defaults), else the
          // saved override.
          const savedVisual = config.visual_settings as Partial<VisualSettings> | null
          const parseSource = themeCss.trim() ? themeCss : savedCss
          const parsedFromCss = parseSource.trim() ? parseCssToVisualSettings(parseSource) : {}
          if (parseSource.trim()) {
            setParsedThemeSettings(parsedFromCss)
          }

          // Migrate platform badge settings from display_settings to visual_settings
          const platformBadgeDefaults: Partial<VisualSettings> = {}
          if (typeof display.show_platform_badge === 'boolean') {
            platformBadgeDefaults.showPlatformBadge = display.show_platform_badge
              ? 'inline'
              : 'none'
          }
          if (
            display.platform_badge_position === 'before' ||
            display.platform_badge_position === 'after'
          ) {
            platformBadgeDefaults.platformBadgePosition = display.platform_badge_position
          }
          if (display.platform_badge_style === 'text' || display.platform_badge_style === 'icon') {
            platformBadgeDefaults.platformBadgeStyle = display.platform_badge_style
          }

          const merged: Partial<VisualSettings> = {
            ...platformBadgeDefaults,
            ...parsedFromCss,
            ...(savedVisual ?? {}),
            // migrate legacy font_size to visualSettings.fontSize if not already present
            fontSize:
              savedVisual?.fontSize ??
              parsedFromCss.fontSize ??
              (typeof display.font_size === 'number' ? `${display.font_size}px` : undefined),
          }
          setVisualSettings(merged)
          sendCssToIframe(merged)

          if (config.filter_settings) {
            setFilterSettings(config.filter_settings)
          }

          if (config.display_settings) {
            const d = config.display_settings
            const loaded: Partial<DisplaySettings> = {}
            if (typeof d.notification_sound_enabled === 'boolean')
              loaded.notification_sound_enabled = d.notification_sound_enabled
            if (typeof d.notification_sound_preset === 'string')
              loaded.notification_sound_preset = d.notification_sound_preset
            if (typeof d.notification_sound_volume === 'number')
              loaded.notification_sound_volume = d.notification_sound_volume
            if (typeof d.notification_sound_cooldown === 'number')
              loaded.notification_sound_cooldown = d.notification_sound_cooldown
            if (typeof d.notification_sound_url === 'string')
              loaded.notification_sound_url = d.notification_sound_url
            setSoundSettings(loaded)
          }

          if (config.display_settings) {
            const d = config.display_settings
            const tts: Partial<DisplaySettings> = {}
            if (typeof d.tts_enabled === 'boolean') tts.tts_enabled = d.tts_enabled
            if (d.tts_provider === 'browser' || d.tts_provider === 'elevenlabs')
              tts.tts_provider = d.tts_provider
            if (typeof d.tts_volume === 'number') tts.tts_volume = d.tts_volume
            if (typeof d.tts_voice_uri === 'string') tts.tts_voice_uri = d.tts_voice_uri
            if (typeof d.tts_rate === 'number') tts.tts_rate = d.tts_rate
            if (typeof d.tts_pitch === 'number') tts.tts_pitch = d.tts_pitch
            if (
              d.tts_filter_mode === 'all' ||
              d.tts_filter_mode === 'sample' ||
              d.tts_filter_mode === 'priority_only'
            ) {
              tts.tts_filter_mode = d.tts_filter_mode
            }
            if (typeof d.tts_sample_rate === 'number') tts.tts_sample_rate = d.tts_sample_rate
            if (typeof d.tts_max_queue === 'number') tts.tts_max_queue = d.tts_max_queue
            if (typeof d.tts_messages_per_minute === 'number')
              tts.tts_messages_per_minute = d.tts_messages_per_minute
            if (typeof d.tts_user_cooldown_seconds === 'number')
              tts.tts_user_cooldown_seconds = d.tts_user_cooldown_seconds
            if (typeof d.tts_staleness_seconds === 'number')
              tts.tts_staleness_seconds = d.tts_staleness_seconds
            if (typeof d.tts_priority_events === 'boolean')
              tts.tts_priority_events = d.tts_priority_events
            if (typeof d.tts_priority_bits_min === 'number')
              tts.tts_priority_bits_min = d.tts_priority_bits_min
            if (typeof d.tts_read_username === 'boolean')
              tts.tts_read_username = d.tts_read_username
            if (typeof d.tts_read_platform === 'boolean')
              tts.tts_read_platform = d.tts_read_platform
            if (typeof d.tts_max_message_chars === 'number')
              tts.tts_max_message_chars = d.tts_max_message_chars
            if (typeof d.tts_skip_emote_only === 'boolean')
              tts.tts_skip_emote_only = d.tts_skip_emote_only
            if (typeof d.tts_skip_links === 'boolean') tts.tts_skip_links = d.tts_skip_links
            if (Array.isArray(d.tts_enabled_platforms)) {
              tts.tts_enabled_platforms = d.tts_enabled_platforms.filter(
                (p: unknown): p is string => typeof p === 'string'
              )
            }
            setTtsSettings(tts)
          }

          // Load ElevenLabs config metadata (has-key + OBS URL).
          // Non-fatal — a 404 on first edit or a non-premium user simply leaves
          // the Advanced block in its unsaved-empty state.
          try {
            const meta: TTSConfigMetadata = await overlaysApi.getTTSConfig(id)
            if (!cancelled) {
              setHasElevenLabsConfig(meta.has_elevenlabs_config)
              setObsUrl(meta.obs_url)
              setElevenLabsVoiceId(meta.voice_id)
            }
          } catch (e) {
            console.warn('[OverlayEditor] getTTSConfig failed:', e)
          }
        } catch (err) {
          console.warn('[OverlayEditor] Failed to load config', err)
        } finally {
          if (!cancelled) setConfigLoaded(true)
        }
      } catch {
        if (!cancelled) setOverlay(null)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    }

    loadData()

    return () => {
      cancelled = true
    }
  }, [id, user]) // eslint-disable-line react-hooks/exhaustive-deps

  // Real-time owner notifications (e.g. share_revoked) over a self-healing
  // authenticated socket. H3 cookie auth: the owner overlay WS handshake is
  // same-origin, so the browser sends the httpOnly access cookie automatically
  // and the gateway authenticates without a JS-readable token (audit H3 / Task 11b).
  // The previous inline socket had no reconnect and no liveness detection, so a
  // single drop — clean or half-open — silently killed these notifications for
  // the rest of the session; useNotificationSocket reconnects with backoff and
  // detects half-open paths via a heartbeat watchdog.
  useNotificationSocket(id, undefined, (envelope) => {
    if (envelope.type === 'share_revoked') {
      const data = envelope.data as { revoked_by_username?: string } | undefined
      const revoker =
        data?.revoked_by_username || t('overlayEditor.toasts.shareRevokedUnknownRevoker')
      toastManager.add({
        title: t('overlayEditor.toasts.shareRevoked'),
        description: t('overlayEditor.toasts.shareRevokedBody', { revoker }),
        type: 'error',
      })
      overlaysApi.getSources(id).then(setSources).catch(console.error)
    }
  })

  // Handle OAuth callback query params (source_added / error)
  useEffect(() => {
    const sourceAdded = searchParams.get('source_added')
    const error = searchParams.get('error')

    if (sourceAdded) {
      toastManager.add({
        title: t('overlayEditor.toasts.sourceAdded'),
        description: t('overlayEditor.toasts.oauthSourceAddedBody', { platform: sourceAdded }),
        type: 'success',
      })
      overlaysApi.getSources(id).then(setSources).catch(console.error)
      window.history.replaceState({}, '', `/overlays/${id}`)
    } else if (error === 'youtube_permission_required') {
      trackEvent('source_add_failed', { platform: 'youtube', reason: 'permission_required' })
      toastManager.add({
        title: t('overlayEditor.toasts.youtubePermissionRequired'),
        description: t('overlayEditor.toasts.youtubePermissionRequiredBody'),
        type: 'error',
      })
    } else if (error === 'youtube_no_channel') {
      trackEvent('source_add_failed', { platform: 'youtube', reason: 'no_channel' })
      toastManager.add({
        title: t('overlayEditor.toasts.youtubeNoChannel'),
        description: t('overlayEditor.toasts.youtubeNoChannelBody'),
        type: 'error',
      })
    } else if (error === 'failed_to_add_source') {
      trackEvent('source_add_failed', { reason: 'failed' })
      toastManager.add({
        title: t('overlayEditor.toasts.oauthSourceAddFailed'),
        description: t('common.toast.tryAgain'),
        type: 'error',
      })
    }

    // Strip a handled add-source ?error= from the URL (mirrors the source_added
    // strip above) so a reload or effect re-run doesn't re-show the toast and
    // re-fire source_add_failed, double-counting the failure in the funnel.
    if (
      error === 'youtube_permission_required' ||
      error === 'youtube_no_channel' ||
      error === 'failed_to_add_source'
    ) {
      window.history.replaceState({}, '', `/overlays/${id}`)
    }
  }, [id, searchParams])

  // --- Source handlers ---

  async function handleRemoveSource(sourceId: string) {
    try {
      await overlaysApi.removeSource(id, sourceId)
      setSources((prev) => prev.filter((s) => s.id !== sourceId))
      toastManager.add({ title: t('overlayEditor.toasts.sourceRemoved'), type: 'success' })
    } catch {
      toastManager.add({
        title: t('overlayEditor.toasts.sourceRemoveFailed'),
        description: t('common.toast.tryAgain'),
        type: 'error',
      })
    }
  }

  // Starts the Twitch add-source OAuth reflow so the streamer can (re)grant the EventSub
  // chat scopes for their own channel. The add-source endpoint now requests
  // user:read:chat + user:bot + channel:bot; once granted, the channel moves to the
  // EventSub listener on the next sync.
  async function handleReconnectTwitchChat() {
    const result = await startAddSourceReflow(`/api/v1/auth/twitch/add-source/${id}`)
    if (result.kind === 'redirect') {
      safeExternalRedirect(result.authUrl)
      return
    }
    if (result.kind === 'added') {
      // Already holds the chat scopes — nothing to re-grant; refresh so the
      // now-migrated source reflects its EventSub state.
      void overlaysApi.getSources(id).then(setSources).catch(console.error)
      toastManager.add({ title: t('overlayEditor.toasts.twitchChatConnected'), type: 'success' })
      return
    }
    toastManager.add({
      title: t('overlayEditor.toasts.twitchChatReconnectFailed'),
      description: result.message,
      type: 'error',
    })
  }

  async function handleAddTikTokSource(username: string) {
    try {
      const source = await overlaysApi.addSource(id, {
        platform: 'tiktok',
        channel_id: username,
      })
      setSources((prev) => [...prev, source])
      trackEvent('source_configured', { platform: 'tiktok' })
      toastManager.add({ title: t('overlayEditor.toasts.tiktokSourceAdded'), type: 'success' })
    } catch {
      toastManager.add({
        title: t('overlayEditor.toasts.tiktokSourceAddFailed'),
        description: t('overlayEditor.toasts.tiktokSourceAddFailedBody'),
        type: 'error',
      })
    }
  }

  async function handleAddManual(platform: string, channelId: string) {
    try {
      const source = await overlaysApi.addSource(id, {
        platform: platform as ChatSource['platform'],
        channel_id: channelId,
      })
      setSources((prev) => [...prev, source])
      trackEvent('source_configured', { platform })
      toastManager.add({ title: t('overlayEditor.toasts.sourceAdded'), type: 'success' })
    } catch {
      toastManager.add({
        title: t('overlayEditor.toasts.manualSourceAddFailed'),
        description: t('overlayEditor.toasts.manualSourceAddFailedBody'),
        type: 'error',
      })
    }
  }

  async function handleAddSharedOverlay(share: AcceptedShare) {
    try {
      await overlaysApi.addSource(id, {
        platform: 'shared_overlay',
        channel_id: share.sender_overlay_id,
        channel_name: `${share.sender_display_name}'s overlay`,
      })
      const updated = await overlaysApi.getSources(id)
      setSources(updated)
      trackEvent('source_configured', { platform: 'shared_overlay' })
      toastManager.add({
        title: t('overlayEditor.toasts.sharedOverlayAdded'),
        description: t('overlayEditor.toasts.sharedOverlayAddedBody', {
          sender: share.sender_display_name,
        }),
        type: 'success',
      })
    } catch {
      toastManager.add({
        title: t('overlayEditor.toasts.sharedOverlayAddFailed'),
        description: t('common.toast.tryAgain'),
        type: 'error',
      })
    }
  }

  // --- Discord relay ---

  function handleRelayConfigSaved(updated: ChatSource) {
    setSources((prev) => prev.map((s) => (s.id === updated.id ? updated : s)))
  }

  // --- OBS URL ---

  function handleCopyObsUrl() {
    const url = `${window.location.origin}/overlay/${id}`
    navigator.clipboard.writeText(url).then(() => {
      trackEvent('obs_url_copied', { surface: 'editor' })
      setCopiedObs(true)
      setTimeout(() => setCopiedObs(false), 2000)
    })
  }

  // The monitor as an OBS/Streamlabs custom browser DOCK, which is a different
  // destination from the URL above: that one is the public overlay a browser
  // source renders on stream, this one is the auth-gated moderator monitor in a
  // narrow docked panel. `dock=1` is what makes it fit one.
  function handleCopyDockUrl() {
    const url = `${window.location.origin}/overlay/${id}/view?dock=1`
    navigator.clipboard.writeText(url).then(() => {
      trackEvent('obs_url_copied', { surface: 'editor_dock' })
      setCopiedDock(true)
      setTimeout(() => setCopiedDock(false), 2000)
    })
  }

  function handleShareClick() {
    if (!user?.is_premium) {
      setShowPremiumRequired(true)
    } else {
      setShareRecipient('')
      setShowShareModal(true)
      setTimeout(() => shareInputRef.current?.focus(), 50)
    }
  }

  async function handleCloneOverlay() {
    setIsCloning(true)
    try {
      const cloned = await overlaysApi.clone(id)
      toastManager.add({ title: t('overlayEditor.toasts.cloned'), type: 'success' })
      router.push(`/overlays/${cloned.id}`)
    } catch {
      toastManager.add({ title: t('overlayEditor.toasts.cloneFailed'), type: 'error' })
    } finally {
      setIsCloning(false)
    }
  }

  async function handleConfirmResetOverlayId() {
    setIsResetting(true)
    try {
      const cloned = await overlaysApi.clone(id)
      if (overlay) {
        await overlaysApi.update(cloned.id, { name: overlay.name })
      }
      await overlaysApi.delete(id)
      toastManager.add({ title: t('overlayEditor.toasts.overlayIdReset'), type: 'success' })
      router.push(`/overlays/${cloned.id}`)
    } catch {
      toastManager.add({ title: t('overlayEditor.toasts.overlayIdResetFailed'), type: 'error' })
      setIsResetting(false)
    }
    setShowResetConfirm(false)
  }

  async function handleSendShareRequest() {
    const username = shareRecipient.trim()
    if (!username) return
    setShareLoading(true)
    try {
      await sharesApi.createRequest(username, id)
      toastManager.add({
        title: t('overlayEditor.toasts.shareRequestSent', { username }),
        type: 'success',
      })
      setShowShareModal(false)
      setShareRecipient('')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t('overlayEditor.toasts.shareRequestFailed')
      toastManager.add({ title: msg, type: 'error' })
    } finally {
      setShareLoading(false)
    }
  }

  // --- Mock message handlers ---

  function handleMockInputChange<K extends keyof MockMessageFormState>(
    field: K,
    value: MockMessageFormState[K]
  ) {
    setMockForm((prev) => ({ ...prev, [field]: value }))
  }

  const resolveMockTarget = (requestedPlatform?: ChatMessage['platform']) => {
    const preferred = sources.find((source) =>
      requestedPlatform ? source.platform === requestedPlatform : true
    )
    if (!preferred) {
      return {
        platform: requestedPlatform || 'twitch',
        channel_id: undefined,
        channel_name: undefined,
      }
    }
    return {
      platform: requestedPlatform || (preferred.platform as ChatMessage['platform']),
      channel_id: preferred.channel_id,
      channel_name: preferred.channel_name || preferred.channel_id,
    }
  }

  async function handleAddMockMessage() {
    if (!mockForm.message.trim()) return
    const target = resolveMockTarget(mockForm.platform)
    try {
      await overlaysApi.sendMockMessage(id, {
        platform: target.platform,
        channel_id: target.channel_id,
        channel_name: target.channel_name,
        text: mockForm.message,
        username:
          mockForm.username || mockForm.displayName.toLowerCase().replace(/\s+/g, '') || 'mockuser',
        display_name: mockForm.displayName || mockForm.username || 'Mock Viewer',
        avatar_url: mockForm.avatarUrl || undefined,
        color: mockForm.color || undefined,
        metadata: { mock: true, source: 'editor-form' },
      })
      setMockForm((prev) => ({ ...prev, message: '' }))
    } catch (error) {
      console.error('[Editor] Failed to send mock message:', error)
      toastManager.add({ title: t('overlayEditor.toasts.mockMessageFailed'), type: 'error' })
    }
  }

  async function handleAddSampleTranscript() {
    for (const [index, sample] of SAMPLE_MOCK_MESSAGES.entries()) {
      const target = resolveMockTarget(sample.platform)
      try {
        await overlaysApi.sendMockMessage(id, {
          platform: target.platform,
          channel_id: target.channel_id,
          channel_name: target.channel_name,
          text: sample.message.text,
          username: sample.user.username,
          display_name: sample.user.display_name,
          avatar_url: sample.user.avatar_url,
          color: sample.user.color,
          badges: sample.user.badges,
          metadata: { ...(sample.metadata || {}), mock: true, preset: true, order: index },
        })
      } catch (error) {
        console.error('[Editor] Failed to send sample message:', error)
        break
      }
    }
  }

  async function handleAddSampleEvents() {
    for (const [index, sample] of SAMPLE_EVENT_MESSAGES.entries()) {
      const target = resolveMockTarget(sample.platform)
      try {
        await overlaysApi.sendMockMessage(id, {
          platform: target.platform,
          channel_id: target.channel_id,
          channel_name: target.channel_name,
          text: sample.message.text,
          username: sample.user.username,
          display_name: sample.user.display_name,
          avatar_url: sample.user.avatar_url,
          color: sample.user.color,
          badges: sample.user.badges,
          event: sample.event,
          metadata: { ...(sample.metadata || {}), mock: true, preset: true, order: index },
        })
        await new Promise((resolve) => setTimeout(resolve, 800))
      } catch (error) {
        console.error('[Editor] Failed to send sample event:', error)
        break
      }
    }
  }

  // --- Save configuration ---

  async function handleSaveConfiguration() {
    setIsSavingConfig(true)
    setConfigAlert(null)
    try {
      // Compute what to persist for custom_css (ADR-0043): only the user's changed/
      // added declarations when they tweaked the theme (so untouched rules keep
      // receiving bundled theme updates), or a full copy when they deleted theme
      // rules (which layering can't express — that overlay detaches). See
      // theme-css-diff.ts.
      const cssDiff = await computeThemeCssDiff(pristineThemeCss, customCss)
      await overlaysApi.updateConfig(id, {
        display_settings: {
          font_size: parseInt(visualSettings.fontSize ?? '16') || 16,
          message_duration: messageDuration,
          max_messages: maxMessages,
          disable_message_fade: disableMessageFade,
          invert_message_order: invertMessageOrder,
          feed_anchor: feedAnchor,
          platform_badge_position: visualSettings.platformBadgePosition ?? 'before',
          platform_badge_style: visualSettings.platformBadgeStyle ?? 'text',
          show_platform_badge: visualSettings.showPlatformBadge !== 'none',
          notification_sound_enabled: soundSettings.notification_sound_enabled ?? false,
          notification_sound_preset: soundSettings.notification_sound_preset ?? 'chime',
          notification_sound_volume: soundSettings.notification_sound_volume ?? 0.5,
          notification_sound_cooldown: soundSettings.notification_sound_cooldown ?? 500,
          ...(soundSettings.notification_sound_url
            ? { notification_sound_url: soundSettings.notification_sound_url }
            : {}),
          // The ElevenLabs API key and voice id are deliberately absent: they
          // live in overlay_tts_configs, not in display_settings.
          ...ttsSettings,
        },
        enable_7tv: enable7tv,
        enable_bttv: enableBttv,
        enable_ffz: enableFfz,
        custom_css: cssDiff.stored,
        theme_id: themeId,
        visual_settings: visualSettings,
        filter_settings: filterSettings,
        // Send the (already-resolved) override only when the user touched it.
        // Sending an empty string clears the override server-side.
        ...(seventvOverrideInput !== seventvOverrideSavedID || seventvOverrideInput === ''
          ? { seventv_emote_set_id: seventvOverrideInput.trim() }
          : {}),
      })
      const newSavedID = seventvOverrideInput.trim()
      setSeventvOverrideSavedID(newSavedID)
      // Mirror the descriptor to whatever the user just verified, so the
      // "Currently active" line refreshes without re-resolving.
      if (newSavedID === '') {
        setSeventvSavedDescriptor(null)
      } else if (
        seventvResolveState.status === 'resolved' &&
        seventvResolveState.setID === newSavedID
      ) {
        setSeventvSavedDescriptor({
          name: seventvResolveState.name,
          emoteCount: seventvResolveState.emoteCount,
        })
      }
      // Reflect the persisted storage mode in the status pill after a successful save.
      setCustomCssMode(cssDiff.mode)
      setConfigAlert({ type: 'success', message: 'Configuration saved!' })
      trackEvent('overlay_settings_saved')
    } catch (error) {
      console.error('[Editor] Failed to save config', error)
      setConfigAlert({ type: 'error', message: 'Failed to save configuration' })
    } finally {
      setIsSavingConfig(false)
      setTimeout(() => setConfigAlert(null), 5000)
    }
  }

  async function handleSetAsExtensionOverlay() {
    // Optimistic update — show Active state immediately
    setIsPublicForViewers(true)
    try {
      const updated = await overlaysApi.update(id, { is_public_for_viewers: true })
      setOverlay(updated)
      toastManager.add({
        title: t('overlayEditor.toasts.extensionOverlaySet'),
        description: t('overlayEditor.toasts.extensionOverlaySetBody'),
        type: 'success',
      })
    } catch {
      // Roll back optimistic update on failure
      setIsPublicForViewers(false)
      toastManager.add({
        title: t('overlayEditor.toasts.extensionOverlayUpdateFailed'),
        type: 'error',
      })
    }
  }

  async function handleUnsetExtensionOverlay() {
    // Optimistic update — show inactive state immediately
    setIsPublicForViewers(false)
    try {
      const updated = await overlaysApi.update(id, { is_public_for_viewers: false })
      setOverlay(updated)
      toastManager.add({ title: t('overlayEditor.toasts.extensionOverlayUnset'), type: 'success' })
    } catch {
      // Roll back optimistic update on failure
      setIsPublicForViewers(true)
      toastManager.add({
        title: t('overlayEditor.toasts.extensionOverlayUpdateFailed'),
        type: 'error',
      })
    }
  }

  // --- Loading / error states ---

  if (isLoading) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
        <div className="flex h-[calc(100vh-60px)] items-center justify-center">
          <div className="w-64 space-y-3">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-4 w-2/3" />
          </div>
        </div>
      </div>
    )
  }

  if (!overlay) {
    return (
      <div className="min-h-screen bg-bg">
        <AppNav />
        <div className="flex h-[calc(100vh-60px)] items-center justify-center">
          <div className="text-center">
            <p className="mb-4 text-lg text-destructive">{t('overlayEditor.page.notFound')}</p>
            <Button variant="outline" onClick={() => router.push('/dashboard')}>
              {t('overlayEditor.page.returnToDashboard')}
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <SplitView overlayId={id} onIframeReady={handleIframeReady}>
        {/* Config panel content */}
        <div className="max-w-none space-y-6 p-6">
          <OnboardingChecklist
            surface="editor"
            variant="inline"
            overlayCount={1}
            sourceCount={sources.length}
            themeId={themeId || null}
            onSpotlightSection={handleSpotlightSection}
          />
          {/* 1. Header */}
          <div className="flex items-start justify-between">
            <div>
              <Button
                onClick={() => router.push('/dashboard')}
                variant="ghost"
                size="sm"
                className="mb-1 px-0"
              >
                <ChevronLeft className="size-4" />
                {t('overlayEditor.page.back')}
              </Button>
              <h1 className="text-xl font-bold text-text">{overlay.name}</h1>
              {overlay.description && (
                <p className="mt-0.5 text-sm text-text-sub">{overlay.description}</p>
              )}
            </div>
            <div className="ml-4 flex shrink-0 flex-wrap gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.open(`/overlay/${id}/view`, '_blank', 'noopener,noreferrer')}
                title={t('overlayEditor.page.monitorViewTitle')}
              >
                {t('overlayEditor.page.monitorView')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => router.push(`/overlays/${id}/events`)}
              >
                {t('overlayEditor.page.eventSettings')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => router.push(`/overlays/${id}/credits`)}
              >
                {t('overlayEditor.page.credits')}
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={isCloning}
                onClick={() => void handleCloneOverlay()}
              >
                {isCloning ? t('overlayEditor.page.cloning') : t('overlayEditor.page.clone')}
              </Button>
            </div>
          </div>

          {/* 2. The two URLs this overlay hands out, side by side because they
              are the same kind of action on different destinations: the OBS URL
              is the public overlay a browser SOURCE renders on stream, the dock
              URL is the auth-gated monitor a browser DOCK renders beside the
              mixer.

              One row of two, rather than a second stacked button-plus-link
              pair, so this block stays the height it was. Everything below it
              in this scrolling column — including the settings nav — slides
              under the sticky Save footer, and axe reports a nav item that
              straddles the footer's top edge as partially obscured (WCAG
              2.5.8). Growing the block moves that edge onto a nav item; keeping
              the height fixed leaves it in the gap above one, where it is on
              main. */}
          <div className="flex gap-2">
            <Button
              variant="outline"
              className="flex flex-1 items-center justify-center gap-2"
              onClick={handleCopyObsUrl}
            >
              <Clipboard className="size-4" />
              {copiedObs
                ? t('overlayEditor.page.copiedObsUrl')
                : t('overlayEditor.page.copyObsUrl')}
            </Button>
            <Button
              variant="outline"
              className="flex flex-1 items-center justify-center gap-2"
              onClick={handleCopyDockUrl}
            >
              <Clipboard className="size-4" />
              {copiedDock
                ? t('overlayEditor.page.copiedDockUrl')
                : t('overlayEditor.page.copyDockUrl')}
            </Button>
          </div>
          {/* Both help triggers centred on one line. They stay inline (rather
              than becoming flex items) so the line box is the height a single
              trigger occupied before, per the note above, and so a narrow
              column wraps them instead of pushing them past its edges. mx-4 on
              each keeps 32px between two link-height targets, over the 24px
              WCAG 2.5.8 spacing floor. */}
          <div className="text-center">
            <Dialog.Root>
              <Dialog.Trigger
                render={
                  <Button
                    type="button"
                    variant="link"
                    size="xs"
                    className="mx-4 h-auto p-0 text-text-sub hover:text-text"
                  >
                    {t('overlayEditor.page.obsHelpTrigger')}
                  </Button>
                }
              />
              <Dialog.Content size="sm">
                <Dialog.Title>{t('overlayEditor.page.obsHelpTitle')}</Dialog.Title>
                <div className="mt-3">
                  <ObsHelpContent />
                </div>
              </Dialog.Content>
            </Dialog.Root>
            <Dialog.Root>
              <Dialog.Trigger
                render={
                  <Button
                    type="button"
                    variant="link"
                    size="xs"
                    className="mx-4 h-auto p-0 text-text-sub hover:text-text"
                  >
                    {t('overlayEditor.page.dockHelpTrigger')}
                  </Button>
                }
              />
              <Dialog.Content size="sm">
                <Dialog.Title>{t('overlayEditor.page.dockHelpTitle')}</Dialog.Title>
                <div className="mt-3">
                  <ObsDockHelpContent />
                </div>
              </Dialog.Content>
            </Dialog.Root>
          </div>

          {/* 2b. Share overlay */}
          <Button
            variant="outline"
            className="flex w-full items-center justify-center gap-2"
            onClick={handleShareClick}
          >
            <Share2 className="size-4" />
            {t('overlayEditor.page.shareOverlay')}
          </Button>

          {/* 2c. Extension overlay */}
          <Card className="p-4">
            <div className="flex items-start gap-3">
              <Puzzle className="mt-0.5 size-4 shrink-0 text-twitch" />
              <div className="min-w-0 flex-1">
                <div className="mb-0.5 flex items-center gap-2">
                  <p className="text-sm font-semibold text-text">
                    {t('overlayEditor.page.extensionHeading')}
                  </p>
                  {isPublicForViewers && (
                    <span className="inline-flex items-center rounded border border-twitch/30 bg-twitch/15 px-1.5 py-0.5 text-[10px] font-semibold text-twitch">
                      {t('overlayEditor.page.extensionActive')}
                    </span>
                  )}
                </div>
                <p className="text-xs text-text-sub">
                  {isPublicForViewers
                    ? t('overlayEditor.page.extensionActiveBody')
                    : t('overlayEditor.page.extensionInactiveBody')}
                </p>
              </div>
              {isPublicForViewers ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="shrink-0 text-xs text-text-sub hover:text-destructive"
                  onClick={handleUnsetExtensionOverlay}
                >
                  {t('overlayEditor.page.extensionDeactivate')}
                </Button>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  className="shrink-0 text-xs"
                  onClick={handleSetAsExtensionOverlay}
                >
                  {t('overlayEditor.page.extensionSetActive')}
                </Button>
              )}
            </div>
          </Card>

          {/* Premium required dialog */}
          <Dialog.Root open={showPremiumRequired} onOpenChange={setShowPremiumRequired}>
            <Dialog.Content size="sm">
              <Dialog.Title>{t('overlayEditor.page.premiumRequiredTitle')}</Dialog.Title>
              <Dialog.Description>
                {interpolateElements(t('overlayEditor.page.premiumRequiredBody'), {
                  upgradeLink: (
                    <PremiumUpsellLink>
                      {t('overlayEditor.page.premiumUpgradeLink')}
                    </PremiumUpsellLink>
                  ),
                })}
              </Dialog.Description>
              <p className="mt-3 text-sm text-text-sub">
                {interpolateElements(t('overlayEditor.page.questionsJoin'), {
                  discordLink: (
                    <a
                      href={DISCORD_INVITE_URL}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-medium text-twitch hover:underline"
                    >
                      {t('overlayEditor.page.discordCommunityLink')}
                    </a>
                  ),
                })}
              </p>
              <div className="mt-5 flex gap-2">
                <Dialog.Close
                  render={
                    <Button type="button" variant="outline" className="flex-1">
                      {t('overlayEditor.page.close')}
                    </Button>
                  }
                />
                <Link href="/upgrade" className="flex-1">
                  <Button className="w-full">{t('overlayEditor.page.upgrade')}</Button>
                </Link>
              </div>
            </Dialog.Content>
          </Dialog.Root>

          {/* Share overlay modal */}
          <Dialog.Root open={showShareModal} onOpenChange={setShowShareModal}>
            <Dialog.Content size="sm">
              <Dialog.Title>{t('overlayEditor.page.shareTitle')}</Dialog.Title>
              <Dialog.Description>
                {emphasise(
                  t('overlayEditor.page.shareBody', { emphasis: overlay.name }),
                  overlay.name,
                  (run) => (
                    <strong>{run}</strong>
                  )
                )}
              </Dialog.Description>
              <div className="mt-4 mb-4">
                <label htmlFor="share-recipient" className="mb-1 block text-xs text-text-sub">
                  {t('overlayEditor.page.shareRecipientLabel')}
                </label>
                <Input
                  id="share-recipient"
                  ref={shareInputRef}
                  type="text"
                  value={shareRecipient}
                  onChange={(e) => setShareRecipient(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleSendShareRequest()}
                  placeholder={t('overlayEditor.page.shareRecipientPlaceholder')}
                  disabled={shareLoading}
                />
              </div>
              <div className="flex gap-2">
                <Dialog.Close
                  render={
                    <Button
                      type="button"
                      variant="outline"
                      className="flex-1"
                      disabled={shareLoading}
                    >
                      {t('overlayEditor.page.shareCancel')}
                    </Button>
                  }
                />
                <Button
                  className="flex-1"
                  onClick={handleSendShareRequest}
                  disabled={shareLoading || !shareRecipient.trim()}
                >
                  {shareLoading
                    ? t('overlayEditor.page.shareSending')
                    : t('overlayEditor.page.shareSend')}
                </Button>
              </div>
            </Dialog.Content>
          </Dialog.Root>

          {/* Settings: search + left nav + one section at a time (ADR-0042) */}
          {/* sticky footer uses position:sticky bottom-0 — works because split-view-config has overflow-y-auto */}
          <div className="relative">
            <SettingsSearch onNavigate={handleSearchNavigate} />
            <div className="@container mt-4">
              <div className="flex flex-col gap-4 @md:flex-row @md:gap-5">
                <EditorNav activeId={activeSection} onSelect={handleSelectSection} />
                <div ref={panelRef} className="min-w-0 flex-1">
                  <EditorSectionHeader id={activeSection} />

                  {activeSection === 'theme' && (
                    <div>
                      <ThemeContent onApply={handleThemeApply} isAdmin={user?.is_admin === true} />
                      <Button
                        type="button"
                        variant="link"
                        size="xs"
                        className="mt-3 h-auto p-0 text-text-sub hover:text-text"
                        onClick={handleResetToTheme}
                      >
                        {t('overlayEditor.page.resetToThemeDefaults')}
                      </Button>
                    </div>
                  )}

                  {/* Appearance groups — promoted from the retired nested AppearancePanel (ADR-0042) */}
                  {activeSection === 'typography' && (
                    <TypographyGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                    />
                  )}
                  {activeSection === 'colors' && (
                    <ColorsGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                    />
                  )}
                  {activeSection === 'background' && (
                    <BackgroundGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                    />
                  )}
                  {activeSection === 'bubble-colors' && (
                    <BubbleColorsGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                      locked={bubbleColorsLocked}
                    />
                  )}
                  {activeSection === 'visibility' && (
                    <VisibilityGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                      visibilityDefaults={iframeVisibilityDefaults}
                    />
                  )}
                  {activeSection === 'sizing' && (
                    <SizingGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                    />
                  )}
                  {activeSection === 'platform-colors' && (
                    <PlatformColorsGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                    />
                  )}
                  {activeSection === 'events' && (
                    <EventsGroup
                      visualSettings={visualSettings}
                      onChange={handleVisualSettingsChange}
                    />
                  )}
                  {activeSection === 'filters' && (
                    <FilterGroup
                      filterSettings={filterSettings}
                      onChange={handleFilterSettingsChange}
                    />
                  )}
                  {activeSection === 'sounds' && (
                    <SoundGroup
                      displaySettings={{ ...soundSettings, ...ttsSettings }}
                      onChange={handleSoundSettingsChange}
                      isPremium={user?.is_premium ?? false}
                    />
                  )}
                  {activeSection === 'tts' && (
                    <TTSGroup
                      displaySettings={{ ...soundSettings, ...ttsSettings }}
                      onChange={handleTTSSettingsChange}
                      isPremium={user?.is_premium ?? false}
                      overlayId={id}
                      hasElevenLabsConfig={hasElevenLabsConfig}
                      obsUrl={obsUrl}
                      onPreview={() => {
                        // Browser Web Speech API preview — fires the fixed sample phrase through the
                        // current rate/pitch/volume/voice_uri. Click again mid-speech cancels.
                        if (
                          typeof window === 'undefined' ||
                          typeof window.speechSynthesis === 'undefined'
                        )
                          return
                        const synth = window.speechSynthesis
                        if (synth.speaking) {
                          synth.cancel()
                          return
                        }
                        const u = new SpeechSynthesisUtterance(
                          'Hello, this is how your chat will sound.'
                        )
                        u.volume = ttsSettings.tts_volume ?? 0.8
                        u.rate = ttsSettings.tts_rate ?? 1.0
                        u.pitch = ttsSettings.tts_pitch ?? 1.0
                        const savedUri = ttsSettings.tts_voice_uri
                        if (savedUri) {
                          const match = synth.getVoices().find((v) => v.voiceURI === savedUri)
                          if (match) u.voice = match
                        }
                        synth.speak(u)
                      }}
                      onPreviewStop={() => {
                        if (typeof window !== 'undefined' && window.speechSynthesis) {
                          window.speechSynthesis.cancel()
                        }
                      }}
                      onSaveKey={handleSaveTTSKey}
                      onSaveVoice={handleSaveTTSVoice}
                      savedVoiceId={elevenLabsVoiceId}
                      onTestKey={handleTestTTSKey}
                      onRotateToken={handleRotateTTSToken}
                      onRemoveKey={handleRemoveTTSKey}
                      onFetchVoices={handleFetchTTSVoices}
                      onPreviewVoices={handlePreviewTTSVoices}
                    />
                  )}

                  {activeSection === 'sources' && (
                    <div>
                      {isLoading ? (
                        <SourceListSkeleton />
                      ) : (
                        <div className="mb-4 space-y-3">
                          {sources.map((source) => (
                            <div key={source.id}>
                              <SourceCard
                                source={source}
                                onRemove={handleRemoveSource}
                                onRevoke={setRevokeTarget}
                                onConfigureRelay={(s) =>
                                  setRelayExpandedSourceId((prev) => (prev === s.id ? null : s.id))
                                }
                                onConfigureStreamSelect={(s) =>
                                  setStreamSelectExpandedSourceId((prev) =>
                                    prev === s.id ? null : s.id
                                  )
                                }
                                isOwnChannel={
                                  // Prefer the server-computed flag (covers linked/non-Twitch-login
                                  // owners per ADR-0016); fall back to the username heuristic only if
                                  // the field is absent (older API responses).
                                  source.is_own_channel ??
                                  (user?.auth_provider === 'twitch' &&
                                    !!user?.username &&
                                    user.username.toLowerCase() === source.channel_id.toLowerCase())
                                }
                                onReconnectChat={handleReconnectTwitchChat}
                              />
                              {source.id === relayExpandedSourceId &&
                                source.platform === 'discord' && (
                                  <RelayPanel
                                    source={source}
                                    overlayId={id}
                                    onSaved={handleRelayConfigSaved}
                                  />
                                )}
                              {source.id === streamSelectExpandedSourceId &&
                                source.platform === 'youtube' && (
                                  <StreamSelectionPanel
                                    source={source}
                                    overlayId={id}
                                    isPremium={user?.is_premium ?? false}
                                    onSaved={async () => {
                                      const updated = await overlaysApi.getSources(id)
                                      setSources(updated)
                                    }}
                                  />
                                )}
                            </div>
                          ))}
                          {sources.length === 0 && (
                            <p className="py-2 text-sm text-text-sub">
                              {t('overlayEditor.sources.empty')}
                            </p>
                          )}
                        </div>
                      )}

                      {/* Accepted shared overlays — add as source */}
                      {acceptedShares.length > 0 && (
                        <div className="mb-4 border-t border-border pt-4">
                          <h3 className="mb-3 text-sm font-medium text-text">
                            {t('overlayEditor.sources.sharedHeading')}
                          </h3>
                          <div className="space-y-2">
                            {acceptedShares.map((share) => (
                              <Button
                                key={share.share_id}
                                onClick={() => handleAddSharedOverlay(share)}
                                variant="outline"
                                size="lg"
                                className="w-full justify-between"
                              >
                                <span>
                                  {t('overlayEditor.sources.sharedOwner', {
                                    owner: share.sender_display_name,
                                  })}
                                </span>
                                <span className="text-xs text-twitch">
                                  {t('overlayEditor.sources.add')}
                                </span>
                              </Button>
                            ))}
                          </div>
                        </div>
                      )}

                      <AddSourceForm
                        overlayId={id}
                        onAddTikTok={handleAddTikTokSource}
                        onAddManual={handleAddManual}
                        onSourceAdded={() =>
                          overlaysApi.getSources(id).then(setSources).catch(console.error)
                        }
                        isAdmin={user?.is_admin === true}
                      />
                    </div>
                  )}

                  {activeSection === 'messages' && (
                    <div className="space-y-5">
                      {/* Max Messages */}
                      <div data-setting-anchor="maxMessages">
                        <label className="mb-1 block text-xs text-text-sub">
                          {interpolateElements(t('overlayEditor.messages.maxMessagesLabel'), {
                            value: <span className="text-twitch">{maxMessages}</span>,
                          })}
                        </label>
                        <input
                          type="range"
                          min="10"
                          max="100"
                          value={maxMessages}
                          onChange={(e) => setMaxMessages(parseInt(e.target.value))}
                          className="w-full accent-twitch"
                        />
                      </div>

                      {/* Message Duration */}
                      <div data-setting-anchor="messageDuration">
                        <label className="mb-1 block text-xs text-text-sub">
                          {interpolateElements(t('overlayEditor.messages.messageDurationLabel'), {
                            value: (
                              <span className="text-twitch">
                                {t('overlayEditor.messages.durationSeconds', {
                                  seconds: messageDuration,
                                })}
                              </span>
                            ),
                          })}
                        </label>
                        <input
                          type="range"
                          min="5"
                          max="60"
                          value={messageDuration}
                          onChange={(e) => setMessageDuration(parseInt(e.target.value))}
                          className="w-full accent-twitch"
                          disabled={disableMessageFade}
                        />
                      </div>

                      {/* Disable fade */}
                      <div data-setting-anchor="disableFade">
                        <label className="flex items-center gap-2 text-xs text-text-sub">
                          <input
                            type="checkbox"
                            checked={disableMessageFade}
                            onChange={(e) => setDisableMessageFade(e.target.checked)}
                            className="accent-twitch"
                          />
                          {t('overlayEditor.messages.disableFade')}
                        </label>
                        <p className="mt-1 ml-5 text-xs text-text-sub">
                          {t('overlayEditor.messages.disableFadeHint')}
                        </p>
                      </div>

                      {/* Feed Anchor — which canvas EDGE the stack rests on.
                          Deliberately adjacent to Invert Message Order, which
                          controls the other axis (which END OF THE LIST is
                          newest). Conflating the two is what made #728. */}
                      <div data-setting-anchor="feedAnchor">
                        <label
                          htmlFor={feedAnchorSelectId}
                          className="mb-1 block text-xs text-text-sub"
                        >
                          {t('overlayEditor.messages.feedAnchorLabel')}
                        </label>
                        <select
                          id={feedAnchorSelectId}
                          value={feedAnchor}
                          onChange={(e) => setFeedAnchor(parseFeedAnchor(e.target.value))}
                          className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-xs text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                        >
                          <option value="top">{t('overlayEditor.messages.feedAnchorTop')}</option>
                          <option value="bottom">
                            {t('overlayEditor.messages.feedAnchorBottom')}
                          </option>
                        </select>
                        <p className="mt-1 text-xs text-text-sub">
                          {t('overlayEditor.messages.feedAnchorHint')}
                        </p>
                      </div>

                      {/* Invert message order */}
                      <div data-setting-anchor="invertOrder">
                        <label className="flex items-center gap-2 text-xs text-text-sub">
                          <input
                            type="checkbox"
                            checked={invertMessageOrder}
                            onChange={(e) => setInvertMessageOrder(e.target.checked)}
                            className="accent-twitch"
                          />
                          {t('overlayEditor.messages.invertOrder')}
                        </label>
                        <p className="mt-1 ml-5 text-xs text-text-sub">
                          {t('overlayEditor.messages.invertOrderHint')}
                        </p>
                      </div>

                      {/* Entry Animation */}
                      <div data-setting-anchor="entryAnimation">
                        <label
                          htmlFor={entryAnimationSelectId}
                          className="mb-1 block text-xs text-text-sub"
                        >
                          {t('overlayEditor.messages.entryAnimationLabel')}
                        </label>
                        <select
                          id={entryAnimationSelectId}
                          value={visualSettings.messageAnimation ?? ''}
                          onChange={(e) =>
                            handleVisualSettingsChange({
                              messageAnimation: isMessageAnimation(e.target.value)
                                ? e.target.value
                                : undefined,
                            })
                          }
                          className="w-full rounded-lg border border-border bg-surface px-3 py-1.5 text-xs text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                        >
                          {ENTRY_ANIMATION_OPTIONS.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                              {t(`overlayEditor.messages.animation${opt.messageStem}`)}
                            </option>
                          ))}
                        </select>
                        <p className="mt-1 text-xs text-text-sub">
                          {t('overlayEditor.messages.entryAnimationHint')}
                        </p>
                      </div>

                      {/* Emote Providers */}
                      <div data-setting-anchor="emoteProviders">
                        <p className="mb-2 text-xs text-text-sub">
                          {t('overlayEditor.messages.emoteProvidersLabel')}
                        </p>
                        <div className="space-y-1.5">
                          <label className="flex items-center gap-2 text-xs text-text-sub">
                            <input
                              type="checkbox"
                              checked={enable7tv}
                              onChange={(e) => setEnable7tv(e.target.checked)}
                              className="accent-twitch"
                            />
                            {t('overlayEditor.messages.seventv')}
                          </label>
                          <label className="flex items-center gap-2 text-xs text-text-sub">
                            <input
                              type="checkbox"
                              checked={enableBttv}
                              onChange={(e) => setEnableBttv(e.target.checked)}
                              className="accent-twitch"
                            />
                            {t('overlayEditor.messages.betterttv')}
                          </label>
                          <label className="flex items-center gap-2 text-xs text-text-sub">
                            <input
                              type="checkbox"
                              checked={enableFfz}
                              onChange={(e) => setEnableFfz(e.target.checked)}
                              className="accent-twitch"
                            />
                            {t('overlayEditor.messages.frankerfacez')}
                          </label>
                        </div>
                      </div>

                      {/* 7TV emote-set override — useful when no Twitch source exists */}
                      <AdvancedDisclosure count={1}>
                        <div data-setting-anchor="seventvOverride">
                          <p className="mb-1 text-xs text-text-sub">
                            {t('overlayEditor.messages.seventvOverrideLabel')}
                          </p>
                          <p className="mb-2 text-[11px] text-text-sub/70">
                            {t('overlayEditor.messages.seventvOverrideHint')}
                          </p>
                          {/* Saved-state pill: shows what's actually attached right now,
                        with a one-click Remove. Hidden while nothing is saved. */}
                          {seventvOverrideSavedID !== '' && (
                            <div className="mb-2 flex items-center justify-between gap-2 rounded-md border border-border bg-surface-2 px-2 py-1.5 text-[11px]">
                              <span className="truncate text-text">
                                <span className="text-text-sub">
                                  {t('overlayEditor.messages.seventvCurrentlyActive')}
                                </span>
                                {seventvSavedDescriptor?.name ? (
                                  <>
                                    <span className="font-medium">
                                      {t('overlayEditor.messages.seventvQuotedName', {
                                        name: seventvSavedDescriptor.name,
                                      })}
                                    </span>
                                    {typeof seventvSavedDescriptor.emoteCount === 'number'
                                      ? t('overlayEditor.messages.seventvEmoteCount', {
                                          count: seventvSavedDescriptor.emoteCount,
                                        })
                                      : ''}
                                  </>
                                ) : (
                                  <span className="font-mono">{seventvOverrideSavedID}</span>
                                )}
                              </span>
                              <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                className="shrink-0 text-[11px] text-red-500 hover:text-red-400"
                                disabled={seventvRemoving || isSavingConfig}
                                onClick={async () => {
                                  setSeventvRemoving(true)
                                  try {
                                    await overlaysApi.updateConfig(id, {
                                      seventv_emote_set_id: '',
                                    })
                                    setSeventvOverrideInput('')
                                    setSeventvOverrideSavedID('')
                                    setSeventvSavedDescriptor(null)
                                    setSeventvResolveState({ status: 'idle' })
                                    setConfigAlert({
                                      type: 'success',
                                      message: t('overlayEditor.messages.seventvRemoved'),
                                    })
                                    setTimeout(() => setConfigAlert(null), 5000)
                                  } catch (err) {
                                    const message =
                                      err instanceof Error
                                        ? err.message
                                        : t('overlayEditor.messages.seventvRemoveFailed')
                                    setConfigAlert({ type: 'error', message })
                                    setTimeout(() => setConfigAlert(null), 5000)
                                  } finally {
                                    setSeventvRemoving(false)
                                  }
                                }}
                              >
                                {seventvRemoving
                                  ? t('overlayEditor.messages.seventvRemoving')
                                  : t('overlayEditor.messages.seventvRemove')}
                              </Button>
                            </div>
                          )}
                          <div className="flex gap-2">
                            <Input
                              type="text"
                              value={seventvOverrideInput}
                              onChange={(e) => {
                                setSeventvOverrideInput(e.target.value)
                                setSeventvResolveState({ status: 'idle' })
                              }}
                              placeholder={
                                seventvOverrideSavedID !== ''
                                  ? t('overlayEditor.messages.seventvReplacePlaceholder')
                                  : t('overlayEditor.messages.seventvUrlPlaceholder')
                              }
                              className="flex-1"
                            />
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              className="text-xs"
                              disabled={
                                seventvResolveState.status === 'resolving' ||
                                seventvOverrideInput.trim() === '' ||
                                seventvOverrideInput.trim() === seventvOverrideSavedID
                              }
                              onClick={async () => {
                                setSeventvResolveState({ status: 'resolving' })
                                try {
                                  const result = await overlaysApi.resolveSevenTV(
                                    id,
                                    seventvOverrideInput.trim()
                                  )
                                  setSeventvResolveState({
                                    status: 'resolved',
                                    setID: result.emote_set_id,
                                    name: result.name,
                                    emoteCount: result.emote_count,
                                  })
                                } catch (err) {
                                  const message =
                                    err instanceof Error
                                      ? err.message
                                      : t('overlayEditor.messages.seventvResolveFailed')
                                  setSeventvResolveState({ status: 'error', message })
                                }
                              }}
                            >
                              {seventvResolveState.status === 'resolving'
                                ? t('overlayEditor.messages.seventvChecking')
                                : t('overlayEditor.messages.seventvVerify')}
                            </Button>
                          </div>
                          {seventvResolveState.status === 'resolved' &&
                            seventvResolveState.setID !== seventvOverrideSavedID && (
                              <p className="mt-1 text-[11px] text-green-500">
                                {resolvedSevenTvNotice(t, seventvResolveState)}
                              </p>
                            )}
                          {seventvResolveState.status === 'error' && (
                            <p className="mt-1 text-[11px] text-red-500">
                              {seventvResolveState.message}
                            </p>
                          )}
                        </div>
                      </AdvancedDisclosure>
                    </div>
                  )}

                  {/* Engagement — polls, predictions & viewer points (issue #523) */}
                  {activeSection === 'engagement' && <EngagementPanel overlayId={id} />}

                  {activeSection === 'moderators' && <ModeratorsPanel overlayId={id} />}

                  {activeSection === 'custom-css' && (
                    <div className="space-y-3">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-xs font-medium text-text">
                          {t('overlayEditor.customCss.heading')}
                        </span>
                        {(() => {
                          const customized = isCustomCssForked(customCss, pristineThemeCss)
                          if (!customized) {
                            return themeId ? (
                              <span className="inline-flex items-center rounded-full border border-green-500/20 bg-green-500/10 px-2 py-0.5 text-[11px] font-medium text-green-400">
                                {t('overlayEditor.customCss.usingTheme', {
                                  theme: getBundledTheme(themeId)?.name ?? themeId,
                                })}
                              </span>
                            ) : (
                              <span className="text-[11px] text-text-dim">
                                {t('overlayEditor.customCss.noThemeApplied')}
                              </span>
                            )
                          }
                          if (!themeId) {
                            return (
                              <span className="inline-flex items-center rounded-full border border-border bg-surface-2 px-2 py-0.5 text-[11px] font-medium text-text-sub">
                                {t('overlayEditor.customCss.customPill')}
                              </span>
                            )
                          }
                          return customCssMode === 'fork' ? (
                            <span className="inline-flex items-center rounded-full border border-amber-500/30 bg-amber-500/10 px-2 py-0.5 text-[11px] font-medium text-amber-400">
                              {t('overlayEditor.customCss.forkedPill')}
                            </span>
                          ) : (
                            <span className="inline-flex items-center rounded-full border border-green-500/20 bg-green-500/10 px-2 py-0.5 text-[11px] font-medium text-green-400">
                              {t('overlayEditor.customCss.layeredPill')}
                            </span>
                          )
                        })()}
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="ml-auto text-xs"
                          onClick={handleResetCustomCss}
                        >
                          {themeId
                            ? t('overlayEditor.customCss.resetToTheme')
                            : t('overlayEditor.customCss.clear')}
                        </Button>
                      </div>

                      <p className="text-xs text-text-sub">
                        {t('overlayEditor.customCss.explainer')}
                      </p>

                      <MonacoCSSEditor
                        value={customCss}
                        onChange={handleCustomCssChange}
                        onValidate={setCssIssues}
                        height="320px"
                        placeholder={t('overlayEditor.customCss.editorPlaceholder')}
                      />

                      {/* Tips for broken CSS, surfaced from Monaco's CSS language service. */}
                      {(() => {
                        const errors = cssIssues.filter((i) => i.severity === 'error')
                        const warnings = cssIssues.filter((i) => i.severity === 'warning')
                        if (errors.length === 0 && warnings.length === 0) {
                          return (
                            <p className="text-xs text-green-400">
                              {t('overlayEditor.customCss.noProblems')}
                            </p>
                          )
                        }
                        const parts: string[] = []
                        if (errors.length > 0)
                          parts.push(cssProblemCount(t, 'error', errors.length))
                        if (warnings.length > 0)
                          parts.push(cssProblemCount(t, 'warning', warnings.length))
                        return (
                          <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-2">
                            <p className="text-xs font-medium text-amber-300">
                              {t('overlayEditor.customCss.problemsAdvice', {
                                counts: parts.join(t('overlayEditor.customCss.problemsSeparator')),
                              })}
                            </p>
                            <ul className="mt-1 space-y-0.5">
                              {[...errors, ...warnings].slice(0, 5).map((issue, idx) => (
                                <li key={idx} className="text-[11px] text-text-sub">
                                  <span className="font-mono text-text-dim">
                                    {t('overlayEditor.customCss.issueLine', { line: issue.line })}
                                  </span>{' '}
                                  {issue.message}
                                </li>
                              ))}
                              {errors.length + warnings.length > 5 && (
                                <li className="text-[11px] text-text-dim">
                                  {t('overlayEditor.customCss.moreIssues', {
                                    count: errors.length + warnings.length - 5,
                                  })}
                                </li>
                              )}
                            </ul>
                          </div>
                        )
                      })()}

                      <p className="text-xs text-text-sub">
                        {interpolateElements(t('overlayEditor.customCss.inspiration'), {
                          docsLink: (
                            <a
                              href="https://github.com/caesarakalaeii/all-chat/tree/main/docs/overlay-themes"
                              target="_blank"
                              rel="noreferrer"
                              className="text-twitch hover:underline"
                            >
                              {t('overlayEditor.customCss.themeDocsLink')}
                            </a>
                          ),
                        })}
                      </p>
                    </div>
                  )}

                  {activeSection === 'testing' && (
                    <div className="space-y-3" data-setting-anchor="mockMessage">
                      <div>
                        <label
                          htmlFor={`${mockFieldId}-platform`}
                          className="mb-1 block text-xs text-text-sub"
                        >
                          {t('overlayEditor.testing.platformLabel')}
                        </label>
                        <select
                          id={`${mockFieldId}-platform`}
                          value={mockForm.platform}
                          onChange={(e) =>
                            handleMockInputChange(
                              'platform',
                              e.target.value as MockMessageFormState['platform']
                            )
                          }
                          className="w-full rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                        >
                          <option value="twitch">{t('common.platforms.twitch')}</option>
                          <option value="youtube">{t('common.platforms.youtube')}</option>
                          <option value="kick">{t('common.platforms.kick')}</option>
                          <option value="tiktok">{t('common.platforms.tiktok')}</option>
                        </select>
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label
                            htmlFor={`${mockFieldId}-display-name`}
                            className="mb-1 block text-xs text-text-sub"
                          >
                            {t('overlayEditor.testing.displayNameLabel')}
                          </label>
                          <Input
                            id={`${mockFieldId}-display-name`}
                            type="text"
                            value={mockForm.displayName}
                            onChange={(e) => handleMockInputChange('displayName', e.target.value)}
                          />
                        </div>
                        <div>
                          <label
                            htmlFor={`${mockFieldId}-username`}
                            className="mb-1 block text-xs text-text-sub"
                          >
                            {t('overlayEditor.testing.usernameLabel')}
                          </label>
                          <Input
                            id={`${mockFieldId}-username`}
                            type="text"
                            value={mockForm.username}
                            onChange={(e) => handleMockInputChange('username', e.target.value)}
                          />
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <label
                            htmlFor={`${mockFieldId}-avatar-url`}
                            className="mb-1 block text-xs text-text-sub"
                          >
                            {t('overlayEditor.testing.avatarUrlLabel')}
                          </label>
                          <Input
                            id={`${mockFieldId}-avatar-url`}
                            type="text"
                            value={mockForm.avatarUrl}
                            onChange={(e) => handleMockInputChange('avatarUrl', e.target.value)}
                            placeholder={t('overlayEditor.testing.avatarUrlPlaceholder')}
                          />
                        </div>
                        <div>
                          <label
                            htmlFor={`${mockFieldId}-color`}
                            className="mb-1 block text-xs text-text-sub"
                          >
                            {t('overlayEditor.testing.nameColorLabel')}
                          </label>
                          <input
                            id={`${mockFieldId}-color`}
                            type="color"
                            value={mockForm.color}
                            onChange={(e) => handleMockInputChange('color', e.target.value)}
                            className="h-9 w-full rounded-lg border border-border bg-surface px-2 py-1.5"
                          />
                        </div>
                      </div>
                      <div>
                        <label
                          htmlFor={`${mockFieldId}-message`}
                          className="mb-1 block text-xs text-text-sub"
                        >
                          {t('overlayEditor.testing.messageLabel')}
                        </label>
                        <textarea
                          id={`${mockFieldId}-message`}
                          value={mockForm.message}
                          onChange={(e) => handleMockInputChange('message', e.target.value)}
                          className="h-16 w-full resize-none rounded-lg border border-border bg-surface px-2 py-2 text-sm text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                          placeholder={t('overlayEditor.testing.messagePlaceholder')}
                        />
                      </div>
                      <Button
                        type="button"
                        onClick={() => void handleAddMockMessage()}
                        disabled={!mockForm.message.trim()}
                        className="w-full"
                      >
                        {t('overlayEditor.testing.injectMessage')}
                      </Button>
                      <div className="flex gap-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="flex-1 text-xs"
                          data-setting-anchor="sampleChat"
                          onClick={() => void handleAddSampleTranscript()}
                        >
                          {t('overlayEditor.testing.sampleChat')}
                        </Button>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="flex-1 border-yellow-600/40 text-xs text-yellow-400 hover:bg-yellow-900/20"
                          data-setting-anchor="sampleEvents"
                          onClick={() => void handleAddSampleEvents()}
                        >
                          {t('overlayEditor.testing.sampleEvents')}
                        </Button>
                      </div>
                    </div>
                  )}

                  {activeSection === 'danger-zone' && (
                    <div className="space-y-3">
                      <p className="text-xs text-text-sub">
                        {t('overlayEditor.dangerZone.explainer')}
                      </p>
                      <Button
                        type="button"
                        variant="outline"
                        className="w-full border-destructive/50 text-destructive hover:bg-destructive/10"
                        onClick={() => setShowResetConfirm(true)}
                        disabled={isResetting}
                      >
                        {isResetting
                          ? t('overlayEditor.dangerZone.resetting')
                          : t('overlayEditor.dangerZone.resetOverlayId')}
                      </Button>
                    </div>
                  )}
                </div>
              </div>
            </div>

            {/* Sticky Save footer — position:sticky works inside overflow-y-auto split-view-config container */}
            <div className="sticky bottom-0 z-10 -mx-6 border-t border-border bg-bg/95 p-4 backdrop-blur-sm">
              <Button
                onClick={() => void handleSaveConfiguration()}
                disabled={!configLoaded || isSavingConfig}
                className="w-full"
              >
                {isSavingConfig
                  ? t('overlayEditor.page.savingConfiguration')
                  : t('overlayEditor.page.saveConfiguration')}
              </Button>
              {/* Always-mounted live region so save success/failure announces
                  to screen readers (WCAG 4.1.3) — conditionally mounting the
                  role="status" element would not announce reliably. */}
              <p
                role="status"
                className={cn(
                  'text-center text-sm',
                  configAlert && 'mt-2',
                  configAlert?.type === 'success' ? 'text-green-400' : 'text-destructive'
                )}
              >
                {configAlert?.message}
              </p>
            </div>
          </div>
        </div>
      </SplitView>

      {/* Revocation Confirm Modal */}
      {revokeTarget && (
        <RevocationConfirmModal
          partnerName={revokeTarget.channel_name || 'shared overlay'}
          shareId={revokeTarget.channel_id}
          onClose={() => setRevokeTarget(null)}
          onRevoked={() => {
            setRevokeTarget(null)
            overlaysApi.getSources(id).then(setSources).catch(console.error)
          }}
        />
      )}

      {/* Theme apply confirm dialog */}
      <Dialog.Root open={showThemeConfirm} onOpenChange={setShowThemeConfirm}>
        <Dialog.Content size="sm" showCloseButton={false}>
          <Dialog.Title>{t('overlayEditor.page.applyThemeTitle')}</Dialog.Title>
          <Dialog.Description>{t('overlayEditor.page.applyThemeBody')}</Dialog.Description>
          <div className="mt-4 flex justify-end gap-2">
            <Dialog.Close
              render={
                <Button type="button" variant="outline" size="sm">
                  {t('overlayEditor.page.applyThemeCancel')}
                </Button>
              }
            />
            <Button
              type="button"
              size="sm"
              onClick={() => {
                if (pendingTheme) {
                  applyThemeImmediately(pendingTheme)
                  setPendingTheme(null)
                }
                setShowThemeConfirm(false)
              }}
            >
              {t('overlayEditor.page.applyThemeContinue')}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>

      {/* Reset Overlay ID confirm dialog */}
      <Dialog.Root open={showResetConfirm} onOpenChange={setShowResetConfirm}>
        <Dialog.Content size="sm" showCloseButton={false}>
          <Dialog.Title>{t('overlayEditor.dangerZone.confirmTitle')}</Dialog.Title>
          <Dialog.Description>{t('overlayEditor.dangerZone.confirmBody')}</Dialog.Description>
          <div className="mt-4 flex justify-end gap-2">
            <Dialog.Close
              render={
                <Button type="button" variant="outline" size="sm">
                  {t('overlayEditor.dangerZone.cancel')}
                </Button>
              }
            />
            <Button
              type="button"
              size="sm"
              className="border-destructive bg-destructive text-destructive-foreground hover:bg-destructive/90"
              disabled={isResetting}
              onClick={() => void handleConfirmResetOverlayId()}
            >
              {isResetting
                ? t('overlayEditor.dangerZone.resetting')
                : t('overlayEditor.dangerZone.confirmReset')}
            </Button>
          </div>
        </Dialog.Content>
      </Dialog.Root>
    </div>
  )
}
