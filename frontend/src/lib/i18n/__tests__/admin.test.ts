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

describe('admin maintenance copy', () => {
  it('keeps the page header and empty state', () => {
    expect(t('admin.maintenance.heading')).toBe('Maintenance')
    expect(t('admin.maintenance.intro')).toBe(
      'Schedule planned downtime windows. Users see a banner on the dashboard for upcoming and active maintenance.'
    )
    expect(t('admin.maintenance.scheduleButton')).toBe('Schedule')
    expect(t('admin.maintenance.emptyTitle')).toBe('No maintenance windows scheduled')
    expect(t('admin.maintenance.emptyBody')).toBe(
      'Schedule a maintenance window to notify users of upcoming downtime.'
    )
  })

  it('keeps the window list labels', () => {
    // The count was a JSX expression between a literal '(' and ')', so the
    // parenthesis has to travel with the copy: a language that brackets
    // differently cannot reorder a bare '(' left at the render site.
    expect(t('admin.maintenance.listHeading', { count: 3 })).toBe('Scheduled Windows (3)')
    expect(t('admin.maintenance.statusActive')).toBe('Active')
    expect(t('admin.maintenance.statusUpcoming')).toBe('Upcoming')
    expect(t('admin.maintenance.deleteLabel', { title: 'Database maintenance' })).toBe(
      'Delete Database maintenance'
    )
    expect(t('admin.maintenance.deleteConfirm')).toBe('Delete this maintenance window?')
  })

  it('keeps the schedule dialog', () => {
    expect(t('admin.maintenance.dialogTitle')).toBe('Schedule Maintenance')
    expect(t('admin.maintenance.dialogBody')).toBe(
      'Create a maintenance window. Users will see a banner on the dashboard until the window ends.'
    )
    expect(t('admin.maintenance.titleLabel')).toBe('Title')
    expect(t('admin.maintenance.titlePlaceholder')).toBe('e.g. Database maintenance')
    expect(t('admin.maintenance.descriptionLabel')).toBe('Description')
    expect(t('admin.maintenance.descriptionPlaceholder')).toBe(
      'Optional details about the maintenance'
    )
    expect(t('admin.maintenance.startsAtLabel')).toBe('Starts at')
    expect(t('admin.maintenance.endsAtLabel')).toBe('Ends at')
    expect(t('admin.maintenance.cancelButton')).toBe('Cancel')
    expect(t('admin.maintenance.submittingButton')).toBe('Scheduling…')
  })
})

describe('admin feature gates copy', () => {
  it('keeps the page header, error and empty state', () => {
    expect(t('admin.features.heading')).toBe('Features')
    expect(t('admin.features.intro')).toBe(
      'Manage capability-level gates. Premium controls paid access; early access restricts a feature to beta testers. Both toggle without a code deploy.'
    )
    expect(t('admin.features.loadError')).toBe(
      'Failed to load feature gates. Refresh the page to try again.'
    )
    expect(t('admin.features.emptyTitle')).toBe('No feature gates configured')
    expect(t('admin.features.emptyBody')).toBe(
      'Feature gates are added automatically when new features ship. Check back after the next deployment.'
    )
  })

  it('keeps the gate list badges and switch labels', () => {
    expect(t('admin.features.listHeading', { count: 7 })).toBe('Feature Gates (7)')
    expect(t('admin.features.badgePremiumOnly')).toBe('Premium only')
    expect(t('admin.features.badgeFreeForAll')).toBe('Free for all')
    expect(t('admin.features.badgeEarlyAccess')).toBe('Early access')
    expect(t('admin.features.badgeStandard')).toBe('Standard')
    expect(t('admin.features.togglePremiumLabel', { feature: 'credits_roll' })).toBe(
      'Toggle premium for credits_roll'
    )
    expect(t('admin.features.toggleEarlyAccessLabel', { feature: 'credits_roll' })).toBe(
      'Toggle early access for credits_roll'
    )
  })

  it('keeps all four confirmation dialogs whole', () => {
    // Two orthogonal dimensions, each with two directions, so four dialogs. Each
    // is a whole title rather than a stem plus a direction fragment: the feature
    // key sits mid-sentence and word order is the first thing a second language
    // changes.
    expect(t('admin.features.makeFreeTitle', { feature: 'credits_roll' })).toBe(
      'Make credits_roll free for all users?'
    )
    expect(t('admin.features.makeFreeBody')).toBe(
      'All authenticated users will gain access immediately. No code deploy required.'
    )
    expect(t('admin.features.makeFreeConfirm')).toBe('Make Free')

    expect(t('admin.features.makePremiumTitle', { feature: 'credits_roll' })).toBe(
      'Restrict credits_roll to premium users?'
    )
    expect(t('admin.features.makePremiumBody')).toBe(
      'Only users with premium access will be able to use this feature.'
    )
    expect(t('admin.features.makePremiumConfirm')).toBe('Make Premium')

    expect(t('admin.features.graduateTitle', { feature: 'credits_roll' })).toBe(
      'Graduate credits_roll from early access?'
    )
    expect(t('admin.features.graduateBody')).toBe(
      'Beta-tester-only access is lifted; the feature defers to its premium gate.'
    )
    expect(t('admin.features.graduateConfirm')).toBe('Graduate')

    expect(t('admin.features.makeEarlyAccessTitle', { feature: 'credits_roll' })).toBe(
      'Restrict credits_roll to beta testers?'
    )
    expect(t('admin.features.makeEarlyAccessBody')).toBe(
      'Only beta testers will be able to use this early-access feature.'
    )
    expect(t('admin.features.makeEarlyAccessConfirm')).toBe('Make Early Access')

    expect(t('admin.features.dialogCancel')).toBe('No, keep as-is')
  })
})
