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

/** The alert WebSocket frame (ADR-0068, ADR-0070). */

/** Mirrors shared/alerts eventDefs, in its order (a unit test holds them equal). */
export const ALERT_EVENT_TYPES = [
  'subscription',
  'resubscription',
  'gift_subscription',
  'mystery_gift',
  'bits',
  'raid',
  'follow',
  'channel_points',
  'charity_donation',
  'super_chat',
  'super_sticker',
  'new_sponsor',
  'member_milestone',
  'membership_gift',
  'subscriber',
  'gift',
  'share',
  'tiktok_subscription',
] as const

export type AlertEventType = (typeof ALERT_EVENT_TYPES)[number]

/**
 * One saved (or default) alert profile, as GET/PUT
 * /overlays/:id/alerts/profiles exchange it (overlay-manager, ADR-0070).
 */
export interface EditableAlertProfile {
  event_type: string
  text_template: string
  sound_url: string
  image_url: string
  duration_ms: number
  /** 0-100. */
  volume: number
  enabled: boolean
  /** Waiting alerts play in ascending sort_after; equal values stay FIFO. */
  sort_after: number
  /**
   * Premium custom code (migration 109, ADR-0071). Any non-empty field makes
   * the alert box render it in a sandboxed iframe instead of the built-in
   * layout. Absent from frames of a gateway older than the columns.
   */
  custom_html?: string
  custom_css?: string
  custom_js?: string
}

/** The resolved profile the gateway attaches (absent for event types without one). */
export interface AlertProfile extends EditableAlertProfile {
  /** text_template with placeholders substituted server-side. */
  text: string
}

/** `data` of a `{type: "alert"}` WebSocket frame (api-gateway, ADR-0070). */
export interface AlertFrame {
  alert_id: string
  overlay_id: string
  platform: string
  event_type: string
  event_data: {
    type: string
    tier?: string
    value?: { amount: number; currency: string; display_text: string }
    /** Seconds. */
    duration?: number
    is_update?: boolean
    metadata?: Record<string, unknown>
  }
  user: { id: string; name: string; avatar_url: string }
  message?: string
  occurred_at?: string
  profile?: AlertProfile
}
