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
 * What /overlay/[id] renders for an alerts/goal/list overlay until the kind's
 * own renderer issue lands (ADR-0064). Deliberately quiet: it sits inside an
 * OBS browser source on the streamer's canvas, so it paints no background and
 * no app chrome — just enough for the streamer to recognize the URL works.
 */

import { useTranslations } from '@/lib/i18n'
import type { OverlayType } from '@/lib/types/overlay'

export function UnsupportedKindPlaceholder({ kind }: { kind: OverlayType }) {
  const t = useTranslations()

  return (
    <div className="flex min-h-screen w-full items-center justify-center bg-transparent p-4">
      <div className="text-center">
        {/* On-stream chrome, like the chat feed below it: white text reads on
            any stream background, the in-app text-text tokens would not. */}
        <h1 className="text-lg font-semibold text-white">
          {t('viewerOverlay.unsupportedKind.heading', { kind: t(`common.overlayKinds.${kind}`) })}
        </h1>
        <p className="mt-1 text-sm text-white/70">{t('viewerOverlay.unsupportedKind.body')}</p>
      </div>
    </div>
  )
}
