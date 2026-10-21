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

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import { Check, CircleAlert, Plus, Scissors } from 'lucide-react'
import { AppNav } from '@/components/AppNav'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { AlertDialog } from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'
import { engagementApi } from '@/lib/api/engagement'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { useTranslations } from '@/lib/i18n'
import { toastManager } from '@/lib/toast'
import { cn } from '@/lib/utils'
import type { ChatCommand, ChatCommandInput, ChatCommandStatus } from '@/lib/types/engagement'
import { CommandEditor } from './CommandEditor'

const BLANK: ChatCommandInput = {
  kind: 'command',
  trigger: '!',
  response: '',
  create_clip: false,
  min_role: 'everyone',
  cooldown_seconds: 5,
  enabled: true,
}

/** Starting points, in the spirit of the classic chat-bot defaults. */
const EXAMPLES: { key: 'hello' | 'clip' | 'dice' | 'shoutout'; command: ChatCommandInput }[] = [
  { key: 'hello', command: { ...BLANK, trigger: '!hello', response: 'Hello {user}, welcome in!' } },
  {
    key: 'clip',
    command: {
      ...BLANK,
      kind: 'keyword',
      trigger: 'clip that',
      response: 'Clipped! {clip}',
      create_clip: true,
      cooldown_seconds: 30,
    },
  },
  { key: 'dice', command: { ...BLANK, trigger: '!dice', response: '{user} rolls a {random 1 6}' } },
  {
    key: 'shoutout',
    command: {
      ...BLANK,
      trigger: '!so',
      response: 'Go check out {touser}: https://twitch.tv/{js touser.toLowerCase()}',
      min_role: 'moderator',
      cooldown_seconds: 10,
    },
  },
]

function inputOf(c: ChatCommand): ChatCommandInput {
  const { kind, trigger, response, create_clip, min_role, cooldown_seconds, enabled } = c
  return { kind, trigger, response, create_clip, min_role, cooldown_seconds, enabled }
}

export default function CommandsPage() {
  return (
    <ProtectedRoute>
      <CommandsContent />
    </ProtectedRoute>
  )
}

