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


import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'

export default function ImpersonationBanner() {
  const router = useRouter()
  const { isImpersonating, impersonatedUsername, stopImpersonation } = useAuthStore()

  const handleExitImpersonation = async () => {
    // H3 cookie auth: the server restores the admin access cookie and returns
    // the restored admin user — no client-side token swap. The store is
    // updated from the response, so a separate init()/me re-fetch is unneeded.
    try {
      await stopImpersonation()
    } catch (err) {
      console.error('Failed to stop impersonation:', err)
    }

    // Redirect back to admin panel
    router.push('/admin/users')
  }

  if (!isImpersonating) {
    return null
  }

  return (
    <div className="flex items-center justify-between bg-orange-600 px-4 py-2 text-white shadow-md">
      <div className="flex items-center gap-3">
        <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
        <div>
          <span className="font-semibold">Admin Mode:</span> Viewing as{' '}
          <span className="font-mono">{impersonatedUsername}</span>
        </div>
      </div>
      <button
        onClick={handleExitImpersonation}
        className="rounded bg-white px-4 py-1 font-medium text-orange-600 transition-colors hover:bg-orange-50"
      >
        Exit & Return to Admin
      </button>
    </div>
  )
}
