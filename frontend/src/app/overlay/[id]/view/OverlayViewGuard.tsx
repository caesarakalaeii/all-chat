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

'use client'

/**
 * Auth gate for the overlay monitor (`/overlay/[id]/view`).
 *
 * The monitor's layout is a server component (it owns `metadata` and paints the
 * solid background). This thin client wrapper lets that server layout require a
 * logged-in user without becoming a client component itself: it simply renders
 * its children inside `ProtectedRoute`, which redirects anonymous visitors home.
 *
 * In dock mode (`?dock=1`) the redirect is replaced by an in-place sign-in
 * panel: an OBS custom browser dock has its own cookie jar and no browser
 * chrome, so sending it to the homepage strands the streamer in a ~320px panel
 * with no way back. See `DockSignIn`.
 *
 * NOTE: the OBS embed route (`/overlay/[id]`) stays public — only this nested
 * monitor view is gated.
 */

import { Suspense } from 'react'
import { useSearchParams } from 'next/navigation'

import { ProtectedRoute } from '@/components/ProtectedRoute'

import { DockSignIn } from './DockSignIn'
import { isDockMode } from './dockMode'

function DockAwareGuard({ children }: { children: React.ReactNode }) {
  const dock = isDockMode(useSearchParams())
  return <ProtectedRoute fallback={dock ? <DockSignIn /> : undefined}>{children}</ProtectedRoute>
}

export function OverlayViewGuard({ children }: { children: React.ReactNode }) {
  // useSearchParams opts this subtree out of static rendering, so it needs its
  // own boundary (same reason as /moderate). The gate renders nothing of its
  // own while suspended — the page below paints the monitor chrome.
  return (
    <Suspense fallback={null}>
      <DockAwareGuard>{children}</DockAwareGuard>
    </Suspense>
  )
}
