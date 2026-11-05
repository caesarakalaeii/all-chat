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
 * Server truth: users.tos_accepted for a streamer and viewers.tos_accepted for a
 * viewer signed in with a platform account (false = prompt shows). Accepting
 * persists via POST /api/v1/auth/me/tos-acceptance or
 * /api/v1/auth/viewer/me/tos-acceptance and refreshes the matching store. When the
 * terms change, resetting both columns to false re-arms the prompt for everyone.
 *
 * Both documents open in a new tab so the user can read them without losing the
 * prompt. The legal pages themselves and the OBS overlay routes never show it.
 */

import { useEffect, useState } from 'react'
import { usePathname } from 'next/navigation'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { authApi } from '@/lib/api/auth'
import { viewerApi } from '@/lib/api/viewer'
import { useAuthStore } from '@/lib/stores/auth-store'
import { useViewerAuthStore } from '@/lib/stores/viewer-auth-store'
import { useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { cn } from '@/lib/utils'

const EXEMPT_PREFIXES = ['/legal', '/overlay']

export default function TosAcceptanceDialog() {
  const t = useTranslations()
  const pathname = usePathname()
  const { user, isImpersonating, init, logout } = useAuthStore()
  const { viewerInfo, init: viewerInit, viewerLogout } = useViewerAuthStore()
  const [agreed, setAgreed] = useState(false)
  const [saving, setSaving] = useState(false)
  const [failed, setFailed] = useState(false)

  // Nothing else loads the viewer store on page load (the viewer pages read the
  // token directly), so this is what learns whether the viewer has accepted.
  useEffect(() => {
    void viewerInit()
  }, [viewerInit])

  // Strict false: a backend without the field must not lock everyone out.
  const streamerPending = user?.tos_accepted === false && !isImpersonating
  const viewerPending = viewerInfo?.tos_accepted === false
  const open =
    (streamerPending || viewerPending) && !EXEMPT_PREFIXES.some((p) => pathname?.startsWith(p))

  if (!open) return null

  const handleAccept = async () => {
    setSaving(true)
    setFailed(false)
    try {
      if (streamerPending) {
        await authApi.acceptTos()
        await init()
      }
      if (viewerPending) {
        await viewerApi.acceptTos()
        await viewerInit()
      }
    } catch {
      setFailed(true)
    } finally {
      setSaving(false)
    }
  }

  const handleLogout = () => {
    if (streamerPending) void logout()
    if (viewerPending) {
      // Same order as AppNav: the request keeps the token it read synchronously.
      viewerApi.logout().catch(() => {})
      viewerLogout()
    }
  }

  return (
    // Controlled with no close handler: Escape and outside clicks cannot dismiss
    // it. The only ways out are accepting or logging out.
    <AlertDialog.Root open>
      {/* The popup portals to <body>, outside any page's .lanes-app wrapper, so
          it carries the lanes class and the font variables itself (as the
          CookieBanner does). */}
      <AlertDialog.Content
        size="lg"
        className={cn(
          'lanes-app w-[calc(100%-2rem)] rounded-none border-lanes-fg-strong bg-lanes-panel',
          archivoBlack.variable,
          spaceMono.variable
        )}
      >
        <AlertDialog.Title className="mb-4 text-xl">
          {t('legal.tosAcceptance.title')}
        </AlertDialog.Title>
        <AlertDialog.Description className="text-sub text-sm leading-relaxed">
          {interpolateElements(t('legal.tosAcceptance.body'), {
            terms: (
              <a
                href="/legal/terms"
                target="_blank"
                rel="noopener noreferrer"
                className="font-bold underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--lanes-kick)"
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
                className="font-bold underline underline-offset-4 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--lanes-kick)"
              >
                {t('legal.tosAcceptance.privacyLinkText')}
                <span className="sr-only"> ({t('legal.tosAcceptance.newTabHint')})</span>
              </a>
            ),
          })}
        </AlertDialog.Description>

        <label className="mt-6 flex cursor-pointer items-start gap-3 text-sm text-lanes-fg">
          <input
            type="checkbox"
            checked={agreed}
            onChange={(e) => setAgreed(e.target.checked)}
            className="mt-0.5 size-4 shrink-0 accent-(--lanes-kick) focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-(--lanes-kick)"
          />
          {t('legal.tosAcceptance.agreement')}
        </label>

        {failed && (
          <p role="alert" className="mt-4 text-sm text-destructive">
            {t('legal.tosAcceptance.error')}
          </p>
        )}

        <div className="mt-6 flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <button className="lanes-btn ghost" onClick={handleLogout} disabled={saving}>
            {t('legal.tosAcceptance.logout')}
          </button>
          <button className="lanes-btn" onClick={handleAccept} disabled={!agreed || saving}>
            {saving ? t('legal.tosAcceptance.accepting') : t('legal.tosAcceptance.accept')}
          </button>
        </div>
      </AlertDialog.Content>
    </AlertDialog.Root>
  )
}
