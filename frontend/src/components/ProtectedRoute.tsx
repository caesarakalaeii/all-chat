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
 *
 * A caller that must not send an anonymous visitor to the homepage passes a
 * `fallback` to render in place of the redirect — see the prop's comment.
 */

'use client'

import { useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useHydrated } from '@/hooks/useHydrated'
import { InfinityLogo } from '@/components/InfinityLogo'
import { useTranslations } from '@/lib/i18n'

interface ProtectedRouteProps {
  children: React.ReactNode
  requireAdmin?: boolean
  /**
   * What to render for an anonymous visitor INSTEAD of redirecting home.
   *
   * Exists for surfaces where the homepage is not reachable copy: an OBS custom
   * browser dock is a chromeless ~320px panel with its own cookie jar, so the
   * redirect lands the streamer on the marketing page with no back button and
   * no sign-in affordance. Absent (the default for every other protected
   * route), the redirect is unchanged.
   */
  fallback?: React.ReactNode
}

// The no-entry sign shown above the 403 heading. Decoration beside copy that
// says the same thing in words, so not part of the catalog.
const FORBIDDEN_GLYPH = '\u{1F6AB}'

export function ProtectedRoute({ children, requireAdmin = false, fallback }: ProtectedRouteProps) {
  const t = useTranslations()
  const router = useRouter()
  const { user, loading, init } = useAuthStore()
  const isHydrated = useHydrated()
  // A JSX element is a fresh object every render, so the effect below depends on
  // whether a fallback was passed rather than on the node itself.
  const hasFallback = fallback !== undefined

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
      // A caller with a fallback renders it in place; navigating away would
      // discard the surface that explains how to sign in.
      if (!hasFallback) {
        router.push('/')
      }
      return
    }
  }, [user, loading, isHydrated, router, hasFallback])

  if (isHydrated && !loading && !user && hasFallback) {
    return <>{fallback}</>
  }

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
          <Button onClick={() => router.push('/dashboard')} size="lg">
            {t('common.forbidden.dashboardButton')}
          </Button>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
