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

'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { useHydrated } from '@/hooks/useHydrated';

interface ProtectedRouteProps {
  children: React.ReactNode;
  requireAdmin?: boolean;
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const router = useRouter();
  const { user, token, loading, init } = useAuthStore();
  const isHydrated = useHydrated();

  useEffect(() => {
    if (isHydrated) {
      init();
    }
  }, [isHydrated, init]);

  useEffect(() => {
    if (!isHydrated || loading) {
      return; // Still hydrating or loading
    }

    if (!token || !user) {
      router.push('/');
      return;
    }
  }, [token, user, loading, isHydrated, router]);

  // Show loading spinner while initializing or checking auth
  if (!isHydrated || loading || !token || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <div className="animate-spin rounded-full h-12 w-12 border-2 border-transparent border-t-twitch"></div>
      </div>
    );
  }

  // Check admin requirement - Show 403 Forbidden
  if (requireAdmin && !user.is_admin) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-bg">
        <div className="max-w-md w-full border border-border bg-surface rounded-xl p-8 text-center">
          <div className="text-5xl mb-4">🚫</div>
          <h1 className="text-2xl font-bold text-text mb-2">403 Forbidden</h1>
          <p className="text-text-sub mb-6">
            You do not have permission to access this page. Admin privileges are required.
          </p>
          <button
            onClick={() => router.push('/dashboard')}
            className="bg-twitch text-white px-6 py-2 rounded-lg hover:opacity-90 transition font-semibold"
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
