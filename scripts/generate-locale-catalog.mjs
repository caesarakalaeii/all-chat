#!/usr/bin/env node
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
 * Turn a localization export into frontend catalog files (ADR-0063).
 *
 * The end of the translation pipeline:
 *
 *   1. An admin approves translations in /admin/localization.
 *   2. The admin downloads the export:
 *        GET /api/v1/admin/localization/export/<locale>
 *      which returns {"locale": "de", "translations": [{key, value}, ...]}.
 *   3. This script turns that JSON into the shape locale #2 needs
 *      (docs/frontend/I18N.md, "Adding locale #2"):
 *        - frontend/src/lib/i18n/messages/<locale>/  — one file per namespace,
 *          nested the same way the English catalog is, `as const` included;
 *          keys missing from the export stay English (missing keys resolve
 *          at the English fallback, so a partial locale ships safely);
 *        - a printed TODO list of the steps the PR still needs (barrel entry,
 *          SUPPORTED_LOCALES, index mapping) — the script does NOT do them,
 *          because each is a one-line edit a reviewer should see in the diff.
 *
 * Usage:
 *   node scripts/generate-locale-catalog.mjs <export.json> [--out <dir>]
 *
 *   <export.json>    the downloaded export file
 *   --out <dir>      output directory (default: frontend/src/lib/i18n/messages)
 *
 * The generated files type-check against the English catalog's shape only
 * after the barrel wiring is done (step 1 of the printed TODO); run
 * `npx tsc --noEmit` after wiring.
 */

import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = path.dirname(fileURLToPath(import.meta.url))
const FRONTEND = path.join(HERE, '..', 'frontend')
const EN_DIR = path.join(FRONTEND, 'src', 'lib', 'i18n', 'messages', 'en')

function fail(message) {
  console.error(`error: ${message}`)
  process.exit(1)
}

// --- Args --------------------------------------------------------------------

const args = process.argv.slice(2)
let exportPath = null
let outDir = path.join(FRONTEND, 'src', 'lib', 'i18n', 'messages')
for (let i = 0; i < args.length; i++) {
  if (args[i] === '--out') {
    outDir = args[++i]
    if (!outDir) fail('--out requires a directory')
  } else if (!exportPath) {
    exportPath = args[i]
  } else {
    fail(`unexpected argument: ${args[i]}`)
  }
}
if (!exportPath) fail('usage: node scripts/generate-locale-catalog.mjs <export.json> [--out <dir>]')

// --- Load the export and the English catalog ----------------------------------

let exportData
try {
  exportData = JSON.parse(readFileSync(exportPath, 'utf8'))
} catch (err) {
  fail(`cannot read export file: ${err.message}`)
}
if (!exportData.locale || !Array.isArray(exportData.translations)) {
  fail('export file must be {"locale": "<code>", "translations": [{key, value}, ...]}')
}
if (!/^[a-z]{2,3}(-[A-Za-z]{2,4})?$/.test(exportData.locale)) {
  fail(`invalid locale code: ${exportData.locale}`)
}

const locale = exportData.locale
const translated = new Map()
for (const row of exportData.translations) {
  if (typeof row.key !== 'string' || typeof row.value !== 'string') {
    fail('every translation must have string key and value fields')
  }
  translated.set(row.key, row.value)
}

// Load the English namespace list straight from the barrel source: the
// filesystem is the source of truth, so the generator can never emit a
// namespace the catalog does not have.
const enBarrel = readFileSync(path.join(EN_DIR, 'index.ts'), 'utf8')
const namespaces = [...enBarrel.matchAll(/import \{ (\w+) \} from '\.\/(\w+)'/g)].map(
  (m) => m[2]
)
if (namespaces.length === 0) fail('could not read namespaces from en/index.ts')

// --- Split dotted keys back into a nested object per namespace ----------------

