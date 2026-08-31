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
 * The site-wide maintenance banner and the monitor header's info popover.
 *
 * The two surfaces show the same announcements but not the same strings: the
 * banner's labels prefix the title on one line and end in a colon, while the
 * popover's sit on a line of their own and do not.
 */

export const maintenanceBanner = {
  activeLabel: 'Maintenance in progress:',
  expectedCompletion: 'Expected completion:',
  scheduledLabel: 'Scheduled maintenance:',
  rangeSeparator: 'to',
  dismissLabel: 'Dismiss maintenance banner: {title}',
  popoverActiveHeading: 'Maintenance in progress',
  popoverScheduledHeading: 'Scheduled maintenance',
  // Whole sentences with the formatted times as placeholders, so a language
  // that orders or separates them differently can say so.
  popoverExpectedCompletion: 'Expected completion: {endsAt}',
  popoverRange: '{startsAt} to {endsAt}',
} as const
