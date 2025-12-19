/**
 * Viewer OAuth Error Page
 *
 * Displays error when viewer authentication fails.
 * The backend redirects here with an error parameter.
 *
 * Route: /chat/auth-error
 */

'use client';

import { Suspense } from 'react';
import { useSearchParams } from 'next/navigation';

function AuthErrorContent() {
  const searchParams = useSearchParams();
  const error = searchParams.get('error') || 'Authentication failed';

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-900">
      <div className="text-center max-w-md">
        <div className="text-6xl mb-6">❌</div>
        <h1 className="text-3xl font-bold text-white mb-4">Authentication Failed</h1>
        <p className="text-red-400 text-lg mb-6">{error}</p>
        <p className="text-gray-400 mb-8">
          There was an error authenticating with your Twitch account.
          Please try again or contact support if the problem persists.
        </p>
        <div className="space-y-4">
          <a
            href="/"
            className="block bg-purple-600 hover:bg-purple-700 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Return to Home
          </a>
          <button
            onClick={() => window.history.back()}
            className="block w-full bg-gray-700 hover:bg-gray-600 text-white font-semibold py-3 px-6 rounded-lg transition-colors"
          >
            Go Back
          </button>
        </div>
      </div>
    </div>
  );
}

export default function AuthErrorPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-gray-900">
          <div className="animate-spin rounded-full h-16 w-16 border-b-2 border-purple-500"></div>
        </div>
      }
    >
      <AuthErrorContent />
    </Suspense>
  );
}
