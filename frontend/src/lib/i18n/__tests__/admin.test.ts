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
 * Copy lock for the admin console.
 *
 * The migration's one hard rule is that copy moves byte-identically: no
 * rewording, no re-capitalising, no normalised punctuation. A rendered-output
 * diff across 229 files is not reviewable, so the strings that were at the
 * render sites are pinned here instead, transcribed from the pre-migration
 * source. If a key's text drifts, this fails and names the key.
 *
 * Values built from a placeholder are asserted through `t()` so the
 * interpolation is covered too, not just the template.
 */

import { describe, expect, it } from 'vitest'

import { getTranslations } from '@/lib/i18n'

const t = getTranslations()

describe('admin navigation copy', () => {
  it('keeps the rail and grid link labels', () => {
    expect(t('admin.nav.dashboardLabel')).toBe('Dashboard')
    expect(t('admin.nav.searchLabel')).toBe('Search')
    expect(t('admin.nav.usersLabel')).toBe('Users')
    expect(t('admin.nav.overlaysLabel')).toBe('Overlays')
    expect(t('admin.nav.sourcesLabel')).toBe('Sources')
    expect(t('admin.nav.viewersLabel')).toBe('Viewers')
    expect(t('admin.nav.cosmeticsLabel')).toBe('Cosmetics')
    expect(t('admin.nav.featuresLabel')).toBe('Features')
    expect(t('admin.nav.maintenanceLabel')).toBe('Maintenance')
  })

  it('keeps the dashboard grid descriptions', () => {
    expect(t('admin.nav.searchDescription')).toBe('Find any user, overlay, source, or viewer')
    expect(t('admin.nav.usersDescription')).toBe('View and manage users')
    expect(t('admin.nav.overlaysDescription')).toBe('Overlays and their owners')
    expect(t('admin.nav.sourcesDescription')).toBe('Every chat source')
    expect(t('admin.nav.viewersDescription')).toBe('Viewer sessions and bans')
    expect(t('admin.nav.cosmeticsDescription')).toBe('Avatar frames and flairs')
    expect(t('admin.nav.featuresDescription')).toBe('Premium feature gates')
    expect(t('admin.nav.maintenanceDescription')).toBe('Maintenance mode and ops')
  })

  it('keeps the sidebar chrome', () => {
    expect(t('admin.sidebar.brandSuffix')).toBe('Admin')
    expect(t('admin.sidebar.backToApp')).toBe('Back to app')
    expect(t('admin.sidebar.logOut')).toBe('Log out')
    expect(t('admin.sidebar.openMenuLabel')).toBe('Open admin menu')
    expect(t('admin.sidebar.closeMenuLabel')).toBe('Close admin menu')
    expect(t('admin.sidebar.menuLabel')).toBe('Admin menu')
  })
})

describe('admin dashboard copy', () => {
  it('keeps the page and section headings', () => {
    expect(t('admin.dashboard.heading')).toBe('Admin Dashboard')
    expect(t('admin.dashboard.activeUsersHeading')).toBe('Active users')
    expect(t('admin.dashboard.activeUsersBody')).toBe(
      'Distinct users with at least one overlay connected in the window (excludes banned users).'
    )
    expect(t('admin.dashboard.sourcesByPlatformHeading')).toBe('Sources by platform')
    expect(t('admin.dashboard.manageHeading')).toBe('Manage')
  })

  it('keeps the stat card labels', () => {
    expect(t('admin.dashboard.totalUsers')).toBe('Total Users')
    expect(t('admin.dashboard.bannedUsers')).toBe('Banned Users')
    expect(t('admin.dashboard.activeOverlays')).toBe('Active Overlays')
    expect(t('admin.dashboard.totalSources')).toBe('Total Sources')
    expect(t('admin.dashboard.last24Hours')).toBe('Last 24 hours')
    expect(t('admin.dashboard.last7Days')).toBe('Last 7 days')
    expect(t('admin.dashboard.last30Days')).toBe('Last 30 days')
  })
})
