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
