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

describe('admin global search copy', () => {
  it('keeps the page header and search field', () => {
    expect(t('admin.search.heading')).toBe('Search')
    expect(t('admin.search.intro')).toBe(
      'Find any user, overlay, source, or viewer and jump straight to it'
    )
    expect(t('admin.search.inputPlaceholder')).toBe('Search users, overlays, sources, viewers...')
    expect(t('admin.search.inputLabel')).toBe('Global admin search')
  })

  it('keeps the three result-list states', () => {
    expect(t('admin.search.promptState')).toBe('Type at least one character to search.')
    expect(t('admin.search.loadingState')).toBe('Searching...')
    // The typographic quotes were &ldquo;/&rdquo; entities around a JSX
    // expression, so they travel with the copy: a language that quotes with
    // guillemets or corner brackets replaces the pair, and cannot if the render
    // site holds one of each.
    expect(t('admin.search.emptyState', { query: 'caesar' })).toBe(
      'Nothing matches \u201Ccaesar\u201D.'
    )
  })

  it('keeps the result group headings', () => {
    expect(t('admin.search.groupUsers')).toBe('Users')
    expect(t('admin.search.groupOverlays')).toBe('Overlays')
    expect(t('admin.search.groupSources')).toBe('Sources')
    expect(t('admin.search.groupViewers')).toBe('Viewers')
    // Two count formats, whole rather than assembled: the truncated one reads
    // "(showing 8 of 30)" and the exact one "(30)".
    expect(t('admin.search.groupCountTruncated', { shown: 8, total: 30 })).toBe('(showing 8 of 30)')
    expect(t('admin.search.groupCountExact', { total: 4 })).toBe('(4)')
  })

  it('keeps the result row labels', () => {
    expect(t('admin.search.badgePremium')).toBe('Premium')
    expect(t('admin.search.badgeBanned')).toBe('Banned')
    // The bare word 'sources' followed a JSX count expression, and the 'in'
    // preceded one. Both become whole phrases: a bare preposition or noun left
    // at the render site cannot be reordered.
    expect(t('admin.search.overlaySourceCount', { count: 3 })).toBe('3 sources')
    expect(t('admin.search.sourceInOverlay', { overlay: 'Main overlay' })).toBe('in Main overlay')
  })
})

describe('admin cosmetics catalog copy', () => {
  it('keeps the page header and tab labels', () => {
    expect(t('admin.cosmetics.heading')).toBe('Cosmetics Catalog')
    expect(t('admin.cosmetics.intro')).toBe('Manage avatar frames and flairs')
    // The tab bar rendered the wire values 'frames'/'flairs' through a CSS
    // `capitalize`, so the visible text was never in the source. It is copy now,
    // and a language whose casing rules differ from CSS's gets to choose it.
    expect(t('admin.cosmetics.tabFrames')).toBe('Frames')
    expect(t('admin.cosmetics.tabFlairs')).toBe('Flairs')
  })

  it('keeps both empty states whole', () => {
    // `itemLabel.toLowerCase()` built "No frames in catalog yet" from a
    // capitalised noun and a lowercasing call. Two whole sentences instead: the
    // noun inflects and the lowercase form is not always the visible one.
    expect(t('admin.cosmetics.emptyFrames')).toBe('No frames in catalog yet')
    expect(t('admin.cosmetics.emptyFlairs')).toBe('No flairs in catalog yet')
  })

  it('keeps the add form labels for both entry kinds', () => {
    expect(t('admin.cosmetics.addFrameHeading')).toBe('Add Frame')
    expect(t('admin.cosmetics.addFlairHeading')).toBe('Add Flair')
    expect(t('admin.cosmetics.nameLabel')).toBe('Name')
    expect(t('admin.cosmetics.framePlaceholder')).toBe('Frame name')
    expect(t('admin.cosmetics.flairPlaceholder')).toBe('Flair name')
    expect(t('admin.cosmetics.imageUrlLabel')).toBe('Image URL')
    expect(t('admin.cosmetics.imageUrlPlaceholder')).toBe('https://example.com/frame.png')
    expect(t('admin.cosmetics.previewAlt')).toBe('Preview')
    expect(t('admin.cosmetics.premiumOnlyLabel')).toBe('Premium only')
    expect(t('admin.cosmetics.submitFrame')).toBe('Add Frame')
    expect(t('admin.cosmetics.submitFlair')).toBe('Add Flair')
    expect(t('admin.cosmetics.submittingButton')).toBe('Adding…')
  })

  it('keeps the entry row badge and delete label', () => {
    expect(t('admin.cosmetics.badgePremium')).toBe('Premium')
    expect(t('admin.cosmetics.deleteLabel', { name: 'Gold Ring' })).toBe('Delete Gold Ring')
  })

  it('keeps the load, add and delete toast copy', () => {
    expect(t('admin.cosmetics.loadFramesError')).toBe('Failed to load frames')
    expect(t('admin.cosmetics.loadFlairsError')).toBe('Failed to load flairs')
    expect(t('admin.cosmetics.frameDeleted')).toBe('Frame deleted')
    expect(t('admin.cosmetics.flairDeleted')).toBe('Flair deleted')
    expect(t('admin.cosmetics.deleteError')).toBe('Delete failed')
    expect(t('admin.cosmetics.frameAdded')).toBe('Frame added')
    expect(t('admin.cosmetics.flairAdded')).toBe('Flair added')
    expect(t('admin.cosmetics.addError')).toBe('Add failed')
  })
})

