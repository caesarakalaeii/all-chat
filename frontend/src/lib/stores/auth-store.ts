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
 * Authentication Store (Zustand)
 *
 * Global state management for user authentication.
 * Stores user info and JWT token in memory and localStorage.
 *
 * Usage in components:
 *   const { user, token, login, logout } = useAuthStore();
 */

import { create } from 'zustand'
import type { User } from '../types/auth'
import { authApi } from '../api/auth'
import { inMemoryTokens } from '../auth/in-memory-store'

/*
 * SECURITY (audit H3): refresh tokens are stored ONLY in the in-memory store
 * (see lib/auth/in-memory-store.ts), never in localStorage. Access JWTs are
 * mirrored to the in-memory store so API clients can prefer it, but are still
 * written to localStorage for legacy readers (admin/settings pages) — full
 * removal is deferred to the httpOnly-cookie migration.
 */

interface AuthStore {
  user: User | null
  token: string | null
  loading: boolean

  // Impersonation state
  isImpersonating: boolean
  impersonatedUsername: string | null

  // Actions
  setToken: (token: string) => void
  setUser: (user: User) => void
  logout: () => void
  init: () => Promise<void>

  // Impersonation actions
  startImpersonation: (token: string, username: string) => void
  stopImpersonation: () => void
}

export const useAuthStore = create<AuthStore>((set, get) => ({
  user: null,
  token: null,
  loading: true,
  isImpersonating: false,
  impersonatedUsername: null,

  setToken: (token: string) => {
    inMemoryTokens.setAccessToken(token)
    if (typeof window !== 'undefined') {
      // TODO(H3): remove localStorage write once all legacy readers are
      // migrated to the in-memory store / httpOnly cookies.
      localStorage.setItem('jwt_token', token)
    }
    set({ token })
  },

  setUser: (user: User) => {
    set({ user, loading: false })
  },

  logout: () => {
    inMemoryTokens.clearAll()
    if (typeof window !== 'undefined') {
      // TODO(H3): remove localStorage clears once legacy readers are migrated.
      localStorage.removeItem('jwt_token')
      localStorage.removeItem('admin_token')
      localStorage.removeItem('impersonating')
      localStorage.removeItem('impersonated_user')
    }
    set({ user: null, token: null, loading: false, isImpersonating: false, impersonatedUsername: null })
  },

  init: async () => {
    if (typeof window === 'undefined') {
      set({ loading: false })
      return
    }

    const token = localStorage.getItem('jwt_token')
    if (!token) {
      set({ loading: false })
      return
    }

    // Mirror the restored access token into the in-memory store so API
    // clients can read it without touching localStorage.
    inMemoryTokens.setAccessToken(token)

    // Restore impersonation state from localStorage on init (e.g. after page reload)
    const isImpersonating = localStorage.getItem('impersonating') === 'true'
    const impersonatedUsername = localStorage.getItem('impersonated_user') || null
    inMemoryTokens.setImpersonating(isImpersonating)
    inMemoryTokens.setImpersonatedUsername(impersonatedUsername)

    set({ token, isImpersonating, impersonatedUsername })

    try {
      const user = await authApi.getMe()
      set({ user, loading: false })
    } catch (error) {
      // Token invalid, clear it
      inMemoryTokens.clearAll()
      localStorage.removeItem('jwt_token')
      localStorage.removeItem('admin_token')
      localStorage.removeItem('impersonating')
      localStorage.removeItem('impersonated_user')
      set({ user: null, token: null, loading: false, isImpersonating: false, impersonatedUsername: null })
    }
  },

  startImpersonation: (token: string, username: string) => {
    // Save the current admin token before overwriting
    const currentToken = get().token
    if (currentToken) {
      inMemoryTokens.setAdminToken(currentToken)
    }
    inMemoryTokens.setAccessToken(token)
    inMemoryTokens.setImpersonating(true)
    inMemoryTokens.setImpersonatedUsername(username)
    if (typeof window !== 'undefined') {
      // TODO(H3): remove localStorage writes once legacy readers are migrated.
      if (currentToken) {
        localStorage.setItem('admin_token', currentToken)
      }
      localStorage.setItem('jwt_token', token)
      localStorage.setItem('impersonating', 'true')
      localStorage.setItem('impersonated_user', username)
    }
    set({ token, isImpersonating: true, impersonatedUsername: username })
  },

  stopImpersonation: () => {
    const adminToken = inMemoryTokens.getAdminToken() ?? (typeof window !== 'undefined' ? localStorage.getItem('admin_token') : null)
    if (adminToken) {
      inMemoryTokens.setAccessToken(adminToken)
      inMemoryTokens.setAdminToken(null)
    }
    inMemoryTokens.setImpersonating(false)
    inMemoryTokens.setImpersonatedUsername(null)
    if (typeof window !== 'undefined') {
      if (adminToken) {
        localStorage.setItem('jwt_token', adminToken)
        localStorage.removeItem('admin_token')
      }
      localStorage.removeItem('impersonating')
      localStorage.removeItem('impersonated_user')

      const restoredToken = localStorage.getItem('jwt_token')
      set({ token: restoredToken, isImpersonating: false, impersonatedUsername: null })
    } else {
      set({ token: adminToken, isImpersonating: false, impersonatedUsername: null })
    }
  },
}))
