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
 * The chrome around the engineering blog: /blog and /blog/<slug>.
 *
 * Only the chrome. The posts themselves live in src/content/blog/ as
 * English-only authored prose, outside the catalog (see that directory's
 * index.ts for why).
 */

export const blog = {
  metaTitle: 'Engineering blog | All-Chat',
  metaDescription:
    'What broke in All-Chat, why, and what it took to fix. Written by the developer, with the dates and numbers.',
  breadcrumb: 'Blog',
  eyebrow: 'Engineering blog',
  heading: 'What broke, and how it got fixed',
  intro:
    'All-Chat merges chat from every platform into one overlay. These posts are about the parts that went wrong on the way.',
  listHeading: 'Posts',
  byline: 'By {author}',
  publishedOn: 'Published {date}',
  backToIndex: 'All posts',
} as const
