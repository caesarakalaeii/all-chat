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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import UpgradePage from '@/app/upgrade/page'
import { getTranslations } from '@/lib/i18n'

vi.mock('@/components/AppNav', () => ({ AppNav: () => null }))

const t = getTranslations()

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

// The rows for the beta-only overlay kinds and features, and for alerts, released to every host (lib/platform-availability.ts).
const betaOnlyStems = ['goalOverlay', 'listOverlay', 'chatCommands'] as const
const alertsStems = ['alertsOverlay', 'alertsEditor'] as const

describe('UpgradePage premium feature list', () => {
  it('lists the official YouTube API mode', () => {
    render(<UpgradePage />)

    expect(
      screen.getByRole('heading', { name: t('marketing.upgrade.youtubeOfficialApiTitle') })
    ).toBeInTheDocument()
    expect(screen.getByText(t('marketing.upgrade.youtubeOfficialApiBody'))).toBeInTheDocument()
  })

  it('lists the alerts overlay outside the beta host', () => {
    render(<UpgradePage />)

    for (const stem of alertsStems) {
      expect(
        screen.getByRole('heading', { name: t(`marketing.upgrade.${stem}Title`) })
      ).toBeInTheDocument()
    }
  })

  it('omits the goal and list overlays and chat commands outside the beta host', () => {
    render(<UpgradePage />)

    for (const stem of betaOnlyStems) {
      expect(
        screen.queryByRole('heading', { name: t(`marketing.upgrade.${stem}Title`) })
      ).not.toBeInTheDocument()
    }
  })

  it('lists the goal and list overlays and chat commands on beta.allch.at', () => {
    vi.spyOn(window, 'location', 'get').mockReturnValue({
      ...window.location,
      hostname: 'beta.allch.at',
    })
    render(<UpgradePage />)

    for (const stem of betaOnlyStems) {
      expect(
        screen.getByRole('heading', { name: t(`marketing.upgrade.${stem}Title`) })
      ).toBeInTheDocument()
      expect(screen.getByText(t(`marketing.upgrade.${stem}Body`))).toBeInTheDocument()
    }
  })
})
