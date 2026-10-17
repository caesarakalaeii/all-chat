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

// Each answer belongs to one session: no shared cache may store or replay it.
const HEADERS = { 'Cache-Control': 'private, no-store', Vary: 'Cookie, Authorization' }

const answer = (apiKey: string, status = 200) =>
  NextResponse.json({ apiKey }, { status, headers: HEADERS })

/**
 * The KLIPY app key for the browser (ADR-0072, ADR-0073, ADR-0074). KLIPY
 * requires search calls to come from the user's browser, so the key has to
 * reach it, but only a signed-in user who may use the alerts editor gets it:
 * 401 without a valid session, 403 without premium or beta access.
 *
 * The session is checked by forwarding the caller's credentials to the
 * gateway's GET /api/v1/auth/me (API_GATEWAY_URL, the in-cluster service),
 * which validates the JWT and returns the user's premium state. Any failure
 * of that check is a 401. Empty key when KLIPY_API_KEY is unset; the picker
 * then says "unavailable".
 */
export async function GET(request: Request) {
  const credentials: Record<string, string> = {}
  for (const name of ['cookie', 'authorization']) {
    const value = request.headers.get(name)
    if (value) credentials[name] = value
  }
  if (Object.keys(credentials).length === 0) return answer('', 401)

  const gateway = process.env.API_GATEWAY_URL || 'http://localhost:8080'
  let user: { is_premium?: unknown; is_beta_tester?: unknown }
  try {
    const res = await fetch(`${gateway}/api/v1/auth/me`, {
      headers: credentials,
      cache: 'no-store',
      signal: AbortSignal.timeout(5000),
    })
    if (!res.ok) return answer('', 401)
    user = await res.json()
  } catch {
    return answer('', 401)
  }
  // What the alerts editor unlocks on (users.is_premium; beta testers get it too).
  if (user.is_premium !== true && user.is_beta_tester !== true) return answer('', 403)

  return answer(process.env.KLIPY_API_KEY ?? '')
}
