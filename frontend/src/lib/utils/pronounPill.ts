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
 * Pronoun Pill Utilities — Phase 9
 *
 * Helper functions for pronoun pill rendering in the overlay page.
 * Extracted for testability — pure functions with no side effects.
 */

/**
 * Determines whether a pronoun pill should render at a given position.
 *
 * @param showPronouns  - Whether pronoun display is enabled (overlay config)
 * @param pronouns      - The pronouns string (e.g. "she/her") or undefined
 * @param position      - Configured position: 'before' | 'after' username
 * @param targetPosition - Which render site is calling: 'before' | 'after' username
 * @returns true if the pill should render at this site
 */
export function shouldRenderPronounPill(
  showPronouns: boolean,
  pronouns: string | undefined,
  position: 'before' | 'after',
  targetPosition: 'before' | 'after',
): boolean {
  return showPronouns && pronouns !== undefined && pronouns !== '' && position === targetPosition;
}

/**
 * Returns props for rendering a pronoun pill <span>.
 *
 * @param pronouns   - The pronouns text to display (e.g. "she/her")
 * @param color      - Background color for the pill (e.g. "#7B68EE")
 */
export function getPronounPillProps(
  pronouns: string,
  color: string,
): {
  text: string;
  className: string;
  style: { backgroundColor: string };
} {
  return {
    text: pronouns,
    className:
      'inline-flex items-center rounded-full px-2 py-1 text-[11px] font-semibold leading-none text-white',
    style: { backgroundColor: color },
  };
}
