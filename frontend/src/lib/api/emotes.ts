/**
 * Emotes API Client
 *
 * Fetches emote data from the Emote Service via API Gateway.
 * Used for client-side emote enrichment in mock messages.
 */

import { apiClient } from './client';

export interface EmoteData {
  code: string;
  url: string;
  provider: '7tv' | 'bttv' | 'ffz';
}

export interface EmoteResponse {
  channel: string;
  emotes: EmoteData[];
}

class EmotesApi {
  /**
   * Fetch all emotes for a channel (7TV, BTTV, FFZ combined)
   */
  async getChannelEmotes(channel: string): Promise<EmoteData[]> {
    try {
      const response = await apiClient.get<EmoteResponse>(
        `/api/v1/emotes/channel/${encodeURIComponent(channel)}`
      );
      return response.emotes;
    } catch (error) {
      console.error('Failed to fetch channel emotes:', error);
      return [];
    }
  }

  /**
   * Fetch emotes from a specific provider
   */
  async getProviderEmotes(provider: string, channel: string): Promise<EmoteData[]> {
    try {
      const response = await apiClient.get<EmoteResponse>(
        `/api/v1/emotes/${provider}/${encodeURIComponent(channel)}`
      );
      return response.emotes;
    } catch (error) {
      console.error(`Failed to fetch ${provider} emotes:`, error);
      return [];
    }
  }
}

export const emotesApi = new EmotesApi();
