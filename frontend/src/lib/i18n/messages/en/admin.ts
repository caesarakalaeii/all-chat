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
 * The admin console.
 */

export const admin = {
  // ADMIN_LINKS in AdminSidebar.tsx is one table feeding two render sites (the
  // rail and the dashboard home grid), so each entry carries a message stem and
  // resolves `<stem>Label` / `<stem>Description` here rather than holding copy.
  nav: {
    dashboardLabel: 'Dashboard',
    searchLabel: 'Search',
    searchDescription: 'Find any user, overlay, source, or viewer',
    usersLabel: 'Users',
    usersDescription: 'View and manage users',
    overlaysLabel: 'Overlays',
    overlaysDescription: 'Overlays and their owners',
    sourcesLabel: 'Sources',
    sourcesDescription: 'Every chat source',
    viewersLabel: 'Viewers',
    viewersDescription: 'Viewer sessions and bans',
    cosmeticsLabel: 'Cosmetics',
    cosmeticsDescription: 'Avatar frames and flairs',
    featuresLabel: 'Features',
    featuresDescription: 'Premium feature gates',
    maintenanceLabel: 'Maintenance',
    maintenanceDescription: 'Maintenance mode and ops',
  },
  sidebar: {
    brandSuffix: 'Admin',
    backToApp: 'Back to app',
    logOut: 'Log out',
    openMenuLabel: 'Open admin menu',
    closeMenuLabel: 'Close admin menu',
    menuLabel: 'Admin menu',
  },
  dashboard: {
    heading: 'Admin Dashboard',
    totalUsers: 'Total Users',
    bannedUsers: 'Banned Users',
    activeOverlays: 'Active Overlays',
    totalSources: 'Total Sources',
    activeUsersHeading: 'Active users',
    activeUsersBody:
      'Distinct users with at least one overlay connected in the window (excludes banned users).',
    last24Hours: 'Last 24 hours',
    last7Days: 'Last 7 days',
    last30Days: 'Last 30 days',
    sourcesByPlatformHeading: 'Sources by platform',
    manageHeading: 'Manage',
  },
  maintenance: {
    heading: 'Maintenance',
    intro:
      'Schedule planned downtime windows. Users see a banner on the dashboard for upcoming and active maintenance.',
    scheduleButton: 'Schedule',
    emptyTitle: 'No maintenance windows scheduled',
    emptyBody: 'Schedule a maintenance window to notify users of upcoming downtime.',
    // The parenthesis is part of the copy, not punctuation the render site holds:
    // a language that brackets differently cannot reorder a fragment left in JSX.
    listHeading: 'Scheduled Windows ({count})',
    statusActive: 'Active',
    statusUpcoming: 'Upcoming',
    deleteLabel: 'Delete {title}',
    deleteConfirm: 'Delete this maintenance window?',
    dialogTitle: 'Schedule Maintenance',
    dialogBody:
      'Create a maintenance window. Users will see a banner on the dashboard until the window ends.',
    titleLabel: 'Title',
    titlePlaceholder: 'e.g. Database maintenance',
    descriptionLabel: 'Description',
    descriptionPlaceholder: 'Optional details about the maintenance',
    startsAtLabel: 'Starts at',
    endsAtLabel: 'Ends at',
    cancelButton: 'Cancel',
    submittingButton: 'Scheduling…',
  },
  features: {
    heading: 'Features',
    intro:
      'Manage capability-level gates. Premium controls paid access; early access restricts a feature to beta testers. Both toggle without a code deploy.',
    loadError: 'Failed to load feature gates. Refresh the page to try again.',
    emptyTitle: 'No feature gates configured',
    emptyBody:
      'Feature gates are added automatically when new features ship. Check back after the next deployment.',
    listHeading: 'Feature Gates ({count})',
    badgePremiumOnly: 'Premium only',
    badgeFreeForAll: 'Free for all',
    badgeEarlyAccess: 'Early access',
    badgeStandard: 'Standard',
    togglePremiumLabel: 'Toggle premium for {feature}',
    toggleEarlyAccessLabel: 'Toggle early access for {feature}',
    // Two orthogonal gate dimensions, each with two directions, so four whole
    // dialogs. The feature key sits mid-sentence in every title, so these cannot
    // be a shared stem plus a direction fragment: a language that fronts the verb
    // or the object could not reassemble the sentence.
    makeFreeTitle: 'Make {feature} free for all users?',
    makeFreeBody: 'All authenticated users will gain access immediately. No code deploy required.',
    makeFreeConfirm: 'Make Free',
    makePremiumTitle: 'Restrict {feature} to premium users?',
    makePremiumBody: 'Only users with premium access will be able to use this feature.',
    makePremiumConfirm: 'Make Premium',
    graduateTitle: 'Graduate {feature} from early access?',
    graduateBody: 'Beta-tester-only access is lifted; the feature defers to its premium gate.',
    graduateConfirm: 'Graduate',
    makeEarlyAccessTitle: 'Restrict {feature} to beta testers?',
    makeEarlyAccessBody: 'Only beta testers will be able to use this early-access feature.',
    makeEarlyAccessConfirm: 'Make Early Access',
    dialogCancel: 'No, keep as-is',
  },
  search: {
    heading: 'Search',
    intro: 'Find any user, overlay, source, or viewer and jump straight to it',
    inputPlaceholder: 'Search users, overlays, sources, viewers...',
    inputLabel: 'Global admin search',
    promptState: 'Type at least one character to search.',
    loadingState: 'Searching...',
    // The typographic quotes are part of the copy: a language that quotes with
    // guillemets or corner brackets replaces the pair, which it cannot do if the
    // render site holds one of each.
    emptyState: 'Nothing matches “{query}”.',
    groupUsers: 'Users',
    groupOverlays: 'Overlays',
    groupSources: 'Sources',
    groupViewers: 'Viewers',
    groupCountTruncated: '(showing {shown} of {total})',
    groupCountExact: '({total})',
    badgePremium: 'Premium',
    badgeBanned: 'Banned',
    // Whole phrases, not a count or a name plus a fragment the render site keeps:
    // a bare noun or preposition in JSX cannot be reordered by a translation.
    overlaySourceCount: '{count} sources',
    sourceInOverlay: 'in {overlay}',
  },
  cosmetics: {
    heading: 'Cosmetics Catalog',
    intro: 'Manage avatar frames and flairs',
    // The tab bar rendered the wire values 'frames'/'flairs' through a CSS
    // `capitalize`, so the visible text was never in the source. It is copy now.
    tabFrames: 'Frames',
    tabFlairs: 'Flairs',
    // `itemLabel` was concatenated into these sentences, once via toLowerCase().
    // Whole sentences per entry kind instead: a noun spliced in by the render
    // site cannot inflect, and its lowercase form is not always the visible one.
    emptyFrames: 'No frames in catalog yet',
    emptyFlairs: 'No flairs in catalog yet',
    addFrameHeading: 'Add Frame',
    addFlairHeading: 'Add Flair',
    nameLabel: 'Name',
    framePlaceholder: 'Frame name',
    flairPlaceholder: 'Flair name',
    imageUrlLabel: 'Image URL',
    imageUrlPlaceholder: 'https://example.com/frame.png',
    previewAlt: 'Preview',
    premiumOnlyLabel: 'Premium only',
    submitFrame: 'Add Frame',
    submitFlair: 'Add Flair',
    submittingButton: 'Adding…',
    badgePremium: 'Premium',
    deleteLabel: 'Delete {name}',
    loadFramesError: 'Failed to load frames',
    loadFlairsError: 'Failed to load flairs',
    frameDeleted: 'Frame deleted',
    flairDeleted: 'Flair deleted',
    deleteError: 'Delete failed',
    frameAdded: 'Frame added',
    flairAdded: 'Flair added',
    addError: 'Add failed',
  },
  overlays: {
    heading: 'Overlays',
    intro: 'Manage overlays and their connected chat sources',
    loadError: 'Failed to load overlays',
    // The typographic quotes were entities around the phrase and travel with
    // the copy: a language that quotes with guillemets replaces the pair.
    statusUnavailable:
      'Live connection status is currently unavailable, so overlays may show as “not connected” even if they are live.',
    // Two whole headings rather than an ' of N' tail the render site appends: a
    // language wording "8 of 30" differently cannot reorder a fragment it does
    // not own.
    listHeadingAll: 'All Overlays ({count})',
    listHeadingFiltered: 'All Overlays ({shown} of {total})',
    searchPlaceholder: 'Search by overlay name, ID, or owner...',
    connectedFilter: 'Connected ({count})',
    emptyNone: 'No overlays found.',
    emptyFiltered: 'No overlays match your search or filter.',
    rowSourceCount: '{count} sources',
    rowIdPrefix: 'ID: {id}',
    rowCreated: 'Created {date}',
    openInNewTabLabel: 'Open overlay {name} in a new tab',
    // The connection dot's tooltip and the elapsed label each either name a
    // time or do not, so each is two whole strings.
    dotConnectedSince: 'Connected since {timestamp}',
    dotConnected: 'Connected',
    connectedFor: 'Connected {duration}',
    connected: 'Connected',
    detailHeading: 'Overlay Details',
    detailName: 'Name',
    detailId: 'ID',
    detailOwner: 'Owner',
    // Four whole owner link texts. The link read a handle-or-fallback plus an
    // optional ' (Display Name)' tail; a fragment beginning with a space is not
    // translatable, and the parenthesis convention is language-specific.
    ownerHandle: '@{username}',
    ownerHandleNamed: '@{username} ({displayName})',
    ownerFallback: 'View user',
    ownerFallbackNamed: 'View user ({displayName})',
    ownerUnknown: 'Unknown',
    detailConnection: 'Connection',
    notConnected: 'Not connected',
    connectedSinceRow: 'Since {timestamp}',
    selectPrompt: 'Select an overlay to view details',
    sourcesHeading: 'Connected Sources ({count})',
    sourceActive: 'Active',
    sourceInactive: 'Inactive',
    sourceAdded: 'Added {date}',
    sourcesEmpty: 'No sources connected',
  },
  sources: {
    heading: 'Sources',
    intro: 'View and manage all chat sources across overlays',
    loadError: 'Failed to load sources',
    // One sentence with the owner name as a param, resolved through
    // interpolateElements so the <Link> lands wherever the language puts it.
    ownerScope: 'Showing sources owned by {owner}',
    ownerScopeClear: 'Clear',
    searchLabel: 'Search',
    searchPlaceholder: 'Search by channel, overlay, or owner...',
    platformLabel: 'Platform',
    platformAll: 'All Platforms',
    statusLabel: 'Status',
    statusAll: 'All Status',
    statusActive: 'Active',
    statusInactive: 'Inactive',
    emptyNone: 'No sources found.',
    emptyFiltered: 'No sources match your filters.',
    listHeading: 'All Sources ({count})',
    tableCaption: 'Chat sources',
    columnPlatform: 'Platform',
    columnChannel: 'Channel',
    columnOverlay: 'Overlay',
    columnOwner: 'Owner',
    columnStatus: 'Status',
    columnCreated: 'Created',
  },
} as const
