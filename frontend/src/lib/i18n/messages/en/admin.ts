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
  viewers: {
    heading: 'Viewer Management',
    intro: 'Search viewer sessions, inspect activity, and manage bans and premium',
    // One key: the count and the word were JSX siblings, so a language that puts
    // the count last could not reorder them.
    totalMatching: '{count} matching',
    searchLabel: 'Search',
    searchPlaceholder: 'Username, display name, or platform user ID...',
    platformLabel: 'Platform',
    platformAll: 'All platforms',
    statusLabel: 'Status',
    statusAny: 'Any',
    statusActive: 'Active',
    statusBanned: 'Banned',
    premiumLabel: 'Premium',
    premiumAny: 'Any',
    premiumOnly: 'Premium',
    premiumFree: 'Free',
    empty: 'No viewer sessions match your search or filters.',
    tableCaption: 'Viewers',
    columnViewer: 'Viewer',
    columnPlatform: 'Platform',
    columnLastMessage: 'Last Message',
    columnPremium: 'Premium',
    columnStatus: 'Status',
    columnActions: 'Actions',
    neverMessaged: 'Never',
    badgeBanned: 'BANNED',
    badgeActive: 'Active',
    banReason: 'Reason: {reason}',
    sessionOnlyTitle: 'Session-only viewer (no linked account)',
    premiumBadge: 'Premium',
    freeBadge: 'Free',
    // Two whole aria labels: the badge word was prefixed to the sentence, and a
    // language that fronts the verb cannot move a fragment it does not own.
    changePremiumPremiumLabel: 'Premium: change premium status for {username}',
    changePremiumFreeLabel: 'Free: change premium status for {username}',
    activityButton: 'Activity',
    unbanButton: 'Unban',
    unbanningButton: 'Unbanning...',
    unbanLabel: 'Unban {username}',
    unbanningLabel: 'Unbanning {username}',
    banButton: 'Ban',
    banLabel: 'Ban {username}',
    // The en dash and 'of' were JSX text between three formatted numbers.
    pageRange: 'Showing {start}–{end} of {total}',
    previousPage: 'Previous',
    nextPage: 'Next',
    activityTitle: 'Activity for “{username}”',
    activityDescription:
      'Messages this viewer has sent through All-Chat, and whose chats they appear in.',
    activityTotalMessages: 'Total messages',
    activityLastMessage: 'Last message',
    activityStreamerFallback: 'View streamer',
    activityOverlayLink: 'overlay',
    activityEmpty: 'No message activity recorded for this viewer.',
    activityClose: 'Close',
    // Two whole titles rather than a 'Revoke'/'Grant' word in front of the rest.
    revokePremiumTitle: 'Revoke premium for “{username}”?',
    grantPremiumTitle: 'Grant premium for “{username}”?',
    revokePremiumBody: 'They will lose access to gradients, avatar frames, and flairs.',
    grantPremiumBody: 'They will be able to use gradients, avatar frames, and flairs.',
    premiumExpires: 'Time-limited — expires {timestamp}',
    premiumDialogCancel: 'Cancel',
    revokePremiumConfirm: 'Revoke Premium',
    grantPremiumConfirm: 'Grant Premium',
    premiumUpdating: 'Updating...',
    unbanTitle: 'Unban “{username}”?',
    unbanBody: 'This will restore their ability to send messages.',
    unbanCancel: 'Cancel',
    unbanConfirm: 'Unban Viewer',
    banTitle: 'Ban Viewer “{username}”?',
    banBody: 'This will prevent {username} from sending messages.',
    banReasonLabel: 'Reason (optional)',
    banReasonPlaceholder: 'Enter reason for ban...',
    banCancel: 'Cancel',
    banConfirm: 'Ban Viewer',
    banning: 'Banning...',
    loadError: 'Failed to load viewers',
    activityLoadError: 'Failed to load activity',
    banSuccess: '{username} banned successfully',
    banError: 'Failed to ban viewer',
    unbanSuccess: '{username} unbanned successfully',
    unbanError: 'Failed to unban viewer',
    // Two whole sentences: 'granted'/'revoked' interpolated as a bare past
    // participle is not a translatable unit.
    premiumGranted: '{username} premium granted',
    premiumRevoked: '{username} premium revoked',
    premiumError: 'Failed to update premium status',
  },
  users: {
    heading: 'Users',
    intro: 'Manage and view all users in the system',
    loadError: 'Failed to load users',
    listHeading: 'All Users ({count})',
    searchPlaceholder: 'Search by username, display name, or platform ID...',
    // Each tab is one string: the label and its parenthesised count were JSX
    // siblings, which a language putting the count first could not reorder.
    tabAll: 'All ({count})',
    tabActive: 'Active ({count})',
    tabBanned: 'Banned ({count})',
    tabPremium: 'Premium ({count})',
    tabBeta: 'Beta ({count})',
    tabNoSetup: 'No setup ({count})',
    tabNoSetupTitle: 'Signed up but never configured a chat source (0 overlays or 0 sources)',
    badgeAmbassador: 'AMBASSADOR',
    badgeBeta: 'BETA',
    badgePremium: 'PREMIUM',
    badgeBanned: 'BANNED',
    badgeNoOverlay: 'NO OVERLAY',
    badgeNoSources: 'NO SOURCES',
    badgeNoSetupTitle: 'Signed up but never configured a chat source',
    rowJoined: 'Joined {date}',
    empty: 'No users match your search or filter.',
    detailId: 'ID',
    detailUsername: 'Username',
    detailDisplayName: 'Display Name',
    detailAuthProvider: 'Auth Provider',
    detailPlatforms: 'Connected Platforms',
    // The colon travels with the label: a language may space or place it
    // differently, which it cannot do while the render site owns it.
    platformIdTwitch: 'Twitch:',
    platformIdYouTube: 'YouTube:',
    platformIdKick: 'Kick:',
    selectPrompt: 'Select a user to view details',
    viewAsButton: 'View as {username}',
    impersonateTitle: 'Impersonate “{username}”?',
    impersonateBody:
      'This will replace your current session. You can return to admin by using the stored admin token.',
    impersonateCancel: 'Cancel',
    impersonateConfirm: 'Impersonate',
    impersonateSwitching: 'Switching...',
    impersonateHint: 'Temporarily act as this user to debug issues',
    impersonateError: 'Failed to start impersonation. Please try again.',
    premiumActiveTitle: 'Premium access active',
    premiumActiveBody: 'This user can create and accept share requests.',
    premiumExpires: 'Time-limited — expires {timestamp}',
    revokePremiumButton: 'Revoke Premium',
    revokePremiumLabel: 'Revoke Premium for {username}',
    revokePremiumTitle: 'Revoke premium for “{username}”?',
    revokePremiumBody: 'They will no longer be able to create or accept share requests.',
    revokePremiumCancel: 'Cancel',
    grantPremiumButton: 'Grant Premium',
    grantPremiumLabel: 'Grant Premium to {username}',
    grantPremiumTitle: 'Grant premium to “{username}”?',
    grantPremiumBody: 'They will be able to create and accept chat overlay share requests.',
    grantPremiumCancel: 'Cancel',
    betaActiveTitle: 'Beta tester',
    betaActiveBody: 'Has all premium features plus early-access ones.',
    revokeBetaButton: 'Revoke Beta Tester',
    revokeBetaLabel: 'Revoke Beta Tester for {username}',
    revokeBetaTitle: 'Revoke beta tester for “{username}”?',
    revokeBetaBody:
      'They lose early-access features. Premium then follows their subscription or any admin override.',
    revokeBetaCancel: 'Cancel',
    grantBetaButton: 'Grant Beta Tester',
    grantBetaLabel: 'Grant Beta Tester to {username}',
    grantBetaTitle: 'Grant beta tester to “{username}”?',
    grantBetaBody:
      'They gain all premium features plus early-access ones. Use this to grandfather pre-monetization premium users.',
    grantBetaCancel: 'Cancel',
    ambassadorActiveTitle: 'Ambassador',
    ambassadorActiveBody:
      'Has all premium plus early-access features. Appears on the homepage only after the streamer opts in from their settings.',
    taglineLabel: 'Showcase tagline',
    taglineOptionalLabel: 'Showcase tagline (optional)',
    taglinePlaceholder: 'e.g. Multistreams to Twitch, YouTube and Kick',
    taglineFieldLabel: 'Ambassador showcase tagline',
    sortOrderLabel: 'Display order (lower shows first)',
    sortOrderFieldLabel: 'Ambassador display order',
    saveShowcaseButton: 'Save showcase card',
    saveShowcaseLabel: 'Save showcase card for {username}',
    revokeAmbassadorButton: 'Revoke Ambassador',
    revokeAmbassadorLabel: 'Revoke Ambassador for {username}',
    revokeAmbassadorTitle: 'Revoke ambassador for “{username}”?',
    revokeAmbassadorBody:
      'They are removed from the homepage showcase and lose early-access features. Premium then follows their subscription or any admin override.',
    revokeAmbassadorCancel: 'Cancel',
    grantAmbassadorButton: 'Grant Ambassador',
    grantAmbassadorLabel: 'Grant Ambassador to {username}',
    bannedReason: 'Banned: {reason}',
    bannedOn: 'Banned on {timestamp}',
    unbanButton: 'Unban User',
    unbanLabel: 'Unban User {username}',
    unbanTitle: 'Unban “{username}”?',
    unbanBody: 'This will restore their access to the platform.',
    unbanCancel: 'Cancel',
    banButton: 'Ban User',
    banLabel: 'Ban User {username}',
    banTitle: 'Ban “{username}”?',
    banBody: 'This will prevent the user from accessing the platform.',
    // The asterisk marks the field required and is part of what the label reads.
    banReasonLabel: 'Reason for ban *',
    banReasonPlaceholder: 'Spam, abuse, ToS violation, etc...',
    banCancel: 'Cancel',
    banning: 'Banning...',
    overlaysHeading: 'Overlays ({count})',
    overlaySourceCount: '{count} sources',
    openOverlayLabel: 'Open the live {name} overlay (opens in a new tab)',
    overlaysEmpty: 'No overlays yet',
    viewSourcesLink: 'View this user’s sources',
    // One key: the premium, beta and ambassador buttons all showed this while
    // their request was in flight.
    saving: 'Saving...',
    banSuccess: '{username} banned successfully',
    banError: 'Failed to ban user',
    unbanSuccess: '{username} unbanned successfully',
    unbanError: 'Failed to unban user',
    // Whole sentences per direction: each differs from its opposite by more than
    // the one word the render site interpolated.
    premiumGranted: '{username} granted premium access',
    premiumRemoved: '{username} premium access removed',
    premiumError: 'Failed to update premium status',
    betaGranted: '{username} is now a beta tester',
    betaRemoved: '{username} is no longer a beta tester',
    betaError: 'Failed to update beta-tester status',
    ambassadorGranted: '{username} is now an ambassador',
    ambassadorRemoved: '{username} is no longer an ambassador',
    ambassadorError: 'Failed to update ambassador status',
  },
  // The premium-grant duration chooser, shared by /admin/users and
  // /admin/viewers. Both callers are admin surfaces, so this sits here rather
  // than in common.*, which holds copy shared across unrelated surfaces.
  premiumDuration: {
    label: 'Duration',
    presetPermanent: 'Permanent',
    preset1Day: '1 day',
    preset7Days: '7 days',
    preset30Days: '30 days',
    preset90Days: '90 days',
    presetCustom: 'Custom',
    customPlaceholder: 'days',
    customFieldLabel: 'Custom duration in days',
    // One string: the en dash came from an &ndash; entity, and a language that
    // words a range differently cannot reorder JSX siblings.
    customRange: 'days (1–{max})',
  },
} as const