describe('admin overlays page copy', () => {
  it('keeps the page header and the degraded-status warning', () => {
    expect(t('admin.overlays.heading')).toBe('Overlays')
    expect(t('admin.overlays.intro')).toBe('Manage overlays and their connected chat sources')
    expect(t('admin.overlays.loadError')).toBe('Failed to load overlays')
    // The typographic quotes were &ldquo;/&rdquo; entities and travel with the
    // copy: a language that quotes with guillemets replaces the pair.
    expect(t('admin.overlays.statusUnavailable')).toBe(
      'Live connection status is currently unavailable, so overlays may show as \u201Cnot connected\u201D even if they are live.'
    )
  })

  it('keeps the list header, search field and filter toggle', () => {
    // Two count shapes rather than a bare ' of N' fragment appended in JSX: a
    // language that words "8 of 30" differently cannot reorder a fragment the
    // render site owns.
    expect(t('admin.overlays.listHeadingAll', { count: 12 })).toBe('All Overlays (12)')
    expect(t('admin.overlays.listHeadingFiltered', { shown: 8, total: 30 })).toBe(
      'All Overlays (8 of 30)'
    )
    expect(t('admin.overlays.searchPlaceholder')).toBe('Search by overlay name, ID, or owner...')
    expect(t('admin.overlays.connectedFilter', { count: 4 })).toBe('Connected (4)')
  })

  it('keeps the two empty-list states', () => {
    expect(t('admin.overlays.emptyNone')).toBe('No overlays found.')
    expect(t('admin.overlays.emptyFiltered')).toBe('No overlays match your search or filter.')
  })

  it('keeps the row labels', () => {
    expect(t('admin.overlays.rowSourceCount', { count: 3 })).toBe('3 sources')
    expect(t('admin.overlays.rowIdPrefix', { id: 'ov_1' })).toBe('ID: ov_1')
    expect(t('admin.overlays.rowCreated', { date: '13/07/2026' })).toBe('Created 13/07/2026')
    expect(t('admin.overlays.openInNewTabLabel', { name: 'Main' })).toBe(
      'Open overlay Main in a new tab'
    )
    // Two whole titles: the dot's tooltip either names the start time or does not.
    expect(t('admin.overlays.dotConnectedSince', { timestamp: '13/07/2026, 08:48' })).toBe(
      'Connected since 13/07/2026, 08:48'
    )
    expect(t('admin.overlays.dotConnected')).toBe('Connected')
    // The elapsed label likewise: with a duration, or bare when the connection
    // start is unknown.
    expect(t('admin.overlays.connectedFor', { duration: '3h 12m' })).toBe('Connected 3h 12m')
    expect(t('admin.overlays.connected')).toBe('Connected')
  })

  it('keeps the detail panel labels', () => {
    expect(t('admin.overlays.detailHeading')).toBe('Overlay Details')
    expect(t('admin.overlays.detailName')).toBe('Name')
    expect(t('admin.overlays.detailId')).toBe('ID')
    expect(t('admin.overlays.detailOwner')).toBe('Owner')
    // The owner link was built from a handle-or-fallback plus an optional
    // ' (Display Name)' tail, so four whole link texts. A fragment starting with
    // a space is not something a translator can reorder, and the parenthesis
    // convention is language-specific.
    expect(t('admin.overlays.ownerHandle', { username: 'caesar' })).toBe('@caesar')
    expect(
      t('admin.overlays.ownerHandleNamed', { username: 'caesar', displayName: 'Caesar' })
    ).toBe('@caesar (Caesar)')
    expect(t('admin.overlays.ownerFallback')).toBe('View user')
    expect(t('admin.overlays.ownerFallbackNamed', { displayName: 'Caesar' })).toBe(
      'View user (Caesar)'
    )
    expect(t('admin.overlays.ownerUnknown')).toBe('Unknown')
    expect(t('admin.overlays.detailConnection')).toBe('Connection')
    expect(t('admin.overlays.notConnected')).toBe('Not connected')
    expect(t('admin.overlays.connectedSinceRow', { timestamp: '13/07/2026, 08:48' })).toBe(
      'Since 13/07/2026, 08:48'
    )
    expect(t('admin.overlays.selectPrompt')).toBe('Select an overlay to view details')
  })

  it('keeps the connected sources panel copy', () => {
    expect(t('admin.overlays.sourcesHeading', { count: 2 })).toBe('Connected Sources (2)')
    expect(t('admin.overlays.sourceActive')).toBe('Active')
    expect(t('admin.overlays.sourceInactive')).toBe('Inactive')
    expect(t('admin.overlays.sourceAdded', { date: '13/07/2026' })).toBe('Added 13/07/2026')
    expect(t('admin.overlays.sourcesEmpty')).toBe('No sources connected')
  })
})

