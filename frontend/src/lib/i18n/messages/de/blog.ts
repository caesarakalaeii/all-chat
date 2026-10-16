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

import type { LocaleNamespace } from '../../translate'

export const blog: LocaleNamespace<'blog'> = {
  metaTitle: 'Engineering-Blog | All-Chat',
  metaDescription:
    'Was in All-Chat kaputtgegangen ist, warum und was es brauchte, um es zu fixen. Geschrieben vom Entwickler, mit Daten und Zahlen.',
  breadcrumb: 'Blog',
  eyebrow: 'Engineering-Blog',
  heading: 'Was kaputtging – und wie es gefixt wurde',
  intro:
    'All-Chat führt den Chat aller Plattformen in einem Overlay zusammen. Diese Posts handeln von den Stellen, die dabei schiefgelaufen sind.',
  listHeading: 'Posts',
  byline: 'Von {author}',
  publishedOn: 'Veröffentlicht {date}',
  backToIndex: 'Alle Posts',
}
