/**
 * Authentication and User Types
 *
 * These types match the backend API responses from the Auth Service.
 * Used throughout the application for type safety.
 */

export interface User {
  id: string;
  twitch_id: string;
  username: string;
  display_name: string;
  profile_image_url?: string;
  email?: string;
  created_at: string;
  updated_at: string;
}

export interface AuthState {
  user: User | null;
  token: string | null;
  loading: boolean;
}

export interface LoginResponse {
  url: string;
}

export interface TokenResponse {
  token: string;
  user: User;
}
