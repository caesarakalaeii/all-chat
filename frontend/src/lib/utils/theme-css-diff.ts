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
 * Semantic theme-CSS diff (ADR-0043).
 *
 * The Advanced editor shows the FULL effective CSS (bundled theme merged with the
 * user's edits), but we only want to persist the user's *changes* so that a fix we
 * later ship to the bundled theme still reaches the overlay for every rule the user
 * didn't touch. This module computes that diff at save time and reconstructs the
 * full editable CSS at load time.
 *
 * Storage modes (encoded as a leading marker comment in `custom_css`; the overlay
 * render is uniform — bundled theme first, then `custom_css` last with the delta's
 * `!important` winning, so the marker + mode are only used by the editor):
 *  - 'linked' → editor equals the theme; `custom_css` empty; overlay fully tracks the bundle.
 *  - 'diff'   → only changed/added declarations stored; untouched theme rules keep updating.
 *  - 'fork'   → the user deleted theme declarations (which CSS layering can't express),
 *               so the full edited CSS is stored and the overlay stops auto-updating.
 */

export type CustomCssMode = 'linked' | 'diff' | 'fork'

export const DIFF_MARKER =
  '/* all-chat:theme-overrides — only your changes are stored; untouched theme rules keep updating. Edit above. */'
export const FORK_MARKER =
  '/* all-chat:theme-fork — full copy stored because theme rules were removed; this overlay will not auto-update. */'

export interface ThemeCssDiff {
  mode: CustomCssMode
  /** The exact string to persist to `custom_css` (includes the marker for diff/fork). */
  stored: string
}

type Decl = { value: string; important: boolean }
type RuleMap = Map<string, Map<string, Decl>>

interface Parsed {
  /** selector → (prop → decl); duplicate selectors collapse last-wins (cascade). */
  rules: RuleMap
  /** at-rule key (`@name params`) → normalized inner text, for keyframes/media/etc. */
  atRules: Map<string, string>
}

async function loadPostcss() {
  return (await import('postcss')).default
}

function normalizeSelector(sel: string): string {
  return sel
    .split(',')
    .map((s) => s.trim().replace(/\s+/g, ' '))
    .join(', ')
}

function parse(root: import('postcss').Root): Parsed {
  const rules: RuleMap = new Map()
  const atRules = new Map<string, string>()

  root.each((node) => {
    if (node.type === 'rule') {
      const sel = normalizeSelector(node.selector)
      const decls = rules.get(sel) ?? new Map<string, Decl>()
      node.walkDecls((d) => {
        // last-wins within a selector, matching the cascade
        decls.set(d.prop.trim().toLowerCase(), {
          value: d.value.trim(),
          important: d.important === true,
        })
      })
      rules.set(sel, decls)
    } else if (node.type === 'atrule') {
      // Key media/keyframes/font-face by name+params; @import keyed by its value.
      const key = `@${node.name} ${(node.params ?? '').trim()}`.trim()
      atRules.set(key, node.toString())
    }
  })

  return { rules, atRules }
}

function declChanged(a: Decl | undefined, b: Decl): boolean {
  return !a || a.value !== b.value || a.important !== b.important
}

/**
 * Compute what to persist for `custom_css` given the pristine bundled theme CSS
 * and the full CSS currently in the editor.
 */
export async function computeThemeCssDiff(
  pristineThemeCss: string,
  editedCss: string
): Promise<ThemeCssDiff> {
  const trimmedEdited = editedCss.trim()
  // No theme to diff against: any content is a plain override (its own "fork").
  if (!pristineThemeCss.trim()) {
    return trimmedEdited ? { mode: 'fork', stored: editedCss } : { mode: 'linked', stored: '' }
  }
  if (!trimmedEdited) return { mode: 'linked', stored: '' }

  const postcss = await loadPostcss()
  let base: Parsed
  let edit: Parsed
  try {
    base = parse(postcss.parse(pristineThemeCss))
    edit = parse(postcss.parse(editedCss))
  } catch {
    // Unparseable edit (mid-typo saved anyway) → store verbatim as a fork.
    return { mode: 'fork', stored: editedCss }
  }

  // Deletion detection: any theme declaration/rule/at-rule absent from the edit.
  // Deletions can't be expressed by layering a later rule, so they force a fork.
  for (const [sel, baseDecls] of base.rules) {
    const editDecls = edit.rules.get(sel)
    if (!editDecls) return { mode: 'fork', stored: FORK_MARKER + '\n' + editedCss }
    for (const prop of baseDecls.keys()) {
      if (!editDecls.has(prop)) return { mode: 'fork', stored: FORK_MARKER + '\n' + editedCss }
    }
  }
  for (const key of base.atRules.keys()) {
    if (!edit.atRules.has(key)) return { mode: 'fork', stored: FORK_MARKER + '\n' + editedCss }
  }

  // No deletions → emit only changed/added declarations + new/changed at-rules.
  const deltaRules: string[] = []
  for (const [sel, editDecls] of edit.rules) {
    const baseDecls = base.rules.get(sel)
    const changed: string[] = []
    for (const [prop, decl] of editDecls) {
      if (declChanged(baseDecls?.get(prop), decl)) {
        // Force !important so the delta beats the theme's own !important rules,
        // regardless of source order.
        changed.push(`  ${prop}: ${decl.value} !important;`)
      }
    }
    if (changed.length > 0) deltaRules.push(`${sel} {\n${changed.join('\n')}\n}`)
  }
  const deltaAtRules: string[] = []
  for (const [key, text] of edit.atRules) {
    if (base.atRules.get(key) !== text) deltaAtRules.push(text)
  }

  const delta = [...deltaAtRules, ...deltaRules].join('\n\n')
  if (!delta.trim()) return { mode: 'linked', stored: '' }
  return { mode: 'diff', stored: `${DIFF_MARKER}\n${delta}\n` }
}

/**
 * Inverse of {@link computeThemeCssDiff} for the editor: given the current bundled
 * theme CSS and the stored `custom_css`, return the full CSS to show in the editor
 * and its mode. In 'diff' mode the stored delta is merged into a fresh copy of the
 * (possibly updated) theme so the editor always reflects the latest theme plus the
 * user's changes.
 */
export async function reconstructEditorCss(
  pristineThemeCss: string,
  storedCustomCss: string
): Promise<{ editor: string; mode: CustomCssMode }> {
  const stored = storedCustomCss ?? ''
  if (!stored.trim()) {
    // Linked: preload the theme so the editor is not a blank box.
    return { editor: pristineThemeCss, mode: 'linked' }
  }
  if (stored.startsWith(FORK_MARKER)) {
    return { editor: stripLeadingMarker(stored, FORK_MARKER), mode: 'fork' }
  }
  if (!stored.startsWith(DIFF_MARKER)) {
    // Legacy / hand-authored custom_css (pre-ADR-0043 diff): show verbatim.
    return { editor: stored, mode: 'fork' }
  }

  const delta = stripLeadingMarker(stored, DIFF_MARKER)
  if (!pristineThemeCss.trim()) return { editor: delta, mode: 'diff' }

  const postcss = await loadPostcss()
  try {
    const themeRoot = postcss.parse(pristineThemeCss)
    const deltaRoot = postcss.parse(delta)
    deltaRoot.each((node) => {
      if (node.type === 'rule') {
        const sel = normalizeSelector(node.selector)
        const matches: import('postcss').Rule[] = []
        themeRoot.walkRules((r) => {
          if (normalizeSelector(r.selector) === sel) matches.push(r)
        })
        if (matches.length === 0) {
          themeRoot.append(node.clone())
        } else {
          // Set each delta declaration into EVERY matching theme rule so the
          // merged stylesheet renders the user's value regardless of duplicates.
          node.walkDecls((d) => {
            for (const rule of matches) {
              let found = false
              rule.walkDecls(d.prop, (existing) => {
                existing.value = d.value
                existing.important = d.important
                found = true
              })
              if (!found) rule.append({ prop: d.prop, value: d.value, important: d.important })
            }
          })
        }
      } else if (node.type === 'atrule') {
        themeRoot.append(node.clone())
      }
    })
    return { editor: themeRoot.toString(), mode: 'diff' }
  } catch {
    // If either side won't parse, fall back to showing the raw delta.
    return { editor: delta, mode: 'diff' }
  }
}

function stripLeadingMarker(stored: string, marker: string): string {
  return stored.slice(marker.length).replace(/^\r?\n/, '')
}
