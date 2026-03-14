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
import Link from 'next/link';
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
    <div className="flex min-h-screen items-center justify-center bg-slate-900">
      {loading ? (
        <div className="text-center">
          <div className="mx-auto mb-4 h-16 w-16 animate-spin rounded-full border-b-2 border-purple-500"></div>
          <p className="text-lg text-white">Completing authentication...</p>
          <p className="mt-2 text-sm text-slate-400">Please wait</p>
        </div>
      ) : !error && window.opener ? (
        <div className="text-center">
          <div className="mb-4 text-6xl">✅</div>
          <p className="mb-4 text-lg text-green-500">Authentication successful!</p>
          <p className="text-sm text-slate-400">You can close this window</p>
        </div>
      ) : error ? (
        <div className="text-center">
          <div className="mb-4 text-6xl">⚠️</div>
          <p className="mb-4 text-lg text-red-500">{error}</p>
          <Link
            href="/"
            className="inline-block rounded-lg bg-purple-600 px-6 py-2 font-semibold text-white transition-colors hover:bg-purple-700"
          >
            Return to Home
          </Link>
        </div>
      ) : null}
    </div>
  );
}

export default function AuthSuccessPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center bg-slate-900">
          <div className="h-16 w-16 animate-spin rounded-full border-b-2 border-purple-500"></div>
        </div>
      }
    >
      <AuthSuccessContent />
    </Suspense>
  );
}
