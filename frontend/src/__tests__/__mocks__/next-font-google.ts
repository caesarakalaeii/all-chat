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
 * Vitest stub for next/font/google. The real loader is a Next.js build-time
 * transform and throws ("Archivo_Black is not a function") under plain Vite.
 * Pages import the shared instances from @/lib/fonts and only spread the
 * returned class names, so a shape-compatible stub is all tests need.
 */

const makeFont = () => () => ({
  className: 'mocked-font',
  variable: '--mocked-font',
  style: { fontFamily: 'mocked-font' },
})

export const Archivo_Black = makeFont()
export const Space_Mono = makeFont()
export const Barlow = makeFont()
export const DM_Mono = makeFont()
