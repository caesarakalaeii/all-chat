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

import { apiClient } from './client'
import type { MaintenanceWindow, CreateMaintenanceRequest } from '@/lib/types/maintenance'

export const maintenanceApi = {
  list(): Promise<MaintenanceWindow[]> {
    return apiClient.get<MaintenanceWindow[]>('/api/v1/admin/maintenance')
  },

  create(data: CreateMaintenanceRequest): Promise<MaintenanceWindow> {
    return apiClient.post<MaintenanceWindow>('/api/v1/admin/maintenance', data)
  },

  remove(id: string): Promise<void> {
    return apiClient.delete(`/api/v1/admin/maintenance/${id}`)
  },

  upcoming(): Promise<MaintenanceWindow[]> {
    return apiClient.get<MaintenanceWindow[]>('/api/v1/maintenance/upcoming')
  },
}
