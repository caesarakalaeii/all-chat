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
 * Screen-reader-only text. Use for labels/status that must be announced but
 * not shown: spinner "Loading…" text, icon-button context, live-region
 * updates. Prefer this component over a bare `sr-only` span — it's greppable
 * and signals intent in JSX.
 */

import { cn } from '@/lib/utils'

export function VisuallyHidden({ className, ...props }: React.ComponentPropsWithoutRef<'span'>) {
  return <span data-slot="visually-hidden" className={cn('sr-only', className)} {...props} />
}
