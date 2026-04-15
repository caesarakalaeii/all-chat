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


import { useEffect, useState } from 'react'
import Link from 'next/link'
import clsx from 'clsx'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { PlatformBadge } from '@/components/ui/badge'
import type { Platform } from '@/lib/platform-colors'

interface Overlay {
  id: string
  name: string
  user_id: string
  created_at: string
  updated_at: string
  sources_count?: number
}

interface OverlaySource {
  id: string
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok'
  channel_id: string
  channel_name: string
  is_active: boolean
  created_at: string
}

export default function OverlaysPage() {
  const [overlays, setOverlays] = useState<Overlay[]>([])
  const [selectedOverlay, setSelectedOverlay] = useState<Overlay | null>(null)
  const [sources, setSources] = useState<OverlaySource[]>([])
  const [activeOverlayIds, setActiveOverlayIds] = useState<Set<string>>(new Set())
  const [loading, setLoading] = useState(true)
  const [sourcesLoading, setSourcesLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searchTerm, setSearchTerm] = useState('')
  const [showConnectedOnly, setShowConnectedOnly] = useState(false)

  // Fetch all overlays
  useEffect(() => {
    async function fetchOverlays() {
      try {
        const token = localStorage.getItem('jwt_token')
        if (!token) {
          setError('Not authenticated')
          setLoading(false)
          return
        }

        const response = await fetch('/api/v1/admin/overlays', {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`)
        }

        const data = await response.json()
        setOverlays(data)
        setLoading(false)
      } catch (err) {
        console.error('Failed to load overlays:', err)
        setError('Failed to load overlays')
        setLoading(false)
      }
    }

    fetchOverlays()

    async function fetchActiveOverlays() {
      try {
        const token = localStorage.getItem('jwt_token')
        if (!token) return
        const response = await fetch('/api/v1/admin/overlays/active', {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (response.ok) {
          const ids: string[] = await response.json()
          setActiveOverlayIds(new Set(ids))
        }
      } catch (err) {
        console.error('Failed to load active overlays:', err)
      }
    }

    fetchActiveOverlays()
    const interval = setInterval(fetchActiveOverlays, 30000)
    return () => clearInterval(interval)
  }, [])

  // Fetch sources for selected overlay
  useEffect(() => {
    async function fetchSources() {
      if (!selectedOverlay) {
        setSources([])
        return
      }

      setSourcesLoading(true)
      try {
        const token = localStorage.getItem('jwt_token')
        const response = await fetch(`/api/v1/admin/overlays/${selectedOverlay.id}/sources`, {
          headers: {
            Authorization: `Bearer ${token}`,
          },
        })

        if (response.ok) {
          const data = await response.json()
          setSources(data)
        } else {
          console.error('Failed to fetch sources:', response.statusText)
          setSources([])
        }
      } catch (err) {
        console.error('Failed to fetch sources:', err)
        setSources([])
      } finally {
        setSourcesLoading(false)
      }
    }

    fetchSources()
  }, [selectedOverlay])

  // Filter overlays by search term and connected status
  const connectedCount = overlays.filter((o) => activeOverlayIds.has(o.id)).length
  const filteredOverlays = overlays.filter((o) => {
    if (showConnectedOnly && !activeOverlayIds.has(o.id)) return false
    if (!searchTerm) return true
    const term = searchTerm.toLowerCase()
    return (
      o.name.toLowerCase().includes(term) ||
      o.id.toLowerCase().includes(term) ||
      o.user_id.toLowerCase().includes(term)
    )
  })

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
        <h1 className="text-2xl font-bold text-text">Overlays</h1>
        <p className="mt-1 text-sm text-text-sub">
          Manage overlays and their connected chat sources
        </p>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Overlays List */}
        <div className="lg:col-span-2">
          {loading ? (
            <Card className="space-y-3 p-6">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full rounded-lg" />
              ))}
            </Card>
          ) : (
            <Card className="overflow-hidden">
              <div className="border-b border-border px-4 py-5">
                <h3 className="text-base font-medium text-text">
                  All Overlays ({overlays.length})
                </h3>

                {/* Search Input */}
                <div className="mt-4 flex items-center gap-3">
                  <input
                    type="text"
                    placeholder="Search by overlay name, ID, or user ID..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="focus-visible:ring-ring flex-1 rounded-lg border border-border bg-surface-2 px-4 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none"
                  />
                  <button
                    onClick={() => setShowConnectedOnly(!showConnectedOnly)}
                    className={clsx(
                      'flex shrink-0 items-center gap-1.5 rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                      showConnectedOnly
                        ? 'border-kick/30 bg-kick/10 text-kick'
                        : 'border-border bg-surface-2 text-text-sub hover:text-text'
                    )}
                  >
                    <span className="inline-block h-2 w-2 rounded-full bg-kick" />
                    Connected ({connectedCount})
                  </button>
                </div>
              </div>
              <ul className="divide-y divide-border">
                {filteredOverlays.map((overlay) => (
                  <li
                    key={overlay.id}
                    className={clsx(
                      'cursor-pointer px-4 py-4 transition-colors hover:bg-surface-2',
                      selectedOverlay?.id === overlay.id && 'bg-surface-2'
                    )}
                    onClick={() => setSelectedOverlay(overlay)}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        <div className="flex items-center">
                          {activeOverlayIds.has(overlay.id) && (
                            <span className="mr-1.5 inline-block h-2 w-2 shrink-0 rounded-full bg-kick" title="Connected" />
                          )}
                          <p className="text-sm font-medium text-text">{overlay.name}</p>
                          <span className="ml-2 inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-text-sub">
                            {overlay.sources_count || 0} sources
                          </span>
                        </div>
                        <p className="mt-1 font-mono text-xs text-text-sub">ID: {overlay.id}</p>
                        <p className="mt-1 text-xs text-text-dim">
                          Created {new Date(overlay.created_at).toLocaleDateString()}
                        </p>
                      </div>
                      <div className="flex items-center space-x-2">
                        <Link
                          href={`/overlay/${overlay.id}`}
                          target="_blank"
                          className="text-sm text-text-sub transition-colors hover:text-text"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <svg
                            className="h-5 w-5"
                            fill="none"
                            stroke="currentColor"
                            viewBox="0 0 24 24"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth="2"
                              d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                            />
                          </svg>
                        </Link>
                        <svg
                          className="h-5 w-5 text-text-dim"
                          fill="none"
                          stroke="currentColor"
                          viewBox="0 0 24 24"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="2"
                            d="M9 5l7 7-7 7"
                          />
                        </svg>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            </Card>
          )}
        </div>

        {/* Overlay Details & Sources */}
        <div className="lg:col-span-1">
          {selectedOverlay ? (
            <div className="space-y-4">
              {/* Overlay Details */}
              <Card className="overflow-hidden">
                <div className="border-b border-border px-4 py-5">
                  <h3 className="text-base font-medium text-text">Overlay Details</h3>
                </div>
                <div className="px-4 py-5">
                  <dl className="space-y-4">
                    <div>
                      <dt className="text-sm font-medium text-text-sub">Name</dt>
                      <dd className="mt-1 text-sm text-text">{selectedOverlay.name}</dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-text-sub">ID</dt>
                      <dd className="mt-1 font-mono text-xs break-all text-text">
                        {selectedOverlay.id}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-text-sub">User ID</dt>
                      <dd className="mt-1 font-mono text-xs break-all text-text">
                        {selectedOverlay.user_id}
                      </dd>
                    </div>
                  </dl>
                </div>
              </Card>

              {/* Sources */}
              <Card className="overflow-hidden">
                <div className="border-b border-border px-4 py-5">
                  <h3 className="text-base font-medium text-text">
                    Connected Sources ({sources.length})
                  </h3>
                </div>
                <div className="px-4 py-5">
                  {sourcesLoading ? (
                    <div className="space-y-2">
                      <Skeleton className="h-10 w-full rounded-lg" />
                      <Skeleton className="h-10 w-full rounded-lg" />
                    </div>
                  ) : sources.length > 0 ? (
                    <ul className="space-y-3">
                      {sources.map((source) => (
                        <li key={source.id} className="rounded-lg border border-border p-3">
                          <div className="flex items-start justify-between">
                            <div className="flex-1">
                              <div className="flex items-center space-x-2">
                                <PlatformBadge platform={source.platform as Platform} size="sm" />
                                {source.is_active ? (
                                  <span className="inline-flex items-center rounded bg-kick/10 px-2 py-0.5 text-xs font-medium text-kick">
                                    Active
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-text-dim">
                                    Inactive
                                  </span>
                                )}
                              </div>
                              <p className="mt-1 text-sm font-medium text-text">
                                {source.channel_name}
                              </p>
                              <p className="mt-1 font-mono text-xs text-text-sub">
                                {source.channel_id}
                              </p>
                              <p className="mt-1 text-xs text-text-dim">
                                Added {new Date(source.created_at).toLocaleDateString()}
                              </p>
                            </div>
                          </div>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="text-sm text-text-dim italic">No sources connected</p>
                  )}
                </div>
              </Card>
            </div>
          ) : (
            <Card className="p-6 text-center">
              <svg
                className="mx-auto h-12 w-12 text-text-dim"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                />
              </svg>
              <p className="mt-2 text-sm text-text-sub">Select an overlay to view details</p>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}
