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
 * Viewer Authentication Store (Zustand)
 *
 * Global state management for viewer (chat participant) authentication.
 * Stores viewer info and JWT token separately from streamer auth.
 *
 * Usage in components:
 *   const { viewerInfo, viewerToken, viewerLogin, viewerLogout } = useViewerAuthStore();
 */

import { create } from 'zustand'
import type { ViewerInfo } from '../types/viewer'
import { viewerApi } from '../api/viewer'

interface ViewerAuthStore {
  viewerInfo: ViewerInfo | null
  viewerToken: string | null
  loading: boolean
  streamer: string | null

  // Actions
  setViewerToken: (token: string) => void
  setViewerInfo: (info: ViewerInfo) => void
  setStreamer: (streamer: string) => void
  viewerLogout: () => void
  init: () => Promise<void>
}

export const useViewerAuthStore = create<ViewerAuthStore>((set, get) => ({
  viewerInfo: null,
  viewerToken: null,
  loading: true,
  streamer: null,

  setViewerToken: (token: string) => {
    if (typeof window !== 'undefined') {
      localStorage.setItem('viewer_jwt_token', token)
    }
    set({ viewerToken: token })
  },

  setViewerInfo: (info: ViewerInfo) => {
    set({ viewerInfo: info, loading: false })
  },

  setStreamer: (streamer: string) => {
    set({ streamer })
  },

  viewerLogout: () => {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('viewer_jwt_token')
      localStorage.removeItem('viewer_streamer')
    }
    set({ viewerInfo: null, viewerToken: null, streamer: null, loading: false })
  },

  init: async () => {
    if (typeof window === 'undefined') {
      set({ loading: false })
      return
    }

    const token = localStorage.getItem('viewer_jwt_token')
    const streamer = localStorage.getItem('viewer_streamer')

    if (!token) {
      set({ loading: false, streamer: streamer || null })
      return
    }

    set({ viewerToken: token, streamer: streamer || null })

    try {
      const viewerInfo = await viewerApi.getMe()
      set({ viewerInfo, loading: false })
    } catch (error) {
      // Token invalid, clear it
      localStorage.removeItem('viewer_jwt_token')
      set({ viewerInfo: null, viewerToken: null, loading: false })
    }
  },
}))
