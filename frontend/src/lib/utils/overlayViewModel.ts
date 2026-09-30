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
 * Pure view-model helpers for the overlay observability view (`.../view`).
 *
 * The view keeps a large scrollback (unlike the OBS overlay), never filters,
 * and instead of removing moderated messages it marks them and logs the action.
 * These helpers express that policy without React so they can be unit-tested.
 */

import type { ChatMessage, DeletionMetadata, EventType } from '@/lib/types/message'

/** A chat message held in the view, possibly annotated with a moderation mark. */
export interface ViewItem extends ChatMessage {
  /** Present when a moderator deleted/timed-out/banned this message; kept visible, struck-through. */
  _moderated?: { kind: ModKind; banDuration?: number }
}

/**
 * How a moderation-log row renders. The first four are derived from deletions;
 * `automod` covers an AutoMod hold and its later resolution, and `action` is the
 * catch-all for every other `channel.moderate` action — including ones Twitch
 * has not shipped yet, which must still produce a row.
 */
export type ModKind = 'delete' | 'timeout' | 'ban' | 'clear' | 'automod' | 'action'

/** A moderation-log entry derived from a deletion. `id` is assigned by the consumer. */
export interface ModEntryData {
  kind: ModKind
  username?: string
  targetUserId?: string
  targetUuid?: string
  /**
   * The deleted message's platform-native id, when the frame carried one. On
   * Twitch a single-message delete reaches this view twice — as a
   * `message_deletion` and as a `channel.moderate` mod_action carrying the
   * acting moderator — and this id is the join key between the two rows.
   */
  targetMsgId?: string
  banDuration?: number
  source: 'replay' | 'live'
  at: number
  /**
   * The raw `channel.moderate` action, verbatim from Twitch, or one of the two
   * synthesized AutoMod actions (`automod_hold`, `automod_resolved`). Absent on
   * deletion-derived entries, which carry no Twitch action name.
   */
  action?: string
  /** Login of the moderator who acted. Absent for AutoMod holds and for deletions. */
  moderator?: string
  reason?: string
  /** Join key between an AutoMod hold and the resolution that closes it. */
  heldMessageId?: string
  heldText?: string
  automodCategory?: string
  automodLevel?: number
  resolution?: 'approved' | 'denied' | 'expired'
  /** Login of whoever resolved a hold. Empty when the hold simply expired. */
  resolvedBy?: string
  /**
   * Set once this row has consumed the `channel.moderate` half of its action
   * pair (timeout/ban/clear), so a second action against the same user inside
   * the fold window appends instead of folding into a completed pair. Not used
   * for delete, whose join key (one native id, one removal) cannot recur.
   */
  paired?: boolean
  /**
   * Set on entries created optimistically by a moderator action in this view,
   * so the matching server-pushed deletion can be deduped and the entry can be
   * rolled back if the action fails. Absent on entries derived purely from WS.
   */
  clientId?: string
}

/** A moderation-log entry with a stable id for rendering. */
export interface ModEntry extends ModEntryData {
  id: number
}

/** Event types that are operational notices rather than audience activity. */
const SYSTEM_EVENT_TYPES: ReadonlySet<EventType> = new Set<EventType>([
  'token_expiration_warning',
  'source_permission_error',
  'listener_deprecation_notice',
])

/**
 * Whether an item is audience activity — i.e. it would land in the `events`
 * pane of {@link partitionItems} (a sub, bit, raid, super chat, gift/rose,
 * follow, …). Excludes plain chat, operational system notices, and deletions.
 * Kept in lock-step with `partitionItems` so a sound plays exactly when a new
 * row appears in the Activity & Events panel, and never otherwise.
 */
export function isAudienceEvent(item: ChatMessage): boolean {
  const type = item.event?.type
  if (!type) return false
  return !SYSTEM_EVENT_TYPES.has(type) && type !== 'message_deletion'
}

/**
 * Split the held items into the three panes the view renders:
 *  - chat:   regular messages (no event)
 *  - events: audience activity (subs, bits, raids, super chats, gifts, follows, …)
 *  - system: operational notices (token expiry, source permission errors)
 * `message_deletion` never appears here (it is handled as a moderation action,
 * never appended as an item) but is excluded defensively.
 */
