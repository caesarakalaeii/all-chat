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
 * ThemeShowcaseSection — the theme preview carousel section between setup
 * and FAQ. The carousel itself (ThemeSwitcher) is untouched: it carries its
 * own heading, copy and accessibility semantics. The section wrapper adds the
 * lanes vocabulary (mono label, panel chrome) via the .theme-showcase class
 * in globals.css.
 */

'use client'

import { ThemeSwitcher } from '@/components/ThemeSwitcher'

export function ThemeShowcaseSection() {
  return (
    <section className="lanes-section theme-showcase" data-reveal>
      <ThemeSwitcher />
    </section>
  )
}
