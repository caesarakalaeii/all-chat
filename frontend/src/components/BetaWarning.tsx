/**
 * Beta Warning Component
 *
 * Displays a warning banner for platforms in closed beta or under OAuth verification.
 * Non-blocking - informs users but allows them to proceed if they're already authorized.
 */

'use client';

interface BetaWarningProps {
  platform: 'youtube' | 'tiktok';
  onCancel: () => void;
  onContinue: () => void;
}

export function BetaWarning({ platform, onCancel, onContinue }: BetaWarningProps) {
  const platformName = platform.charAt(0).toUpperCase() + platform.slice(1);

  // YouTube-specific messaging: Under Google OAuth verification review
  const isYouTube = platform === 'youtube';
  const title = isYouTube ? 'YouTube - OAuth Verification in Progress' : `${platformName} - Closed Beta`;
  const message = isYouTube
    ? 'YouTube integration is currently under Google OAuth verification review. We cannot add new test users during this period.'
    : `${platformName} integration is currently in closed beta. If you haven't been added to the beta program yet, authentication will fail.`;
  const existingUserMessage = isYouTube
    ? 'If you were previously added as a test user, you can continue to use YouTube integration.'
    : 'If you\'re already in the beta, you can proceed with authentication.';

  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm flex items-center justify-center z-50 p-4">
      <div className="bg-gray-800 border border-yellow-500/30 rounded-lg max-w-md w-full p-6 shadow-xl">
        {/* Warning Icon */}
        <div className="flex items-start gap-4">
          <div className="flex-shrink-0">
            <svg
              className="w-8 h-8 text-yellow-500"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>

          <div className="flex-1">
            <h3 className="text-xl font-semibold text-yellow-400 mb-2">
              {title}
            </h3>
            <p className="text-gray-300 mb-4">
              {message}
            </p>
            <p className="text-gray-300 mb-4">
              {isYouTube
                ? 'Join our Discord community to stay updated on verification progress and get support:'
                : 'To join the beta, please join our Discord community:'}
            </p>
            <a
              href="https://discord.gg/xCGBSuz39P"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 text-blue-400 hover:text-blue-300 underline underline-offset-4 mb-4"
            >
              <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/>
              </svg>
              Join Discord Server
            </a>
            <p className="text-sm text-gray-400">
              {existingUserMessage}
            </p>
          </div>
        </div>

        {/* Action Buttons */}
        <div className="mt-6 flex gap-3 justify-end">
          <button
            onClick={onCancel}
            className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white rounded-lg transition-colors font-medium"
          >
            Cancel
          </button>
          <button
            onClick={onContinue}
            className="px-4 py-2 bg-yellow-600 hover:bg-yellow-700 text-white rounded-lg transition-colors font-medium"
          >
            I Understand, Continue
          </button>
        </div>
      </div>
    </div>
  );
}
