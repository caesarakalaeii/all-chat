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
 */

'use client'

import clsx from 'clsx'
import { BarChart3, ExternalLink, Info, RotateCw, SlidersHorizontal } from 'lucide-react'
import Link from 'next/link'
import { use, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { toastManager } from '@/lib/toast'

import { MaintenanceInfoButton } from '@/components/MaintenanceInfoButton'
import PlatformStatusIndicators from '@/components/PlatformStatusIndicators'
import { ActivityPanel } from '@/components/overlay/ActivityPanel'
import { ChatPanel, type ChatPanelModeration } from '@/components/overlay/ChatPanel'
import { ChatSendBar } from '@/components/overlay/ChatSendBar'
import { ConnectionBadge } from '@/components/overlay/ConnectionBadge'
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
  mergeByAgg,
  partitionItems,
  toModEntry,
  type ModEntry,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'
import { OFFLINE_THRESHOLD } from '@/lib/utils/connectionStatusLabel'
import { createSoundPlayer, type SoundPlayer, type SoundSettings } from '@/lib/utils/soundPlayer'

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
  const { id } = use(params)

  const [items, setItems] = useState<ViewItem[]>([])
  const [moderationLog, setModerationLog] = useState<ModEntry[]>([])
  const [observedEventTypes, setObservedEventTypes] = useState<Set<string>>(new Set())
  const [eventSettings, setEventSettings] = useState<EventSettings | null>(null)
  const [light, setLight] = useState(false)
  const [showDetails, setShowDetails] = useState(false)
  const [showEngagement, setShowEngagement] = useState(false)
  const [prefs, setPrefs] = useState<MonitorViewPrefs>(DEFAULT_VIEW_PREFS)
  const [layout, setLayout] = useState<ViewLayout>(DEFAULT_VIEW_LAYOUT)
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
          title: 'Could not start moderation setup. Please try again.',
          type: 'error',
        })
      }
    },
    [id]
  )

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
          title: `Connecting ${platform} is not available yet. Ask the streamer to moderate there for now.`,
          type: 'error',
        })
      }
    },
    []
  )

  // Discord's equivalent of connectAsModerator, and deliberately not the same call: the moderator
  // grants no scopes and All-Chat keeps no token — it only learns which Discord account they are,
  // so it can read their own server permissions before acting through the shared bot.
  const linkDiscordAccount = useCallback(async () => {
    try {
      await startDiscordAccountLink('moderate')
    } catch {
      toastManager.add({
        title:
          'Linking Discord is not available right now. Ask the streamer to moderate there for now.',
        type: 'error',
      })
    }
  }, [])

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
        toastManager.add({ title: 'Could not start re-login. Please try again.', type: 'error' })
      }
    },
    [id]
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
  const [rediscovering, setRediscovering] = useState(false)
  const handleYouTubeRediscover = useCallback(async () => {
    setRediscovering(true)
    try {
      await moderationApi.forceYouTubeRediscover(id)
      toastManager.add({ title: 'Re-discovering YouTube stream…', type: 'success' })
    } catch (err) {
      const status = err instanceof ApiError ? err.status : 0
      if (status === 429)
        toastManager.add({ title: 'Please wait a moment before retrying', type: 'error' })
      else if (status === 403)
        toastManager.add({ title: 'Not authorized for this overlay', type: 'error' })
      else toastManager.add({ title: 'Could not trigger re-discovery', type: 'error' })
    } finally {
      setRediscovering(false)
    }
  }, [id])

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
          return isModerator ? `Connect your own ${platform} account to moderate here` : null
        case 'owner_channel_unverified':
          // For a moderator, only the streamer can fix this, so the copy stops at the cause rather
          // than offering a button that would do nothing. An owner can hit it too (the anchor gates
          // their own path on YouTube), and there it is theirs to fix by reconnecting the account.
          return isModerator
            ? `This streamer's ${platform} account isn't connected, so nothing can be moderated here`
            : `Your ${platform} account isn't connected for this channel — reconnect it to moderate here`
        case 'delegation_unsupported':
          return `Moderators can't act on ${platform} yet — ask the streamer to handle this one`
        case 'target_not_actionable':
          // Not about the caller at all — the platform protects this person from everyone, so there
          // is no CTA to offer either role.
          return `${platform} won't let anyone moderate this person — they're the channel owner or another moderator`
        // Discord's five. The shared bot performs every write there, so All-Chat's own check is
        // the only authority and these codes carry the entire explanation — which makes naming
        // the right person to ask the whole job of this copy.
        case 'discord_link_required':
          return 'Link your Discord account to moderate here'
        case 'mod_not_in_guild':
          return "You're not in this Discord server — ask the streamer to invite you"
        case 'mod_lacks_permission':
          return "Your Discord roles don't allow this — ask the streamer for a role that does"
        case 'mod_below_target':
          return "Discord's role hierarchy blocks this — your highest role has to sit above theirs"
        case 'bot_missing_permission':
          return "The All-Chat bot wasn't given this Discord permission — ask the streamer to re-invite it"
        default:
          return null
      }
    },
    [isModerator]
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
            title: `${platform} needs you to re-authorize moderation`,
            type: 'error',
          })
        } else if (delegated !== null) {
          toastManager.add({ title: delegated, type: 'error' })
        } else {
          toastManager.add({ title: 'Moderation action failed', type: 'error' })
        }
      }
    },
    [delegatedFailureMessage]
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
        'Message deleted'
      )
    },
    [id, runModeration]
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
        `Timed out ${req.target_username || 'user'}`
      )
    },
    [id, runModeration]
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
        `Banned ${req.target_username || 'user'}`
      )
    },
    [id, runModeration]
  )

  // Unban has no message-level visual mark; just fire and toast.
  const handleUnban = useCallback(
    (item: ViewItem) => {
      const name = item.user?.display_name || item.user?.username || 'user'
      moderationApi
        .unbanUser(id, buildUnbanRequest(item))
        .then(() => toastManager.add({ title: `Unbanned ${name}`, type: 'success' }))
        .catch((err) => {
          const delegated = delegatedFailureMessage(err, item.platform)
          if (isModerationReauthError(err)) {
            setReauthPrompt({ platform: item.platform })
            toastManager.add({
              title: `${item.platform} needs you to re-authorize moderation`,
              type: 'error',
            })
          } else if (delegated !== null) {
            toastManager.add({ title: delegated, type: 'error' })
          } else {
            toastManager.add({ title: 'Unban failed', type: 'error' })
          }
        })
    },
    [id, delegatedFailureMessage]
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

  return (
    <div
      id="overlay-view-root"
      className={clsx('overlay-view flex h-screen min-h-0 flex-col', light && 'light')}
    >
      {/* Header */}
      <header className="flex flex-wrap items-center gap-3 border-b border-border bg-surface px-4 py-2">
        <div className="flex min-w-0 items-center gap-3">
          <h1 className="min-w-0 truncate text-sm font-semibold text-text" title={title}>
            {title}
          </h1>
          <ConnectionBadge status={connectionStatus} attempts={reconnectAttempts} />
        </div>

        <div className="ml-auto flex flex-wrap items-center gap-2">
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
            Details
          </button>
          {isOwner && (
            <button
              onClick={() => setShowEngagement((v) => !v)}
              aria-pressed={showEngagement}
              title="Run polls and predictions for this overlay"
              className={clsx(
                'flex items-center gap-2 rounded-lg border px-3 py-1.5 text-xs font-medium transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                showEngagement
                  ? 'border-border-md bg-surface-2 text-text'
                  : 'border-border text-text-sub hover:border-border-md hover:text-text'
              )}
            >
              <BarChart3 className="h-3.5 w-3.5" />
              Engagement
            </button>
          )}
          <LayoutPicker layout={layout} onChange={updateLayout} />
          <ViewSettingsBar
            prefs={prefs}
            onChange={updatePrefs}
            onTestActivitySound={testActivitySound}
          />
          <OverlayViewThemeToggle light={light} onToggle={() => setLight((v) => !v)} />
          {isOwner && hasYouTubeSource && (
            <button
              onClick={handleYouTubeRediscover}
              disabled={rediscovering}
              title="Force YouTube to re-discover the live stream — use if chat stopped after a stream crash or restart"
              className="flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
            >
              <RotateCw className={clsx('h-3.5 w-3.5', rediscovering && 'animate-spin')} />
              Re-discover YouTube
            </button>
          )}
          <Link
            href={`/overlay/${id}`}
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-2 rounded-lg border border-border px-3 py-1.5 text-xs font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            <ExternalLink className="h-3.5 w-3.5" />
            OBS overlay
          </Link>
        </div>
      </header>

      {/* Sustained-reconnect reassurance. The badge escalates to red at four
          consecutive failures (~13s), which a redeploy routinely outlasts. The
          badge has room for two words; this is where the sentence goes, and it
          exists because the intuitive response to a red badge — close the
          overlay and reopen it — discards the watermark and causes exactly the
          loss the badge is warning about. Monitor only: the OBS overlay is a
          chat feed on a live stream, not a diagnostics surface. */}
      {connectionStatus === 'reconnecting' && reconnectAttempts >= OFFLINE_THRESHOLD && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          Still reconnecting — this recovers on its own, and messages sent meanwhile replay when
          the connection returns. Closing this page is what loses them.
        </div>
      )}

      {/* Truncated-replay notice: the gateway told us our watermark predates its
          buffer, so part of the gap is unrecoverable. Sticky, because the hole
          it describes stays in the feed above. */}
      {replayTruncated && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          Some earlier messages may be missing — the disconnection outlasted the replay buffer, so
          the oldest part of the gap could not be recovered.
        </div>
      )}

      {/* No-role notice: viewing is allowed, moderation is not. Says nothing about the
          overlay itself — the payload behind it is identical for an overlay that does not
          exist, so it must not be phrased as a fact about this one. */}
      {capabilities && !hasRole && (
        <div className="flex items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          You can view this monitor, but you don&apos;t moderate here — moderation is disabled.
        </div>
      )}

      {/* Feature-gated notice. The gate is keyed on the OWNER, so a moderator is told
          whose plan it is and given no call to action: /upgrade would sell them a plan
          that is not theirs to buy. */}
      {featureGated &&
        (isOwner ? (
          <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
            <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
            <span>Chat moderation is a premium feature.</span>
            <Link
              href="/upgrade"
              className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
            >
              Upgrade to moderate from your overlay
            </Link>
          </div>
        ) : (
          <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
            <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
            <span>
              This streamer&apos;s plan doesn&apos;t include moderation right now, so your actions
              are unavailable until they renew it.
            </span>
          </div>
        ))}

      {/* Connect-to-moderate notices: a delegated moderator acts with their OWN account,
          and consent is deferred to the first time they need it — so this is the normal
          state on a fresh grant rather than an error. */}
      {moderationEnabled &&
        isModerator &&
        needsConsentSources.map((s) => (
          <div
            key={s.channel_id}
            className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub"
          >
            <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
            <span>
              Connect your own {s.platform} account to moderate
              {s.channel_name ? ` ${s.channel_name}` : ''}.
            </span>
            {s.platform === 'twitch' || s.platform === 'kick' || s.platform === 'youtube' ? (
              <button
                type="button"
                onClick={() =>
                  void connectAsModerator(
                    s.platform as DelegatablePlatform,
                    capabilities?.delegated_actions ?? []
                  )
                }
                className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                Connect {s.platform}
              </button>
            ) : null}
          </div>
        ))}

      {/* Discord account-link notices. One banner for the whole overlay rather than one per
          source: the link is per PERSON, not per server, so repeating it per channel would offer
          the same one-time action several times over. */}
      {moderationEnabled && isModerator && needsDiscordLinkSources.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>
            Link your Discord account to moderate Discord here — All-Chat checks your own server
            permissions before acting.
          </span>
          <button
            type="button"
            onClick={() => void linkDiscordAccount()}
            className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            Link Discord
          </button>
        </div>
      )}

      {/* Missing-scope notices: owner must grant permissions per platform. Owner-only —
          the flow behind it re-consents the STREAMER's broadcaster credential, which is
          not a moderator's to re-consent. */}
      {moderationEnabled &&
        isOwner &&
        missingScopeSources.map((s) => (
          <div
            key={s.channel_id}
            className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub"
          >
            <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
            <span>
              {s.platform === 'discord'
                ? `Re-invite the bot with moderation permissions to enable mod actions for ${s.platform}`
                : `Grant moderation permissions to enable mod actions for ${s.platform}`}
              {s.channel_name ? ` (${s.channel_name})` : ''}.
            </span>
            {s.platform === 'twitch' ||
            s.platform === 'kick' ||
            s.platform === 'youtube' ||
            s.platform === 'discord' ? (
              <button
                type="button"
                onClick={() => enableModeration(s.platform)}
                className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                {s.platform === 'discord'
                  ? 'Re-invite the bot'
                  : 'Enable moderation & chat sending'}
              </button>
            ) : (
              <span className="text-text-dim">(coming soon for {s.platform})</span>
            )}
          </div>
        ))}

      {/* Re-auth prompt: a moderation action failed because the platform token can no
          longer perform it. The backend asked for re-consent; give the actor the CTA —
          which differs by role. Sending a moderator down the streamer's re-consent path
          would half-succeed and then error: that flow is an add-source state, and
          add-source 404s for anyone who does not own the overlay. */}
      {reauthPrompt && (
        <div className="flex flex-wrap items-center gap-2 border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-sub">
          <Info className="h-3.5 w-3.5 shrink-0 text-text-dim" />
          <span>
            Your {reauthPrompt.platform} moderation permission expired or was never granted —
            re-authorize to keep moderating {isOwner ? 'from your overlay' : 'here'}.
          </span>
          <button
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
            className="font-medium text-twitch hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
          >
            {reauthPrompt.platform === 'discord'
              ? 'Re-invite the bot'
              : isModerator
                ? `Reconnect ${reauthPrompt.platform}`
                : 'Re-authorize moderation & chat sending'}
          </button>
        </div>
      )}

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

      {/* Resizable Chat | Activity — orientation/order driven by the layout picker. */}
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
