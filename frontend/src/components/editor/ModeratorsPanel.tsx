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

'use client'

/**
 * Owner-side Moderators panel (ADR-0048): invite, narrow and revoke the people who
 * may moderate this overlay's chat.
 *
 * Three behaviours here are load-bearing rather than cosmetic:
 *
 * 1. An invite secret is shown EXACTLY once. Only its SHA-256 is stored, so there is
 *    no endpoint that could show it again — the recovery is a new invite, and the copy
 *    has to say so rather than implying a second chance.
 * 2. The moderator cap is enforced locally from `used`/`cap`, so the streamer never
 *    meets a 409 the UI could have prevented.
 * 3. `verification` is telemetry, never authorization. A single transient platform 403
 *    can set `not_a_moderator`, so it renders as "worth checking", never as a denial —
 *    the platform's answer at action time is the only authority.
 *
 * Revocation is deliberately never gated: the server keeps it working after a rollback
 * of the delegation feature, so this panel must not re-introduce the trap of a streamer
 * holding moderators they cannot remove.
 */

import { useCallback, useEffect, useState } from 'react'

import { AlertDialog } from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Dialog } from '@/components/ui/dialog'
import { Switch } from '@/components/ui/switch'
import { Input } from '@/components/ui/input'
import { boundInviteAccount, delegationErrorCode, moderationApi } from '@/lib/api/moderation'
import { useTranslations, type TFunction } from '@/lib/i18n'
import { emphasise } from '@/lib/i18n/emphasise'
import {
  DELEGATABLE_ACTIONS,
  DELEGATABLE_PLATFORMS,
  type DelegatablePlatform,
  type GrantVerification,
  type InviteCreated,
  type ModerationAction,
  type ModeratorGrant,
  type ModeratorList,
} from '@/lib/types/moderation'
import { cn } from '@/lib/utils'

/** Display name for a platform, from the catalog. */
function platformLabel(t: TFunction, platform: DelegatablePlatform): string {
  return t(`common.platforms.${platform}`)
}

/** Display name for a delegatable action, from the catalog. */
function actionLabel(t: TFunction, action: ModerationAction): string {
  return t(`moderation.actions.${action}`)
}

/**
 * Advisory readiness copy for a leg. Every string is phrased as something to check or
 * do, never as a verdict about the person: this value is the last platform answer we
 * happened to observe, not a decision.
 */
function readinessNote(
  t: TFunction,
  platform: DelegatablePlatform,
  v: GrantVerification
): string | null {
  const name = platformLabel(t, platform)
  switch (v) {
    case 'not_a_moderator':
      return t('moderation.readiness.notAModerator', { platform: name })
    case 'needs_consent':
      return t('moderation.readiness.needsConsent', { platform: name })
    case 'needs_discord_link':
      return t('moderation.readiness.needsDiscordLink')
    case 'unavailable':
      return t('moderation.readiness.unavailable', { platform: name })
    case 'verified':
    case 'unverified':
      return null
  }
}

/** Human label for a grant's lifecycle state. */
function statusLabel(t: TFunction, grant: ModeratorGrant): string {
  switch (grant.status) {
    case 'pending':
      return t('moderation.status.pending')
    case 'suspended':
      return t('moderation.status.suspended')
    case 'revoked':
      return t('moderation.status.revoked')
    case 'active':
      return t('moderation.status.active')
  }
}

/** Who a row is about: the accepted account, else whatever the streamer typed. */
function grantName(t: TFunction, grant: ModeratorGrant): string {
  return grant.display_name || grant.invitee_label || t('moderation.unnamedInvite')
}

/**
 * The redemption link for a fresh invite secret.
 *
 * Absolute, because the streamer pastes it into a DM on some other service where a
 * relative path means nothing. Built from the current origin rather than a configured
 * base URL so a preview deployment hands out links to itself.
 */
function inviteURL(token: string): string {
  const path = `/moderate/accept?token=${encodeURIComponent(token)}`
  return typeof window === 'undefined' ? path : `${window.location.origin}${path}`
}

