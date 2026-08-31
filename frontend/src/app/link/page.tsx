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
 * /link — the device approve screen (ADR-0049 step 3).
 *
 * Both delivery paths land here:
 *
 *   loopback   the plugin opened the system browser at /link?request_id=… so the
 *              request is already identified. Nothing is typed.
 *   code       the streamer is on a different machine and types the XXXX-XXXX
 *              code the plugin is showing them.
 *
 * WHAT THIS PAGE MUST MAKE UNMISTAKABLE, because it is the only human decision
 * point in the whole flow:
 *
 *   1. WHICH OVERLAY the device will be able to drive. A device token is bound to
 *      one overlay at pairing time, which is the property a personal access
 *      token structurally cannot have.
 *   2. WHICH SCOPES are granted, and that they may be narrowed here.
 *   3. That the device name is SELF-REPORTED by the plugin, not verified by us.
 *   4. The honest limit of the overlay binding: chat send has no overlay
 *      dimension, so a device granted chat:write can post to every connected
 *      platform on the account regardless of which overlay is picked. Saying this
 *      plainly is better than implying a stronger guarantee than exists.
 *
 * NO SECRET IS RENDERED HERE, ever. The device token goes from auth-service's
 * exchange endpoint to the plugin over the loopback redirect; the browser never
 * sees it. That is the difference from /settings/api-tokens, where a plaintext
 * PAT is shown exactly once.
 */

import { useRouter, useSearchParams } from 'next/navigation'
import Link from 'next/link'
import { useCallback, useEffect, useId, useState } from 'react'

import { AppNav } from '@/components/AppNav'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import { approveDevice, denyDevice, getPendingLink, type PendingLink } from '@/lib/api/devices'
import { overlaysApi } from '@/lib/api/overlays'
import type { Overlay } from '@/lib/types/overlay'
import { toastManager } from '@/lib/toast'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

/**
 * Scope string to its pair of `auth.link.scope*` leaves. `as const satisfies`
 * rather than a plain annotation: an annotation widens the stems to string, and
 * a typo would then resolve to a missing key at runtime instead of failing tsc.
 */
const SCOPE_MESSAGE_STEMS = {
  'chat:write': 'ChatWrite',
  'engagement:write': 'EngagementWrite',
} as const satisfies Record<string, string>

/**
 * Human copy for a scope, so an unexpected value still renders. An unknown
 * scope keeps its raw string as the title -- that is a protocol value, not
 * copy -- and borrows a generic description.
 *
 * Takes `t` as its first argument because a module-scope function cannot call
 * a hook.
 */
function scopeLabel(t: TFunction, scope: string): { title: string; description: string } {
  // Read defensively. Indexing a plain object with an unvalidated scope string
  // can resolve to an inherited Object prototype member, so the resolved value
  // must be one of our own stems before it is used to build a message key.
  const stem: unknown = SCOPE_MESSAGE_STEMS[scope as keyof typeof SCOPE_MESSAGE_STEMS]
  if (stem !== 'ChatWrite' && stem !== 'EngagementWrite') {
    return { title: scope, description: t('auth.link.scopeUnknownBody') }
  }
  return {
    title: t(`auth.link.scope${stem}Title`),
    description: t(`auth.link.scope${stem}Body`),
  }
}

// ---------------------------------------------------------------------------
// CodeEntry — the fallback path's first step
// ---------------------------------------------------------------------------

/**
 * The typed-code form. Reached when the plugin could not bind a loopback port or
 * could not open a browser — a Stream Deck driving a second PC, or a headless
 * capture box.
 *
 * The input is labelled, described, and its error is announced: `Field.Root`
 * wires aria-describedby and aria-invalid, and the error region carries
 * role="alert" so a screen reader hears a rejected code without the user having
 * to go looking for it.
 */
