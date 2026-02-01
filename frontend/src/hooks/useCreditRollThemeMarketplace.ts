/**
 * Credit Roll Theme Marketplace Hook
 *
 * Manages credit roll theme loading, caching, filtering, and favorites
 */

import { useState, useEffect, useCallback } from 'react';
import type { Theme, ThemeCacheData } from '@/lib/theme-marketplace/types';
import { fetchAllCreditRollThemes } from '@/lib/theme-marketplace/credit-roll-github-api';

const CACHE_KEY = 'credit-roll-themes-marketplace';
const FAVORITES_KEY = 'credit-roll-themes-favorites';
const CACHE_TTL = 24 * 60 * 60 * 1000; // 24 hours

export function useCreditRollThemeMarketplace() {
  const [themes, setThemes] = useState<Theme[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedTags, setSelectedTags] = useState<string[]>([]);
  const [favorites, setFavorites] = useState<string[]>([]);

  // Load favorites from localStorage
  useEffect(() => {
    const stored = localStorage.getItem(FAVORITES_KEY);
    if (stored) {
      try {
        setFavorites(JSON.parse(stored));
      } catch {
        setFavorites([]);
      }
    }
  }, []);

  // Save favorites to localStorage
  useEffect(() => {
    localStorage.setItem(FAVORITES_KEY, JSON.stringify(favorites));
  }, [favorites]);

  const loadThemes = useCallback(async (forceRefresh = false) => {
    setLoading(true);
    setError(null);

    try {
      if (!forceRefresh && typeof window !== 'undefined') {
        const cached = localStorage.getItem(CACHE_KEY);
        if (cached) {
          const data: ThemeCacheData = JSON.parse(cached);
          const now = Date.now();
          if (now - data.timestamp <= data.ttl) {
            setThemes(data.themes);
            setLoading(false);
            return;
          }
        }
      }

      const fetchedThemes = await fetchAllCreditRollThemes();
      setThemes(fetchedThemes);

      if (typeof window !== 'undefined') {
        const cacheData: ThemeCacheData = {
          timestamp: Date.now(),
          ttl: CACHE_TTL,
          themes: fetchedThemes,
        };
        localStorage.setItem(CACHE_KEY, JSON.stringify(cacheData));
      }
    } catch (err) {
      console.error('Failed to load credit roll themes:', err);
      setError(err instanceof Error ? err.message : 'Failed to load themes');
      setThemes([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadThemes();
  }, [loadThemes]);

  const filteredThemes = themes.filter((theme) => {
    const matchesSearch =
      searchQuery === '' ||
      theme.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      theme.description.toLowerCase().includes(searchQuery.toLowerCase());

    const matchesTags =
      selectedTags.length === 0 ||
      selectedTags.every((tag) => theme.tags.includes(tag));

    return matchesSearch && matchesTags;
  });

  const sortedThemes = [...filteredThemes].sort((a, b) => {
    const aFav = favorites.includes(a.id);
    const bFav = favorites.includes(b.id);
    if (aFav && !bFav) return -1;
    if (!aFav && bFav) return 1;
    return a.name.localeCompare(b.name);
  });

  const availableTags = Array.from(
    new Set(themes.flatMap((theme) => theme.tags))
  ).sort();

  const toggleTag = useCallback((tag: string) => {
    setSelectedTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  }, []);

  const toggleFavorite = useCallback((themeId: string) => {
    setFavorites((prev) =>
      prev.includes(themeId)
        ? prev.filter((id) => id !== themeId)
        : [...prev, themeId]
    );
  }, []);

  const clearFilters = useCallback(() => {
    setSearchQuery('');
    setSelectedTags([]);
  }, []);

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
    hasActiveFilters: searchQuery !== '' || selectedTags.length > 0,
    totalCount: themes.length,
    filteredCount: filteredThemes.length,
    refreshThemes: () => loadThemes(true),
  };
}