describe('admin sources page copy', () => {
  it('keeps the page header and load error', () => {
    expect(t('admin.sources.heading')).toBe('Sources')
    expect(t('admin.sources.intro')).toBe('View and manage all chat sources across overlays')
    expect(t('admin.sources.loadError')).toBe('Failed to load sources')
  })

  it('keeps the owner scope banner as one sentence with the link as a param', () => {
    // The owner name sat inside the sentence as a <Link>. The whole sentence is
    // one key with an {owner} hole; interpolateElements splits the unresolved
    // template so the link can move where the language puts it.
    expect(t('admin.sources.ownerScope', { owner: 'caesar' })).toBe(
      'Showing sources owned by caesar'
    )
    expect(t('admin.sources.ownerScopeClear')).toBe('Clear')
  })

  it('keeps the filter labels and select options', () => {
    expect(t('admin.sources.searchLabel')).toBe('Search')
    expect(t('admin.sources.searchPlaceholder')).toBe('Search by channel, overlay, or owner...')
    expect(t('admin.sources.platformLabel')).toBe('Platform')
    expect(t('admin.sources.platformAll')).toBe('All Platforms')
    expect(t('admin.sources.statusLabel')).toBe('Status')
    expect(t('admin.sources.statusAll')).toBe('All Status')
    expect(t('admin.sources.statusActive')).toBe('Active')
    expect(t('admin.sources.statusInactive')).toBe('Inactive')
  })

  it('keeps the two empty-list states', () => {
    expect(t('admin.sources.emptyNone')).toBe('No sources found.')
    expect(t('admin.sources.emptyFiltered')).toBe('No sources match your filters.')
  })

  it('keeps the table heading, caption and column headers', () => {
    expect(t('admin.sources.listHeading', { count: 12 })).toBe('All Sources (12)')
    expect(t('admin.sources.tableCaption')).toBe('Chat sources')
    expect(t('admin.sources.columnPlatform')).toBe('Platform')
    expect(t('admin.sources.columnChannel')).toBe('Channel')
    expect(t('admin.sources.columnOverlay')).toBe('Overlay')
    expect(t('admin.sources.columnOwner')).toBe('Owner')
    expect(t('admin.sources.columnStatus')).toBe('Status')
    expect(t('admin.sources.columnCreated')).toBe('Created')
  })

  it('reads the four platform stat labels from the shared namespace', () => {
    // The stat cards spelled 'Twitch', 'YouTube', 'Kick' and 'TikTok' inline.
    // They are the tenth duplicate of the platform name table, so they read
    // common.platforms.* like the other nine.
    expect(t('common.platforms.twitch')).toBe('Twitch')
    expect(t('common.platforms.youtube')).toBe('YouTube')
    expect(t('common.platforms.kick')).toBe('Kick')
    expect(t('common.platforms.tiktok')).toBe('TikTok')
  })
})

