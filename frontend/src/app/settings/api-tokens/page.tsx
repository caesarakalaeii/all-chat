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
 * Settings → API tokens.
 *
 * Mint, list and revoke the personal access tokens (`allchat_pat_…`) that the
 * Stream Deck and StreamController plugins authenticate with.
 *
 * SECURITY INVARIANT, and the reason this file reads the way it does: the
 * plaintext token exists in exactly one place — the create response — and the
 * server stores only a SHA-256 digest, so it can never be shown again. It is
 * therefore held ONLY in the `minted` React state of ApiTokensContent, and
 * dropped when the user dismisses the reveal. It is deliberately NOT written to
 * web storage, a Zustand store, a cookie, or the URL: any of those would turn a
 * one-shot secret into a durable one that outlives the tab and is trivially
 * exfiltrated by any script on the origin — and this file is grepped in CI to
 * keep it that way. The listing below renders metadata only; it has no access
 * to a plaintext, by construction.
 */

import Link from 'next/link'
import { useCallback, useEffect, useId, useState } from 'react'

import { AppNav } from '@/components/AppNav'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Field } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Switch } from '@/components/ui/switch'
import {
  API_TOKEN_SCOPES,
  createApiToken,
  listApiTokens,
  revokeApiToken,
  type ApiToken,
  type ApiTokenScope,
  type CreatedApiToken,
} from '@/lib/api/api-tokens'
import { toastManager } from '@/lib/toast'

const STREAMDECK_README = 'https://github.com/caesarakalaeii/all-chat/tree/main/streamdeck-plugin'
const STREAMCONTROLLER_README =
  'https://github.com/caesarakalaeii/all-chat/tree/main/streamcontroller-plugin'

const SCOPE_LABELS: Record<ApiTokenScope, { title: string; description: string }> = {
  'chat:write': {
    title: 'Send chat messages',
    description: 'Lets the plugin post messages to your connected chats.',
  },
  'engagement:write': {
    title: 'Run polls and predictions',
    description: 'Lets the plugin open, resolve and cancel polls and predictions.',
  },
}

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

// ---------------------------------------------------------------------------
// MintedTokenReveal — the one-and-only sighting of a plaintext token
// ---------------------------------------------------------------------------

/**
 * `token` arrives as a prop and leaves with the component. Nothing here writes
 * it anywhere but the DOM node the user is looking at and, on explicit request,
 * the system clipboard (which is the user's own choice, not our storage).
 */
function MintedTokenReveal({
  minted,
  onDismiss,
}: {
  minted: CreatedApiToken
  onDismiss: () => void
}) {
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState<string | null>(null)

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(minted.token)
      setCopied(true)
      setCopyError(null)
    } catch {
      setCopyError('Could not copy automatically — select the token and copy it manually.')
    }
  }, [minted.token])

  return (
    <Card
      className="border-twitch/60 p-6"
      role="region"
      aria-label={`New token ${minted.name}`}
      data-testid="minted-token-reveal"
    >
      <h2 className="text-lg font-semibold text-text">Copy your new token now</h2>
      <p className="mt-1 text-sm text-amber-400">
        This is the only time <strong>{minted.name}</strong> will ever be shown. We store only a
        hash of it, so it cannot be displayed again — if you lose it, revoke this token and create a
        new one.
      </p>

      <code
        data-testid="minted-token-value"
        className="mt-4 block overflow-x-auto rounded-lg border border-border bg-surface-2 p-3 font-mono text-sm break-all text-text"
      >
        {minted.token}
      </code>

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <Button variant="outline" size="sm" onClick={() => void handleCopy()}>
          {copied ? 'Copied ✓' : 'Copy token'}
        </Button>
        <Button size="sm" onClick={onDismiss}>
          I&apos;ve saved it
        </Button>
      </div>
      {copyError && <p className="mt-2 text-xs text-red-400">{copyError}</p>}
    </Card>
  )
}

// ---------------------------------------------------------------------------
// CreateTokenForm — name + scopes
// ---------------------------------------------------------------------------

/**
 * Scopes are multi-select. There is no `checkbox.tsx` primitive in this repo, so
 * each scope is a `Switch` inside a `Field.Root` — which gives the label ↔
 * control association for free — rather than adding a new dependency.
 */
