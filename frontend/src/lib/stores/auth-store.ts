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
