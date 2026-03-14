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

interface ProtectedRouteProps {
  children: React.ReactNode
  requireAdmin?: boolean
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const router = useRouter()
  const { user, token, loading, init } = useAuthStore()
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

    if (!token || !user) {
      router.push('/')
      return
    }
  }, [token, user, loading, isHydrated, router])

  // Show loading state while initializing or checking auth
  if (!isHydrated || loading || !token || !user) {
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
          <div className="mb-4 text-5xl">🚫</div>
          <h1 className="mb-2 text-2xl font-bold text-text">403 Forbidden</h1>
          <p className="mb-6 text-text-sub">
            You do not have permission to access this page. Admin privileges are required.
          </p>
          <button
            onClick={() => router.push('/dashboard')}
            className="rounded-lg bg-twitch px-6 py-2 font-semibold text-white transition hover:opacity-90"
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
