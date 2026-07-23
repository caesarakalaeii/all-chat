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
 * Vendor Monaco Editor into public/ so the CSS editor loads it from our own
 * origin instead of cdn.jsdelivr.net.
 *
 * WHY: @monaco-editor/react defaults to fetching the Monaco engine from
 * https://cdn.jsdelivr.net/npm/monaco-editor@<ver>/min/vs. The app CSP
 * script-src allows only 'self' (+ a couple of named hosts), so that CDN
 * script is blocked and the editor hangs forever on "Loading editor...".
 * Serving Monaco from /monaco/vs keeps every request same-origin — no CSP
 * hole for a third-party CDN, and Monaco's web workers load under
 * default-src 'self' with no worker-src/blob relaxation. See ADR-0040.
 *
 * Source: node_modules/monaco-editor/min/vs. monaco-editor is a peer
 * dependency of @monaco-editor/react — npm auto-installs it and the lockfile
 * pins it (0.55.1), so `npm ci` provides it deterministically without a
 * separate direct-dependency entry (kept out to avoid lockfile churn).
 * Output: public/monaco/vs. The output is gitignored (16 MB, regenerated
 * from node_modules) and produced at build time via the `prebuild`/`predev`
 * npm hooks.
 */

import { cpSync, existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __dirname = dirname(fileURLToPath(import.meta.url))
const require = createRequire(import.meta.url)

// monaco-editor >=0.56.0 removed "./package.json" from its exports map, which
// breaks both resolves below — the version is pinned via package.json
// "overrides" until this script resolves the root another way.
const MONACO_VERSION = require('monaco-editor/package.json').version
const SRC_VS = join(dirname(require.resolve('monaco-editor/package.json')), 'min', 'vs')
const DEST_DIR = join(__dirname, '..', 'public', 'monaco')
const DEST_VS = join(DEST_DIR, 'vs')
const VERSION_MARKER = join(DEST_DIR, '.monaco-version')

// Skip the copy when public/monaco already holds this exact monaco-editor
// version — keeps `npm run dev`/`build` startup fast. A version bump (or a
// missing marker) forces a fresh copy so the vendored assets never go stale.
const alreadyCurrent =
  existsSync(VERSION_MARKER) &&
  existsSync(join(DEST_VS, 'loader.js')) &&
  readFileSync(VERSION_MARKER, 'utf8').trim() === MONACO_VERSION

if (alreadyCurrent) {
  console.log(`[copy-monaco] public/monaco already at ${MONACO_VERSION}, skipping`)
  process.exit(0)
}

if (!existsSync(SRC_VS)) {
  console.error(
    `[copy-monaco] source not found: ${SRC_VS}\n  Is monaco-editor installed? Run \`npm install\`.`
  )
  process.exit(1)
}

mkdirSync(DEST_DIR, { recursive: true })
cpSync(SRC_VS, DEST_VS, { recursive: true })
writeFileSync(VERSION_MARKER, MONACO_VERSION + '\n')
console.log(`[copy-monaco] copied monaco-editor@${MONACO_VERSION} -> public/monaco/vs`)
