'use client'

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

import { useSyncExternalStore } from 'react'
import { getActiveLocale, subscribeToLocale } from './active-locale'
import { DEFAULT_LOCALE, type Locale } from './config'
import { activeTranslator } from './translators'
import type { TFunction } from './translate'

/** The active UI locale, re-rendering on a switch. DEFAULT_LOCALE while hydrating. */
export function useActiveLocale(): Locale {
  return useSyncExternalStore(subscribeToLocale, getActiveLocale, () => DEFAULT_LOCALE)
}

/**
 * For Client Components.
 *
 * Subscribes to the module-level active locale rather than a Context: the
 * frontend has no React Context anywhere, and Storybook renders components
 * standalone under a required CI gate, so a component needing a wrapping
 * provider would fail to render there. A locale switch re-renders every
 * subscriber.
 *
 * The returned function is the same object for the life of the page, across
 * switches too, and resolves the active locale on each call. A German user's
 * stored choice is applied right after hydration, so if `t` changed identity
 * then, every effect listing it would run twice on every page load, and some of
 * those are one-shot (the OAuth callback exchanges a single-use code).
 * The cost: a useMemo keyed only on `t` keeps the copy it computed first.
 *
 * Its own 'use client' module because the barrel is also imported by Server
 * Components, where a React hook import is a build error.
 */
export function useTranslations(): TFunction {
  useActiveLocale()
  return activeTranslator
}
