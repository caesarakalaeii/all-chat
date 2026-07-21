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
import { DiscordIcon } from '@/components/icons/DiscordIcon'
import { DISCORD_INVITE_URL } from '@/lib/constants'

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
          href={DISCORD_INVITE_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-2 inline-flex items-center gap-2 text-sm text-blue-400 underline underline-offset-4 hover:text-blue-300"
        >
          <DiscordIcon className="h-4 w-4 shrink-0" />
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