/** Insert value at a dotted path into a nested object, creating levels. */
function insertNested(tree, segments, value) {
  let node = tree
  for (let i = 0; i < segments.length - 1; i++) {
    if (typeof node[segments[i]] !== 'object' || node[segments[i]] === null) {
      node[segments[i]] = {}
    }
    node = node[segments[i]]
  }
  node[segments[segments.length - 1]] = value
}

/** Render a nested object as a TS literal with `as const`, sorted keys. */
function renderLiteral(node, indent) {
  const pad = ' '.repeat(indent)
  const lines = []
  for (const key of Object.keys(node).sort()) {
    const value = node[key]
    if (typeof value === 'string') {
      lines.push(`${pad}${key}: ${JSON.stringify(value)},`)
    } else {
      lines.push(`${pad}${key}: {`)
      lines.push(renderLiteral(value, indent + 2))
      lines.push(`${pad}},`)
    }
  }
  return lines.join('\n')
}
let usedKeys = 0
const usedKeyNames = new Set()
const localeDir = path.join(outDir, locale)
mkdirSync(localeDir, { recursive: true })

for (const ns of namespaces) {
  // Nested {key: value} for the keys of this namespace that have translations.
  const tree = {}
  for (const [key, value] of translated) {
    const segments = key.split('.')
    if (segments[0] !== ns) continue
    if (segments.length < 2) {
      // Not a namespaced catalog key; the export and the catalog have
      // drifted, and it is reported after the loop instead of being given a
      // guessed home.
      continue
    }
    insertNested(tree, segments.slice(1), value)
    usedKeys++
    usedKeyNames.add(key)
  }
  if (Object.keys(tree).length === 0) continue

  const banner = `/**
 * ${locale} translations for the ${ns} namespace. Generated from the
 * localization export (ADR-0063) — edit translations in /translate, not here.
 * Keys not present in the export are absent here and resolve to English.
 */
`
  const body = `export const ${ns} = {\n${renderLiteral(tree, 2)}\n} as const\n`
  writeFileSync(path.join(localeDir, `${ns}.ts`), banner + body)
}

// Keys in the export that matched no namespace: the export and the catalog
// have drifted, so name them instead of dropping them silently.
const skippedKeys = [...translated.keys()].filter((k) => !usedKeyNames.has(k))

// --- Report ------------------------------------------------------------------

const totalCatalogKeys = namespaces.reduce((count, ns) => {
  const source = readFileSync(path.join(EN_DIR, `${ns}.ts`), 'utf8')
  // Count string leaves: lines that are `key: '...'` at any depth. Good enough
  // for a progress report — the authoritative key set is the compiled catalog.
  return count + (source.match(/^\s*\w+: '/gm) ?? []).length
}, 0)

console.log(`Generated ${usedKeys} keys into ${localeDir}/ (${skippedKeys.length} skipped as non-namespaced)`)
console.log(`Catalog coverage: ${usedKeys}/${totalCatalogKeys} English keys (${Math.round((100 * usedKeys) / totalCatalogKeys)}%)`)
if (skippedKeys.length > 0) {
  console.log('Skipped keys:')
  for (const key of skippedKeys) console.log(`  ${key}`)
}
console.log('')
console.log('Remaining steps for the PR (deliberately NOT done by this script):')
console.log(`  1. Add \`messages/${locale}/index.ts\` composing the generated files`)
console.log(`     (copy the shape of messages/en/index.ts, import from './${locale}').`)
console.log('  2. Add the locale to SUPPORTED_LOCALES in src/lib/i18n/config.ts.')
console.log('  3. Map the locale to its catalog in src/lib/i18n/index.ts.')
console.log('  4. Run `npx tsc --noEmit` in frontend/ — the typed catalog check')
console.log('     only becomes total after step 3.')
console.log('  5. Cover the release steps in CLAUDE.md "Shipping a Feature".')
if (!existsSync(path.join(outDir, locale, 'index.ts'))) {
  console.log('')
  console.log(`Note: this run generated only the namespaces with translations.`)
  console.log(`Re-running the script with a fuller export adds the rest without`)
  console.log(`touching the existing files' keys.`)
}
