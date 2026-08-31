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
import { type TFunction, formatDateTime, useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'
import { toastManager } from '@/lib/toast'

const STREAMDECK_README = 'https://github.com/caesarakalaeii/all-chat/tree/main/streamdeck-plugin'
const STREAMCONTROLLER_README =
  'https://github.com/caesarakalaeii/all-chat/tree/main/streamcontroller-plugin'

/**
 * Catalog key stem per scope. `satisfies` rather than a type annotation so the
 * stems stay literal — an annotation widens them to string and a typo stops
 * failing tsc at the t() call.
 */
const SCOPE_MESSAGE_STEMS = {
  'chat:write': 'scopeChatWrite',
  'engagement:write': 'scopeEngagementWrite',
} as const satisfies Record<ApiTokenScope, string>

function formatDayOrUnknown(t: TFunction, value: string | null): string {
  if (!value) return t('settings.apiTokens.unknownDate')
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return t('settings.apiTokens.unknownDate')
  return formatDateTime(parsed, { year: 'numeric', month: 'short', day: 'numeric' })
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
  const t = useTranslations()
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState<string | null>(null)

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(minted.token)
      setCopied(true)
      setCopyError(null)
    } catch {
      setCopyError(t('settings.apiTokens.copyFailed'))
    }
  }, [minted.token, t])

  return (
    <Card
      className="border-twitch/60 p-6"
      role="region"
      aria-label={t('settings.apiTokens.revealRegionLabel', { name: minted.name })}
      data-testid="minted-token-reveal"
    >
      <h2 className="text-lg font-semibold text-text">{t('settings.apiTokens.revealHeading')}</h2>
      <p className="mt-1 text-sm text-amber-400">
        {interpolateElements(t('settings.apiTokens.revealWarning'), {
          name: <strong>{minted.name}</strong>,
        })}
      </p>

      <code
        data-testid="minted-token-value"
        className="mt-4 block overflow-x-auto rounded-lg border border-border bg-surface-2 p-3 font-mono text-sm break-all text-text"
      >
        {minted.token}
      </code>

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <Button variant="outline" size="sm" onClick={() => void handleCopy()}>
          {copied ? t('settings.apiTokens.copied') : t('settings.apiTokens.copyToken')}
        </Button>
        <Button size="sm" onClick={onDismiss}>
          {t('settings.apiTokens.dismissReveal')}
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
  const t = useTranslations()
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
        err instanceof Error && err.message ? err.message : t('settings.apiTokens.createFailed')
      )
    } finally {
      setCreating(false)
    }
  }

  return (
    <Card className="p-6">
      <h2 className="text-lg font-semibold text-text">{t('settings.apiTokens.createHeading')}</h2>
      <p className="mt-1 mb-4 text-sm text-text-sub">{t('settings.apiTokens.createBody')}</p>

      <form onSubmit={handleSubmit} className="space-y-5">
        <Field.Root>
          <Field.Label htmlFor={nameId}>{t('settings.apiTokens.nameLabel')}</Field.Label>
          <Field.Control
            render={
              <Input
                id={nameId}
                value={name}
                maxLength={120}
                placeholder={t('settings.apiTokens.namePlaceholder')}
                onChange={(event: React.ChangeEvent<HTMLInputElement>) =>
                  setName(event.target.value)
                }
              />
            }
          />
          <Field.Description>{t('settings.apiTokens.nameDescription')}</Field.Description>
        </Field.Root>

        <fieldset className="space-y-3">
          <legend className="text-sm font-medium text-text">
            {t('settings.apiTokens.scopesLegend')}
          </legend>
          {API_TOKEN_SCOPES.map((scope) => {
            const title = t(`settings.apiTokens.${SCOPE_MESSAGE_STEMS[scope]}Title`)
            return (
              <Field.Root key={scope} className="flex-row items-start gap-3">
                <Switch.Root
                  checked={scopes.has(scope)}
                  onCheckedChange={(checked: boolean) => toggleScope(scope, checked)}
                  aria-label={title}
                >
                  <Switch.Thumb />
                </Switch.Root>
                <div className="flex flex-col gap-0.5">
                  <Field.Label className="cursor-pointer">{title}</Field.Label>
                  <Field.Description className="text-xs">
                    {t(`settings.apiTokens.${SCOPE_MESSAGE_STEMS[scope]}Description`)}{' '}
                    <code className="text-text-dim">{scope}</code>
                  </Field.Description>
                </div>
              </Field.Root>
            )
          })}
          {scopes.size === 0 && (
            <p className="text-xs text-amber-400">{t('settings.apiTokens.noScopesWarning')}</p>
          )}
        </fieldset>

        {error && <p className="text-sm text-red-400">{error}</p>}

        <Button type="submit" disabled={!canSubmit}>
          {creating ? t('settings.apiTokens.creating') : t('settings.apiTokens.create')}
        </Button>
      </form>
    </Card>
  )
}

