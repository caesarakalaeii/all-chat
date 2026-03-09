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
  status: 'pending' | 'accepted' | 'rejected' | 'expired';
  created_at: string;
  responded_at?: string;
  expires_at: string;
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
