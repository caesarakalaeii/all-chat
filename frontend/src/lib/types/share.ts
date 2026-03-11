/**
 * Share Request Types
 *
 * Type definitions for chat overlay sharing functionality.
 */

export interface ShareRequest {
  id: string;
  sender_user_id: string;
  sender_overlay_id: string;
  recipient_user_id: string;
  status: 'pending' | 'accepted' | 'rejected' | 'expired' | 'revoked';
  created_at: string;
  responded_at?: string;
  expires_at: string;
  has_seen_acceptance?: boolean; // Phase 15-03: Tracks if sender has seen acceptance notification
  sender_display_name?: string;  // Phase 15-03: Recipient's display name for unseen acceptances
  // Populated by JOIN in future (Phase 15)
  sender?: {
    id: string;
    username: string;
    display_name: string;
    profile_image_url: string;
  };
  overlay_sources?: Array<{
    platform: string;
    channel_name: string;
  }>;
}

export interface UserSearchResult {
  id: string;
  username: string;
  display_name: string;
  profile_image_url: string;
}

/**
 * Accepted share detail — returned by GET /api/v1/shares/accepted
 * Represents a shared overlay the current user (recipient) can add as a source.
 */
export interface AcceptedShare {
  share_id: string;
  sender_overlay_id: string;
  sender_overlay_name: string;
  sender_display_name: string;
  share_status: string;
}
