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

import React from 'react'
import { EDITOR_GROUPS, EDITOR_SECTIONS, type EditorSectionId } from './sectionRegistry'

/**
 * Breadcrumb + heading + description for the active settings section
 * (ADR-0042). One h2 per visible section keeps screen-reader heading
 * navigation equivalent to the old one-drawer-one-heading structure.
 */
export function EditorSectionHeader({ id }: { id: EditorSectionId }): React.ReactElement | null {
  const section = EDITOR_SECTIONS.find((s) => s.id === id)
  if (section === undefined) return null
  const group = EDITOR_GROUPS.find((g) => g.id === section.group)
  return (
    <div className="mb-4">
      <p className="text-[10px] font-medium tracking-widest text-text-sub uppercase">
        {group?.label}
      </p>
      <h2 className="mt-0.5 text-lg font-semibold text-text">{section.title}</h2>
      <p className="mt-0.5 text-sm text-text-sub">{section.description}</p>
    </div>
  )
}
