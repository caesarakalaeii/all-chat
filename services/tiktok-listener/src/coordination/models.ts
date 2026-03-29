/**
 * Coordination Models
 *
 * TypeScript interfaces for leadership-based coordination with source-manager.
 * Matches Go shared/sourcemanager patterns.
 */

/**
 * LeadershipRequest is the payload for claim/renew/release endpoints.
 * Matches Go source-manager handler expectations.
 */
export interface LeadershipRequest {
  platform: string;
  stream_id: string;
  caller_id: string;
}

/**
 * ClaimResponse from POST /leadership/claim.
 */
export interface ClaimResponse {
  acquired: boolean;
  platform?: string;
  stream_id?: string;
  instance_id?: string;
}

/**
 * RenewResponse from POST /leadership/renew.
 */
export interface RenewResponse {
  renewed: boolean;
  platform?: string;
  stream_id?: string;
}

/**
 * RegisterPeerRequest for POST /leadership/peers/register.
 */
export interface RegisterPeerRequest {
  platform: string;
  caller_id: string;
}

/**
 * RegisterPeerResponse from POST /leadership/peers/register.
 */
export interface RegisterPeerResponse {
  peer_count: number;
  platform: string;
  caller_id: string;
}
