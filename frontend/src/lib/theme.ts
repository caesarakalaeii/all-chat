/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
 * The site's light/dark theme (ADR-0087). Dark is the default; a visitor's
 * choice is kept in this browser and applied as data-theme="light" on <html>,
 * which switches the token values in globals.css.
 *
 * THEME_INIT_SCRIPT applies a stored choice before the first paint (no flash
 * of dark), and components/ThemeSync.tsx keeps the attribute right across
 * client-side navigation. A module singleton like the UI locale
 * (lib/i18n/active-locale.ts), for the same reasons.
 */

export type Theme = 'dark' | 'light'

export const THEME_STORAGE_KEY = 'allchat_theme'

// OBS browser sources (/overlay/*) and the editor's preview iframes of one
// (/overlays/[id]/preview/*) are broadcast art: they render as the streamer
// styled them, whatever this browser picked for the site.
const UNTHEMED_PATHS = /^\/(?:overlay\/|overlays\/[^/]+\/preview\/)/

export function isThemedPath(pathname: string): boolean {
  return !UNTHEMED_PATHS.test(pathname)
}

const listeners = new Set<() => void>()
let current: Theme | null = null

function readStored(): Theme {
  try {
    return localStorage.getItem(THEME_STORAGE_KEY) === 'light' ? 'light' : 'dark'
  } catch {
    // Storage blocked (private mode, sandboxed frame): the default applies.
    return 'dark'
  }
}

/** This browser's choice, whether or not the current page shows it. */
export function getTheme(): Theme {
  current ??= readStored()
  return current
}

export function subscribeToTheme(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Show the chosen theme on `pathname`, or none on an unthemed route. */
export function syncTheme(pathname: string): void {
  const root = document.documentElement
  if (getTheme() === 'light' && isThemedPath(pathname)) root.dataset.theme = 'light'
  else delete root.dataset.theme
}

/** Switch the theme and remember the choice for this browser. */
export function chooseTheme(theme: Theme): void {
  current = theme
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme)
  } catch {
    // Blocked storage: the switch still applies to this tab, it just is not remembered.
  }
  syncTheme(window.location.pathname)
  for (const listener of listeners) listener()
}

/**
 * Inline <head> script: applies a stored light choice before the first paint.
 * Built from the same key and route rule as the functions above, so the two
 * cannot drift.
 */
export const THEME_INIT_SCRIPT = `try{if(localStorage.getItem(${JSON.stringify(
  THEME_STORAGE_KEY
)})==='light'&&!${UNTHEMED_PATHS}.test(location.pathname))document.documentElement.dataset.theme='light'}catch(e){}`
