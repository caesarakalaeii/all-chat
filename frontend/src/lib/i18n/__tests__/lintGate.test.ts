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

  it('passes a stylesheet injected through dangerouslySetInnerHTML', async () => {
    // How every overlay route injects global CSS. It is an attribute rather than
    // a child, so the rule does not see it.
    //
    // The gate CANNOT accept the styled-jsx `<style jsx>{`...`}</style>` shape
    // instead: react/jsx-no-literals reports a template-literal child of any
    // element, and its elementOverrides option only resolves capitalised
    // component names, so a lowercase <style> cannot be named. Hoisting the CSS
    // to a constant does not help either — styled-jsx only transforms an inline
    // literal, and a hoisted one ships unscoped and unminified, measured with a
    // real `next build`. So a styled-jsx stylesheet is a violation here by
    // design, and the fix is to inject it the way the neighbours do.
    const source =
      'export const P = () => <style dangerouslySetInnerHTML={{ __html: `.a { color: red; }` }} />\n'
    expect(await lint(source)).toEqual([])
  })

  it('passes a code sample inside <Pre>, which is a program rather than copy', async () => {
    // Both /docs pages document the WebSocket API by showing the JSON frames and
    // the JavaScript and Python that read them. That is a program: translating
    // an identifier would produce a sample that does not run. <Pre> is
    // capitalised, so unlike <style> it can be named in elementOverrides.
    const source = 'export const P = () => <Pre lang="json">{`{ "type": "ping" }`}</Pre>\n'
    expect(await lint(source)).toEqual([])
  })

  it('passes a wire field name inside <Code>', async () => {
    // <Code> holds a field name, a JSON fragment or a chat command - values that
    // cross a process boundary and must stay byte-identical.
    expect(await lint('export const P = () => <Code>overlay_id</Code>\n')).toEqual([])
  })

  it('still flags copy in an element nested inside <Pre>', async () => {
    // applyToNestedElements defaults to true, so the override would otherwise
    // exempt a whole subtree. Prose that happens to sit under a <Pre> is still
    // copy, and this is the case that would let it through unnoticed.
    expect(await lint('export const P = () => <Pre><p>Save changes</p></Pre>\n')).toContain(
      'react/jsx-no-literals'
    )
  })
})

describe('i18n lint gate configuration', () => {
  const config = readFileSync(CONFIG_PATH, 'utf8')

  it('ignores inline eslint comments, so the suppressions file is the only escape hatch', () => {
    expect(config).toContain('noInlineConfig: true')
  })
})
