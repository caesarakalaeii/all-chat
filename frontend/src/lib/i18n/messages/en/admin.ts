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
} as const
