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

/**
 * Client for services/tiktok-relay, which repeats TikTok's live lookup by
 * handle from a US region. Accounts hosted by TikTok's US entity answer that
 * lookup with user_not_found from our EU egress, while the room ID it returns
 * works from the EU (check_alive, WebSocket). Only the lookup goes through the
 * relay.
 */

export interface RelayLookup {
  found: boolean;
  live: boolean;
  roomId?: string;
}

export interface UsRelay {
  lookup(username: string): Promise<RelayLookup>;
}

export function createUsRelay(
  baseUrl: string,
  token: string,
  fetchImpl: typeof fetch = fetch
): UsRelay | undefined {
  if (!baseUrl || !token) return undefined;
  const base = baseUrl.replace(/\/+$/, '');
  return {
    async lookup(username: string): Promise<RelayLookup> {
      const res = await fetchImpl(`${base}/v1/live?handle=${encodeURIComponent(username)}`, {
        headers: { 'x-relay-token': token },
        signal: AbortSignal.timeout(15_000)
      });
      if (!res.ok) throw new Error(`tiktok-relay answered HTTP ${res.status}`);
      const body = (await res.json()) as { found?: boolean; live?: boolean; roomId?: string | null };
      return {
        found: body.found === true,
        live: body.live === true,
        roomId: body.roomId ?? undefined
      };
    }
  };
}
