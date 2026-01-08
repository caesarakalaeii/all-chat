/**
 * GitHub API Client for Theme Marketplace
 *
 * Fetches theme files from the GitHub repository.
 */

import type { Theme } from './types';
import { parseThemeMetadata } from './metadata-parser';

const GITHUB_REPO = 'caesarakalaeii/all-chat';
const THEMES_PATH = 'docs/overlay-themes';
const GITHUB_RAW_BASE = `https://raw.githubusercontent.com/${GITHUB_REPO}/main`;

interface GitHubContent {
  name: string;
  path: string;
  sha: string;
  size: number;
  url: string;
  html_url: string;
  git_url: string;
  download_url: string;
  type: 'file' | 'dir';
}

/**
 * Fetch list of CSS theme files from GitHub
 */
export async function fetchThemeList(): Promise<string[]> {
  const response = await fetch(
    `https://api.github.com/repos/${GITHUB_REPO}/contents/${THEMES_PATH}`,
    {
      headers: {
        Accept: 'application/vnd.github.v3+json',
      },
    }
  );

  if (!response.ok) {
    throw new Error(`GitHub API error: ${response.status}`);
  }

  const files = (await response.json()) as GitHubContent[];

  // Filter for CSS files only
  return files
    .filter((file) => file.type === 'file' && file.name.endsWith('.css'))
    .map((file) => file.name);
}

/**
 * Fetch raw CSS content from GitHub
 */
export async function fetchThemeContent(filename: string): Promise<string> {
  const response = await fetch(
    `${GITHUB_RAW_BASE}/${THEMES_PATH}/${filename}`
  );

  if (!response.ok) {
    throw new Error(`Failed to fetch ${filename}: ${response.status}`);
  }

  return response.text();
}

/**
 * Fetch all themes from GitHub repository
 */
export async function fetchAllThemes(): Promise<Theme[]> {
  try {
    // Get list of theme files
    const filenames = await fetchThemeList();

    // Fetch each theme's CSS content in parallel
    const themes = await Promise.all(
      filenames.map(async (filename) => {
        const css = await fetchThemeContent(filename);
        const metadata = parseThemeMetadata(css, filename);

        return {
          id: filename.replace('.css', ''),
          filename,
          css,
          ...metadata,
        };
      })
    );

    return themes;
  } catch (error) {
    console.error('Failed to fetch themes from GitHub:', error);
    throw error;
  }
}
