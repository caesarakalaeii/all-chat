/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

import { request as undiciRequest } from 'undici';

export interface ProxyEntry {
  /** host:port, ready for Chromium's --proxy-server. */
  host: string;
  username: string;
  password: string;
}

/**
 * Per-proxy credentials, keyed by host:port. Webshare issues a username and
 * password PER PROXY (mode=direct), not one pair for the list — verified
 * 2026-09-16 when every lane except the first failed Chromium's proxy auth
 * with ERR_INVALID_AUTH_CREDENTIALS after webshare's auto-replace rotated
 * six IPs and issued fresh per-proxy credentials.
 */
export type ProxyCredentials = Record<string, { username: string; password: string }>;

export interface ProxyList {
  hosts: string[];
  /**
   * Shared credentials (webshare's documented shape) when every entry
   * carries the same pair; null when they are per-proxy, in which case
   * `credentials` is the authoritative map.
   */
  username?: string;
  password?: string;
  /** Per-proxy credentials, always populated. */
  credentials: ProxyCredentials;
}

interface WebshareProxyEntry {
  proxy_address: string;
  port: number;
  username: string;
  password: string;
  valid: boolean;
}

interface WebshareListResponse {
  count: number;
  results: WebshareProxyEntry[];
}

/**
 * Fetch the account's static residential proxy list from the webshare API.
 * The signer reads its proxies from the API instead of a hand-maintained
 * secret so replacements/rotations in the webshare dashboard propagate on
 * the next refresh without touching the cluster.
 */
export async function fetchWebshareProxies(
  token: string,
  { page = 1, pageSize = 100 }: { page?: number; pageSize?: number } = {}
): Promise<ProxyList> {
  const response = await undiciRequest(
    `https://proxy.webshare.io/api/v2/proxy/list/?mode=direct&page=${page}&page_size=${pageSize}`,
    { headers: { Authorization: `Token ${token}` } }
  );
  if (response.statusCode !== 200) {
    throw new Error(`webshare proxy list returned ${response.statusCode}`);
  }
  const body = JSON.parse(await response.body.text()) as WebshareListResponse;
  const valid = body.results.filter((entry) => entry.valid);
  if (valid.length === 0) {
    throw new Error('webshare proxy list contained no valid proxies');
  }
  const credentials: ProxyCredentials = {};
  for (const entry of valid) {
    credentials[`${entry.proxy_address}:${entry.port}`] = {
      username: entry.username,
      password: entry.password
    };
  }
  // A list where every entry shares one pair keeps the shared shape so
  // callers that only want "the" credentials still work.
  const [first, ...rest] = valid;
  const shared = rest.every(
    (e) => e.username === first.username && e.password === first.password
  );
  return {
    hosts: valid.map((entry) => `${entry.proxy_address}:${entry.port}`),
    ...(shared ? { username: first.username, password: first.password } : {}),
    credentials
  };
}
