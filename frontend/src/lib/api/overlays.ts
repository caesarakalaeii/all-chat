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
 * Overlays API
 *
 * Functions for managing overlays and chat sources.
 * Communicates with the Overlay Manager service via API Gateway.
 */

import { apiClient } from './client'
import type {
  Overlay,
  OverlayConfig,
  ChatSource,
  CreateOverlayRequest,
  UpdateOverlayRequest,
  AddSourceRequest,
  MockMessagePayload,
  CreditRollConfig,
  CreditRollResponse,
} from '../types/overlay'

// --- Phase 13: TTS types (see Plan 02 handlers/tts.go) ---

export interface TTSConfigMetadata {
  has_elevenlabs_config: boolean
  voice_id?: string
  obs_url?: string
}

export interface ElevenLabsVoice {
  voice_id: string
  name: string
  category?: string
  preview_url?: string
  labels?: Record<string, string>
}

export interface TestKeyResult {
  ok: boolean
  charactersRemaining?: number
  charactersLimit?: number
  errorCode?: number
  audioBlob?: Blob
}

export const overlaysApi = {
  /**
   * Get all overlays for the authenticated user
   */
  async list(): Promise<Overlay[]> {
    const response = await apiClient.get<Overlay[]>('/api/v1/overlays')
    return response
  },

  /**
   * Get a specific overlay by ID
   */
  async get(id: string): Promise<Overlay> {
    return apiClient.get<Overlay>(`/api/v1/overlays/${id}`)
  },

  /**
   * Create a new overlay
   */
  async create(data: CreateOverlayRequest): Promise<Overlay> {
    return apiClient.post<Overlay>('/api/v1/overlays', data)
  },

  /**
   * Update an existing overlay
   */
  async update(id: string, data: UpdateOverlayRequest): Promise<Overlay> {
    return apiClient.put<Overlay>(`/api/v1/overlays/${id}`, data)
  },

  /**
   * Delete an overlay
   */
  async delete(id: string): Promise<void> {
    await apiClient.delete(`/api/v1/overlays/${id}`)
  },

  /**
   * Clone an overlay (creates a full copy with new ID)
   */
  async clone(id: string): Promise<Overlay> {
    return apiClient.post<Overlay>(`/api/v1/overlays/${id}/clone`, {})
  },

  /**
   * Get overlay configuration (display settings, filters, emotes)
   */
  async getConfig(id: string): Promise<OverlayConfig> {
    return apiClient.get<OverlayConfig>(`/api/v1/overlays/${id}/config`)
  },

  /**
   * Update overlay configuration
   */
  async updateConfig(id: string, config: Partial<OverlayConfig>): Promise<OverlayConfig> {
    return apiClient.put<OverlayConfig>(`/api/v1/overlays/${id}/config`, config)
  },

  /**
   * Get all chat sources for an overlay
   */
  async getSources(id: string): Promise<ChatSource[]> {
    const response = await apiClient.get<ChatSource[]>(`/api/v1/overlays/${id}/sources`)
    return response
  },

  /**
   * Add a chat source to an overlay
   */
  async addSource(id: string, data: AddSourceRequest): Promise<ChatSource> {
    return apiClient.post<ChatSource>(`/api/v1/overlays/${id}/sources`, data)
  },

  /**
   * Update a chat source's config (generic — works for any platform config)
   */
  async updateSourceConfig(overlayId: string, sourceId: string, config: Record<string, unknown>): Promise<void> {
    await apiClient.patch(`/api/v1/overlays/${overlayId}/sources/${sourceId}`, { config })
  },

  /**
   * Remove a chat source from an overlay
   */
  async removeSource(overlayId: string, sourceId: string): Promise<void> {
    await apiClient.delete(`/api/v1/overlays/${overlayId}/sources/${sourceId}`)
  },

  /**
   * Send a mock message through the backend pipeline
   */
  async sendMockMessage(id: string, payload: MockMessagePayload): Promise<void> {
    await apiClient.post(`/api/v1/overlays/${id}/mock-messages`, payload)
  },

  /**
   * Get credit roll configuration for an overlay
   */
  async getCreditRollConfig(id: string): Promise<CreditRollConfig> {
    return apiClient.get<CreditRollConfig>(`/api/v1/overlays/${id}/creditroll`)
  },

  /**
   * Update credit roll configuration
   */
  async updateCreditRollConfig(
    id: string,
    config: Partial<CreditRollConfig>
  ): Promise<CreditRollConfig> {
    return apiClient.post<CreditRollConfig>(`/api/v1/overlays/${id}/creditroll`, config)
  },

  /**
   * Resolve a YouTube handle, URL, or channel ID to a canonical channel ID
   */
  async resolveYouTubeChannel(input: string): Promise<{
    channel_id: string
    title?: string
    custom_url?: string
    thumbnail?: string
    input_type: string
  }> {
    return apiClient.post('/api/v1/youtube/resolve', { input })
  },

  /**
   * Get credit roll data (public endpoint)
   */
  async getCreditRoll(id: string): Promise<CreditRollResponse> {
    const response = await fetch(`/api/v1/overlays/${id}/credit-roll`)
    if (!response.ok) {
      throw new Error('Failed to fetch credit roll')
    }
    return response.json()
  },

  // --- Phase 13: TTS endpoints (see services/overlay-manager/handlers/tts.go from Plan 02) ---

  /**
   * Save an ElevenLabs API key + voice for the overlay.
   * Backend: POST /api/v1/overlays/:id/tts-config (RequirePremium("tts")).
   * The plaintext key is sent once; it is AES-GCM encrypted server-side and never returned.
   */
  async saveTTSKey(overlayId: string, apiKey: string, voiceId: string): Promise<void> {
    await apiClient.post<{ status: string }>(
      `/api/v1/overlays/${overlayId}/tts-config`,
      { api_key: apiKey, voice_id: voiceId },
    )
  },

  /**
   * Remove the saved ElevenLabs key/voice + signing secret for the overlay.
   * Backend: DELETE /api/v1/overlays/:id/tts-config.
   */
  async removeTTSKey(overlayId: string): Promise<void> {
    await apiClient.delete(`/api/v1/overlays/${overlayId}/tts-config`)
  },

  /**
   * Rotate the per-overlay signing secret, invalidating all previously issued
   * tts_token JWTs, and return the new OBS URL (camelCase).
   * Backend: POST /api/v1/overlays/:id/tts-config/rotate-token.
   */
  async rotateTTSToken(overlayId: string): Promise<{ obsUrl: string }> {
    const resp = await apiClient.post<{ obs_url: string }>(
      `/api/v1/overlays/${overlayId}/tts-config/rotate-token`,
      {},
    )
    if (typeof resp?.obs_url !== 'string' || resp.obs_url === '') {
      throw new Error('rotate-token returned an invalid response shape')
    }
    return { obsUrl: resp.obs_url }
  },

  /**
   * Fetch the ElevenLabs voice list for the overlay's saved key (lazy on dropdown open).
   * Backend: GET /api/v1/overlays/:id/tts-voices (proxy to ElevenLabs /v1/voices).
   * Plan 02's proxy streams the ElevenLabs JSON body through unchanged, which has shape
   * `{voices: [...]}`. Handle both shapes defensively.
   */
  async getTTSVoices(overlayId: string): Promise<ElevenLabsVoice[]> {
    const resp = await apiClient.get<{ voices?: ElevenLabsVoice[] } | ElevenLabsVoice[]>(
      `/api/v1/overlays/${overlayId}/tts-voices`,
    )
    if (Array.isArray(resp)) return resp
    if (resp && Array.isArray(resp.voices)) return resp.voices
    return []
  },

  /**
   * One-shot proxy that lists voices for an UNSAVED ElevenLabs key. Breaks the
   * chicken-and-egg between save (requires voice_id) and the saved-key voice
   * list (requires a saved key) — see Plan 02 / TTS bugfix.
   * Backend: POST /api/v1/overlays/:id/tts-voices/preview with `{api_key}`.
   * The key is NOT persisted server-side.
   */
  async previewTTSVoices(overlayId: string, apiKey: string): Promise<ElevenLabsVoice[]> {
    const resp = await apiClient.post<{ voices?: ElevenLabsVoice[] } | ElevenLabsVoice[]>(
      `/api/v1/overlays/${overlayId}/tts-voices/preview`,
      { api_key: apiKey },
    )
    if (Array.isArray(resp)) return resp
    if (resp && Array.isArray(resp.voices)) return resp.voices
    return []
  },

  /**
   * Validate the saved ElevenLabs key and retrieve the remaining character quota,
   * plus a ~2-second audio sample for user feedback.
   * Backend: POST /api/v1/overlays/:id/tts-config/test.
   *
   * Response on success is audio/mpeg + `x-characters-remaining` / `x-characters-limit`
   * headers — the standard `apiClient.post` assumes JSON, so use `fetch` directly and
   * replicate the Bearer-token auth pattern from `client.ts`.
   */
  async testTTSKey(overlayId: string): Promise<TestKeyResult> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem('jwt_token')
      if (token) headers['Authorization'] = `Bearer ${token}`
    }
    try {
      const r = await fetch(`/api/v1/overlays/${overlayId}/tts-config/test`, {
        method: 'POST',
        headers,
      })
      if (!r.ok) {
        return { ok: false, errorCode: r.status }
      }
      const remainingHeader = r.headers.get('x-characters-remaining')
      const limitHeader = r.headers.get('x-characters-limit')
      const charactersRemaining =
        remainingHeader !== null && remainingHeader !== '' ? Number(remainingHeader) : NaN
      const charactersLimit =
        limitHeader !== null && limitHeader !== '' ? Number(limitHeader) : NaN
      const audioBlob = await r.blob()
      return {
        ok: true,
        charactersRemaining: Number.isFinite(charactersRemaining) ? charactersRemaining : undefined,
        charactersLimit: Number.isFinite(charactersLimit) ? charactersLimit : undefined,
        audioBlob,
      }
    } catch {
      return { ok: false, errorCode: 0 /* network */ }
    }
  },

  /**
   * Read the metadata for the overlay's TTS config (no secrets returned).
   * Backend: GET /api/v1/overlays/:id/tts-config.
   */
  async getTTSConfig(overlayId: string): Promise<TTSConfigMetadata> {
    return apiClient.get<TTSConfigMetadata>(
      `/api/v1/overlays/${overlayId}/tts-config`,
    )
  },
}
