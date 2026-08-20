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
import { toastManager } from '@/lib/toast'

const STREAMDECK_GUIDE =
  'https://github.com/caesarakalaeii/all-chat/blob/main/docs/guides/streamdeck.md'

function formatDate(value: string | null): string {
  if (!value) return '—'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '—'
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
function statusOf(device: PairedDevice): string {
  if (device.revoked_at) return `Revoked ${formatDate(device.revoked_at)}`
  if (!isLive(device)) return `Expired ${formatDate(device.expires_at)}`
  return `Active until ${formatDate(device.expires_at)}`
}

// ---------------------------------------------------------------------------
// EmptyState — how linking actually starts
// ---------------------------------------------------------------------------

function EmptyState() {
  return (
    <div className="rounded-lg border border-dashed border-border p-6 text-center">
      <p className="text-sm font-medium text-text">No paired devices yet</p>
      <p className="mx-auto mt-2 max-w-md text-sm text-text-sub">
        Linking starts in the plugin, not here: open your Stream Deck or StreamController settings
        and press <strong className="font-medium text-text">Link with All-Chat</strong>. Your
        browser opens an approve screen, you pick an overlay, and the plugin receives its credential
        directly — nothing is copied or pasted.
      </p>
      <p className="mt-3 text-sm text-text-sub">
        <Link
          href={STREAMDECK_GUIDE}
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
        >
          Setup guide
        </Link>{' '}
        ·{' '}
        <Link
          href="/link"
          className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
        >
          I have a pairing code
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
  const live = isLive(device)
  return (
    <li className="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-border px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-text">{device.name}</p>
        <p className="mt-0.5 text-xs text-text-sub">
          Controls{' '}
          <span className="font-medium text-text">{device.overlay_name || device.overlay_id}</span>{' '}
          · {statusOf(device)}
        </p>
        <p className="mt-0.5 text-xs text-text-sub">
          Last used {device.last_used_at ? formatDate(device.last_used_at) : 'never'} · Paired{' '}
          {formatDate(device.created_at)}
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
          aria-label={`Revoke ${device.name}`}
        >
          Revoke
        </Button>
      )}
    </li>
  )
}

// ---------------------------------------------------------------------------
// Page content
// ---------------------------------------------------------------------------

function DevicesContent() {
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
      setLoadError('Could not load your paired devices. Refresh the page to try again.')
    } finally {
      setLoading(false)
    }
  }, [])

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
          <h1 className="text-2xl font-bold text-text">Paired devices</h1>
          <p className="mt-1 text-sm text-text-sub">
            Stream Deck and StreamController control surfaces linked to your account. Each one is
            locked to a single overlay and lapses on its own if it stops being used.
          </p>
        </div>

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text">Your devices</h2>
          <p className="mt-1 mb-4 text-sm text-text-sub">
            Only the details below are stored. The credential itself was sent straight to the plugin
            and is kept as a hash here — it is never shown in this dashboard, which is why there is
            nothing on this page to copy.
          </p>

          {loading ? (
            <div className="space-y-2" role="status">
              <Skeleton className="h-20 w-full" />
              <Skeleton className="h-20 w-full" />
              <span className="sr-only">Loading paired devices</span>
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
            On a second machine or a headless box?
          </h2>
          <p className="mt-1 text-sm text-text-sub">
            Linking needs the plugin and this browser on the same computer. When they are not — a
            Stream Deck driving a capture PC, a server with no desktop — use a{' '}
            <Link
              href="/settings/api-tokens"
              className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
            >
              personal access token
            </Link>{' '}
            instead, or start with{' '}
            <Link
              href="/link"
              className="font-medium text-twitch hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-twitch"
            >
              a pairing code
            </Link>{' '}
            if your plugin is showing one.
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
          <AlertDialog.Title>Revoke this device?</AlertDialog.Title>
          <AlertDialog.Description>
            {revokeTarget
              ? `“${revokeTarget.name}” stops working immediately. Link it again from the plugin if you still want to use it.`
              : ''}
          </AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={revoking}
              onClick={() => setRevokeTarget(null)}
            >
              Cancel
            </Button>
            <Button size="sm" disabled={revoking} onClick={() => void handleConfirmRevoke()}>
              {revoking ? 'Revoking…' : 'Revoke device'}
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
