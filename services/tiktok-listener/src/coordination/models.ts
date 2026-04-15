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
