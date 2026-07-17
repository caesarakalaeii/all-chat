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
 * deep-links each hit into the URL-addressable admin views (ADR-0036). Users,
 * overlays, and sources are federated on the client over the existing admin
 * list endpoints; viewers use the server-side ?q= search.
 */

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { Card } from '@/components/ui/card'
import { PlatformBadge } from '@/components/ui/badge'
import { UserAvatar } from '@/components/UserAvatar'

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
interface ViewerRow {
  id: string
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

function readInitialQuery(): string {
  if (typeof window === 'undefined') return ''
  return new URLSearchParams(window.location.search).get('q') ?? ''
}

export default function AdminSearchPage() {
  // Seed both from ?q= via lazy initializers (e.g. arriving from a deep link) so
  // results appear immediately without a synchronous setState in an effect.
  const [query, setQuery] = useState(readInitialQuery)
  const [debounced, setDebounced] = useState(() => readInitialQuery().trim())
  const [users, setUsers] = useState<UserRow[]>([])
  const [overlays, setOverlays] = useState<OverlayRow[]>([])
  const [sources, setSources] = useState<SourceRow[]>([])
  const [viewers, setViewers] = useState<ViewerRow[]>([])
  const [loading, setLoading] = useState(false)

  // Debounce the query.
  useEffect(() => {
    const t = setTimeout(() => setDebounced(query.trim()), 300)
    return () => clearTimeout(t)
  }, [query])

  useEffect(() => {
    const term = debounced.toLowerCase()
    if (!term) return // render shows the prompt; keep prior results untouched
    let cancelled = false

    async function run() {
      setLoading(true)
      try {
        const [allUsers, allOverlays, allSources, viewerResp] = await Promise.all([
          getJson<UserRow[]>('/api/v1/admin/users'),
          getJson<OverlayRow[]>('/api/v1/admin/overlays'),
          getJson<SourceRow[]>('/api/v1/admin/sources'),
          getJson<{ viewers: ViewerRow[] }>(
            `/api/v1/admin/viewers?limit=${GROUP_LIMIT}&q=${encodeURIComponent(debounced)}`
          ),
        ])
        if (cancelled) return
        setUsers(
          allUsers.filter(
            (u) =>
              u.username.toLowerCase().includes(term) ||
              u.display_name.toLowerCase().includes(term) ||
              u.id.toLowerCase().includes(term)
          )
        )
        setOverlays(
          allOverlays.filter(
            (o) =>
              o.name.toLowerCase().includes(term) ||
              o.id.toLowerCase().includes(term) ||
              (o.owner_username?.toLowerCase().includes(term) ?? false)
          )
        )
        setSources(
          allSources.filter(
            (s) =>
              s.channel_name.toLowerCase().includes(term) ||
              s.channel_id.toLowerCase().includes(term) ||
              (s.channel_handle?.toLowerCase().includes(term) ?? false) ||
              (s.owner_username?.toLowerCase().includes(term) ?? false)
          )
        )
        setViewers(viewerResp.viewers ?? [])
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
        <h1 className="text-2xl font-bold text-text">Search</h1>
        <p className="mt-1 text-sm text-text-sub">
          Find any user, overlay, source, or viewer and jump straight to it
        </p>
      </div>

      <input
        type="search"
        placeholder="Search users, overlays, sources, viewers..."
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        aria-label="Global admin search"
        className="focus-visible:ring-ring mb-6 w-full rounded-lg border border-border bg-surface-2 px-4 py-3 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none"
      />

      {!debounced ? (
        <Card className="p-8 text-center text-sm text-text-dim">
          Type at least one character to search.
        </Card>
      ) : loading && !hasResults ? (
        <Card className="p-8 text-center text-sm text-text-dim">Searching...</Card>
      ) : !hasResults ? (
        <Card className="p-8 text-center text-sm text-text-dim">
          Nothing matches &ldquo;{debounced}&rdquo;.
        </Card>
      ) : (
        <div className="space-y-6">
          {users.length > 0 && (
            <ResultGroup title="Users" count={users.length}>
              {users.slice(0, GROUP_LIMIT).map((u) => (
                <Link
                  key={u.id}
                  href={`/admin/users?user=${u.id}`}
                  className="flex items-center gap-3 rounded-lg border border-border p-3 transition-colors hover:bg-surface-2"
                >
                  <UserAvatar avatarUrl={u.profile_image_url} displayName={u.display_name} size={32} />
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-text">{u.display_name}</div>
                    <div className="truncate text-xs text-text-sub">@{u.username}</div>
                  </div>
                  <div className="ml-auto flex gap-1">
                    {u.is_premium && (
                      <span className="rounded bg-amber-400/10 px-2 py-0.5 text-xs font-medium text-amber-400">
                        Premium
                      </span>
                    )}
                    {u.is_banned && (
                      <span className="bg-destructive/10 text-destructive rounded px-2 py-0.5 text-xs font-medium">
                        Banned
                      </span>
                    )}
                  </div>
                </Link>
              ))}
            </ResultGroup>
          )}

          {overlays.length > 0 && (
            <ResultGroup title="Overlays" count={overlays.length}>
              {overlays.slice(0, GROUP_LIMIT).map((o) => (
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
                  <span className="shrink-0 text-xs text-text-dim">{o.sources_count ?? 0} sources</span>
                </Link>
              ))}
            </ResultGroup>
          )}

          {sources.length > 0 && (
            <ResultGroup title="Sources" count={sources.length}>
              {sources.slice(0, GROUP_LIMIT).map((s) => (
                <Link
                  key={s.id}
                  href={`/admin/overlays?overlay=${s.overlay_id}`}
                  className="flex items-center gap-3 rounded-lg border border-border p-3 transition-colors hover:bg-surface-2"
                >
                  <PlatformBadge platform={s.platform} size="sm" />
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-text">{s.channel_name}</div>
                    <div className="truncate text-xs text-text-sub">
                      in {s.overlay_name}
                      {s.owner_username ? ` · @${s.owner_username}` : ''}
                    </div>
                  </div>
                </Link>
              ))}
            </ResultGroup>
          )}

          {viewers.length > 0 && (
            <ResultGroup title="Viewers" count={viewers.length}>
              {viewers.slice(0, GROUP_LIMIT).map((v) => (
                <Link
                  key={v.id}
                  href={`/admin/viewers?q=${encodeURIComponent(v.username)}`}
                  className="flex items-center gap-3 rounded-lg border border-border p-3 transition-colors hover:bg-surface-2"
                >
                  <PlatformBadge platform={v.platform} size="sm" />
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-text">{v.display_name}</div>
                    <div className="truncate text-xs text-text-sub">@{v.username}</div>
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
  return (
    <section>
      <h2 className="mb-2 text-sm font-semibold text-text-sub">
        {title} {count > GROUP_LIMIT ? `(showing ${GROUP_LIMIT} of ${count})` : `(${count})`}
      </h2>
      <div className="space-y-2">{children}</div>
    </section>
  )
}
