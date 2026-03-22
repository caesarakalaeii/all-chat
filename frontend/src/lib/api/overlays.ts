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
}
