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
 * Authentication Store (Zustand) — H3 cookie-auth.
 *
 * Login state is derived from the httpOnly access cookie: `init` calls
 * `GET /auth/me`, which succeeds only when the cookie is valid. The store no
 * longer holds a JS-readable access token (cookies are httpOnly), so there is
 * no `token` state and no localStorage token juggling.
 *
 * Impersonation is server-driven: `startImpersonation(targetUserId)` POSTs the
 * admin endpoint which sets an impersonated-user access cookie; the backend
 * returns the impersonated user. `stopImpersonation` restores the admin cookie
 * the same way. No token is ever swapped in JS.
 *
 * Usage in components:
 *   const { user, loading, logout } = useAuthStore();
 */

import { create } from 'zustand'
import type { User } from '../types/auth'
import { authApi } from '../api/auth'

interface AuthStore {
  user: User | null
  loading: boolean

  // Impersonation state
  isImpersonating: boolean
  impersonatedUsername: string | null

  // Actions
  setUser: (user: User) => void
  logout: () => Promise<void>
  init: () => Promise<void>

  // Impersonation actions
  startImpersonation: (targetUserId: string) => Promise<void>
  stopImpersonation: () => Promise<void>
}

export const useAuthStore = create<AuthStore>((set) => ({
  user: null,
  loading: true,
  isImpersonating: false,
  impersonatedUsername: null,

  setUser: (user: User) => {
    set({ user, loading: false })
  },

  logout: async () => {
    try {
      await authApi.logout() // POST /auth/logout — cookie cleared server-side
    } catch {
      // ignore — clearing is best-effort
    }
    set({ user: null, loading: false, isImpersonating: false, impersonatedUsername: null })
    if (typeof window !== 'undefined') {
      window.location.href = '/'
    }
  },

  init: async () => {
    if (typeof window === 'undefined') {
      set({ loading: false })
      return
    }
    try {
      const user = await authApi.getMe() // GET /auth/me — succeeds if access cookie valid
      set({ user, loading: false })
    } catch {
      set({ user: null, loading: false })
    }
  },

  startImpersonation: async (targetUserId: string) => {
    const res = await authApi.impersonate(targetUserId) // POST /admin/users/:id/impersonate — sets cookie
    // The endpoint returns a partial user ({id,username,display_name}); the
    // store holds it for immediate UI (banner/nav). is_admin is intentionally
    // absent for the impersonated (non-admin) view.
    set({
      user: res.user as unknown as User,
      isImpersonating: true,
      impersonatedUsername: res.user.username,
    })
  },

  stopImpersonation: async () => {
    const res = await authApi.stopImpersonation() // POST /auth/stop-impersonation — restores admin cookie
    // The endpoint returns a partial user ({id,username,is_admin}); the store
    // holds it for immediate UI until the next /auth/me refresh.
    set({
      user: res.user as unknown as User,
      isImpersonating: false,
      impersonatedUsername: null,
    })
  },
}))
