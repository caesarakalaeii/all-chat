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
 * NOTE: the OBS embed route (`/overlay/[id]`) stays public — only this nested
 * monitor view is gated.
 */

import { ProtectedRoute } from '@/components/ProtectedRoute'

export function OverlayViewGuard({ children }: { children: React.ReactNode }) {
  return <ProtectedRoute>{children}</ProtectedRoute>
}