export function ModeratorsPanel({ overlayId }: { overlayId: string }) {
  const t = useTranslations()
  const [list, setList] = useState<ModeratorList | null>(null)
  const [loadFailed, setLoadFailed] = useState(false)
  const [inviteOpen, setInviteOpen] = useState(false)
  const [revoking, setRevoking] = useState<ModeratorGrant | null>(null)
  const [revokeAllOpen, setRevokeAllOpen] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [rowError, setRowError] = useState<string | null>(null)

  // A failure here includes the code-less owner-only 403, which is identical for an
  // unauthorized caller and for an overlay that does not exist — so it tells us nothing
  // we may report about the caller, and renders as a plain retry.
  const fetchRoster = useCallback(() => moderationApi.listModerators(overlayId), [overlayId])

  // The initial load sets state from the promise callback rather than the effect body:
  // a synchronous setState in an effect cascades renders. `cancelled` keeps a slow
  // response from landing after unmount, or from overwriting a newer overlay's roster.
  useEffect(() => {
    let cancelled = false
    fetchRoster()
      .then((next) => {
        if (cancelled) return
        setList(next)
        setLoadFailed(false)
      })
      .catch(() => {
        if (!cancelled) setLoadFailed(true)
      })
    return () => {
      cancelled = true
    }
  }, [fetchRoster])

  /** Imperative refresh after a mutation or a retry click (an event, not an effect). */
  const reload = useCallback(async () => {
    try {
      setList(await fetchRoster())
      setLoadFailed(false)
    } catch {
      setLoadFailed(true)
    }
  }, [fetchRoster])

  const handleToggleLeg = async (
    grant: ModeratorGrant,
    platform: DelegatablePlatform,
    enabled: boolean
  ) => {
    setRowError(null)
    try {
      // A partial map: only the platform that moved is sent, so the rest keep whatever
      // the streamer set before.
      await moderationApi.updateGrant(overlayId, grant.id, { platforms: { [platform]: enabled } })
      await reload()
    } catch {
      setRowError(
        t('moderation.row.legChangeFailed', {
          platform: platformLabel(t, platform),
          name: grantName(t, grant),
        })
      )
    }
  }

  const handleToggleAction = async (grant: ModeratorGrant, action: ModerationAction) => {
    const next = grant.actions.includes(action)
      ? grant.actions.filter((a) => a !== action)
      : [...grant.actions, action]
    // An empty action set is a 400 server-side and a nonsense grant besides, so removing
    // the last one is not offered as an edit — remove the moderator instead.
    if (next.length === 0) {
      setRowError(t('moderation.row.needsOneAction', { name: grantName(t, grant) }))
      return
    }
    setRowError(null)
    try {
      await moderationApi.updateGrant(overlayId, grant.id, {
        actions: DELEGATABLE_ACTIONS.filter((a) => next.includes(a)),
      })
      await reload()
    } catch {
      setRowError(t('moderation.row.updateFailed', { name: grantName(t, grant) }))
    }
  }

  const handleRevoke = async () => {
    if (!revoking) return
    const name = grantName(t, revoking)
    setRevoking(null)
    try {
      await moderationApi.revokeGrant(overlayId, revoking.id)
      setNotice(t('moderation.row.removed', { name }))
      await reload()
    } catch {
      setRowError(t('moderation.row.removeFailed', { name }))
    }
  }

  const handleRevokeAll = async () => {
    setRevokeAllOpen(false)
    try {
      const { revoked } = await moderationApi.revokeAllModerators(overlayId)
      setNotice(
        t(revoked === 1 ? 'moderation.row.removedCountOne' : 'moderation.row.removedCountMany', {
          count: String(revoked),
        })
      )
      await reload()
    } catch {
      setRowError(t('moderation.row.removeAllFailed'))
    }
  }

  if (loadFailed) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-text-sub">{t('moderation.roster.loadFailed')}</p>
        <Button variant="outline" size="sm" onClick={() => void reload()}>
          {t('moderation.roster.tryAgain')}
        </Button>
      </div>
    )
  }

  if (list === null) {
    return <p className="text-sm text-text-dim">{t('moderation.roster.loading')}</p>
  }

  const atCap = list.used >= list.cap

  return (
    <div className="space-y-4">
      <p className="text-sm text-text-sub">
        {emphasise(
          t('moderation.roster.explainer', {
            emphasis: t('moderation.roster.explainerEmphasis'),
          }),
          t('moderation.roster.explainerEmphasis'),
          (run) => (
            <strong>{run}</strong>
          )
        )}
      </p>

      <div className="flex flex-wrap items-center gap-3">
        <Button size="sm" disabled={atCap} onClick={() => setInviteOpen(true)}>
          {t('moderation.roster.inviteButton')}
        </Button>
        <span className="text-xs text-text-dim">
          {t('moderation.roster.usage', { used: String(list.used), cap: String(list.cap) })}
        </span>
        {list.moderators.length > 0 && (
          <Button
            variant="destructive"
            size="sm"
            className="ml-auto"
            onClick={() => setRevokeAllOpen(true)}
          >
            {t('moderation.roster.removeAll')}
          </Button>
        )}
      </div>

      {atCap && (
        <p className="text-xs text-text-dim">
          {t('moderation.roster.atCap', { cap: String(list.cap) })}
        </p>
      )}

      {notice !== null && (
        <p role="status" className="text-xs text-text-sub">
          {notice}
        </p>
      )}
      {rowError !== null && (
        <p role="alert" className="text-xs text-destructive">
          {rowError}
        </p>
      )}

      {list.moderators.length === 0 ? (
        <p className="text-sm text-text-dim">{t('moderation.roster.empty')}</p>
      ) : (
        <ul className="space-y-3">
          {list.moderators.map((grant) => (
            <ModeratorRow
              key={grant.id}
              grant={grant}
              onToggleLeg={(platform, enabled) => void handleToggleLeg(grant, platform, enabled)}
              onToggleAction={(action) => void handleToggleAction(grant, action)}
              onRemove={() => setRevoking(grant)}
            />
          ))}
        </ul>
      )}

      <InviteDialog
        overlayId={overlayId}
        open={inviteOpen}
        onOpenChange={(open) => {
          setInviteOpen(open)
          if (!open) void reload()
        }}
      />

      <AlertDialog.Root
        open={revoking !== null}
        onOpenChange={(open) => {
          if (!open) setRevoking(null)
        }}
      >
        <AlertDialog.Content size="sm">
          <AlertDialog.Title className="text-sm font-medium">
            {t('moderation.revoke.title', {
              name: revoking === null ? '' : grantName(t, revoking),
            })}
          </AlertDialog.Title>
          <AlertDialog.Description className="text-xs">
            {t('moderation.revoke.description')}
          </AlertDialog.Description>
          <div className="mt-4 flex justify-end gap-2">
            <AlertDialog.Close
              render={
                <Button variant="outline" size="sm">
                  {t('moderation.revoke.cancel')}
                </Button>
              }
            />
            <Button variant="destructive" size="sm" onClick={() => void handleRevoke()}>
              {t('moderation.revoke.confirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>

      <AlertDialog.Root open={revokeAllOpen} onOpenChange={setRevokeAllOpen}>
        <AlertDialog.Content size="sm">
          <AlertDialog.Title className="text-sm font-medium">
            {t('moderation.revokeAll.title')}
          </AlertDialog.Title>
          <AlertDialog.Description className="text-xs">
            {t('moderation.revokeAll.description')}
          </AlertDialog.Description>
          <div className="mt-4 flex justify-end gap-2">
            <AlertDialog.Close
              render={
                <Button variant="outline" size="sm">
                  {t('moderation.revoke.cancel')}
                </Button>
              }
            />
            <Button variant="destructive" size="sm" onClick={() => void handleRevokeAll()}>
              {t('moderation.revokeAll.confirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </div>
  )
}

interface ModeratorRowProps {
  grant: ModeratorGrant
  onToggleLeg: (platform: DelegatablePlatform, enabled: boolean) => void
  onToggleAction: (action: ModerationAction) => void
  onRemove: () => void
}

function ModeratorRow({ grant, onToggleLeg, onToggleAction, onRemove }: ModeratorRowProps) {
  const t = useTranslations()
  const name = grantName(t, grant)
  const legs = new Map(grant.platforms.map((leg) => [leg.platform, leg]))

  return (
    <li className="rounded-lg border border-border bg-surface p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-text">{name}</p>
          <p className="text-xs text-text-dim">
            {statusLabel(t, grant)}
            {grant.status === 'pending' && grant.expected_account !== undefined && (
              <> · {t('moderation.row.forAccount', { account: grant.expected_account })}</>
            )}
          </p>
        </div>
        <Button
          variant="destructive"
          size="sm"
          aria-label={t('moderation.row.removeLabel', { name })}
          onClick={onRemove}
        >
          {t('moderation.row.removeButton')}
        </Button>
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {DELEGATABLE_ACTIONS.map((action) => {
          const on = grant.actions.includes(action)
          return (
            <button
              key={action}
              type="button"
              aria-pressed={on}
              onClick={() => onToggleAction(action)}
              className={cn(
                'rounded-md border px-2 py-0.5 text-xs transition-colors focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                on
                  ? 'border-twitch/40 bg-twitch/10 text-text'
                  : 'border-border bg-surface-2 text-text-dim'
              )}
            >
              {actionLabel(t, action)}
            </button>
          )
        })}
      </div>

      <ul className="mt-3 space-y-1.5">
        {DELEGATABLE_PLATFORMS.map((platform) => {
          const leg = legs.get(platform)
          const enabled = leg?.enabled ?? false
          const note = leg === undefined ? null : readinessNote(t, platform, leg.verification)
          return (
            <li key={platform} className="flex items-start gap-2">
              <Switch.Root
                checked={enabled}
                aria-label={t('moderation.row.platformToggleLabel', {
                  platform: platformLabel(t, platform),
                  name,
                })}
                onCheckedChange={(next: boolean) => onToggleLeg(platform, next)}
              >
                <Switch.Thumb />
              </Switch.Root>
              <div className="min-w-0">
                <span className="text-xs text-text-sub">{platformLabel(t, platform)}</span>
                {enabled && note !== null && <p className="text-xs text-text-dim">{note}</p>}
              </div>
            </li>
          )
        })}
      </ul>
    </li>
  )
}

interface InviteDialogProps {
  overlayId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

function InviteDialog({ overlayId, open, onOpenChange }: InviteDialogProps) {
  const t = useTranslations()
  const [actions, setActions] = useState<ModerationAction[]>(['delete', 'timeout'])
  const [platforms, setPlatforms] = useState<DelegatablePlatform[]>([])
  const [label, setLabel] = useState('')
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<InviteCreated | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [gateBlocked, setGateBlocked] = useState(false)
  const [copied, setCopied] = useState(false)

  const reset = () => {
    setActions(['delete', 'timeout'])
    setPlatforms([])
    setLabel('')
    setCreated(null)
    setError(null)
    setGateBlocked(false)
    setCopied(false)
  }

  const handleCreate = async () => {
    // Unreachable via the UI (the button is disabled), but an explicitly empty action
    // list is a 400 server-side, so never let one leave here.
    if (actions.length === 0) return
    setCreating(true)
    setError(null)
    setGateBlocked(false)
    try {
      setCreated(
        await moderationApi.createInvite(overlayId, {
          actions: DELEGATABLE_ACTIONS.filter((a) => actions.includes(a)),
          platforms: DELEGATABLE_PLATFORMS.filter((p) => platforms.includes(p)),
          invitee_label: label.trim(),
        })
      )
    } catch (err) {
      const code = delegationErrorCode(err)
      if (code === 'delegation_unavailable') {
        // Sticky and inline, not a toast: this is something the streamer has to act on.
        setGateBlocked(true)
      } else if (code === 'moderator_cap_reached') {
        setError(t('moderation.invite.capReached'))
      } else {
        const bound = boundInviteAccount(err)
        setError(
          bound === null
            ? t('moderation.invite.createFailed')
            : t('moderation.invite.boundToAccount', { account: bound.account })
        )
      }
    } finally {
      setCreating(false)
    }
  }

  // What the streamer actually sends. A bare secret is not actionable — the recipient
  // would have to be told separately where to paste it — so the invite is handed over as
  // the link that redeems it. The secret is still the only thing that authorizes, and it
  // still exists only in this response.
  const inviteLink = created === null ? '' : inviteURL(created.invite_token)

  const handleCopy = async () => {
    if (created === null) return
    try {
      await navigator.clipboard.writeText(inviteLink)
      setCopied(true)
    } catch {
      setError(t('moderation.invite.copyFailed'))
    }
  }

  return (
    <Dialog.Root
      open={open}
      onOpenChange={(next: boolean) => {
        if (!next) reset()
        onOpenChange(next)
      }}
    >
      <Dialog.Content size="sm">
        <Dialog.Title className="text-sm font-medium">{t('moderation.invite.title')}</Dialog.Title>

        {created !== null ? (
          <div className="space-y-3">
            <Dialog.Description className="text-xs">
              {emphasise(
                t('moderation.invite.sendLink', {
                  emphasis: t('moderation.invite.sendLinkEmphasis'),
                }),
                t('moderation.invite.sendLinkEmphasis'),
                (run) => (
                  <strong>{run}</strong>
                )
              )}
            </Dialog.Description>
            <code className="block overflow-x-auto rounded-lg border border-border bg-surface-2 p-2 text-xs break-all text-text">
              {inviteLink}
            </code>
            <div className="flex justify-end gap-2">
              <Button variant="outline" size="sm" onClick={() => void handleCopy()}>
                {copied ? t('moderation.invite.copied') : t('moderation.invite.copyLink')}
              </Button>
              <Button size="sm" onClick={() => onOpenChange(false)}>
                {t('moderation.invite.done')}
              </Button>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <Dialog.Description className="text-xs">
              {t('moderation.invite.acceptExplainer')}
            </Dialog.Description>

            <label className="block space-y-1">
              <span className="text-xs text-text-sub">{t('moderation.invite.labelPrompt')}</span>
              <Input
                type="text"
                value={label}
                maxLength={120}
                onChange={(e) => setLabel(e.target.value)}
                placeholder={t('moderation.invite.labelPlaceholder')}
              />
            </label>

            <fieldset className="space-y-1.5">
              <legend className="text-xs text-text-sub">
                {t('moderation.invite.actionsLegend')}
              </legend>
              {DELEGATABLE_ACTIONS.map((action) => (
                <label key={action} className="flex items-center gap-2 text-xs text-text">
                  <input
                    type="checkbox"
                    checked={actions.includes(action)}
                    onChange={(e) =>
                      setActions((prev) =>
                        e.target.checked ? [...prev, action] : prev.filter((a) => a !== action)
                      )
                    }
                    className="accent-twitch"
                  />
                  {actionLabel(t, action)}
                </label>
              ))}
            </fieldset>

            <fieldset className="space-y-1.5">
              <legend className="text-xs text-text-sub">
                {t('moderation.invite.platformsLegend')}
              </legend>
              {DELEGATABLE_PLATFORMS.map((platform) => (
                <label key={platform} className="flex items-center gap-2 text-xs text-text">
                  <input
                    type="checkbox"
                    checked={platforms.includes(platform)}
                    onChange={(e) =>
                      setPlatforms((prev) =>
                        e.target.checked ? [...prev, platform] : prev.filter((p) => p !== platform)
                      )
                    }
                    className="accent-twitch"
                  />
                  {platformLabel(t, platform)}
                </label>
              ))}
            </fieldset>

            {gateBlocked && (
              <div className="space-y-1 rounded-lg border border-border bg-surface-2 p-2">
                <p className="text-xs text-text-sub">{t('moderation.invite.premiumGate')}</p>
                <a
                  href="/upgrade"
                  className="text-xs text-twitch underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                >
                  {t('moderation.invite.upgradeLink')}
                </a>
              </div>
            )}
            {error !== null && (
              <p role="alert" className="text-xs text-destructive">
                {error}
              </p>
            )}

            <div className="flex justify-end gap-2">
              <Dialog.Close
                render={
                  <Button variant="outline" size="sm">
                    {t('moderation.invite.cancel')}
                  </Button>
                }
              />
              <Button
                size="sm"
                disabled={actions.length === 0 || creating}
                onClick={() => void handleCreate()}
              >
                {creating ? t('moderation.invite.creating') : t('moderation.invite.create')}
              </Button>
            </div>
          </div>
        )}
      </Dialog.Content>
    </Dialog.Root>
  )
}
