/**
 * Theme Marketplace Cache Manager
 *
 * Manages localStorage caching of theme data.
 */

import type { Theme, ThemeCacheData } from './types';

const CACHE_KEY = 'theme_marketplace_cache';
const FAVORITES_KEY = 'theme_marketplace_favorites';
const CACHE_TTL = 24 * 60 * 60 * 1000; // 24 hours in milliseconds

/**
 * Get cached themes from localStorage
 */
export function getCachedThemes(): ThemeCacheData | null {
  if (typeof window === 'undefined') return null;

  try {
    const cached = localStorage.getItem(CACHE_KEY);
    if (!cached) return null;

    const data: ThemeCacheData = JSON.parse(cached);
    return data;
  } catch (error) {
    console.error('Failed to read theme cache:', error);
    return null;
  }
}

/**
 * Check if cache is expired
 */
export function isCacheExpired(cache: ThemeCacheData): boolean {
  const now = Date.now();
  return now - cache.timestamp > cache.ttl;
}

/**
 * Save themes to cache
 */
export function cacheThemes(themes: Theme[]): void {
  if (typeof window === 'undefined') return;

  try {
    const cacheData: ThemeCacheData = {
      timestamp: Date.now(),
      ttl: CACHE_TTL,
      themes,
    };

    localStorage.setItem(CACHE_KEY, JSON.stringify(cacheData));
  } catch (error) {
    console.error('Failed to cache themes:', error);
  }
}

/**
 * Clear theme cache
 */
export function clearCache(): void {
  if (typeof window === 'undefined') return;
  localStorage.removeItem(CACHE_KEY);
}

/**
 * Get favorite theme IDs
 */
export function getFavorites(): string[] {
  if (typeof window === 'undefined') return [];

  try {
    const favorites = localStorage.getItem(FAVORITES_KEY);
    return favorites ? JSON.parse(favorites) : [];
  } catch (error) {
    console.error('Failed to read favorites:', error);
    return [];
  }
}

/**
 * Save favorite theme IDs
 */
export function saveFavorites(favorites: string[]): void {
  if (typeof window === 'undefined') return;

  try {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites));
  } catch (error) {
    console.error('Failed to save favorites:', error);
  }
}

/**
 * Toggle favorite status for a theme
 */
export function toggleFavorite(themeId: string): string[] {
  const favorites = getFavorites();
  const newFavorites = favorites.includes(themeId)
    ? favorites.filter((id) => id !== themeId)
    : [...favorites, themeId];

  saveFavorites(newFavorites);
  return newFavorites;
}
