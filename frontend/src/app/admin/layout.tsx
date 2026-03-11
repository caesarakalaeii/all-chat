import { ReactNode } from 'react'
import { ProtectedRoute } from '@/components/ProtectedRoute'
import { ToastProvider } from '@/components/admin/ToastProvider'
import { AdminNav } from '@/components/AdminNav'

function AdminLayoutContent({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-bg">
      <ToastProvider />
      <AdminNav />
      <main>{children}</main>
    </div>
  )
}

export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <ProtectedRoute requireAdmin={true}>
      <AdminLayoutContent>{children}</AdminLayoutContent>
    </ProtectedRoute>
  )
}
