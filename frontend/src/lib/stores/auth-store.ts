/**
 * Authentication Store (Zustand)
 *
 * Global state management for user authentication.
 * Stores user info and JWT token in memory and localStorage.
 *
 * Usage in components:
 *   const { user, token, login, logout } = useAuthStore();
 */

import { create } from 'zustand';
import type { User } from '../types/auth';
import { authApi } from '../api/auth';

interface AuthStore {
  user: User | null;
  token: string | null;
  loading: boolean;

  // Actions
  setToken: (token: string) => void;
  setUser: (user: User) => void;
  logout: () => void;
  init: () => Promise<void>;
}

export const useAuthStore = create<AuthStore>((set, get) => ({
  user: null,
  token: null,
  loading: true,

  setToken: (token: string) => {
    if (typeof window !== 'undefined') {
      localStorage.setItem('jwt_token', token);
    }
    set({ token });
  },

  setUser: (user: User) => {
    set({ user, loading: false });
  },

  logout: () => {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('jwt_token');
    }
    set({ user: null, token: null, loading: false });
  },

  init: async () => {
    if (typeof window === 'undefined') {
      set({ loading: false });
      return;
    }

    const token = localStorage.getItem('jwt_token');
    if (!token) {
      set({ loading: false });
      return;
    }

    set({ token });

    try {
      const user = await authApi.getMe();
      set({ user, loading: false });
    } catch (error) {
      // Token invalid, clear it
      localStorage.removeItem('jwt_token');
      set({ user: null, token: null, loading: false });
    }
  }
}));
