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
 * Settings → Paired devices.
 *
 * The list of Stream Deck / StreamController control surfaces linked to this
 * account (ADR-0049), with the overlay each is bound to, its scopes, when it was
 * last used, when it lapses, and one-click revoke.
 *
 * MODELLED ON /settings/api-tokens, WITH ONE DELIBERATE OMISSION: there is no
 * "shown once" plaintext anywhere on this page, and no state that could hold one.
 * A device token's secret travels from auth-service's exchange endpoint to the
 * plugin over the loopback redirect; it never enters a browser. That is the whole
 * reason ADR-0049 preferred this flow over a pasted token — a secret that is
 * never rendered cannot be read aloud, screenshotted or leaked on camera.
 *
 * So: nothing here reveals, copies or stores a credential. There is no `minted`
 * state, because there is nothing to mint from the browser. Linking starts in the
 * plugin and is approved at /link.
 *
 * Personal access tokens still exist and are still supported — see
 * /settings/api-tokens. They are the path for a headless capture box, a second PC
 * or a CLI, which a loopback redirect cannot reach.
 */

import Link from 'next/link'
import { useCallback, useEffect, useState } from 'react'

import { AppNav } from '@/components/AppNav'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { listDevices, revokeDevice, type PairedDevice } from '@/lib/api/devices'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { toastManager } from '@/lib/toast'

const STREAMDECK_GUIDE =
  'https://github.com/caesarakalaeii/all-chat/blob/main/docs/guides/streamdeck.md'