describe('admin viewers page copy', () => {
  it('keeps the page header and the total count', () => {
    expect(t('admin.viewers.heading')).toBe('Viewer Management')
    expect(t('admin.viewers.intro')).toBe(
      'Search viewer sessions, inspect activity, and manage bans and premium'
    )
    // The count and the word 'matching' were siblings in JSX. One key, so a
    // language that puts the count last can.
    expect(t('admin.viewers.totalMatching', { count: '1,204' })).toBe('1,204 matching')
  })

  it('keeps the search and filter controls', () => {
    expect(t('admin.viewers.searchLabel')).toBe('Search')
    expect(t('admin.viewers.searchPlaceholder')).toBe(
      'Username, display name, or platform user ID...'
    )
    expect(t('admin.viewers.platformLabel')).toBe('Platform')
    expect(t('admin.viewers.platformAll')).toBe('All platforms')
    expect(t('admin.viewers.statusLabel')).toBe('Status')
    expect(t('admin.viewers.statusAny')).toBe('Any')
    expect(t('admin.viewers.statusActive')).toBe('Active')
    expect(t('admin.viewers.statusBanned')).toBe('Banned')
    expect(t('admin.viewers.premiumLabel')).toBe('Premium')
    expect(t('admin.viewers.premiumAny')).toBe('Any')
    expect(t('admin.viewers.premiumOnly')).toBe('Premium')
    expect(t('admin.viewers.premiumFree')).toBe('Free')
  })

  it('keeps the table caption, column headers and empty state', () => {
    expect(t('admin.viewers.empty')).toBe('No viewer sessions match your search or filters.')
    expect(t('admin.viewers.tableCaption')).toBe('Viewers')
    expect(t('admin.viewers.columnViewer')).toBe('Viewer')
    expect(t('admin.viewers.columnPlatform')).toBe('Platform')
    expect(t('admin.viewers.columnLastMessage')).toBe('Last Message')
    expect(t('admin.viewers.columnPremium')).toBe('Premium')
    expect(t('admin.viewers.columnStatus')).toBe('Status')
    expect(t('admin.viewers.columnActions')).toBe('Actions')
  })

  it('keeps the row badges, controls and their aria labels', () => {
    expect(t('admin.viewers.neverMessaged')).toBe('Never')
    expect(t('admin.viewers.badgeBanned')).toBe('BANNED')
    expect(t('admin.viewers.badgeActive')).toBe('Active')
    expect(t('admin.viewers.banReason', { reason: 'spam' })).toBe('Reason: spam')
    expect(t('admin.viewers.sessionOnlyTitle')).toBe('Session-only viewer (no linked account)')
    expect(t('admin.viewers.premiumBadge')).toBe('Premium')
    expect(t('admin.viewers.freeBadge')).toBe('Free')
    // Two whole aria labels: the badge word sits in front of the sentence, and
    // a language that fronts the verb cannot move a fragment the render site
    // assembled.
    expect(t('admin.viewers.changePremiumPremiumLabel', { username: 'kate' })).toBe(
      'Premium: change premium status for kate'
    )
    expect(t('admin.viewers.changePremiumFreeLabel', { username: 'kate' })).toBe(
      'Free: change premium status for kate'
    )
    expect(t('admin.viewers.activityButton')).toBe('Activity')
    expect(t('admin.viewers.unbanButton')).toBe('Unban')
    expect(t('admin.viewers.unbanningButton')).toBe('Unbanning...')
    expect(t('admin.viewers.unbanLabel', { username: 'kate' })).toBe('Unban kate')
    expect(t('admin.viewers.unbanningLabel', { username: 'kate' })).toBe('Unbanning kate')
    expect(t('admin.viewers.banButton')).toBe('Ban')
    expect(t('admin.viewers.banLabel', { username: 'kate' })).toBe('Ban kate')
  })

  it('keeps the pagination copy', () => {
    // The range was three formatted numbers and two separators interleaved in
    // JSX. One sentence: a language that words a range differently now can.
    expect(t('admin.viewers.pageRange', { start: '1', end: '50', total: '1,204' })).toBe(
      'Showing 1\u201350 of 1,204'
    )
    expect(t('admin.viewers.previousPage')).toBe('Previous')
    expect(t('admin.viewers.nextPage')).toBe('Next')
  })

  it('keeps the activity dialog copy', () => {
    // The typographic quotes were entities either side of the username.
    expect(t('admin.viewers.activityTitle', { username: 'kate' })).toBe(
      'Activity for \u201Ckate\u201D'
    )
    expect(t('admin.viewers.activityDescription')).toBe(
      'Messages this viewer has sent through All-Chat, and whose chats they appear in.'
    )
    expect(t('admin.viewers.activityTotalMessages')).toBe('Total messages')
    expect(t('admin.viewers.activityLastMessage')).toBe('Last message')
    expect(t('admin.viewers.activityStreamerFallback')).toBe('View streamer')
    expect(t('admin.viewers.activityOverlayLink')).toBe('overlay')
    expect(t('admin.viewers.activityEmpty')).toBe('No message activity recorded for this viewer.')
    expect(t('admin.viewers.activityClose')).toBe('Close')
  })

  it('keeps the premium dialog copy', () => {
    // Two whole titles rather than a 'Revoke'/'Grant' word the render site puts
    // in front of the rest.
    expect(t('admin.viewers.revokePremiumTitle', { username: 'kate' })).toBe(
      'Revoke premium for \u201Ckate\u201D?'
    )
    expect(t('admin.viewers.grantPremiumTitle', { username: 'kate' })).toBe(
      'Grant premium for \u201Ckate\u201D?'
    )
    expect(t('admin.viewers.revokePremiumBody')).toBe(
      'They will lose access to gradients, avatar frames, and flairs.'
    )
    expect(t('admin.viewers.grantPremiumBody')).toBe(
      'They will be able to use gradients, avatar frames, and flairs.'
    )
    expect(t('admin.viewers.premiumExpires', { timestamp: '13/07/2026, 08:48' })).toBe(
      'Time-limited \u2014 expires 13/07/2026, 08:48'
    )
    expect(t('admin.viewers.premiumDialogCancel')).toBe('Cancel')
    expect(t('admin.viewers.revokePremiumConfirm')).toBe('Revoke Premium')
    expect(t('admin.viewers.grantPremiumConfirm')).toBe('Grant Premium')
    expect(t('admin.viewers.premiumUpdating')).toBe('Updating...')
  })

  it('keeps the unban and ban dialog copy', () => {
    expect(t('admin.viewers.unbanTitle', { username: 'kate' })).toBe('Unban \u201Ckate\u201D?')
    expect(t('admin.viewers.unbanBody')).toBe('This will restore their ability to send messages.')
    expect(t('admin.viewers.unbanCancel')).toBe('Cancel')
    expect(t('admin.viewers.unbanConfirm')).toBe('Unban Viewer')
    expect(t('admin.viewers.banTitle', { username: 'kate' })).toBe('Ban Viewer \u201Ckate\u201D?')
    expect(t('admin.viewers.banBody', { username: 'kate' })).toBe(
      'This will prevent kate from sending messages.'
    )
    expect(t('admin.viewers.banReasonLabel')).toBe('Reason (optional)')
    expect(t('admin.viewers.banReasonPlaceholder')).toBe('Enter reason for ban...')
    expect(t('admin.viewers.banCancel')).toBe('Cancel')
    expect(t('admin.viewers.banConfirm')).toBe('Ban Viewer')
    expect(t('admin.viewers.banning')).toBe('Banning...')
  })

  it('keeps the viewer mutation toast copy', () => {
    expect(t('admin.viewers.loadError')).toBe('Failed to load viewers')
    expect(t('admin.viewers.activityLoadError')).toBe('Failed to load activity')
    expect(t('admin.viewers.banSuccess', { username: 'kate' })).toBe('kate banned successfully')
    expect(t('admin.viewers.banError')).toBe('Failed to ban viewer')
    expect(t('admin.viewers.unbanSuccess', { username: 'kate' })).toBe('kate unbanned successfully')
    expect(t('admin.viewers.unbanError')).toBe('Failed to unban viewer')
    // Two whole sentences: the render site interpolated 'granted'/'revoked' as a
    // bare past participle, which is not a translatable unit.
    expect(t('admin.viewers.premiumGranted', { username: 'kate' })).toBe('kate premium granted')
    expect(t('admin.viewers.premiumRevoked', { username: 'kate' })).toBe('kate premium revoked')
    expect(t('admin.viewers.premiumError')).toBe('Failed to update premium status')
  })
})

