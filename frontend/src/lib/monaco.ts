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
 * Monaco loader configuration (self-hosted).
 *
 * @monaco-editor/react loads the Monaco engine from cdn.jsdelivr.net by
 * default. The app CSP (see next.config.js) restricts script-src to 'self'
 * plus a couple of named hosts, so the CDN loader is blocked and the editor
 * hangs on "Loading editor..." forever. Pointing the loader at our own
 * /monaco/vs (vendored into public/ by scripts/copy-monaco.mjs) keeps every
 * request same-origin — no third-party CDN in the CSP, and Monaco's web
 * workers load under default-src 'self' with no blob/worker-src relaxation.
 *
 * Import this module for its side effect BEFORE any <Editor> mounts. It is
 * safe to import more than once; loader.config only takes effect until the
 * first loader.init(), and every editor entry point routes through here.
 *
 * See ADR-0040.
 */

import { loader } from '@monaco-editor/react'

loader.config({ paths: { vs: '/monaco/vs' } })

export { loader }
