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
 * GitHub API Client for Credit Roll Theme Marketplace
 *
 * Fetches credit roll themes from docs/credit-roll-themes/ directory
 */

import type { Theme } from './types'
import { parseThemeMetadata } from './metadata-parser'

const GITHUB_REPO = 'caesarakalaeii/all-chat'
const THEMES_PATH = 'docs/credit-roll-themes'
const GITHUB_RAW_BASE = `https://raw.githubusercontent.com/${GITHUB_REPO}/main`

interface GitHubContent {
  name: string
  path: string
  type: 'file' | 'dir'
  download_url: string
}

/**
 * Fetch list of credit roll theme files from GitHub
 */
export async function fetchCreditRollThemeList(): Promise<string[]> {
  const response = await fetch(
    `https://api.github.com/repos/${GITHUB_REPO}/contents/${THEMES_PATH}`,
    {
      headers: {
        Accept: 'application/vnd.github.v3+json',
      },
    }
  )

  if (!response.ok) {
    throw new Error(`GitHub API error: ${response.status}`)
  }

  const files = (await response.json()) as GitHubContent[]
  return files
    .filter((file) => file.type === 'file' && file.name.endsWith('.css'))
    .map((file) => file.name)
}

/**
 * Fetch credit roll theme CSS content from GitHub
 */
export async function fetchCreditRollThemeContent(filename: string): Promise<string> {
  const response = await fetch(`${GITHUB_RAW_BASE}/${THEMES_PATH}/${filename}`)

  if (!response.ok) {
    throw new Error(`Failed to fetch ${filename}: ${response.status}`)
  }

  return response.text()
}

/**
 * Fetch all credit roll themes with metadata
 */
export async function fetchAllCreditRollThemes(): Promise<Theme[]> {
  const filenames = await fetchCreditRollThemeList()

  const themes = await Promise.all(
    filenames.map(async (filename) => {
      const css = await fetchCreditRollThemeContent(filename)
      const metadata = parseThemeMetadata(css, filename)

      return {
        id: filename.replace('.css', ''),
        filename,
        css,
        ...metadata,
      }
    })
  )

  return themes
}