describe('admin users page copy', () => {
  it('keeps the page header, load error and search field', () => {
    expect(t('admin.users.heading')).toBe('Users')
    expect(t('admin.users.intro')).toBe('Manage and view all users in the system')
    expect(t('admin.users.loadError')).toBe('Failed to load users')
    expect(t('admin.users.listHeading', { count: 84 })).toBe('All Users (84)')
    expect(t('admin.users.searchPlaceholder')).toBe(
      'Search by username, display name, or platform ID...'
    )
  })

  it('keeps the six filter tabs and the "no setup" tooltip', () => {
    // Each tab is one key with its count inside, not a label plus a parenthesised
    // number the render site assembles.
    expect(t('admin.users.tabAll', { count: 84 })).toBe('All (84)')
    expect(t('admin.users.tabActive', { count: 80 })).toBe('Active (80)')
    expect(t('admin.users.tabBanned', { count: 4 })).toBe('Banned (4)')
    expect(t('admin.users.tabPremium', { count: 12 })).toBe('Premium (12)')
    expect(t('admin.users.tabBeta', { count: 3 })).toBe('Beta (3)')
    expect(t('admin.users.tabNoSetup', { count: 9 })).toBe('No setup (9)')
    expect(t('admin.users.tabNoSetupTitle')).toBe(
      'Signed up but never configured a chat source (0 overlays or 0 sources)'
    )
  })

  it('keeps the row badges and the empty list state', () => {
    expect(t('admin.users.badgeAmbassador')).toBe('AMBASSADOR')
    expect(t('admin.users.badgeBeta')).toBe('BETA')
    expect(t('admin.users.badgePremium')).toBe('PREMIUM')
    expect(t('admin.users.badgeBanned')).toBe('BANNED')
    expect(t('admin.users.badgeNoOverlay')).toBe('NO OVERLAY')
    expect(t('admin.users.badgeNoSources')).toBe('NO SOURCES')
    expect(t('admin.users.badgeNoSetupTitle')).toBe('Signed up but never configured a chat source')
    expect(t('admin.users.rowJoined', { date: '13/07/2026' })).toBe('Joined 13/07/2026')
    expect(t('admin.users.empty')).toBe('No users match your search or filter.')
  })

  it('keeps the detail panel field labels', () => {
    expect(t('admin.users.detailId')).toBe('ID')
    expect(t('admin.users.detailUsername')).toBe('Username')
    expect(t('admin.users.detailDisplayName')).toBe('Display Name')
    expect(t('admin.users.detailAuthProvider')).toBe('Auth Provider')
    expect(t('admin.users.detailPlatforms')).toBe('Connected Platforms')
    // The three platform rows read '<Platform>:'. The colon is punctuation a
    // language may place differently, so it travels with the label.
    expect(t('admin.users.platformIdTwitch')).toBe('Twitch:')
    expect(t('admin.users.platformIdYouTube')).toBe('YouTube:')
    expect(t('admin.users.platformIdKick')).toBe('Kick:')
    expect(t('admin.users.selectPrompt')).toBe('Select a user to view details')
  })

  it('keeps the impersonation copy', () => {
    expect(t('admin.users.viewAsButton', { username: 'kate' })).toBe('View as kate')
    expect(t('admin.users.impersonateTitle', { username: 'kate' })).toBe(
      'Impersonate \u201Ckate\u201D?'
    )
    expect(t('admin.users.impersonateBody')).toBe(
      'This will replace your current session. You can return to admin by using the stored admin token.'
    )
    expect(t('admin.users.impersonateCancel')).toBe('Cancel')
    expect(t('admin.users.impersonateConfirm')).toBe('Impersonate')
    expect(t('admin.users.impersonateSwitching')).toBe('Switching...')
    expect(t('admin.users.impersonateHint')).toBe('Temporarily act as this user to debug issues')
    expect(t('admin.users.impersonateError')).toBe(
      'Failed to start impersonation. Please try again.'
    )
  })

  it('keeps the premium section copy', () => {
    expect(t('admin.users.premiumActiveTitle')).toBe('Premium access active')
    expect(t('admin.users.premiumActiveBody')).toBe(
      'This user can create and accept share requests.'
    )
    expect(t('admin.users.premiumExpires', { timestamp: '13/07/2026, 08:48' })).toBe(
      'Time-limited \u2014 expires 13/07/2026, 08:48'
    )
    expect(t('admin.users.revokePremiumButton')).toBe('Revoke Premium')
    expect(t('admin.users.revokePremiumLabel', { username: 'kate' })).toBe(
      'Revoke Premium for kate'
    )
    expect(t('admin.users.revokePremiumTitle', { username: 'kate' })).toBe(
      'Revoke premium for \u201Ckate\u201D?'
    )
    expect(t('admin.users.revokePremiumBody')).toBe(
      'They will no longer be able to create or accept share requests.'
    )
    expect(t('admin.users.grantPremiumButton')).toBe('Grant Premium')
    expect(t('admin.users.grantPremiumLabel', { username: 'kate' })).toBe('Grant Premium to kate')
    expect(t('admin.users.grantPremiumTitle', { username: 'kate' })).toBe(
      'Grant premium to \u201Ckate\u201D?'
    )
    expect(t('admin.users.grantPremiumBody')).toBe(
      'They will be able to create and accept chat overlay share requests.'
    )
  })

  it('keeps the beta tester section copy', () => {
    expect(t('admin.users.betaActiveTitle')).toBe('Beta tester')
    expect(t('admin.users.betaActiveBody')).toBe('Has all premium features plus early-access ones.')
    expect(t('admin.users.revokeBetaButton')).toBe('Revoke Beta Tester')
    expect(t('admin.users.revokeBetaLabel', { username: 'kate' })).toBe(
      'Revoke Beta Tester for kate'
    )
    expect(t('admin.users.revokeBetaTitle', { username: 'kate' })).toBe(
      'Revoke beta tester for \u201Ckate\u201D?'
    )
    expect(t('admin.users.revokeBetaBody')).toBe(
      'They lose early-access features. Premium then follows their subscription or any admin override.'
    )
    expect(t('admin.users.grantBetaButton')).toBe('Grant Beta Tester')
    expect(t('admin.users.grantBetaLabel', { username: 'kate' })).toBe('Grant Beta Tester to kate')
    expect(t('admin.users.grantBetaTitle', { username: 'kate' })).toBe(
      'Grant beta tester to \u201Ckate\u201D?'
    )
    expect(t('admin.users.grantBetaBody')).toBe(
      'They gain all premium features plus early-access ones. Use this to grandfather pre-monetization premium users.'
    )
  })

  it('keeps the ambassador section copy', () => {
    expect(t('admin.users.ambassadorActiveTitle')).toBe('Ambassador')
    expect(t('admin.users.ambassadorActiveBody')).toBe(
      'Has all premium plus early-access features. Appears on the homepage only after the streamer opts in from their settings.'
    )
    expect(t('admin.users.taglineLabel')).toBe('Showcase tagline')
    expect(t('admin.users.taglineOptionalLabel')).toBe('Showcase tagline (optional)')
    expect(t('admin.users.taglinePlaceholder')).toBe(
      'e.g. Multistreams to Twitch, YouTube and Kick'
    )
    expect(t('admin.users.taglineFieldLabel')).toBe('Ambassador showcase tagline')
    expect(t('admin.users.sortOrderLabel')).toBe('Display order (lower shows first)')
    expect(t('admin.users.sortOrderFieldLabel')).toBe('Ambassador display order')
    expect(t('admin.users.saveShowcaseButton')).toBe('Save showcase card')
    expect(t('admin.users.saveShowcaseLabel', { username: 'kate' })).toBe(
      'Save showcase card for kate'
    )
    expect(t('admin.users.revokeAmbassadorButton')).toBe('Revoke Ambassador')
    expect(t('admin.users.revokeAmbassadorLabel', { username: 'kate' })).toBe(
      'Revoke Ambassador for kate'
    )
    expect(t('admin.users.revokeAmbassadorTitle', { username: 'kate' })).toBe(
      'Revoke ambassador for \u201Ckate\u201D?'
    )
    expect(t('admin.users.revokeAmbassadorBody')).toBe(
      'They are removed from the homepage showcase and lose early-access features. Premium then follows their subscription or any admin override.'
    )
    expect(t('admin.users.grantAmbassadorButton')).toBe('Grant Ambassador')
    expect(t('admin.users.grantAmbassadorLabel', { username: 'kate' })).toBe(
      'Grant Ambassador to kate'
    )
  })

  it('keeps the ban and unban section copy', () => {
    expect(t('admin.users.bannedReason', { reason: 'spam' })).toBe('Banned: spam')
    expect(t('admin.users.bannedOn', { timestamp: '13/07/2026, 08:48' })).toBe(
      'Banned on 13/07/2026, 08:48'
    )
    expect(t('admin.users.unbanButton')).toBe('Unban User')
    expect(t('admin.users.unbanLabel', { username: 'kate' })).toBe('Unban User kate')
    expect(t('admin.users.unbanTitle', { username: 'kate' })).toBe('Unban \u201Ckate\u201D?')
    expect(t('admin.users.unbanBody')).toBe('This will restore their access to the platform.')
    expect(t('admin.users.unbanCancel')).toBe('Cancel')
    expect(t('admin.users.banButton')).toBe('Ban User')
    expect(t('admin.users.banLabel', { username: 'kate' })).toBe('Ban User kate')
    expect(t('admin.users.banTitle', { username: 'kate' })).toBe('Ban \u201Ckate\u201D?')
    expect(t('admin.users.banBody')).toBe('This will prevent the user from accessing the platform.')
    // The asterisk marks the field required and is part of the visible label.
    expect(t('admin.users.banReasonLabel')).toBe('Reason for ban *')
    expect(t('admin.users.banReasonPlaceholder')).toBe('Spam, abuse, ToS violation, etc...')
    expect(t('admin.users.banCancel')).toBe('Cancel')
    expect(t('admin.users.banning')).toBe('Banning...')
  })

  it('keeps the overlays section copy', () => {
    expect(t('admin.users.overlaysHeading', { count: 2 })).toBe('Overlays (2)')
    expect(t('admin.users.overlaySourceCount', { count: 3 })).toBe('3 sources')
    expect(t('admin.users.openOverlayLabel', { name: 'Main' })).toBe(
      'Open the live Main overlay (opens in a new tab)'
    )
    expect(t('admin.users.overlaysEmpty')).toBe('No overlays yet')
    // The apostrophe was a &rsquo; entity, so it is the typographic character.
    expect(t('admin.users.viewSourcesLink')).toBe('View this user\u2019s sources')
  })

  it('keeps the shared saving label and the mutation toasts', () => {
    // One 'Saving...' key: the premium, beta and ambassador buttons all showed
    // the identical word while their request was in flight.
    expect(t('admin.users.saving')).toBe('Saving...')
    expect(t('admin.users.banSuccess', { username: 'kate' })).toBe('kate banned successfully')
    expect(t('admin.users.banError')).toBe('Failed to ban user')
    expect(t('admin.users.unbanSuccess', { username: 'kate' })).toBe('kate unbanned successfully')
    expect(t('admin.users.unbanError')).toBe('Failed to unban user')
    // Two whole sentences per role change: the render site interpolated only the
    // username, but the rest of each sentence differs by more than one word.
    expect(t('admin.users.premiumGranted', { username: 'kate' })).toBe(
      'kate granted premium access'
    )
    expect(t('admin.users.premiumRemoved', { username: 'kate' })).toBe(
      'kate premium access removed'
    )
    expect(t('admin.users.premiumError')).toBe('Failed to update premium status')
    expect(t('admin.users.betaGranted', { username: 'kate' })).toBe('kate is now a beta tester')
    expect(t('admin.users.betaRemoved', { username: 'kate' })).toBe(
      'kate is no longer a beta tester'
    )
    expect(t('admin.users.betaError')).toBe('Failed to update beta-tester status')
    expect(t('admin.users.ambassadorGranted', { username: 'kate' })).toBe(
      'kate is now an ambassador'
    )
    expect(t('admin.users.ambassadorRemoved', { username: 'kate' })).toBe(
      'kate is no longer an ambassador'
    )
    expect(t('admin.users.ambassadorError')).toBe('Failed to update ambassador status')
  })
})
