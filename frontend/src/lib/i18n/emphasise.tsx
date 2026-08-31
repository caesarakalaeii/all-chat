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

import type React from 'react'

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