// ---------------------------------------------------------------------------
// EmptyState — what tokens are for, and where the plugins live
// ---------------------------------------------------------------------------

function EmptyState() {
  const t = useTranslations()
  return (
    <div className="rounded-lg border border-dashed border-border p-6 text-center">
      <p className="text-sm font-medium text-text">{t('settings.apiTokens.emptyHeading')}</p>
      <p className="mx-auto mt-2 max-w-md text-sm text-text-sub">
        {t('settings.apiTokens.emptyBody')}
      </p>
      <p className="mt-3 text-sm text-text-sub">
        {t('settings.apiTokens.setupGuides')}{' '}
        <Link
          href={STREAMDECK_README}
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-twitch hover:underline"
        >
          {t('settings.apiTokens.streamDeckReadme')}
        </Link>{' '}
        ·{' '}
        <Link
          href={STREAMCONTROLLER_README}
          target="_blank"
          rel="noopener noreferrer"
          className="font-medium text-twitch hover:underline"
        >
          {t('settings.apiTokens.streamControllerReadme')}
        </Link>
      </p>
    </div>
  )
}

// ---------------------------------------------------------------------------
// TokenRow — metadata only
// ---------------------------------------------------------------------------

function TokenRow({ token, onRevoke }: { token: ApiToken; onRevoke: (token: ApiToken) => void }) {
  const t = useTranslations()
  return (
    <li className="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-border px-4 py-3">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-text">{token.name}</p>
        <p className="mt-0.5 text-xs text-text-sub">
          {t('settings.apiTokens.tokenDates', {
            created: formatDayOrUnknown(t, token.created_at),
            lastUsed: token.last_used_at
              ? formatDayOrUnknown(t, token.last_used_at)
              : t('settings.apiTokens.neverUsed'),
          })}
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
        aria-label={t('settings.apiTokens.revokeLabel', { name: token.name })}
      >
        {t('settings.apiTokens.revoke')}
      </Button>
    </li>
  )
}

// ---------------------------------------------------------------------------
// Page content
// ---------------------------------------------------------------------------

function ApiTokensContent() {
  const t = useTranslations()
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
      setLoadError(t('settings.apiTokens.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t])

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
          <h1 className="text-2xl font-bold text-text">{t('settings.apiTokens.heading')}</h1>
          <p className="mt-1 text-sm text-text-sub">{t('settings.apiTokens.subheading')}</p>
        </div>

        {minted && <MintedTokenReveal minted={minted} onDismiss={() => setMinted(null)} />}

        <CreateTokenForm onCreated={handleCreated} disabled={loading} />

        <Card className="p-6">
          <h2 className="text-lg font-semibold text-text">{t('settings.apiTokens.listHeading')}</h2>
          <p className="mt-1 mb-4 text-sm text-text-sub">{t('settings.apiTokens.listBody')}</p>

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
          <AlertDialog.Title>{t('settings.apiTokens.revokeConfirmTitle')}</AlertDialog.Title>
          <AlertDialog.Description>
            {revokeTarget
              ? t('settings.apiTokens.revokeConfirmBody', { name: revokeTarget.name })
              : ''}
          </AlertDialog.Description>
          <div className="mt-6 flex justify-end gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={revoking}
              onClick={() => setRevokeTarget(null)}
            >
              {t('settings.apiTokens.revokeCancel')}
            </Button>
            <Button size="sm" disabled={revoking} onClick={() => void handleConfirmRevoke()}>
              {revoking ? t('settings.apiTokens.revoking') : t('settings.apiTokens.revokeConfirm')}
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
