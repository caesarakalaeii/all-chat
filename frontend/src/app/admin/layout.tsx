'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { ReactNode } from 'react';
import { ProtectedRoute } from '@/components/ProtectedRoute';
import { ToastProvider } from '@/components/admin/ToastProvider';

function AdminLayoutContent({ children }: { children: ReactNode }) {
  const pathname = usePathname();

  const navLinkClass = (href: string) =>
    `inline-flex items-center px-1 pt-1 text-sm font-medium border-b-2 ${
      pathname === href
        ? 'text-blue-600 border-blue-500'
        : 'text-gray-700 border-transparent hover:text-gray-900 hover:border-blue-500'
    }`;

  return (
    <div className="min-h-screen bg-gray-50">
      <ToastProvider />

      {/* Admin Navigation */}
      <nav className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex">
              <Link href="/admin" className="flex items-center">
                <span className="text-xl font-bold text-gray-900">All-Chat Admin</span>
              </Link>
              <div className="ml-10 flex space-x-8">
                <Link href="/admin/users" className={navLinkClass('/admin/users')}>
                  Users
                </Link>
                <Link href="/admin/overlays" className={navLinkClass('/admin/overlays')}>
                  Overlays
                </Link>
                <Link href="/admin/sources" className={navLinkClass('/admin/sources')}>
                  Sources
                </Link>
                <Link href="/admin/viewers" className={navLinkClass('/admin/viewers')}>
                  Viewers
                </Link>
              </div>
            </div>
            <div className="flex items-center">
              <Link
                href="/"
                className="text-sm font-medium text-gray-700 hover:text-gray-900"
              >
                Back to App
              </Link>
            </div>
          </div>
        </div>
      </nav>

      {/* Admin Content */}
      <main className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        {children}
      </main>
    </div>
  );
}

export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <ProtectedRoute requireAdmin={true}>
      <AdminLayoutContent>{children}</AdminLayoutContent>
    </ProtectedRoute>
  );
}
