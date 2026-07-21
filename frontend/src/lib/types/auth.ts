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
 * Authentication and User Types
 *
 * These types match the backend API responses from the Auth Service.
 * Used throughout the application for type safety.
 */

export interface User {
  id: string
  twitch_id?: string | null
  google_id?: string | null
  kick_id?: string | null
  auth_provider?: string
  username: string
  display_name: string
  profile_image_url?: string
  is_admin: boolean
  is_premium: boolean
  is_beta_tester: boolean
  // Ambassador role (ADR-0041): premium + early-access + eligible for the public
  // homepage showcase. Admin-granted; the streamer opts into the public card separately.
  is_ambassador: boolean
  // Impersonation state surfaced from /auth/me when the JWT carries an
  // ImpersonatedBy claim (audit H3). Absent when not impersonating.
  impersonating?: boolean
  impersonated_by?: string
  created_at: string
  updated_at: string
  // null = user has not finished/dismissed the first-run setup guide.
  onboarding_completed_at?: string | null
}

export interface LoginResponse {
  auth_url: string
}
