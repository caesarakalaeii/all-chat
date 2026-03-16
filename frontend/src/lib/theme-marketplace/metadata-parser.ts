/**
 * CSS Theme Metadata Parser
 *
 * Parses JSDoc-style metadata from CSS theme files.
 */

import type { ThemeMetadata } from './types'

/**
 * Parse theme metadata from CSS content
 */
export function parseThemeMetadata(css: string, filename: string): ThemeMetadata {
  // Extract first JSDoc comment block (/** ... */)
  const headerRegex = /\/\*\*\s*([\s\S]*?)\s*\*\//
  const match = css.match(headerRegex)

  if (!match) {
    // Fallback to filename-based defaults
    return {
      name: filenameToTitle(filename),
      description: 'No description available',
      tags: ['uncategorized'],
    }
  }

  const headerContent = match[1]

  // Extract individual fields using regex
  const extractField = (field: string): string => {
    const regex = new RegExp(`\\*\\s*${field}:\\s*(.+)`, 'i')
    const fieldMatch = headerContent.match(regex)
    return fieldMatch ? fieldMatch[1].trim() : ''
  }

  // Parse tags (comma-separated)
  const tagsString = extractField('Tags')
  const tags = tagsString
    .split(',')
    .map((t) => t.trim().toLowerCase())
    .filter(Boolean)

  return {
    name: extractField('Theme Name') || filenameToTitle(filename),
    description: extractField('Description') || 'No description available',
    tags: tags.length > 0 ? tags : ['uncategorized'],
    author: extractField('Author') || undefined,
    version: extractField('Version') || undefined,
    updated: extractField('Updated') || undefined,
  }
}

/**
 * Convert filename to title case
 * Example: "minimal-theme.css" -> "Minimal Theme"
 */
function filenameToTitle(filename: string): string {
  return filename
    .replace('.css', '')
    .replace(/-/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}
