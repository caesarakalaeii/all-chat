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

// Register only the languages the docs actually use. Idempotent across imports.
hljs.registerLanguage('javascript', javascript)
hljs.registerLanguage('python', python)
hljs.registerLanguage('json', json)
hljs.registerLanguage('css', css)

export type CodeLang = 'javascript' | 'python' | 'json' | 'css'

export function Code({ children }: { children: ReactNode }) {
  return (
    <code className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[0.85em] text-text">
      {children}
    </code>
  )
}

export function Pre({ children, lang }: { children: string; lang?: CodeLang }) {
  const className =
    'my-4 overflow-x-auto rounded-lg border border-border bg-surface-2 p-4 text-sm leading-relaxed text-text-sub'
  if (!lang) {
    return (
      <pre className={className}>
        <code className="font-mono">{children}</code>
      </pre>
    )
  }
  const highlighted = hljs.highlight(children, { language: lang, ignoreIllegals: true }).value
  return (
    <pre className={className}>
      <code className="hljs font-mono" dangerouslySetInnerHTML={{ __html: highlighted }} />
    </pre>
  )
}

export interface Field {
  name: string
  type: string
  desc: string
}

export function FieldTable({ rows }: { rows: Field[] }) {
  return (
    <div className="my-4 overflow-x-auto rounded-lg border border-border">
      <table className="w-full border-collapse text-left text-sm">
        <thead>
          <tr className="bg-surface-2 text-text">
            <th className="px-4 py-2 font-semibold">Field</th>
            <th className="px-4 py-2 font-semibold">Type</th>
            <th className="px-4 py-2 font-semibold">Description</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name} className="border-t border-border align-top">
              <td className="px-4 py-2">
                <span className="font-mono text-text">{r.name}</span>
              </td>
              <td className="px-4 py-2 font-mono text-text-dim">{r.type}</td>
              <td className="px-4 py-2 text-text-sub">{r.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
