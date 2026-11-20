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
  metaTitle: 'Blog de ingeniería',
  metaDescription:
    'Qué se rompió en All-Chat, por qué y qué hizo falta para arreglarlo. Escrito por el desarrollador, con fechas y cifras.',
  breadcrumb: 'Blog',
  eyebrow: 'Blog de ingeniería',
  heading: 'Qué se rompió y cómo se arregló',
  intro:
    'All-Chat une el chat de todas las plataformas en un solo overlay. Estas entradas tratan de lo que salió mal por el camino.',
  listHeading: 'Entradas',
  byline: 'Por {author}',
  publishedOn: 'Publicado el {date}',
  backToIndex: 'Todas las entradas',
}
