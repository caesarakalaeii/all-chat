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
* tiktok-signer entry point. ADR-0052 step 1: the self-hosted replacement for
* Euler Stream's signing, so TikTok ingest stops depending on a third-party
* sign service (connection ceiling, paywalled gift enrichment, credential
* exposure — see the ADR for the measured costs).
*/

import { createServer } from './api.js';
import { RelayHub } from './signing/relay.js';
import { SigningSession } from './signing/session.js';
import { ViewerPool, findDualListedRooms } from './signing/viewer.js';
import { WarmCurator } from './signing/warm-curator.js';
import { fetchWebshareProxies } from './signing/webshare.js';
// Stealth-plugin page hooks race lane/browser shutdown: rotateLaneProfile and
// refreshProxies close the browser while the plugin's onPageCreated hook is
// still calling into the CDP session of a target that just closed. The
// resulting TargetCloseError/ProtocolError rejection arrives after every
// await we own, so it lands here as "unhandled" and kills the service
// (measured 2026-09-16: two signer restarts in twelve minutes under direct
// mode, where the single lane rotates on every failure streak). The target
// was closing anyway — log and keep serving.
process.on('unhandledRejection', (reason) => {
  if (
    typeof reason === 'object' &&
    reason !== null &&
    'name' in reason &&
    (reason.name === 'TargetCloseError' || reason.name === 'ProtocolError')
  ) {
    const message =
      typeof reason === 'object' && 'message' in reason ? String(reason.message) : '';
    console.warn(
      JSON.stringify({ level: 'warn', message: 'swallowing CDP close race', error: message.slice(0, 200) })
    );
    return;
  }
  console.error(JSON.stringify({ level: 'error', message: 'unhandled rejection, crashing', error: String(reason).slice(0, 400) }));
  process.exit(1);
});

const PORT = parseInt(process.env.PORT || '8092', 10);
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';

const logger = {
  info(message: string, meta?: Record<string, unknown>): void {
    if (LOG_LEVEL !== 'silent') {
      console.log(JSON.stringify({ level: 'info', message, ...meta }));
    }
  },
  warn(message: string, meta?: Record<string, unknown>): void {
    if (LOG_LEVEL !== 'silent') {
      console.warn(JSON.stringify({ level: 'warn', message, ...meta }));
    }
  },
  error(message: string, meta?: Record<string, unknown>): void {
    console.error(JSON.stringify({ level: 'error', message, ...meta }));
  }
};

const userDataDir = process.env.SIGNER_USER_DATA_DIR || '/tmp/tiktok-signer-profile';

const session = new SigningSession({
  proxyHost: process.env.SIGNER_PROXY_HOST,
  proxyUser: process.env.SIGNER_PROXY_USER,
  proxyPass: process.env.SIGNER_PROXY_PASS,
  executablePath: process.env.PUPPETEER_EXECUTABLE_PATH,
  userDataDir
});

// Page-viewer mode (ADR-0052 post-2026-09-09): TikTok gates webcast data on a
// browser-grade session, so signing alone no longer serves the fetch. When
// enabled, /v1/sign requests carrying a username are served by a real viewer
// tab instead of the signature path. The signing session stays up for
// /v1/sign-url (the gift-list seam still only needs a signature).
const viewerMode = (process.env.SIGNER_VIEWER_MODE || 'signature') === 'page';
// Residential proxy pool for the viewer lanes: TikTok gates the chat bootstrap
// on IP reputation, so viewer browsers egress through residential IPs. One
// browser per proxy (own profile = own session identity); rooms are pinned to
// their lane, and a failing lane is benched for a cooldown while another
// serves. Proxies come from the webshare API (SIGNER_WEBSHARE_TOKEN) so
// dashboard-side rotations propagate without touching the cluster; a static
// comma-separated SIGNER_PROXY_HOSTS is the fallback. The legacy singular
// SIGNER_PROXY_HOST still applies to the signature session.
const webshareToken = (process.env.SIGNER_WEBSHARE_TOKEN || '').trim();
const staticProxyHosts = (process.env.SIGNER_PROXY_HOSTS || '')
  .split(',')
  .map((h) => h.trim())
  .filter(Boolean);
let proxyUser = process.env.SIGNER_PROXY_USER;
let proxyPass = process.env.SIGNER_PROXY_PASS;
/** Comma-separated room list from env, one canonical lowercase form (the
 * canary set, warm rooms, pool tabs, breaker and relay subscriber keys all
 * compare on this form — a mixed-case entry must not fork identity). */
function parseRoomList(env: string | undefined): string[] {
  return (env || '')
    .split(',')
    .map((r) => r.trim().toLowerCase())
    .filter(Boolean);
}

