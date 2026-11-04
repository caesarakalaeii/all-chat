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
 * Global admin entity search (ADR-0035).
 *
 * Resolves a free-text query across users, overlays, sources, and viewers and
 * deep-links each hit into the URL-addressable admin views (ADR-0036). Each
 * group asks its admin list endpoint for the first few server-side ?q= matches
 * and their total (ADR-0086); no full list is loaded.
 */

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { Card } from '@/components/ui/card'
import { PlatformBadge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { UserAvatar } from '@/components/UserAvatar'
import { useDebounce } from '@/hooks/useDebounce'
import { useTranslations } from '@/lib/i18n'
import { readQueryParam } from '@/lib/utils'

const GROUP_LIMIT = 8

interface UserRow {
  id: string
  username: string
  display_name: string
  profile_image_url: string
  is_premium: boolean
  is_banned: boolean
}
interface OverlayRow {
  id: string
  name: string
  user_id: string
  owner_username?: string
  sources_count?: number
}
interface SourceRow {
  id: string
  overlay_id: string
  overlay_name: string
  platform: string
  channel_id: string
  channel_name: string
  channel_handle?: string | null
  owner_username?: string
  user_id: string
}
// One platform account; an identity that never signed in has no username (ADR-0034).
interface ViewerRow {
  platform: string
  username: string
  display_name: string
  platform_user_id: string
}

async function getJson<T>(url: string): Promise<T> {
  const res = await fetch(url, { credentials: 'same-origin' })
  if (!res.ok) throw new Error(`HTTP ${res.status}`)
  return res.json()
}

export default function AdminSearchPage() {
  const t = useTranslations()
  // Seeded from ?q= (e.g. arriving from a deep link) via a lazy initializer, so
  // results appear immediately without a synchronous setState in an effect.
  const [query, setQuery] = useState(() => readQueryParam('q'))
  const debounced = useDebounce(query.trim(), 300)
  const [users, setUsers] = useState<UserRow[]>([])
  const [overlays, setOverlays] = useState<OverlayRow[]>([])
  const [sources, setSources] = useState<SourceRow[]>([])
  const [viewers, setViewers] = useState<ViewerRow[]>([])
  // Matches per group, of which only the first GROUP_LIMIT are fetched.
  const [totals, setTotals] = useState({ users: 0, overlays: 0, sources: 0, viewers: 0 })
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!debounced) return // render shows the prompt; keep prior results untouched
    let cancelled = false

    async function run() {
      setLoading(true)
      const page = `limit=${GROUP_LIMIT}&q=${encodeURIComponent(debounced)}`
      try {
        const [u, o, s, v] = await Promise.all([
          getJson<{ users: UserRow[]; total: number }>(`/api/v1/admin/users?${page}`),
          getJson<{ overlays: OverlayRow[]; total: number }>(`/api/v1/admin/overlays?${page}`),
          getJson<{ sources: SourceRow[]; total: number }>(`/api/v1/admin/sources?${page}`),
          getJson<{ viewers: ViewerRow[]; total: number }>(`/api/v1/admin/viewers?${page}`),
        ])
        if (cancelled) return
        setUsers(u.users ?? [])
        setOverlays(o.overlays ?? [])
        setSources(s.sources ?? [])
        setViewers(v.viewers ?? [])
        setTotals({ users: u.total, overlays: o.total, sources: s.total, viewers: v.total })
      } catch (err) {
        if (!cancelled) console.error('Global search failed:', err)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    run()
    return () => {
      cancelled = true
    }
  }, [debounced])

  const hasResults =
    users.length > 0 || overlays.length > 0 || sources.length > 0 || viewers.length > 0

  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">{t('admin.search.heading')}</h1>
        <p className="mt-1 text-sm text-text-sub">{t('admin.search.intro')}</p>
      </div>

      <Input
        type="search"
        placeholder={t('admin.search.inputPlaceholder')}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        aria-label={t('admin.search.inputLabel')}
        className="mb-6"
      />

      {!debounced ? (
        <Card className="p-8 text-center text-sm text-text-dim">
          {t('admin.search.promptState')}
        </Card>
      ) : loading && !hasResults ? (
        <Card className="p-8 text-center text-sm text-text-dim">
          {t('admin.search.loadingState')}
        </Card>
      ) : !hasResults ? (
        <Card className="p-8 text-center text-sm text-text-dim">
          {t('admin.search.emptyState', { query: debounced })}
        </Card>
      ) : (
        <div className="space-y-6">
          {users.length > 0 && (
            <ResultGroup title={t('admin.search.groupUsers')} count={totals.users}>
              {users.map((u) => (
                <Link
                  key={u.id}
                  href={`/admin/users?user=${u.id}`}
                  className="flex items-center gap-3 rounded-lg border border-border p-3 transition-colors hover:bg-surface-2"
                >
                  <UserAvatar
                    avatarUrl={u.profile_image_url}
                    displayName={u.display_name}
                    size={32}
                  />
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-text">{u.display_name}</div>
                    <div className="truncate text-xs text-text-sub">@{u.username}</div>
                  </div>
                  <div className="ml-auto flex gap-1">
                    {u.is_premium && (
                      <span className="rounded bg-premium/10 px-2 py-0.5 text-xs font-medium text-premium">
                        {t('admin.search.badgePremium')}
                      </span>
                    )}
                    {u.is_banned && (
                      <span className="rounded bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">
                        {t('admin.search.badgeBanned')}
                      </span>
                    )}
                  </div>
                </Link>
              ))}
            </ResultGroup>
          )}

          {overlays.length > 0 && (
            <ResultGroup title={t('admin.search.groupOverlays')} count={totals.overlays}>
              {overlays.map((o) => (
                <Link
                  key={o.id}
                  href={`/admin/overlays?overlay=${o.id}`}
                  className="flex items-center justify-between gap-3 rounded-lg border border-border p-3 transition-colors hover:bg-surface-2"
                >
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-text">{o.name}</div>
                    <div className="truncate text-xs text-text-sub">
                      {o.owner_username ? `@${o.owner_username}` : o.user_id.slice(0, 8)}
                    </div>
                  </div>
                  <span className="shrink-0 text-xs text-text-dim">
                    {t('admin.search.overlaySourceCount', { count: o.sources_count ?? 0 })}
                  </span>
                </Link>
              ))}
            </ResultGroup>
          )}

          {sources.length > 0 && (
            <ResultGroup title={t('admin.search.groupSources')} count={totals.sources}>
              {sources.map((s) => (
                <Link
                  key={s.id}
                  href={`/admin/overlays?overlay=${s.overlay_id}`}
                  className="flex items-center gap-3 rounded-lg border border-border p-3 transition-colors hover:bg-surface-2"
                >
                  <PlatformBadge platform={s.platform} size="sm" />
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-text">{s.channel_name}</div>
                    <div className="truncate text-xs text-text-sub">
                      {t('admin.search.sourceInOverlay', { overlay: s.overlay_name })}
                      {s.owner_username ? ` · @${s.owner_username}` : ''}
                    </div>
                  </div>
                </Link>
              ))}
            </ResultGroup>
          )}

          {viewers.length > 0 && (
            <ResultGroup title={t('admin.search.groupViewers')} count={totals.viewers}>
              {viewers.map((v) => (
                <Link
                  key={`${v.platform}:${v.platform_user_id}`}
                  href={`/admin/viewers?q=${encodeURIComponent(v.username || v.platform_user_id)}`}
                  className="flex items-center gap-3 rounded-lg border border-border p-3 transition-colors hover:bg-surface-2"
                >
                  <PlatformBadge platform={v.platform} size="sm" />
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-text">
                      {v.display_name || v.platform_user_id}
                    </div>
                    {v.username && (
                      <div className="truncate text-xs text-text-sub">@{v.username}</div>
                    )}
                  </div>
                </Link>
              ))}
            </ResultGroup>
          )}
        </div>
      )}
    </div>
  )
}

function ResultGroup({
  title,
  count,
  children,
}: {
  title: string
  count: number
  children: React.ReactNode
}) {
  const t = useTranslations()
  return (
    <section>
      <h2 className="mb-2 text-sm font-semibold text-text-sub">
        {title}{' '}
        {count > GROUP_LIMIT
          ? t('admin.search.groupCountTruncated', { shown: GROUP_LIMIT, total: count })
          : t('admin.search.groupCountExact', { total: count })}
      </h2>
      <div className="space-y-2">{children}</div>
    </section>
  )
}
