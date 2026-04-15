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
 * Beta Warning Component
 *
 * Displays a warning for platforms in closed beta or under OAuth verification.
 * Non-blocking - informs users but allows them to proceed if already authorized.
 *
 * Uses Dialog component for accessible modal rendering.
 */

'use client'

import { Dialog } from '@/components/ui/dialog'

interface BetaWarningProps {
  platform: 'youtube' | 'tiktok'
  onCancel: () => void
  onContinue: () => void
}

export function BetaWarning({ platform, onCancel, onContinue }: BetaWarningProps) {
  const platformName = platform.charAt(0).toUpperCase() + platform.slice(1)

  // YouTube-specific messaging: Under Google OAuth verification review
  const isYouTube = platform === 'youtube'
  const title = isYouTube
    ? 'YouTube — OAuth Verification in Progress'
    : `${platformName} — Closed Beta`
  const message = isYouTube
    ? 'YouTube integration is currently under Google OAuth verification review. We cannot add new test users during this period.'
    : `${platformName} integration is currently in closed beta. If you haven't been added to the beta program yet, authentication will fail.`
  const existingUserMessage = isYouTube
    ? 'If you were previously added as a test user, you can continue to use YouTube integration.'
    : "If you're already in the beta, you can proceed with authentication."

  return (
    <Dialog.Root
      open
      onOpenChange={(open: boolean) => {
        if (!open) onCancel()
      }}
    >
      <Dialog.Content showCloseButton={false}>
        {/* Warning header */}
        <div className="mb-4 flex items-start gap-4">
          <div className="mt-0.5 flex-shrink-0">
            <svg
              className="h-6 w-6 text-yellow-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              aria-hidden="true"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
          </div>
          <Dialog.Title className="text-yellow-400">{title}</Dialog.Title>
        </div>

        <Dialog.Description>{message}</Dialog.Description>

        <p className="mt-3 text-sm text-text-sub">
          {isYouTube
            ? 'Join our Discord community to stay updated on verification progress and get support:'
            : 'To join the beta, please join our Discord community:'}
        </p>

        <a
          href="https://discord.gg/xCGBSuz39P"
          target="_blank"
          rel="noopener noreferrer"
          className="mt-2 inline-flex items-center gap-2 text-sm text-blue-400 underline underline-offset-4 hover:text-blue-300"
        >
          <svg
            className="h-4 w-4 shrink-0"
            fill="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z" />
          </svg>
          Join Discord Server
        </a>

        <p className="mt-3 text-xs text-text-sub">{existingUserMessage}</p>

        {/* Action buttons */}
        <div className="mt-6 flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="rounded-lg bg-surface-2 px-4 py-2 font-medium text-text transition-opacity hover:opacity-80"
          >
            Cancel
          </button>
          <button
            onClick={onContinue}
            className="rounded-lg bg-yellow-600 px-4 py-2 font-medium text-white transition-opacity hover:opacity-80"
          >
            I Understand, Continue
          </button>
        </div>
      </Dialog.Content>
    </Dialog.Root>
  )
}
