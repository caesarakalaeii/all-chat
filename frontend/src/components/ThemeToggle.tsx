'use client'

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

import { useSyncExternalStore } from 'react'
import { Moon, Sun } from 'lucide-react'
import { useTranslations } from '@/lib/i18n'
import { chooseTheme, getTheme, subscribeToTheme, type Theme } from '@/lib/theme'
import { cn } from '@/lib/utils'

const serverTheme = (): Theme => 'dark'

/**
 * The site light/dark switch (ADR-0087). Unstyled beyond its icon: each nav
 * gives it the look of its own buttons through `className`, and `withLabel`
 * shows the name next to the icon where the nav's rows are labelled. The server
 * renders it as dark; the stored choice shows after hydration.
 */
export function ThemeToggle({ className, withLabel }: { className?: string; withLabel?: boolean }) {
  const t = useTranslations()
  const theme = useSyncExternalStore(subscribeToTheme, getTheme, serverTheme)
  const light = theme === 'light'
  const label = t('common.theme.lightMode')

  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-pressed={light}
      onClick={() => chooseTheme(light ? 'dark' : 'light')}
      className={cn('theme-toggle', className)}
    >
      {light ? (
        <Moon aria-hidden="true" className="size-4" />
      ) : (
        <Sun aria-hidden="true" className="size-4" />
      )}
      {withLabel && <span className="truncate">{label}</span>}
    </button>
  )
}
