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
    if (typeof window !== 'undefined') {
      localStorage.setItem('jwt_token', token)
    }
    set({ token })
  },

  setUser: (user: User) => {
    set({ user, loading: false })
  },

  logout: () => {
    if (typeof window !== 'undefined') {
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

    // Restore impersonation state from localStorage on init (e.g. after page reload)
    const isImpersonating = localStorage.getItem('impersonating') === 'true'
    const impersonatedUsername = localStorage.getItem('impersonated_user') || null

    set({ token, isImpersonating, impersonatedUsername })

    try {
      const user = await authApi.getMe()
      set({ user, loading: false })
    } catch (error) {
      // Token invalid, clear it
      localStorage.removeItem('jwt_token')
      localStorage.removeItem('admin_token')
      localStorage.removeItem('impersonating')
      localStorage.removeItem('impersonated_user')
      set({ user: null, token: null, loading: false, isImpersonating: false, impersonatedUsername: null })
    }
  },

  startImpersonation: (token: string, username: string) => {
    if (typeof window !== 'undefined') {
      // Save the current admin token before overwriting
      const currentToken = get().token
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
    if (typeof window !== 'undefined') {
      const adminToken = localStorage.getItem('admin_token')
      if (adminToken) {
        localStorage.setItem('jwt_token', adminToken)
        localStorage.removeItem('admin_token')
      }
      localStorage.removeItem('impersonating')
      localStorage.removeItem('impersonated_user')

      const restoredToken = localStorage.getItem('jwt_token')
      set({ token: restoredToken, isImpersonating: false, impersonatedUsername: null })
    } else {
      set({ isImpersonating: false, impersonatedUsername: null })
    }
  },
}))