function CommandsContent() {
  const t = useTranslations()
  const [commands, setCommands] = useState<ChatCommand[]>([])
  const [status, setStatus] = useState<ChatCommandStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadFailed, setLoadFailed] = useState(false)
  const [editing, setEditing] = useState<{ initial: ChatCommandInput; id?: string } | null>(null)
  const [deleting, setDeleting] = useState<ChatCommand | null>(null)

  const fetchList = useCallback(async () => {
    try {
      const data = await engagementApi.listChatCommands()
      setCommands(data.commands)
      setStatus(data.status)
      setLoadFailed(false)
    } catch {
      setLoadFailed(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void (async () => {
      await fetchList()
    })()
  }, [fetchList])

  function retry() {
    setLoading(true)
    void fetchList()
  }

  function onSaved(saved: ChatCommand) {
    setCommands((list) =>
      list.some((c) => c.id === saved.id)
        ? list.map((c) => (c.id === saved.id ? saved : c))
        : [...list, saved]
    )
    setEditing(null)
    toastManager.add({ title: t('commands.editor.saved'), type: 'success' })
  }

  async function toggle(c: ChatCommand, enabled: boolean) {
    try {
      const saved = await engagementApi.updateChatCommand(c.id, { ...inputOf(c), enabled })
      setCommands((list) => list.map((x) => (x.id === saved.id ? saved : x)))
    } catch {
      toastManager.add({ title: t('commands.list.toggleFailed'), type: 'error' })
    }
  }

  async function confirmDelete() {
    if (!deleting) return
    try {
      await engagementApi.deleteChatCommand(deleting.id)
      setCommands((list) => list.filter((c) => c.id !== deleting.id))
    } catch {
      toastManager.add({ title: t('commands.delete.failed'), type: 'error' })
    } finally {
      setDeleting(null)
    }
  }

  return (
    <div className={cn('lanes-app min-h-screen', archivoBlack.variable, spaceMono.variable)}>
      <AppNav />
      <main id="main-content" tabIndex={-1} className="mx-auto max-w-4xl space-y-6 px-4 py-6">
        <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-2">
            <h1 className="text-2xl">{t('commands.heading')}</h1>
            <p className="max-w-2xl text-sm text-text-sub">{t('commands.intro')}</p>
          </div>
          <Button onClick={() => setEditing({ initial: BLANK })} className="shrink-0">
            <Plus aria-hidden="true" />
            {t('commands.newCommand')}
          </Button>
        </header>

        {status && <StatusPanel status={status} />}

        {loading && (
          <p role="status" className="py-8 text-center text-text-sub">
            {t('commands.loading')}
          </p>
        )}

        {!loading && loadFailed && (
          <div role="alert" className="space-y-3 py-8 text-center">
            <p className="text-sm text-destructive">{t('commands.loadFailed')}</p>
            <Button variant="outline" size="sm" onClick={retry}>
              {t('commands.retry')}
            </Button>
          </div>
        )}

        {!loading && !loadFailed && commands.length === 0 && (
          <Card className="space-y-3 p-5">
            <p className="text-sm text-text-sub">{t('commands.empty')}</p>
          </Card>
        )}

        {!loading && !loadFailed && commands.length > 0 && (
          <ul className="space-y-2">
            {commands.map((c) => (
              <CommandRow
                key={c.id}
                command={c}
                onToggle={(enabled) => void toggle(c, enabled)}
                onEdit={() => setEditing({ initial: inputOf(c), id: c.id })}
                onDelete={() => setDeleting(c)}
              />
            ))}
          </ul>
        )}

        {!loading && !loadFailed && (
          <section aria-labelledby="examples-heading" className="space-y-2">
            <h2 id="examples-heading" className="text-sm font-semibold text-text-sub">
              {t('commands.examplesHeading')}
            </h2>
            <div className="flex flex-wrap gap-2">
              {EXAMPLES.map((ex) => (
                <Button
                  key={ex.key}
                  variant="outline"
                  size="sm"
                  onClick={() => setEditing({ initial: ex.command })}
                >
                  {t(`commands.examples.${ex.key}`)}
                </Button>
              ))}
            </div>
          </section>
        )}
      </main>

      {editing && (
        <CommandEditor
          initial={editing.initial}
          editingId={editing.id}
          onClose={() => setEditing(null)}
          onSaved={onSaved}
        />
      )}

      <AlertDialog.Root
        open={deleting !== null}
        onOpenChange={(open) => !open && setDeleting(null)}
      >
        <AlertDialog.Content>
          <AlertDialog.Title>
            {t('commands.delete.title', { trigger: deleting?.trigger ?? '' })}
          </AlertDialog.Title>
          <AlertDialog.Description>{t('commands.delete.body')}</AlertDialog.Description>
          {/* Cancel first in DOM order: the least destructive action gets initial focus. */}
          <div className="mt-6 flex justify-end gap-2">
            <AlertDialog.Close render={<Button variant="outline" />}>
              {t('commands.delete.cancel')}
            </AlertDialog.Close>
            <Button variant="destructive" onClick={() => void confirmDelete()}>
              {t('commands.delete.confirm')}
            </Button>
          </div>
        </AlertDialog.Content>
      </AlertDialog.Root>
    </div>
  )
}

function CommandRow({
  command: c,
  onToggle,
  onEdit,
  onDelete,
}: {
  command: ChatCommand
  onToggle: (enabled: boolean) => void
  onEdit: () => void
  onDelete: () => void
}) {
  const t = useTranslations()
  return (
    <li>
      <Card
        className={cn(
          'flex flex-col gap-3 p-4 sm:flex-row sm:items-center',
          !c.enabled && 'opacity-60'
        )}
      >
        <div className="min-w-0 flex-1 space-y-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-mono text-base text-text">{c.trigger}</span>
            <Badge size="sm">
              {c.kind === 'command'
                ? t('commands.list.kindCommand')
                : t('commands.list.kindKeyword')}
            </Badge>
            <Badge size="sm">{t(`commands.roles.${c.min_role}`)}</Badge>
            {c.create_clip && (
              <Badge size="sm" className="gap-1">
                <Scissors className="size-3" aria-hidden="true" />
                {t('commands.list.clips')}
              </Badge>
            )}
          </div>
          <p className="truncate font-mono text-xs text-text-sub">
            {c.response || t('commands.list.noReply')}
          </p>
          <p className="flex gap-3 text-xs text-text-dim">
            <span>{t('commands.list.cooldown', { seconds: c.cooldown_seconds })}</span>
            <span>{t('commands.list.uses', { count: c.use_count })}</span>
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Switch.Root
            checked={c.enabled}
            aria-label={t('commands.list.enabledLabel', { trigger: c.trigger })}
            onCheckedChange={(v: boolean) => onToggle(v)}
          >
            <Switch.Thumb />
          </Switch.Root>
          <Button
            size="sm"
            variant="outline"
            aria-label={t('commands.list.editLabel', { trigger: c.trigger })}
            onClick={onEdit}
          >
            {t('commands.list.edit')}
          </Button>
          <Button
            size="sm"
            variant="ghost"
            aria-label={t('commands.list.deleteLabel', { trigger: c.trigger })}
            onClick={onDelete}
          >
            {t('commands.list.delete')}
          </Button>
        </div>
      </Card>
    </li>
  )
}

function StatusPanel({ status }: { status: ChatCommandStatus }) {
  const t = useTranslations()
  const [granting, setGranting] = useState(false)
  const twitchReady = status.twitch_can_send && status.twitch_can_clip

  async function grant() {
    if (!status.consent_overlay_id) return
    setGranting(true)
    try {
      window.location.href = await engagementApi.getChatCommandsConsentUrl(
        status.consent_overlay_id
      )
    } catch {
      setGranting(false)
      toastManager.add({ title: t('commands.status.grantFailed'), type: 'error' })
    }
  }

  return (
    <Card className="space-y-3 p-4">
      <h2 className="text-sm font-semibold text-text">{t('commands.status.heading')}</h2>
      {status.twitch_connected ? (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <ul className="space-y-1 text-sm">
            <Grant label={t('commands.status.twitchReplies')} ok={status.twitch_can_send} />
            <Grant label={t('commands.status.twitchClips')} ok={status.twitch_can_clip} />
          </ul>
          {!twitchReady &&
            (status.consent_overlay_id ? (
              <Button size="sm" variant="outline" disabled={granting} onClick={() => void grant()}>
                {granting ? t('commands.status.granting') : t('commands.status.grant')}
              </Button>
            ) : (
              <p className="text-xs text-text-sub sm:max-w-xs">
                {t('commands.status.grantNeedsOverlay')}
              </p>
            ))}
        </div>
      ) : (
        <p className="text-sm text-text-sub">{t('commands.status.twitchNoLogin')}</p>
      )}
      <div className="flex flex-col gap-1 text-sm sm:flex-row sm:items-center sm:justify-between">
        <p className={status.is_premium ? 'text-text' : 'text-text-sub'}>
          {status.is_premium
            ? t('commands.status.youtubePremium')
            : t('commands.status.youtubeLocked')}
        </p>
        {!status.is_premium && (
          <Link href="/upgrade" className={buttonVariants({ size: 'sm', variant: 'link' })}>
            {t('commands.status.upgrade')}
          </Link>
        )}
      </div>
      <p className="text-xs text-text-dim">{t('commands.status.kick')}</p>
      <p className="text-xs text-text-dim">{t('commands.status.others')}</p>
    </Card>
  )
}

function Grant({ label, ok }: { label: string; ok: boolean }) {
  const t = useTranslations()
  return (
    <li className="flex items-center gap-2">
      {ok ? (
        <Check className="size-4 text-success" aria-hidden="true" />
      ) : (
        <CircleAlert className="size-4 text-warning" aria-hidden="true" />
      )}
      <span className="text-text">{label}</span>
      <span className={ok ? 'text-success' : 'text-warning'}>
        {ok ? t('commands.status.allowed') : t('commands.status.missing')}
      </span>
    </li>
  )
}
