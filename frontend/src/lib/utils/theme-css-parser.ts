import type { VisualSettings } from '@/lib/types/visual-settings'
import { PROPERTY_MAP } from '@/lib/utils/visual-settings-to-css'

/**
 * Reverse map from CSS custom property name to VisualSettings field name.
 * Built once at module load from PROPERTY_MAP — Map is never mutated.
 */
const REVERSE_MAP = new Map<string, keyof VisualSettings>(
  PROPERTY_MAP.map(([field, cssVar]) => [cssVar, field])
)

/**
 * Parses a CSS string and returns a Partial<VisualSettings> by reverse-mapping
 * --chat-* and --platform-* custom property declarations to VisualSettings fields.
 *
 * - Known CSS vars are mapped to their VisualSettings field with trimmed value
 * - Unknown CSS vars produce a console.warn and are excluded from the result
 * - Empty input returns {}
 * - CSS_VAR_REGEX is defined inside the function to avoid stale lastIndex across calls
 */
export function parseCssToVisualSettings(css: string): Partial<VisualSettings> {
  const result: Partial<VisualSettings> = {}

  // Define regex INSIDE the function body — fresh instance per call avoids stale lastIndex
  const CSS_VAR_REGEX = /(--(chat|platform)-[\w-]+)\s*:\s*([^;}\n]+?)\s*;/g

  let match: RegExpExecArray | null
  while ((match = CSS_VAR_REGEX.exec(css)) !== null) {
    const cssVar = match[1]
    const value = match[3]

    const field = REVERSE_MAP.get(cssVar)
    if (field !== undefined) {
      ;(result as Record<string, string>)[field] = value
    } else {
      console.warn('[theme-css-parser] Unknown CSS variable: ' + cssVar)
    }
  }

  return result
}
