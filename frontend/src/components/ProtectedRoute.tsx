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
  }, [token, user, loading, isInitialized, router]);

  // Show loading spinner while initializing or checking auth
  if (!isInitialized || loading || !token || !user) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-900">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-twitch"></div>
      </div>
    );
  }

  // Check admin requirement - Show 403 Forbidden
  if (requireAdmin && !user.is_admin) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="max-w-md w-full bg-white shadow-lg rounded-lg p-8 text-center">
          <div className="text-6xl mb-4">🚫</div>
          <h1 className="text-3xl font-bold text-gray-900 mb-2">403 Forbidden</h1>
          <p className="text-gray-600 mb-6">
            You do not have permission to access this page. Admin privileges are required.
          </p>
          <button
            onClick={() => router.push('/dashboard')}
            className="bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700 transition"
          >
            Go to Dashboard
          </button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