export function partitionItems(items: ViewItem[]): {
  chat: ViewItem[]
  events: ViewItem[]
  system: ViewItem[]
} {
  const chat: ViewItem[] = []
  const events: ViewItem[] = []
  const system: ViewItem[] = []
  for (const item of items) {
    const type = item.event?.type
    if (!item.event) {
      chat.push(item)
    } else if (type && SYSTEM_EVENT_TYPES.has(type)) {
      system.push(item)
    } else if (type !== 'message_deletion') {
      events.push(item)
    }
  }
  return { chat, events, system }
}

/** Classify a deletion into a moderation-log kind. */
export function deletionKind(meta: DeletionMetadata): ModKind {
  switch (meta.deletion_type) {
    case 'single':
      return 'delete'
    case 'batch':
      return meta.ban_duration && meta.ban_duration > 0 ? 'timeout' : 'ban'
    case 'clear':
      return 'clear'
    default:
      return 'delete'
  }
}

/** Build a moderation-log entry (without an id — the consumer assigns one). */
export function toModEntry(
  meta: DeletionMetadata,
  source: 'replay' | 'live',
  at: number
): ModEntryData {
  return {
    kind: deletionKind(meta),
    username: meta.target_username,
    targetUserId: meta.target_user_id,
    targetUuid: meta.target_uuid,
    targetMsgId: meta.target_msg_id,
    banDuration: meta.ban_duration,
    source,
    at,
  }
}

/** Read a string field off untyped WebSocket metadata, or undefined if absent/blank. */
function metadataString(metadata: Record<string, unknown>, key: string): string | undefined {
  const value = metadata[key]
  return typeof value === 'string' && value !== '' ? value : undefined
}

/** Read a number field off untyped WebSocket metadata, or undefined if absent. */
function metadataNumber(metadata: Record<string, unknown>, key: string): number | undefined {
  const value = metadata[key]
  return typeof value === 'number' ? value : undefined
}

/** How a `channel.moderate` action renders. Unknown actions fall through to 'action'. */
function modActionKind(action: string): ModKind {
  switch (action) {
    case 'timeout':
      return 'timeout'
    case 'ban':
      return 'ban'
    case 'delete':
      return 'delete'
    case 'clear':
      return 'clear'
    case 'automod_hold':
    case 'automod_resolved':
      return 'automod'
    default:
      // Twitch adds actions over time (and All-Chat passes them through
      // verbatim), so anything unrecognised must still render as a row.
      return 'action'
  }
}

/**
 * Build a moderation-log entry from a `mod_action` event's metadata (a Twitch
 * `channel.moderate` action, or one of the two AutoMod actions All-Chat
 * synthesizes). Null only when there is no action name to render at all.
 *
 * Everything else is best-effort: fields Twitch omits for a given action (no
 * target on `clear`, no moderator on an AutoMod hold) simply stay undefined and
 * the row degrades to what is known.
 */
export function toModActionEntry(
  metadata: Record<string, unknown>,
  source: 'replay' | 'live',
  at: number
): ModEntryData | null {
  const action = metadataString(metadata, 'action')
  if (!action) return null
  const resolution = metadataString(metadata, 'resolution')
  return {
    kind: modActionKind(action),
    action,
    username: metadataString(metadata, 'target_login'),
    targetUserId: metadataString(metadata, 'target_user_id'),
    targetMsgId: metadataString(metadata, 'message_id'),
    banDuration: metadataNumber(metadata, 'ban_duration'),
    moderator: metadataString(metadata, 'moderator_login'),
    reason: metadataString(metadata, 'reason'),
    heldMessageId: metadataString(metadata, 'held_message_id'),
    heldText: metadataString(metadata, 'held_text'),
    automodCategory: metadataString(metadata, 'automod_category'),
    automodLevel: metadataNumber(metadata, 'automod_level'),
    resolution:
      resolution === 'approved' || resolution === 'denied' || resolution === 'expired'
        ? resolution
        : undefined,
    resolvedBy: metadataString(metadata, 'resolved_by'),
    source,
    at,
  }
}

