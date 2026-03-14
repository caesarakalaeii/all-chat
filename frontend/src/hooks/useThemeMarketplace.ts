/**
 * Theme Marketplace Hook
 *
 * Manages theme marketplace state with filtering and favorites.
 */

'use client';

import { useState, useEffect, useMemo, useCallback } from 'react';
import type { Theme } from '@/lib/theme-marketplace/types';
import { fetchAllThemes } from '@/lib/theme-marketplace/github-api';
import {
  getCachedThemes,
  isCacheExpired,
  cacheThemes,
  getFavorites,
  toggleFavorite as toggleFavoriteCache,
} from '@/lib/theme-marketplace/cache';
import { EMBEDDED_FALLBACK_THEMES } from '@/lib/theme-marketplace/constants';

/**
 * Debounce hook
 */
function useDebounce<T>(value: T, delay: number): T {
  const [debouncedValue, setDebouncedValue] = useState(value);

  useEffect(() => {
    const handler = setTimeout(() => setDebouncedValue(value), delay);
    return () => clearTimeout(handler);
  }, [value, delay]);

  return debouncedValue;
}

/**
 * Filter themes by search query and tags
 */
function filterThemes(
  themes: Theme[],
  searchQuery: string,
  selectedTags: string[]
): Theme[] {
  return themes.filter((theme) => {
    // Search filter: match name, description, or tags
    const searchMatch =
      searchQuery === '' ||
      [theme.name, theme.description, ...theme.tags].some((field) =>
        field.toLowerCase().includes(searchQuery.toLowerCase())
      );

    // Tag filter: AND logic (all selected tags must match)
    const tagMatch =
      selectedTags.length === 0 ||
      selectedTags.every((tag) => theme.tags.includes(tag));

    return searchMatch && tagMatch;
  });
}

/**
 * Extract all unique tags from themes
 */
function extractUniqueTags(themes: Theme[]): string[] {
  const tagSet = new Set<string>();
  themes.forEach((theme) => {
    theme.tags.forEach((tag) => tagSet.add(tag));
  });
  return Array.from(tagSet).sort();
}

export function useThemeMarketplace() {
  const [themes, setThemes] = useState<Theme[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [searchQuery, setSearchQuery] = useState('');
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [favorites, setFavorites] = useState<string[]>([]);

  // Debounce search query
  const debouncedSearch = useDebounce(searchQuery, 300);

  const loadThemes = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      // Check cache first
      const cached = getCachedThemes();
      if (cached && !isCacheExpired(cached)) {
        setThemes(cached.themes);
        setLoading(false);
        return;
      }

      // Fetch from GitHub
      const fetchedThemes = await fetchAllThemes();
      setThemes(fetchedThemes);
      cacheThemes(fetchedThemes);
      setLoading(false);
    } catch (err) {
      console.error('Failed to load themes:', err);
      setError('Failed to load themes from GitHub. Using fallback themes.');

      // Use fallback themes
      setThemes(EMBEDDED_FALLBACK_THEMES);
      setLoading(false);
    }
  }, []);

  // Load themes on mount — setState calls here are intentional on mount only
  /* eslint-disable react-hooks/set-state-in-effect */
  useEffect(() => {
    loadThemes();
    setFavorites(getFavorites());
  }, [loadThemes]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // Filter themes
  const filteredThemes = useMemo(
    () => filterThemes(themes, debouncedSearch, selectedTags),
    [themes, debouncedSearch, selectedTags]
  );

  // Sort: favorites first
  const sortedThemes = useMemo(() => {
    return [...filteredThemes].sort((a, b) => {
      const aFav = favorites.includes(a.id);
      const bFav = favorites.includes(b.id);
      if (aFav && !bFav) return -1;
      if (!aFav && bFav) return 1;
      return 0;
    });
  }, [filteredThemes, favorites]);

  // Extract unique tags from ALL themes (not filtered)
  const availableTags = useMemo(() => extractUniqueTags(themes), [themes]);

  // Toggle tag filter
  const toggleTag = useCallback((tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  }, []);

  // Toggle favorite
  const toggleFavorite = useCallback((themeId: string) => {
    const newFavorites = toggleFavoriteCache(themeId);
    setFavorites(newFavorites);
  }, []);

  // Clear all filters
  const clearFilters = useCallback(() => {
    setSearchQuery('');
    setSelectedTags([]);
  }, []);

  // Check if any filters are active
  const hasActiveFilters = searchQuery !== '' || selectedTags.length > 0;

  return {
    themes: sortedThemes,
    loading,
    error,
    searchQuery,
    setSearchQuery,
    selectedTags,
    toggleTag,
    favorites,
    toggleFavorite,
    availableTags,
    clearFilters,
    hasActiveFilters,
    totalCount: themes.length,
    filteredCount: filteredThemes.length,
    refreshThemes: loadThemes,
  };
}
