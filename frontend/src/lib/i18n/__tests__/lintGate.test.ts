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

  it('passes a not-copy value referenced through a named constant', async () => {
    // The route every not-copy value in this sweep takes: a wire field name, a
    // brand mark, a chat command or a code sample is hoisted to a named module
    // constant with a comment saying why, and the render site reads the
    // constant. Both /docs pages need this for the ~51 API identifiers they
    // name in <Code> and the JSON/JavaScript/Python samples in <Pre>:
    // translating an identifier produces a sample that does not run, and
    // translating a field name names a field the gateway does not send.
    //
    // Two narrower fixes were tried first and BOTH fail, so do not reach for
    // them again:
    //
    //   - elementOverrides: { Pre: ..., Code: ... }. Setting the option at all
    //     registers a VariableDeclaration handler (jsx-no-literals.js:439) that
    //     calls isRequireStatement(d.init) with no null check, so ESLint crashes
    //     outright on any uninitialised `let` anywhere in the linted set. `let
    //     token: string` in src/app/chat/auth-success/page.tsx:113 is one.
    //   - Wrapping the value in an expression container, `<Code>{'x'}</Code>`.
    //     Under noStrings: true the rule reports a string literal inside a
    //     container too, so this changes nothing.
    const source =
      "const WIRE_TYPE = 'chat_message'\nexport const P = () => <Code>{WIRE_TYPE}</Code>\n"
    expect(await lint(source)).toEqual([])
  })

  it('still flags copy inside <Pre>, so it is not a blanket exemption', async () => {
    // Nothing about <Pre> is exempt: the case above passes because of what the
    // child IS, not where it sits. Prose under a <Pre> is still copy.
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
