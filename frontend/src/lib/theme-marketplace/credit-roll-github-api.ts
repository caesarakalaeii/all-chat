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
