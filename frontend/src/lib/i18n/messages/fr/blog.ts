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
  metaTitle: 'Blog technique',
  metaDescription:
    "Ce qui a cassé dans All-Chat, pourquoi, et ce qu'il a fallu pour le réparer. Écrit par le développeur, avec les dates et les chiffres.",
  breadcrumb: 'Blog',
  eyebrow: 'Blog technique',
  heading: 'Ce qui a cassé, et comment ça a été réparé',
  intro:
    'All-Chat fusionne le chat de toutes les plateformes dans un seul overlay. Ces articles parlent de ce qui a mal tourné en chemin.',
  listHeading: 'Articles',
  byline: 'Par {author}',
  publishedOn: 'Publié le {date}',
  backToIndex: 'Tous les articles',
}
