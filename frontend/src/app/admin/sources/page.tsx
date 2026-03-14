'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { PlatformBadge } from '@/components/ui/badge'
import type { Platform } from '@/lib/platform-colors'

interface Source {
  id: string
  overlay_id: string
  overlay_name: string
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  channel_id: string
  channel_name: string
  is_active: boolean
  created_at: string
  user_id: string
}

export default function SourcesPage() {
  const [sources, setSources] = useState<Source[]>([])
  const [filteredSources, setFilteredSources] = useState<Source[]>([])
  const [platformFilter, setPlatformFilter] = useState<string>('all')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [searchTerm, setSearchTerm] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Fetch all sources
  useEffect(() => {
    async function fetchSources() {
      try {
        const token = localStorage.getItem('jwt_token')
        if (!token) {
          setError('Not authenticated')
          setLoading(false)
          return
        }

        const response = await fetch('/api/v1/admin/sources', {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`)
        }

        const data = await response.json()
        setSources(data)
        setFilteredSources(data)
        setLoading(false)
      } catch (err) {
        console.error('Failed to load sources:', err)
        setError('Failed to load sources')
        setLoading(false)
      }
    }

    fetchSources()
  }, [])

  // Filter sources based on platform, status, and search
  useEffect(() => {
    let filtered = [...sources]

    // Platform filter
    if (platformFilter !== 'all') {
      filtered = filtered.filter((s) => s.platform === platformFilter)
    }

    // Status filter
    if (statusFilter === 'active') {
      filtered = filtered.filter((s) => s.is_active)
    } else if (statusFilter === 'inactive') {
      filtered = filtered.filter((s) => !s.is_active)
    }

    // Search filter
    if (searchTerm) {
      const term = searchTerm.toLowerCase()
      filtered = filtered.filter(
        (s) =>
          s.channel_name.toLowerCase().includes(term) ||
          s.channel_id.toLowerCase().includes(term) ||
          s.overlay_name.toLowerCase().includes(term)
      )
    }

    setFilteredSources(filtered)
  }, [platformFilter, statusFilter, searchTerm, sources])

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

      {/* Stats Cards */}
      <div className="mb-6 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card className="p-4">
          <div className="flex items-center gap-3">
            <div className="flex-shrink-0 rounded-lg bg-twitch/20 p-2">
              <svg className="h-5 w-5 text-twitch" fill="currentColor" viewBox="0 0 20 20">
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
              <svg className="h-5 w-5 text-youtube" fill="currentColor" viewBox="0 0 20 20">
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
              <svg className="h-5 w-5 text-kick" fill="currentColor" viewBox="0 0 20 20">
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
              <svg className="h-5 w-5 text-tiktok" fill="currentColor" viewBox="0 0 20 20">
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
            <label className="mb-2 block text-sm font-medium text-text-sub">Search</label>
            <input
              type="text"
              placeholder="Search by channel name or ID..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="focus-visible:ring-ring block w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none sm:text-sm"
            />
          </div>
          <div>
            <label className="mb-2 block text-sm font-medium text-text-sub">Platform</label>
            <select
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
            <label className="mb-2 block text-sm font-medium text-text-sub">Status</label>
            <select
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
      ) : (
        <Card className="overflow-hidden">
          <div className="border-b border-border px-4 py-5">
            <h3 className="text-base font-medium text-text">
              All Sources ({filteredSources.length})
            </h3>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-border bg-surface-2">
                <tr>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Platform</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Channel</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Overlay</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Status</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Created</th>
                  <th className="px-4 py-3 text-left font-medium text-text-sub">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredSources.map((source) => (
                  <tr key={source.id} className="transition-colors hover:bg-surface-2">
                    <td className="px-4 py-3">
                      <PlatformBadge platform={source.platform as Platform} size="sm" />
                    </td>
                    <td className="px-4 py-3">
                      <div className="text-sm font-medium text-text">{source.channel_name}</div>
                      <div className="font-mono text-xs text-text-sub">{source.channel_id}</div>
                    </td>
                    <td className="px-4 py-3">
                      <Link
                        href="/admin/overlays"
                        className="text-sm text-text-sub transition-colors hover:text-text"
                      >
                        {source.overlay_name}
                      </Link>
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
                    <td className="px-4 py-3 text-sm">
                      <Link
                        href="/admin/overlays"
                        className="font-medium text-text-sub transition-colors hover:text-text"
                      >
                        View
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </div>
  )
}
