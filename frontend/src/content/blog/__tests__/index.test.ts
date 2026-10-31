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

import { readdirSync } from 'node:fs'
import path from 'node:path'
import { describe, expect, it } from 'vitest'
import { POSTS } from '../index'

// A post file that is not in POSTS never reaches /blog, the sitemap or llms.txt.
describe('blog post registry', () => {
  const postFiles = readdirSync(path.join(__dirname, '..'))
    .filter((f) => f.endsWith('.tsx'))
    .map((f) => f.replace(/\.tsx$/, ''))

  it('registers every post file, under its file name as the slug', () => {
    expect(POSTS.map((p) => p.meta.slug).sort()).toEqual([...postFiles].sort())
  })

  it('lists posts newest first', () => {
    const dates = POSTS.map((p) => p.meta.date)
    expect(dates).toEqual([...dates].sort().reverse())
  })
})