/**
 * Append a moderation entry, folding an AutoMod resolution into the hold it
 * closes instead of adding a second row.
 *
 * A hold and its resolution are one event to a moderator: two rows would
 * double-count them and make "how many are still waiting" unreadable. The
 * hold keeps its original `at` so the row does not jump position when the
 * resolution lands, possibly minutes later.
 *
 * Generic over the entry type so the view can merge its own rendered entries
 * (`ModEntry`, which adds the render id) without losing that id.
 */
export function mergeAutoModResolution<T extends ModEntryData>(log: T[], entry: T): T[] {
  if (entry.action !== 'automod_resolved' || !entry.heldMessageId) return [...log, entry]
  const index = log.findIndex((held) => held.heldMessageId === entry.heldMessageId)
  if (index === -1) return [...log, entry]
  const next = [...log]
  next[index] = { ...log[index], resolution: entry.resolution, resolvedBy: entry.resolvedBy }
  return next
}

/**
 * Append or fold a pair of frames that describe one moderation action, so a
 * single delete, timeout, ban or clear renders as one row regardless of frame
 * arrival order.
 *
 * On Twitch one action reaches this view as two frames: a `message_deletion`
 * (what was removed, derived via {@link toModEntry}) and a `channel.moderate`
 * mod_action (who did it, derived via {@link toModActionEntry}). They are one
 * action to a moderator and must render as one row — the earlier row,
 * enriched with the other frame's moderator, username, target ids, duration —
 * never two, and never the bare unattributed row. The folded row keeps its
 * original `at` so it does not jump position when the second frame lands.
 * Per kind, the join key is:
 *
 * - delete: `targetMsgId`, Twitch's native message id, carried by both frames.
 * - timeout/ban: `targetUserId`, the moderated user's Twitch id. The deletion
 *   frame (channel.chat.clear_user_messages) carries no duration, so it reads
 *   as kind `ban` even when the action was a timeout; the fold matches across
 *   `timeout`/`ban` and upgrades the kind, recovering the duration only the
 *   mod_action frame knows.
 * - clear: both frames carry no target, so kind alone identifies the pair.
 *
 * Which frame is incoming decides the match rule, and it is deliberately
 * asymmetric. A mod_action frame folds only into a deletion-derived row (this
 * view's optimistic row, or the WS deletion frame) — never into another
 * mod_action row, so a Twitch double-send of the same webhook stays visible as
 * its own row. A deletion-derived frame folds into any row holding the same
 * join key, including the mod_action row that already carries the moderator.
 * For delete the match is the first (oldest) row with the same id — one
 * native message id is one removal, so an older unmatched row always belongs
 * to this pair. For timeout/ban and clear the pair is recognized by state, not
 * order: a mod_action frame matches only a row that has not yet consumed its
 * mod_action half (no `paired` flag), and the fold sets the flag — so a mod
 * timing out a user and then banning them inside the fold window still
 * produces two rows, one per action. A deletion frame carries no new
 * attribution, so it folds into a completed pair too; the later mod_action
 * echo of the second action then appends, which keeps two actions at two
 * rows regardless of arrival order. Rows older than the fold window are never
 * match candidates: a pair lands seconds apart, and anything later than that
 * is a new action.
 *
 * This is the single append-or-fold decision for an incoming frame — the
 * caller must pass the log WITHOUT the entry already appended.
 */

/** How long after an earlier row the second frame of an action pair may arrive. */
const PAIR_FOLD_WINDOW_MS = 120_000

