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
import { useTranslations } from '@/lib/i18n'

interface BetaWarningProps {
  platform: 'youtube' | 'tiktok'
  onCancel: () => void
  onContinue: () => void
}

export function BetaWarning({ platform, onCancel, onContinue }: BetaWarningProps) {
  const t = useTranslations()

  // Whole strings per platform rather than one template with the platform name
  // spliced in: YouTube is under Google OAuth verification review and TikTok is
  // in closed beta, so the two sets differ by more than the name.
  const isYouTube = platform === 'youtube'
  const title = isYouTube
    ? t('common.betaWarning.youtubeTitle')
    : t('common.betaWarning.tiktokTitle')
  const message = isYouTube
    ? t('common.betaWarning.youtubeBody')
    : t('common.betaWarning.tiktokBody')
  const existingUserMessage = isYouTube
    ? t('common.betaWarning.youtubeExistingUser')
    : t('common.betaWarning.tiktokExistingUser')

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
            ? t('common.betaWarning.youtubeDiscordPrompt')
            : t('common.betaWarning.tiktokDiscordPrompt')}
        </p>

        <a
          href={DISCORD_INVITE_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-2 inline-flex items-center gap-2 text-sm text-blue-400 underline underline-offset-4 hover:text-blue-300"
        >
          <DiscordIcon className="h-4 w-4 shrink-0" />
          {t('common.betaWarning.discordLink')}
        </a>

        <p className="mt-3 text-xs text-text-sub">{existingUserMessage}</p>

        {/* Action buttons */}
        <div className="mt-6 flex justify-end gap-3">
          <button
            onClick={onCancel}
            className="rounded-lg bg-surface-2 px-4 py-2 font-medium text-text transition-opacity hover:opacity-80"
          >
            {t('common.betaWarning.cancelButton')}
          </button>
          <button
            onClick={onContinue}
            className="rounded-lg bg-yellow-600 px-4 py-2 font-medium text-white transition-opacity hover:opacity-80"
          >
            {t('common.betaWarning.continueButton')}
          </button>
        </div>
      </Dialog.Content>
    </Dialog.Root>
  )
}