function formatDate(t: TFunction, value: string | null): string {
  if (!value) return t('settings.devices.unknownDate')
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return t('settings.devices.unknownDate')
  return parsed.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

/** True when the device is neither revoked nor past its sliding expiry. */
function isLive(device: PairedDevice): boolean {
  if (device.revoked_at) return false
  const expires = new Date(device.expires_at)
  return Number.isNaN(expires.getTime()) || expires.getTime() > Date.now()
}

/** Short human status, so a lapsed pairing is obvious without reading dates. */
function statusOf(t: TFunction, device: PairedDevice): string {
  if (device.revoked_at)
    return t('settings.devices.statusRevoked', { date: formatDate(t, device.revoked_at) })
  if (!isLive(device))
    return t('settings.devices.statusExpired', { date: formatDate(t, device.expires_at) })
  return t('settings.devices.statusActive', { date: formatDate(t, device.expires_at) })
}

// ---------------------------------------------------------------------------
// EmptyState — how linking actually starts
// ---------------------------------------------------------------------------

function EmptyState() {
  const t = useTranslations()
  return (
    <div className="rounded-lg border border-dashed border-border p-6 text-center">
      <p className="text-sm font-medium text-text">{t('settings.devices.emptyHeading')}</p>
      <p className="mx-auto mt-2 max-w-md text-sm text-text-sub">
        {interpolateElements(t('settings.devices.emptyBody'), {
          linkAction: (
            <strong className="font-medium text-text">
              {t('settings.devices.emptyLinkAction')}
            </strong>
          ),
        })}
      </p>
      <p className="mt-3 text-sm text-text-sub">
        <Link
          href={STREAMDECK_GUIDE}
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
        >
          {t('settings.devices.setupGuide')}
        </Link>{' '}
        ·{' '}
        <Link
          href="/link"
          className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
        >
          {t('settings.devices.havePairingCode')}
        </Link>
      </p>
    </div>
  )
}

// ---------------------------------------------------------------------------
// DeviceRow — metadata only, by construction
// ---------------------------------------------------------------------------

function DeviceRow({
  device,
  onRevoke,
}: {
  device: PairedDevice
  onRevoke: (device: PairedDevice) => void
}) {
  const t = useTranslations()
  const live = isLive(device)
  return (
    <li className="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-border px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-text">{device.name}</p>
        <p className="mt-0.5 text-xs text-text-sub">
          {interpolateElements(
            t('settings.devices.controlsOverlay', { status: statusOf(t, device) }),
            {
              overlay: (
                <span className="font-medium text-text">
                  {device.overlay_name || device.overlay_id}
                </span>
              ),
            }
          )}
        </p>
        <p className="mt-0.5 text-xs text-text-sub">
          {t('settings.devices.rowDates', {
            lastUsed: device.last_used_at
              ? formatDate(t, device.last_used_at)
              : t('settings.devices.neverUsed'),
            paired: formatDate(t, device.created_at),
          })}
        </p>
        <p className="mt-1 flex flex-wrap gap-1.5">
          {device.scopes.map((scope) => (
            <span
              key={scope}
              className="rounded-full bg-surface-2 px-2 py-0.5 font-mono text-xs text-text-sub"
            >
              {scope}
            </span>
          ))}
        </p>
      </div>
      {live && (
        <Button
          variant="outline"
          size="sm"
          onClick={() => onRevoke(device)}
          aria-label={t('settings.devices.revokeLabel', { name: device.name })}
        >
          {t('settings.devices.revoke')}
        </Button>
      )}
    </li>
  )
}

// ---------------------------------------------------------------------------
// Page content
// ---------------------------------------------------------------------------

function DevicesContent() {
  const t = useTranslations()
  const [devices, setDevices] = useState<PairedDevice[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  // Two-step revoke: the row's button only arms this, the alert dialog confirms.
  const [revokeTarget, setRevokeTarget] = useState<PairedDevice | null>(null)
  const [revoking, setRevoking] = useState(false)

  const refresh = useCallback(async () => {
    try {
      setDevices(await listDevices())
      setLoadError(null)
    } catch {
      setLoadError(t('settings.devices.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void (async () => {
      await refresh()
    })()
  }, [refresh])

  async function handleConfirmRevoke() {
    if (!revokeTarget) return
    setRevoking(true)
    try {
      await revokeDevice(revokeTarget.id)
      setDevices((previous) => previous.filter((d) => d.id !== revokeTarget.id))
      toastManager.add({ title: `Revoked ${revokeTarget.name}`, type: 'success' })
      setRevokeTarget(null)
    } catch {
      toastManager.add({ title: 'Could not revoke that device', type: 'error' })
    } finally {
      setRevoking(false)
    }
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <div>
          <h1 className="text-2xl font-bold text-text">{t('settings.devices.heading')}</h1>
          <p className="mt-1 text-sm text-text-sub">{t('settings.devices.subheading')}</p>
        </div>

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text">{t('settings.devices.listHeading')}</h2>
          <p className="mt-1 mb-4 text-sm text-text-sub">{t('settings.devices.listBody')}</p>

          {loading ? (
            <div className="space-y-2" role="status">
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-20 w-full" />
              <span className="sr-only">{t('settings.devices.loadingLabel')}</span>
            </div>
          ) : loadError ? (
            <p role="alert" className="text-sm text-red-400">
              {loadError}
            </p>
          ) : devices.length === 0 ? (
            <EmptyState />
          ) : (
            <ul className="space-y-2">
              {devices.map((device) => (
                <DeviceRow key={device.id} device={device} onRevoke={setRevokeTarget} />
              ))}
            </ul>
          )}
        </Card>

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text">
            {t('settings.devices.headlessHeading')}
          </h2>
          <p className="mt-1 text-sm text-text-sub">
            {interpolateElements(t('settings.devices.headlessBody'), {
              tokenLink: (
                <Link
                  href="/settings/api-tokens"
                  className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
                >
                  {t('settings.devices.headlessTokenLink')}
                </Link>
              ),
              pairingLink: (
                <Link
                  href="/link"
                  className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
                >
                  {t('settings.devices.headlessPairingLink')}
                </Link>
              ),
            })}
          </p>
        </Card>
      </main>

      <AlertDialog.Root
        open={revokeTarget !== null}
        onOpenChange={(open: boolean) => {
          if (!open && !revoking) setRevokeTarget(null)
        }}
      >
        <AlertDialog.Content size="sm">
          <AlertDialog.Title>{t('settings.devices.revokeConfirmTitle')}</AlertDialog.Title>
          <AlertDialog.Description>
            {revokeTarget
              ? t('settings.devices.revokeConfirmBody', { name: revokeTarget.name })
              : ''}
          </AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={revoking}
              onClick={() => setRevokeTarget(null)}
            >
              {t('settings.devices.revokeCancel')}
            </Button>
            <Button size="sm" disabled={revoking} onClick={() => void handleConfirmRevoke()}>
              {revoking ? t('settings.devices.revoking') : t('settings.devices.revokeConfirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </div>
  )
}

export default function DevicesPage() {
  return (
    <ProtectedRoute>
      <DevicesContent />
    </ProtectedRoute>
  )
}
