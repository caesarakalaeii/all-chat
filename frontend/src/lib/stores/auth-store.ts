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

export const useAuthStore = create<AuthStore>((set, get) => ({
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
      const me = await authApi.getMe() // GET /auth/me — succeeds if access cookie valid
      // Restore impersonation state from the JWT's ImpersonatedBy claim so the
      // banner + admin-route guards survive a page reload (audit H3).
      set({
        user: me,
        loading: false,
        isImpersonating: !!me.impersonating,
        impersonatedUsername: me.impersonating ? me.username : null,
      })
    } catch {
      set({ user: null, loading: false })
    }
  },

  startImpersonation: async (targetUserId: string) => {
    // POST /admin/users/:id/impersonate — server sets an impersonated-user
    // access cookie. The response only carries a partial user
    // ({id,username,display_name}), so fetch the full User via /auth/me to
    // avoid an inconsistent store shape (audit L12 — previously a
    // `as unknown as User` double-cast masked the missing fields).
    const res = await authApi.impersonate(targetUserId)
    try {
      const me = await authApi.getMe()
      set({
        user: me,
        isImpersonating: true,
        impersonatedUsername: me.username,
      })
    } catch {
      // Cookie swap succeeded but /auth/me failed — fall back to merging the
      // partial response into the current user so the banner still shows.
      const current = get().user
      const partial: Partial<User> = {
        id: res.user.id,
        username: res.user.username,
      }
      if (res.user.display_name) {
        partial.display_name = res.user.display_name
      }
      set({
        user: current ? { ...current, ...partial } : null,
        isImpersonating: true,
        impersonatedUsername: res.user.username,
      })
    }
  },

  stopImpersonation: async () => {
    // POST /auth/stop-impersonation — server restores the admin access cookie.
    // The response only carries a partial user ({id,username,is_admin}), so
    // fetch the full User via /auth/me for a consistent store shape (audit L12).
    const res = await authApi.stopImpersonation()
    try {
      const me = await authApi.getMe()
      set({
        user: me,
        isImpersonating: false,
        impersonatedUsername: null,
      })
    } catch {
      // Cookie swap succeeded but /auth/me failed — fall back to merging the
      // partial response into the current user.
      const current = get().user
      const partial: Partial<User> = {
        id: res.user.id,
        username: res.user.username,
        is_admin: res.user.is_admin,
      }
      set({
        user: current ? { ...current, ...partial } : null,
        isImpersonating: false,
        impersonatedUsername: null,
      })
    }
  },
}))