// Warm rooms (pure-Node transport PR 1): the classic rooms whose captured
// WS session GET /v1/session leases to the listener.
const warmRooms = parseRoomList(process.env.SIGNER_WARM_ROOMS);
const warmRoomSet = new Set(warmRooms);
// Canary isolation (PR 1 item 6): canary rooms capture on their own
// profiles so a flag earned on a primary jar cannot blind the canary.
const canaryRooms = new Set(parseRoomList(process.env.SIGNER_RELAY_CANARY_ROOMS));
const dualListed = findDualListedRooms(canaryRooms, warmRooms);
if (dualListed.length > 0) {
  // Refuse to start: a dual-listed room's canary jar would be leased as the
  // primary session — invisible in logs, while a crash-looping pod is
  // GitOps- and alert-visible. Deliberate config-validation exit.
  console.error(`SIGNER_WARM_ROOMS and SIGNER_RELAY_CANARY_ROOMS overlap: ${dualListed.join(', ')}`);
  process.exit(1);
}
const viewer = viewerMode
  ? new ViewerPool({
      proxyHosts: staticProxyHosts,
      proxyUser,
      proxyPass,
      executablePath: process.env.PUPPETEER_EXECUTABLE_PATH,
      userDataDir: userDataDir + '-viewer',
      canaryProfileDir:
        process.env.SIGNER_CANARY_PROFILE || userDataDir + '-viewer-canary',
      canaryRooms,
      pinnedRooms: warmRoomSet,
      display: process.env.SIGNER_DISPLAY,
      maxLaneAttempts: parseInt(process.env.SIGNER_MAX_LANE_ATTEMPTS || '2', 10),
      logger: { info: logger.info, warn: logger.warn }
    })
  : undefined;

// Warm-room auto-curator (2026-09-20): SIGNER_WARM_ROOMS seeds the list;
// when live coverage drops the curator discovers, verifies and ADDS
// rooms on its own (see warm-curator.ts). It never removes. Off by
// default until the caesar flag flip - the seed list alone must keep
// serving, and an off-curve curator can simply be turned off.
const autocurate = (process.env.SIGNER_WARM_AUTOCURATE || '').trim().toLowerCase() === 'on';
const curator = autocurate && viewer
  ? new WarmCurator({
      warmRooms: warmRoomSet,
      canaryRooms,
      viewer,
      logger: { info: logger.info, warn: logger.warn }
    })
  : undefined;
if (curator) curator.start();

// Premium fallback gate (2026-09-16 transport plan, phase 3): when on,
// GET /v1/stream/:username serves any room with a warm tab, not just the
// canary set — the listener's premium-fallback tier promotes rooms onto
// the relay after their primary WS exhausts flap retries.
const relayFallbackEnabled = (process.env.SIGNER_RELAY_FALLBACK || '').trim().toLowerCase() === 'on';
// Refresh the proxy list from webshare hourly: replacements and removals in
// the dashboard propagate without a redeploy. Surviving lanes keep their
// browsers and pinned rooms (see ViewerPool.refreshProxies).
let proxyRefresh: ReturnType<typeof setInterval> | undefined;
async function refreshProxiesFromWebshare(): Promise<void> {
  if (!viewer || !webshareToken) return;
  try {
    const list = await fetchWebshareProxies(webshareToken);
    proxyUser = list.username;
    proxyPass = list.password;
    // Credentials ride with the refresh: the pool was constructed from
    // the static-list fallback and may have none, and lane pages
    // authenticate per-request from the pool's own options. Webshare's
    // per-proxy credentials go straight to the pool.
    await viewer.refreshProxies(
      list.hosts,
      list.username ? { username: list.username, password: list.password ?? '' } : undefined,
      list.credentials
    );
    logger.info('viewer proxy pool refreshed from webshare', { proxies: list.hosts.length });
  } catch (error) {
    logger.error('webshare proxy refresh failed; keeping current lanes', {
      error: (error as Error).message
    });
  }
}

const relay = viewer
  ? new RelayHub((username) => { viewer.pinTab(username); }, { logger })
  : undefined;

const server = createServer({ port: PORT, session, viewer, relay, canaryRooms, relayFallbackEnabled, warmRooms: [...warmRoomSet], logger });

server.listen(PORT, () => {
  logger.info('tiktok-signer listening', { port: PORT });
  if (viewer && webshareToken) {
    void refreshProxiesFromWebshare();
    proxyRefresh = setInterval(() => void refreshProxiesFromWebshare(), 60 * 60 * 1000);
  } else if (viewer && staticProxyHosts.length > 0) {
    logger.info('viewer proxy pool configured from static list', {
      proxies: staticProxyHosts.length
    });
  }
});

async function shutdown(signal: string): Promise<void> {
  logger.info('shutting down', { signal });
  if (proxyRefresh) clearInterval(proxyRefresh);
  await relay?.close();
  await viewer?.close();
  await session.close();
  process.exit(0);
}

process.on('SIGTERM', () => void shutdown('SIGTERM'));
process.on('SIGINT', () => void shutdown('SIGINT'));
