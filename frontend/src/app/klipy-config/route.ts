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

import { NextResponse } from 'next/server'

// Not under /api/: the ingress sends /api/* to the gateway, never here.
// Read the key per request (it comes from the allchat-secrets k8s secret at
// runtime), never at build time.
export const dynamic = 'force-dynamic'

/**
 * The KLIPY app key for the browser (ADR-0072). KLIPY requires search calls
 * to come from the user's browser, so the key is public by nature: it travels
 * in every search URL. Empty when unset; the picker then says "unavailable".
 */
export function GET() {
  return NextResponse.json(
    { apiKey: process.env.KLIPY_API_KEY ?? '' },
    { headers: { 'Cache-Control': 'no-store' } }
  )
}
