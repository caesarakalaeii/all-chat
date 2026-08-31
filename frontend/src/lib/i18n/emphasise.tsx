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

import React from 'react'

/**
 * Render a catalog sentence with one run of it wrapped in an element.
 *
 * Copy that emboldens or links part of a sentence would otherwise have to be
 * split into before/emphasis/after fragments, which is exactly the
 * concatenation docs/frontend/I18N.md forbids: word order is the first thing a
 * second language changes, and a translator cannot place emphasis inside a
 * fragment they cannot see. So the catalog keeps the sentence whole, with an
 * `{emphasis}` placeholder, plus a sibling key holding the emphasised words:
 *
 *   t('ns.notice', { emphasis: t('ns.noticeEmphasis') })
 *
 * `t()` returns a string, so the element cannot be passed in as a parameter.
 * The resolved sentence is split around the emphasised words here instead.
 *
 * @param sentence The resolved sentence, emphasis already substituted in.
 * @param emphasis The emphasised words, as resolved from the sibling key.
 * @param wrap Builds the element the emphasised run is rendered inside.
 */
export function emphasise(
  sentence: string,
  emphasis: string,
  wrap: (run: string) => React.ReactNode
): React.ReactNode {
  const at = sentence.indexOf(emphasis)
  // No match means the sentence and its emphasis key drifted apart. Render the
  // sentence plainly rather than dropping it: losing the emphasis is cosmetic,
  // losing the copy leaves a blank paragraph on the page.
  if (at === -1) return sentence
  return (
    <>
      {sentence.slice(0, at)}
      {wrap(emphasis)}
      {sentence.slice(at + emphasis.length)}
    </>
  )
}

/** Matches a `{name}` placeholder, capturing the name. Mirrors translate.ts. */
const PLACEHOLDER = /\{(\w+)\}/g

/**
 * Render a catalog sentence with each named placeholder replaced by an element.
 *
 * `emphasise` covers the common case of one emphasised run. Some copy has
 * several — the engagement panel's points explainer interleaves four <code>
 * chat commands — and `emphasise` cannot be applied repeatedly for it, because
 * one placeholder's value can occur inside another's (`2` inside `!vote 2`), so
 * resolving the sentence first and searching for the values afterwards wraps
 * the wrong run.
 *
 * The split is therefore done on the UNRESOLVED template, where the
 * placeholder names are unambiguous. Pass the template straight from the
 * catalog, without calling `t()` with these parameters:
 *
 *   interpolateElements(t('ns.explainer'), { cmd: <code>!vote 2</code> })
 *
 * A placeholder with no entry in `elements` is left in the output verbatim,
 * braces included, for the same reason `t()` never throws: the reader keeps the
 * rest of the sentence, and the stray braces name the missing key.
 *
 * @param template The catalog sentence, placeholders unresolved.
 * @param elements The element to render for each placeholder name.
 */
export function interpolateElements(
  template: string,
  elements: Readonly<Record<string, React.ReactNode>>
): React.ReactNode {
  const parts: React.ReactNode[] = []
  let cursor = 0
  for (const match of template.matchAll(PLACEHOLDER)) {
    const name = match[1]
    if (!(name in elements)) continue
    parts.push(template.slice(cursor, match.index))
    // Keyed by name AND source offset, not by name alone: copy may legitimately
    // name one placeholder twice ('a {status} frame ... {status} data:'), and
    // two fragments keyed 'status' make React log a duplicate-key error. The
    // offset is stable across renders because the template is.
    parts.push(<React.Fragment key={`${name}@${match.index}`}>{elements[name]}</React.Fragment>)
    cursor = match.index + match[0].length
  }
  parts.push(template.slice(cursor))
  return <>{parts}</>
}
