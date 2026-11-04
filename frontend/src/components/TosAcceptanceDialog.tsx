'use client'

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
 * Blocking post-login prompt to accept the Terms of Service and Privacy Policy.
 *
 * Server truth: users.tos_accepted (false = prompt shows). Accepting persists via
 * POST /api/v1/auth/me/tos-acceptance and refreshes the auth store. When the terms
 * change, resetting the column to false re-arms the prompt for everyone.
 *
 * Both documents open in a new tab so the user can read them without losing the
 * prompt. The legal pages themselves and the OBS overlay routes never show it.
 */

import { useState } from 'react'
import { usePathname } from 'next/navigation'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { authApi } from '@/lib/api/auth'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

const EXEMPT_PREFIXES = ['/legal', '/overlay']

export default function TosAcceptanceDialog() {
  const t = useTranslations()
  const pathname = usePathname()
  const { user, isImpersonating, init, logout } = useAuthStore()
  const [agreed, setAgreed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [failed, setFailed] = useState(false)

  // Strict false: a backend without the field must not lock everyone out.
  const open =
    user?.tos_accepted === false &&
    !isImpersonating &&
    !EXEMPT_PREFIXES.some((p) => pathname?.startsWith(p))

  if (!open) return null

  const handleAccept = async () => {
    setSaving(true)
    setFailed(false)
    try {
      await authApi.acceptTos()
      await init()
    } catch {
      setFailed(true)
    } finally {
      setSaving(false)
    }
  }

  return (
    // Controlled with no close handler: Escape and outside clicks cannot dismiss
    // it. The only ways out are accepting or logging out.
    <AlertDialog.Root open>
      <AlertDialog.Content size="lg">
        <AlertDialog.Title>{t('legal.tosAcceptance.title')}</AlertDialog.Title>
        <AlertDialog.Description>
          {interpolateElements(t('legal.tosAcceptance.body'), {
            terms: (
              <a
                href="/legal/terms"
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-twitch underline decoration-twitch/30 underline-offset-4 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                {t('legal.tosAcceptance.termsLinkText')}
                <span className="sr-only"> ({t('legal.tosAcceptance.newTabHint')})</span>
              </a>
            ),
            privacy: (
              <a
                href="/legal/privacy"
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-twitch underline decoration-twitch/30 underline-offset-4 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
              >
                {t('legal.tosAcceptance.privacyLinkText')}
                <span className="sr-only"> ({t('legal.tosAcceptance.newTabHint')})</span>
              </a>
            ),
          })}
        </AlertDialog.Description>

        <label className="mt-4 flex cursor-pointer items-start gap-2 text-sm text-text">
          <input
            type="checkbox"
            checked={agreed}
            onChange={(e) => setAgreed(e.target.checked)}
            className="mt-0.5 accent-twitch"
          />
          {t('legal.tosAcceptance.agreement')}
        </label>

        {failed && (
          <p role="alert" className="mt-3 text-sm text-destructive">
            {t('legal.tosAcceptance.error')}
          </p>
        )}

        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" onClick={() => void logout()} disabled={saving}>
            {t('legal.tosAcceptance.logout')}
          </Button>
          <Button onClick={handleAccept} disabled={!agreed || saving}>
            {saving ? t('legal.tosAcceptance.accepting') : t('legal.tosAcceptance.accept')}
          </Button>
        </div>
      </AlertDialog.Content>
    </AlertDialog.Root>
  )
}
