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
 * Share Request Types
 *
 * Type definitions for chat overlay sharing functionality.
 */

export interface ShareRequest {
  id: string
  sender_user_id: string
  sender_overlay_id: string
  recipient_user_id: string
  status: 'pending' | 'accepted' | 'rejected' | 'expired' | 'revoked'
  created_at: string
  responded_at?: string
  expires_at: string
  has_seen_acceptance?: boolean // Phase 15-03: Tracks if sender has seen acceptance notification
  sender_display_name?: string // Phase 15-03: Recipient's display name for unseen acceptances
  // Populated by JOIN in future (Phase 15)
  sender?: {
    id: string
    username: string
    display_name: string
    profile_image_url: string
  }
  overlay_sources?: Array<{
    platform: string
    channel_name: string
  }>
}

export interface UserSearchResult {
  id: string
  username: string
  display_name: string
  profile_image_url: string
}

/**
 * Accepted share detail — returned by GET /api/v1/shares/accepted
 * Represents a shared overlay the current user (recipient) can add as a source.
 */
export interface AcceptedShare {
  share_id: string
  sender_overlay_id: string
  sender_overlay_name: string
  sender_display_name: string
  share_status: string
}
