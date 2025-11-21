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

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';

interface ProtectedRouteProps {
  children: React.ReactNode;
  requireAdmin?: boolean;
}

export function ProtectedRoute({ children, requireAdmin = false }: ProtectedRouteProps) {
  const router = useRouter();
  const { user, token, loading, init } = useAuthStore();
  const [isInitialized, setIsInitialized] = useState(false);

  useEffect(() => {
    init().then(() => setIsInitialized(true));
  }, [init]);

  useEffect(() => {
    if (!isInitialized || loading) {
      return;
    }

    if (!token || !user) {
      router.push('/');
      return;
    }

    if (requireAdmin && !user.is_admin) {
      router.push('/dashboard');
      return;
    }
  }, [token, user, loading, isInitialized, requireAdmin, router]);

  // Show loading spinner while initializing or checking auth
  if (!isInitialized || loading || !token || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-twitch"></div>
      </div>
    );
  }

  // Check admin requirement
  if (requireAdmin && !user.is_admin) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-white mb-4">Access Denied</h1>
          <p className="text-gray-400">You don't have permission to access this page.</p>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
