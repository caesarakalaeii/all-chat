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
import { SigningSession } from './signing/session.js';
import { ViewerPool } from './signing/viewer.js';
import { fetchWebshareProxies } from './signing/webshare.js';
const PORT = parseInt(process.env.PORT || '8092', 10);
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';
const logger = {
    info(message, meta) {
        if (LOG_LEVEL !== 'silent') {
            console.log(JSON.stringify({ level: 'info', message, ...meta }));
        }
    },
    warn(message, meta) {
        if (LOG_LEVEL !== 'silent') {
            console.warn(JSON.stringify({ level: 'warn', message, ...meta }));
        }
    },
    error(message, meta) {
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
const viewer = viewerMode
    ? new ViewerPool({
        proxyHosts: staticProxyHosts,
        proxyUser,
        proxyPass,
        executablePath: process.env.PUPPETEER_EXECUTABLE_PATH,
        userDataDir: userDataDir + '-viewer',
        display: process.env.SIGNER_DISPLAY,
        maxLaneAttempts: parseInt(process.env.SIGNER_MAX_LANE_ATTEMPTS || '3', 10),
        logger: { info: logger.info, warn: logger.warn }
    })
    : undefined;
// Refresh the proxy list from webshare hourly: replacements and removals in
// the dashboard propagate without a redeploy. Surviving lanes keep their
// browsers and pinned rooms (see ViewerPool.refreshProxies).
let proxyRefresh;
async function refreshProxiesFromWebshare() {
    if (!viewer || !webshareToken)
        return;
    try {
        const list = await fetchWebshareProxies(webshareToken);
        proxyUser = list.username;
        proxyPass = list.password;
        // Credentials ride with the refresh: the pool was constructed from the
        // static-list fallback and may have none, and lane pages authenticate
        // per-request from the pool's own options.
        await viewer.refreshProxies(list.hosts, { username: list.username, password: list.password });
        logger.info('viewer proxy pool refreshed from webshare', { proxies: list.hosts.length });
    }
    catch (error) {
        logger.error('webshare proxy refresh failed; keeping current lanes', {
            error: error.message
        });
    }
}
const server = createServer({ port: PORT, session, viewer, logger });
server.listen(PORT, () => {
    logger.info('tiktok-signer listening', { port: PORT });
    if (viewer && webshareToken) {
        void refreshProxiesFromWebshare();
        proxyRefresh = setInterval(() => void refreshProxiesFromWebshare(), 60 * 60 * 1000);
    }
    else if (viewer && staticProxyHosts.length > 0) {
        logger.info('viewer proxy pool configured from static list', {
            proxies: staticProxyHosts.length
        });
    }
});
async function shutdown(signal) {
    logger.info('shutting down', { signal });
    if (proxyRefresh)
        clearInterval(proxyRefresh);
    server.close();
    await session.close();
    await viewer?.close();
    process.exit(0);
}
process.on('SIGTERM', () => void shutdown('SIGTERM'));
process.on('SIGINT', () => void shutdown('SIGINT'));
//# sourceMappingURL=index.js.map