export function mergeDeletionPair<T extends ModEntryData>(log: T[], entry: T): T[] {
  if (entry.kind !== 'delete' && entry.kind !== 'timeout' && entry.kind !== 'ban' && entry.kind !== 'clear') {
    return [...log, entry]
  }
  // The pair's join key: the native message id for a delete, the moderated
  // user's id for a timeout/ban. A clear has no target on either frame, so
  // null means "kind alone identifies the pair".
  const joinKey =
    entry.kind === 'delete'
      ? entry.targetMsgId
      : entry.kind === 'timeout' || entry.kind === 'ban'
        ? entry.targetUserId
        : null
  if (joinKey === undefined) return [...log, entry]
  const modActionFrame = entry.action !== undefined
  // A timeout and a ban of the same user are two frames of one action pair
  // (clear_user_messages cannot distinguish them), so kind matching is by
  // group, not by exact kind — except delete and clear, which pair only with
  // themselves.
  const kindGroup = (kind: ModKind): 'delete' | 'user' | 'clear' | 'other' =>
    kind === 'delete'
      ? 'delete'
      : kind === 'timeout' || kind === 'ban'
        ? 'user'
        : kind === 'clear'
          ? 'clear'
          : 'other'

  const matches = (other: ModEntryData): boolean => {
    if (kindGroup(other.kind) !== kindGroup(entry.kind)) return false
    // A mod_action frame folds only into a row that has not yet consumed its
    // mod_action half — never into another mod_action row (a Twitch
    // double-send of the same webhook stays visible as its own row), and
    // never into a completed pair (a second action against the same user is a
    // new action: a timeout followed by a ban must render as two rows).
    if (modActionFrame && (other.action !== undefined || other.paired)) return false
    // A deletion frame carries no new attribution, so it also folds into a
    // completed pair: within the window a repeat clear_user_messages/clear is
    // the same action's echo, not a second action.
    if (joinKey === null) return true
    const otherKey = other.kind === 'delete' ? other.targetMsgId : other.targetUserId
    return otherKey === joinKey
  }
  // Delete: first (oldest) match — one native message id is one removal.
  // Timeout/ban/clear: newest match inside the fold window — the pair lands
  // seconds apart, but a repeat action minutes later must append as its own
  // row.
  let index = -1
  if (entry.kind === 'delete') {
    index = log.findIndex(matches)
  } else {
    for (let i = log.length - 1; i >= 0; i--) {
      if (!matches(log[i])) continue
      if (entry.at - log[i].at > PAIR_FOLD_WINDOW_MS) break
      index = i
      break
    }
  }
  if (index === -1) return [...log, entry]
  // The mod_action frame knows the real action name and duration; the
  // deletion frame knows the internal uuid. Keep the more specific of each.
  const mergeKind = (oldKind: ModKind): ModKind =>
    oldKind === 'ban' && entry.kind === 'timeout' ? 'timeout' : oldKind
  return log.map((other, i) =>
    i === index
      ? {
          ...other,
          kind: mergeKind(other.kind),
          moderator: other.moderator || entry.moderator,
          username: other.username || entry.username,
          targetUserId: other.targetUserId || entry.targetUserId,
          targetUuid: other.targetUuid || entry.targetUuid,
          banDuration: other.banDuration || entry.banDuration,
          // The fold consumed the mod_action half of the pair; mark it so a
          // later action against the same target appends (see matches).
          paired: entry.kind === 'delete' ? other.paired : true,
        }
      : other
  )
}

/**
 * A stable signature for a deletion, used to dedupe an optimistic moderation
 * entry against the server-pushed deletion that confirms the same action.
 */
export function deletionSignature(meta: DeletionMetadata): string {
  return [
    meta.deletion_type,
    meta.target_uuid ?? '',
    meta.target_user_id ?? '',
    meta.ban_duration ?? 0,
  ].join(':')
}

/** Does this deletion target the given item? */
export function isDeletionTarget(item: ChatMessage, meta: DeletionMetadata): boolean {
  switch (meta.deletion_type) {
    case 'single':
      return !!meta.target_uuid && item.id === meta.target_uuid
    case 'batch':
      return !!meta.target_user_id && item.user?.id === meta.target_user_id
    case 'clear':
      return true
    default:
      return false
  }
}

/** Mark every item a deletion targets as moderated (kept visible), returning a new array. */
export function applyModerationMark(items: ViewItem[], meta: DeletionMetadata): ViewItem[] {
  const kind = deletionKind(meta)
  let changed = false
  const next = items.map((item) => {
    if (isDeletionTarget(item, meta)) {
      changed = true
      return { ...item, _moderated: { kind, banDuration: meta.ban_duration } }
    }
    return item
  })
  return changed ? next : items
}

