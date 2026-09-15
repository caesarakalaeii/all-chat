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
 * Localization contributor API (ADR-0063): the beta-tester translation tool.
 * Contributor endpoints live under /localization (service-gated behind the
 * localization_contribution early-access gate); admin endpoints under
 * /admin/localization. The key list is derived from the English catalog in the
 * frontend, never from the backend.
 */

import { apiClient } from './client'

export interface LocalizationLocale {
  code: string
  english_name: string
  native_name: string
}

export interface LocalizationTranslation {
  key: string
  value: string
  status: 'pending' | 'approved' | 'rejected'
  review_note?: string | null
}

export interface SubmitRow {
  key: string
  value: string
  /** English source, sent for placeholder-parity checking server-side. */
  source_value: string
}

export interface SubmitResult {
  accepted: number
  failed: Array<{ key: string; error: string }>
}

export const localizationApi = {
  listLocales(): Promise<LocalizationLocale[]> {
    return apiClient
      .get<{ locales: LocalizationLocale[] }>('/api/v1/localization/locales')
      .then((r) => r.locales)
  },

  requestLocale(data: {
    code: string
    english_name: string
    native_name: string
  }): Promise<{ message: string; code: string }> {
    return apiClient.post('/api/v1/localization/locales', data)
  },

  myTranslations(code: string): Promise<LocalizationTranslation[]> {
    return apiClient
      .get<{ translations: LocalizationTranslation[] }>(
        `/api/v1/localization/locales/${encodeURIComponent(code)}/translations`
      )
      .then((r) => r.translations)
  },

  submit(
    code: string,
    translations: SubmitRow[]
  ): Promise<SubmitResult> {
    return apiClient.post(
      `/api/v1/localization/locales/${encodeURIComponent(code)}/translations`,
      { translations }
    )
  },
}

// --- Admin endpoints (review queue, locale approval, export) ---

export interface LocaleRequest {
  code: string
  english_name: string
  native_name: string
  requested_by?: string
}

export interface AdminTranslation extends LocalizationTranslation {
  locale: string
  submitted_by?: string
}

export interface LocaleProgress {
  code: string
  pending: number
  approved: number
  rejected: number
}

export const localizationAdminApi = {
  localeRequests(): Promise<LocaleRequest[]> {
    return apiClient
      .get<{ locales: LocaleRequest[] }>('/api/v1/admin/localization/locales/requests')
      .then((r) => r.locales)
  },

  reviewLocaleRequest(
    code: string,
    approved: boolean
  ): Promise<{ message: string; code: string }> {
    return apiClient.post(
      `/api/v1/admin/localization/locales/${encodeURIComponent(code)}/review`,
      { approved }
    )
  },

  reviewQueue(): Promise<AdminTranslation[]> {
    return apiClient
      .get<{ translations: AdminTranslation[] }>('/api/v1/admin/localization/review')
      .then((r) => r.translations)
  },

  reviewSubmission(
    locale: string,
    key: string,
    approved: boolean,
    review_note?: string
  ): Promise<{ message: string; locale: string; key: string; status: string }> {
    return apiClient.post(
      `/api/v1/admin/localization/review/${encodeURIComponent(locale)}/${encodeURIComponent(key)}`,
      { approved, review_note: review_note ?? '' }
    )
  },

  exportLocale(code: string): Promise<{ locale: string; translations: Array<{ key: string; value: string }> }> {
    return apiClient.get(
      `/api/v1/admin/localization/export/${encodeURIComponent(code)}`
    )
  },

  progress(): Promise<LocaleProgress[]> {
    return apiClient
      .get<{ locales: LocaleProgress[] }>('/api/v1/admin/localization/progress')
      .then((r) => r.locales)
  },
}
