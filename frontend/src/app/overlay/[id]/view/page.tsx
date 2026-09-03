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
 * Overlay Monitor View (/overlay/[id]/view) — authenticated dashboard.
 *
 * A Twitch-dashboard-inspired monitor for streamers: a resizable live Chat
 * panel + Activity feed (pause-on-scroll scrollback and a click-a-username 1:1
 * chat filter live in ChatPanel), platform connection indicators, an
 * overlay-config summary, its own light/dark mode, view-local display toggles,
 * an optional per-browser sound cue for new Activity items (distinct from the
 * overlay's on-stream notification sounds; see viewPrefs `activitySound*`),
 * and per-message / per-user moderation controls for the overlay's owner. It
 * reuses the exact realtime pipeline the OBS overlay speaks (useOverlayStream)
 * but renders a readable, animation-free dashboard that ignores the overlay's
 * CSS themes.
 *
 * Auth is enforced by the route's layout (ProtectedRoute via OverlayViewGuard);
 * moderation is further gated on overlay ownership + per-source capabilities.
 *
 * With `?dock=1` the same route renders in DOCK MODE for an OBS/Streamlabs
 * custom browser dock: a ~320-450px chromeless panel. Same data path, same
 * moderation, narrower chrome — a single non-wrapping header row with one
 * overflow menu, the notice strips collapsed into one status row, and Chat |
 * Activity as two tabs instead of a split. See ./dockMode.
 */

'use client'

import clsx from 'clsx'
import { Button } from '@/components/ui/button'
import { BarChart3, ExternalLink, Info, RotateCw, SlidersHorizontal } from 'lucide-react'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'
import { use, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toastManager } from '@/lib/toast'

import { MaintenanceInfoButton } from '@/components/MaintenanceInfoButton'
import PlatformStatusIndicators from '@/components/PlatformStatusIndicators'
import { ActivityPanel } from '@/components/overlay/ActivityPanel'
import { ChatPanel, type ChatPanelModeration } from '@/components/overlay/ChatPanel'
import { ChatSendBar } from '@/components/overlay/ChatSendBar'
import { ConnectionBadge } from '@/components/overlay/ConnectionBadge'
import { DockNoticeBar } from '@/components/overlay/DockNoticeBar'
import { DockOverflowMenu } from '@/components/overlay/DockOverflowMenu'
import { DOCK_PANEL_ID, DockTabPicker } from '@/components/overlay/DockTabPicker'
import { EngagementControls } from '@/components/overlay/EngagementControls'
import { LayoutPicker } from '@/components/overlay/LayoutPicker'
import { ObservabilitySummary } from '@/components/overlay/ObservabilitySummary'
import { OverlayViewThemeToggle } from '@/components/overlay/OverlayViewThemeToggle'
import { ViewSettingsBar } from '@/components/overlay/ViewSettingsBar'
import { ResizableSplit } from '@/components/ResizableSplit'
import { useOverlayStream } from '@/hooks/useOverlayStream'
import { ApiError } from '@/lib/api/client'
import { startDiscordAccountLink, startDiscordModerationReinvite } from '@/lib/api/discord'
import {
  buildBanRequest,
  buildDeleteRequest,
  buildTimeoutRequest,
  buildUnbanRequest,
  isModerationReauthError,
  moderationActionCode,
  moderationApi,
} from '@/lib/api/moderation'
import type { ChatMessage, DeletionMetadata } from '@/lib/types/message'
import type {
  DelegatablePlatform,
  ModerationAction,
  ModerationCapabilities,
  SourceCapability,
} from '@/lib/types/moderation'
import type { EventSettings } from '@/lib/types/overlay'
import {
  applyModerationMark,
  deletionSignature,
  isAudienceEvent,
  mergeAutoModResolution,
  mergeByAgg,
  partitionItems,
  toModActionEntry,
  toModEntry,
  type ModEntry,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { OFFLINE_THRESHOLD } from '@/lib/utils/connectionStatusLabel'
import { createSoundPlayer, type SoundPlayer, type SoundSettings } from '@/lib/utils/soundPlayer'

import { DEFAULT_DOCK_TAB, isDockMode, loadDockTab, saveDockTab, type DockTab } from './dockMode'
import { shouldOfferModLogOptIn } from './modLogOptIn'
import {
  DEFAULT_VIEW_LAYOUT,
  LAYOUT_CONFIG,
  loadViewLayout,
  saveViewLayout,
  type ViewLayout,
} from './viewLayout'
import {
  DEFAULT_VIEW_PREFS,
  loadViewPrefs,
  saveViewPrefs,
  type MonitorViewPrefs,
} from './viewPrefs'

const MAX_ITEMS = 500
const MAX_MOD_LOG = 200
const THEME_KEY = 'overlay-view-theme'

// Gap between activity sounds. Longer than the overlay chat cooldown because a
// single audience action can fan out into a burst (e.g. a mystery gift emits
// many gift-sub events); one gentle ping per burst is what a moderator wants.
const ACTIVITY_SOUND_COOLDOWN_MS = 1500

/**
 * The missing-scope notice for one source.
 *
 * Discord's remedy is a bot re-invite rather than a scope grant, and the channel
 * name is optional, so the two variables give four whole sentences. The
 * platform is the raw lowercase wire value, which is what renders today.
 */
function missingScopeNotice(t: TFunction, platform: string, channel?: string): string {
  if (platform === 'discord') {
    return channel
      ? t('viewerOverlay.monitor.missingScopeDiscordChannel', { platform, channel })
      : t('viewerOverlay.monitor.missingScopeDiscord', { platform })
  }
  return channel
    ? t('viewerOverlay.monitor.missingScopeChannel', { platform, channel })
    : t('viewerOverlay.monitor.missingScope', { platform })
}

/** Map the moderator's view prefs onto the shared sound-player settings. */
function toActivitySoundSettings(prefs: MonitorViewPrefs): SoundSettings {
  return {
    enabled: prefs.activitySoundEnabled,
    preset: prefs.activitySoundPreset,
    volume: prefs.activitySoundVolume,
    cooldownMs: ACTIVITY_SOUND_COOLDOWN_MS,
  }
}

export default function OverlayMonitorView({ params }: { params: Promise<{ id: string }> }) {
  const t = useTranslations()
  const { id } = use(params)
  // Presentation only. Every hook below runs identically in both modes; dock
  // mode changes what the return statement wraps them in.
  const dock = isDockMode(useSearchParams())

  const [items, setItems] = useState<ViewItem[]>([])
  const [moderationLog, setModerationLog] = useState<ModEntry[]>([])
  const [observedEventTypes, setObservedEventTypes] = useState<Set<string>>(new Set())
  const [eventSettings, setEventSettings] = useState<EventSettings | null>(null)
  const [light, setLight] = useState(false)
  const [showDetails, setShowDetails] = useState(false)
  const [showEngagement, setShowEngagement] = useState(false)
  const [prefs, setPrefs] = useState<MonitorViewPrefs>(DEFAULT_VIEW_PREFS)
  const [layout, setLayout] = useState<ViewLayout>(DEFAULT_VIEW_LAYOUT)
  const [dockTab, setDockTab] = useState<DockTab>(DEFAULT_DOCK_TAB)
  const [capabilities, setCapabilities] = useState<ModerationCapabilities | null>(null)
  const modSeqRef = useRef(0)
  // Signatures of deletions we applied optimistically, awaiting their WS echo —
  // so the server-pushed confirmation doesn't double-log the action.
  const pendingDeletionsRef = useRef<Set<string>>(new Set())
  // Live mirror of `items` so optimistic moderation can compute exactly which
  // rows it struck through (for rollback) without an impure state updater.
  const itemsRef = useRef<ViewItem[]>([])
  useEffect(() => {
    itemsRef.current = items
  }, [items])

  // --- Stream callbacks ----------------------------------------------------

  // Per-browser player for the activity sound. Created after mount (client
  // only) and kept in sync with prefs by the effects below; onChat plays
  // through it and the player itself gates on enabled/cooldown/volume, so
  // onChat needs no prefs in its dependency list.
  const activitySoundRef = useRef<SoundPlayer | null>(null)

  const onChat = useCallback((message: ChatMessage) => {
    setItems((prev) => [...prev, message].slice(-MAX_ITEMS))
    const type = message.event?.type
    if (type) {
      setObservedEventTypes((prev) => (prev.has(type) ? prev : new Set(prev).add(type)))
    }
    // Audible cue for easy-to-miss audience activity (channel points, gifts,
    // follows, …). No-op unless the moderator enabled the activity sound.
    if (isAudienceEvent(message)) {
      activitySoundRef.current?.play()
    }
  }, [])

  const onMessageUpdate = useCallback((message: ChatMessage) => {
    setItems((prev) => mergeByAgg(prev, message, MAX_ITEMS))
  }, [])

  const onDeletion = useCallback((deletion: DeletionMetadata, source: 'replay' | 'live') => {
    // Observability: keep the message visible (struck-through) and log the action.
    setItems((prev) => applyModerationMark(prev, deletion))
    // Dedup: if this confirms an optimistic action we already logged, swallow it.
    const sig = deletionSignature(deletion)
    if (pendingDeletionsRef.current.delete(sig)) return
    setModerationLog((prev) =>
      [
        ...prev,
        { id: (modSeqRef.current += 1), ...toModEntry(deletion, source, Date.now()) },
      ].slice(-MAX_MOD_LOG)
    )
  }, [])

  // Twitch moderation-log / AutoMod frames. No optimistic dedup: unlike the
  // deletion path, this client never produces a mod_action itself, so every
  // frame here is news. An AutoMod resolution folds into the hold it closes
  // rather than adding a row.
  const onModAction = useCallback(
    (metadata: Record<string, unknown>, source: 'replay' | 'live') => {
      const entry = toModActionEntry(metadata, source, Date.now())
      if (!entry) return
      setModerationLog((prev) =>
        mergeAutoModResolution(prev, { id: (modSeqRef.current += 1), ...entry }).slice(-MAX_MOD_LOG)
      )
    },
    []
  )

  const {
    config,
    sources,
    activeChannels,
    channelStatuses,
    connectionStatus,
    reconnectAttempts,
    replayTruncated,
  } = useOverlayStream(id, {
    onChat,
    onMessageUpdate,
    onDeletion,
    onModAction,
  })

  // Fetch per-overlay event toggles (degrades gracefully if disabled — setState
  // happens in a promise callback, so this is not a synchronous set-state-in-effect).
  useEffect(() => {
    let cancelled = false
    fetch(`/api/v1/overlays/public/${id}/event-settings`)
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (!cancelled && data) setEventSettings(data as EventSettings)
      })
      .catch(() => {
        /* event-settings panel falls back to observed event types */
      })
    return () => {
      cancelled = true
    }
  }, [id])

  // Fetch moderation capabilities once the user is known. A caller with no role gets
  // { role:'none', sources:[] } — the same body an overlay that does not exist produces,
  // so it says nothing about the overlay. Any failure leaves moderation disabled.
  useEffect(() => {
    let cancelled = false
    moderationApi
      .getCapabilities(id)
      .then((data) => {
        if (!cancelled) setCapabilities(data)
      })
      .catch(() => {
        if (!cancelled)
          setCapabilities({
            role: 'none',
            is_owner: false,
            enabled: false,
            can_moderate: false,
            sources: [],
          })
      })
    return () => {
      cancelled = true
    }
  }, [id])

  // Restore saved view prefs once on mount (localStorage; client only).
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
    setPrefs(loadViewPrefs())
  }, [])

  const updatePrefs = useCallback((next: MonitorViewPrefs) => {
    setPrefs(next)
    saveViewPrefs(next)
  }, [])

  // Build the activity-sound player once (client only), then keep its settings
  // in lock-step with prefs. Split into create-once + sync so we never leak
  // audio elements across re-renders.
  useEffect(() => {
    const player = createSoundPlayer(toActivitySoundSettings(DEFAULT_VIEW_PREFS))
    activitySoundRef.current = player
    return () => {
      player.destroy()
      activitySoundRef.current = null
    }
  }, [])

  useEffect(() => {
    activitySoundRef.current?.updateSettings(toActivitySoundSettings(prefs))
  }, [prefs])

  // Preview the activity sound. Bypasses the player's cooldown/enabled gate so
  // "Test" always previews, and the click doubles as the user gesture browsers
  // require before any programmatic audio may play in this tab.
  const testActivitySound = useCallback(() => {
    const el = new Audio(`/sounds/${prefs.activitySoundPreset}.mp3`)
    el.volume = prefs.activitySoundVolume
    el.play().catch(() => {
      /* autoplay may still be blocked; the next attempt after a gesture works */
    })
  }, [prefs.activitySoundPreset, prefs.activitySoundVolume])

  // Restore the saved panel layout after mount (per-overlay; localStorage,
  // client only — guards against SSR hydration mismatch like the prefs/theme).
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
    setLayout(loadViewLayout(id))
  }, [id])

  const updateLayout = useCallback(
    (next: ViewLayout) => {
      setLayout(next)
      saveViewLayout(id, next)
    },
    [id]
  )

  // Same one-time-restore shape as the layout above. Unconditional so the tab a
  // streamer left the dock on survives a dock reload, which is the only way OBS
  // gives them to reopen the panel.
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
    setDockTab(loadDockTab(id))
  }, [id])

  const updateDockTab = useCallback(
    (next: DockTab) => {
      setDockTab(next)
      saveDockTab(id, next)
    },
    [id]
  )

  // Start the opt-in moderation setup (ADR-0017). For the OAuth platforms this fetches a
  // consent URL (auth-service requests only the minimal moderation scopes for the actions
  // the platform supports) and redirects to it: Twitch and Kick grant all four actions
  // (Kick across two scopes — delete is a separate grant from ban/timeout/unban, so a
  // streamer who consented before delete existed re-consents here); YouTube does timeout and ban.
  // Discord
  // is different — its moderation authority is a guild-level BOT permission, so "enabling"
  // it is a bot RE-INVITE with the elevated permissions, not an OAuth re-consent.
  const enableModeration = useCallback(
    async (platform: string) => {
      try {
        if (platform === 'discord') {
          await startDiscordModerationReinvite()
          return
        }
        let url: string | null = null
        if (platform === 'twitch') {
          url = await moderationApi.getTwitchConsentUrl(id, ['delete', 'timeout', 'ban', 'unban'])
        } else if (platform === 'kick') {
          url = await moderationApi.getKickConsentUrl(id, ['delete', 'timeout', 'ban', 'unban'])
        } else if (platform === 'youtube') {
          // Timeout and ban share one scope (force-ssl); delete and unban are unavailable on
          // YouTube for want of a usable id, so asking for them would request nothing extra.
          url = await moderationApi.getYouTubeConsentUrl(id, ['timeout', 'ban'])
        }
        if (url) window.location.href = url
      } catch {
        toastManager.add({
          title: t('viewerOverlay.monitor.consentStartFailed'),
          type: 'error',
        })
      }
    },
    [id, t]
  )

  // Opt-in for the Twitch moderation log (channel.moderate + AutoMod holds), which is a
  // separate grant from the moderation write-path above: it reads the channel's own
  // moderation history rather than acting on it, so a streamer who granted moderation
  // before this existed still has to consent here. Owner-only — the scopes belong to the
  // broadcaster credential, which is not a moderator's to re-consent.
  const enableModLog = useCallback(async () => {
    try {
      window.location.href = await moderationApi.getTwitchModLogConsentUrl(id)
    } catch {
      toastManager.add({
        title: t('viewerOverlay.monitor.twitchConsentStartFailed'),
        type: 'error',
      })
    }
  }, [id, t])

  // A delegated moderator connecting their OWN account for a platform (ADR-0048).
  //
  // A different flow from enableModeration above, not a variant of it: it carries no
  // overlay id, because Twitch's and Kick's moderation scopes are role-based rather than
  // channel-scoped — one consent covers every streamer who delegated that platform. It
  // also requests only the delegated actions, so a volunteer is never shown a consent
  // screen asking for channel-point or subscription read on their own channel.
  const connectAsModerator = useCallback(
    async (platform: DelegatablePlatform, actions: ModerationAction[]) => {
      try {
        const url = await moderationApi.getModConsentUrl(platform, actions)
        if (url) window.location.href = url
      } catch {
        toastManager.add({
          title: t('viewerOverlay.monitor.modConnectUnavailable', { platform }),
          type: 'error',
        })
      }
    },
    [t]
  )

  // Discord's equivalent of connectAsModerator, and deliberately not the same call: the moderator
  // grants no scopes and All-Chat keeps no token — it only learns which Discord account they are,
  // so it can read their own server permissions before acting through the shared bot.
  const linkDiscordAccount = useCallback(async () => {
    try {
      await startDiscordAccountLink('moderate')
    } catch {
      toastManager.add({
        title: t('viewerOverlay.monitor.discordLinkUnavailable'),
        type: 'error',
      })
    }
  }, [t])

  // Re-consent when a send fails with `reauth_required` (the streamer's platform
  // send token expired or was revoked). Chat sending requires the advanced-controls
  // grant — Twitch `user:write:chat`, Kick `chat:write`, YouTube force-ssl — which is
  // issued ONLY by the moderation/advanced-controls re-consent. A plain login can NOT
  // restore it: login omits the send scope, skips force_verify (so Twitch silently
  // reissues the old, narrower grant), and its callback lands on the dashboard.
  // Re-running the advanced-controls consent reissues a fresh token as a superset of
  // the streamer's existing grant (force_verify) and its callback returns here
  // (/overlay/{id}/view). Mirrors enableModeration; Discord has no send path so it is
  // never the reauth target. Falls back to Twitch when no platform is reported.
  const reauthenticate = useCallback(
    async (platform?: string) => {
      try {
        let url: string | null = null
        if (platform === 'kick') {
          url = await moderationApi.getKickConsentUrl(id, ['delete', 'timeout', 'ban', 'unban'])
        } else if (platform === 'youtube') {
          url = await moderationApi.getYouTubeConsentUrl(id, ['timeout', 'ban'])
        } else {
          url = await moderationApi.getTwitchConsentUrl(id, ['delete', 'timeout', 'ban', 'unban'])
        }
        if (url) window.location.href = url
      } catch {
        toastManager.add({
          title: t('viewerOverlay.monitor.reloginStartFailed'),
          type: 'error',
        })
      }
    },
    [id, t]
  )

  // Restore the saved theme once on mount.
  useEffect(() => {
    const stored = localStorage.getItem(THEME_KEY)
    // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from localStorage
    if (stored === 'light') setLight(true)
  }, [])

  // Apply the theme: flip the body background (the layout paints dark by default)
  // and persist. Side-effects only — no setState.
  useEffect(() => {
    document.body.style.setProperty('background', light ? '#f8f9fa' : '#07070a', 'important')
    try {
      localStorage.setItem(THEME_KEY, light ? 'light' : 'dark')
    } catch {
      /* storage unavailable */
    }
  }, [light])

  // --- Moderation capability lookups ---------------------------------------

  const isOwner = capabilities?.is_owner === true
  // A delegated moderator (ADR-0048). They get the moderation controls their grant allows
  // and nothing else: no engagement, no send bar, no stream re-discovery — those are
  // ownership powers, not moderation powers.
  const isModerator = capabilities?.role === 'moderator'
  const hasRole = isOwner || isModerator
  // The moderation feature gate (ADR-0008), keyed on the OVERLAY OWNER. Someone with a
  // role but a closed gate can view the monitor but gets no controls (the endpoints would
  // 403 anyway).
  const moderationEnabled = capabilities?.can_moderate === true
  const featureGated = hasRole && capabilities?.can_moderate === false
  const capabilitiesByChannel = useMemo(() => {
    const map = new Map<string, SourceCapability>()
    capabilities?.sources.forEach((s) => map.set(s.channel_id, s))
    return map
  }, [capabilities])

  // Sources the owner could moderate but hasn't granted the scope for.
  const missingScopeSources = useMemo(
    () => (capabilities?.sources ?? []).filter((s) => s.reason === 'missing_scope'),
    [capabilities]
  )

  // Sources a delegated moderator could moderate once they connect their OWN account for
  // that platform. Consent is deferred to first use, so this is the normal state on a
  // fresh grant rather than an error.
  const needsConsentSources = useMemo(
    () => (capabilities?.sources ?? []).filter((s) => s.reason === 'needs_consent'),
    [capabilities]
  )

  // Discord sources waiting on the MODERATOR's own account link. Discord has no per-user
  // moderation API, so what they must supply is an identity rather than a consent: the shared bot
  // writes, and All-Chat checks their own server permissions before it will. Deliberately not
  // merged with needsConsentSources — that banner offers an OAuth consent, and this one a
  // different flow that grants All-Chat nothing beyond knowing who they are.
  const needsDiscordLinkSources = useMemo(
    () => (capabilities?.sources ?? []).filter((s) => s.reason === 'needs_discord_link'),
    [capabilities]
  )

  // Whether any source can currently send chat from the monitor (gates the send
  // bar). TikTok/Discord have no send path, so only twitch/youtube/kick count.
  const hasSendableSource = useMemo(
    () =>
      (capabilities?.sources ?? []).some(
        (s) =>
          s.can_send === true &&
          (s.platform === 'twitch' || s.platform === 'youtube' || s.platform === 'kick')
      ),
    [capabilities]
  )

  // YouTube stream re-discovery: owner self-service recovery for the "platform shows
  // connected but no chat" case (YouTube keeps reporting an ended/crashed stream as
  // live). Shown only to the owner when the overlay carries a YouTube source.
  const hasYouTubeSource = useMemo(
    () => Array.from(sources.values()).some((s) => s.platform === 'youtube'),
    [sources]
  )

  // Only Twitch produces mod-log events, so a Twitch source is the precondition for
  // offering the opt-in at all; whether the grant already exists is capabilities'
  // mod_log_granted (see shouldOfferModLogOptIn).
  const hasTwitchSource = useMemo(
    () => Array.from(sources.values()).some((s) => s.platform === 'twitch'),
    [sources]
  )
  const [rediscovering, setRediscovering] = useState(false)
  const handleYouTubeRediscover = useCallback(async () => {
    setRediscovering(true)
    try {
      await moderationApi.forceYouTubeRediscover(id)
      toastManager.add({ title: t('viewerOverlay.monitor.rediscoverStarted'), type: 'success' })
    } catch (err) {
      const status = err instanceof ApiError ? err.status : 0
      if (status === 429)
        toastManager.add({ title: t('viewerOverlay.monitor.rediscoverRateLimited'), type: 'error' })
      else if (status === 403)
        toastManager.add({ title: t('viewerOverlay.monitor.rediscoverForbidden'), type: 'error' })
      else toastManager.add({ title: t('viewerOverlay.monitor.rediscoverFailed'), type: 'error' })
    } finally {
      setRediscovering(false)
    }
  }, [id, t])

  // --- Optimistic moderation actions ---------------------------------------

  // A moderation action that failed because the platform token can no longer perform
  // it (missing/lapsed scope, or a Helix 401 a refresh couldn't fix). The backend asked
  // for re-consent (requires_reauth); we surface a per-platform banner so the streamer
  // can re-authorize — without it the action just dead-ends on a generic error toast,
  // and the capabilities endpoint can even keep advertising the action as available
  // (granted_scopes may overstate the real grant), so it would fail again on every click.
  const [reauthPrompt, setReauthPrompt] = useState<{ platform: string } | null>(null)

  // Copy for a moderation failure the actor can do something about (ADR-0048). Returns null when
  // the failure is not one of the delegation-aware ones, so the caller falls back to its own
  // wording — and to the re-consent banner, which is checked first because it has a CTA.
  const delegatedFailureMessage = useCallback(
    (err: unknown, platform: string): string | null => {
      switch (moderationActionCode(err)) {
        case 'connect_required':
          // Consent is deferred to first use, so for a moderator this is the expected first click,
          // not a fault. The per-source banner already offers the button; this names the reason.
          return isModerator ? t('viewerOverlay.monitor.connectRequired', { platform }) : null
        case 'owner_channel_unverified':
          // For a moderator, only the streamer can fix this, so the copy stops at the cause rather
          // than offering a button that would do nothing. An owner can hit it too (the anchor gates
          // their own path on YouTube), and there it is theirs to fix by reconnecting the account.
          return isModerator
            ? t('viewerOverlay.monitor.ownerChannelUnverifiedModerator', { platform })
            : t('viewerOverlay.monitor.ownerChannelUnverifiedOwner', { platform })
        case 'delegation_unsupported':
          return t('viewerOverlay.monitor.delegationUnsupported', { platform })
        case 'target_not_actionable':
          // Not about the caller at all — the platform protects this person from everyone, so there
          // is no CTA to offer either role.
          return t('viewerOverlay.monitor.targetNotActionable', { platform })
        // Discord's five. The shared bot performs every write there, so All-Chat's own check is
        // the only authority and these codes carry the entire explanation — which makes naming
        // the right person to ask the whole job of this copy.
        case 'discord_link_required':
          return t('viewerOverlay.monitor.discordLinkRequired')
        case 'mod_not_in_guild':
          return t('viewerOverlay.monitor.modNotInGuild')
        case 'mod_lacks_permission':
          return t('viewerOverlay.monitor.modLacksPermission')
        case 'mod_below_target':
          return t('viewerOverlay.monitor.modBelowTarget')
        case 'bot_missing_permission':
          return t('viewerOverlay.monitor.botMissingPermission')
        default:
          return null
      }
    },
    [isModerator, t]
  )

  // Apply an optimistic mark + log entry, fire the API, and roll back on error.
  const runModeration = useCallback(
    async (
      platform: string,
      meta: DeletionMetadata,
      call: () => Promise<unknown>,
      successMsg: string
    ) => {
      const sig = deletionSignature(meta)
      const clientId = crypto.randomUUID()
      const entryId = (modSeqRef.current += 1)

      // Snapshot which items this mark touches so we can revert exactly them.
      const before = itemsRef.current
      const after = applyModerationMark(before, meta)
      const touched = after.filter((it, i) => it !== before[i]).map((it) => it.id)
      setItems((prev) => applyModerationMark(prev, meta))
      pendingDeletionsRef.current.add(sig)
      setModerationLog((prev) =>
        [...prev, { id: entryId, clientId, ...toModEntry(meta, 'live', Date.now()) }].slice(
          -MAX_MOD_LOG
        )
      )

      try {
        await call()
        toastManager.add({ title: successMsg, type: 'success' })
      } catch (err) {
        // Roll back: drop the optimistic entry + clear the dedup signature, and
        // un-mark exactly the items we struck through.
        pendingDeletionsRef.current.delete(sig)
        setModerationLog((prev) => prev.filter((e) => e.clientId !== clientId))
        const touchedSet = new Set(touched)
        setItems((prev) =>
          prev.map((it) => (touchedSet.has(it.id) ? { ...it, _moderated: undefined } : it))
        )
        const delegated = delegatedFailureMessage(err, platform)
        if (isModerationReauthError(err)) {
          // Actionable failure: the actor must re-authorize this platform. Show the
          // recovery banner and a toast pointing at it (mirrors the chat-send reauth path).
          setReauthPrompt({ platform })
          toastManager.add({
            title: t('viewerOverlay.monitor.reauthNeededToast', { platform }),
            type: 'error',
          })
        } else if (delegated !== null) {
          toastManager.add({ title: delegated, type: 'error' })
        } else {
          toastManager.add({ title: t('viewerOverlay.monitor.actionFailed'), type: 'error' })
        }
      }
    },
    [delegatedFailureMessage, t]
  )

  const handleDelete = useCallback(
    (item: ViewItem) => {
      const req = buildDeleteRequest(item)
      const meta: DeletionMetadata = {
        deletion_type: 'single',
        target_uuid: req.target_uuid,
        target_msg_id: req.native_message_id,
      }
      void runModeration(
        item.platform,
        meta,
        () => moderationApi.deleteMessage(id, req),
        t('viewerOverlay.monitor.messageDeleted')
      )
    },
    [id, runModeration, t]
  )

  const handleTimeout = useCallback(
    (item: ViewItem, durationSeconds: number) => {
      const req = buildTimeoutRequest(item, durationSeconds)
      const meta: DeletionMetadata = {
        deletion_type: 'batch',
        target_user_id: req.target_user_id,
        target_username: req.target_username,
        ban_duration: durationSeconds,
      }
      void runModeration(
        item.platform,
        meta,
        () => moderationApi.timeoutUser(id, req),
        t('viewerOverlay.monitor.timedOut', {
          name: req.target_username || t('viewerOverlay.monitor.unnamedTarget'),
        })
      )
    },
    [id, runModeration, t]
  )

  const handleBan = useCallback(
    (item: ViewItem) => {
      const req = buildBanRequest(item)
      const meta: DeletionMetadata = {
        deletion_type: 'batch',
        target_user_id: req.target_user_id,
        target_username: req.target_username,
        ban_duration: 0,
      }
      void runModeration(
        item.platform,
        meta,
        () => moderationApi.banUser(id, req),
        t('viewerOverlay.monitor.banned', {
          name: req.target_username || t('viewerOverlay.monitor.unnamedTarget'),
        })
      )
    },
    [id, runModeration, t]
  )

  // Unban has no message-level visual mark; just fire and toast.
  const handleUnban = useCallback(
    (item: ViewItem) => {
      const name =
        item.user?.display_name || item.user?.username || t('viewerOverlay.monitor.unnamedTarget')
      moderationApi
        .unbanUser(id, buildUnbanRequest(item))
        .then(() =>
          toastManager.add({
            title: t('viewerOverlay.monitor.unbanned', { name }),
            type: 'success',
          })
        )
        .catch((err) => {
          const delegated = delegatedFailureMessage(err, item.platform)
          if (isModerationReauthError(err)) {
            setReauthPrompt({ platform: item.platform })
            toastManager.add({
              title: t('viewerOverlay.monitor.reauthNeededToast', {
                platform: item.platform,
              }),
              type: 'error',
            })
          } else if (delegated !== null) {
            toastManager.add({ title: delegated, type: 'error' })
          } else {
            toastManager.add({ title: t('viewerOverlay.monitor.unbanFailed'), type: 'error' })
          }
        })
    },
    [id, delegatedFailureMessage, t]
  )

  // Only owners in the rollout cohort get live action callbacks; everyone else
  // (non-owners, or owners the feature gate hasn't reached) views read-only.
  const moderation: ChatPanelModeration | undefined = moderationEnabled
    ? {
        onDelete: handleDelete,
        onTimeout: handleTimeout,
        onBan: handleBan,
        onUnban: handleUnban,
      }
    : undefined

  const { chat, events, system } = partitionItems(items)
  const sourceNames = Array.from(sources.values()).map((s) => s.channelName)
  const title = sourceNames.length > 0 ? sourceNames.join(' · ') : 'Overlay Monitor'

  // The header's controls, identical in both modes. The wide header lays them
  // out with flex-wrap; the dock header puts the same nodes inside one overflow
  // menu, minus LayoutPicker — with no split there is no layout to pick.
  const headerControls = (
    <>
      {sources.size > 0 && (
        <PlatformStatusIndicators
          configuredSources={sources}
          activeChannels={activeChannels}
          channelStatuses={channelStatuses}
          variant="inline"
        />
      )}
      <MaintenanceInfoButton />
      <button
        onClick={() => setShowDetails((v) => !v)}
        aria-pressed={showDetails}
        className={clsx(
          'flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
          showDetails
            ? 'border-border-md bg-surface-2 text-text'
            : 'border-border text-text-sub hover:border-border-md hover:text-text'
        )}
      >
        <SlidersHorizontal className="h-3.5 w-3.5" />
        {t('viewerOverlay.monitor.details')}
      </button>
      {isOwner && (
        <button
          onClick={() => setShowEngagement((v) => !v)}
          aria-pressed={showEngagement}
          title={t('viewerOverlay.monitor.engagementTitle')}
          className={clsx(
            'flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
            showEngagement
              ? 'border-border-md bg-surface-2 text-text'
              : 'border-border text-text-sub hover:border-border-md hover:text-text'
          )}
        >
          <BarChart3 className="h-3.5 w-3.5" />
          {t('viewerOverlay.monitor.engagement')}
        </button>
      )}
      {!dock && <LayoutPicker layout={layout} onChange={updateLayout} />}
      <ViewSettingsBar
        prefs={prefs}
        onChange={updatePrefs}
        onTestActivitySound={testActivitySound}
      />
      <OverlayViewThemeToggle light={light} onToggle={() => setLight((v) => !v)} />
      {isOwner && hasYouTubeSource && (
        <Button
          onClick={handleYouTubeRediscover}
          disabled={rediscovering}
          title={t('viewerOverlay.monitor.rediscoverYouTubeTitle')}
          variant="outline"
          size="sm"
        >
          <RotateCw className={clsx('h-3.5 w-3.5', rediscovering && 'animate-spin')} />
          {t('viewerOverlay.monitor.rediscoverYouTube')}
        </Button>
      )}
      {/* Leaves the monitor. In a dock that is a chromeless panel with no back
          button, so it must open a real browser window rather than replace the
          panel; noopener is explicit because OBS embeds an older CEF. */}
      <Link
        href={`/overlay/${id}`}
        target="_blank"
        rel={dock ? 'noopener noreferrer' : 'noreferrer'}
        className="flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
      >
        <ExternalLink className="h-3.5 w-3.5" />
        {t('viewerOverlay.monitor.obsOverlay')}
      </Link>
    </>
  )

  // Which notice strips apply right now. Hoisted out of the JSX below so the
  // dock's collapsed status row can count them without restating a single one of
  // these conditions — two copies of "is this notice showing?" is how a notice
  // ends up counted but not rendered, or rendered but not counted.
  const showStillReconnecting =
    connectionStatus === 'reconnecting' && reconnectAttempts >= OFFLINE_THRESHOLD
  const showNoRole = capabilities !== null && !hasRole
  const consentNotices = moderationEnabled && isModerator ? needsConsentSources : []
  const showDiscordLink = moderationEnabled && isModerator && needsDiscordLinkSources.length > 0
  const missingScopeNotices = moderationEnabled && isOwner ? missingScopeSources : []
  const showModLogOptIn = shouldOfferModLogOptIn({
    isOwner,
    hasTwitchSource,
    modLogGranted: capabilities?.mod_log_granted,
  })
  const noticeCount =
    Number(showStillReconnecting) +
    Number(replayTruncated) +
    Number(showNoRole) +
    Number(featureGated) +
    consentNotices.length +
    Number(showDiscordLink) +
    missingScopeNotices.length +
    Number(showModLogOptIn) +
    Number(reauthPrompt !== null)

  // Every notice the wide view would stack, unchanged. The dock renders the same
  // fragment inside one collapsed status row instead of eight full-width bars.
  const notices = (
    <>
      {/* Sustained-reconnect reassurance. The badge escalates to red at four
          consecutive failures (~13s), which a redeploy routinely outlasts. The
          badge has room for two words; this is where the sentence goes, and it
          exists because the intuitive response to a red badge — close the
          overlay and reopen it — discards the watermark and causes exactly the
          loss the badge is warning about. Monitor only: the OBS overlay is a
          chat feed on a live stream, not a diagnostics surface. */}
      {showStillReconnecting && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          {t('viewerOverlay.monitor.stillReconnecting')}
        </div>
      )}

      {/* Truncated-replay notice: the gateway told us our watermark predates its
          buffer, so part of the gap is unrecoverable. Sticky, because the hole
          it describes stays in the feed above. */}
      {replayTruncated && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          {t('viewerOverlay.monitor.replayTruncated')}
        </div>
      )}

      {/* No-role notice: viewing is allowed, moderation is not. Says nothing about the
          overlay itself — the payload behind it is identical for an overlay that does not
          exist, so it must not be phrased as a fact about this one. */}
      {showNoRole && (
        <div className="flex items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          {t('viewerOverlay.monitor.noRole')}
        </div>
      )}

      {/* Feature-gated notice. The gate is keyed on the OWNER, so a moderator is told
          whose plan it is and given no call to action: /upgrade would sell them a plan
          that is not theirs to buy. */}
      {featureGated &&
        (isOwner ? (
          <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
            <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
            <span>{t('viewerOverlay.monitor.featureGatedOwner')}</span>
            <Link
              href="/upgrade"
              className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              {t('viewerOverlay.monitor.featureGatedUpgrade')}
            </Link>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
            <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
            <span>{t('viewerOverlay.monitor.featureGatedModerator')}</span>
          </div>
        ))}

      {/* Connect-to-moderate notices: a delegated moderator acts with their OWN account,
          and consent is deferred to the first time they need it — so this is the normal
          state on a fresh grant rather than an error. */}
      {consentNotices.map((s) => (
        <div
          key={s.channel_id}
          className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub"
        >
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>
            {s.channel_name
              ? t('viewerOverlay.monitor.needsConsentChannel', {
                  platform: s.platform,
                  channel: s.channel_name,
                })
              : t('viewerOverlay.monitor.needsConsent', { platform: s.platform })}
          </span>
          {s.platform === 'twitch' || s.platform === 'kick' || s.platform === 'youtube' ? (
            <Button
              type="button"
              onClick={() =>
                void connectAsModerator(
                  s.platform as DelegatablePlatform,
                  capabilities?.delegated_actions ?? []
                )
              }
              variant="link"
              className="h-auto p-0 font-medium"
            >
              {t('viewerOverlay.monitor.connectPlatform', { platform: s.platform })}
            </Button>
          ) : null}
        </div>
      ))}

      {/* Discord account-link notices. One banner for the whole overlay rather than one per
          source: the link is per PERSON, not per server, so repeating it per channel would offer
          the same one-time action several times over. */}
      {showDiscordLink && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>{t('viewerOverlay.monitor.needsDiscordLink')}</span>
          <Button
            type="button"
            onClick={() => void linkDiscordAccount()}
            variant="link"
            className="h-auto p-0 font-medium"
          >
            {t('viewerOverlay.monitor.linkDiscord')}
          </Button>
        </div>
      )}

      {/* Missing-scope notices: owner must grant permissions per platform. Owner-only —
          the flow behind it re-consents the STREAMER's broadcaster credential, which is
          not a moderator's to re-consent. */}
      {missingScopeNotices.map((s) => (
        <div
          key={s.channel_id}
          className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub"
        >
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>{missingScopeNotice(t, s.platform, s.channel_name)}</span>
          {s.platform === 'twitch' ||
          s.platform === 'kick' ||
          s.platform === 'youtube' ||
          s.platform === 'discord' ? (
            <Button
              type="button"
              onClick={() => enableModeration(s.platform)}
              variant="link"
              className="h-auto p-0 font-medium"
            >
              {s.platform === 'discord'
                ? t('viewerOverlay.monitor.reinviteBot')
                : t('viewerOverlay.monitor.enableModeration')}
            </Button>
          ) : (
            <span className="text-text-dim">
              {t('viewerOverlay.monitor.comingSoonFor', { platform: s.platform })}
            </span>
          )}
        </div>
      ))}

      {/* Twitch moderation-log opt-in. The scope note is not padding: the consent screen
          asks for moderator:manage:automod, which on a read-only feature looks like a
          mistake and gets declined — Twitch requires it to create the AutoMod hold
          subscription and offers no read-only alternative. */}
      {showModLogOptIn && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>{t('viewerOverlay.monitor.modLogOptIn')}</span>
          <Button
            type="button"
            onClick={() => void enableModLog()}
            variant="link"
            className="h-auto p-0 font-medium"
          >
            {t('viewerOverlay.monitor.enableModLog')}
          </Button>
        </div>
      )}

      {/* Re-auth prompt: a moderation action failed because the platform token can no
          longer perform it. The backend asked for re-consent; give the actor the CTA —
          which differs by role. Sending a moderator down the streamer's re-consent path
          would half-succeed and then error: that flow is an add-source state, and
          add-source 404s for anyone who does not own the overlay. */}
      {reauthPrompt && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>
            {isOwner
              ? t('viewerOverlay.monitor.reauthOwner', { platform: reauthPrompt.platform })
              : t('viewerOverlay.monitor.reauthModerator', { platform: reauthPrompt.platform })}
          </span>
          <Button
            type="button"
            onClick={() => {
              const platform = reauthPrompt.platform
              setReauthPrompt(null)
              if (isModerator) {
                void connectAsModerator(
                  platform as DelegatablePlatform,
                  capabilities?.delegated_actions ?? []
                )
                return
              }
              void enableModeration(platform)
            }}
            variant="link"
            className="h-auto p-0 font-medium"
          >
            {reauthPrompt.platform === 'discord'
              ? t('viewerOverlay.monitor.reinviteBot')
              : isModerator
                ? t('viewerOverlay.monitor.reconnectPlatform', {
                    platform: reauthPrompt.platform,
                  })
                : t('viewerOverlay.monitor.reauthorizeModeration')}
          </Button>
        </div>
      )}
    </>
  )

  return (
    <div
      id="overlay-view-root"
      className={clsx('overlay-view flex h-screen min-h-0 flex-col', light && 'light')}
    >
      {/* Header. Dock mode keeps ONE non-wrapping row: at dock width the wide
          header's flex-wrap gives roughly one control per line, and the panel is
          then all header and no chat. */}
      {dock ? (
        <header className="flex items-center gap-2 border-b border-border bg-surface px-3 py-2">
          <h1 className="min-w-0 flex-1 truncate text-xs font-semibold text-text" title={title}>
            {title}
          </h1>
          <ConnectionBadge status={connectionStatus} attempts={reconnectAttempts} />
          <DockOverflowMenu>{headerControls}</DockOverflowMenu>
        </header>
      ) : (
        <header className="flex flex-wrap items-center gap-3 border-b border-border bg-surface px-4 py-2">
          <div className="flex min-w-0 items-center gap-3">
            <h1 className="min-w-0 truncate text-sm font-semibold text-text" title={title}>
              {title}
            </h1>
            <ConnectionBadge status={connectionStatus} attempts={reconnectAttempts} />
          </div>

          <div className="ml-auto flex flex-wrap items-center gap-2">{headerControls}</div>
        </header>
      )}

      {dock ? <DockNoticeBar count={noticeCount}>{notices}</DockNoticeBar> : notices}

      {showDetails && (
        <ObservabilitySummary
          config={config}
          sources={sources}
          activeChannels={activeChannels}
          eventSettings={eventSettings}
          observedEventTypes={observedEventTypes}
        />
      )}

      {/* Poll & prediction controls — owner only (issue #523). key={id} remounts on an
          in-tab overlay switch so a finished round + half-typed create form don't leak
          across overlays (L-C5). */}
      {isOwner && showEngagement && <EngagementControls key={id} overlayId={id} />}

      {/* Chat | Activity. Two side-by-side columns are ~150px each at dock width,
          so the dock switches between them instead of splitting. Only the active
          panel is mounted: its scrollback comes from `items` and is rebuilt on
          return, and keeping the other one alive behind `hidden` would leave it
          measuring a zero-height box. */}
      {dock ? (
        <>
          <DockTabPicker tab={dockTab} onChange={updateDockTab} />
          {/* Same box ResizableSplit gives each panel: both are `h-full min-h-0`
              and need a sized parent to scroll inside instead of growing. */}
          <div id={DOCK_PANEL_ID} role="tabpanel" className="min-h-0 flex-1 overflow-hidden">
            {dockTab === 'chat' ? (
              <ChatPanel
                items={chat}
                prefs={prefs}
                capabilities={capabilitiesByChannel}
                moderation={moderation}
              />
            ) : (
              <ActivityPanel events={events} system={system} moderationLog={moderationLog} />
            )}
          </div>
        </>
      ) : (
        <ResizableSplit
          storageKey={`overlay-view-split-${id}`}
          orientation={LAYOUT_CONFIG[layout].orientation}
          reversed={LAYOUT_CONFIG[layout].reversed}
          left={
            <ChatPanel
              items={chat}
              prefs={prefs}
              capabilities={capabilitiesByChannel}
              moderation={moderation}
            />
          }
          right={<ActivityPanel events={events} system={system} moderationLog={moderationLog} />}
        />
      )}

      {/* Send bar — owner only, and only when ≥1 platform source can send. */}
      {isOwner && hasSendableSource && capabilities && (
        <ChatSendBar
          sources={capabilities.sources}
          onEnable={enableModeration}
          onReauth={reauthenticate}
        />
      )}
    </div>
  )
}
