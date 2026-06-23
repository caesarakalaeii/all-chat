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
 * Global state for viewer (chat participant) authentication — viewer info + JWT,
 * stored separately from streamer auth. (The streamer-facing viewer chat UI was
 * retired; the viewer session still backs OAuth linking and nav login state.)
 *
 * Usage in components:
 *   const { viewerInfo, viewerToken, viewerLogout } = useViewerAuthStore();
 */

import { create } from 'zustand'
import type { ViewerInfo } from '../types/viewer'
import { viewerApi } from '../api/viewer'
import { inMemoryTokens } from '../auth/in-memory-store'

/*
 * SECURITY (audit H3): the viewer access token is mirrored to the in-memory
 * store so API clients can read it without localStorage. It is still written
 * to localStorage for legacy readers (settings/viewer page reads it directly
 * in several places) — full removal is deferred to the httpOnly-cookie
 * migration.
 */

interface ViewerAuthStore {
  viewerInfo: ViewerInfo | null
  viewerToken: string | null
  loading: boolean

  // Actions
  setViewerToken: (token: string) => void
  setViewerInfo: (info: ViewerInfo) => void
  viewerLogout: () => void
  init: () => Promise<void>
}

export const useViewerAuthStore = create<ViewerAuthStore>((set) => ({
  viewerInfo: null,
  viewerToken: null,
  loading: true,

  setViewerToken: (token: string) => {
    inMemoryTokens.setViewerAccessToken(token)
    if (typeof window !== 'undefined') {
      // TODO(H3): remove localStorage write once legacy readers are migrated.
      localStorage.setItem('viewer_jwt_token', token)
    }
    set({ viewerToken: token })
  },

  setViewerInfo: (info: ViewerInfo) => {
    set({ viewerInfo: info, loading: false })
  },

  viewerLogout: () => {
    inMemoryTokens.setViewerAccessToken(null)
    if (typeof window !== 'undefined') {
      localStorage.removeItem('viewer_jwt_token')
    }
    set({ viewerInfo: null, viewerToken: null, loading: false })
  },

  init: async () => {
    if (typeof window === 'undefined') {
      set({ loading: false })
      return
    }

    const token = localStorage.getItem('viewer_jwt_token')

    if (!token) {
      set({ loading: false })
      return
    }

    inMemoryTokens.setViewerAccessToken(token)
    set({ viewerToken: token })

    try {
      const viewerInfo = await viewerApi.getMe()
      set({ viewerInfo, loading: false })
    } catch {
      // Token invalid, clear it
      inMemoryTokens.setViewerAccessToken(null)
      localStorage.removeItem('viewer_jwt_token')
      set({ viewerInfo: null, viewerToken: null, loading: false })
    }
  },
}))