function CodeEntry({
  onFound,
  initialError,
}: {
  onFound: (pending: PendingLink, userCode: string) => void
  initialError: string | null
}) {
  const t = useTranslations()
  const [code, setCode] = useState('')
  const [error, setError] = useState<string | null>(initialError)
  const [checking, setChecking] = useState(false)
  const codeId = useId()
  const errorId = useId()

  const normalized = code.replace(/[\s-]/g, '').toUpperCase()
  const canSubmit = normalized.length === 8 && !checking

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!canSubmit) return
    setChecking(true)
    setError(null)
    try {
      const pending = await getPendingLink({ userCode: normalized })
      onFound(pending, normalized)
    } catch {
      setError(t('auth.link.codeInvalid'))
    } finally {
      setChecking(false)
    }
  }

  return (
    <Card className="p-6">
      <h2 className="text-lg font-semibold text-text">{t('auth.link.codeTitle')}</h2>
      <p className="mt-1 mb-4 text-sm text-text-sub">{t('auth.link.codeBody')}</p>

      <form onSubmit={handleSubmit} className="space-y-4">
        <Field.Root>
          <Field.Label htmlFor={codeId}>{t('auth.link.codeLabel')}</Field.Label>
          <Field.Control
            render={
              <Input
                id={codeId}
                value={code}
                autoComplete="off"
                autoCapitalize="characters"
                spellCheck={false}
                maxLength={9}
                placeholder={t('auth.link.codePlaceholder')}
                aria-invalid={error !== null}
                aria-describedby={error ? errorId : undefined}
                onChange={(event: React.ChangeEvent<HTMLInputElement>) =>
                  setCode(event.target.value)
                }
              />
            }
          />
          <Field.Description>{t('auth.link.codeHint')}</Field.Description>
        </Field.Root>

        {error && (
          <p id={errorId} role="alert" className="text-sm text-red-400">
            {error}
          </p>
        )}

        <Button type="submit" disabled={!canSubmit}>
          {checking ? t('auth.link.codeChecking') : t('auth.link.codeContinue')}
        </Button>
      </form>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// ApproveForm — what is being granted
// ---------------------------------------------------------------------------

function ApproveForm({
  pending,
  userCode,
  overlays,
  state,
}: {
  pending: PendingLink
  userCode: string | null
  overlays: Overlay[]
  /**
   * The plugin's `state` value, forwarded from the URL the plugin opened. We do
   * not interpret it — it is the plugin's own CSRF check on its own loopback
   * listener, so all this page does is carry it through to the callback, which
   * echoes it into the redirect.
   */
  state: string | null
}) {
  const t = useTranslations()
  const router = useRouter()
  // Default to the only overlay when there is exactly one: the common case is a
  // streamer with a single overlay, and making them pick from a list of one is
  // friction on the step where friction is least affordable.
  const [overlayId, setOverlayId] = useState(overlays.length === 1 ? overlays[0].id : '')
  const [scopes, setScopes] = useState<Set<string>>(new Set(pending.requested_scopes))
  const [deviceName, setDeviceName] = useState(pending.device_name_self_reported)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const nameId = useId()
  const errorId = useId()
  const overlayGroupId = useId()

  const canApprove = overlayId !== '' && scopes.size > 0 && !busy

  const toggleScope = useCallback((scope: string, on: boolean) => {
    setScopes((previous) => {
      const next = new Set(previous)
      if (on) next.add(scope)
      else next.delete(scope)
      return next
    })
  }, [])

  async function handleApprove(event: React.FormEvent) {
    event.preventDefault()
    if (!canApprove) return
    setBusy(true)
    setError(null)
    try {
      const approved = await approveDevice({
        requestId: userCode ? undefined : pending.request_id,
        userCode: userCode ?? undefined,
        overlayId,
        scopes: Array.from(scopes),
        deviceName: deviceName.trim() || undefined,
      })
      if (approved.redirect_to) {
        // The loopback flow: hand the browser to the server-side callback, which
        // re-validates the STORED redirect and emits the Location header itself.
        // The browser never chooses where the one-time code goes.
        const target = state
          ? `${approved.redirect_to}&state=${encodeURIComponent(state)}`
          : approved.redirect_to
        window.location.assign(target)
        return
      }
      // The code flow: the plugin is polling and will pick the token up itself.
      toastManager.add({
        title: t('auth.link.approvedTitle'),
        description: t('auth.link.approvedBody'),
        type: 'success',
      })
      router.push('/settings/devices')
    } catch {
      setError(t('auth.link.approveFailed'))
    } finally {
      setBusy(false)
    }
  }

  async function handleDeny() {
    setBusy(true)
    try {
      await denyDevice({
        requestId: userCode ? undefined : pending.request_id,
        userCode: userCode ?? undefined,
      })
      toastManager.add({ title: t('auth.link.deniedTitle'), type: 'success' })
      router.push('/settings/devices')
    } catch {
      setError(t('auth.link.denyFailed'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={handleApprove} className="space-y-6">
      <Card className="p-6">
        <h2 className="text-lg font-semibold text-text">{t('auth.link.approveTitle')}</h2>
        <p className="mt-1 text-sm text-text-sub">
          {interpolateElements(
            t('auth.link.approveBody', { name: pending.device_name_self_reported }),
            { selfReported: <em>{t('auth.link.approveSelfReported')}</em> }
          )}
        </p>
      </Card>

      <Card className="p-6">
        <fieldset>
          <legend className="text-sm font-medium text-text" id={overlayGroupId}>
            {t('auth.link.overlayLegend')}
          </legend>
          <p className="mt-1 mb-3 text-sm text-text-sub">{t('auth.link.overlayBody')}</p>
          {overlays.length === 0 ? (
            <p className="text-sm text-amber-400">{t('auth.link.overlayNone')}</p>
          ) : (
            <div className="space-y-2" role="radiogroup" aria-labelledby={overlayGroupId}>
              {overlays.map((overlay) => (
                <label
                  key={overlay.id}
                  className="flex cursor-pointer items-center gap-3 rounded-lg border border-border px-4 py-3 text-sm text-text has-[:focus-visible]:border-twitch"
                >
                  <input
                    type="radio"
                    name="overlay"
                    value={overlay.id}
                    checked={overlayId === overlay.id}
                    onChange={() => setOverlayId(overlay.id)}
                    className="size-6 accent-twitch focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
                  />
                  <span className="min-w-0 truncate">{overlay.name}</span>
                </label>
              ))}
            </div>
          )}
        </fieldset>
      </Card>

      <Card className="p-6">
        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-text">{t('auth.link.scopeLegend')}</legend>
          <p className="mb-1 text-sm text-text-sub">{t('auth.link.scopeBody')}</p>
          {pending.requested_scopes.map((scope) => (
            <Field.Root key={scope} className="flex-row items-start gap-3">
              <Switch.Root
                checked={scopes.has(scope)}
                onCheckedChange={(checked: boolean) => toggleScope(scope, checked)}
                aria-label={scopeLabel(t, scope).title}
              >
                <Switch.Thumb />
              </Switch.Root>
              <div className="flex flex-col gap-0.5">
                <Field.Label className="cursor-pointer">{scopeLabel(t, scope).title}</Field.Label>
                <Field.Description className="text-xs">
                  {scopeLabel(t, scope).description} <code className="text-text-dim">{scope}</code>
                </Field.Description>
              </div>
            </Field.Root>
          ))}
          {scopes.size === 0 && (
            <p className="text-xs text-amber-400">{t('auth.link.scopeNone')}</p>
          )}
          {scopes.has('chat:write') && (
            <p className="rounded-lg border border-amber-500/40 bg-amber-500/5 px-3 py-2 text-xs text-amber-200">
              {t('auth.link.chatWriteCaveat')}
            </p>
          )}
        </fieldset>
      </Card>

      <Card className="p-6">
        <Field.Root>
          <Field.Label htmlFor={nameId}>{t('auth.link.nameLabel')}</Field.Label>
          <Field.Control
            render={
              <Input
                id={nameId}
                value={deviceName}
                maxLength={120}
                placeholder={t('auth.link.namePlaceholder')}
                onChange={(event: React.ChangeEvent<HTMLInputElement>) =>
                  setDeviceName(event.target.value)
                }
              />
            }
          />
          <Field.Description>{t('auth.link.nameHint')}</Field.Description>
        </Field.Root>
      </Card>

      {error && (
        <p id={errorId} role="alert" className="text-sm text-red-400">
          {error}
        </p>
      )}

      <div className="flex flex-wrap gap-3">
        <Button type="submit" disabled={!canApprove}>
          {busy ? t('auth.link.approving') : t('auth.link.approve')}
        </Button>
        <Button type="button" variant="outline" disabled={busy} onClick={() => void handleDeny()}>
          {t('auth.link.deny')}
        </Button>
      </div>
    </form>
  )
}

// ---------------------------------------------------------------------------
// Page content
// ---------------------------------------------------------------------------

function LinkDeviceContent() {
  const t = useTranslations()
  const searchParams = useSearchParams()
  const requestId = searchParams.get('request_id')
  const state = searchParams.get('state')

  const [pending, setPending] = useState<PendingLink | null>(null)
  const [userCode, setUserCode] = useState<string | null>(null)
  const [overlays, setOverlays] = useState<Overlay[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    void (async () => {
      try {
        setOverlays(await overlaysApi.list())
      } catch {
        setLoadError(t('auth.link.overlaysFailed'))
      }
      if (requestId) {
        try {
          setPending(await getPendingLink({ requestId }))
        } catch {
          setLoadError(t('auth.link.requestExpired'))
        }
      }
      setLoading(false)
    })()
  }, [requestId, t])

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <div>
          <h1 className="text-2xl font-bold text-text">{t('auth.link.title')}</h1>
          <p className="mt-1 text-sm text-text-sub">
            {interpolateElements(t('auth.link.intro'), {
              devices: (
                <Link
                  href="/settings/devices"
                  className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
                >
                  {t('auth.link.introDevices')}
                </Link>
              ),
            })}
          </p>
        </div>

        {loading ? (
          <div className="space-y-3" role="status">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-40 w-full" />
            <span className="sr-only">{t('auth.link.loading')}</span>
          </div>
        ) : pending ? (
          <ApproveForm pending={pending} userCode={userCode} overlays={overlays} state={state} />
        ) : (
          <CodeEntry
            initialError={loadError}
            onFound={(found, code) => {
              setPending(found)
              setUserCode(code)
              setLoadError(null)
            }}
          />
        )}
      </main>
    </div>
  )
}

export default function LinkDevicePage() {
  return (
    <ProtectedRoute>
      <LinkDeviceContent />
    </ProtectedRoute>
  )
}
