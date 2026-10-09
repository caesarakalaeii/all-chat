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

'use client'

/**
 * Kind badge for the dashboard's overlay cards (ADR-0064).
 *
 * Renders nothing for chat overlays: they are the overwhelming majority and
 * their cards must stay byte-identical to how they looked before kinds
 * existed. The three new kinds get a badge so a streamer can tell an alerts
 * overlay apart from a chat one at a glance.
 */

import { Badge } from '@/components/ui/badge'
import { useTranslations } from '@/lib/i18n'
import type { OverlayType } from '@/lib/types/overlay'

export function OverlayKindBadge({ kind }: { kind: OverlayType }) {
  const t = useTranslations()

  if (kind === 'chat') return null

  return <Badge size="sm">{t(`common.overlayKinds.${kind}`)}</Badge>
}
