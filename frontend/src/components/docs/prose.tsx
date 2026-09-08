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
 * Shared docs prose primitives, used by both the user guide (/docs) and the
 * developer API reference (/docs/api). Highlighting runs server-side (these are
 * server components), so nothing but rendered HTML ships to the client.
 */

import type { ReactNode } from 'react'
import hljs from 'highlight.js/lib/core'
import javascript from 'highlight.js/lib/languages/javascript'
import python from 'highlight.js/lib/languages/python'
import json from 'highlight.js/lib/languages/json'
import css from 'highlight.js/lib/languages/css'
import { getTranslations, type MessageKey } from '@/lib/i18n'

// Register only the languages the docs actually use. Idempotent across imports.
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('python', python)
hljs.registerLanguage('json', json)
hljs.registerLanguage('css', css)

export type CodeLang = 'javascript' | 'python' | 'json' | 'css'

export function Code({ children }: { children: ReactNode }) {
  return (
    <code className="border-2 border-white bg-black px-1.5 py-0.5 text-[0.85em]">{children}</code>
  )
}

export function Pre({ children, lang }: { children: string; lang?: CodeLang }) {
  const className =
    'my-4 overflow-x-auto border-2 border-white bg-black p-4 text-sm leading-relaxed'
  if (!lang) {
    return (
      <pre className={className}>
        <code>{children}</code>
      </pre>
    )
  }
  const highlighted = hljs.highlight(children, { language: lang, ignoreIllegals: true }).value
  return (
    <pre className={className}>
      <code className="hljs" dangerouslySetInnerHTML={{ __html: highlighted }} />
    </pre>
  )
}

export interface Field {
  /** Wire field name, as the gateway sends it. Not copy, so not a key. */
  name: string
  /** Wire type. Not copy either. */
  type: string
  /** The description, which is the only translatable part of a row. */
  descKey: MessageKey
}

export function FieldTable({ rows }: { rows: readonly Field[] }) {
  // getTranslations, not useTranslations: these are Server Components.
  const t = getTranslations()
  return (
    <div className="my-4 overflow-x-auto border-2 border-white">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr>
            <th className="px-4 py-2 font-bold">{t('docs.fieldTable.columnField')}</th>
            <th className="px-4 py-2 font-bold">{t('docs.fieldTable.columnType')}</th>
            <th className="px-4 py-2 font-bold">{t('docs.fieldTable.columnDescription')}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="align-top">
              <td className="px-4 py-2">
                <span className="text-white">{r.name}</span>
              </td>
              <td className="text-dim px-4 py-2">{r.type}</td>
              <td className="text-sub px-4 py-2">{t(r.descKey)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
