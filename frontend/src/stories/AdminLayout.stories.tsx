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

import type { Meta, StoryObj } from '@storybook/nextjs-vite'
import { AdminNav } from '@/components/AdminNav'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Users, LayoutGrid, Radio, Eye } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'

function StatCard({
  label,
  value,
  icon: Icon,
}: {
  label: string
  value: number
  icon: LucideIcon
}) {
  return (
    <Card className="p-6">
      <div className="mb-2 flex items-center gap-3">
        <Icon className="size-5 text-text-sub" aria-hidden="true" />
        <span className="text-sm text-text-sub">{label}</span>
      </div>
      <p className="text-3xl font-bold text-text">{value.toLocaleString()}</p>
    </Card>
  )
}

const STATS: { label: string; value: number; Icon: LucideIcon }[] = [
  { label: 'Users', value: 1_247, Icon: Users },
  { label: 'Overlays', value: 384, Icon: LayoutGrid },
  { label: 'Sources', value: 912, Icon: Radio },
  { label: 'Viewers', value: 8_403, Icon: Eye },
]

// Self-contained admin layout preview with dark theme
function AdminLayoutPreview() {
  return (
    <div className="min-h-screen bg-bg">
      <AdminNav />
      <div className="mx-auto max-w-7xl px-4 py-8">
        <h1 className="mb-8 text-2xl font-bold text-text">Admin Dashboard</h1>
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {STATS.map(({ label, value, Icon }) => (
            <StatCard key={label} label={label} value={value} icon={Icon} />
          ))}
        </div>
      </div>
    </div>
  )
}

// Loading state — stats show skeleton placeholders
function AdminLayoutLoading() {
  return (
    <div className="min-h-screen bg-bg">
      <AdminNav />
      <div className="mx-auto max-w-7xl px-4 py-8">
        <Skeleton className="mb-8 h-8 w-48" />
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <Card key={i} className="p-6">
              <Skeleton className="mb-2 h-5 w-24" />
              <Skeleton className="h-8 w-16" />
            </Card>
          ))}
        </div>
      </div>
    </div>
  )
}

const meta: Meta<typeof AdminLayoutPreview> = {
  title: 'Pages/Admin',
  component: AdminLayoutPreview,
}
export default meta
type Story = StoryObj<typeof AdminLayoutPreview>

export const Default: Story = {}

export const Loading: StoryObj<typeof AdminLayoutLoading> = {
  render: () => <AdminLayoutLoading />,
}
