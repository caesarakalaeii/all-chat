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

import { useEffect, useRef, useState } from 'react'
import Link from 'next/link'
import clsx from 'clsx'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { PlatformBadge } from '@/components/ui/badge'
import { ChannelLink } from '@/components/ChannelLink'
import { formatConnectedFor } from '@/lib/utils'

// Connections open longer than this are highlighted so admins can spot overlays
// that are likely open but no longer live ("dead but open").
const LONG_OPEN_MS = 12 * 60 * 60 * 1000

interface Overlay {
  id: string
  name: string
  user_id: string
  owner_username?: string
  owner_display_name?: string
  created_at: string
  updated_at: string
  sources_count?: number
}

// Shape of GET /api/v1/admin/overlays/active: overlays with a live WebSocket
// connection, each with when that connection began (null while unavailable).
interface ActiveOverlay {
  overlay_id: string
  connected_since?: string | null
}

interface OverlaySource {
  id: string
  // Matches the Sources page union; the badge neutral-styles anything it does
  // not recognize (discord, shared_overlay), so no cast is needed.
  platform: 'twitch' | 'youtube' | 'kick' | 'tiktok' | 'discord' | 'shared_overlay'
  channel_id: string
  channel_name: string
  channel_handle?: string | null
  is_active: boolean
  created_at: string
}

