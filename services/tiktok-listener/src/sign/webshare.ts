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

import { request as undiciRequest } from 'undici';

/**
 * Webshare proxy-list client, mirrored from services/tiktok-signer.
 *
 * The listener pins each connection's WebSocket egress to the residential
 * proxy the signer's viewer captured the session through; that needs the
 * current proxy credentials, and webshare rotates them dashboard-side
 * without notice. Reading them from the API (token only, stable) keeps a
 * redeploy out of the rotation loop.
 */

export interface ProxyCredentials {
  username: string;
  password: string;
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
 * Fetch the account's current shared proxy credentials from the webshare
 * API. Webshare's static-residential product uses one user/pass pair across
 * the list; any valid entry carries them.
 */
export async function fetchWebshareCredentials(token: string): Promise<ProxyCredentials> {
  const response = await undiciRequest(
    'https://proxy.webshare.io/api/v2/proxy/list/?mode=direct&page=1&page_size=1',
    { headers: { Authorization: `Token ${token}` } }
  );
  if (response.statusCode !== 200) {
    throw new Error(`webshare proxy list returned ${response.statusCode}`);
  }
  const body = JSON.parse(await response.body.text()) as WebshareListResponse;
  const valid = body.results.find((entry) => entry.valid);
  if (!valid) {
    throw new Error('webshare proxy list contained no valid proxies');
  }
  return { username: valid.username, password: valid.password };
}
