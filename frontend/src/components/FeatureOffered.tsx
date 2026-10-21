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

import type { ReactNode } from 'react'
import { useFeatureAvailable, type BetaFeature } from '@/lib/platform-availability'

/**
 * Renders its children only where `feature` may be offered (beta-only features
 * are hidden outside beta.allch.at). For server components, which cannot call the
 * hook themselves.
 */
export function FeatureOffered({
  feature,
  children,
}: {
  feature: BetaFeature
  children: ReactNode
}) {
  return useFeatureAvailable()(feature) ? children : null
}
