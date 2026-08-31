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
 * Protected Route Component
 *
 * HOC that requires authentication before rendering children.
 * Redirects to home page if user is not authenticated.
 *
 * Usage:
 *   <ProtectedRoute>
 *     <YourProtectedContent />
 *   </ProtectedRoute>
 */

'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useHydrated } from '@/hooks/useHydrated'
import { InfinityLogo } from '@/components/InfinityLogo'
import { useTranslations } from '@/lib/i18n'

interface ProtectedRouteProps {
  children: React.ReactNode
  requireAdmin?: boolean
}

// The no-entry sign shown above the 403 heading. Decoration beside copy that
// says the same thing in words, so not part of the catalog.
const FORBIDDEN_GLYPH = '\u{1F6AB}'

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const t = useTranslations()
  const router = useRouter()
  const { user, loading, init } = useAuthStore()
  const isHydrated = useHydrated()

  useEffect(() => {
    if (isHydrated) {
      init()
    }
  }, [isHydrated, init])

  useEffect(() => {
    if (!isHydrated || loading) {
      return // Still hydrating or loading
    }

    if (!user) {
      router.push('/')
      return
    }
  }, [user, loading, isHydrated, router])

  // Show loading state while initializing or checking auth
  if (!isHydrated || loading || !user) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg">
        <InfinityLogo size={64} />
      </div>
    )
  }

  // Check admin requirement - Show 403 Forbidden
  if (requireAdmin && !user.is_admin) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-bg">
        <div className="w-full max-w-md rounded-xl border border-border bg-surface p-8 text-center">
          <div className="mb-4 text-5xl">{FORBIDDEN_GLYPH}</div>
          <h1 className="mb-2 text-2xl font-bold text-text">{t('common.forbidden.heading')}</h1>
          <p className="mb-6 text-text-sub">{t('common.forbidden.body')}</p>
          <button
            onClick={() => router.push('/dashboard')}
            className="rounded-lg bg-twitch px-6 py-2 font-semibold text-bg transition hover:opacity-90"
          >
            {t('common.forbidden.dashboardButton')}
          </button>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
