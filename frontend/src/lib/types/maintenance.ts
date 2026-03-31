export interface MaintenanceWindow {
  id: string
  title: string
  description: string
  starts_at: string
  ends_at: string
  created_by: string
  created_at: string
}

export interface CreateMaintenanceRequest {
  title: string
  description?: string
  starts_at: string
  ends_at: string
}
