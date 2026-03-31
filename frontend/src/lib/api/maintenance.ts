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
