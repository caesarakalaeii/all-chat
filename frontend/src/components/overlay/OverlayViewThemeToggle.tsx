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

import { Moon, Sun } from 'lucide-react'
import { Button } from '@/components/ui/button'

/** Controlled light/dark toggle for the observability view header. */
export function OverlayViewThemeToggle({
  light,
  onToggle,
}: {
  light: boolean
  onToggle: () => void
}) {
  return (
    <Button
      onClick={onToggle}
      variant="outline"
      size="sm"
      aria-label={light ? 'Switch to dark mode' : 'Switch to light mode'}
    >
      {light ? <Moon className="h-3.5 w-3.5" /> : <Sun className="h-3.5 w-3.5" />}
      {light ? 'Dark' : 'Light'}
    </Button>
  )
}
