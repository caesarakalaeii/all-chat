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
 * Source Manager Client
 *
 * HTTP client for leadership-based coordination with source-manager.
 * Matches Go shared/sourcemanager/client.go patterns.
 */

import axios, { AxiosInstance, AxiosError } from 'axios';
import { Logger } from '../types/logger.js';
import {
  LeadershipRequest,
  ClaimResponse,
  RenewResponse,
  RegisterPeerResponse,
} from './models.js';
import { generateServiceJWT } from '../auth/jwt.js';

const JWT_TTL_MS = 24 * 60 * 60 * 1000; // 24 hours
const JWT_REFRESH_THRESHOLD_MS = 5 * 60 * 1000; // Refresh when < 5 minutes remaining

/**
 * SourceManagerClient is an HTTP client for source-manager leadership API.
 * Replaces the old CoordinatorClient (assignment-based) with leadership endpoints.
 */
export class SourceManagerClient {
  private serviceSecret: string;
  private serviceJWT: string;
  private jwtExpiresAt: number;
  private serviceName: string;
  private httpClient: AxiosInstance;
  private logger: Logger;

  constructor(baseURL: string, serviceSecret: string, logger: Logger) {
    this.serviceSecret = serviceSecret;
    this.logger = logger;
    this.serviceName = 'tiktok-listener';

    this.serviceJWT = generateServiceJWT(this.serviceName, this.serviceSecret, JWT_TTL_MS);
    this.jwtExpiresAt = Date.now() + JWT_TTL_MS;

    this.logger.info('Generated service JWT for source-manager authentication', {
      service_name: this.serviceName,
    });

    this.httpClient = axios.create({
      baseURL,
      timeout: 10000,
    });

    // Auto-refresh JWT before expiry
    this.httpClient.interceptors.request.use((config) => {
      if (Date.now() >= this.jwtExpiresAt - JWT_REFRESH_THRESHOLD_MS) {
        this.serviceJWT = generateServiceJWT(this.serviceName, this.serviceSecret, JWT_TTL_MS);
        this.jwtExpiresAt = Date.now() + JWT_TTL_MS;
        this.logger.debug('Refreshed service JWT');
      }
      config.headers['Authorization'] = `Bearer ${this.serviceJWT}`;
      return config;
    });
  }

  /**
   * ClaimLeadership attempts to become leader for the given stream ID.
   * Returns true if leadership was acquired.
   */
  async claimLeadership(platform: string, streamID: string, callerID: string): Promise<boolean> {
    const body: LeadershipRequest = { platform, stream_id: streamID, caller_id: callerID };

    try {
      const resp = await this.httpClient.post<ClaimResponse>('/leadership/claim', body);
      return resp.data.acquired;
    } catch (error) {
      if (axios.isAxiosError(error) && error.response?.status === 401) {
        throw new Error('source-manager authorization failed');
      }
      throw error;
    }
  }

  /**
   * RenewLeadership refreshes an existing leadership claim.
   * Returns true if renewed. Throws 'leadership_lost' if 410 GONE.
   */
  async renewLeadership(platform: string, streamID: string, callerID: string): Promise<boolean> {
    const body: LeadershipRequest = { platform, stream_id: streamID, caller_id: callerID };

    try {
      const resp = await this.httpClient.post<RenewResponse>('/leadership/renew', body);
      return resp.data.renewed;
    } catch (error) {
      if (axios.isAxiosError(error)) {
        if (error.response?.status === 410) {
          throw new Error('leadership_lost');
        }
        if (error.response?.status === 401) {
          throw new Error('source-manager authorization failed');
        }
      }
      throw error;
    }
  }

  /**
   * ReleaseLeadership releases a leadership claim.
   */
  async releaseLeadership(platform: string, streamID: string, callerID: string): Promise<void> {
    const body: LeadershipRequest = { platform, stream_id: streamID, caller_id: callerID };

    try {
      await this.httpClient.post('/leadership/release', body);
    } catch (error) {
      if (axios.isAxiosError(error) && error.response?.status === 401) {
        throw new Error('source-manager authorization failed');
      }
      // Release failures are non-fatal — lease will expire naturally
      this.logger.warn('Failed to release leadership', {
        platform, stream_id: streamID, error: String(error),
      });
    }
  }

  /**
   * RegisterPeer registers this instance as an active peer for rebalancing.
   */
  async registerPeer(platform: string, callerID: string): Promise<number> {
    try {
      const resp = await this.httpClient.post<RegisterPeerResponse>(
        '/leadership/peers/register',
        { platform, caller_id: callerID },
      );
      return resp.data.peer_count;
    } catch (error) {
      this.logger.warn('Failed to register peer', { platform, error: String(error) });
      return 1; // Assume single peer on failure
    }
  }

  /**
   * GetDemand queries for the current demanded sources.
   * Used by the 60s safety-net poll to restore state after missed Pub/Sub events.
   */
  async getDemand(platform?: string): Promise<{ source_id: string; channel_id: string; platform: string; overlay_id: string }[]> {
    const url = platform
      ? `/demand?platform=${encodeURIComponent(platform)}`
      : '/demand';

    try {
      const resp = await this.httpClient.get<{ sources: { source_id: string; channel_id: string; platform: string; overlay_id: string }[] }>(url);
      return resp.data.sources;
    } catch (error) {
      if (axios.isAxiosError(error)) {
        this.logger.error('Failed to get demand', {
          platform, error: error.message, status_code: error.response?.status,
        });
        throw new Error(`Failed to get demand: ${error.message}`);
      }
      throw error;
    }
  }
}
