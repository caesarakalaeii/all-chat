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

import { ReactNode } from 'react'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { ToastProvider } from '@/components/admin/ToastProvider'
import { AdminSidebar } from '@/components/AdminSidebar'

function AdminLayoutContent({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-bg">
      <ToastProvider />
      <AdminSidebar />
      {/* Offset for the fixed desktop sidebar; full width below the lg breakpoint. */}
      <div className="lg:pl-60">
        <main>{children}</main>
      </div>
    </div>
  )
}

export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <ProtectedRoute requireAdmin={true}>
      <AdminLayoutContent>{children}</AdminLayoutContent>
    </ProtectedRoute>
  )
}
