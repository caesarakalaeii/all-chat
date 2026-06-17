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

import { readdirSync, readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { BUNDLED_THEMES } from '../bundled-themes.generated'

/**
 * Guards the codegen: the committed bundled-themes.generated.ts must match the
 * canonical docs/overlay-themes/*.css. If a theme is edited without re-running
 * `npm run generate:themes`, this fails — preventing the stale-snapshot class
 * of bug this whole refactor exists to kill.
 */
const THEMES_DIR = fileURLToPath(new URL('../../../../../docs/overlay-themes', import.meta.url))

const sourceFiles = readdirSync(THEMES_DIR)
  .filter((f) => f.endsWith('.css'))
  .sort()

describe('bundled themes are in sync with docs/overlay-themes', () => {
  it('bundles every source theme exactly once', () => {
    const bundledFilenames = BUNDLED_THEMES.map((t) => t.filename).sort()
    expect(bundledFilenames).toEqual(sourceFiles)
  })

  it.each(sourceFiles)('%s CSS matches the committed bundle', (filename) => {
    const expected = readFileSync(`${THEMES_DIR}/${filename}`, 'utf8')
    const bundled = BUNDLED_THEMES.find((t) => t.filename === filename)
    expect(bundled, `theme ${filename} missing from bundle — run npm run generate:themes`).toBeDefined()
    expect(bundled!.css, `theme ${filename} is stale — run npm run generate:themes`).toBe(expected)
  })
})