export default function OverlaysPage() {
  const [overlays, setOverlays] = useState<Overlay[]>([])
  const [selectedOverlay, setSelectedOverlay] = useState<Overlay | null>(null)
  const [sources, setSources] = useState<OverlaySource[]>([])
  const [activeOverlayIds, setActiveOverlayIds] = useState<Set<string>>(new Set())
  // overlay id -> connection start timestamp (RFC3339), for "connected for X".
  const [connectedSince, setConnectedSince] = useState<Map<string, string>>(new Map())
  const [loading, setLoading] = useState(true)
  const [sourcesLoading, setSourcesLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [searchTerm, setSearchTerm] = useState('')
  const [showConnectedOnly, setShowConnectedOnly] = useState(false)
  // True when the live-connection endpoint can't be read, so the UI can say
  // "status unknown" instead of implying every overlay is disconnected.
  const [connectionStatusUnavailable, setConnectionStatusUnavailable] = useState(false)
  // Ticking clock so the "connected for" / long-open highlighting stays fresh
  // without calling Date.now() during render (react-hooks/purity).
  const [now, setNow] = useState(() => Date.now())
  const detailRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 60_000)
    return () => clearInterval(t)
  }, [])

  // Fetch all overlays
  useEffect(() => {
    async function fetchOverlays() {
      try {
        // Auth is via the httpOnly session cookie (same-origin); the gateway
        // CookieToBearer middleware copies the access cookie to Authorization
        // before backend validation (no JS-readable token).
        const response = await fetch('/api/v1/admin/overlays', {
          credentials: 'same-origin',
        })

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`)
        }

        const data = await response.json()
        setOverlays(data)
        setLoading(false)

        // Deep-link: auto-select an overlay passed via ?overlay=<id>
        // (e.g. from the Sources page "View" action).
        const targetId = new URLSearchParams(window.location.search).get('overlay')
        if (targetId) {
          const match = (data as Overlay[]).find((o) => o.id === targetId)
          if (match) {
            setSelectedOverlay(match)
            // Surface the target in the (potentially long) list.
            setSearchTerm(match.name)
          }
        }
      } catch (err) {
        console.error('Failed to load overlays:', err)
        setError('Failed to load overlays')
        setLoading(false)
      }
    }

    fetchOverlays()

    async function fetchActiveOverlays() {
      try {
        const response = await fetch('/api/v1/admin/overlays/active', {
          credentials: 'same-origin',
        })
        if (response.ok) {
          const active: ActiveOverlay[] = await response.json()
          setActiveOverlayIds(new Set(active.map((o) => o.overlay_id)))
          setConnectedSince(
            new Map(
              active
                .filter((o) => o.connected_since)
                .map((o) => [o.overlay_id, o.connected_since as string])
            )
          )
          setConnectionStatusUnavailable(false)
        } else {
          // A failed status read must not masquerade as "everything idle".
          setConnectionStatusUnavailable(true)
        }
      } catch (err) {
        console.error('Failed to load active overlays:', err)
        setConnectionStatusUnavailable(true)
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
        const response = await fetch(`/api/v1/admin/overlays/${selectedOverlay.id}/sources`, {
          credentials: 'same-origin',
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

  // On narrow screens the detail panel sits below the (long) list, so bring it
  // into view when an overlay is selected. On desktop the panel is sticky and
  // already visible, so leave the scroll position alone.
  useEffect(() => {
    if (!selectedOverlay) return
    if (window.matchMedia('(max-width: 1023px)').matches) {
      detailRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, [selectedOverlay])

  // Connection start (epoch ms) for sorting; unknown timestamps sort last.
  const connectedStartMs = (id: string): number => {
    const since = connectedSince.get(id)
    const parsed = since ? Date.parse(since) : Number.NaN
    return Number.isNaN(parsed) ? Number.POSITIVE_INFINITY : parsed
  }

  // Filter overlays by search term and connected status
  const connectedCount = overlays.filter((o) => activeOverlayIds.has(o.id)).length
  const filteredOverlays = overlays.filter((o) => {
    if (showConnectedOnly && !activeOverlayIds.has(o.id)) return false
    if (!searchTerm) return true
    const term = searchTerm.toLowerCase()
    return (
      o.name.toLowerCase().includes(term) ||
      o.id.toLowerCase().includes(term) ||
      o.user_id.toLowerCase().includes(term) ||
      (o.owner_username?.toLowerCase().includes(term) ?? false)
    )
  })

  // When narrowed to connected overlays, surface the longest-open first so the
  // likely "dead but open" overlays rise to the top.
  const orderedOverlays = showConnectedOnly
    ? [...filteredOverlays].sort((a, b) => connectedStartMs(a.id) - connectedStartMs(b.id))
    : filteredOverlays

  // Connection status for the detail panel of the selected overlay.
  const selectedConnected = selectedOverlay ? activeOverlayIds.has(selectedOverlay.id) : false
  const selectedSince = selectedOverlay ? connectedSince.get(selectedOverlay.id) : undefined
  const selectedConnectedFor = formatConnectedFor(selectedSince)
  const selectedLongOpen =
    selectedConnected && !!selectedSince && now - Date.parse(selectedSince) >= LONG_OPEN_MS

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

      {connectionStatusUnavailable && (
        <div className="mb-4 rounded-lg border border-amber-500/30 bg-amber-500/10 px-4 py-2 text-sm text-amber-400">
          Live connection status is currently unavailable, so overlays may show as &ldquo;not
          connected&rdquo; even if they are live.
        </div>
      )}

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
                  All Overlays ({filteredOverlays.length}
                  {filteredOverlays.length !== overlays.length ? ` of ${overlays.length}` : ''})
                </h3>

                {/* Search Input */}
                <div className="mt-4 flex items-center gap-3">
                  <input
                    type="text"
                    placeholder="Search by overlay name, ID, or owner..."
                    value={searchTerm}
                    onChange={(e) => setSearchTerm(e.target.value)}
                    className="focus-visible:ring-ring flex-1 rounded-lg border border-border bg-surface-2 px-4 py-2 text-text placeholder:text-text-dim focus-visible:ring-2 focus-visible:outline-none"
                  />
                  <button
                    type="button"
                    aria-pressed={showConnectedOnly}
                    onClick={() => setShowConnectedOnly(!showConnectedOnly)}
                    className={clsx(
                      'flex shrink-0 items-center gap-1.5 rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                      showConnectedOnly
                        ? 'border-kick/30 bg-kick/10 text-kick'
                        : 'border-border bg-surface-2 text-text-sub hover:text-text'
                    )}
                  >
                    <span
                      aria-hidden="true"
                      className="inline-block h-2 w-2 rounded-full bg-kick"
                    />
                    Connected ({connectedCount})
                  </button>
                </div>
              </div>
              <ul className="max-h-[70vh] divide-y divide-border overflow-y-auto">
                {orderedOverlays.map((overlay) => {
                  const isConnected = activeOverlayIds.has(overlay.id)
                  const since = connectedSince.get(overlay.id)
                  const connectedFor = formatConnectedFor(since)
                  const isLongOpen =
                    isConnected && !!since && now - Date.parse(since) >= LONG_OPEN_MS
                  return (
                    <li
                      key={overlay.id}
                      className={clsx(
                        'relative px-4 py-4 transition-colors hover:bg-surface-2',
                        selectedOverlay?.id === overlay.id && 'bg-surface-2'
                      )}
                    >
                      <div className="flex items-center justify-between">
                        {/* Stretched button: after:inset-0 makes the whole row select the
                          overlay; the external-link <Link> below is positioned (relative)
                          so it stays clickable above the stretched hit area. */}
                        <button
                          type="button"
                          onClick={() => setSelectedOverlay(overlay)}
                          aria-current={selectedOverlay?.id === overlay.id ? 'true' : undefined}
                          className="min-w-0 flex-1 cursor-pointer text-left after:absolute after:inset-0"
                        >
                          <div className="flex items-center">
                            {isConnected && (
                              <span
                                className={clsx(
                                  'mr-1.5 inline-block h-2 w-2 shrink-0 rounded-full',
                                  isLongOpen ? 'bg-amber-400' : 'bg-kick'
                                )}
                                title={
                                  since
                                    ? `Connected since ${new Date(since).toLocaleString()}`
                                    : 'Connected'
                                }
                              />
                            )}
                            <p className="text-sm font-medium text-text">{overlay.name}</p>
                            <span className="ml-2 inline-flex items-center rounded bg-badge-bg px-2 py-0.5 text-xs font-medium text-text-sub">
                              {overlay.sources_count || 0} sources
                            </span>
                          </div>
                          <p className="mt-1 font-mono text-xs text-text-sub">ID: {overlay.id}</p>
                          <p className="mt-1 text-xs text-text-dim">
                            Created {new Date(overlay.created_at).toLocaleDateString()}
                            {isConnected && (
                              <>
                                {' · '}
                                <span
                                  className={clsx(
                                    'font-medium',
                                    isLongOpen ? 'text-amber-400' : 'text-kick'
                                  )}
                                >
                                  {connectedFor ? `Connected ${connectedFor}` : 'Connected'}
                                </span>
                              </>
                            )}
                          </p>
                        </button>
                        <div className="flex items-center space-x-2">
                          <Link
                            href={`/overlay/${overlay.id}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            aria-label={`Open overlay ${overlay.name} in a new tab`}
                            className="relative text-sm text-text-sub transition-colors hover:text-text"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <svg
                              aria-hidden="true"
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
                            aria-hidden="true"
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
                  )
                })}
                {orderedOverlays.length === 0 && (
                  <li className="px-4 py-10 text-center text-sm text-text-dim">
                    {overlays.length === 0
                      ? 'No overlays found.'
                      : 'No overlays match your search or filter.'}
                  </li>
                )}
              </ul>
            </Card>
          )}
        </div>

        {/* Overlay Details & Sources */}
        <div ref={detailRef} className="lg:sticky lg:top-8 lg:col-span-1 lg:self-start">
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
                      <dt className="text-sm font-medium text-text-sub">Owner</dt>
                      <dd className="mt-1 text-sm">
                        {selectedOverlay.user_id ? (
                          <Link
                            href={`/admin/users?user=${selectedOverlay.user_id}`}
                            className="text-primary hover:underline"
                          >
                            {selectedOverlay.owner_username
                              ? `@${selectedOverlay.owner_username}`
                              : 'View user'}
                            {selectedOverlay.owner_display_name
                              ? ` (${selectedOverlay.owner_display_name})`
                              : ''}
                          </Link>
                        ) : (
                          <span className="text-text-dim">Unknown</span>
                        )}
                      </dd>
                      <dd className="mt-1 font-mono text-xs break-all text-text-dim">
                        {selectedOverlay.user_id}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-sm font-medium text-text-sub">Connection</dt>
                      <dd className="mt-1 text-sm">
                        {selectedConnected ? (
                          <span className="inline-flex items-center gap-1.5">
                            <span
                              className={clsx(
                                'inline-block h-2 w-2 shrink-0 rounded-full',
                                selectedLongOpen ? 'bg-amber-400' : 'bg-kick'
                              )}
                            />
                            <span
                              className={clsx(
                                'font-medium',
                                selectedLongOpen ? 'text-amber-400' : 'text-kick'
                              )}
                            >
                              {selectedConnectedFor
                                ? `Connected ${selectedConnectedFor}`
                                : 'Connected'}
                            </span>
                          </span>
                        ) : (
                          <span className="text-text-dim">Not connected</span>
                        )}
                      </dd>
                      {selectedConnected && selectedSince && (
                        <dd className="mt-1 text-xs text-text-dim">
                          Since {new Date(selectedSince).toLocaleString()}
                        </dd>
                      )}
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
                                <PlatformBadge platform={source.platform} size="sm" />
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
                              <p className="mt-1">
                                <ChannelLink
                                  platform={source.platform}
                                  channelId={source.channel_id}
                                  channelName={source.channel_name}
                                  channelHandle={source.channel_handle}
                                  className="text-sm font-medium text-text"
                                />
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
                aria-hidden="true"
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
