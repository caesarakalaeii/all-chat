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
 * Catalog lookup and placeholder interpolation.
 *
 * This module is pure: no React, no request state, no I/O. Locale selection
 * happens above it, in ./index.ts.
 */

import type { EnMessages } from './messages/en'

/** A catalog: nested objects bottoming out in strings. */
export type MessageCatalog = {
  readonly [key: string]: string | MessageCatalog
}

/**
 * Dotted paths to every string leaf of `T`.
 *
 * Must stay a union of literals. If it ever widens to `string` the
 * `@ts-expect-error` lines in __tests__/translate.test.ts become unused and
 * `tsc --noEmit` fails, which is how that regression is caught.
 */
export type MessageKeyOf<T> = {
  [K in keyof T & string]: T[K] extends string ? K : `${K}.${MessageKeyOf<T[K]>}`
}[keyof T & string]

/** Dotted paths into the English catalog — the keys every call site uses. */
export type MessageKey = MessageKeyOf<EnMessages>

export type MessageParams = Record<string, string | number>

/**
 * The call-site signature, identical for the server and client entry points so
 * that adding a locale does not touch any component.
 */
export type TFunction = (key: MessageKey, params?: MessageParams) => string

const PLACEHOLDER = /\{(\w+)\}/g

// Warnings are for developers, and a production overlay must not spend work on
// string formatting nobody reads.
function warnInDevelopment(message: string): void {
  if (process.env.NODE_ENV !== 'production') {
    console.warn(message)
  }
}

function lookup(messages: MessageCatalog, key: string): string | undefined {
  let node: string | MessageCatalog = messages
  for (const segment of key.split('.')) {
    if (typeof node === 'string') return undefined
    const next: string | MessageCatalog | undefined = node[segment]
    if (next === undefined) return undefined
    node = next
  }
  return typeof node === 'string' ? node : undefined
}

/**
 * Resolve `key` in `messages` and substitute `{name}` placeholders from `params`.
 *
 * Never throws. Overlay pages render inside OBS browser sources on live
 * broadcasts, so an exception raised from a string lookup would blank out
 * someone's stream; a visible key or a visible `{placeholder}` is strictly
 * better than a black overlay. Both cases warn outside production.
 */
export function translate<T extends MessageCatalog>(
  messages: T,
  key: MessageKeyOf<T>,
  params?: MessageParams
): string {
  const template = lookup(messages, key)
  if (template === undefined) {
    warnInDevelopment(`[i18n] Missing message for key "${key}"`)
    return key
  }

  return template.replace(PLACEHOLDER, (placeholder, name: string) => {
    const value = params?.[name]
    if (value === undefined) {
      warnInDevelopment(`[i18n] Missing param "${name}" for key "${key}"`)
      return placeholder
    }
    return String(value)
  })
}
