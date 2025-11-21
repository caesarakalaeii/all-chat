/**
 * Authentication and User Types
 *
 * These types match the backend API responses from the Auth Service.
 * Used throughout the application for type safety.
 */

export interface User {
  id: string;
  twitch_id?: string | null;
  google_id?: string | null;
  tiktok_open_id?: string | null;
  kick_id?: string | null;
  auth_provider?: string;
  username: string;
  display_name: string;
  profile_image_url?: string;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuthState {
  user: User | null;
  token: string | null;
  loading: boolean;
}

export interface LoginResponse {
  auth_url: string;
}

export interface TokenResponse {
  token: string;
  user: User;
}