/**
 * Append a chat/event item, merging TikTok like-aggregate updates in place by
 * `aggregation_id`. New items respect the scrollback `cap`; an in-place merge
 * does not grow the array so it is not re-capped.
 */
export function mergeByAgg(prev: ViewItem[], updated: ViewItem, cap: number): ViewItem[] {
  const aggId = updated.event?.aggregation_id
  if (!aggId) return [...prev, updated].slice(-cap)
  const idx = prev.findIndex((m) => m.event?.aggregation_id === aggId)
  if (idx === -1) return [...prev, updated].slice(-cap)
  const next = [...prev]
  next[idx] = updated
  return next
}

/**
 * Whether a panel pinned to the bottom should auto-scroll on new content: true
 * when the viewport is within `threshold` px of the bottom (i.e. the user has
 * not scrolled up to read history).
 */
export function shouldAutoScroll(
  metrics: { scrollHeight: number; scrollTop: number; clientHeight: number },
  threshold = 40
): boolean {
  return metrics.scrollHeight - metrics.scrollTop - metrics.clientHeight <= threshold
}

/**
 * Whether the viewport still sits on the LIVE EDGE — the edge the newest
 * message arrives at — and so should keep following new content.
 *
 * Which edge that is depends on the render order: the bottom when the newest
 * message is last (`shouldAutoScroll`, the Twitch-like default), the top when
 * the monitor's `newestFirst` pref inverts it. Getting this wrong is not merely
 * cosmetic: it makes the panel pause exactly when the reader is watching live
 * chat and follow exactly when they are reading scrollback.
 */
export function isPinnedToLiveEdge(
  metrics: { scrollHeight: number; scrollTop: number; clientHeight: number },
  newestFirst: boolean,
  threshold = 40
): boolean {
  return newestFirst ? metrics.scrollTop <= threshold : shouldAutoScroll(metrics, threshold)
}

/**
 * React keys for a chat list, one per item, that do not depend on render order.
 *
 * Keying by position in the RENDERED list is fine while the newest message is
 * appended at the end, but under `newestFirst` every arrival is a prepend, so
 * every position shifts and React remounts the whole (500-row) buffer on every
 * single message. Keys are therefore derived from the CHRONOLOGICAL array and
 * read back per rendered row.
 *
 * `${id}#${occurrence}` rather than the id alone because ids are not guaranteed
 * unique in the buffer (a replayed message can arrive again live). Counting
 * occurrences keeps a row's key stable as items are appended or prepended; only
 * the trimmed head of the buffer loses its keys, which is what we want.
 */
export function chatRowKeys(items: ViewItem[]): string[] {
  const seen = new Map<string, number>()
  return items.map((item) => {
    const occurrence = seen.get(item.id) ?? 0
    seen.set(item.id, occurrence + 1)
    return `${item.id}#${occurrence}`
  })
}

/**
 * One platform identity of a chatter, used to narrow the chat panel to a 1:1
 * conversation. Keyed per platform: the same person on Twitch and Kick is two
 * identities, which is what moderation and replies operate on too.
 */
export interface UserFilter {
  /** Stable identity key: `platform:user.id`, falling back to the lowercased username. */
  key: string
  /** Human-readable name for the filter bar. */
  label: string
}

/** Build the filter targeting an item's author, or null when it carries no user identity. */
export function userFilterFor(item: ChatMessage): UserFilter | null {
  const user = item.user
  if (!user) return null
  const id = user.id || user.username?.toLowerCase()
  if (!id) return null
  return { key: `${item.platform}:${id}`, label: user.display_name || user.username || id }
}

/** Whether an item was written by the identity a filter targets. */
export function matchesUserFilter(item: ChatMessage, filter: UserFilter): boolean {
  return userFilterFor(item)?.key === filter.key
}

/**
 * How many live items arrived after a paused snapshot was taken. Compared by
 * message id (not length) because the live buffer is capped and trims from the
 * front while paused.
 */
export function countNewItems(snapshot: ViewItem[], live: ViewItem[]): number {
  const seen = new Set(snapshot.map((m) => m.id))
  let count = 0
  for (const m of live) if (!seen.has(m.id)) count += 1
  return count
}
