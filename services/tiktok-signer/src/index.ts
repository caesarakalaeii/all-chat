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

const PORT = parseInt(process.env.PORT || '8092', 10);
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';

const logger = {
  info(message: string, meta?: Record<string, unknown>): void {
    if (LOG_LEVEL !== 'silent') {
      console.log(JSON.stringify({ level: 'info', message, ...meta }));
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

const server = createServer({ port: PORT, session, logger });

server.listen(PORT, () => {
  logger.info('tiktok-signer listening', { port: PORT });
});

async function shutdown(signal: string): Promise<void> {
  logger.info('shutting down', { signal });
  server.close();
  await session.close();
  process.exit(0);
}

process.on('SIGTERM', () => void shutdown('SIGTERM'));
process.on('SIGINT', () => void shutdown('SIGINT'));
