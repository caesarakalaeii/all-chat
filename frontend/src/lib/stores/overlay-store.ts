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
 * Overlay Store (Zustand)
 *
 * Global state management for user's overlays.
 * Handles CRUD operations and caching.
 *
 * Usage:
 *   const { overlays, fetchOverlays, createOverlay } = useOverlayStore();
 */

import { create } from 'zustand'
import type { Overlay, ChatSource } from '../types/overlay'
import { overlaysApi } from '../api/overlays'

interface OverlayStore {
  overlays: Overlay[]
  loading: boolean
  error: string | null

  // Actions
  fetchOverlays: () => Promise<void>
  createOverlay: (data: { name: string; description?: string }) => Promise<Overlay>
  updateOverlay: (id: string, data: Partial<Overlay>) => Promise<Overlay>
  deleteOverlay: (id: string) => Promise<void>
}

export const useOverlayStore = create<OverlayStore>((set, get) => ({
  overlays: [],
  loading: false,
  error: null,

  fetchOverlays: async () => {
    set({ loading: true, error: null })
    try {
      const overlays = await overlaysApi.list()
      set({ overlays: overlays || [], loading: false })
    } catch (error) {
      set({
        overlays: [], // Reset to empty array on error
        error: error instanceof Error ? error.message : 'Failed to fetch overlays',
        loading: false,
      })
    }
  },

  createOverlay: async (data) => {
    set({ loading: true, error: null })
    try {
      const overlay = await overlaysApi.create(data)
      set((state) => ({
        overlays: [...state.overlays, overlay],
        loading: false,
      }))
      return overlay
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to create overlay',
        loading: false,
      })
      throw error
    }
  },

  updateOverlay: async (id, data) => {
    set({ loading: true, error: null })
    try {
      const updated = await overlaysApi.update(id, data)
      set((state) => ({
        overlays: state.overlays.map((o) => (o.id === id ? updated : o)),
        loading: false,
      }))
      return updated
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to update overlay',
        loading: false,
      })
      throw error
    }
  },

  deleteOverlay: async (id) => {
    set({ loading: true, error: null })
    try {
      await overlaysApi.delete(id)
      set((state) => ({
        overlays: state.overlays.filter((o) => o.id !== id),
        loading: false,
      }))
    } catch (error) {
      set({
        error: error instanceof Error ? error.message : 'Failed to delete overlay',
        loading: false,
      })
      throw error
    }
  },
}))
