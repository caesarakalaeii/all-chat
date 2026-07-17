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

import { useEffect, useId, useState } from 'react'
import Link from 'next/link'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { PlatformBadge } from '@/components/ui/badge'
import { ChannelLink } from '@/components/ChannelLink'

interface Source {
  id: string
  overlay_id: string
  overlay_name: string
  // Stored platforms outgrow the chromatic set (discord, shared_overlay); the
  // badge neutral-styles anything it doesn't recognize, so no cast is needed.
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'discord' | 'shared_overlay'
  channel_id: string
  channel_name: string
  channel_handle?: string | null
  is_active: boolean
  created_at: string
  user_id: string
  owner_username?: string
  owner_display_name?: string
}

// Short, stable label for a source's owner: the resolved username, else a
// truncated user id (orphaned/unjoined rows), else a dash.
function ownerLabel(source: Source): string {
  if (source.owner_username) return `@${source.owner_username}`
  if (source.user_id) return source.user_id.slice(0, 8)
  return '—'
}

export default function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([])
  const [platformFilter, setPlatformFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [searchTerm, setSearchTerm] = useState('')
  // Optional owner scope from ?user=<id> (set when arriving from the Users page
  // "View this user's sources" link). Narrows the list to one owner.
  const [userFilter, setUserFilter] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const searchId = useId()
  const platformFilterId = useId()
  const statusFilterId = useId()

  // Fetch all sources
  useEffect(() => {
    async function fetchSources() {
      try {
        // Auth is via the httpOnly session cookie (same-origin); the gateway
        // CookieToBearer middleware copies the access cookie to Authorization
        // before backend validation (no JS-readable token).
        const response = await fetch('/api/v1/admin/sources', {
          credentials: 'same-origin',
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`)
        }

        const data = await response.json()
        setSources(data)
        // Scope from URL (client-only; read post-await): ?user=<id> owner scope
        // and ?platform=<p> (e.g. from the dashboard's per-platform breakdown).
        const params = new URLSearchParams(window.location.search)
        setUserFilter(params.get('user'))
        const p = params.get('platform')
        if (p && ['twitch', 'youtube', 'kick', 'tiktok', 'discord', 'shared_overlay'].includes(p)) {
          setPlatformFilter(p)
        }
        setLoading(false)
      } catch (err) {
        console.error('Failed to load sources:', err)
        setError('Failed to load sources')
        setLoading(false)
      }
    }

    fetchSources()
  }, [])

  // Derived during render (no state-in-effect): filter by owner scope, platform,
  // status, and search (channel name/id, overlay name, owner username).
  const searchLower = searchTerm.trim().toLowerCase()
  const filteredSources = sources.filter((s) => {
    if (userFilter && s.user_id !== userFilter) return false
    if (platformFilter !== 'all' && s.platform !== platformFilter) return false
    if (statusFilter === 'active' && !s.is_active) return false
    if (statusFilter === 'inactive' && s.is_active) return false
    if (searchLower) {
      return (
        s.channel_name.toLowerCase().includes(searchLower) ||
        s.channel_id.toLowerCase().includes(searchLower) ||
        s.overlay_name.toLowerCase().includes(searchLower) ||
        (s.owner_username?.toLowerCase().includes(searchLower) ?? false)
      )
    }
    return true
  })

  // Label the active owner scope with a resolved username when we have one.
  const userFilterLabel = userFilter
    ? (sources.find((s) => s.user_id === userFilter)?.owner_username ?? userFilter.slice(0, 8))
    : null

  const platformCounts = {
    twitch: sources.filter((s) => s.platform === 'twitch').length,
    youtube: sources.filter((s) => s.platform === 'youtube').length,
    kick: sources.filter((s) => s.platform === 'kick').length,
    tiktok: sources.filter((s) => s.platform === 'tiktok').length,
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8">
        <Card className="border-destructive p-4">
          <p className="text-destructive">{error}</p>
        </Card>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-text">Sources</h1>
        <p className="mt-1 text-sm text-text-sub">
          View and manage all chat sources across overlays
        </p>
      </div>

      {/* Owner scope (from ?user=) */}
      {userFilter && (
        <div className="mb-4 flex items-center gap-3 rounded-lg border border-border bg-surface-2 px-4 py-2 text-sm">
          <span className="text-text-sub">
            Showing sources owned by{' '}
            <Link
              href={`/admin/users?user=${userFilter}`}
              className="font-medium text-text hover:underline"
            >
              {userFilterLabel}
            </Link>
          </span>
          <button
            type="button"
            onClick={() => setUserFilter(null)}
            className="ml-auto text-text-sub transition-colors hover:text-text"
          >
            Clear
          </button>
        </div>
      )}

      {/* Stats Cards */}
      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="flex-shrink-0 rounded-lg bg-twitch/20 p-2">
              <svg
                aria-hidden="true"
                className="h-5 w-5 text-twitch"
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path d="M2 10a8 8 0 018-8v8h8a8 8 0 11-16 0z" />
                <path d="M12 2.252A8.014 8.014 0 0117.748 8H12V2.252z" />
              </svg>
            </div>
            <div>
              <div className="text-xs text-text-sub">Twitch</div>
              <div className="text-lg font-semibold text-text">{platformCounts.twitch}</div>
            </div>
          </div>
        </Card>

        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="flex-shrink-0 rounded-lg bg-youtube/20 p-2">
              <svg
                aria-hidden="true"
                className="h-5 w-5 text-youtube"
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path d="M2 6a2 2 0 012-2h6a2 2 0 012 2v8a2 2 0 01-2 2H4a2 2 0 01-2-2V6zM14.553 7.106A1 1 0 0014 8v4a1 1 0 00.553.894l2 1A1 1 0 0018 13V7a1 1 0 00-1.447-.894l-2 1z" />
              </svg>
            </div>
            <div>
              <div className="text-xs text-text-sub">YouTube</div>
              <div className="text-lg font-semibold text-text">{platformCounts.youtube}</div>
            </div>
          </div>
        </Card>

        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="flex-shrink-0 rounded-lg bg-kick/20 p-2">
              <svg
                aria-hidden="true"
                className="h-5 w-5 text-kick"
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path
                  fillRule="evenodd"
                  d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z"
                  clipRule="evenodd"
                />
              </svg>
            </div>
            <div>
              <div className="text-xs text-text-sub">Kick</div>
              <div className="text-lg font-semibold text-text">{platformCounts.kick}</div>
            </div>
          </div>
        </Card>

        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="flex-shrink-0 rounded-lg bg-tiktok/20 p-2">
              <svg
                aria-hidden="true"
                className="h-5 w-5 text-tiktok"
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path d="M9 6a3 3 0 11-6 0 3 3 0 016 0zM17 6a3 3 0 11-6 0 3 3 0 016 0zM12.93 17c.046-.327.07-.66.07-1a6.97 6.97 0 00-1.5-4.33A5 5 0 0119 16v1h-6.07zM6 11a5 5 0 015 5v1H1v-1a5 5 0 015-5z" />
              </svg>
            </div>
            <div>
              <div className="text-xs text-text-sub">TikTok</div>
              <div className="text-lg font-semibold text-text">{platformCounts.tiktok}</div>
            </div>
          </div>
        </Card>
      </div>

      {/* Filters */}
      <Card className="mb-6 p-4">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
          <div>
            <label htmlFor={searchId} className="mb-2 block text-sm font-medium text-text-sub">
              Search
            </label>
            <input
              id={searchId}
              type="text"
              placeholder="Search by channel, overlay, or owner..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
            />
          </div>
          <div>
            <label
              htmlFor={platformFilterId}
              className="mb-2 block text-sm font-medium text-text-sub"
            >
              Platform
            </label>
            <select
              id={platformFilterId}
              value={platformFilter}
              onChange={(e) => setPlatformFilter(e.target.value)}
              className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
            >
              <option value="all">All Platforms</option>
              <option value="twitch">Twitch</option>
              <option value="youtube">YouTube</option>
              <option value="kick">Kick</option>
              <option value="tiktok">TikTok</option>
            </select>
          </div>
          <div>
            <label
              htmlFor={statusFilterId}
              className="mb-2 block text-sm font-medium text-text-sub"
            >
              Status
            </label>
            <select
              id={statusFilterId}
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
            >
              <option value="all">All Status</option>
              <option value="active">Active</option>
              <option value="inactive">Inactive</option>
            </select>
          </div>
        </div>
      </Card>

      {/* Sources Table */}
      {loading ? (
        <Card className="space-y-3 p-6">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-lg" />
          ))}
        </Card>
      ) : filteredSources.length === 0 ? (
        <Card className="p-8 text-center text-sm text-text-dim">
          {sources.length === 0 ? 'No sources found.' : 'No sources match your filters.'}
        </Card>
      ) : (
        <>
          {/* Desktop table */}
          <Card className="hidden overflow-hidden md:block">
            <div className="border-b border-border px-4 py-5">
              <h3 className="text-base font-medium text-text">
                All Sources ({filteredSources.length})
              </h3>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <caption className="sr-only">Chat sources</caption>
                <thead className="border-b border-border bg-surface-2">
                  <tr>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Platform
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Channel
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Overlay
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Owner
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Status
                    </th>
                    <th scope="col" className="px-4 py-3 text-left font-medium text-text-sub">
                      Created
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {filteredSources.map((source) => (
                    <tr key={source.id} className="transition-colors hover:bg-surface-2">
                      <td className="px-4 py-3">
                        <PlatformBadge platform={source.platform} size="sm" />
                      </td>
                      <td className="px-4 py-3">
                        <ChannelLink
                          platform={source.platform}
                          channelId={source.channel_id}
                          channelName={source.channel_name}
                          channelHandle={source.channel_handle}
                          className="text-sm font-medium text-text"
                        />
                        <div className="font-mono text-xs text-text-sub">{source.channel_id}</div>
                      </td>
                      <td className="px-4 py-3">
                        <Link
                          href={`/admin/overlays?overlay=${source.overlay_id}`}
                          className="text-sm text-text-sub transition-colors hover:text-text"
                        >
                          {source.overlay_name}
                        </Link>
                      </td>
                      <td className="px-4 py-3">
                        {source.user_id ? (
                          <Link
                            href={`/admin/users?user=${source.user_id}`}
                            className="text-sm text-text-sub transition-colors hover:text-text"
                          >
                            {ownerLabel(source)}
                          </Link>
                        ) : (
                          <span className="text-sm text-text-dim">—</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        {source.is_active ? (
                          <span className="inline-flex items-center rounded-full bg-kick/10 px-2 py-0.5 text-xs font-medium text-kick">
                            Active
                          </span>
                        ) : (
                          <span className="inline-flex items-center rounded-full bg-badge-bg px-2 py-0.5 text-xs font-medium text-text-dim">
                            Inactive
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-sm text-text-sub">
                        {new Date(source.created_at).toLocaleDateString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>

          {/* Mobile card list */}
          <div className="space-y-3 md:hidden">
            <h3 className="text-sm font-medium text-text-sub">
              All Sources ({filteredSources.length})
            </h3>
            {filteredSources.map((source) => (
              <Card key={source.id} className="p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <ChannelLink
                      platform={source.platform}
                      channelId={source.channel_id}
                      channelName={source.channel_name}
                      channelHandle={source.channel_handle}
                      className="truncate text-sm font-medium text-text"
                    />
                    <div className="truncate font-mono text-xs text-text-sub">
                      {source.channel_id}
                    </div>
                  </div>
                  <PlatformBadge platform={source.platform} size="sm" />
                </div>
                <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-sub">
                  <Link
                    href={`/admin/overlays?overlay=${source.overlay_id}`}
                    className="truncate transition-colors hover:text-text"
                  >
                    {source.overlay_name}
                  </Link>
                  {source.user_id && (
                    <Link
                      href={`/admin/users?user=${source.user_id}`}
                      className="truncate transition-colors hover:text-text"
                    >
                      {ownerLabel(source)}
                    </Link>
                  )}
                  <span>{new Date(source.created_at).toLocaleDateString()}</span>
                  {source.is_active ? (
                    <span className="inline-flex items-center rounded-full bg-kick/10 px-2 py-0.5 font-medium text-kick">
                      Active
                    </span>
                  ) : (
                    <span className="inline-flex items-center rounded-full bg-badge-bg px-2 py-0.5 font-medium text-text-dim">
                      Inactive
                    </span>
                  )}
                </div>
              </Card>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
