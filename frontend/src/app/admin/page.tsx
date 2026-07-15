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


import Link from 'next/link'
import { useEffect, useState } from 'react'
import { Users, LayoutGrid, Radio, Eye, Ban, Activity } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'

interface AdminStats {
  total_users: number
  banned_users: number
  active_overlays: number
  total_sources: { [platform: string]: number }
  // Active users = distinct non-banned users with an overlay connected within
  // the window (overlays.last_connected_at). DAU / WAU / MAU.
  active_users_24h: number
  active_users_7d: number
  active_users_30d: number
}

function StatCard({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: number | undefined
  icon: LucideIcon
}) {
  return (
    <Card className="p-6">
      <div className="mb-2 flex items-center gap-3">
        <Icon className="size-5 text-text-sub" aria-hidden="true" />
        <span className="text-sm text-text-sub">{label}</span>
      </div>
      {value === undefined ? (
        <Skeleton className="h-8 w-16" />
      ) : (
        <p className="text-3xl font-bold text-text">{value.toLocaleString()}</p>
      )}
    </Card>
  )
}

export default function AdminDashboard() {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function fetchStats() {
      try {
        // Auth is via the httpOnly session cookie (same-origin); the gateway
        // CookieToBearer middleware copies the access cookie to Authorization
        // before backend validation (no JS-readable token).
        const response = await fetch('/api/v1/admin/stats', {
          credentials: 'same-origin',
        })

        if (response.ok) {
          const data = await response.json()
          setStats(data)
        }
      } catch (err) {
        console.error('Failed to load stats:', err)
      } finally {
        setLoading(false)
      }
    }

    fetchStats()
  }, [])

  const totalSources = stats?.total_sources
    ? Object.values(stats.total_sources).reduce((a, b) => a + b, 0)
    : undefined

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      <h1 className="mb-8 text-2xl font-bold text-text">Admin Dashboard</h1>

      {/* Stats grid */}
      <div className="mb-8 grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          label="Total Users"
          value={loading ? undefined : stats?.total_users}
          icon={Users}
        />
        <StatCard
          label="Banned Users"
          value={loading ? undefined : stats?.banned_users}
          icon={Ban}
        />
        <StatCard
          label="Active Overlays"
          value={loading ? undefined : stats?.active_overlays}
          icon={LayoutGrid}
        />
        <StatCard label="Total Sources" value={loading ? undefined : totalSources} icon={Radio} />
      </div>

      {/* Active users */}
      <div className="mb-2 flex items-center gap-2">
        <Activity className="size-5 text-text-sub" aria-hidden="true" />
        <h2 className="text-lg font-semibold text-text">Active users</h2>
      </div>
      <p className="mb-4 text-sm text-text-sub">
        Distinct users with at least one overlay connected in the window (excludes banned users).
      </p>
      <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard
          label="Last 24 hours"
          value={loading ? undefined : stats?.active_users_24h}
          icon={Activity}
        />
        <StatCard
          label="Last 7 days"
          value={loading ? undefined : stats?.active_users_7d}
          icon={Activity}
        />
        <StatCard
          label="Last 30 days"
          value={loading ? undefined : stats?.active_users_30d}
          icon={Activity}
        />
      </div>

      {/* Navigation cards */}
      <h2 className="mb-4 text-lg font-semibold text-text">Manage</h2>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Link href="/admin/users">
          <Card className="cursor-pointer p-5 transition-colors hover:bg-surface-2">
            <div className="flex items-center gap-3">
              <Users className="size-6 text-text-sub" aria-hidden="true" />
              <div>
                <p className="text-sm font-medium text-text">Users</p>
                <p className="text-xs text-text-sub">View and manage users</p>
              </div>
            </div>
          </Card>
        </Link>

        <Link href="/admin/overlays">
          <Card className="cursor-pointer p-5 transition-colors hover:bg-surface-2">
            <div className="flex items-center gap-3">
              <LayoutGrid className="size-6 text-text-sub" aria-hidden="true" />
              <div>
                <p className="text-sm font-medium text-text">Overlays</p>
                <p className="text-xs text-text-sub">Manage overlays</p>
              </div>
            </div>
          </Card>
        </Link>

        <Link href="/admin/sources">
          <Card className="cursor-pointer p-5 transition-colors hover:bg-surface-2">
            <div className="flex items-center gap-3">
              <Radio className="size-6 text-text-sub" aria-hidden="true" />
              <div>
                <p className="text-sm font-medium text-text">Sources</p>
                <p className="text-xs text-text-sub">View all sources</p>
              </div>
            </div>
          </Card>
        </Link>

        <Link href="/admin/viewers">
          <Card className="cursor-pointer p-5 transition-colors hover:bg-surface-2">
            <div className="flex items-center gap-3">
              <Eye className="size-6 text-text-sub" aria-hidden="true" />
              <div>
                <p className="text-sm font-medium text-text">Viewers</p>
                <p className="text-xs text-text-sub">View all viewers</p>
              </div>
            </div>
          </Card>
        </Link>
      </div>
    </div>
  )
}
