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
 * The kind picker on the overlay create surfaces (ADR-0064), shared by
 * /overlays/new and the onboarding create dialog. Controlled: the parent owns
 * the state and the request payload. Renders nothing while only one kind is
 * creatable: a single-option radio group is a question with no choice.
 */

import { CREATABLE_OVERLAY_KINDS } from '@/lib/utils/overlayKind'
import { useTranslations } from '@/lib/i18n'
import type { OverlayType } from '@/lib/types/overlay'

export function OverlayKindPicker({
  value,
  onChange,
  kinds = CREATABLE_OVERLAY_KINDS,
}: {
  value: OverlayType
  onChange: (kind: OverlayType) => void
  kinds?: readonly OverlayType[]
}) {
  const t = useTranslations()

  if (kinds.length < 2) return null

  return (
    <fieldset className="grid grid-cols-1 gap-2 sm:grid-cols-2">
      <legend className="mb-2 text-sm font-medium text-text">
        {t('overlayEditor.create.kindLabel')}
      </legend>
      {/* The two text spans sit directly under the <label> (grid rows 1 and 2,
          radio spanning both): jsx-a11y/label-has-associated-control does not
          find accessible text through a wrapper element, so the picker cannot
          use a single wrapping span for the text column. */}
      {kinds.map((kind) => (
        <label
          key={kind}
          className="grid cursor-pointer grid-cols-[auto_1fr] items-start gap-x-2 gap-y-0.5 rounded-lg border border-border bg-surface p-3 text-sm transition-colors has-checked:border-primary has-checked:bg-primary/5"
        >
          <input
            type="radio"
            name="overlay-type"
            className="col-start-1 row-span-2 mt-0.5 accent-primary"
            checked={value === kind}
            onChange={() => onChange(kind)}
          />
          <span className="col-start-2 font-medium text-text">
            {t(`common.overlayKinds.${kind}`)}
          </span>
          <span className="col-start-2 text-text-sub">
            {t(`overlayEditor.createKindDescriptions.${kind}`)}
          </span>
        </label>
      ))}
    </fieldset>
  )
}
