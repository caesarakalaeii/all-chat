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

import { apiClient } from './client'
import { safeExternalRedirect } from '../auth/redirect-allowlist'

export interface PaymentStatus {
  connected: boolean
  status?: string // active | declined | former | expired | none
  tier_id?: string
  cents?: number
  renews_at?: string | null
  is_premium?: boolean
}

export async function getPaymentStatus(): Promise<PaymentStatus> {
  return apiClient.get<PaymentStatus>('/api/v1/payment/status')
}

// startPatreonConnect fetches the Patreon consent URL and redirects the browser to it.
export async function startPatreonConnect(): Promise<void> {
  const data = await apiClient.get<{ auth_url: string }>('/api/v1/payment/patreon/connect')
  if (data.auth_url) {
    safeExternalRedirect(data.auth_url)
  }
}

export async function disconnectPatreon(): Promise<void> {
  return apiClient.delete('/api/v1/payment/patreon/connection')
}
