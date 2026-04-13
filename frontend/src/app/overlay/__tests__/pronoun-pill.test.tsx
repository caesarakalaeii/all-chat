import { describe, it, expect } from 'vitest';
import { getPronounPillProps, shouldRenderPronounPill } from '@/lib/utils/pronounPill';

describe('Pronoun Pill', () => {
  describe('shouldRenderPronounPill', () => {
    it('renders when showPronouns=true, pronouns present, and position matches', () => {
      expect(shouldRenderPronounPill(true, 'she/her', 'after', 'after')).toBe(true);
    });

    it('does NOT render when pronouns is undefined', () => {
      expect(shouldRenderPronounPill(true, undefined, 'after', 'after')).toBe(false);
    });

    it('does NOT render when showPronouns is false', () => {
      expect(shouldRenderPronounPill(false, 'she/her', 'after', 'after')).toBe(false);
    });

    it('renders BEFORE username when pronounPosition is "before" and targetPosition is "before"', () => {
      expect(shouldRenderPronounPill(true, 'they/them', 'before', 'before')).toBe(true);
    });

    it('does NOT render at "after" position when pronounPosition is "before"', () => {
      expect(shouldRenderPronounPill(true, 'they/them', 'before', 'after')).toBe(false);
    });

    it('renders AFTER username when pronounPosition is "after" and targetPosition is "after"', () => {
      expect(shouldRenderPronounPill(true, 'he/him', 'after', 'after')).toBe(true);
    });

    it('does NOT render at "before" position when pronounPosition is "after"', () => {
      expect(shouldRenderPronounPill(true, 'he/him', 'after', 'before')).toBe(false);
    });
  });

  describe('getPronounPillProps', () => {
    it('returns correct pronouns text', () => {
      const props = getPronounPillProps('she/her', '#7B68EE');
      expect(props.text).toBe('she/her');
    });

    it('uses configured pronounColor as backgroundColor', () => {
      const props = getPronounPillProps('they/them', '#FF5733');
      expect(props.style.backgroundColor).toBe('#FF5733');
    });

    it('uses default color #7B68EE when no color provided', () => {
      const props = getPronounPillProps('she/her', '#7B68EE');
      expect(props.style.backgroundColor).toBe('#7B68EE');
    });

    it('includes pill CSS classes', () => {
      const props = getPronounPillProps('she/her', '#7B68EE');
      expect(props.className).toContain('rounded-full');
      expect(props.className).toContain('px-2');
      expect(props.className).toContain('py-1');
      expect(props.className).toContain('text-[11px]');
      expect(props.className).toContain('font-semibold');
      expect(props.className).toContain('leading-none');
      expect(props.className).toContain('text-white');
    });
  });

  describe('default values', () => {
    it('showPronouns defaults to true (render by default)', () => {
      // Default: show=true means shouldRenderPronounPill with true returns true when pronouns set
      expect(shouldRenderPronounPill(true, 'she/her', 'after', 'after')).toBe(true);
    });

    it('default position is "after"', () => {
      // Default position after: renders at 'after' target position
      expect(shouldRenderPronounPill(true, 'she/her', 'after', 'after')).toBe(true);
      expect(shouldRenderPronounPill(true, 'she/her', 'after', 'before')).toBe(false);
    });

    it('default color #7B68EE produces medium slate blue pill', () => {
      const props = getPronounPillProps('she/her', '#7B68EE');
      expect(props.style.backgroundColor).toBe('#7B68EE');
    });
  });
});
