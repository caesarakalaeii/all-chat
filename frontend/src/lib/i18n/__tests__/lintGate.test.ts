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
 * Behaviour lock for the i18n lint gate itself (eslint.i18n.config.mjs).
 *
 * The gate has to draw a line between copy and everything else that can appear
 * as JSX text. Getting that line wrong in either direction is expensive and
 * silent: too strict and the ratchet can never drain, so the required check
 * blocks every PR; too loose and new literals land unnoticed, which is the whole
 * thing #799 exists to stop.
 *
 * These cases are asserted by running ESLint in-process over source strings,
 * which is the same surface CI runs, rather than by reading the config object.
 */

import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

import { ESLint } from 'eslint'
import { describe, expect, it } from 'vitest'

const CONFIG_PATH = fileURLToPath(new URL('../../../../eslint.i18n.config.mjs', import.meta.url))

/** Rule ids reported for `source`, treated as a .tsx file under src/. */
async function lint(source: string): Promise<string[]> {
  const eslint = new ESLint({
    overrideConfigFile: CONFIG_PATH,
    cwd: fileURLToPath(new URL('../../../../', import.meta.url)),
  })
  const [result] = await eslint.lintText(source, { filePath: 'src/components/Probe.tsx' })
  return result.messages.filter((m) => m.severity === 2).map((m) => m.ruleId ?? '')
}

describe('i18n lint gate', () => {
  it('flags a copy string rendered as JSX text', async () => {
    expect(await lint('export const P = () => <p>Save changes</p>\n')).toContain(
      'react/jsx-no-literals'
    )
  })

  it('flags a literal aria-label', async () => {
    expect(await lint('export const B = () => <button aria-label="Close" />\n')).toContain(
      'no-restricted-syntax'
    )
  })

  it('passes a className, which is machine input rather than copy', async () => {
    expect(await lint('export const P = () => <p className="text-sm">{copy}</p>\n')).toEqual([])
  })

  it('passes a stylesheet written as styled-jsx <style> children', async () => {
    // styled-jsx requires the CSS to be an inline template literal child: hoisting
    // it to a constant makes the transform skip it, so the CSS ships unscoped and
    // unminified. That was measured against a real `next build` — the keyframes
    // came through verbatim instead of minified. So the gate has to accept this
    // shape, and a <style> element's children are a stylesheet by definition.
    const source =
      'export const P = () => (\n  <style jsx global>{`\n    .a { color: red; }\n  `}</style>\n)\n'
    expect(await lint(source)).toEqual([])
  })
})

describe('i18n lint gate configuration', () => {
  const config = readFileSync(CONFIG_PATH, 'utf8')

  it('ignores inline eslint comments, so the suppressions file is the only escape hatch', () => {
    expect(config).toContain('noInlineConfig: true')
  })
})
