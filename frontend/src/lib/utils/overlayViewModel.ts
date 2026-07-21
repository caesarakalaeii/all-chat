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

export type ModKind = 'delete' | 'timeout' | 'ban' | 'clear'

/** A moderation-log entry derived from a deletion. `id` is assigned by the consumer. */
export interface ModEntryData {
  kind: ModKind
  username?: string
  targetUserId?: string
  targetUuid?: string
  banDuration?: number
  source: 'replay' | 'live'
  at: number
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
    banDuration: meta.ban_duration,
    source,
    at,
  }
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
