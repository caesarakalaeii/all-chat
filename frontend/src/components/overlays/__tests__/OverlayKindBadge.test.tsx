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

// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest'
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'

import { OverlayKindBadge } from '@/components/overlays/OverlayKindBadge'

afterEach(() => cleanup())

describe('OverlayKindBadge', () => {
  it('labels a non-chat kind', () => {
    render(<OverlayKindBadge kind="alerts" />)
    expect(screen.getByText('Alerts')).toBeVisible()
  })

  it('renders nothing for a chat overlay', () => {
    // Every chat overlay must keep today's dashboard card byte-identical, so
    // the badge cannot leave an empty span behind.
    const { container } = render(<OverlayKindBadge kind="chat" />)
    expect(container).toBeEmptyDOMElement()
  })
})