function CreateTokenForm({
  onCreated,
  disabled,
}: {
  onCreated: (minted: CreatedApiToken) => void
  disabled: boolean
}) {
  const nameId = useId()
  const [name, setName] = useState('')
  const [scopes, setScopes] = useState<Set<ApiTokenScope>>(new Set(['chat:write']))
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const toggleScope = (scope: ApiTokenScope, enabled: boolean) => {
    setScopes((previous) => {
      const next = new Set(previous)
      if (enabled) next.add(scope)
      else next.delete(scope)
      return next
    })
  }

  const trimmedName = name.trim()
  const canSubmit = trimmedName.length > 0 && scopes.size > 0 && !creating && !disabled

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!canSubmit) return

    setCreating(true)
    setError(null)
    try {
      // The resolved value carries the plaintext. It is handed straight up to the
      // page's ephemeral state and never touched again here.
      const minted = await createApiToken(trimmedName, [...scopes])
      onCreated(minted)
      setName('')
    } catch (err) {
      setError(
        err instanceof Error && err.message ? err.message : 'Could not create the token. Try again.'
      )
    } finally {
      setCreating(false)
    }
  }

  return (
    <Card className="p-6">
      <h2 className="text-lg font-semibold text-text">Create a token</h2>
      <p className="mt-1 mb-4 text-sm text-text-sub">
        Give the token a name you will recognise later, and grant it only what the device needs.
      </p>

      <form onSubmit={handleSubmit} className="space-y-5">
        <Field.Root>
          <Field.Label htmlFor={nameId}>Token name</Field.Label>
          <Field.Control
            render={
              <Input
                id={nameId}
                value={name}
                maxLength={120}
                placeholder="Stream Deck (studio PC)"
                onChange={(event: React.ChangeEvent<HTMLInputElement>) =>
                  setName(event.target.value)
                }
              />
            }
          />
          <Field.Description>Shown in the list below so you know what to revoke.</Field.Description>
        </Field.Root>

        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-text">Scopes</legend>
          {API_TOKEN_SCOPES.map((scope) => (
            <Field.Root key={scope} className="flex-row items-start gap-3">
              <Switch.Root
                checked={scopes.has(scope)}
                onCheckedChange={(checked: boolean) => toggleScope(scope, checked)}
                aria-label={SCOPE_LABELS[scope].title}
              >
                <Switch.Thumb />
              </Switch.Root>
              <div className="flex flex-col gap-0.5">
                <Field.Label className="cursor-pointer">{SCOPE_LABELS[scope].title}</Field.Label>
                <Field.Description className="text-xs">
                  {SCOPE_LABELS[scope].description} <code className="text-text-dim">{scope}</code>
                </Field.Description>
              </div>
            </Field.Root>
          ))}
          {scopes.size === 0 && (
            <p className="text-xs text-amber-400">
              Pick at least one scope — a token with none can authenticate but do nothing.
            </p>
          )}
        </fieldset>

        {error && <p className="text-sm text-red-400">{error}</p>}

        <Button type="submit" disabled={!canSubmit}>
          {creating ? 'Creating…' : 'Create token'}
        </Button>
      </form>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// EmptyState — what tokens are for, and where the plugins live
// ---------------------------------------------------------------------------

function EmptyState() {
  return (
    <div className="rounded-lg border border-dashed border-border p-6 text-center">
      <p className="text-sm font-medium text-text">You don&apos;t have any API tokens yet</p>
      <p className="mx-auto mt-2 max-w-md text-sm text-text-sub">
        A personal access token lets a device sign in as you without your password — it&apos;s how
        the Stream Deck and StreamController plugins send chat messages and run polls and
        predictions on your behalf. Create one per device so you can revoke it on its own.
      </p>
      <p className="mt-3 text-sm text-text-sub">
        Setup guides:{' '}
        <Link
          href={STREAMDECK_README}
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-twitch hover:underline"
        >
          Stream Deck plugin README
        </Link>{' '}
        ·{' '}
        <Link
          href={STREAMCONTROLLER_README}
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-twitch hover:underline"
        >
          StreamController plugin README
        </Link>
      </p>
    </div>
  )
}

// ---------------------------------------------------------------------------
// TokenRow — metadata only
// ---------------------------------------------------------------------------

function TokenRow({ token, onRevoke }: { token: ApiToken; onRevoke: (token: ApiToken) => void }) {
  return (
    <li className="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-border px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-text">{token.name}</p>
        <p className="mt-0.5 text-xs text-text-sub">
          Created {formatDate(token.created_at)} · Last used{' '}
          {token.last_used_at ? formatDate(token.last_used_at) : 'never'}
        </p>
        <p className="mt-1 flex flex-wrap gap-1.5">
          {token.scopes.map((scope) => (
            <span
              key={scope}
              className="rounded-full bg-surface-2 px-2 py-0.5 font-mono text-xs text-text-sub"
            >
              {scope}
            </span>
          ))}
        </p>
      </div>
      <Button
        variant="outline"
        size="sm"
        onClick={() => onRevoke(token)}
        aria-label={`Revoke ${token.name}`}
      >
        Revoke
      </Button>
    </li>
  )
}

// ---------------------------------------------------------------------------
// Page content
// ---------------------------------------------------------------------------

function ApiTokensContent() {
  const [tokens, setTokens] = useState<ApiToken[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  /**
   * The plaintext token, and the ONLY place it lives. Ephemeral component state
   * on purpose — see the file header. `setMinted(null)` is the whole of the
   * "forget it" path: there is nothing else to clean up, because nothing else
   * ever held it.
   */
  const [minted, setMinted] = useState<CreatedApiToken | null>(null)

  // Two-step revoke: the row's button only arms this, the alert dialog confirms.
  const [revokeTarget, setRevokeTarget] = useState<ApiToken | null>(null)
  const [revoking, setRevoking] = useState(false)

  const refresh = useCallback(async () => {
    try {
      setTokens(await listApiTokens())
      setLoadError(null)
    } catch {
      setLoadError('Could not load your tokens. Refresh the page to try again.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void (async () => {
      await refresh()
    })()
  }, [refresh])

  const handleCreated = useCallback((created: CreatedApiToken) => {
    setMinted(created)
    // Show the new token in the list straight away, without its plaintext: the
    // list state is typed ApiToken, so the secret cannot ride along.
    const { token: _plaintext, ...metadata } = created
    void _plaintext
    setTokens((previous) => [metadata, ...previous])
  }, [])

  async function handleConfirmRevoke() {
    if (!revokeTarget) return
    setRevoking(true)
    try {
      await revokeApiToken(revokeTarget.id)
      setTokens((previous) => previous.filter((t) => t.id !== revokeTarget.id))
      toastManager.add({ title: `Revoked ${revokeTarget.name}`, type: 'success' })
      setRevokeTarget(null)
    } catch {
      toastManager.add({ title: 'Could not revoke that token', type: 'error' })
    } finally {
      setRevoking(false)
    }
  }

  return (
    <div className="min-h-screen bg-bg">
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-2xl space-y-6 px-4 py-12">
        <div>
          <h1 className="text-2xl font-bold text-text">API Tokens</h1>
          <p className="mt-1 text-sm text-text-sub">
            Personal access tokens for the Stream Deck and StreamController plugins.
          </p>
        </div>

        {minted && <MintedTokenReveal minted={minted} onDismiss={() => setMinted(null)} />}

        <CreateTokenForm onCreated={handleCreated} disabled={loading} />

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text">Your tokens</h2>
          <p className="mt-1 mb-4 text-sm text-text-sub">
            Only the details below are stored — the token itself is kept as a hash and can never be
            shown again.
          </p>

          {loading ? (
            <div className="space-y-2">
              <Skeleton className="h-16 w-full" />
              <Skeleton className="h-16 w-full" />
            </div>
          ) : loadError ? (
            <p className="text-sm text-red-400">{loadError}</p>
          ) : tokens.length === 0 ? (
            <EmptyState />
          ) : (
            <ul className="space-y-2">
              {tokens.map((token) => (
                <TokenRow key={token.id} token={token} onRevoke={setRevokeTarget} />
              ))}
            </ul>
          )}
        </Card>
      </main>

      <AlertDialog.Root
        open={revokeTarget !== null}
        onOpenChange={(open: boolean) => {
          if (!open && !revoking) setRevokeTarget(null)
        }}
      >
        <AlertDialog.Content size="sm">
          <AlertDialog.Title>Revoke this token?</AlertDialog.Title>
          <AlertDialog.Description>
            {revokeTarget
              ? `“${revokeTarget.name}” stops working immediately. Any device using it will need a new token.`
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
              {revoking ? 'Revoking…' : 'Revoke token'}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </div>
  )
}

export default function ApiTokensPage() {
  return (
    <ProtectedRoute>
      <ApiTokensContent />
    </ProtectedRoute>
  )
}
