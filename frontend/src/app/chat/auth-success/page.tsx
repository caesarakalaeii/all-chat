/**
 * Viewer OAuth Success Page
 *
 * Handles successful OAuth authentication for viewers.
 * The backend redirects here with a JWT token as a query parameter.
 *
 * Flow:
 * 1. Extract token from URL (?token=xxx&streamer=yyy)
 * 2. Store viewer token in localStorage
 * 3. Fetch viewer info from API
 * 4. Redirect to chat page with streamer
 *
 * Route: /chat/auth-success
 */

'use client';

import { Suspense, useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { useViewerAuthStore } from '@/lib/stores/viewer-auth-store';
import { viewerApi } from '@/lib/api/viewer';

function AuthSuccessContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { setViewerToken, setViewerInfo } = useViewerAuthStore();

  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const handleSuccess = async () => {
      const token = searchParams.get('token');
      const streamer = searchParams.get('streamer');

      if (!token) {
        setError('No authentication token received');
        setLoading(false);

        // Notify opener (extension) about error
        if (window.opener) {
          window.opener.postMessage({
            type: 'ALLCHAT_AUTH_ERROR',
            error: 'No authentication token received',
          }, '*');
        }
        return;
      }

      // Check if this was opened from an extension popup
      const isExtensionPopup = window.opener && !window.opener.closed;

      if (isExtensionPopup) {
        // Extension flow: post message to opener and close
        console.log('[AllChat Auth] Posting token to extension opener');
        window.opener.postMessage({
          type: 'ALLCHAT_AUTH_SUCCESS',
          token,
          streamer,
        }, '*');

        // Show success message briefly before closing
        setLoading(false);
        setTimeout(() => {
          window.close();
        }, 1000);
        return;
      }

      // Web app flow: store token and redirect
      setViewerToken(token);

      try {
        // Fetch viewer info
        const viewerInfo = await viewerApi.getMe();
        setViewerInfo(viewerInfo);

        // Get the streamer username
        const redirectStreamer = streamer || localStorage.getItem('viewer_streamer');

        if (redirectStreamer) {
          // Redirect to chat page for the streamer
          router.push(`/chat/${redirectStreamer}`);
        } else {
          // No streamer context, redirect to home
          router.push('/');
        }
      } catch (err) {
        console.error('Failed to fetch viewer info:', err);
        setError('Failed to complete authentication. Please try again.');
        setLoading(false);
      }
    };

    handleSuccess();
  }, [searchParams, setViewerToken, setViewerInfo, router]);

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-900">
      {loading ? (
        <div className="text-center">
          <div className="animate-spin rounded-full h-16 w-16 border-b-2 border-purple-500 mx-auto mb-4"></div>
          <p className="text-white text-lg">Completing authentication...</p>
          <p className="text-gray-400 text-sm mt-2">Please wait</p>
        </div>
      ) : !error && window.opener ? (
        <div className="text-center">
          <div className="text-6xl mb-4">✅</div>
          <p className="text-green-500 text-lg mb-4">Authentication successful!</p>
          <p className="text-gray-400 text-sm">You can close this window</p>
        </div>
      ) : error ? (
        <div className="text-center">
          <div className="text-6xl mb-4">⚠️</div>
          <p className="text-red-500 text-lg mb-4">{error}</p>
          <a
            href="/"
            className="inline-block bg-purple-600 hover:bg-purple-700 text-white font-semibold py-2 px-6 rounded-lg transition-colors"
          >
            Return to Home
          </a>
        </div>
      ) : null}
    </div>
  );
}

export default function AuthSuccessPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-gray-900">
          <div className="animate-spin rounded-full h-16 w-16 border-b-2 border-purple-500"></div>
        </div>
      }
    >
      <AuthSuccessContent />
    </Suspense>
  );
}
