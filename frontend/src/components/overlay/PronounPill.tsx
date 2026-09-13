/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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

import { getPronounPillProps, shouldRenderPronounPill } from '@/lib/utils/pronounPill'

/**
 * The pronoun pill, rendered at one of the two username-adjacent sites. Both
 * overlay surfaces (live overlay and editor preview) render it before and
 * after the username; this component carries the shared render so the two
 * surfaces cannot drift apart. Render logic itself lives in
 * {@link shouldRenderPronounPill} / {@link getPronounPillProps}, which are
 * unit-tested separately.
 */
export function PronounPill({
  showPronouns,
  pronouns,
  position,
  targetPosition,
  color,
}: {
  showPronouns: boolean
  pronouns: string | undefined
  position: 'before' | 'after'
  targetPosition: 'before' | 'after'
  color: string
}) {
  if (!shouldRenderPronounPill(showPronouns, pronouns, position, targetPosition)) {
    return null
  }
  const pill = getPronounPillProps(pronouns!, color)
  return (
    <span className={pill.className} style={pill.style}>
      {pill.text}
    </span>
  )
}
